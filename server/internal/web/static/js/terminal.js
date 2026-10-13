// ==========================================
// Terminal Engine (XTerm, Split, Quake, Search)
// ==========================================

// Interactive Terminal Objects (Primary & Split)
let termObj = null;
let fitAddon = null;
let termWs = null;
let termObj2 = null;
let fitAddon2 = null;
let termWs2 = null;
let isSplitActive = false;
let splitDirection = 'horizontal';
let splitSecondarySession = null;
let isDraggingSplit = false;
let primaryRatio = 0.5;
let contextMenuSessionId = null;
let contextMenuSessionName = '';
let lastSentCols = 0;
let lastSentRows = 0;
let lastSentCols2 = 0;
let lastSentRows2 = 0;



// Terminal Search State
let searchAddon = null;
let searchAddon2 = null;
let isSearchCaseSensitive = false;



// Terminal Customization & Font Size State
let sessionCustomNames = {};
try {
    sessionCustomNames = JSON.parse(localStorage.getItem('term_custom_names') || '{}');
} catch(e) {
    sessionCustomNames = {};
}

let currentTermFontSize = parseInt(localStorage.getItem('term_font_size') || '14', 10);
if (isNaN(currentTermFontSize) || currentTermFontSize < 10) currentTermFontSize = 14;

function changeTerminalFontSize(delta) {
    let nextSize = currentTermFontSize + delta;
    if (nextSize < 10) nextSize = 10;
    if (nextSize > 26) nextSize = 26;
    currentTermFontSize = nextSize;
    try { localStorage.setItem('term_font_size', currentTermFontSize); } catch(e) {}
    applyTerminalFontSize();
}

function resetTerminalFontSize() {
    currentTermFontSize = 14;
    try { localStorage.setItem('term_font_size', currentTermFontSize); } catch(e) {}
    applyTerminalFontSize();
}

function applyTerminalFontSize() {
    document.querySelectorAll('.term-font-size-val, #term-font-size-display').forEach(el => {
        el.textContent = currentTermFontSize + 'px';
    });
    if (termObj) {
        termObj.options.fontSize = currentTermFontSize;
        fitActiveTerminal();
    }
    if (termObj2) {
        termObj2.options.fontSize = currentTermFontSize;
        fitSplitTerminal();
    }
    if (quakeTermObj) {
        quakeTermObj.options.fontSize = currentTermFontSize;
        try { if (quakeFitAddon) quakeFitAddon.fit(); } catch(e) {}
    }
}

async function promptRenameSession(sessId) {
    const sess = allSessions.find(s => s.id === sessId);
    if (!sess) return;
    const currentName = sessionCustomNames[sessId] || sess.name;
    const newName = await appPrompt(`Renombrar terminal "${currentName}" a:`, currentName, "Renombrar Terminal", "Guardar", "✏️");
    if (newName && newName.trim() && newName.trim() !== currentName) {
        const finalName = newName.trim();
        sessionCustomNames[sessId] = finalName;
        try {
            localStorage.setItem('term_custom_names', JSON.stringify(sessionCustomNames));
        } catch(e) {}

        // Sync with server persistent state
        const token = serverInfo ? serverInfo.authToken : '';
        fetch(`/api/sessions/${sessId}`, {
            method: 'PATCH',
            headers: {
                'Content-Type': 'application/json',
                ...(token ? { 'Authorization': 'Bearer ' + token } : {})
            },
            body: JSON.stringify({ name: finalName })
        }).catch(err => console.warn("Sync rename error:", err));

        updateBubbleSessionsUI();
        if (activeBubbleSession && activeBubbleSession.id === sessId) {
            const pTitle = document.getElementById('term-pane-primary-title');
            if (pTitle) pTitle.textContent = finalName;
        }
        if (splitSecondarySession && splitSecondarySession.id === sessId) {
            const sTitle = document.getElementById('term-pane-secondary-title');
            if (sTitle) sTitle.textContent = finalName;
        }
        showToast(`Terminal renombrada a "${finalName}"`);
    }
}

function triggerRenameFromCtx() {
    const menu = document.getElementById('subtab-context-menu');
    if (menu) menu.style.display = 'none';
    if (contextMenuSessionId) {
        promptRenameSession(contextMenuSessionId);
    }
}

function copyTextFallback(text) {
    const ta = document.createElement('textarea');
    ta.value = text;
    ta.style.position = 'fixed';
    ta.style.left = '-9999px';
    document.body.appendChild(ta);
    ta.focus();
    ta.select();
    try {
        document.execCommand('copy');
        showToast("Copiado al portapapeles");
    } catch(e) {}
    document.body.removeChild(ta);
}

