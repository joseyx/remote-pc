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

// Pair Modal & QR Generation
let currentQRCode = null;

async function openPairModal() {
    if (!serverInfo) {
        await fetchServerInfo();
    }
    setupPairIpOptions();
    updatePairQR();
    const modal = document.getElementById('modal-pair');
    if (modal) modal.classList.add('open');
}

function closePairModal() {
    const modal = document.getElementById('modal-pair');
    if (modal) modal.classList.remove('open');
}

function setupPairIpOptions() {
    const select = document.getElementById('pair-ip-select');
    if (!select || !serverInfo) return;

    const previousVal = select.value;
    select.innerHTML = '';

    const ipSet = new Set();
    const candidates = [];

    // Current browser hostname if not localhost
    const curHost = window.location.hostname;
    if (curHost && curHost !== 'localhost' && curHost !== '127.0.0.1' && curHost !== '::1') {
        candidates.push({ ip: curHost, label: `Host Actual (${curHost})` });
        ipSet.add(curHost);
    }

    // Server detected LAN IPs
    if (Array.isArray(serverInfo.lanIps)) {
        for (const ip of serverInfo.lanIps) {
            if (!ip || ipSet.has(ip)) continue;
            ipSet.add(ip);
            let label = `LAN: ${ip}`;
            if (ip.startsWith('100.')) {
                label = `Tailscale VPN: ${ip}`;
            } else if (ip.startsWith('192.168.')) {
                label = `Wi-Fi / Red Local: ${ip}`;
            } else if (ip.startsWith('10.') || ip.startsWith('172.')) {
                label = `Red Privada: ${ip}`;
            }
            candidates.push({ ip, label });
        }
    }

    // Localhost fallback if set is empty
    if (candidates.length === 0) {
        candidates.push({ ip: curHost || '127.0.0.1', label: `Localhost: ${curHost || '127.0.0.1'}` });
    }

    for (const c of candidates) {
        const opt = document.createElement('option');
        opt.value = c.ip;
        opt.textContent = c.label;
        select.appendChild(opt);
    }

    if (previousVal && ipSet.has(previousVal)) {
        select.value = previousVal;
    }
}

function getPairUri() {
    if (!serverInfo) return '';
    const select = document.getElementById('pair-ip-select');
    const host = (select && select.value) ? select.value : ((serverInfo.lanIps && serverInfo.lanIps[0]) || window.location.hostname);
    const port = serverInfo.port || '8080';
    const token = serverInfo.authToken || '';
    return token ? `orbit://${host}:${port}?token=${encodeURIComponent(token)}` : `orbit://${host}:${port}`;
}

function updatePairQR() {
    if (!serverInfo) return;
    const select = document.getElementById('pair-ip-select');
    const host = (select && select.value) ? select.value : ((serverInfo.lanIps && serverInfo.lanIps[0]) || window.location.hostname);
    const port = serverInfo.port || '8080';
    const token = serverInfo.authToken || '';

    // Update manual text display
    const elIp = document.getElementById('pair-ip');
    const elPort = document.getElementById('pair-port');
    const elToken = document.getElementById('pair-token');
    if (elIp) elIp.textContent = host;
    if (elPort) elPort.textContent = port;
    if (elToken) elToken.textContent = token || '(ninguno)';

    const uri = getPairUri();
    const container = document.getElementById('pair-qr-canvas');
    if (!container) return;

    container.innerHTML = '';
    if (typeof QRCode !== 'undefined') {
        try {
            new QRCode(container, {
                text: uri,
                width: 200,
                height: 200,
                colorDark: '#0f172a',
                colorLight: '#ffffff',
                correctLevel: QRCode.CorrectLevel.M
            });
        } catch (e) {
            console.error('Error rendering QR code:', e);
            container.innerHTML = `<span style="color:red; font-size:12px;">Error al generar QR: ${e.message}</span>`;
        }
    } else {
        container.innerHTML = `<span style="color:var(--text-dim); font-size:12px;">Biblioteca QR no cargada</span>`;
    }
}

function copyPairUri() {
    const uri = getPairUri();
    if (!uri) return;
    navigator.clipboard.writeText(uri);
    showToast("Enlace orbit:// copiado!");
}

