// ==========================================
// Workspace & Bubble Management
// ==========================================

let allWorkspaces = [];

async function fetchWorkspaces() {
    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = token ? { 'Authorization': 'Bearer ' + token } : {};
        const res = await fetch('/api/workspaces', { headers });
        if (!res.ok) return;
        allWorkspaces = await res.json();
        renderWorkspaceTabs();
    } catch (e) {
        console.error("Error al obtener workspaces:", e);
    }
}

function getWorkspaceIconSvg(ws) {
    if (ws.id === 'aditum') {
        return `<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="vertical-align: -1px; margin-right: 4px;"><rect width="20" height="14" x="2" y="7" rx="2" ry="2"></rect><path d="M16 21V5a2 2 0 0 0-2-2h-4a2 2 0 0 0-2 2v16"></path></svg>`;
    } else if (ws.id === 'brif') {
        return `<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="vertical-align: -1px; margin-right: 4px;"><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"></polygon></svg>`;
    }
    return `<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="vertical-align: -1px; margin-right: 4px;"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path></svg>`;
}

function renderWorkspaceTabs() {
    const container = document.getElementById('ws-tabs-container');
    if (!container) return;

    let html = '';
    (allWorkspaces || []).forEach(ws => {
        const isActive = activeFilter === ws.id;
        const icon = getWorkspaceIconSvg(ws);
        html += `
            <button class="ws-tab-btn ${isActive ? 'active' : ''} ${escapeHtml(ws.id)}" data-ws="${escapeHtml(ws.id)}" onclick="setWorkspaceFilter('${escapeHtml(ws.id)}')" title="${escapeHtml(ws.name)} (${escapeHtml(ws.basePath)})">
                ${icon}${escapeHtml(ws.name)}
            </button>
        `;
    });

    const isAllActive = activeFilter === 'all';
    html += `
        <button class="ws-tab-btn ${isAllActive ? 'active' : ''}" data-ws="all" onclick="setWorkspaceFilter('all')">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="vertical-align: -1px; margin-right: 4px;"><circle cx="12" cy="12" r="10"></circle><line x1="2" y1="12" x2="22" y2="12"></line><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"></path></svg>Todos
        </button>
    `;

    html += `
        <button class="ws-tab-btn ws-tab-add" onclick="openWorkspacesModal()" title="Gestionar y Añadir Workspaces" style="color: var(--accent); font-weight: 700;">
            ⚙️ Proyectos
        </button>
    `;

    container.innerHTML = html;
}

// Fetch Bubbles
async function fetchBubbles() {
    try {
        if (!allWorkspaces || allWorkspaces.length === 0) {
            await fetchWorkspaces();
        }
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = token ? { 'Authorization': 'Bearer ' + token } : {};
        const res = await fetch('/api/bubbles', { headers });
        if (!res.ok) return;
        allBubbles = await res.json();
        allBubbles.sort((a, b) => (a.name || '').localeCompare(b.name || '', undefined, { numeric: true, sensitivity: 'base' }));
        renderBubblesList();
        // If a bubble was active, update its status
        if (currentBubble) {
            const updated = allBubbles.find(b => b.id === currentBubble.id);
            if (updated) {
                currentBubble = updated;
                updateBubbleHeaderUI();
            }
        }
    } catch (e) {
        console.error("Error al obtener bubbles:", e);
    }
}

let filterOnlyWithTerminals = false;

function setTaskTermFilter(onlyWithTerms) {
    filterOnlyWithTerminals = onlyWithTerms;
    document.getElementById('btn-filter-all-tasks')?.classList.toggle('active', !onlyWithTerms);
    document.getElementById('btn-filter-term-tasks')?.classList.toggle('active', onlyWithTerms);
    renderBubblesList();
}

function getBubbleSessions(b) {
    if (!b) return [];
    return allSessions
        .filter(s => s.bubbleId === b.id || (s.cwd && b.path && s.cwd.toLowerCase() === b.path.toLowerCase()))
        .sort((a, b) => {
            const tA = new Date(a.createdAt || 0).getTime();
            const tB = new Date(b.createdAt || 0).getTime();
            if (tA !== tB) return tA - tB;
            return (a.id || '').localeCompare(b.id || '');
        });
}

function setWorkspaceFilter(filter) {
    activeFilter = filter;
    document.querySelectorAll('#ws-tabs-container .ws-tab-btn').forEach(btn => {
        btn.classList.toggle('active', btn.getAttribute('data-ws') === filter);
    });
    renderBubblesList();
}

