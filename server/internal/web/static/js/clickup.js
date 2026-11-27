// ==========================================
// ClickUp mentions: inbox, live toast + daily note refresh
// ==========================================
// The daemon's ClickUp poller publishes a {"type":"clickup"} hint on /ws/events when it finds new
// mentions or some get resolved. This page then refreshes the inbox (header badge + side panel),
// asks for the mentions it has not announced yet and shows a toast / desktop notification, and
// refreshes the daily note on screen when the plugin also writes there.
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
    refreshClickUpInbox(); // badge and panel follow every hint (new or resolved items)

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

// ==========================================
// Inbox: header badge + side panel
// ==========================================
// Every list comes from the daemon (GET /api/clickup/mentions): pending items by default, or the
// last 30 days with "Todas". The DOM is built with textContent only: ClickUp text is never HTML.

const CLICKUP_INBOX_REFRESH_MS = 5 * 60 * 1000; // also keeps "Hoy" / "Ayer" labels current
const CLICKUP_LOOKBACK_HOURS = 72;
const CLICKUP_ID_RE = /^[A-Za-z0-9_-]{1,64}$/;

let clickupInboxView = 'pending'; // 'pending' | 'all'
let clickupInboxData = null;
let clickupInboxSeq = 0; // drops responses that arrive after a newer request
let clickupPollBusy = false;

function clickupAuthHeaders(json) {
    const token = serverInfo ? serverInfo.authToken : '';
    return {
        ...(json ? { 'Content-Type': 'application/json' } : {}),
        ...(token ? { 'Authorization': 'Bearer ' + token } : {})
    };
}

function initClickUpInbox() {
    refreshClickUpInbox();
    setInterval(refreshClickUpInbox, CLICKUP_INBOX_REFRESH_MS);
    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape' && isClickUpInboxOpen()) closeClickUpInbox();
    });
    document.addEventListener('mousedown', (e) => {
        if (!isClickUpInboxOpen()) return;
        const panel = document.getElementById('clickup-inbox');
        const btn = document.getElementById('header-clickup-btn');
        // Clicks in dialogs opened from the panel (alerts) do not close it either.
        if (panel.contains(e.target) || (btn && btn.contains(e.target)) || e.target.closest('.modal-overlay, #toast')) return;
        closeClickUpInbox();
    });
}

async function refreshClickUpInbox() {
    const seq = ++clickupInboxSeq;
    const btn = document.getElementById('header-clickup-btn');
    try {
        const q = clickupInboxView === 'all' ? '?include_answered=1' : '';
        const res = await fetch('/api/clickup/mentions' + q, { headers: clickupAuthHeaders(false) });
        if (seq !== clickupInboxSeq) return;
        if (!res.ok) { // 503: integration unavailable
            if (btn) btn.style.display = 'none';
            return;
        }
        const data = await res.json();
        if (seq !== clickupInboxSeq) return;
        clickupInboxData = data;
        const pending = Number(data.pendingCount) || 0;
        if (btn) btn.style.display = (data.enabled || pending > 0) ? 'inline-flex' : 'none';
        renderClickUpBadge(pending);
        if (isClickUpInboxOpen()) renderClickUpInbox();
    } catch (e) { /* daemon unreachable: keep what is on screen */ }
}

function renderClickUpBadge(pending) {
    const badge = document.getElementById('header-clickup-badge');
    if (!badge) return;
    badge.textContent = pending > 99 ? '99+' : String(pending);
    badge.hidden = pending === 0;
    const btn = document.getElementById('header-clickup-btn');
    if (btn) btn.title = pending ? `Bandeja de ClickUp: ${pending} pendiente(s)` : 'Bandeja de ClickUp: nada pendiente';
    const tabCount = document.getElementById('clickup-tab-pending-count');
    if (tabCount) tabCount.textContent = pending ? `(${pending})` : '';
}

function isClickUpInboxOpen() {
    const panel = document.getElementById('clickup-inbox');
    return !!panel && !panel.hidden;
}

function toggleClickUpInbox() {
    if (isClickUpInboxOpen()) closeClickUpInbox();
    else openClickUpInbox();
}