function copyToken() {
    if (!serverInfo) return;
    navigator.clipboard.writeText(serverInfo.authToken || '');
    showToast("Token de autenticación copiado!");
}

function copyFullConfig() {
    if (!serverInfo) return;
    const select = document.getElementById('pair-ip-select');
    const host = (select && select.value) ? select.value : ((serverInfo.lanIps && serverInfo.lanIps[0]) || window.location.hostname);
    const config = {
        host: host,
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
    if (changelogEl) {
        const rawNotes = (latestUpdateInfo.changelog || 'Sin notas de versión disponibles.').trim();
        if (typeof marked !== 'undefined') {
            try {
                changelogEl.innerHTML = marked.parse(rawNotes);
            } catch (err) {
                console.warn('Failed to parse changelog with marked:', err);
                changelogEl.textContent = rawNotes;
            }
        } else {
            changelogEl.textContent = rawNotes;
        }
    }

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

// ==========================================
// Orbit Doctor Diagnostic UI
// ==========================================

async function openDoctorModal() {
    const modal = document.getElementById('modal-doctor');
    if (!modal) return;
    modal.classList.add('open');
    await loadDoctorDiagnostics();
}

function closeDoctorModal() {
    const modal = document.getElementById('modal-doctor');
    if (modal) modal.classList.remove('open');
}

async function loadDoctorDiagnostics() {
    const listEl = document.getElementById('doctor-checks-list');
    const summaryText = document.getElementById('doctor-summary-text');
    const timeEl = document.getElementById('doctor-timestamp');

    if (summaryText) summaryText.textContent = 'Ejecutando diagnóstico...';
    if (listEl) {
        listEl.innerHTML = '<div style="font-size: 12px; color: var(--text-dim); text-align: center; padding: 20px;">Analizando subsistemas del entorno...</div>';
    }

    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = token ? { 'Authorization': 'Bearer ' + token } : {};
        const res = await fetch('/api/system/doctor', { headers });
        if (!res.ok) throw new Error('HTTP ' + res.status);
        const report = await res.json();

        if (timeEl && report.timestamp) {
            timeEl.textContent = 'Ejecutado: ' + new Date(report.timestamp).toLocaleTimeString();
        }

        if (summaryText) {
            if (report.errors === 0 && report.warnings === 0) {
                summaryText.innerHTML = `<span style="color: #34d399;">✓ ${report.passed} verificaciones superadas al 100%</span> · Orbit listo`;
            } else if (report.errors === 0) {
                summaryText.innerHTML = `<span style="color: #34d399;">✓ ${report.passed} superadas</span> · <span style="color: #fbbf24;">${report.warnings} aviso(s)</span> · 0 errores`;
            } else {
                summaryText.innerHTML = `<span style="color: #ef4444;">✗ ${report.errors} error(es)</span> · <span style="color: #fbbf24;">${report.warnings} aviso(s)</span> · ${report.passed} superadas`;
            }
        }

        if (listEl && report.items) {
            listEl.innerHTML = '';
            report.items.forEach(it => {
                const card = document.createElement('div');
                card.style.cssText = 'background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-sm); padding: 10px 12px; display: flex; flex-direction: column; gap: 4px;';

                let badgeHtml = '';
                if (it.status === 'ok') {
                    badgeHtml = '<span style="background: rgba(52, 211, 153, 0.15); color: #34d399; font-size: 11px; font-weight: 700; padding: 2px 7px; border-radius: 4px;">✓ OK</span>';
                } else if (it.status === 'warning') {
                    badgeHtml = '<span style="background: rgba(251, 191, 36, 0.15); color: #fbbf24; font-size: 11px; font-weight: 700; padding: 2px 7px; border-radius: 4px;">! AVISO</span>';
                } else {
                    badgeHtml = '<span style="background: rgba(239, 68, 68, 0.15); color: #ef4444; font-size: 11px; font-weight: 700; padding: 2px 7px; border-radius: 4px;">✗ ERROR</span>';
                }

                let detailsHtml = '';
                if (it.details) {
                    detailsHtml = `<div style="font-size: 11px; color: var(--text-dim); margin-top: 2px;">${escapeHtml(it.details)}</div>`;
                }

                let remediationHtml = '';
                if (it.remediation) {
                    remediationHtml = `<div style="font-size: 11px; color: #60a5fa; background: rgba(59, 130, 246, 0.08); border-left: 2px solid #60a5fa; padding: 4px 8px; border-radius: 2px; margin-top: 4px;">💡 <strong>Sugerencia:</strong> ${escapeHtml(it.remediation)}</div>`;
                }

                card.innerHTML = `
                    <div style="display: flex; justify-content: space-between; align-items: baseline;">
                        <div style="display: flex; align-items: center; gap: 6px;">
                            <span style="font-size: 10px; color: var(--text-dim); text-transform: uppercase; font-weight: 700; letter-spacing: 0.5px;">${escapeHtml(it.category || 'General')}</span>
                            <span style="color: var(--border-subtle);">·</span>
                            <strong style="font-size: 13px; color: var(--text-main);">${escapeHtml(it.name)}</strong>
                        </div>
                        ${badgeHtml}
                    </div>
                    <div style="font-size: 12px; color: var(--text-muted);">${escapeHtml(it.message)}</div>
                    ${detailsHtml}
                    ${remediationHtml}
                `;
                listEl.appendChild(card);
            });
        }

    } catch (err) {
        console.error('Error cargando diagnóstico:', err);
        if (summaryText) summaryText.textContent = 'Error al ejecutar diagnóstico';
        if (listEl) {
            listEl.innerHTML = `<div style="font-size: 12px; color: var(--red); padding: 14px; text-align: center;">Error al comunicarse con Orbit: ${escapeHtml(err.message || String(err))}</div>`;
        }
    }
}

// ==========================================
// Orbit Daemon Service Manager
// ==========================================
let serviceStatusData = null;

async function openServiceModal() {
    const modal = document.getElementById('modal-service');
    if (modal) modal.classList.add('open');
    await fetchServiceStatus();
}

function closeServiceModal() {
    const modal = document.getElementById('modal-service');
    if (modal) modal.classList.remove('open');
}

async function fetchServiceStatus() {
    const card = document.getElementById('service-status-card');
    const actions = document.getElementById('service-action-container');
    if (card) {
        card.innerHTML = `<span style="color: var(--text-dim);">Consultando estado del servicio...</span>`;
    }

    try {
        const res = await fetch('/api/system/service');
        if (!res.ok) throw new Error(await res.text());
        serviceStatusData = await res.json();
        renderServiceStatusUI(serviceStatusData);
    } catch (e) {
        if (card) card.innerHTML = `<span style="color: var(--red);">Error al obtener estado: ${escapeHtml(e.message || String(e))}</span>`;
    }
}

function renderServiceStatusUI(st) {
    const card = document.getElementById('service-status-card');
    const actions = document.getElementById('service-action-container');
    if (!card || !actions) return;

    const instBadge = st.installed
        ? `<span style="color: var(--green); font-weight: 600;">✓ Registrado en Autoarranque</span>`
        : `<span style="color: var(--amber); font-weight: 600;">! No Registrado en Autoarranque</span>`;

    const runBadge = st.running
        ? `<span style="color: var(--green); font-weight: 600;">🟢 Activo ${st.pid ? `(PID ${st.pid})` : ''}</span>`
        : `<span style="color: var(--red); font-weight: 600;">🔴 Inactivo</span>`;

    card.innerHTML = `
        <div style="display: flex; justify-content: space-between;">
            <span style="color: var(--text-dim);">Autoarranque OS:</span>
            ${instBadge}
        </div>
        <div style="display: flex; justify-content: space-between;">
            <span style="color: var(--text-dim);">Estado Proceso:</span>
            ${runBadge}
        </div>
        <div style="display: flex; justify-content: space-between;">
            <span style="color: var(--text-dim);">Tipo de Servicio:</span>
            <span style="color: var(--text-main);">${escapeHtml(st.serviceType || 'N/A')}</span>
        </div>
        <div style="display: flex; justify-content: space-between;">
            <span style="color: var(--text-dim);">Puerto HTTP:</span>
            <span style="color: var(--accent);">${escapeHtml(st.port)} (http://localhost:${escapeHtml(st.port)}/)</span>
        </div>
        ${st.uptime ? `
        <div style="display: flex; justify-content: space-between;">
            <span style="color: var(--text-dim);">Tiempo Activo:</span>
            <span style="color: var(--text-main);">${escapeHtml(st.uptime)}</span>
        </div>` : ''}
        ${st.exePath ? `
        <div style="margin-top: 4px; padding-top: 6px; border-top: 1px dashed var(--border-subtle); font-size: 11px; color: var(--text-dim); word-break: break-all;">
            <span>Ejecutable:</span> ${escapeHtml(st.exePath)}
        </div>` : ''}
    `;

    actions.innerHTML = '';

    if (!st.installed) {
        const installBtn = document.createElement('button');
        installBtn.className = 'btn btn-sm btn-primary';
        installBtn.innerHTML = '⚡ Registrar Autoarranque';
        installBtn.onclick = () => executeServiceAction('install');
        actions.appendChild(installBtn);
    } else {
        const uninstallBtn = document.createElement('button');
        uninstallBtn.className = 'btn btn-sm';
        uninstallBtn.style.color = 'var(--red)';
        uninstallBtn.innerHTML = '🗑️ Desinstalar Autoarranque';
        uninstallBtn.onclick = () => executeServiceAction('uninstall');
        actions.appendChild(uninstallBtn);
    }

    const refreshBtn = document.createElement('button');
    refreshBtn.className = 'btn btn-sm';
    refreshBtn.innerHTML = '🔄 Actualizar';
    refreshBtn.onclick = fetchServiceStatus;
    actions.appendChild(refreshBtn);
}

async function executeServiceAction(action) {
    showToast(`Ejecutando acción '${action}'...`);
    try {
        const res = await fetch('/api/system/service', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${serverInfo ? serverInfo.authToken : ''}`
            },
            body: JSON.stringify({ action })
        });
        const data = await res.json();
        if (!res.ok || data.error) {
            appAlert(`Error: ${data.error || 'Fallo al ejecutar acción'}`, 'Error de Servicio', '❌');
        } else {
            showToast(data.message || 'Acción completada con éxito');
            if (data.status) {
                renderServiceStatusUI(data.status);
            } else {
                await fetchServiceStatus();
            }
        }
    } catch (e) {
        appAlert(`Error de red: ${e.message}`, 'Error de Comunicación', '❌');
    }
}

// ==========================================
// Security, Token Rotation & PIN Management
// ==========================================
let securityStatusData = null;
let tokenMasked = true;

async function openSecurityModal() {
    const modal = document.getElementById('modal-security');
    if (modal) modal.classList.add('open');
    tokenMasked = true;
    updateTokenMaskUI();
    await fetchSecurityStatus();
}

function closeSecurityModal() {
    const modal = document.getElementById('modal-security');
    if (modal) modal.classList.remove('open');
}

function toggleTokenMask() {
    tokenMasked = !tokenMasked;
    updateTokenMaskUI();
}

function updateTokenMaskUI() {
    const input = document.getElementById('security-token-input');
    const btn = document.getElementById('btn-toggle-token-mask');
    if (!input || !btn) return;

    input.type = tokenMasked ? 'password' : 'text';
    btn.textContent = tokenMasked ? '👁️' : '🔒';
    btn.title = tokenMasked ? 'Revelar token' : 'Ocultar token';
}

async function fetchSecurityStatus() {
    const pinCard = document.getElementById('security-pin-card');
    const tokenInput = document.getElementById('security-token-input');

    try {
        const res = await fetch('/api/system/security', {
            headers: {
                'Authorization': `Bearer ${serverInfo ? serverInfo.authToken : ''}`
            }
        });
        if (!res.ok) throw new Error(await res.text());
        securityStatusData = await res.json();

        if (tokenInput && securityStatusData.token) {
            tokenInput.value = securityStatusData.token;
        }

        renderSecurityPINUI(securityStatusData);
    } catch (e) {
        console.error('Error fetching security status:', e);
        if (pinCard) {
            pinCard.innerHTML = `<span style="color: var(--red);">Error al consultar seguridad: ${escapeHtml(e.message || String(e))}</span>`;
        }
    }
}

function renderSecurityPINUI(sec) {
    const pinCard = document.getElementById('security-pin-card');
    if (!pinCard) return;

    if (sec.hasPin) {
        pinCard.innerHTML = `
            <div style="display: flex; justify-content: space-between; align-items: center;">
                <div style="display: flex; align-items: center; gap: 6px;">
                    <span style="color: var(--green); font-weight: 700;">✓ PIN Activo</span>
                    <span style="font-size: 11px; color: var(--text-dim);">(4-8 dígitos configurados)</span>
                </div>
                <div style="display: flex; gap: 6px;">
                    <button class="btn btn-sm" onclick="promptSetPIN(true)">Cambiar</button>
                    <button class="btn btn-sm" style="color: var(--red);" onclick="promptRemovePIN()">Eliminar</button>
                </div>
            </div>
            <div style="font-size: 11px; color: var(--text-dim);">
                El PIN es requerido para autorizar rotaciones de credenciales y desbloquear el Cockpit.
            </div>
        `;
    } else {
        pinCard.innerHTML = `
            <div style="display: flex; justify-content: space-between; align-items: center;">
                <div style="display: flex; align-items: center; gap: 6px;">
                    <span style="color: var(--amber); font-weight: 600;">! Sin PIN de Seguridad</span>
                </div>
                <button class="btn btn-sm btn-primary" onclick="promptSetPIN(false)">⚡ Configurar PIN</button>
            </div>
            <div style="font-size: 11px; color: var(--text-dim);">
                Agrega un PIN numérico para proteger cambios de configuración y bloquear la pantalla.
            </div>
        `;
    }
}

async function promptRotateToken() {
    const confirmed = await appConfirm(
        "¿Estás seguro de que deseas rotar el token de autenticación? Todas las aplicaciones móviles Orbit y herramientas MCP conectadas deberán actualizarse.",
        "Rotar Token de Autenticación",
        "Sí, rotar token",
        "Cancelar",
        true
    );
    if (!confirmed) return;

    let pin = '';
    if (securityStatusData && securityStatusData.hasPin) {
        pin = await appPrompt(
            "Introduce tu PIN de seguridad para autorizar la rotación del token:",
            "",
            "Autorización Requerida",
            "Confirmar",
            "🛡️"
        );
        if (!pin) return;
    }

    try {
        const res = await fetch('/api/system/token/rotate', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${serverInfo ? serverInfo.authToken : ''}`
            },
            body: JSON.stringify({ pin })
        });
        const data = await res.json();
        if (!res.ok || data.error) {
            appAlert(`Error: ${data.error || 'No se pudo rotar el token'}`, 'Fallo de Seguridad', '❌');
        } else {
            // Update in-memory credentials immediately
            if (serverInfo) serverInfo.authToken = data.token;
            const input = document.getElementById('security-token-input');
            if (input) input.value = data.token;
            const pairToken = document.getElementById('pair-token');
            if (pairToken) pairToken.textContent = data.token;

            // Update QR Code
            if (typeof updatePairQR === 'function') {
                updatePairQR();
            }

            showToast("✓ Token rotado exitosamente. Generado nuevo QR.");
        }
    } catch (e) {
        appAlert(`Error de red: ${e.message}`, 'Error de Comunicación', '❌');
    }
}

