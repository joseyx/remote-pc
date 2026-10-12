// ==========================================
// System Stats, Process Manager, VPN & Pairing
// ==========================================

// Fetch Server Info & VPN
async function fetchServerInfo() {
    try {
        const res = await fetch('/api/server-info');
        if (!res.ok) return;
        serverInfo = await res.json();
        updateServerInfoUI();
    } catch (e) {
        console.error("Error al obtener server-info:", e);
    }
}

function updateServerInfoUI() {
    if (!serverInfo) return;
    const primaryIp = (serverInfo.lanIps && serverInfo.lanIps.length > 0) ? serverInfo.lanIps[0] : window.location.hostname;
    document.getElementById('val-lan-ip').textContent = primaryIp + ':' + (serverInfo.port || '8080');
    document.getElementById('pair-ip').textContent = primaryIp;
    document.getElementById('pair-port').textContent = serverInfo.port || '8080';
    document.getElementById('pair-token').textContent = serverInfo.authToken || '';

    const verEl = document.getElementById('modal-orbit-version');
    if (verEl && serverInfo.version) {
        verEl.textContent = 'Orbit v' + serverInfo.version;
    }

    if (serverInfo.vpn) {
        updateVpnUI(serverInfo.vpn);
    }
}

function updateVpnUI(vpn) {
    const hDot = document.getElementById('header-vpn-dot');
    const hText = document.getElementById('header-vpn-text');
    const mDot = document.getElementById('vpn-mini-dot');
    const mLoc = document.getElementById('vpn-mini-loc');
    const mIp = document.getElementById('vpn-mini-ip');
    const mBtn = document.getElementById('btn-mini-vpn-toggle');

    const modDot = document.getElementById('modal-vpn-dot');
    const modLoc = document.getElementById('modal-vpn-loc');
    const modIp = document.getElementById('modal-vpn-ip');
    const modBtn = document.getElementById('modal-vpn-toggle-btn');
    const modFw = document.getElementById('modal-vpn-firewall');

    if (modFw) modFw.textContent = vpn.firewall || 'Off';

    if (!vpn.installed) {
        hText.textContent = "VPN: No instalada";
        hDot.className = "vpn-dot";
        mLoc.textContent = "Windscribe no detectado";
        mIp.textContent = "CLI no encontrado";
        mBtn.disabled = true;
        return;
    }

    if (vpn.state === "Connecting") {
        hText.textContent = "VPN: Conectando...";
        hDot.className = "vpn-dot connecting";
        mDot.className = "vpn-dot connecting";
        mLoc.textContent = "Conectando túnel...";
        mIp.textContent = "Estableciendo conexión segura";
        mBtn.disabled = true;
        mBtn.textContent = "...";
        if (modDot) modDot.className = "vpn-dot connecting";
        if (modLoc) modLoc.textContent = "Conectando...";
        if (modBtn) { modBtn.disabled = true; modBtn.textContent = "Conectando..."; }
    } else if (vpn.connected) {
        const locStr = vpn.location || "Protegido";
        hText.textContent = `VPN: ${locStr}`;
        hDot.className = "vpn-dot connected";
        mDot.className = "vpn-dot connected";
        mLoc.textContent = locStr;
        mIp.textContent = `IP: ${vpn.ip || '--'} · ${vpn.protocol || 'Auto'}`;
        mBtn.disabled = false;
        mBtn.className = "btn btn-danger btn-sm";
        mBtn.textContent = "Desconectar";

        if (modDot) modDot.className = "vpn-dot connected";
        if (modLoc) modLoc.textContent = locStr;
        if (modIp) modIp.textContent = `IP: ${vpn.ip || '--'} · Protocolo: ${vpn.protocol || 'Auto'}`;
        if (modBtn) {
            modBtn.disabled = false;
            modBtn.className = "btn btn-danger";
            modBtn.textContent = "Desconectar";
        }
    } else {
        hText.textContent = "VPN: Desconectada";
        hDot.className = "vpn-dot";
        mDot.className = "vpn-dot";
        mLoc.textContent = "Túnel Inactivo";
        mIp.textContent = `IP Pública: ${vpn.ip || '--'}`;
        mBtn.disabled = false;
        mBtn.className = "btn btn-primary btn-sm";
        mBtn.textContent = "Conectar";

        if (modDot) modDot.className = "vpn-dot";
        if (modLoc) modLoc.textContent = "Desconectado";
        if (modIp) modIp.textContent = `IP Pública: ${vpn.ip || '--'}`;
        if (modBtn) {
            modBtn.disabled = false;
            modBtn.className = "btn btn-primary";
            modBtn.textContent = "Conectar";
        }
    }
}