function setupTerminalKeyHandlers(term, getWs) {
    if (!term) return;
    term.attachCustomKeyEventHandler((e) => {
        // Zoom shortcuts: Ctrl + =, Ctrl + -, Ctrl + 0
        if ((e.ctrlKey || e.metaKey) && (e.key === '=' || e.key === '+')) {
            if (e.type === 'keydown') changeTerminalFontSize(1);
            return false;
        }
        if ((e.ctrlKey || e.metaKey) && (e.key === '-' || e.key === '_')) {
            if (e.type === 'keydown') changeTerminalFontSize(-1);
            return false;
        }
        if ((e.ctrlKey || e.metaKey) && e.key === '0') {
            if (e.type === 'keydown') resetTerminalFontSize();
            return false;
        }

        // Ctrl+C / Cmd+C: Smart Copy or SIGINT
        if ((e.ctrlKey || e.metaKey) && !e.shiftKey && (e.key === 'c' || e.key === 'C')) {
            if (term.hasSelection()) {
                if (e.type === 'keydown') {
                    const sel = term.getSelection();
                    if (navigator.clipboard && navigator.clipboard.writeText) {
                        navigator.clipboard.writeText(sel).then(() => {
                            showToast("Copiado al portapapeles");
                        }).catch(() => copyTextFallback(sel));
                    } else {
                        copyTextFallback(sel);
                    }
                }
                return false; // Intercept: don't send SIGINT
            }
            // No selection: let xterm send \x03 SIGINT
            return true;
        }

        // Ctrl+Shift+C: Explicit copy
        if ((e.ctrlKey || e.metaKey) && e.shiftKey && (e.key === 'c' || e.key === 'C')) {
            if (e.type === 'keydown' && term.hasSelection()) {
                const sel = term.getSelection();
                if (navigator.clipboard && navigator.clipboard.writeText) {
                    navigator.clipboard.writeText(sel).then(() => {
                        showToast("Copiado al portapapeles");
                    }).catch(() => copyTextFallback(sel));
                } else {
                    copyTextFallback(sel);
                }
            }
            return false;
        }

        // Shift+Enter: Insert newline without submitting (Claude Code / Multiline Input)
        if (e.key === 'Enter' && e.shiftKey) {
            if (e.type === 'keydown') {
                const ws = typeof getWs === 'function' ? getWs() : getWs;
                if (ws && ws.readyState === WebSocket.OPEN) {
                    ws.send(JSON.stringify({ type: "input", data: "\x1b\r" }));
                }
            }
            return false;
        }

        // Ctrl+V / Ctrl+Shift+V: Clean Paste (prevents double pasting bug)
        if ((e.ctrlKey || e.metaKey) && (e.key === 'v' || e.key === 'V')) {
            // Returning false prevents xterm from translating to raw \x16 (SYN).
            // The browser fires native paste on xterm's helper textarea, which
            // xterm handles cleanly and emits once via term.onData.
            return false;
        }

        return true;
    });
}

// Terminal Management for Active Bubble
let bubbleLastSessionMap = {};
let currentConnectedSessionId = null;