function renderBubblesList() {
    const container = document.getElementById('bubbles-container');
    if (!container) return;

    // Calculate total bubbles with open terminals across all bubbles
    const totalWithTerm = allBubbles.filter(b => getBubbleSessions(b).length > 0).length;
    const countWithTermEl = document.getElementById('count-with-term');
    if (countWithTermEl) countWithTermEl.textContent = totalWithTerm;

    const filtered = allBubbles.filter(b => {
        if (activeFilter !== 'all' && b.workspace !== activeFilter) {
            return false;
        }
        if (filterOnlyWithTerminals) {
            return getBubbleSessions(b).length > 0;
        }
        return true;
    });
    
    // Apply chosen sorting mode
    if (bubbleSortMode === 'priority') {
        filtered.sort((a, b) => {
            const wA = getPriorityInfo(a.priority).weight;
            const wB = getPriorityInfo(b.priority).weight;
            if (wA !== wB) return wA - wB;
            const rA = (typeof a.customRank === 'number' && a.customRank > 0) ? a.customRank : 99999;
            const rB = (typeof b.customRank === 'number' && b.customRank > 0) ? b.customRank : 99999;
            if (rA !== rB) return rA - rB;
            return (a.name || '').localeCompare(b.name || '', undefined, { numeric: true, sensitivity: 'base' });
        });
    } else if (bubbleSortMode === 'custom') {
        filtered.sort((a, b) => {
            const rA = (typeof a.customRank === 'number' && a.customRank > 0) ? a.customRank : 99999;
            const rB = (typeof b.customRank === 'number' && b.customRank > 0) ? b.customRank : 99999;
            if (rA !== rB) return rA - rB;
            return (a.name || '').localeCompare(b.name || '', undefined, { numeric: true, sensitivity: 'base' });
        });
    } else {
        // Default 'alpha'
        filtered.sort((a, b) => (a.name || '').localeCompare(b.name || '', undefined, { numeric: true, sensitivity: 'base' }));
    }

    const countTitle = filterOnlyWithTerminals ? `CON TERMINALES (${filtered.length})` : `TAREAS ACTIVAS (${filtered.length})`;
    const countEl = document.getElementById('sidebar-bubbles-count');
    if (countEl) countEl.textContent = countTitle;

    if (filtered.length === 0) {
        if (filterOnlyWithTerminals) {
            container.innerHTML = `
                <div style="padding: 24px 12px; text-align: center; color: var(--text-dim); font-size: 12px;">
                    <div style="font-size: 26px; margin-bottom: 8px;">💻</div>
                    No hay tareas con terminales abiertas ${activeFilter !== 'all' ? `en ${activeFilter}` : ''}.<br><br>
                    <button class="btn btn-sm btn-ghost" onclick="setTaskTermFilter(false)">Mostrar todas las tareas</button>
                </div>
            `;
        } else {
            container.innerHTML = `
                <div style="padding: 24px 12px; text-align: center; color: var(--text-dim); font-size: 12px;">
                    No hay tareas en este workspace.<br>
                    Haz clic en <strong>+ Nueva Tarea</strong> para comenzar.
                </div>
            `;
        }
        return;
    }

    container.innerHTML = filtered.map(b => {
        const isActive = currentBubble && currentBubble.id === b.id;
        const devRunning = b.devRunning;
        const isWorktree = b.isWorktree;
        const bSessions = getBubbleSessions(b);
        const hasTerms = bSessions.length > 0;
        const claudeCount = bSessions.filter(s => s.role === 'claude').length;
        const pInfo = getPriorityInfo(b.priority);
        const isCustom = bubbleSortMode === 'custom';

        return `
            <div class="bubble-card ${isActive ? 'active' : ''} ${isCustom ? 'is-draggable' : ''}" 
                 id="bubble-card-${b.id}"
                 draggable="${isCustom ? 'true' : 'false'}"
                 ondragstart="onBubbleDragStart(event, '${b.id}')"
                 ondragover="onBubbleDragOver(event)"
                 ondragleave="onBubbleDragLeave(event)"
                 ondrop="onBubbleDrop(event, '${b.id}')"
                 ondragend="onBubbleDragEnd(event)"
                 onclick="selectBubble('${b.id}')">
                <div class="bubble-card-top">
                    <div class="bubble-card-title">
                        ${isCustom ? `<span class="card-drag-handle" title="Arrastrar para ordenar">⠿</span>` : ''}
                        <span class="badge-ws ${b.workspace}">${b.workspace}</span>
                        <span class="bubble-name" title="${escapeHtml(b.name)}">${escapeHtml(b.name)}</span>
                    </div>
                    <div class="bubble-card-status-group">
                        <span class="badge-priority ${pInfo.class}" onclick="openPriorityPicker(event, '${b.id}')" title="Prioridad Notion: ${pInfo.label} (Clic para cambiar)">
                            ${pInfo.icon} ${b.priority || 'P-'}
                        </span>
                        <span class="badge-dev ${devRunning ? 'running' : 'stopped'}">
                            ${devRunning ? '● DEV' : '○ OFF'}
                        </span>
                    </div>
                </div>
                <div class="bubble-card-meta">
                    <span class="bubble-card-badges">
                        <span class="badge-port">:${b.port || (b.workspace === 'aditum' ? '8080' : '8000')}</span>
                        <span style="color: ${isWorktree ? 'var(--accent)' : 'var(--purple)'};">${isWorktree ? 'worktree' : 'linear'}</span>
                    </span>
                    <div class="bubble-card-meta-right">
                        ${hasTerms ? `
                            <span class="badge-term-active" title="${bSessions.length} terminal(es) abierta(s)">
                                💻 ${bSessions.length}${claudeCount > 0 ? ` 🤖${claudeCount}` : ''}
                            </span>
                        ` : ''}
                        <span class="bubble-card-branch" title="${escapeHtml(b.branch)}">${escapeHtml(b.branch)}</span>
                    </div>
                </div>
            </div>
        `;
    }).join('');
}

// Bubble Sort Modes & Controls
function setBubbleSortMode(mode) {
    bubbleSortMode = mode;
    localStorage.setItem('remotepc_bubble_sort_mode', mode);
    updateSortBarUI();
    renderBubblesList();
    if (mode === 'custom') {
        showToast("Modo manual: arrastra las tarjetas para ordenar");
    } else if (mode === 'priority') {
        showToast("Ordenado por prioridad de Notion");
    } else {
        showToast("Ordenado alfabéticamente (A-Z)");
    }
}

function updateSortBarUI() {
    ['alpha', 'priority', 'custom'].forEach(m => {
        const btn = document.getElementById(`sort-chip-${m}`);
        if (btn) btn.classList.toggle('active', bubbleSortMode === m);
    });
}

// Priority Picker
function openPriorityPicker(e, bubbleId) {
    if (e) {
        e.stopPropagation();
        e.preventDefault();
    }
    if (!bubbleId && currentBubble) {
        bubbleId = currentBubble.id;
    }
    if (!bubbleId) return;

    priorityPickerBubbleId = bubbleId;
    const menu = document.getElementById('priority-picker-menu');
    if (!menu) return;

    const b = allBubbles.find(x => x.id === bubbleId);
    const currentP = (b && b.priority) ? b.priority : '';

    menu.querySelectorAll('.priority-picker-item').forEach(item => {
        const itemOnClick = item.getAttribute('onclick') || '';
        item.classList.toggle('active', itemOnClick.includes(`'${currentP}'`));
    });

    menu.style.display = 'flex';
    const rect = (e && e.currentTarget) ? e.currentTarget.getBoundingClientRect() : { top: 120, left: 160, bottom: 140 };
    const menuWidth = 180;
    let left = rect.left;
    if (left + menuWidth > window.innerWidth - 10) {
        left = window.innerWidth - menuWidth - 10;
    }
    let top = rect.bottom + 4;
    if (top + 210 > window.innerHeight) {
        top = Math.max(10, rect.top - 210);
    }
    menu.style.left = `${Math.max(10, left)}px`;
    menu.style.top = `${Math.max(10, top)}px`;
}

function closePriorityPicker() {
    const menu = document.getElementById('priority-picker-menu');
    if (menu) menu.style.display = 'none';
    priorityPickerBubbleId = null;
}

async function selectTaskPriority(priority) {
    if (!priorityPickerBubbleId) return;
    const bId = priorityPickerBubbleId;
    closePriorityPicker();

    const target = allBubbles.find(x => x.id === bId);
    if (target) {
        target.priority = priority;
        if (currentBubble && currentBubble.id === bId) {
            currentBubble.priority = priority;
            updateBubbleHeaderUI();
        }
        renderBubblesList();
    }

    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': 'Bearer ' + token } : {})
        };
        const res = await fetch(`/api/bubbles/${bId}`, {
            method: 'PATCH',
            headers,
            body: JSON.stringify({ priority })
        });
        if (!res.ok) {
            console.error("Error setting priority:", await res.text());
            showToast("Error al guardar prioridad");
        } else {
            const updated = await res.json();
            if (target) {
                target.priority = updated.priority;
            }
            if (currentBubble && currentBubble.id === bId) {
                currentBubble.priority = updated.priority;
                updateBubbleHeaderUI();
            }
            renderBubblesList();
            showToast(`Prioridad ${priority || 'removida'} guardada`);
        }
    } catch (err) {
        console.error("Error setting priority:", err);
        showToast("Error al conectar con servidor");
    }
}

// Drag and Drop for Custom Sorting
function onBubbleDragStart(e, bubbleId) {
    if (bubbleSortMode !== 'custom') return;
    draggedBubbleId = bubbleId;
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData('text/plain', bubbleId);
    const card = document.getElementById(`bubble-card-${bubbleId}`);
    if (card) {
        setTimeout(() => card.classList.add('dragging'), 0);
    }
}

function onBubbleDragOver(e) {
    if (bubbleSortMode !== 'custom' || !draggedBubbleId) return;
    e.preventDefault();
    e.dataTransfer.dropEffect = 'move';
    const card = e.currentTarget;
    if (card && !card.classList.contains('dragging')) {
        card.classList.add('drag-over');
    }
}