// VPN Modal Controls
function openVPNModal() {
    document.getElementById('modal-vpn').classList.add('open');
}

function closeVPNModal() {
    document.getElementById('modal-vpn').classList.remove('open');
}

function onModalVpnProtoChange() {
    const proto = document.getElementById('modal-vpn-proto-select').value;
    const portSel = document.getElementById('modal-vpn-port-select');
    const PROTO_PORTS = {
        wireguard: ["443", "80", "53", "1194", "65142"],
        stealth: ["443", "8080", "8443"],
        udp: ["443", "80", "53", "1194"],
        tcp: ["443", "80", "53"],
        wstunnel: ["443", "8080"],
        ikev2: ["500"]
    };
    const ports = PROTO_PORTS[proto] || ["443"];
    portSel.innerHTML = ports.map(p => `<option value="${p}">${p}</option>`).join('');
}

function onModalVpnLocChange() {
    const loc = document.getElementById('modal-vpn-loc-select').value;
    const cust = document.getElementById('modal-vpn-custom-loc');
    if (loc === 'custom') {
        cust.style.display = 'block';
        cust.focus();
    } else {
        cust.style.display = 'none';
    }
}

async function toggleVPNMini() {
    await toggleVPN('toggle');
}

async function toggleVPN(action = 'toggle') {
    const token = serverInfo ? serverInfo.authToken : '';
    const headers = {
        'Content-Type': 'application/json',
        ...(token ? { 'Authorization': 'Bearer ' + token } : {})
    };

    const isConnected = serverInfo && serverInfo.vpn && serverInfo.vpn.connected;
    let endpoint = '/api/vpn/connect';
    let body = null;

    if (action === 'toggle' && isConnected) {
        endpoint = '/api/vpn/disconnect';
    } else {
        endpoint = '/api/vpn/connect';
        let loc = document.getElementById('modal-vpn-loc-select')?.value || 'best';
        if (loc === 'custom') {
            loc = document.getElementById('modal-vpn-custom-loc')?.value.trim() || 'best';
        }
        const proto = document.getElementById('modal-vpn-proto-select')?.value || 'wireguard';
        const port = document.getElementById('modal-vpn-port-select')?.value || '443';
        body = JSON.stringify({ location: loc, protocol: proto, port: port });
    }

    try {
        showToast(endpoint === '/api/vpn/disconnect' ? "Desconectando VPN..." : "Conectando Windscribe VPN...");
        const res = await fetch(endpoint, { method: 'POST', headers, body });
        if (res.ok) {
            setTimeout(fetchServerInfo, 1000);
            setTimeout(fetchServerInfo, 3500);
            setTimeout(fetchServerInfo, 7000);
        } else {
            appAlert("Fallo acción VPN: " + (await res.text()), "Error de VPN", "❌");
        }
    } catch (e) {
        appAlert("Error de red: " + e, "Error de Conexión", "❌");
    }
}

// Pair Modal
function openPairModal() {
    document.getElementById('modal-pair').classList.add('open');
}

function closePairModal() {
    document.getElementById('modal-pair').classList.remove('open');
}

function copyToken() {
    if (!serverInfo) return;
    navigator.clipboard.writeText(serverInfo.authToken);
    showToast("Token de autenticación copiado!");
}

function copyFullConfig() {
    if (!serverInfo) return;
    const primaryIp = (serverInfo.lanIps && serverInfo.lanIps.length > 0) ? serverInfo.lanIps[0] : window.location.hostname;
    const config = {
        host: primaryIp,
        port: serverInfo.port || "8080",
        token: serverInfo.authToken || ""
    };
    navigator.clipboard.writeText(JSON.stringify(config, null, 2));
    showToast("Configuración JSON copiada!");
}



// ==========================================
// QoL 4 & 5: System Stats & Audio Chime
// ==========================================
let ramHistory = [];
const MAX_RAM_HISTORY = 30;

