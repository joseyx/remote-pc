// ==========================================
// Main Application Lifecycle & Event Listeners
// ==========================================

function toggleSidebar() {
    const sidebar = document.querySelector('.sidebar');
    const btn = document.getElementById('btn-toggle-sidebar');
    if (!sidebar) return;
    sidebar.classList.toggle('collapsed');
    const isCollapsed = sidebar.classList.contains('collapsed');
    if (btn) {
        btn.style.color = isCollapsed ? 'var(--accent)' : 'var(--text-main)';
        btn.title = isCollapsed ? "Mostrar barra lateral (Ctrl+B)" : "Ocultar barra lateral (Ctrl+B)";
    }
    setTimeout(() => {
        if (isSplitActive) updateSplitDimensions();
        else fitActiveTerminal();
    }, 250);
}

let windowResizeTimer = null;
function handleWindowResize() {
    clearTimeout(windowResizeTimer);
    windowResizeTimer = setTimeout(() => {
        if (isSplitActive) {
            updateSplitDimensions();
        } else if (activeTab === 'terminal') {
            fitActiveTerminal();
        }
    }, 120);

    const statsModal = document.getElementById('modal-sys-stats');
    if (statsModal && statsModal.classList.contains('open') && statsModal.style.display !== 'none') {
        if (!sysStatsCustomPos) {
            positionSysStatsDefault();
        } else {
            const maxLeft = Math.max(0, window.innerWidth - statsModal.offsetWidth);
            const maxTop = Math.max(0, window.innerHeight - statsModal.offsetHeight);
            const left = Math.max(0, Math.min(sysStatsCustomPos.x, maxLeft));
            const top = Math.max(0, Math.min(sysStatsCustomPos.y, maxTop));
            statsModal.style.left = `${Math.round(left)}px`;
            statsModal.style.top = `${Math.round(top)}px`;
        }
    }

    if (isQuakeOpen && quakeFitAddon) {
        try { quakeFitAddon.fit(); } catch (e) {}
    }
}



let lastSessionsSnapshot = '';

async function fetchSessions() {
    try {
        if (document.hidden) return; // Skip heavy DOM operations when tab is backgrounded
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = token ? { 'Authorization': 'Bearer ' + token } : {};
        const res = await fetch('/api/sessions', { headers });
        if (!res.ok) return;
        const sessions = await res.json();
        sessions.sort((a, b) => {
            const tA = new Date(a.createdAt || 0).getTime();
            const tB = new Date(b.createdAt || 0).getTime();
            if (tA !== tB) return tA - tB;
            return (a.id || '').localeCompare(b.id || '');
        });

        // Compute fingerprint to avoid destroying and recreating DOM every 3.5s when unchanged
        const snapshot = sessions.map(s => `${s.id}:${s.isRunning}:${s.role}:${s.name || ''}:${s.bubbleId || ''}`).join('|');
        allSessions = sessions;
        allSessions.forEach(s => {
            if (s.name && !sessionCustomNames[s.id]) {
                sessionCustomNames[s.id] = s.name;
            }
        });

        if (snapshot !== lastSessionsSnapshot) {
            lastSessionsSnapshot = snapshot;
            renderBubblesList();
            if (currentBubble) {
                updateBubbleSessionsUI();
            }
        } else if (currentBubble && activeBubbleSession && (!termWs || termWs.readyState === WebSocket.CLOSED)) {
            // Reconnect dropped terminal if server is healthy (deduplicated with ws.onclose, backed off,
            // skipped for exited sessions and never switches the active tab)
            scheduleTerminalReconnect(activeBubbleSession.id);
        }
    } catch (e) {
        console.error("Error al obtener sessions:", e);
    }
}