function onBubbleDragLeave(e) {
    const card = e.currentTarget;
    if (card) {
        card.classList.remove('drag-over');
    }
}

function onBubbleDragEnd(e) {
    document.querySelectorAll('.bubble-card').forEach(el => {
        el.classList.remove('dragging', 'drag-over');
    });
    draggedBubbleId = null;
}

async function onBubbleDrop(e, targetBubbleId) {
    if (bubbleSortMode !== 'custom' || !draggedBubbleId) return;
    e.preventDefault();
    e.stopPropagation();

    const sourceId = draggedBubbleId;
    document.querySelectorAll('.bubble-card').forEach(el => {
        el.classList.remove('dragging', 'drag-over');
    });
    draggedBubbleId = null;

    if (sourceId === targetBubbleId) return;

    // Visible filtered list currently displayed
    const visible = allBubbles.filter(b => {
        if (activeFilter !== 'all' && b.workspace !== activeFilter) return false;
        if (filterOnlyWithTerminals) return getBubbleSessions(b).length > 0;
        return true;
    });

    // Sort visible in their current order
    visible.sort((a, b) => {
        const rA = (typeof a.customRank === 'number' && a.customRank > 0) ? a.customRank : 99999;
        const rB = (typeof b.customRank === 'number' && b.customRank > 0) ? b.customRank : 99999;
        if (rA !== rB) return rA - rB;
        return (a.name || '').localeCompare(b.name || '', undefined, { numeric: true, sensitivity: 'base' });
    });

    const sourceIdx = visible.findIndex(b => b.id === sourceId);
    const targetIdx = visible.findIndex(b => b.id === targetBubbleId);
    if (sourceIdx === -1 || targetIdx === -1) return;

    const moved = visible.splice(sourceIdx, 1)[0];
    visible.splice(targetIdx, 0, moved);

    // Reassign ranks
    visible.forEach((b, idx) => {
        b.customRank = idx + 1;
    });

    renderBubblesList();

    // Persist order to backend
    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': 'Bearer ' + token } : {})
        };
        const order = visible.map(b => b.id);
        const res = await fetch('/api/bubbles/reorder', {
            method: 'POST',
            headers,
            body: JSON.stringify({ order })
        });
        if (!res.ok) {
            console.error("Error saving bubble order:", await res.text());
            showToast("Error al guardar orden en servidor");
        } else {
            showToast("Orden personalizado guardado");
        }
    } catch (err) {
        console.error("Error persisting order:", err);
    }
}

document.addEventListener('click', (e) => {
    const menu = document.getElementById('priority-picker-menu');
    if (menu && menu.style.display === 'flex' && !menu.contains(e.target) && !e.target.closest('.badge-priority')) {
        closePriorityPicker();
    }
});

// Select Bubble
async function selectBubble(id) {
    const b = allBubbles.find(x => x.id === id);
    if (!b) return;
    currentBubble = b;
    renderBubblesList();

    document.getElementById('workspace-empty-view').style.display = 'none';
    const activeCont = document.getElementById('active-bubble-container');
    activeCont.style.display = 'flex';

    updateBubbleHeaderUI();

    // Load subtab sessions for this bubble
    await updateBubbleSessionsUI();

    // Auto-switch preview url
    const port = b.port || (b.workspace === 'aditum' ? 8080 : 8000);
    document.getElementById('preview-url-input').value = `http://localhost:${port}/`;
    if (activeTab === 'preview') {
        reloadPreviewFrame();
    } else if (activeTab === 'git') {
        loadGitStatus();
    } else if (activeTab === 'obsidian') {
        switchObsidianVault(b.workspace === 'aditum' ? 'aditum' : 'brif');
    }
}

function updateBubbleHeaderUI() {
    if (!currentBubble) return;
    document.getElementById('active-bubble-ws-badge').textContent = currentBubble.workspace.toUpperCase();
    document.getElementById('active-bubble-ws-badge').className = `badge-ws ${currentBubble.workspace}`;
    document.getElementById('active-bubble-title').textContent = currentBubble.name;
    document.getElementById('active-bubble-branch-name').textContent = currentBubble.branch || 'main';

    const prioBadge = document.getElementById('active-bubble-priority');
    if (prioBadge) {
        const pInfo = getPriorityInfo(currentBubble.priority);
        prioBadge.className = `badge-priority ${pInfo.class}`;
        prioBadge.innerHTML = `${pInfo.icon} ${currentBubble.priority ? pInfo.label : 'Prioridad'}`;
    }

    const port = currentBubble.port || (currentBubble.workspace === 'aditum' ? 8080 : 8000);
    document.getElementById('active-bubble-port').textContent = port;

    const schemaChip = document.getElementById('active-bubble-schema');
    if (currentBubble.dbSchema) {
        schemaChip.style.display = 'inline-flex';
        document.getElementById('active-bubble-schema-name').textContent = currentBubble.dbSchema;
    } else {
        schemaChip.style.display = 'none';
    }

    const devBtn = document.getElementById('btn-bubble-dev-toggle');
    if (currentBubble.devRunning) {
        devBtn.className = "btn btn-danger btn-sm";
        devBtn.textContent = `⏹ Detener Dev (PID ${currentBubble.devPid})`;
    } else {
        devBtn.className = "btn btn-success btn-sm";
        devBtn.textContent = `▶ Iniciar Dev (: ${port})`;
    }

    const celeryDropdown = document.getElementById('brif-celery-dropdown');
    if (celeryDropdown) {
        celeryDropdown.style.display = (currentBubble.workspace === 'brif') ? 'block' : 'none';
    }
    updateQuickActionsUI();
}

// Switch Workspace Tabs
function switchWorkspaceTab(tab) {
    if (tab === 'term') tab = 'terminal';
    activeTab = tab;
    document.querySelectorAll('.tab-btn').forEach(btn => btn.classList.remove('active'));
    document.querySelectorAll('.tab-pane').forEach(p => p.classList.remove('active'));

    const btn = document.getElementById(`tab-btn-${tab}`) || document.getElementById('tab-btn-term');
    if (btn) btn.classList.add('active');

    const pane = document.getElementById(`tab-pane-${tab}`) || document.getElementById('tab-pane-terminal');
    if (pane) pane.classList.add('active');

    if (tab === 'terminal') {
        setTimeout(fitActiveTerminal, 100);
    } else if (tab === 'preview') {
        reloadPreviewFrame();
    } else if (tab === 'git') {
        loadGitStatus();
    } else if (tab === 'obsidian') {
        if (currentBubble) {
            switchObsidianVault(currentBubble.workspace === 'aditum' ? 'aditum' : 'brif');
        } else {
            switchObsidianVault('aditum');
        }
    } else if (tab === 'calendar') {
        loadCalendarView();
    }
}

// Fetch Sessions


