// ==========================================
// ClickUp mentions: live toast + daily note refresh
// ==========================================
// The daemon's ClickUp poller publishes a {"type":"clickup"} hint on /ws/events when it adds new
// mentions to the daily note. This page then asks for the ones it has not announced yet and shows a
// toast / desktop notification, and refreshes the note on screen.
//
// "Not announced yet" is a cursor (detectedAt ms + commentId, so mentions detected in the same
// instant are told apart) kept per browser in localStorage, or in memory when storage fails.

const CLICKUP_CURSOR_KEY = 'orbit.clickup.cursor';
const CLICKUP_PAGE_LIMIT = 100; // the API returns at most this many per call
const CLICKUP_MAX_PAGES = 5;

let clickupCursorMemory = null;

function clickupParseCursor(raw) {
    if (!raw) return null;
    const sep = raw.indexOf('|');
    const t = parseInt(sep === -1 ? raw : raw.slice(0, sep), 10);
    if (!Number.isFinite(t) || t <= 0) return null;
    return { t, id: sep === -1 ? '' : raw.slice(sep + 1) };
}

function clickupCursorAfter(a, b) { // is a later than b?
    if (!b) return !!a;
    if (!a) return false;
    return a.t > b.t || (a.t === b.t && a.id > b.id);
}

function clickupReadCursor() {
    let stored = null;
    try {
        stored = clickupParseCursor(localStorage.getItem(CLICKUP_CURSOR_KEY));
    } catch (e) { /* private mode or blocked storage: the in-memory cursor is used */ }
    return clickupCursorAfter(clickupCursorMemory, stored) ? clickupCursorMemory : stored;
}

function clickupWriteCursor(c) {
    clickupCursorMemory = c;
    try {
        localStorage.setItem(CLICKUP_CURSOR_KEY, `${c.t}|${c.id}`);
    } catch (e) { /* kept in memory for this page */ }
}

async function fetchClickUpMentionsAfter(cursor) {
    const token = serverInfo ? serverInfo.authToken : '';
    const headers = token ? { 'Authorization': 'Bearer ' + token } : {};
    const q = `detected_after=${cursor.t}&after_id=${encodeURIComponent(cursor.id)}`;
    const res = await fetch(`/api/clickup/mentions?${q}`, { headers });
    if (!res.ok) return null;
    const data = await res.json();
    return (data && Array.isArray(data.mentions)) ? data.mentions : [];
}

async function refreshClickUpMentions() {
    let cursor = clickupReadCursor();
    if (!cursor) {
        // First time on this browser: start from now instead of announcing the whole backlog.
        clickupWriteCursor({ t: Date.now(), id: '' });
        return;
    }

    // With a cursor the API returns items ordered by (detectedAt, commentId), so moving the cursor
    // to the last item of each batch never skips any, even when several share the same instant.
    const mentions = [];
    try {
        for (let page = 0; page < CLICKUP_MAX_PAGES; page++) {
            const batch = await fetchClickUpMentionsAfter(cursor);
            if (!batch || !batch.length) break;
            mentions.push(...batch);
            const last = batch[batch.length - 1];
            const t = Date.parse(last.detectedAt);
            if (Number.isFinite(t)) cursor = { t, id: last.commentId || '' };
            if (batch.length < CLICKUP_PAGE_LIMIT) break;
        }
    } catch (e) {
        return;
    }
    if (!mentions.length) return;
    clickupWriteCursor(cursor);

    const latest = mentions[mentions.length - 1];
    let msg = mentions.length === 1
        ? `📬 ${latest.author || 'ClickUp'} en ${latest.taskName || 'una tarea'}`
        : `📬 ${mentions.length} menciones nuevas de ClickUp`;

    // The to-do lines went to the daily note: refresh it if that day is on screen.
    const touchesOpenNote = typeof calSelectedDate !== 'undefined' && mentions.some(m => m.noteDate === calSelectedDate);
    if (touchesOpenNote) {
        if (typeof isCalEditing !== 'undefined' && isCalEditing) {
            // Saving sends the loaded note as base and the server keeps these lines (atomic merge).
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