async function fetchSystemStats(force = false) {
    try {
        const modal = document.getElementById('modal-sys-stats');
        const isModalOpen = modal && modal.classList.contains('open') && modal.style.display !== 'none';
        const url = (isModalOpen || force) ? '/api/system/stats?top=true' : '/api/system/stats';
        const res = await fetch(url);
        if (!res.ok) return;
        const stats = await res.json();
        updateSystemStatsUI(stats);
    } catch(e) {}
}

function updateSystemStatsUI(stats) {
    if (!stats || !stats.memory) return;
    const ramText = `${stats.memory.usedGB}/${stats.memory.totalGB} GB (${stats.memory.percent}%)`;
    const ramEl = document.getElementById('val-sys-ram');
    if (ramEl) {
        ramEl.textContent = ramText;
        if (stats.memory.percent > 85) ramEl.style.color = 'var(--red)';
        else if (stats.memory.percent > 70) ramEl.style.color = 'var(--amber)';
        else ramEl.style.color = 'var(--text-main)';
    }

    const srvEl = document.getElementById('sys-services-badge');
    if (srvEl && stats.services) {
        let srvs = [];
        const mysql = stats.services.find(s => s.id === 'mysql');
        if (mysql && mysql.running) srvs.push('<span style="color: var(--green);" title="MySQL activo (:3306)">● MySQL</span>');
        else srvs.push('<span style="color: var(--text-dim);" title="MySQL inactivo (:3306)">○ MySQL</span>');

        const redis = stats.services.find(s => s.id === 'redis');
        if (redis && redis.running) srvs.push('<span style="color: var(--green);" title="Redis activo (:6379)">● Redis</span>');
        srvEl.innerHTML = srvs.join(' · ');
    }

    // Record history point
    ramHistory.push({
        time: Date.now(),
        percent: stats.memory.percent,
        usedGB: stats.memory.usedGB
    });
    if (ramHistory.length > MAX_RAM_HISTORY) {
        ramHistory.shift();
    }

    // Update modal if open
    updateModalStatsUI(stats);
}

let sysStatsCustomPos = null;
let sysStatsIsDragging = false;
let sysStatsDragStartX = 0;
let sysStatsDragStartY = 0;
let sysStatsInitialLeft = 0;
let sysStatsInitialTop = 0;

function positionSysStatsDefault() {
    const modal = document.getElementById('modal-sys-stats');
    const trigger = document.getElementById('header-sys-stats');
    if (!modal) return;

    const modalWidth = Math.min(580, window.innerWidth - 20);
    modal.style.width = modalWidth + 'px';

    if (trigger) {
        const rect = trigger.getBoundingClientRect();
        let left = rect.left + (rect.width / 2) - (modalWidth / 2);
        if (left + modalWidth > window.innerWidth - 12) {
            left = window.innerWidth - modalWidth - 12;
        }
        if (left < 12) {
            left = 12;
        }
        const top = rect.bottom + 8;
        modal.style.left = `${Math.round(left)}px`;
        modal.style.top = `${Math.round(top)}px`;
    } else {
        modal.style.left = `${Math.round((window.innerWidth - modalWidth) / 2)}px`;
        modal.style.top = '60px';
    }
    modal.style.right = 'auto';
    modal.style.bottom = 'auto';
}

function resetSystemStatsPosition() {
    sysStatsCustomPos = null;
    positionSysStatsDefault();
    showToast("Posición del monitor restablecida");
}