async function updateBubbleSessionsUI() {
    if (!currentBubble) return;
    const container = document.getElementById('term-subtabs-container');
    if (!container) return;

    const bubbleSessions = allSessions.filter(s => s.bubbleId === currentBubble.id || (s.cwd && s.cwd.toLowerCase() === currentBubble.path.toLowerCase()));

    if (bubbleSessions.length === 0) {
        activeBubbleSession = null;
        currentConnectedSessionId = null;

        // Close split if active
        if (isSplitActive) {
            closeSplit();
        }

        // Disconnect and clean up primary terminal
        if (termWs) {
            termWs.onopen = null;
            termWs.onclose = null;
            termWs.onerror = null;
            termWs.onmessage = null;
            termWs.close();
            termWs = null;
        }
        if (fitAddon) {
            try { fitAddon.dispose(); } catch (e) {}
            fitAddon = null;
        }
        if (searchAddon) {
            try { searchAddon.dispose(); } catch (e) {}
            searchAddon = null;
        }
        if (termObj) {
            try { termObj.dispose(); } catch (e) {}
            termObj = null;
        }

        container.innerHTML = `
            <span style="font-size: 11px; color: var(--text-dim); padding: 4px 6px;">
                Sin sesiones activas en este bubble.
            </span>
        `;

        const pTitle = document.getElementById('term-pane-primary-title');
        if (pTitle) pTitle.textContent = "Sin Terminal";

        const termEmbed = document.getElementById('terminal-embed');
        if (termEmbed) {
            termEmbed.innerHTML = `
                <div style="display: flex; flex-direction: column; align-items: center; justify-content: center; height: 100%; color: var(--text-dim); text-align: center; padding: 24px;">
                    <div style="font-size: 38px; margin-bottom: 12px; opacity: 0.5;">🐚</div>
                    <div style="font-size: 14px; font-weight: 600; color: var(--text-main); margin-bottom: 6px;">
                        No hay terminales activas en "${escapeHtml(currentBubble.name)}"
                    </div>
                    <div style="font-size: 12px; color: var(--text-dim); margin-bottom: 16px; max-width: 360px;">
                        Inicia Claude Code o una terminal Shell para trabajar de inmediato en esta tarea.
                    </div>
                    <div style="display: flex; gap: 8px;">
                        <button class="btn btn-primary btn-sm" onclick="spawnClaudeInBubble()">
                            🤖 + Claude Code
                        </button>
                        <button class="btn btn-sm" style="background: linear-gradient(135deg, #7c3aed, #6d28d9); border-color: #a78bfa; color: white;" onclick="spawnAgyInBubble()">
                            🚀 + Antigravity CLI
                        </button>
                        <button class="btn btn-sm" onclick="spawnShellInBubble()">
                            🐚 + Shell
                        </button>
                    </div>
                </div>
            `;
        }
        return;
    }

    // Bubble HAS active sessions:
    const belongsToBubble = activeBubbleSession && bubbleSessions.some(s => s.id === activeBubbleSession.id);
    if (!belongsToBubble) {
        const rememberedId = bubbleLastSessionMap[currentBubble.id];
        const targetSess = bubbleSessions.find(s => s.id === rememberedId) || bubbleSessions[0];
        activeBubbleSession = targetSess;
        connectToSession(activeBubbleSession.id);
    } else {
        if (currentConnectedSessionId !== activeBubbleSession.id || !termObj || !termWs || termWs.readyState !== WebSocket.OPEN) {
            connectToSession(activeBubbleSession.id);
        }
    }

    // Count duplicates of displayName in the same bubble
    const nameFrequency = {};
    bubbleSessions.forEach(s => {
        const name = sessionCustomNames[s.id] || s.name;
        nameFrequency[name] = (nameFrequency[name] || 0) + 1;
    });
    const nameOccurrence = {};

    container.innerHTML = bubbleSessions.map((s, idx) => {
        const isSel = activeBubbleSession && activeBubbleSession.id === s.id;
        const isSplitSel = isSplitActive && splitSecondarySession && splitSecondarySession.id === s.id;
        const roleIcon = s.role === 'claude' ? '🤖' : (s.role === 'agy' ? '🚀' : (s.role === 'server' ? '⚙️' : '🐚'));
        let displayName = sessionCustomNames[s.id] || s.name;
        if (nameFrequency[displayName] > 1 && !sessionCustomNames[s.id]) {
            nameOccurrence[displayName] = (nameOccurrence[displayName] || 0) + 1;
            displayName = `${displayName} #${nameOccurrence[displayName]}`;
        }
        return `
            <button class="term-subtab-btn ${isSel ? 'active' : ''} ${isSplitSel ? 'split-active' : ''}" 
                    onclick="switchTerminalSession('${s.id}')"
                    ondblclick="promptRenameSession('${s.id}')"
                    oncontextmenu="handleSubtabContextMenu(event, '${s.id}', '${escapeHtml(displayName)}')"
                    title="Terminal #${idx + 1} · Click para ver · Doble clic para renombrar · Click derecho para opciones">
                <span style="font-size: 10px; color: ${isSel ? 'var(--accent)' : 'var(--text-dim)'}; font-family: monospace; font-weight: 700; margin-right: 2px;">#${idx + 1}</span>
                ${isSel ? '<span class="term-active-pill" title="Terminal activa">●</span>' : ''}
                <span>${roleIcon}</span>
                <span class="term-subtab-title">${escapeHtml(displayName)}</span>
                <span style="font-size: 10px; color: ${s.isRunning ? 'var(--green)' : 'var(--red)'};">●</span>
                <span class="term-subtab-rename-btn" onclick="event.stopPropagation(); promptRenameSession('${s.id}')" title="Renombrar terminal">✏️</span>
                <span class="term-subtab-close" onclick="closeTerminalSession(event, '${s.id}', '${escapeHtml(displayName)}')" title="Cerrar terminal">✕</span>
            </button>
        `;
    }).join('');
}

function switchTerminalSession(sessId) {
    const sess = allSessions.find(s => s.id === sessId);
    if (!sess) return;
    activeBubbleSession = sess;
    if (currentBubble) {
        bubbleLastSessionMap[currentBubble.id] = sessId;
    }
    connectToSession(sessId);
    updateBubbleSessionsUI();
    if (isSplitActive) updateSplitSelectorOptions();
}

