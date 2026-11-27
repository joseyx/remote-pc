// ==========================================
// ClickUp mentions: live toast + daily note refresh
// ==========================================
// The daemon's ClickUp poller publishes a {"type":"clickup"} hint on /ws/events when it adds new
// mentions to the daily note. This page then asks for the ones it has not announced yet (tracked per
// browser in localStorage), shows a toast / desktop notification and refreshes the note on screen.

const CLICKUP_LAST_SEEN_KEY = 'orbit.clickup.lastDetected';

function clickupReadLastSeen() {
    try {
        const v = parseInt(localStorage.getItem(CLICKUP_LAST_SEEN_KEY) || '', 10);
        return Number.isFinite(v) ? v : 0;
    } catch (e) {
        return 0;
    }
}

function clickupWriteLastSeen(ms) {
    try {
        localStorage.setItem(CLICKUP_LAST_SEEN_KEY, String(ms));
    } catch (e) { /* private mode or blocked storage: we just re-announce next time */ }
}

async function refreshClickUpMentions() {
    const lastSeen = clickupReadLastSeen();
    if (!lastSeen) {
        // First time on this browser: start from now instead of announcing the whole backlog.
        clickupWriteLastSeen(Date.now());
        return;
    }

    let data;
    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = token ? { 'Authorization': 'Bearer ' + token } : {};
        const res = await fetch(`/api/clickup/mentions?detected_after=${lastSeen}`, { headers });
        if (!res.ok) return;
        data = await res.json();
    } catch (e) {
        return;
    }

    const mentions = (data && Array.isArray(data.mentions)) ? data.mentions : [];
    if (!mentions.length) return;

    let newest = lastSeen;
    for (const m of mentions) {
        const t = Date.parse(m.detectedAt);
        if (Number.isFinite(t) && t > newest) newest = t;
    }
    clickupWriteLastSeen(newest);

    const first = mentions[0];
    let msg = mentions.length === 1
        ? `📬 ${first.author || 'ClickUp'} en ${first.taskName || 'una tarea'}`
        : `📬 ${mentions.length} menciones nuevas de ClickUp`;

    // The to-do lines went to the daily note: refresh it if that day is on screen.
    const touchesOpenNote = typeof calSelectedDate !== 'undefined' && mentions.some(m => m.noteDate === calSelectedDate);
    if (touchesOpenNote) {
        if (typeof isCalEditing !== 'undefined' && isCalEditing) {
            msg += ' · guarda tu edición y recarga la nota para verla';
        } else if (typeof reloadCalendarNote === 'function') {
            reloadCalendarNote();
        }
    }

    showToast(msg); // textContent: ClickUp text is never parsed as HTML here
    if (typeof playNotificationChime === 'function') playNotificationChime();
    if (document.hidden && 'Notification' in window && Notification.permission === 'granted') {
        try {
            const body = mentions.length === 1
                ? `${first.author || ''} en ${first.taskName || ''}: ${first.text || ''}`
                : msg;
            new Notification('Orbit · ClickUp', { body });
        } catch (e) { /* notifications unavailable */ }
    }
}