// Delete Bubble
async function confirmDeleteBubble() {
    if (!currentBubble) return;
    const isAditum = currentBubble.workspace === 'aditum';
    const msg = isAditum
        ? `¿Eliminar worktree ${currentBubble.name}?\nSe ejecutará remove-worktree.sh y se limpiará el esquema MySQL asignado.`
        : `¿Eliminar bubble ${currentBubble.name}?`;

    const ok = await appConfirm(msg, "Eliminar Tarea", "Eliminar Worktree", "Cancelar", true, "🗑️");
    if (!ok) return;

    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = token ? { 'Authorization': 'Bearer ' + token } : {};
        const res = await fetch(`/api/bubbles/${currentBubble.id}`, { method: 'DELETE', headers });
        if (res.ok) {
            showToast("Worktree eliminado.");
            currentBubble = null;
            document.getElementById('active-bubble-container').style.display = 'none';
            document.getElementById('workspace-empty-view').style.display = 'flex';
            await fetchBubbles();
        } else {
            appAlert("Error al eliminar worktree: " + (await res.text()), "Error", "❌");
        }
    } catch (e) {
        appAlert("Error de red: " + e, "Error de Red", "❌");
    }
}

// New Bubble Wizard Modal
let wizardWs = 'aditum';

function renderWizardWorkspaceTabs() {
    const container = document.getElementById('wizard-ws-tabs');
    if (!container) return;

    if (!allWorkspaces || allWorkspaces.length === 0) {
        container.innerHTML = `<div style="padding: 6px; font-size: 11px; color: var(--text-dim);">No hay workspaces disponibles</div>`;
        return;
    }

    container.innerHTML = allWorkspaces.map(ws => {
        const isActive = wizardWs === ws.id;
        const icon = getWorkspaceIconSvg(ws);
        return `
            <button type="button" class="ws-tab-btn ${isActive ? 'active' : ''} ${escapeHtml(ws.id)}" id="wizard-tab-${escapeHtml(ws.id)}" onclick="setWizardWorkspace('${escapeHtml(ws.id)}')">
                ${icon}${escapeHtml(ws.name)}
            </button>
        `;
    }).join('');
}

let workspaceBranchesCache = {};
let discoveredSqlDumps = [];

async function loadWorkspaceBranches(wsId) {
    const targetWs = (wsId === 'generic') ? wizardWs : wsId;
    if (!targetWs) return;
    const loadingEl = document.getElementById(`${wsId}-branch-loading`) || document.getElementById(`${targetWs}-branch-loading`);
    if (loadingEl) loadingEl.style.display = 'inline';
    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = token ? { 'Authorization': 'Bearer ' + token } : {};
        const res = await fetch(`/api/workspaces/${encodeURIComponent(targetWs)}/branches`, { headers });
        if (!res.ok) return;
        const branches = await res.json();
        workspaceBranchesCache[targetWs] = branches || [];
        if (wsId === 'generic') {
            workspaceBranchesCache['generic'] = branches || [];
        }
        filterBranches(wsId);
    } catch (e) {
        console.error("Error al cargar ramas:", e);
    } finally {
        if (loadingEl) loadingEl.style.display = 'none';
    }
}

function filterBranches(wsId) {
    const input = document.getElementById(`${wsId}-branch-search`);
    const dropdown = document.getElementById(`${wsId}-branch-dropdown`);
    if (!dropdown) return;

    const targetWs = (wsId === 'generic') ? wizardWs : wsId;
    const branches = workspaceBranchesCache[targetWs] || workspaceBranchesCache[wsId];

    if (!branches) {
        dropdown.innerHTML = `<div style="padding: 10px; color: var(--text-dim); font-size: 12px; text-align: center;"><span class="orbit-spinner" style="display:inline-block; margin-right: 6px;">⏳</span> Cargando ramas del repositorio...</div>`;
        dropdown.style.display = 'block';
        loadWorkspaceBranches(wsId);
        return;
    }

    const query = (input?.value || '').trim().toLowerCase();
    const filtered = query ? branches.filter(b => (b.name || '').toLowerCase().includes(query)) : branches;

    if (filtered.length === 0) {
        dropdown.innerHTML = `
            <div style="padding: 12px; text-align: center; color: var(--text-dim); font-size: 12px;">
                No se encontraron ramas coincidentes con "<strong>${escapeHtml(query)}</strong>"
            </div>
        `;
        dropdown.style.display = 'block';
        return;
    }

    const slice = filtered.slice(0, 100);
    let html = slice.map(b => {
        const isRemote = !!b.isRemote;
        const tagText = isRemote ? 'origin' : 'local';
        const tagBg = isRemote ? 'rgba(74, 158, 255, 0.15)' : 'rgba(0, 255, 150, 0.15)';
        const tagColor = isRemote ? '#4a9eff' : 'var(--accent)';
        const safeBranch = b.name.replace(/\\/g, '\\\\').replace(/'/g, "\\'");
        return `
            <div class="branch-item" onclick="chooseBranch('${wsId}', '${safeBranch}')">
                <span style="font-family: var(--font-mono); font-weight: 500; color: var(--text-main); word-break: break-all;">${escapeHtml(b.name)}</span>
                <span style="font-size: 10px; padding: 2px 6px; border-radius: 3px; font-weight: 600; background: ${tagBg}; color: ${tagColor};">${tagText}</span>
            </div>
        `;
    }).join('');

    if (filtered.length > 100) {
        html += `
            <div style="padding: 8px 12px; text-align: center; font-size: 11px; color: var(--text-dim); background: var(--bg-hover);">
                Mostrando primeros 100 de ${filtered.length} resultados. Escribe más para afinar.
            </div>
        `;
    }

    dropdown.innerHTML = html;
    dropdown.style.display = 'block';
}

function showBranchDropdown(wsId) {
    filterBranches(wsId);
}

function chooseBranch(wsId, branchName) {
    const hiddenInput = document.getElementById(`${wsId}-existing-branch-select`);
    if (hiddenInput) hiddenInput.value = branchName;

    const searchInput = document.getElementById(`${wsId}-branch-search`);
    if (searchInput) searchInput.value = branchName;

    const badge = document.getElementById(`${wsId}-branch-selected-badge`);
    const nameEl = document.getElementById(`${wsId}-branch-selected-name`);
    if (badge && nameEl) {
        nameEl.textContent = branchName;
        badge.style.display = 'flex';
    }

    const dropdown = document.getElementById(`${wsId}-branch-dropdown`);
    if (dropdown) dropdown.style.display = 'none';

    // Auto-fill Task ID if empty or updating
    if (wsId === 'aditum') {
        const taskIdInput = document.getElementById('aditum-task-id');
        if (taskIdInput) {
            taskIdInput.value = branchName;
            updateAditumPortPreview();
        }
    } else if (wsId === 'brif') {
        const taskIdInput = document.getElementById('brif-task-id');
        if (taskIdInput) taskIdInput.value = branchName;
    } else {
        const taskIdInput = document.getElementById('generic-task-id');
        if (taskIdInput) taskIdInput.value = branchName;
    }
}

function clearSelectedBranch(wsId) {
    const hiddenInput = document.getElementById(`${wsId}-existing-branch-select`);
    if (hiddenInput) hiddenInput.value = '';

    const searchInput = document.getElementById(`${wsId}-branch-search`);
    if (searchInput) searchInput.value = '';

    const badge = document.getElementById(`${wsId}-branch-selected-badge`);
    if (badge) badge.style.display = 'none';

    const dropdown = document.getElementById(`${wsId}-branch-dropdown`);
    if (dropdown) dropdown.style.display = 'none';
}

function toggleBranchSource(wsId) {
    const isExisting = document.querySelector(`input[name="${wsId}-branch-type"][value="existing"]`)?.checked;
    const existingContainer = document.getElementById(`${wsId}-existing-branch-container`);
    const newContainer = document.getElementById(`${wsId}-new-branch-container`);
    if (existingContainer) existingContainer.style.display = isExisting ? 'block' : 'none';
    if (newContainer) newContainer.style.display = isExisting ? 'none' : 'block';

    const labelExisting = document.getElementById(`${wsId}-branch-type-label-existing`);
    const labelNew = document.getElementById(`${wsId}-branch-type-label-new`);
    if (labelExisting) labelExisting.classList.toggle('active', !!isExisting);
    if (labelNew) labelNew.classList.toggle('active', !isExisting);

    if (isExisting) {
        const targetWs = (wsId === 'generic') ? wizardWs : wsId;
        if (!workspaceBranchesCache[targetWs] || workspaceBranchesCache[targetWs].length === 0) {
            loadWorkspaceBranches(wsId);
        } else {
            showBranchDropdown(wsId);
        }
    }
}

// Global click-outside listener to dismiss branch dropdowns
document.addEventListener('click', (e) => {
    ['aditum', 'brif', 'generic'].forEach(wsId => {
        const container = document.getElementById(`${wsId}-existing-branch-container`);
        const dropdown = document.getElementById(`${wsId}-branch-dropdown`);
        if (dropdown && dropdown.style.display !== 'none' && container && !container.contains(e.target)) {
            dropdown.style.display = 'none';
        }
    });
});

async function loadSqlDumps() {
    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = token ? { 'Authorization': 'Bearer ' + token } : {};
        const res = await fetch('/api/system/sql-dumps', { headers });
        if (!res.ok) return;
        discoveredSqlDumps = await res.json();
        populateSqlDumpsSelect();
    } catch (e) {
        console.error("Error al obtener dumps SQL:", e);
    }
}