function connectToSession(sessionId) {
    const container = document.getElementById('terminal-embed');
    if (!container) return;

    // Ensure terminal tab is active and visible
    if (activeTab !== 'terminal') {
        switchWorkspaceTab('terminal');
    }

    // Don't reconnect if already connected to this session with active terminal
    if (currentConnectedSessionId === sessionId && termObj && termWs && termWs.readyState === WebSocket.OPEN) {
        return;
    }
    currentConnectedSessionId = sessionId;
    lastSentCols = 0;
    lastSentRows = 0;

    if (termWs) {
        termWs.onopen = null;
        termWs.onclose = null;
        termWs.onerror = null;
        termWs.onmessage = null;
        termWs.close();
        termWs = null;
    }
    if (fitAddon) {
        try { fitAddon.dispose(); } catch (e) {}
        fitAddon = null;
    }
    if (searchAddon) {
        try { searchAddon.dispose(); } catch (e) {}
        searchAddon = null;
    }
    if (termObj) {
        try { termObj.dispose(); } catch (e) {}
        termObj = null;
    }

    container.innerHTML = '';

    const pTitle = document.getElementById('term-pane-primary-title');
    if (pTitle && activeBubbleSession) {
        pTitle.textContent = sessionCustomNames[activeBubbleSession.id] || activeBubbleSession.name;
    }

    termObj = new Terminal({
        theme: {
            background: '#0d1117',
            foreground: '#c9d1d9',
            cursor: '#38bdf8'
        },
        fontFamily: "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace",
        fontSize: currentTermFontSize,
        cursorBlink: true,
        convertEol: false
    });

    if (typeof FitAddon !== 'undefined' && FitAddon.FitAddon) {
        fitAddon = new FitAddon.FitAddon();
        termObj.loadAddon(fitAddon);
    }
    if (typeof SearchAddon !== 'undefined' && SearchAddon.SearchAddon) {
        searchAddon = new SearchAddon.SearchAddon();
        termObj.loadAddon(searchAddon);
    }

    termObj.open(container);
    setupTerminalKeyHandlers(termObj, () => termWs);
    requestAnimationFrame(() => {
        setTimeout(fitActiveTerminal, 60);
        setTimeout(fitActiveTerminal, 250);
    });

    termObj.onData(data => {
        if (termWs && termWs.readyState === WebSocket.OPEN) {
            termWs.send(JSON.stringify({ type: "input", data }));
        }
    });

    const token = serverInfo ? serverInfo.authToken : '';
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws/session/${sessionId}?token=${encodeURIComponent(token)}`;

    termWs = new WebSocket(wsUrl);
    termWs.binaryType = 'arraybuffer';

    termWs.onopen = () => {
        fitActiveTerminal();
    };

    termWs.onmessage = (e) => {
        if (!termObj) return;
        outputChunkCount++;
        if (outputChunkCount > 3) hadRecentActiveStream = true;
        lastTermActivityTime = Date.now();
        try {
            if (e.data instanceof ArrayBuffer) {
                termObj.write(new Uint8Array(e.data));
            } else if (e.data instanceof Blob) {
                e.data.arrayBuffer().then(buf => {
                    if (termObj) termObj.write(new Uint8Array(buf));
                });
            } else {
                termObj.write(e.data);
            }
        } catch (err) {
            console.warn("xterm write ignored:", err);
        }
    };

    termWs.onclose = () => {
        if (termObj) {
            try { termObj.writeln("\r\n\x1b[33m[Sesión cerrada]\x1b[0m"); } catch (e) {}
        }
    };
}

function fitActiveTerminal() {
    const container = document.getElementById('terminal-embed');
    if (!container || container.offsetWidth <= 0 || container.offsetHeight <= 0) {
        return;
    }
    if (fitAddon && termObj && termWs && termWs.readyState === WebSocket.OPEN) {
        try {
            fitAddon.fit();
            if (termObj.cols > 0 && termObj.rows > 0) {
                if (termObj.cols !== lastSentCols || termObj.rows !== lastSentRows) {
                    lastSentCols = termObj.cols;
                    lastSentRows = termObj.rows;
                    termWs.send(JSON.stringify({ type: "resize", cols: termObj.cols, rows: termObj.rows }));
                }
            }
        } catch (e) {}
    }
}

// Split Terminal Engine
function handleSubtabContextMenu(e, sessId, name) {
    e.preventDefault();
    contextMenuSessionId = sessId;
    contextMenuSessionName = name;
    const menu = document.getElementById('subtab-context-menu');
    document.getElementById('ctx-menu-title').textContent = name;
    document.getElementById('ctx-btn-close-split').style.display = isSplitActive ? 'inline-flex' : 'none';

    menu.style.display = 'flex';
    let x = e.clientX;
    let y = e.clientY;
    const rect = menu.getBoundingClientRect();
    if (x + rect.width > window.innerWidth) x = window.innerWidth - rect.width - 10;
    if (y + rect.height > window.innerHeight) y = window.innerHeight - rect.height - 10;
    menu.style.left = x + 'px';
    menu.style.top = y + 'px';
}

async function closeTerminalSession(e, sessId, name) {
    if (e) e.stopPropagation();
    const ok = await appConfirm(`¿Cerrar y terminar la terminal "${name}"?`, "Cerrar Terminal", "Cerrar", "Cancelar", true, "💻");
    if (!ok) return;
    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = token ? { 'Authorization': 'Bearer ' + token } : {};
        await fetch(`/api/sessions/${sessId}`, { method: 'DELETE', headers });
        showToast(`Terminal "${name}" cerrada.`);
        if (activeBubbleSession && activeBubbleSession.id === sessId) {
            activeBubbleSession = null;
        }
        if (splitSecondarySession && splitSecondarySession.id === sessId) {
            closeSplit();
        }
        await fetchSessions();
    } catch (err) {
        appAlert("Error al cerrar terminal: " + err, "Error", "❌");
    }
}

function triggerSplit(dir) {
    const ctxMenu = document.getElementById('subtab-context-menu');
    if (ctxMenu) ctxMenu.style.display = 'none';
    if (!currentBubble) return;

    const bubbleSessions = allSessions.filter(s => s.bubbleId === currentBubble.id || (s.cwd && s.cwd.toLowerCase() === currentBubble.path.toLowerCase()));

    // Find secondary session
    let targetSess = null;
    if (contextMenuSessionId && contextMenuSessionId !== activeBubbleSession?.id) {
        targetSess = bubbleSessions.find(s => s.id === contextMenuSessionId);
    }
    if (!targetSess) {
        targetSess = bubbleSessions.find(s => s.id !== activeBubbleSession?.id);
    }

    if (!targetSess) {
        showToast("Iniciando una nueva terminal Shell para dividir...");
        spawnShellInBubble().then(() => {
            setTimeout(() => {
                const updated = allSessions.filter(s => s.bubbleId === currentBubble.id || (s.cwd && s.cwd.toLowerCase() === currentBubble.path.toLowerCase()));
                const newS = updated.find(s => s.id !== activeBubbleSession?.id);
                if (newS) applySplitLayout(dir, newS);
            }, 800);
        });
        return;
    }

    applySplitLayout(dir, targetSess);
}

function initSplitResizer() {
    const resizer = document.getElementById('term-split-resizer');
    const wrapper = document.getElementById('terminal-embed-wrapper');
    if (!resizer || !wrapper) return;

    const onStartDrag = (e) => {
        if (!isSplitActive) return;
        isDraggingSplit = true;
        resizer.classList.add('dragging');
        document.body.style.cursor = (splitDirection === 'horizontal') ? 'col-resize' : 'row-resize';
        document.body.style.userSelect = 'none';
        document.querySelectorAll('.xterm').forEach(el => el.style.pointerEvents = 'none');
    };

    resizer.addEventListener('mousedown', (e) => {
        e.preventDefault();
        onStartDrag(e);
    });

    resizer.addEventListener('touchstart', (e) => {
        onStartDrag(e);
    }, { passive: true });

    resizer.addEventListener('dblclick', (e) => {
        if (!isSplitActive) return;
        e.preventDefault();
        primaryRatio = 0.5;
        updateSplitDimensions();
        showToast("Split restablecido al 50% / 50%");
    });

    const onMoveDrag = (clientX, clientY) => {
        if (!isDraggingSplit || !isSplitActive) return;
        const rect = wrapper.getBoundingClientRect();
        let ratio;
        if (splitDirection === 'horizontal') {
            const offset = clientX - rect.left;
            ratio = offset / rect.width;
        } else {
            const offset = clientY - rect.top;
            ratio = offset / rect.height;
        }

        if (ratio < 0.15) ratio = 0.15;
        if (ratio > 0.85) ratio = 0.85;

        primaryRatio = ratio;
        updateSplitDimensions();
    };

    window.addEventListener('mousemove', (e) => {
        onMoveDrag(e.clientX, e.clientY);
    });

    window.addEventListener('touchmove', (e) => {
        if (e.touches && e.touches.length > 0) {
            onMoveDrag(e.touches[0].clientX, e.touches[0].clientY);
        }
    }, { passive: true });

    const onEndDrag = () => {
        if (isDraggingSplit) {
            isDraggingSplit = false;
            resizer.classList.remove('dragging');
            document.body.style.cursor = '';
            document.body.style.userSelect = '';
            document.querySelectorAll('.xterm').forEach(el => el.style.pointerEvents = '');
            fitActiveTerminal();
            fitSplitTerminal();
        }
    };

    window.addEventListener('mouseup', onEndDrag);
    window.addEventListener('touchend', onEndDrag);
}

function updateSplitDimensions() {
    if (!isSplitActive) return;
    const primaryPane = document.getElementById('term-pane-primary');
    const secondaryPane = document.getElementById('term-pane-secondary');
    if (!primaryPane || !secondaryPane) return;

    const pct1 = (primaryRatio * 100).toFixed(2);
    const pct2 = ((1 - primaryRatio) * 100).toFixed(2);

    if (splitDirection === 'horizontal') {
        primaryPane.style.flex = `0 0 calc(${pct1}% - 3px)`;
        primaryPane.style.width = `calc(${pct1}% - 3px)`;
        primaryPane.style.height = '100%';
        secondaryPane.style.flex = `0 0 calc(${pct2}% - 3px)`;
        secondaryPane.style.width = `calc(${pct2}% - 3px)`;
        secondaryPane.style.height = '100%';
    } else {
        primaryPane.style.flex = `0 0 calc(${pct1}% - 3px)`;
        primaryPane.style.height = `calc(${pct1}% - 3px)`;
        primaryPane.style.width = '100%';
        secondaryPane.style.flex = `0 0 calc(${pct2}% - 3px)`;
        secondaryPane.style.height = `calc(${pct2}% - 3px)`;
        secondaryPane.style.width = '100%';
    }

    fitActiveTerminal();
    fitSplitTerminal();
}

function applySplitLayout(dir, targetSess) {
    isSplitActive = true;
    splitDirection = dir;
    splitSecondarySession = targetSess;

    const wrapper = document.getElementById('terminal-embed-wrapper');
    const primaryHeader = document.getElementById('term-pane-primary-header');
    const secondaryPane = document.getElementById('term-pane-secondary');
    const primaryTitle = document.getElementById('term-pane-primary-title');
    const secondaryTitle = document.getElementById('term-pane-secondary-title');
    const resizer = document.getElementById('term-split-resizer');

    primaryHeader.style.display = 'flex';
    primaryTitle.textContent = activeBubbleSession ? activeBubbleSession.name : 'Terminal 1';

    secondaryPane.style.display = 'flex';
    secondaryTitle.textContent = targetSess.name;

    if (resizer) {
        resizer.style.display = 'block';
        resizer.className = `split-resizer ${dir}`;
    }

    if (dir === 'horizontal') {
        wrapper.style.flexDirection = 'row';
        secondaryPane.style.borderLeft = 'none';
        secondaryPane.style.borderTop = 'none';
    } else {
        wrapper.style.flexDirection = 'column';
        secondaryPane.style.borderTop = 'none';
        secondaryPane.style.borderLeft = 'none';
    }

    updateSplitDimensions();
    updateSplitSelectorOptions();
    connectToSplitSession(targetSess.id);

    setTimeout(() => {
        fitActiveTerminal();
        fitSplitTerminal();
    }, 150);

    updateBubbleSessionsUI();
    showToast(`Panel dividido ${dir === 'horizontal' ? 'horizontalmente' : 'verticalmente'}.`);
}

let currentSplitConnectedSessionId = null;

function connectToSplitSession(sessionId) {
    const container = document.getElementById('terminal-embed-2');
    if (!container) return;

    if (currentSplitConnectedSessionId === sessionId && termObj2 && termWs2 && termWs2.readyState === WebSocket.OPEN) {
        return;
    }
    currentSplitConnectedSessionId = sessionId;
    lastSentCols2 = 0;
    lastSentRows2 = 0;

    if (termWs2) {
        termWs2.onopen = null;
        termWs2.onclose = null;
        termWs2.onerror = null;
        termWs2.onmessage = null;
        termWs2.close();
        termWs2 = null;
    }
    if (fitAddon2) {
        try { fitAddon2.dispose(); } catch (e) {}
        fitAddon2 = null;
    }
    if (searchAddon2) {
        try { searchAddon2.dispose(); } catch (e) {}
        searchAddon2 = null;
    }
    if (termObj2) {
        try { termObj2.dispose(); } catch (e) {}
        termObj2 = null;
    }

    container.innerHTML = '';

    termObj2 = new Terminal({
        theme: {
            background: '#0d1117',
            foreground: '#c9d1d9',
            cursor: '#10b981'
        },
        fontFamily: "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace",
        fontSize: currentTermFontSize,
        cursorBlink: true,
        convertEol: false
    });

    if (typeof FitAddon !== 'undefined' && FitAddon.FitAddon) {
        fitAddon2 = new FitAddon.FitAddon();
        termObj2.loadAddon(fitAddon2);
    }
    if (typeof SearchAddon !== 'undefined' && SearchAddon.SearchAddon) {
        searchAddon2 = new SearchAddon.SearchAddon();
        termObj2.loadAddon(searchAddon2);
    }

    termObj2.open(container);
    setupTerminalKeyHandlers(termObj2, () => termWs2);
    requestAnimationFrame(() => {
        setTimeout(fitSplitTerminal, 60);
        setTimeout(fitSplitTerminal, 250);
    });

    termObj2.onData(data => {
        if (termWs2 && termWs2.readyState === WebSocket.OPEN) {
            termWs2.send(JSON.stringify({ type: "input", data }));
        }
    });

    const token = serverInfo ? serverInfo.authToken : '';
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws/session/${sessionId}?token=${encodeURIComponent(token)}`;

    termWs2 = new WebSocket(wsUrl);
    termWs2.binaryType = 'arraybuffer';

    termWs2.onopen = () => {
        fitSplitTerminal();
    };

    termWs2.onmessage = (e) => {
        if (!termObj2) return;
        try {
            if (e.data instanceof ArrayBuffer) {
                termObj2.write(new Uint8Array(e.data));
            } else if (e.data instanceof Blob) {
                e.data.arrayBuffer().then(buf => {
                    if (termObj2) termObj2.write(new Uint8Array(buf));
                });
            } else {
                termObj2.write(e.data);
            }
        } catch (err) {
            console.warn("split xterm write ignored:", err);
        }
    };

    termWs2.onclose = () => {
        if (termObj2) {
            try { termObj2.writeln("\r\n\x1b[33m[Sesión dividida cerrada]\x1b[0m"); } catch (e) {}
        }
    };
}