function initSysStatsDrag() {
    const modal = document.getElementById('modal-sys-stats');
    const handle = document.getElementById('sys-stats-drag-handle');
    if (!modal || !handle) return;

    function onPointerDown(e) {
        if (e.target.closest('button')) return;

        sysStatsIsDragging = true;
        modal.classList.add('is-dragging');

        const clientX = e.clientX ?? (e.touches && e.touches[0] ? e.touches[0].clientX : 0);
        const clientY = e.clientY ?? (e.touches && e.touches[0] ? e.touches[0].clientY : 0);

        sysStatsDragStartX = clientX;
        sysStatsDragStartY = clientY;

        const rect = modal.getBoundingClientRect();
        sysStatsInitialLeft = rect.left;
        sysStatsInitialTop = rect.top;

        document.addEventListener('mousemove', onPointerMove, { passive: false });
        document.addEventListener('mouseup', onPointerUp);
        document.addEventListener('touchmove', onPointerMove, { passive: false });
        document.addEventListener('touchend', onPointerUp);
    }

    function onPointerMove(e) {
        if (!sysStatsIsDragging) return;
        e.preventDefault();

        const clientX = e.clientX ?? (e.touches && e.touches[0] ? e.touches[0].clientX : 0);
        const clientY = e.clientY ?? (e.touches && e.touches[0] ? e.touches[0].clientY : 0);

        const deltaX = clientX - sysStatsDragStartX;
        const deltaY = clientY - sysStatsDragStartY;

        let newLeft = sysStatsInitialLeft + deltaX;
        let newTop = sysStatsInitialTop + deltaY;

        const maxLeft = Math.max(0, window.innerWidth - modal.offsetWidth);
        const maxTop = Math.max(0, window.innerHeight - modal.offsetHeight);

        newLeft = Math.max(0, Math.min(newLeft, maxLeft));
        newTop = Math.max(0, Math.min(newTop, maxTop));

        modal.style.left = `${Math.round(newLeft)}px`;
        modal.style.top = `${Math.round(newTop)}px`;

        sysStatsCustomPos = { x: newLeft, y: newTop };
    }

    function onPointerUp() {
        if (!sysStatsIsDragging) return;
        sysStatsIsDragging = false;
        modal.classList.remove('is-dragging');

        document.removeEventListener('mousemove', onPointerMove);
        document.removeEventListener('mouseup', onPointerUp);
        document.removeEventListener('touchmove', onPointerMove);
        document.removeEventListener('touchend', onPointerUp);
    }

    handle.addEventListener('mousedown', onPointerDown);
    handle.addEventListener('touchstart', onPointerDown, { passive: true });
}

function openSystemStatsModal() {
    const modal = document.getElementById('modal-sys-stats');
    if (!modal) return;
    modal.classList.add('open');
    modal.style.display = 'flex';

    if (sysStatsCustomPos) {
        const maxLeft = Math.max(0, window.innerWidth - modal.offsetWidth);
        const maxTop = Math.max(0, window.innerHeight - modal.offsetHeight);
        const left = Math.max(0, Math.min(sysStatsCustomPos.x, maxLeft));
        const top = Math.max(0, Math.min(sysStatsCustomPos.y, maxTop));
        modal.style.left = `${Math.round(left)}px`;
        modal.style.top = `${Math.round(top)}px`;
    } else {
        positionSysStatsDefault();
    }

    fetchSystemStats(true);
    setTimeout(drawRamHistoryChart, 60);
}

function closeSystemStatsModal() {
    const modal = document.getElementById('modal-sys-stats');
    if (modal) {
        modal.classList.remove('open');
        modal.style.display = 'none';
    }
}

function toggleSystemStatsModal() {
    const modal = document.getElementById('modal-sys-stats');
    if (!modal) return;
    if (modal.classList.contains('open') && modal.style.display !== 'none') {
        closeSystemStatsModal();
    } else {
        openSystemStatsModal();
    }
}