function populateSqlDumpsSelect() {
    const select = document.getElementById('aditum-detected-dumps-select');
    if (!select) return;
    let html = `<option value="">-- Seleccionar archivo dump en Descargas / Work (${discoveredSqlDumps.length}) --</option>`;
    discoveredSqlDumps.forEach(d => {
        html += `<option value="${escapeHtml(d.path)}">${escapeHtml(d.name)} (${escapeHtml(d.sizeStr)})</option>`;
    });
    select.innerHTML = html;
}

function onSelectDetectedDump() {
    const select = document.getElementById('aditum-detected-dumps-select');
    const path = select?.value;
    if (path) {
        const fileInput = document.getElementById('aditum-custom-dump-file');
        if (fileInput) fileInput.value = path;
    }
}

function toggleAditumSeedCustom() {
    const mode = document.getElementById('aditum-seed-mode').value;
    const dumpContainer = document.getElementById('aditum-custom-dump-container');
    const schemaContainer = document.getElementById('aditum-custom-schema-container');
    const hint = document.getElementById('aditum-seed-hint');

    if (dumpContainer) dumpContainer.style.display = (mode === 'custom_dump') ? 'block' : 'none';
    if (schemaContainer) schemaContainer.style.display = (mode === 'custom_schema') ? 'block' : 'none';

    if (mode === 'custom_dump') {
        hint.textContent = "Se creará el schema y se importará directamente el archivo .sql o .sql.gz seleccionado.";
        if (!discoveredSqlDumps || discoveredSqlDumps.length === 0) {
            loadSqlDumps();
        }
    } else if (mode === 'custom_schema') {
        hint.textContent = "Se clonará la estructura y datos desde otro schema MySQL existente en localhost:3306.";
    } else if (mode === 'slim') {
        hint.textContent = "⭐ Clonado ultra rápido de aditum_slim (~25s) para desarrollo cotidiano.";
    } else {
        hint.textContent = "Clonado completo de la base de datos aditum viva (~3.5 GB, 3-10 min).";
    }
}

function openNewBubbleModal(prefWs = null) {
    if (prefWs) {
        wizardWs = prefWs;
    } else if (activeFilter !== 'all') {
        wizardWs = activeFilter;
    } else if (allWorkspaces && allWorkspaces.length > 0) {
        wizardWs = allWorkspaces[0].id;
    } else {
        wizardWs = 'aditum';
    }

    renderWizardWorkspaceTabs();
    setWizardWorkspace(wizardWs);
    suggestNextAditumIndex();

    // Reset radio buttons, branch selections and dropdowns (default to existing branch search)
    ['aditum', 'brif', 'generic'].forEach(id => {
        clearSelectedBranch(id);
        const radioExisting = document.querySelector(`input[name="${id}-branch-type"][value="existing"]`);
        if (radioExisting) radioExisting.checked = true;
        toggleBranchSource(id);
    });

    // Reset seed mode
    const seedMode = document.getElementById('aditum-seed-mode');
    if (seedMode) {
        seedMode.value = 'slim';
        toggleAditumSeedCustom();
    }

    // Preload branches in background
    loadWorkspaceBranches(wizardWs);

    document.getElementById('modal-new-bubble').classList.add('open');
}

function closeNewBubbleModal() {
    document.getElementById('modal-new-bubble').classList.remove('open');
}

function setWizardWorkspace(wsId) {
    wizardWs = wsId;
    document.querySelectorAll('#wizard-ws-tabs .ws-tab-btn').forEach(btn => {
        btn.classList.toggle('active', btn.id === `wizard-tab-${wsId}`);
    });

    const fieldsAditum = document.getElementById('wizard-fields-aditum');
    const fieldsBrif = document.getElementById('wizard-fields-brif');
    const fieldsGeneric = document.getElementById('wizard-fields-generic');

    if (fieldsAditum) fieldsAditum.style.display = (wsId === 'aditum') ? 'block' : 'none';
    if (fieldsBrif) fieldsBrif.style.display = (wsId === 'brif') ? 'block' : 'none';
    if (fieldsGeneric) fieldsGeneric.style.display = (wsId !== 'aditum' && wsId !== 'brif') ? 'block' : 'none';

    if (wsId !== 'aditum' && wsId !== 'brif') {
        const ws = (allWorkspaces || []).find(w => w.id === wsId);
        if (ws) {
            const baseBranchEl = document.getElementById('generic-base-branch');
            if (baseBranchEl && ws.defaultBase) baseBranchEl.value = ws.defaultBase;
            const portEl = document.getElementById('generic-port');
            if (portEl && ws.defaultPort) portEl.value = ws.defaultPort;
        }
    }

    loadWorkspaceBranches(wsId);
}

function suggestNextAditumIndex() {
    // Check used ports in allBubbles
    const usedPorts = new Set(allBubbles.filter(b => b.workspace === 'aditum').map(b => b.port));
    for (let i = 1; i <= 99; i++) {
        if (!usedPorts.has(8080 + i)) {
            document.getElementById('aditum-index').value = i;
            updateAditumPortPreview();
            break;
        }
    }
}

function updateAditumPortPreview() {
    const idx = parseInt(document.getElementById('aditum-index').value) || 1;
    document.getElementById('aditum-port-preview').textContent = 8080 + idx;
}

function toggleBrifModeHelp() {
    const mode = document.getElementById('brif-mode').value;
    const hint = document.getElementById('brif-mode-hint');
    if (mode === 'linear') {
        hint.textContent = "Trabajo directo y ágil en tu repositorio principal (brif-scrapper) sin carpetas extra.";
    } else {
        hint.textContent = "Crea una carpeta worktree aislada (brif-scrapper-<slug>) para trabajar en paralelo.";
    }
}