function fitSplitTerminal() {
    const container = document.getElementById('terminal-embed-2');
    if (!container || container.offsetWidth <= 0 || container.offsetHeight <= 0) {
        return;
    }
    if (fitAddon2 && termObj2 && termWs2 && termWs2.readyState === WebSocket.OPEN) {
        try {
            fitAddon2.fit();
            if (termObj2.cols > 0 && termObj2.rows > 0) {
                if (termObj2.cols !== lastSentCols2 || termObj2.rows !== lastSentRows2) {
                    lastSentCols2 = termObj2.cols;
                    lastSentRows2 = termObj2.rows;
                    termWs2.send(JSON.stringify({ type: "resize", cols: termObj2.cols, rows: termObj2.rows }));
                }
            }
        } catch (e) {}
    }
}

function closeSplit() {
    isSplitActive = false;
    splitSecondarySession = null;

    const ctxMenu = document.getElementById('subtab-context-menu');
    if (ctxMenu) ctxMenu.style.display = 'none';

    if (termWs2) {
        termWs2.close();
        termWs2 = null;
    }
    if (termObj2) {
        termObj2.dispose();
        termObj2 = null;
    }

    const wrapper = document.getElementById('terminal-embed-wrapper');
    const primaryPane = document.getElementById('term-pane-primary');
    const primaryHeader = document.getElementById('term-pane-primary-header');
    const secondaryPane = document.getElementById('term-pane-secondary');
    const resizer = document.getElementById('term-split-resizer');

    if (resizer) resizer.style.display = 'none';
    if (primaryHeader) primaryHeader.style.display = 'none';
    if (secondaryPane) {
        secondaryPane.style.display = 'none';
        secondaryPane.style.flex = '1';
        secondaryPane.style.width = '';
        secondaryPane.style.height = '';
    }
    if (primaryPane) {
        primaryPane.style.flex = '1';
        primaryPane.style.width = '100%';
        primaryPane.style.height = '100%';
    }
    if (wrapper) wrapper.style.flexDirection = 'row';

    setTimeout(() => {
        fitActiveTerminal();
    }, 100);

    updateBubbleSessionsUI();
    showToast("Panel dividido cerrado.");
}

