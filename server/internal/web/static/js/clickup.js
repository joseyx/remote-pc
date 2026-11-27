// ==========================================
// ClickUp mentions: live toast + daily note refresh
// ==========================================
// The daemon's ClickUp poller publishes a {"type":"clickup"} hint on /ws/events when it adds new
// mentions to the daily note. This page then asks for the ones it has not announced yet (a cursor
// kept per browser in localStorage, or in memory when storage is unavailable), shows a toast /
// desktop notification and refreshes the note on screen.

const CLICKUP_LAST_SEEN_KEY = 'orbit.clickup.lastDetected';
const CLICKUP_PAGE_LIMIT = 100; // the API returns at most this many per call
const CLICKUP_MAX_PAGES = 5;

let clickupLastSeenMemory = 0;

function clickupReadLastSeen() {
    let stored = 0;
    try {
        const v = parseInt(localStorage.getItem(CLICKUP_LAST_SEEN_KEY) || '', 10);
        if (Number.isFinite(v)) stored = v;
    } catch (e) { /* private mode or blocked storage: the in-memory cursor is used */ }
    return Math.max(stored, clickupLastSeenMemory);
}

function clickupWriteLastSeen(ms) {
    clickupLastSeenMemory = ms;
    try {
        localStorage.setItem(CLICKUP_LAST_SEEN_KEY, String(ms));
    } catch (e) { /* kept in memory for this page */ }
}

async function fetchClickUpMentionsAfter(cursor) {
    const token = serverInfo ? serverInfo.authToken : '';
    const headers = token ? { 'Authorization': 'Bearer ' + token } : {};
    const res = await fetch(`/api/clickup/mentions?detected_after=${cursor}`, { headers });
    if (!res.ok) return null;
    const data = await res.json();
    return (data && Array.isArray(data.mentions)) ? data.mentions : [];
}

async function refreshClickUpMentions() {
    let cursor = clickupReadLastSeen();
    if (!cursor) {
        // First time on this browser: start from now instead of announcing the whole backlog.
        clickupWriteLastSeen(Date.now());
        return;
    }

    // The API returns items oldest-detected first when given a cursor, so advancing the cursor to
    // the last item of each batch never skips any.
    const mentions = [];
    try {
        for (let page = 0; page < CLICKUP_MAX_PAGES; page++) {
            const batch = await fetchClickUpMentionsAfter(cursor);
            if (!batch || !batch.length) break;
            mentions.push(...batch);
            for (const m of batch) {
                const t = Date.parse(m.detectedAt);
                if (Number.isFinite(t) && t > cursor) cursor = t;
            }
            if (batch.length < CLICKUP_PAGE_LIMIT) break;
        }
    } catch (e) {
        return;
    }
    if (!mentions.length) return;
    clickupWriteLastSeen(cursor);

    const latest = mentions[mentions.length - 1];
    let msg = mentions.length === 1
        ? `📬 ${latest.author || 'ClickUp'} en ${latest.taskName || 'una tarea'}`
        : `📬 ${mentions.length} menciones nuevas de ClickUp`;

    // The to-do lines went to the daily note: refresh it if that day is on screen.
    const touchesOpenNote = typeof calSelectedDate !== 'undefined' && mentions.some(m => m.noteDate === calSelectedDate);
    if (touchesOpenNote) {
        if (typeof isCalEditing !== 'undefined' && isCalEditing) {
            // saveCalendarNote merges these lines into the edit instead of overwriting them.
            msg += ' · se agregará a tu nota al guardar';
        } else if (typeof reloadCalendarNote === 'function') {
            reloadCalendarNote();
        }
    }

    showToast(msg); // textContent: ClickUp text is never parsed as HTML here
    if (typeof playNotificationChime === 'function') playNotificationChime();
    if (document.hidden && 'Notification' in window && Notification.permission === 'granted') {
        try {
            const body = mentions.length === 1
                ? `${latest.author || ''} en ${latest.taskName || ''}: ${latest.text || ''}`
                : msg;
            new Notification('Orbit · ClickUp', { body });
        } catch (e) { /* notifications unavailable */ }
    }
}

// --- Keeping concurrent to-dos when a note edited in the browser is saved ---
// Background jobs (the ClickUp poller) can add lines tagged <!-- source:id --> while the note is
// open in the editor. Saving the editor's text as-is would silently drop them, so before saving we
// re-read the note and carry over every tagged line that appeared after the editor was loaded.
const TODO_MARKER_RE = /<!--\s*([a-z][a-z0-9_]*:[A-Za-z0-9_-]+)\s*-->/;
const NOTES_GENERAL_HEADER = '## 📌 Notas Generales';

function taggedLines(text) {
    const out = new Map();
    for (const line of String(text || '').split('\n')) {
        const m = line.match(TODO_MARKER_RE);
        if (m && !out.has(m[1])) out.set(m[1], line);
    }
    return out;
}

// mergeConcurrentTodoLines returns editedContent plus the tagged lines that exist in latestContent
// but were neither in loadedContent (what the editor started from) nor in the edit itself. Lines the
// user deleted while editing were in loadedContent, so they stay deleted.
function mergeConcurrentTodoLines(loadedContent, latestContent, editedContent) {
    const loaded = taggedLines(loadedContent);
    const edited = taggedLines(editedContent);
    const added = [];
    for (const [marker, line] of taggedLines(latestContent)) {
        if (!loaded.has(marker) && !edited.has(marker)) added.push(line);
    }
    if (!added.length) return { content: editedContent, added: 0 };

    const idx = editedContent.indexOf(NOTES_GENERAL_HEADER);
    let content;
    if (idx === -1) {
        content = editedContent.replace(/\s*$/, '') + '\n\n' + NOTES_GENERAL_HEADER + '\n' + added.join('\n') + '\n';
    } else {
        const at = idx + NOTES_GENERAL_HEADER.length;
        content = editedContent.slice(0, at) + '\n' + added.join('\n') + editedContent.slice(at);
    }
    return { content, added: added.length };
}