function toggleGenericModeHelp() {
    const mode = document.getElementById('generic-mode')?.value;
    const hint = document.getElementById('generic-mode-hint');
    if (!hint) return;
    if (mode === 'worktree') {
        hint.textContent = "Crea una carpeta de trabajo independiente (.worktrees/<taskId>) para desarrollo en paralelo.";
    } else {
        hint.textContent = "Cambia de rama directamente en la carpeta principal del repositorio.";
    }
}

async function submitCreateBubble() {
    const btn = document.getElementById('btn-submit-bubble');
    btn.disabled = true;
    btn.textContent = "Creando... Por favor espera";

    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': 'Bearer ' + token } : {})
        };

        let body = {};
        if (wizardWs === 'aditum') {
            const isExistingBranch = document.querySelector('input[name="aditum-branch-type"][value="existing"]')?.checked;
            const existingBranch = document.getElementById('aditum-existing-branch-select')?.value.trim();
            const taskId = document.getElementById('aditum-task-id').value.trim() || existingBranch;
            if (!taskId) {
                appAlert("Por favor ingresa un ID de tarea (ej. TAR-1130) o selecciona una rama existente", "Campo Requerido", "⚠️");
                btn.disabled = false;
                btn.textContent = "⚡ Crear y Configurar Bubble";
                return;
            }
            const index = parseInt(document.getElementById('aditum-index').value) || 1;
            const baseBranch = document.getElementById('aditum-base-branch').value.trim() || 'development';
            const seedMode = document.getElementById('aditum-seed-mode').value;

            body = {
                workspace: 'aditum',
                taskId: taskId,
                index: index,
                baseBranch: baseBranch,
                useSlim: seedMode === 'slim'
            };

            if (isExistingBranch && existingBranch) {
                body.cloneBranch = existingBranch;
            }
            if (seedMode === 'custom_dump') {
                const dumpFile = document.getElementById('aditum-custom-dump-file').value.trim();
                if (!dumpFile) {
                    appAlert("Por favor selecciona o especifica la ruta del dump SQL", "Dump Requerido", "⚠️");
                    btn.disabled = false;
                    btn.textContent = "⚡ Crear y Configurar Bubble";
                    return;
                }
                body.seedDump = dumpFile;
            } else if (seedMode === 'custom_schema') {
                const schema = document.getElementById('aditum-custom-schema-input').value.trim();
                if (schema) body.seedSchema = schema;
            }
        } else if (wizardWs === 'brif') {
            const isExistingBranch = document.querySelector('input[name="brif-branch-type"][value="existing"]')?.checked;
            const existingBranch = document.getElementById('brif-existing-branch-select')?.value.trim();
            const taskId = document.getElementById('brif-task-id').value.trim() || existingBranch;
            if (!taskId) {
                appAlert("Por favor ingresa un nombre de tarea o selecciona una rama", "Campo Requerido", "⚠️");
                btn.disabled = false;
                btn.textContent = "⚡ Crear y Configurar Bubble";
                return;
            }
            const isWorktree = document.getElementById('brif-mode').value === 'worktree';
            const baseBranch = document.getElementById('brif-base-branch').value.trim() || 'develop';

            body = {
                workspace: 'brif',
                taskId: taskId,
                isWorktree: isWorktree,
                baseBranch: baseBranch
            };
            if (isExistingBranch && existingBranch) {
                body.cloneBranch = existingBranch;
            }
        } else {
            // Generic workspace
            const isExistingBranch = document.querySelector('input[name="generic-branch-type"][value="existing"]')?.checked;
            const existingBranch = document.getElementById('generic-existing-branch-select')?.value.trim();
            const taskId = document.getElementById('generic-task-id').value.trim() || existingBranch;
            if (!taskId) {
                appAlert("Por favor ingresa un identificador de tarea o selecciona una rama", "Campo Requerido", "⚠️");
                btn.disabled = false;
                btn.textContent = "⚡ Crear y Configurar Bubble";
                return;
            }
            const name = document.getElementById('generic-task-name').value.trim();
            const isWorktree = document.getElementById('generic-mode').value === 'worktree';
            const baseBranch = document.getElementById('generic-base-branch').value.trim() || 'main';
            const port = parseInt(document.getElementById('generic-port').value) || 0;

            body = {
                workspace: wizardWs,
                taskId: taskId,
                name: name,
                isWorktree: isWorktree,
                baseBranch: baseBranch,
                port: port
            };
            if (isExistingBranch && existingBranch) {
                body.cloneBranch = existingBranch;
            }
        }

        const res = await fetch('/api/bubbles', { method: 'POST', headers, body: JSON.stringify(body) });
        if (res.ok) {
            const created = await res.json();
            closeNewBubbleModal();
            showToast(`Bubble ${created.name} creada exitosamente!`);
            await fetchBubbles();
            selectBubble(created.id);
        } else {
            appAlert("Error al crear bubble: " + (await res.text()), "Error al Crear Tarea", "❌");
        }
    } catch (e) {
        appAlert("Error de red: " + e, "Error de Conexión", "❌");
    } finally {
        btn.disabled = false;
        btn.textContent = "⚡ Crear y Configurar Bubble";
    }
}

// ==========================================
// Workspace Manager Modal & CRUD
// ==========================================

async function openWorkspacesModal() {
    await fetchWorkspaces();
    renderWorkspacesListInModal();
    resetWorkspaceForm();
    const modal = document.getElementById('modal-workspaces');
    if (modal) modal.classList.add('open');
}

function closeWorkspacesModal() {
    const modal = document.getElementById('modal-workspaces');
    if (modal) modal.classList.remove('open');
}

function renderWorkspacesListInModal() {
    const container = document.getElementById('ws-list-container');
    const countEl = document.getElementById('ws-count-label');
    if (countEl) countEl.textContent = `${(allWorkspaces || []).length} workspaces`;
    if (!container) return;

    if (!allWorkspaces || allWorkspaces.length === 0) {
        container.innerHTML = `<div style="padding: 16px; text-align: center; color: var(--text-dim); font-size: 12px;">No hay workspaces configurados. Añade uno con el formulario debajo.</div>`;
        return;
    }

    container.innerHTML = allWorkspaces.map(ws => {
        const isBuiltin = ws.id === 'aditum' || ws.id === 'brif';
        return `
            <div style="background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-sm); padding: 12px 14px; display: flex; justify-content: space-between; align-items: center; gap: 12px;">
                <div style="flex: 1; min-width: 0;">
                    <div style="display: flex; align-items: center; gap: 8px;">
                        <strong style="color: var(--text-main); font-size: 13px;">${escapeHtml(ws.name)}</strong>
                        <span class="badge-ws ${escapeHtml(ws.id)}" style="font-size: 10px; padding: 1px 6px;">${escapeHtml(ws.id)}</span>
                        ${ws.defaultPort ? `<span style="font-size: 11px; color: var(--accent); font-weight: 600;">:${escapeHtml(String(ws.defaultPort))}</span>` : ''}
                    </div>
                    <div style="font-size: 11px; color: var(--text-muted); font-family: monospace; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; margin-top: 3px;" title="${escapeHtml(ws.basePath)}">
                        📁 ${escapeHtml(ws.basePath)}
                    </div>
                    ${ws.repoUrl ? `<div style="font-size: 11px; color: var(--accent); margin-top: 2px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;" title="${escapeHtml(ws.repoUrl)}">🔗 ${escapeHtml(ws.repoUrl)}</div>` : ''}
                    <div style="font-size: 11px; color: var(--text-dim); margin-top: 2px; display: flex; gap: 12px;">
                        <span>Rama base: <code style="color: var(--text-main);">${escapeHtml(ws.defaultBase || 'main')}</code></span>
                        ${ws.devCommand ? `<span>Dev: <code style="color: var(--green);">${escapeHtml(ws.devCommand)}</code></span>` : ''}
                    </div>
                </div>
                <div style="display: flex; gap: 6px;">
                    <button class="btn btn-sm btn-ghost" onclick="editWorkspace('${escapeHtml(ws.id)}')" title="Editar configuración">
                        ✏️ Editar
                    </button>
                    <button class="btn btn-sm btn-danger" onclick="confirmDeleteWorkspace('${escapeHtml(ws.id)}', '${escapeHtml(ws.name)}')" title="Eliminar este workspace" ${isBuiltin ? 'style="opacity: 0.5;"' : ''}>
                        🗑️
                    </button>
                </div>
            </div>
        `;
    }).join('');
}