function openClickUpInbox() {
    const panel = document.getElementById('clickup-inbox');
    if (!panel) return;
    panel.hidden = false;
    renderClickUpInbox(); // what we have now, then fresh data
    refreshClickUpInbox();
}

function closeClickUpInbox() {
    const panel = document.getElementById('clickup-inbox');
    if (panel) panel.hidden = true;
}

function setClickUpInboxView(view) {
    clickupInboxView = view === 'all' ? 'all' : 'pending';
    document.getElementById('clickup-tab-pending')?.classList.toggle('active', clickupInboxView === 'pending');
    document.getElementById('clickup-tab-all')?.classList.toggle('active', clickupInboxView === 'all');
    clickupInboxData = null; // the other list is not loaded yet
    renderClickUpInbox();
    refreshClickUpInbox();
}

function clickupDate(raw) {
    const d = new Date(raw);
    return (isNaN(d.getTime()) || d.getFullYear() < 2000) ? null : d; // Go zero time = never
}

function clickupDayLabel(d) {
    const startOf = (x) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime();
    const days = Math.round((startOf(new Date()) - startOf(d)) / 86400000);
    if (days === 0) return 'Hoy';
    if (days === 1) return 'Ayer';
    return d.toLocaleDateString('es', { weekday: 'long', day: 'numeric', month: 'short' });
}

function clickupTime(d) {
    return d.toLocaleTimeString('es', { hour: '2-digit', minute: '2-digit' });
}

// Link to the comment itself; ClickUp opens the task and scrolls to it (the task alone if the
// parameters are ignored).
function clickupCommentURL(m) {
    if (!CLICKUP_ID_RE.test(m.taskId || '')) return null;
    let url = 'https://app.clickup.com/t/' + m.taskId;
    const root = m.threadId || m.commentId;
    if (CLICKUP_ID_RE.test(root || '')) {
        url += '?comment=' + root;
        if (m.threadId && CLICKUP_ID_RE.test(m.commentId || '')) url += '&threadedComment=' + m.commentId;
    }
    return url;
}

function clickupEl(tag, className, text) {
    const el = document.createElement(tag);
    if (className) el.className = className;
    if (text !== undefined) el.textContent = text;
    return el;
}

function renderClickUpStatus() {
    const el = document.getElementById('clickup-inbox-status');
    if (!el) return;
    el.classList.remove('is-error');
    if (clickupPollBusy) {
        el.textContent = 'Revisando ClickUp…';
        return;
    }
    const data = clickupInboxData;
    if (!data) {
        el.textContent = 'Cargando…';
        return;
    }
    if (!data.enabled) {
        el.textContent = 'El plugin ClickUp está desactivado (Orbit > Plugins).';
        el.classList.add('is-error');
        return;
    }
    const st = data.status || {};
    const end = clickupDate(st.lastPollEnd);
    if (!end) {
        el.textContent = 'Orbit todavía no ha revisado ClickUp.';
        return;
    }
    let txt = `Última revisión: ${clickupDayLabel(end).toLowerCase()} ${clickupTime(end)} · ${st.lastChecked || 0} tarea(s)`;
    if (st.lastError) {
        txt += ` · ${st.lastError}`;
        el.classList.add('is-error');
    }
    el.textContent = txt;
}

function renderClickUpInbox() {
    renderClickUpStatus();
    const list = document.getElementById('clickup-inbox-list');
    if (!list) return;
    list.replaceChildren();
    const data = clickupInboxData;
    if (!data) return;

    const mentions = Array.isArray(data.mentions) ? data.mentions : [];
    if (!mentions.length) {
        list.appendChild(clickupEl('div', 'clickup-empty', clickupInboxView === 'pending'
            ? '🎉 Nada pendiente en ClickUp.'
            : 'Sin menciones en los últimos 30 días.'));
        return;
    }
    let lastDay = '';
    for (const m of mentions) { // newest comment first
        const d = clickupDate(m.date);
        const day = d ? clickupDayLabel(d) : 'Sin fecha';
        if (day !== lastDay) {
            list.appendChild(clickupEl('div', 'clickup-day', day));
            lastDay = day;
        }
        list.appendChild(renderClickUpItem(m, d));
    }
}

