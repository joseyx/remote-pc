// ==========================================
// Process & Tool Launchers (Claude, AGY, Celery, Dev Server)
// ==========================================

function toggleCeleryMenu() {
    const popup = document.getElementById('celery-menu-popup');
    if (!popup) return;
    popup.style.display = (popup.style.display === 'flex') ? 'none' : 'flex';
}

// Close celery popup when clicking outside
window.addEventListener('click', (e) => {
    const dropdown = document.getElementById('brif-celery-dropdown');
    const popup = document.getElementById('celery-menu-popup');
    if (popup && dropdown && !dropdown.contains(e.target)) {
        popup.style.display = 'none';
    }
});

async function spawnCeleryWorker(type) {
    const popup = document.getElementById('celery-menu-popup');
    if (popup) popup.style.display = 'none';
    if (!currentBubble) return;

    let cmd = '';
    let name = '';
    const venvCelery = '.\\venv\\Scripts\\celery.exe';

    switch (type) {
        case 'orchestrator':
            name = 'Celery (Orchestrator)';
            cmd = `powershell.exe -NoExit -Command "& '${venvCelery}' -A config worker --loglevel=info -E --pool=gevent -c 15 -n orchestrator@$env:COMPUTERNAME -Q high_priority,celery"`;
            break;
        case 'articles':
            name = 'Celery (Articles)';
            cmd = `powershell.exe -NoExit -Command "& '${venvCelery}' -A config worker --loglevel=info -E --pool=gevent -c 100 -n articles@$env:COMPUTERNAME -Q articles"`;
            break;
        case 'fetcher':
            name = 'Celery (Fetcher)';
            cmd = `powershell.exe -NoExit -Command "& '${venvCelery}' -A config worker --loglevel=info -E --pool=gevent -c 60 -n fetcher@$env:COMPUTERNAME -Q fetch"`;
            break;
        case 'beat':
            name = 'Celery (Beat)';
            cmd = `powershell.exe -NoExit -Command "& '${venvCelery}' -A config beat -l info --scheduler django_celery_beat.schedulers:DatabaseScheduler"`;
            break;
        case 'flower':
            name = 'Celery (Flower)';
            cmd = `powershell.exe -NoExit -Command "& '${venvCelery}' -A config flower --address=0.0.0.0 --port=5555"`;
            break;
        case 'solo':
            name = 'Celery (Debug Solo)';
            cmd = `powershell.exe -NoExit -Command "& '${venvCelery}' -A config worker --loglevel=info --pool=solo -Q high_priority,celery,articles,fetch"`;
            break;
    }

    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': 'Bearer ' + token } : {})
        };
        const body = {
            name: name,
            cwd: currentBubble.path,
            command: cmd,
            bubbleId: currentBubble.id,
            role: 'server',
            cols: 100,
            rows: 30
        };
        const res = await fetch('/api/sessions', { method: 'POST', headers, body: JSON.stringify(body) });
        if (res.ok) {
            const sess = await res.json();
            showToast(`${name} iniciado!`);
            switchWorkspaceTab('terminal');
            activeBubbleSession = sess;
            await fetchSessions();
            switchTerminalSession(sess.id);
        } else {
            appAlert("Error al iniciar worker de Celery: " + (await res.text()), "Error en Celery", "❌");
        }
    } catch (e) {
        appAlert("Error de red: " + e, "Error de Conexión", "❌");
    }
}

// Spawn Claude with VPN Guard Modal
function spawnClaudeInBubble() {
    if (!currentBubble) {
        showToast("Selecciona una tarea primero.");
        return;
    }
    switchWorkspaceTab('terminal');

    // VPN GUARD: If VPN is disconnected, warn user with custom app modal
    const isVpnConnected = serverInfo && serverInfo.vpn && serverInfo.vpn.connected;
    if (!isVpnConnected) {
        openVpnWarningModal();
        return;
    }

    spawnClaudeProcess();
}

function openVpnWarningModal() {
    const locEl = document.getElementById('vpn-warning-target-loc');
    if (locEl) {
        const targetLoc = (serverInfo && serverInfo.vpn && (serverInfo.vpn.location || serverInfo.vpn.lastLocation)) || 'Miami Vice (US)';
        locEl.textContent = `Ubicación: ${targetLoc}`;
    }
    const modal = document.getElementById('modal-vpn-warning');
    if (modal) {
        modal.classList.add('open');
    }
}

function closeVpnWarningModal() {
    const modal = document.getElementById('modal-vpn-warning');
    if (modal) {
        modal.classList.remove('open');
    }
}

async function proceedClaudeWithVpn() {
    closeVpnWarningModal();
    showToast("Conectando Windscribe VPN...");
    try {
        await toggleVPN('connect');
    } catch(e) {
        console.warn("VPN connect error:", e);
    }
    showToast("Estableciendo conexión segura... Iniciando Claude...");
    setTimeout(async () => {
        await spawnClaudeProcess();
    }, 3500);
}

async function proceedClaudeWithoutVpn() {
    closeVpnWarningModal();
    showToast("Iniciando Claude sin VPN...");
    await spawnClaudeProcess();
}