async function promptSetPIN(isChanging = false) {
    let currentPin = '';
    if (isChanging) {
        currentPin = await appPrompt(
            "Introduce tu PIN actual:",
            "",
            "Verificar PIN Actual",
            "Continuar",
            "🔑"
        );
        if (!currentPin) return;
    }

    const newPin = await appPrompt(
        "Introduce tu nuevo PIN (4 a 8 caracteres o dígitos):",
        "",
        isChanging ? "Cambiar PIN" : "Configurar PIN",
        "Guardar",
        "🔒"
    );
    if (!newPin || !newPin.trim()) return;

    try {
        const res = await fetch('/api/system/pin/set', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${serverInfo ? serverInfo.authToken : ''}`
            },
            body: JSON.stringify({ currentPin, newPin: newPin.trim() })
        });
        const data = await res.json();
        if (!res.ok || data.error) {
            appAlert(`Error: ${data.error || 'No se pudo configurar el PIN'}`, 'Error de PIN', '❌');
        } else {
            showToast(data.message || "✓ PIN configurado correctamente");
            await fetchSecurityStatus();
        }
    } catch (e) {
        appAlert(`Error de red: ${e.message}`, 'Error de Comunicación', '❌');
    }
}

async function promptRemovePIN() {
    const pin = await appPrompt(
        "Introduce tu PIN actual para confirmar su eliminación:",
        "",
        "Eliminar PIN de Seguridad",
        "Eliminar",
        "🗑️"
    );
    if (!pin) return;

    try {
        const res = await fetch('/api/system/pin/remove', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${serverInfo ? serverInfo.authToken : ''}`
            },
            body: JSON.stringify({ pin })
        });
        const data = await res.json();
        if (!res.ok || data.error) {
            appAlert(`Error: ${data.error || 'No se pudo eliminar el PIN'}`, 'Error de PIN', '❌');
        } else {
            showToast(data.message || "✓ PIN eliminado correctamente");
            await fetchSecurityStatus();
        }
    } catch (e) {
        appAlert(`Error de red: ${e.message}`, 'Error de Comunicación', '❌');
    }
}