function updateSplitSelectorOptions() {
    const sel = document.getElementById('term-split-session-select');
    if (!sel || !currentBubble) return;
    const bubbleSessions = allSessions.filter(s => s.bubbleId === currentBubble.id || (s.cwd && s.cwd.toLowerCase() === currentBubble.path.toLowerCase()));

    sel.innerHTML = bubbleSessions.map(s => {
        const isCurrent = splitSecondarySession && splitSecondarySession.id === s.id;
        return `<option value="${s.id}" ${isCurrent ? 'selected' : ''}>${escapeHtml(s.name)}</option>`;
    }).join('');
}

function onSplitSessionChange(sessId) {
    const sess = allSessions.find(s => s.id === sessId);
    if (!sess) return;
    splitSecondarySession = sess;
    document.getElementById('term-pane-secondary-title').textContent = sess.name;
    connectToSplitSession(sessId);
    updateBubbleSessionsUI();
}

function triggerClearBuffer() {
    const menu = document.getElementById('subtab-context-menu');
    if (menu) menu.style.display = 'none';
    if (termObj) termObj.clear();
    if (termObj2) termObj2.clear();
    showToast("Buffer de pantalla limpiado.");
}

function triggerCopySessId() {
    const menu = document.getElementById('subtab-context-menu');
    if (menu) menu.style.display = 'none';
    if (contextMenuSessionId) {
        navigator.clipboard.writeText(contextMenuSessionId);
        showToast("ID de sesión copiado al portapapeles.");
    }
}