async function spawnClaudeProcess() {
    if (!currentBubble) {
        showToast("Selecciona una tarea primero.");
        return;
    }
    try {
        switchWorkspaceTab('terminal');
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': 'Bearer ' + token } : {})
        };
        const existingClaude = getBubbleSessions(currentBubble).filter(s => s.role === 'claude').length;
        const defaultName = existingClaude > 0 
            ? `Claude (${currentBubble.name}) #${existingClaude + 1}` 
            : `Claude (${currentBubble.name})`;
        const body = {
            name: defaultName,
            cwd: currentBubble.path,
            command: 'claude',
            bubbleId: currentBubble.id,
            role: 'claude',
            cols: 100,
            rows: 30
        };
        const res = await fetch('/api/sessions', { method: 'POST', headers, body: JSON.stringify(body) });
        if (res.ok) {
            const sess = await res.json();
            showToast("Instancia de Claude Code iniciada.");
            activeBubbleSession = sess;
            await fetchSessions();
            switchTerminalSession(sess.id);
        } else {
            appAlert("Error al iniciar Claude: " + (await res.text()), "Error en Claude", "❌");
        }
    } catch (e) {
        appAlert("Error de red: " + e, "Error de Conexión", "❌");
    }
}

async function spawnAgyInBubble() {
    if (!currentBubble) {
        showToast("Selecciona una tarea primero.");
        return;
    }
    try {
        switchWorkspaceTab('terminal');
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': 'Bearer ' + token } : {})
        };
        const existingAgy = getBubbleSessions(currentBubble).filter(s => s.role === 'agy').length;
        const defaultName = existingAgy > 0 
            ? `Antigravity (${currentBubble.name}) #${existingAgy + 1}` 
            : `Antigravity (${currentBubble.name})`;
        const body = {
            name: defaultName,
            cwd: currentBubble.path,
            command: 'agy',
            bubbleId: currentBubble.id,
            role: 'agy',
            cols: 100,
            rows: 30
        };
        showToast("Iniciando Antigravity CLI...");
        const res = await fetch('/api/sessions', { method: 'POST', headers, body: JSON.stringify(body) });
        if (res.ok) {
            const sess = await res.json();
            showToast("Instancia de Antigravity CLI iniciada.");
            activeBubbleSession = sess;
            await fetchSessions();
            switchTerminalSession(sess.id);
        } else {
            appAlert("Error al iniciar Antigravity: " + (await res.text()), "Error en Antigravity", "❌");
        }
    } catch (e) {
        appAlert("Error de red: " + e, "Error de Conexión", "❌");
    }
}

async function spawnShellInBubble() {
    if (!currentBubble) {
        showToast("Selecciona una tarea primero.");
        return;
    }
    switchWorkspaceTab('terminal');
    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': 'Bearer ' + token } : {})
        };
        const existingShell = getBubbleSessions(currentBubble).filter(s => s.role === 'shell').length;
        const defaultName = existingShell > 0 
            ? `Shell (${currentBubble.name}) #${existingShell + 1}` 
            : `Shell (${currentBubble.name})`;
        const body = {
            name: defaultName,
            cwd: currentBubble.path,
            command: 'powershell.exe',
            bubbleId: currentBubble.id,
            role: 'shell',
            cols: 100,
            rows: 30
        };
        const res = await fetch('/api/sessions', { method: 'POST', headers, body: JSON.stringify(body) });
        if (res.ok) {
            const sess = await res.json();
            showToast("Terminal shell iniciada.");
            activeBubbleSession = sess;
            await fetchSessions();
            switchTerminalSession(sess.id);
        }
    } catch (e) {
        appAlert("Error al iniciar shell: " + e, "Error en Terminal", "❌");
    }
}

// Dev Server Control
async function toggleBubbleDevServer() {
    if (!currentBubble) return;
    const token = serverInfo ? serverInfo.authToken : '';
    const headers = token ? { 'Authorization': 'Bearer ' + token } : {};

    if (currentBubble.devRunning) {
        // Stop
        try {
            const res = await fetch(`/api/bubbles/${currentBubble.id}/dev/stop`, { method: 'POST', headers });
            if (res.ok) {
                currentBubble.devRunning = false;
                currentBubble.devPid = 0;
                updateBubbleHeaderUI();
                renderBubblesList();
                showToast("Dev Server detenido.");
            }
        } catch (e) {
            appAlert("Error al detener dev server: " + e, "Error", "❌");
        }
    } else {
        // Start
        try {
            const res = await fetch(`/api/bubbles/${currentBubble.id}/dev/start`, { method: 'POST', headers });
            if (res.ok) {
                const data = await res.json();
                currentBubble.devRunning = true;
                currentBubble.devPid = data.pid;
                updateBubbleHeaderUI();
                renderBubblesList();
                showToast(`Dev Server iniciado (PID ${data.pid})`);
                if (activeTab === 'preview') {
                    setTimeout(reloadPreviewFrame, 1500);
                }
            } else {
                appAlert("Error al iniciar dev server: " + (await res.text()), "Error en Dev Server", "❌");
            }
        } catch (e) {
            appAlert("Error de red: " + e, "Error de Conexión", "❌");
        }
    }
}



function openFlowerInPreview() {
    switchWorkspaceTab('preview');
    const input = document.getElementById('preview-url-input');
    const frame = document.getElementById('preview-iframe');
    if (input) input.value = 'http://localhost:5555/';
    if (frame) frame.src = 'http://localhost:5555/';
    showToast("Flower Dashboard cargado en Live Preview");
}