function updateModalStatsUI(stats) {
    const modal = document.getElementById('modal-sys-stats');
    if (!modal || !modal.classList.contains('open')) return;

    // Physical RAM
    const ramPct = document.getElementById('modal-ram-percent');
    const ramBar = document.getElementById('modal-ram-bar');
    const ramUsed = document.getElementById('modal-ram-used');
    const ramFree = document.getElementById('modal-ram-free');
    const ramTotal = document.getElementById('modal-ram-total');
    const curLoad = document.getElementById('modal-chart-cur-load');

    if (ramPct) ramPct.textContent = `${stats.memory.percent}%`;
    if (ramBar) {
        ramBar.style.width = `${stats.memory.percent}%`;
        if (stats.memory.percent > 85) ramBar.style.background = 'linear-gradient(90deg, #f59e0b, #f43f5e)';
        else if (stats.memory.percent > 70) ramBar.style.background = 'linear-gradient(90deg, #38bdf8, #f59e0b)';
        else ramBar.style.background = 'linear-gradient(90deg, #38bdf8, #818cf8)';
    }
    if (ramUsed) ramUsed.textContent = `${stats.memory.usedGB} GB`;
    if (ramFree) ramFree.textContent = `${stats.memory.freeGB} GB`;
    if (ramTotal) ramTotal.textContent = `${stats.memory.totalGB} GB`;
    if (curLoad) curLoad.textContent = `Carga actual: ${stats.memory.percent}% (${stats.memory.usedGB} GB)`;

    // Pagefile / Swap
    const pagePct = document.getElementById('modal-page-percent');
    const pageBar = document.getElementById('modal-page-bar');
    const pageUsed = document.getElementById('modal-page-used');
    const pageTotal = document.getElementById('modal-page-total');

    if (pagePct) pagePct.textContent = `${stats.memory.pagePercent || 0}%`;
    if (pageBar) pageBar.style.width = `${stats.memory.pagePercent || 0}%`;
    if (pageUsed) pageUsed.textContent = `${stats.memory.usedPageGB || 0} GB`;
    if (pageTotal) pageTotal.textContent = `${stats.memory.totalPageGB || 0} GB`;

    // Services list
    const srvList = document.getElementById('modal-services-list');
    if (srvList && stats.services) {
        srvList.innerHTML = stats.services.map(s => {
            const statusDot = s.running ? '<span style="color: var(--green);">●</span>' : '<span style="color: var(--text-dim);">○</span>';
            const pingText = s.running ? `<span style="font-size: 10px; color: var(--green); font-weight: 600;">${(s.pingMs && s.pingMs > 0) ? s.pingMs + ' ms' : '< 1 ms'}</span>` : '<span style="font-size: 10px; color: var(--text-dim);">Inactivo</span>';

            let actionsHtml = '';
            if (s.id === 'redis') {
                if (s.running) {
                    actionsHtml = `<button class="btn btn-sm btn-danger" onclick="controlService('redis', 'stop')" style="padding: 1px 6px; font-size: 10px;" title="Detener servicio Redis en WSL">⏹ Parar</button>`;
                } else {
                    actionsHtml = `<button class="btn btn-sm" onclick="controlService('redis', 'start')" style="padding: 1px 6px; font-size: 10px; color: var(--green);" title="Iniciar servicio Redis en WSL">▶ Iniciar</button>`;
                }
            } else if (s.id === 'flower') {
                if (s.running) {
                    actionsHtml = `
                        <button class="btn btn-sm" onclick="openFlowerInPreview()" style="padding: 1px 6px; font-size: 10px; color: var(--accent);" title="Abrir Flower en el Live Preview">🌸 Preview</button>
                        <button class="btn btn-sm btn-danger" onclick="controlService('flower', 'stop')" style="padding: 1px 6px; font-size: 10px;" title="Detener dashboard Flower">⏹</button>
                    `;
                } else {
                    actionsHtml = `<button class="btn btn-sm" onclick="controlService('flower', 'start')" style="padding: 1px 6px; font-size: 10px; color: var(--green);" title="Lanzar dashboard Flower en puerto 5555">▶ Iniciar</button>`;
                }
            }

            return `
                <div style="background: var(--bg-surface); padding: 8px 10px; border-radius: 6px; border: 1px solid var(--border-subtle); display: flex; justify-content: space-between; align-items: center;">
                    <div>
                        <div style="font-size: 12px; font-weight: 600; display: flex; align-items: center; gap: 6px;">
                            ${statusDot} ${escapeHtml(s.name)}
                        </div>
                        <div style="font-size: 10px; color: var(--text-dim); font-family: monospace;">Puerto :${s.port}</div>
                    </div>
                    <div style="display: flex; align-items: center; gap: 6px;">
                        ${pingText}
                        ${actionsHtml}
                    </div>
                </div>
            `;
        }).join('');
    }

    // Top Processes list
    const procList = document.getElementById('modal-top-processes-list');
    if (procList && stats.topProcesses) {
        if (stats.topProcesses.length === 0) {
            procList.innerHTML = '<div style="font-size: 11px; color: var(--text-dim); text-align: center; padding: 6px;">Sin procesos pesados detectados.</div>';
        } else {
            procList.innerHTML = stats.topProcesses.map(p => {
                const ramLabel = p.ramMB >= 1024 
                    ? `${(p.ramMB / 1024).toFixed(2)} GB` 
                    : `${p.ramMB.toFixed(1)} MB`;
                const isHeavy = p.ramMB > 500;
                const badgeColor = isHeavy ? '#f43f5e' : 'var(--accent)';
                return `
                    <div style="background: var(--bg-surface); padding: 6px 10px; border-radius: 6px; border: 1px solid var(--border-subtle); display: flex; justify-content: space-between; align-items: center;">
                        <div style="display: flex; align-items: center; gap: 8px;">
                            <span style="font-size: 12px; font-weight: 600; color: var(--text-main); font-family: monospace;">${escapeHtml(p.name)}</span>
                            <span style="font-size: 10px; background: rgba(255,255,255,0.06); padding: 1px 5px; border-radius: 3px; color: var(--text-dim);">PID ${p.pid}</span>
                        </div>
                        <div style="display: flex; align-items: center; gap: 10px;">
                            <span style="font-size: 11px; font-weight: 700; color: ${badgeColor};">${ramLabel}</span>
                            <button class="btn btn-sm btn-danger" onclick="killProcessPrompt(${p.pid}, '${escapeHtml(p.name)}')" style="padding: 2px 7px; font-size: 10px;" title="Forzar finalización de este proceso">💀 Finalizar</button>
                        </div>
                    </div>
                `;
            }).join('');
        }
    }

    // Host info
    const hostInfo = document.getElementById('modal-host-info');
    if (hostInfo && serverInfo) {
        hostInfo.textContent = `Host: ${serverInfo.hostname || 'Localhost'} · Uptime: ${serverInfo.uptime || 'Activo'}`;
    }

    drawRamHistoryChart();
}