async function triggerKillFromCtx() {
    const menu = document.getElementById('subtab-context-menu');
    if (menu) menu.style.display = 'none';
    if (contextMenuSessionId) {
        await closeTerminalSession(null, contextMenuSessionId, contextMenuSessionName);
    }
}



function clearActiveTerminal() {
    if (termObj) termObj.clear();
    sendTerminalKey('\x0c');
}

function sendTerminalKey(keyStr) {
    if (termWs && termWs.readyState === WebSocket.OPEN) {
        termWs.send(JSON.stringify({ type: "input", data: keyStr }));
    }
}

function handleMobileInputKey(e) {
    if (e.key === 'Enter') submitMobileInput();
}

function submitMobileInput() {
    const input = document.getElementById('term-mobile-input');
    if (!input || !termWs || termWs.readyState !== WebSocket.OPEN) return;
    const text = input.value;
    termWs.send(JSON.stringify({ type: "input", data: (text ? text + "\r" : "\r") }));
    input.value = '';
}

// Celery Workers for Brif


// ==========================================
// QoL 1: Terminal In-Buffer Search (Ctrl+F)
// ==========================================
function openTerminalSearch() {
    const bar = document.getElementById('term-search-bar');
    if (!bar) return;
    bar.style.display = 'flex';
    const input = document.getElementById('term-search-input');
    if (input) {
        input.focus();
        input.select();
    }
}

function closeTerminalSearch() {
    const bar = document.getElementById('term-search-bar');
    if (bar) bar.style.display = 'none';
    if (termObj) termObj.focus();
}

function getActiveSearchAddon() {
    return searchAddon;
}

function handleSearchInput(query) {
    const addon = getActiveSearchAddon();
    if (!addon || !query) return;
    addon.findNext(query, { caseSensitive: isSearchCaseSensitive, incremental: true });
}

function findSearchNext() {
    const input = document.getElementById('term-search-input');
    const addon = getActiveSearchAddon();
    if (!addon || !input || !input.value) return;
    addon.findNext(input.value, { caseSensitive: isSearchCaseSensitive, incremental: false });
}

function findSearchPrev() {
    const input = document.getElementById('term-search-input');
    const addon = getActiveSearchAddon();
    if (!addon || !input || !input.value) return;
    addon.findPrevious(input.value, { caseSensitive: isSearchCaseSensitive });
}

function toggleSearchCase() {
    isSearchCaseSensitive = !isSearchCaseSensitive;
    const btn = document.getElementById('btn-search-case');
    if (btn) {
        btn.style.color = isSearchCaseSensitive ? 'var(--accent)' : 'var(--text-muted)';
        btn.style.borderColor = isSearchCaseSensitive ? 'var(--accent)' : 'var(--border-subtle)';
    }
    findSearchNext();
}

