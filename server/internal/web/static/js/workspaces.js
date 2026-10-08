// ==========================================
// Workspace & Bubble Management
// ==========================================

// Fetch Bubbles
async function fetchBubbles() {
    try {
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
    document.querySelectorAll('.ws-tab-btn').forEach(btn => {
        btn.classList.remove('active');
    });
    if (filter === 'aditum') document.querySelector('.ws-tab-btn.aditum')?.classList.add('active');
    else if (filter === 'brif') document.querySelector('.ws-tab-btn.brif')?.classList.add('active');
    else document.querySelectorAll('.ws-tab-btn')[2]?.classList.add('active');

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
                        <span title="${escapeHtml(b.name)}">${escapeHtml(b.name)}</span>
                    </div>
                    <div style="display: flex; gap: 4px; align-items: center;">
                        <span class="badge-priority ${pInfo.class}" onclick="openPriorityPicker(event, '${b.id}')" title="Prioridad Notion: ${pInfo.label} (Clic para cambiar)">
                            ${pInfo.icon} ${b.priority || 'P-'}
                        </span>
                        ${hasTerms ? `
                            <span class="badge-term-active" title="${bSessions.length} terminal(es) abierta(s)">
                                💻 ${bSessions.length}${claudeCount > 0 ? ` 🤖${claudeCount}` : ''}
                            </span>
                        ` : ''}
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
                    <span style="overflow: hidden; text-overflow: ellipsis; max-width: 130px;" title="${escapeHtml(b.branch)}">${escapeHtml(b.branch)}</span>
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

function openNewBubbleModal(prefWs = null) {
    if (prefWs) {
        setWizardWorkspace(prefWs);
    } else {
        setWizardWorkspace(activeFilter === 'brif' ? 'brif' : 'aditum');
    }
    suggestNextAditumIndex();
    document.getElementById('modal-new-bubble').classList.add('open');
}

function closeNewBubbleModal() {
    document.getElementById('modal-new-bubble').classList.remove('open');
}

function setWizardWorkspace(ws) {
    wizardWs = ws;
    const tabAditum = document.getElementById('wizard-tab-aditum');
    const tabBrif = document.getElementById('wizard-tab-brif');
    const fieldsAditum = document.getElementById('wizard-fields-aditum');
    const fieldsBrif = document.getElementById('wizard-fields-brif');

    if (ws === 'aditum') {
        tabAditum.classList.add('active');
        tabBrif.classList.remove('active');
        fieldsAditum.style.display = 'block';
        fieldsBrif.style.display = 'none';
    } else {
        tabBrif.classList.add('active');
        tabAditum.classList.remove('active');
        fieldsBrif.style.display = 'block';
        fieldsAditum.style.display = 'none';
    }
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
            const taskId = document.getElementById('aditum-task-id').value.trim();
            if (!taskId) {
                appAlert("Por favor ingresa un ID de tarea (ej. TAR-1130)", "Campo Requerido", "⚠️");
                btn.disabled = false;
                btn.textContent = "⚡ Crear y Configurar Bubble";
                return;
            }
            const index = parseInt(document.getElementById('aditum-index').value) || 1;
            const baseBranch = document.getElementById('aditum-base-branch').value.trim() || 'development';
            const useSlim = document.getElementById('aditum-seed-mode').value === 'slim';

            body = {
                workspace: 'aditum',
                taskId: taskId,
                index: index,
                baseBranch: baseBranch,
                useSlim: useSlim
            };
        } else {
            const taskId = document.getElementById('brif-task-id').value.trim();
            if (!taskId) {
                appAlert("Por favor ingresa un nombre de tarea o spider (ej. dialogos-mx)", "Campo Requerido", "⚠️");
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
