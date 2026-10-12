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
    }, 230);
}

function handleWindowResize() {
    if (isSplitActive) {
        updateSplitDimensions();
    } else if (activeTab === 'terminal') {
        fitActiveTerminal();
    }

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



async function fetchSessions() {
    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = token ? { 'Authorization': 'Bearer ' + token } : {};
        const res = await fetch('/api/sessions', { headers });
        if (!res.ok) return;
        allSessions = await res.json();
        allSessions.sort((a, b) => {
            const tA = new Date(a.createdAt || 0).getTime();
            const tB = new Date(b.createdAt || 0).getTime();
            if (tA !== tB) return tA - tB;
            return (a.id || '').localeCompare(b.id || '');
        });
        allSessions.forEach(s => {
            if (s.name && !sessionCustomNames[s.id]) {
                sessionCustomNames[s.id] = s.name;
            }
        });
        renderBubblesList();
        if (currentBubble) {
            updateBubbleSessionsUI();
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
    initSysStatsDrag();
    await refreshAll();
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
    await fetchServerInfo();
    await fetchSessions();
}