function handleSearchKeyDown(e) {
    if (e.key === 'Enter') {
        e.preventDefault();
        if (e.shiftKey) findSearchPrev();
        else findSearchNext();
    } else if (e.key === 'Escape') {
        closeTerminalSearch();
    }
}



// ==========================================
// Quake Terminal Dropdown (F12 / Ctrl+`)
// ==========================================
let quakeSessionId = null;
let quakeTermObj = null;
let quakeFitAddon = null;
let quakeTermWs = null;
let isQuakeOpen = false;

function fitQuakeTerminal() {
    if (!quakeFitAddon || !quakeTermObj) return;
    try {
        quakeFitAddon.fit();
        if (quakeTermWs && quakeTermWs.readyState === WebSocket.OPEN) {
            quakeTermWs.send(JSON.stringify({
                type: 'resize',
                cols: quakeTermObj.cols,
                rows: quakeTermObj.rows
            }));
        }
    } catch (e) {}
}

async function toggleQuakeTerminal() {
    const container = document.getElementById('quake-terminal-container');
    if (!container) return;

    isQuakeOpen = !isQuakeOpen;
    if (isQuakeOpen) {
        container.classList.add('open');
        if (!quakeSessionId || !quakeTermWs || quakeTermWs.readyState !== WebSocket.OPEN) {
            await initQuakeTerminal();
        } else {
            setTimeout(() => {
                fitQuakeTerminal();
                if (quakeTermObj) quakeTermObj.focus();
            }, 120);
        }
    } else {
        container.classList.remove('open');
        if (activeTab === 'terminal' && termObj) {
            termObj.focus();
        }
    }
}

async function initQuakeTerminal() {
    const embed = document.getElementById('quake-terminal-embed');
    if (!embed) return;

    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': 'Bearer ' + token } : {})
        };

        // Find or create quake session (must be running)
        let sess = allSessions.find(s => s.name === '⚡ Quick Terminal' && s.isRunning);
        if (!sess) {
            const cwd = (currentBubble && currentBubble.path) ? currentBubble.path : '';
            const res = await fetch('/api/sessions', {
                method: 'POST',
                headers,
                body: JSON.stringify({
                    name: '⚡ Quick Terminal',
                    command: 'powershell.exe',
                    cwd: cwd,
                    cols: 120,
                    rows: 25
                })
            });
            if (res.ok) {
                sess = await res.json();
                await fetchSessions();
            }
        }

        if (!sess) return;
        quakeSessionId = sess.id;

        // Cleanup existing WS / Term if any
        if (quakeTermWs) {
            quakeTermWs.onopen = null;
            quakeTermWs.onclose = null;
            quakeTermWs.onmessage = null;
            quakeTermWs.close();
            quakeTermWs = null;
        }
        if (quakeTermObj) {
            try { quakeTermObj.dispose(); } catch (e) {}
            quakeTermObj = null;
        }

        embed.innerHTML = '';
        quakeTermObj = new Terminal({
            theme: {
                background: '#0a0e17',
                foreground: '#e2e8f0',
                cursor: '#38bdf8'
            },
            fontFamily: "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace",
            fontSize: currentTermFontSize,
            cursorBlink: true,
            convertEol: false
        });

        if (typeof FitAddon !== 'undefined' && FitAddon.FitAddon) {
            quakeFitAddon = new FitAddon.FitAddon();
            quakeTermObj.loadAddon(quakeFitAddon);
        }

        quakeTermObj.open(embed);
        setupTerminalKeyHandlers(quakeTermObj, () => quakeTermWs);

        quakeTermObj.onData(data => {
            if (quakeTermWs && quakeTermWs.readyState === WebSocket.OPEN) {
                quakeTermWs.send(JSON.stringify({ type: "input", data }));
            }
        });

        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws/session/${quakeSessionId}?token=${encodeURIComponent(token)}`;

        quakeTermWs = new WebSocket(wsUrl);
        quakeTermWs.binaryType = 'arraybuffer';

        quakeTermWs.onopen = () => {
            setTimeout(fitQuakeTerminal, 80);
            setTimeout(fitQuakeTerminal, 250);
        };

        quakeTermWs.onmessage = (e) => {
            if (!quakeTermObj) return;
            try {
                if (e.data instanceof ArrayBuffer) {
                    quakeTermObj.write(new Uint8Array(e.data));
                } else if (e.data instanceof Blob) {
                    e.data.arrayBuffer().then(buf => {
                        if (quakeTermObj) quakeTermObj.write(new Uint8Array(buf));
                    });
                } else {
                    quakeTermObj.write(e.data);
                }
            } catch (err) {
                console.warn("quake xterm write error:", err);
            }
        };

        quakeTermWs.onclose = () => {
            quakeSessionId = null;
        };

        setTimeout(() => {
            fitQuakeTerminal();
            if (quakeTermObj) quakeTermObj.focus();
        }, 150);

    } catch (err) {
        console.error("Failed to init Quake terminal:", err);
    }
}

function clearQuakeTerminal() {
    if (quakeTermObj) quakeTermObj.clear();
}