// ==========================================
// Cockpit Lock Screen
// ==========================================
function lockCockpit() {
    const overlay = document.getElementById('cockpit-lock-overlay');
    const input = document.getElementById('lock-screen-input');
    const err = document.getElementById('lock-screen-error');
    if (!overlay) return;

    overlay.style.display = 'flex';
    if (input) {
        input.value = '';
        setTimeout(() => input.focus(), 100);
    }
    if (err) err.style.display = 'none';

    try {
        localStorage.setItem('orbit_cockpit_locked', 'true');
    } catch (_) {}
}

async function unlockCockpit() {
    const input = document.getElementById('lock-screen-input');
    const err = document.getElementById('lock-screen-error');
    const overlay = document.getElementById('cockpit-lock-overlay');
    if (!input || !overlay) return;

    const entered = input.value.trim();
    if (!entered) return;

    try {
        const res = await fetch('/api/system/pin/verify', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ pin: entered })
        });
        const data = await res.json();
        if (data.valid) {
            overlay.style.display = 'none';
            if (err) err.style.display = 'none';
            try {
                localStorage.removeItem('orbit_cockpit_locked');
            } catch (_) {}
            showToast("✓ Cockpit desbloqueado");
        } else {
            if (err) {
                err.style.display = 'block';
                err.textContent = 'PIN o credencial incorrecta. Inténtalo de nuevo.';
            }
            input.value = '';
            input.focus();
        }
    } catch (e) {
        if (err) {
            err.style.display = 'block';
            err.textContent = `Error de red: ${e.message}`;
        }
    }
}