async function killProcessPrompt(pid, name) {
    const ok = await appConfirm(`¿Estás seguro de que deseas forzar el cierre del proceso?\n\nNombre: ${name}\nPID: ${pid}\n\nEsta acción finalizará el proceso inmediatamente.`, "Forzar Cierre de Proceso", "Finalizar Proceso", "Cancelar", true, "💀");
    if (!ok) return;
    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': 'Bearer ' + token } : {})
        };
        const res = await fetch('/api/system/process/kill', {
            method: 'POST',
            headers,
            body: JSON.stringify({ pid })
        });
        if (res.ok) {
            showToast(`Proceso "${name}" (PID ${pid}) finalizado.`);
            fetchSystemStats(true);
        } else {
            const err = await res.json();
            appAlert(`No se pudo finalizar el proceso: ${err.error || 'Error desconocido'}`, "Error de Proceso", "❌");
        }
    } catch (e) {
        appAlert("Error de conexión: " + e, "Error", "❌");
    }
}

async function controlService(serviceId, action) {
    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': 'Bearer ' + token } : {})
        };
        showToast(`Enviando comando "${action}" a ${serviceId}...`);
        const res = await fetch(`/api/services/${serviceId}/${action}`, { method: 'POST', headers });
        if (res.ok) {
            showToast(`Servicio "${serviceId}" ${action === 'start' ? 'iniciado' : 'detenido'}.`);
            setTimeout(() => fetchSystemStats(true), 600);
            setTimeout(() => fetchSystemStats(true), 2200);
        } else {
            const err = await res.json();
            appAlert(`Error al gestionar servicio: ${err.error || 'Error desconocido'}`, "Error en Servicio", "❌");
        }
    } catch(e) {
        appAlert("Error de conexión: " + e, "Error", "❌");
    }
}