// Initial Load
window.addEventListener('DOMContentLoaded', async () => {
    applyTerminalFontSize();
    updateSortBarUI();
    initSplitResizer();
    initTerminalResizeObserver();
    initSysStatsDrag();
    await refreshAll();
    connectEventsChannel();
    loadPlugins();
    checkForUpdates(true);
    setInterval(fetchServerAndSessions, 3500);
    setInterval(fetchSystemStats, 5000);
    setInterval(() => checkForUpdates(true), 15 * 60 * 1000);
    fetchSystemStats();
    window.addEventListener('resize', handleWindowResize);
    window.addEventListener('keydown', (e) => {
        if (e.key === 'F12' || ((e.ctrlKey || e.metaKey) && (e.key === '`' || e.key === '\\'))) {
            e.preventDefault();
            toggleQuakeTerminal();
        } else if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'b') {
            e.preventDefault();
            toggleSidebar();
        } else if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
            e.preventDefault();
            openCommandPalette();
        } else if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'f') {
            if (activeTab === 'terminal') {
                e.preventDefault();
                openTerminalSearch();
            }
        } else if (e.key === 'Escape') {
            if (customDialogResolver) {
                handleCustomDialogCancel();
                return;
            }
            closePriorityPicker();
            if (isQuakeOpen) {
                toggleQuakeTerminal();
                return;
            }
            closeCommandPalette();
            closeTerminalSearch();
            closeSystemStatsModal();
            closeVpnWarningModal();
            closeUpdateModal();
            closeDoctorModal();
        }
    });
    window.addEventListener('click', (e) => {
        const ctxMenu = document.getElementById('subtab-context-menu');
        if (ctxMenu && ctxMenu.style.display !== 'none' && !ctxMenu.contains(e.target)) {
            ctxMenu.style.display = 'none';
        }
        const qaMenu = document.getElementById('quick-actions-menu-popup');
        const qaBtn = document.getElementById('quick-actions-dropdown');
        if (qaMenu && qaBtn && !qaBtn.contains(e.target)) {
            qaMenu.style.display = 'none';
        }
    });

    document.addEventListener('visibilitychange', () => {
        if (!document.hidden) {
            fetchServerAndSessions();
            fetchSystemStats();
        }
    });

    // Activity settle check for audio/desktop notifications
    setInterval(() => {
        if (hadRecentActiveStream && (Date.now() - lastTermActivityTime > 2500)) {
            hadRecentActiveStream = false;
            outputChunkCount = 0;
            if (document.hidden) {
                playNotificationChime();
                if ('Notification' in window && Notification.permission === 'granted') {
                    new Notification("Remote PC", { body: "Terminal completó la ejecución" });
                }
            }
        }
    }, 1000);
});

async function refreshAll() {
    await fetchServerInfo();
    await fetchBubbles();
    await fetchSessions();
}

async function fetchServerAndSessions() {
    if (document.hidden) return; // Do not poll when tab is hidden
    await fetchServerInfo();
    // Bubbles can be created or changed outside this page (MCP agents, other tabs, the Android app).
    // While the live channel is up those changes arrive as pushed hints, so the poll only re-checks
    // them every BUBBLES_FALLBACK_POLL_MS. Fetched before sessions so a new bubble's terminal is
    // listed under it in the same pass.
    if (!isEventsChannelLive() || Date.now() - lastBubblesFetchAt >= BUBBLES_FALLBACK_POLL_MS) {
        await fetchBubbles({ onlyIfChanged: true });
    }
    await fetchSessions();
}

// --- Live change hints (/ws/events) ---
// The daemon pushes {"type":"bubbles"} or {"type":"sessions"} whenever those lists change (creation,
// rename, deletion, exit, status/priority, dev server state; never terminal output). Hints carry no
// data: each one makes the page re-fetch that list, which only re-renders if something changed.
const BUBBLES_FALLBACK_POLL_MS = 15000;
const EVENTS_RECONNECT_MAX_MS = 10000;
let eventsWs = null;
let eventsReconnectTimer = null;
let eventsReconnectAttempts = 0;
const eventRefreshState = {
    bubbles: { running: false, again: false },
    sessions: { running: false, again: false },
};

function isEventsChannelLive() {
    return !!eventsWs && eventsWs.readyState === WebSocket.OPEN;
}

function connectEventsChannel() {
    if (eventsWs && (eventsWs.readyState === WebSocket.OPEN || eventsWs.readyState === WebSocket.CONNECTING)) return;
    clearTimeout(eventsReconnectTimer);
    eventsReconnectTimer = null;

    const token = serverInfo ? serverInfo.authToken : '';
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const ws = new WebSocket(`${protocol}//${window.location.host}/ws/events?token=${encodeURIComponent(token)}`);
    eventsWs = ws;

    ws.onopen = () => {
        eventsReconnectAttempts = 0;
        // Catch up on anything that changed while the channel was down (e.g. a daemon restart).
        refreshFromEvent('bubbles');
        refreshFromEvent('sessions');
    };
    ws.onmessage = (ev) => {
        let msg;
        try { msg = JSON.parse(ev.data); } catch (e) { return; }
        if (msg && (msg.type === 'bubbles' || msg.type === 'sessions')) refreshFromEvent(msg.type);
    };
    ws.onclose = () => {
        if (eventsWs !== ws) return;
        eventsWs = null;
        // Same backoff as terminals; the regular poll covers bubbles while the channel is down.
        const delay = Math.min(1000 * Math.pow(2, eventsReconnectAttempts), EVENTS_RECONNECT_MAX_MS);
        eventsReconnectAttempts++;
        eventsReconnectTimer = setTimeout(connectEventsChannel, delay);
    };
}

// Runs at most one refresh per kind at a time; hints that arrive meanwhile trigger one more pass.
async function refreshFromEvent(kind) {
    const st = eventRefreshState[kind];
    if (st.running) { st.again = true; return; }
    st.running = true;
    try {
        do {
            st.again = false;
            if (kind === 'bubbles') await fetchBubbles({ onlyIfChanged: true });
            else await fetchSessions();
        } while (st.again);
    } finally {
        st.running = false;
    }
}