// Auto-check lock on initial load
window.addEventListener('DOMContentLoaded', () => {
    try {
        if (localStorage.getItem('orbit_cockpit_locked') === 'true') {
            lockCockpit();
        }
    } catch (_) {}
});

// Shortcut Alt+L to lock cockpit
window.addEventListener('keydown', (e) => {
    if (e.altKey && (e.key === 'l' || e.key === 'L')) {
        e.preventDefault();
        lockCockpit();
    }
});

// ==========================================
// System Header Dropdown Menu & LAN Utilities
// ==========================================
function toggleSystemMenu(e) {
    if (e) {
        e.preventDefault();
        e.stopPropagation();
    }
    const wrapper = document.getElementById('sys-dropdown-wrapper');
    if (!wrapper) return;
    wrapper.classList.toggle('open');
}

function closeSystemMenu() {
    const wrapper = document.getElementById('sys-dropdown-wrapper');
    if (wrapper) wrapper.classList.remove('open');
}

// Global click outside to close dropdown
document.addEventListener('click', (e) => {
    const wrapper = document.getElementById('sys-dropdown-wrapper');
    if (wrapper && wrapper.classList.contains('open')) {
        if (!wrapper.contains(e.target)) {
            closeSystemMenu();
        }
    }
});

// Escape key closes system dropdown
window.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
        closeSystemMenu();
    }
});

function copyLanUrl() {
    if (!serverInfo) return;
    const primaryIp = (serverInfo.lanIps && serverInfo.lanIps.length > 0) ? serverInfo.lanIps[0] : window.location.hostname;
    const port = serverInfo.port || '8080';
    const url = `http://${primaryIp}:${port}`;
    navigator.clipboard.writeText(url);
    showToast("URL LAN copiada: " + url);
}