function drawRamHistoryChart() {
    const canvas = document.getElementById('ram-history-canvas');
    if (!canvas) return;
    if (canvas.offsetWidth && canvas.width !== canvas.offsetWidth) {
        canvas.width = canvas.offsetWidth;
    }
    const ctx = canvas.getContext('2d');
    const width = canvas.width;
    const height = canvas.height;

    ctx.clearRect(0, 0, width, height);

    if (ramHistory.length < 2) {
        ctx.fillStyle = '#64748b';
        ctx.font = '11px sans-serif';
        ctx.textAlign = 'center';
        ctx.fillText('Recopilando historial de memoria en tiempo real...', width / 2, height / 2);
        return;
    }

    // Grid lines at 25%, 50%, 75%
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.05)';
    ctx.lineWidth = 1;
    for (let p of [25, 50, 75]) {
        const y = height - (p / 100) * (height - 14) - 4;
        ctx.beginPath();
        ctx.moveTo(0, y);
        ctx.lineTo(width, y);
        ctx.stroke();

        ctx.fillStyle = 'rgba(148, 163, 184, 0.35)';
        ctx.font = '9px monospace';
        ctx.textAlign = 'right';
        ctx.fillText(`${p}%`, width - 6, y - 2);
    }

    // Points calculation
    const step = width / (MAX_RAM_HISTORY - 1);
    const startIndex = MAX_RAM_HISTORY - ramHistory.length;

    const getY = (pct) => height - (pct / 100) * (height - 14) - 4;

    // Area Gradient Fill
    const grad = ctx.createLinearGradient(0, 0, 0, height);
    grad.addColorStop(0, 'rgba(56, 189, 248, 0.35)');
    grad.addColorStop(1, 'rgba(56, 189, 248, 0.01)');

    ctx.beginPath();
    ctx.moveTo(startIndex * step, height);

    ramHistory.forEach((pt, i) => {
        const x = (startIndex + i) * step;
        const y = getY(pt.percent);
        ctx.lineTo(x, y);
    });

    const lastX = (startIndex + ramHistory.length - 1) * step;
    ctx.lineTo(lastX, height);
    ctx.closePath();
    ctx.fillStyle = grad;
    ctx.fill();

    // Line Stroke
    ctx.beginPath();
    ramHistory.forEach((pt, i) => {
        const x = (startIndex + i) * step;
        const y = getY(pt.percent);
        if (i === 0) ctx.moveTo(x, y);
        else ctx.lineTo(x, y);
    });
    ctx.strokeStyle = '#38bdf8';
    ctx.lineWidth = 2;
    ctx.stroke();

    // Glowing dot on newest point
    const curPt = ramHistory[ramHistory.length - 1];
    const curY = getY(curPt.percent);

    ctx.beginPath();
    ctx.arc(lastX, curY, 4, 0, Math.PI * 2);
    ctx.fillStyle = '#38bdf8';
    ctx.fill();
    ctx.strokeStyle = '#ffffff';
    ctx.lineWidth = 2;
    ctx.stroke();
}

// ==========================================
// Orbit Auto-Updater Functions
// ==========================================

let latestUpdateInfo = null;
let isUpdatingOrbit = false;

async function checkForUpdates(silent = true) {
    const btnCheck = document.getElementById('btn-check-update-stats');
    if (btnCheck) {
        btnCheck.disabled = true;
        btnCheck.textContent = 'Verificando...';
    }

    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = token ? { 'Authorization': 'Bearer ' + token } : {};
        const res = await fetch('/api/system/version', { headers });
        if (!res.ok) throw new Error('HTTP ' + res.status);
        const data = await res.json();
        latestUpdateInfo = data;

        const curVerStr = data.currentVersion ? 'v' + data.currentVersion : 'v1.0.0';
        const verEl = document.getElementById('modal-orbit-version');
        if (verEl) verEl.textContent = 'Orbit ' + curVerStr;

        const headerBtn = document.getElementById('header-update-btn');
        const headerText = document.getElementById('header-update-text');

        if (data.updateAvailable) {
            if (headerBtn && headerText) {
                headerText.textContent = 'v' + data.latestVersion;
                headerBtn.style.display = 'inline-flex';
            }
            if (!silent) {
                openUpdateModal();
            }
        } else {
            if (headerBtn) headerBtn.style.display = 'none';
            if (!silent) {
                alert(`✓ Orbit ${curVerStr} ya está en la versión más reciente.`);
            }
        }
    } catch (err) {
        console.error('Error verificando actualizaciones:', err);
        if (!silent) {
            alert('Error al verificar actualizaciones de Orbit: ' + (err.message || err));
        }
    } finally {
        if (btnCheck) {
            btnCheck.disabled = false;
            btnCheck.textContent = '🔍 Buscar actualización';
        }
    }
}