function renderClickUpItem(m, d) {
    const resolved = !!m.answered || !!m.doneAt;
    const item = clickupEl('div', 'clickup-item' + (resolved ? ' is-resolved' : ''));

    const icon = m.reason === 'hilo' ? '↩️' : (m.reason === 'ticket' ? '📝' : '💬');
    const verb = m.reason === 'hilo' ? 'respondió tu hilo' : (m.reason === 'ticket' ? 'comentó tu ticket' : 'te mencionó');
    const head = clickupEl('div', 'clickup-item-head');
    head.appendChild(clickupEl('span', '', icon));
    head.appendChild(clickupEl('strong', '', m.author || 'Alguien'));
    head.appendChild(clickupEl('span', '', verb));
    head.appendChild(clickupEl('span', 'clickup-item-time', d ? clickupTime(d) : ''));
    item.appendChild(head);

    const url = clickupCommentURL(m);
    const task = clickupEl(url ? 'a' : 'span', 'clickup-item-task', m.taskName || m.taskId || 'Tarea');
    if (url) {
        task.href = url;
        task.target = '_blank';
        task.rel = 'noopener noreferrer';
    }
    item.appendChild(task);

    if (m.text) item.appendChild(clickupEl('div', 'clickup-item-text', `«${m.text}»`));

    const actions = clickupEl('div', 'clickup-item-actions');
    if (url) {
        const open = clickupEl('a', 'btn btn-sm', 'Abrir en ClickUp ↗');
        open.href = url;
        open.target = '_blank';
        open.rel = 'noopener noreferrer';
        actions.appendChild(open);
    }
    if (m.answered) {
        actions.appendChild(clickupEl('span', 'clickup-chip', 'Respondida'));
    } else if (m.doneAt) {
        actions.appendChild(clickupEl('span', 'clickup-chip', 'Atendida'));
        const undo = clickupEl('button', 'btn btn-sm', '↺ Pendiente');
        undo.title = 'Devolver a pendientes';
        undo.onclick = () => markClickUpMention(m.commentId, false, undo);
        actions.appendChild(undo);
    } else {
        const done = clickupEl('button', 'btn btn-sm btn-success', '✓ Atendida');
        done.title = 'Marcar como atendida';
        done.onclick = () => markClickUpMention(m.commentId, true, done);
        actions.appendChild(done);
    }
    item.appendChild(actions);
    return item;
}

async function markClickUpMention(commentId, done, btn) {
    if (!CLICKUP_ID_RE.test(commentId || '')) return;
    if (btn) btn.disabled = true;
    try {
        const res = await fetch(`/api/clickup/mentions/${encodeURIComponent(commentId)}/done`, {
            method: 'POST',
            headers: clickupAuthHeaders(true),
            body: JSON.stringify({ done })
        });
        if (!res.ok) {
            const err = await res.json().catch(() => ({}));
            throw new Error(err.error || `HTTP ${res.status}`);
        }
    } catch (e) {
        if (btn) btn.disabled = false;
        appAlert('No se pudo actualizar la mención: ' + (e.message || e), 'ClickUp');
        return;
    }
    await refreshClickUpInbox();
}

// hours = 0: a regular check now; hours > 0: re-read that far back (recovers older comments).
async function pollClickUpNow(hours) {
    if (clickupPollBusy) return;
    clickupPollBusy = true;
    const buttons = ['clickup-btn-poll', 'clickup-btn-lookback'].map(id => document.getElementById(id));
    buttons.forEach(b => { if (b) b.disabled = true; });
    renderClickUpStatus();
    try {
        const res = await fetch('/api/clickup/poll', {
            method: 'POST',
            headers: clickupAuthHeaders(true),
            body: hours > 0 ? JSON.stringify({ lookback_hours: hours }) : ''
        });
        const data = await res.json().catch(() => ({}));
        if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
        const r = data.result || {};
        showToast(r.new ? `📬 ${r.new} mención(es) nueva(s) de ClickUp` : 'ClickUp revisado: nada nuevo');
    } catch (e) {
        appAlert('La revisión de ClickUp falló: ' + (e.message || e), 'ClickUp');
    } finally {
        clickupPollBusy = false;
        buttons.forEach(b => { if (b) b.disabled = false; });
        await refreshClickUpInbox();
        renderClickUpStatus();
    }
}