function resetWorkspaceForm() {
    document.getElementById('ws-form-edit-id').value = '';
    document.getElementById('ws-form-title').textContent = '➕ Añadir Nuevo Workspace';
    document.getElementById('ws-input-name').value = '';
    document.getElementById('ws-input-id').value = '';
    document.getElementById('ws-input-path').value = '';
    document.getElementById('ws-input-repo').value = '';
    document.getElementById('ws-input-base').value = 'main';
    document.getElementById('ws-input-port').value = '';
    document.getElementById('ws-input-cmd').value = '';
    document.getElementById('ws-input-desc').value = '';
    document.getElementById('ws-cancel-edit-btn').style.display = 'none';
    document.getElementById('ws-save-btn').textContent = '💾 Guardar Workspace';
}

function editWorkspace(id) {
    const ws = (allWorkspaces || []).find(w => w.id === id);
    if (!ws) return;

    document.getElementById('ws-form-edit-id').value = ws.id;
    document.getElementById('ws-form-title').textContent = `✏️ Editar Workspace: ${ws.name}`;
    document.getElementById('ws-input-name').value = ws.name || '';
    document.getElementById('ws-input-id').value = ws.id || '';
    document.getElementById('ws-input-path').value = ws.basePath || '';
    document.getElementById('ws-input-repo').value = ws.repoUrl || '';
    document.getElementById('ws-input-base').value = ws.defaultBase || 'main';
    document.getElementById('ws-input-port').value = ws.defaultPort || '';
    document.getElementById('ws-input-cmd').value = ws.devCommand || '';
    document.getElementById('ws-input-desc').value = ws.description || '';
    document.getElementById('ws-cancel-edit-btn').style.display = 'inline-block';
    document.getElementById('ws-save-btn').textContent = '💾 Actualizar Workspace';

    document.getElementById('ws-input-name').focus();
}

async function saveWorkspaceFromForm() {
    const editId = document.getElementById('ws-form-edit-id').value.trim();
    const name = document.getElementById('ws-input-name').value.trim();
    const id = document.getElementById('ws-input-id').value.trim() || name.toLowerCase().replace(/[^a-z0-9_-]/g, '-');
    const basePath = document.getElementById('ws-input-path').value.trim();
    const repoUrl = document.getElementById('ws-input-repo').value.trim();
    const defaultBase = document.getElementById('ws-input-base').value.trim() || 'main';
    const defaultPort = parseInt(document.getElementById('ws-input-port').value) || 0;
    const devCommand = document.getElementById('ws-input-cmd').value.trim();
    const description = document.getElementById('ws-input-desc').value.trim();

    if (!name) {
        appAlert("Por favor ingresa un nombre para el workspace.", "Campo Requerido", "⚠️");
        return;
    }
    if (!basePath) {
        appAlert("Por favor ingresa la ruta del repositorio en disco.", "Campo Requerido", "⚠️");
        return;
    }

    const payload = {
        id: id,
        name: name,
        type: id,
        basePath: basePath,
        repoUrl: repoUrl,
        defaultBase: defaultBase,
        defaultPort: defaultPort,
        devCommand: devCommand,
        description: description
    };

    const token = serverInfo ? serverInfo.authToken : '';
    const headers = {
        'Content-Type': 'application/json',
        ...(token ? { 'Authorization': 'Bearer ' + token } : {})
    };

    try {
        let res;
        if (editId) {
            res = await fetch(`/api/workspaces/${encodeURIComponent(editId)}`, {
                method: 'PUT',
                headers,
                body: JSON.stringify(payload)
            });
        } else {
            res = await fetch('/api/workspaces', {
                method: 'POST',
                headers,
                body: JSON.stringify(payload)
            });
        }

        if (res.ok) {
            if (repoUrl) {
                try {
                    await fetch(`/api/workspaces/${encodeURIComponent(editId || id)}/link-repo`, {
                        method: 'POST',
                        headers,
                        body: JSON.stringify({ repoUrl: repoUrl })
                    });
                } catch (e) {
                    console.warn("Error vinculando remote origin:", e);
                }
            }
            showToast(editId ? "Workspace actualizado con éxito." : "Workspace añadido con éxito.");
            resetWorkspaceForm();
            await fetchWorkspaces();
            renderWorkspacesListInModal();
            await fetchBubbles();
        } else {
            const err = await res.text();
            appAlert("Error al guardar workspace: " + err, "Error", "❌");
        }
    } catch (e) {
        appAlert("Error de red: " + e, "Error de Red", "❌");
    }
}

async function confirmDeleteWorkspace(id, name) {
    const ok = await appConfirm(`¿Estás seguro de eliminar el workspace "${name}"?\nEsto desvinculará sus bubbles de Orbit (tus archivos en disco NO serán borrados).`, "Eliminar Workspace", "Eliminar", "Cancelar", true, "🗑️");
    if (!ok) return;

    const token = serverInfo ? serverInfo.authToken : '';
    const headers = token ? { 'Authorization': 'Bearer ' + token } : {};

    try {
        const res = await fetch(`/api/workspaces/${encodeURIComponent(id)}`, { method: 'DELETE', headers });
        if (res.ok) {
            showToast(`Workspace "${name}" eliminado.`);
            if (activeFilter === id) {
                setWorkspaceFilter('all');
            }
            await fetchWorkspaces();
            renderWorkspacesListInModal();
            await fetchBubbles();
        } else {
            appAlert("Error al eliminar workspace: " + (await res.text()), "Error", "❌");
        }
    } catch (e) {
        appAlert("Error de red: " + e, "Error de Red", "❌");
    }
}

// ==========================================
// Folder Browser Modal
// ==========================================

let folderPickerTargetInput = null;
let folderPickerCurrentPath = '';
let folderPickerParentPath = '';

async function openFolderPicker(targetInputId) {
    folderPickerTargetInput = targetInputId;
    const initialPath = document.getElementById(targetInputId)?.value.trim() || '';
    const modal = document.getElementById('modal-folder-picker');
    if (modal) modal.classList.add('open');
    await browseFolder(initialPath);
}

function closeFolderPicker() {
    const modal = document.getElementById('modal-folder-picker');
    if (modal) modal.classList.remove('open');
}