function openUpdateModal() {
    if (!latestUpdateInfo) {
        checkForUpdates(false);
        return;
    }

    const modal = document.getElementById('modal-update');
    if (!modal) return;

    const curVerEl = document.getElementById('update-cur-ver');
    const newVerEl = document.getElementById('update-new-ver');
    const changelogEl = document.getElementById('update-changelog');

    if (curVerEl) curVerEl.textContent = 'v' + (latestUpdateInfo.currentVersion || '1.0.0');
    if (newVerEl) newVerEl.textContent = 'v' + (latestUpdateInfo.latestVersion || '1.0.0');
    if (changelogEl) changelogEl.textContent = latestUpdateInfo.changelog || 'Sin notas de versión disponibles.';

    const instBtn = document.getElementById('update-download-installer-btn');
    if (instBtn) {
        if (latestUpdateInfo.installerUrl) {
            instBtn.href = latestUpdateInfo.installerUrl;
            instBtn.style.display = 'inline-block';
        } else if (latestUpdateInfo.releaseUrl) {
            instBtn.href = latestUpdateInfo.releaseUrl;
            instBtn.textContent = '🌐 Ver en GitHub';
            instBtn.style.display = 'inline-block';
        } else {
            instBtn.style.display = 'none';
        }
    }

    // Reset progress UI
    const progressBox = document.getElementById('update-progress-box');
    if (progressBox) progressBox.style.display = 'none';
    const confirmBtn = document.getElementById('update-confirm-btn');
    if (confirmBtn) {
        confirmBtn.disabled = false;
        confirmBtn.textContent = '🚀 Actualizar Ahora';
    }
    const cancelBtn = document.getElementById('update-cancel-btn');
    if (cancelBtn) cancelBtn.disabled = false;

    modal.classList.add('open');
}

function closeUpdateModal() {
    if (isUpdatingOrbit) return;
    const modal = document.getElementById('modal-update');
    if (modal) modal.classList.remove('open');
}

async function applyOrbitUpdate() {
    if (isUpdatingOrbit) return;
    isUpdatingOrbit = true;

    const progressBox = document.getElementById('update-progress-box');
    const statusMsg = document.getElementById('update-status-msg');
    const confirmBtn = document.getElementById('update-confirm-btn');
    const cancelBtn = document.getElementById('update-cancel-btn');

    if (progressBox) progressBox.style.display = 'block';
    if (statusMsg) {
        statusMsg.style.color = '#60a5fa';
        statusMsg.textContent = 'Descargando paquete y actualizando Orbit...';
    }
    if (confirmBtn) {
        confirmBtn.disabled = true;
        confirmBtn.textContent = 'Actualizando...';
    }
    if (cancelBtn) cancelBtn.disabled = true;

    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': 'Bearer ' + token } : {})
        };

        const res = await fetch('/api/system/update', {
            method: 'POST',
            headers,
            body: JSON.stringify({ force: false })
        });

        const data = await res.json();
        if (!res.ok || data.error) {
            throw new Error(data.error || 'Error al aplicar actualización');
        }

        if (statusMsg) {
            statusMsg.textContent = '¡Actualización completada! Reiniciando Orbit, reconectando...';
        }

        setTimeout(() => {
            pollServerRestart();
        }, 2000);

    } catch (err) {
        console.error('Error aplicando actualización:', err);
        isUpdatingOrbit = false;
        if (statusMsg) {
            statusMsg.style.color = 'var(--red, #ef4444)';
            statusMsg.textContent = 'Fallo en la actualización: ' + (err.message || err);
        }
        if (confirmBtn) {
            confirmBtn.disabled = false;
            confirmBtn.textContent = 'Reintentar';
        }
        if (cancelBtn) cancelBtn.disabled = false;
    }
}

function pollServerRestart() {
    const statusMsg = document.getElementById('update-status-msg');
    let attempts = 0;
    const interval = setInterval(async () => {
        attempts++;
        if (statusMsg) {
            statusMsg.textContent = `Reconectando con Orbit (${attempts})...`;
        }
        try {
            const res = await fetch('/api/health');
            if (res.ok) {
                clearInterval(interval);
                if (statusMsg) {
                    statusMsg.textContent = '¡Orbit en línea con la nueva versión! Recargando...';
                }
                setTimeout(() => {
                    window.location.reload();
                }, 800);
            }
        } catch (e) {
            // Keep polling
        }
    }, 1500);
}