async function browseFolder(path) {
    const token = serverInfo ? serverInfo.authToken : '';
    const headers = token ? { 'Authorization': 'Bearer ' + token } : {};
    const url = `/api/fs/directories${path ? '?path=' + encodeURIComponent(path) : ''}`;

    try {
        const res = await fetch(url, { headers });
        if (!res.ok) return;
        const data = await res.json();
        folderPickerCurrentPath = data.current;
        folderPickerParentPath = data.parent;

        const pathInput = document.getElementById('folder-picker-current-path');
        if (pathInput) pathInput.value = data.current;

        const listEl = document.getElementById('folder-picker-list');
        if (!listEl) return;

        if (!data.dirs || data.dirs.length === 0) {
            listEl.innerHTML = `<div style="padding: 18px; text-align: center; color: var(--text-dim); font-size: 12px;">No se encontraron subcarpetas aquí.</div>`;
            return;
        }

        listEl.innerHTML = data.dirs.map(d => {
            const escapedPath = d.path.replace(/\\/g, '/');
            return `
                <div style="padding: 6px 10px; border-radius: var(--radius-sm); display: flex; align-items: center; gap: 8px; cursor: pointer; transition: background 0.15s ease;"
                     onmouseover="this.style.background='var(--bg-hover)'"
                     onmouseout="this.style.background='transparent'"
                     onclick="browseFolder('${escapeHtml(escapedPath)}')">
                    <span style="font-size: 14px;">📁</span>
                    <span style="font-size: 12px; color: var(--text-main); font-weight: 500;">${escapeHtml(d.name)}</span>
                </div>
            `;
        }).join('');
    } catch (e) {
        console.error("Error al explorar carpetas:", e);
    }
}

function folderPickerGoUp() {
    if (folderPickerParentPath && folderPickerParentPath !== folderPickerCurrentPath) {
        browseFolder(folderPickerParentPath);
    }
}

function selectCurrentFolderPicker() {
    if (folderPickerTargetInput && folderPickerCurrentPath) {
        const el = document.getElementById(folderPickerTargetInput);
        if (el) {
            el.value = folderPickerCurrentPath;
            const nameEl = document.getElementById('ws-input-name');
            if (nameEl && !nameEl.value.trim()) {
                const parts = folderPickerCurrentPath.replace(/\\/g, '/').split('/').filter(Boolean);
                if (parts.length > 0) {
                    nameEl.value = parts[parts.length - 1];
                }
            }
        }
    }
    closeFolderPicker();
}



// ==========================================
// QoL 3: Workspace Quick Actions
// ==========================================
function updateQuickActionsUI() {
    const popup = document.getElementById('quick-actions-menu-popup');
    if (!popup) return;
    if (!currentBubble) {
        popup.innerHTML = `<span style="font-size: 11px; color: var(--text-dim); padding: 4px;">Selecciona una burbuja primero</span>`;
        return;
    }

    if (currentBubble.workspace === 'aditum') {
        popup.innerHTML = `
            <div style="font-size: 10px; font-weight: 700; color: var(--text-dim); text-transform: uppercase; padding: 4px 6px;">Aditum Quick Ops</div>
            <button class="btn btn-sm" style="text-align: left; justify-content: flex-start; background: transparent; border: none; width: 100%;" onclick="executeQuickAction('bash ./scripts/apply-migrations.sh')">
                🗄️ Aplicar Migraciones Liquibase
            </button>
            <button class="btn btn-sm" style="text-align: left; justify-content: flex-start; background: transparent; border: none; width: 100%;" onclick="executeQuickAction('./mvnw compile -Dmaven.test.skip=true')">
                ☕ Maven Compile (skip tests)
            </button>
            <button class="btn btn-sm" style="text-align: left; justify-content: flex-start; background: transparent; border: none; width: 100%;" onclick="executeQuickAction('git pull origin main')">
                🌿 Git Pull (main)
            </button>
            <button class="btn btn-sm" style="text-align: left; justify-content: flex-start; background: transparent; border: none; width: 100%;" onclick="executeQuickAction('git status')">
                📦 Git Status
            </button>
        `;
    } else {
        popup.innerHTML = `
            <div style="font-size: 10px; font-weight: 700; color: var(--text-dim); text-transform: uppercase; padding: 4px 6px;">Brif & Celery Quick Ops</div>
            <button class="btn btn-sm" style="text-align: left; justify-content: flex-start; background: transparent; border: none; width: 100%;" onclick="controlService('celery-worker', 'start')">
                ⚙️ Lanzar Celery Worker
            </button>
            <button class="btn btn-sm" style="text-align: left; justify-content: flex-start; background: transparent; border: none; width: 100%;" onclick="controlService('celery-beat', 'start')">
                ⏰ Lanzar Celery Beat
            </button>
            <button class="btn btn-sm" style="text-align: left; justify-content: flex-start; background: transparent; border: none; width: 100%;" onclick="openFlowerInPreview()">
                🌸 Abrir Flower en Live Preview
            </button>
            <button class="btn btn-sm" style="text-align: left; justify-content: flex-start; background: transparent; border: none; width: 100%;" onclick="controlService('redis', 'start')">
                🟥 Iniciar Redis Server (WSL)
            </button>
            <button class="btn btn-sm" style="text-align: left; justify-content: flex-start; background: transparent; border: none; width: 100%;" onclick="executeQuickAction('.\\\\venv\\\\Scripts\\\\celery.exe -A config purge -f')">
                🧹 Purgar Colas Celery
            </button>
            <button class="btn btn-sm" style="text-align: left; justify-content: flex-start; background: transparent; border: none; width: 100%;" onclick="executeQuickAction('.\\\\venv\\\\Scripts\\\\python.exe manage.py migrate')">
                🐍 Django Migrate
            </button>
            <button class="btn btn-sm" style="text-align: left; justify-content: flex-start; background: transparent; border: none; width: 100%;" onclick="executeQuickAction('git status')">
                📦 Git Status
            </button>
        `;
    }

    // Add Transcribe action if groq_whisper plugin is enabled
    const groqPlugin = (typeof allPlugins !== 'undefined') && allPlugins.find(p => p.id === 'groq_whisper' && p.enabled);
    if (groqPlugin) {
        popup.innerHTML += `
            <div style="border-top: 1px solid var(--border-subtle); margin: 4px 0;"></div>
            <button class="btn btn-sm" style="text-align: left; justify-content: flex-start; background: transparent; border: none; width: 100%; color: var(--accent);" onclick="toggleQuickActionsMenu(); openTranscribeModal();">
                🎙️ Transcribir Video / Audio (Whisper)
            </button>
        `;
    }
}

function toggleQuickActionsMenu() {
    const popup = document.getElementById('quick-actions-menu-popup');
    if (!popup) return;
    updateQuickActionsUI();
    popup.style.display = (popup.style.display === 'flex') ? 'none' : 'flex';
}

function executeQuickAction(cmd) {
    const popup = document.getElementById('quick-actions-menu-popup');
    if (popup) popup.style.display = 'none';
    if (termWs && termWs.readyState === WebSocket.OPEN) {
        termWs.send(JSON.stringify({ type: "input", data: cmd + "\r" }));
        showToast(`Enviado: ${cmd}`);
    } else {
        showToast("No hay terminal activa conectada.");
    }
}
