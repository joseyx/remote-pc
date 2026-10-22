// ==========================================
// Calendar, Daily Notes & Handoff System
// ==========================================

/* =====================================================
   CALENDAR & DAILY NOTES & AGENT HANDOFF LOGIC
   ===================================================== */
let calCurrentYear = new Date().getFullYear();
let calCurrentMonth = new Date().getMonth() + 1; // 1-12
let calSelectedDate = getTodayFormatted(); // YYYY-MM-DD
let calMonthSummary = [];
let calActiveNote = null;
let isCalEditing = false;
let calHandoffStatusFilter = 'all';
let calHandoffSearchQuery = '';
let calCurrentDetailHandoff = null;
let calNotesCollapsed = false;
let calAllHandoffsExpanded = false;

function getTodayFormatted() {
    const d = new Date();
    const y = d.getFullYear();
    const m = String(d.getMonth() + 1).padStart(2, '0');
    const day = String(d.getDate()).padStart(2, '0');
    return `${y}-${m}-${day}`;
}

function getEffectiveHandoffDate(d = new Date()) {
    if (d.getHours() < 5) {
        const target = new Date(d);
        target.setDate(target.getDate() - 1);
        while (target.getDay() === 0 || target.getDay() === 6) { // 0=Sunday, 6=Saturday
            target.setDate(target.getDate() - 1);
        }
        const y = target.getFullYear();
        const m = String(target.getMonth() + 1).padStart(2, '0');
        const day = String(target.getDate()).padStart(2, '0');
        return `${y}-${m}-${day}`;
    }
    const y = d.getFullYear();
    const m = String(d.getMonth() + 1).padStart(2, '0');
    const day = String(d.getDate()).padStart(2, '0');
    return `${y}-${m}-${day}`;
}

async function loadCalendarView() {
    const today = getTodayFormatted();
    if (!calSelectedDate) calSelectedDate = getEffectiveHandoffDate();
    await fetchCalendarMonth(calCurrentYear, calCurrentMonth);
    await selectCalendarDate(calSelectedDate);
    await fetchCalendarStandup();
}

async function changeCalendarMonth(delta) {
    calCurrentMonth += delta;
    if (calCurrentMonth < 1) {
        calCurrentMonth = 12;
        calCurrentYear--;
    } else if (calCurrentMonth > 12) {
        calCurrentMonth = 1;
        calCurrentYear++;
    }
    await fetchCalendarMonth(calCurrentYear, calCurrentMonth);
}

async function goToToday() {
    const now = new Date();
    calCurrentYear = now.getFullYear();
    calCurrentMonth = now.getMonth() + 1;
    calSelectedDate = getTodayFormatted();
    await fetchCalendarMonth(calCurrentYear, calCurrentMonth);
    await selectCalendarDate(calSelectedDate);
}

async function fetchCalendarMonth(year, month) {
    const monthNames = [
        "Enero", "Febrero", "Marzo", "Abril", "Mayo", "Junio",
        "Julio", "Agosto", "Septiembre", "Octubre", "Noviembre", "Diciembre"
    ];
    const titleEl = document.getElementById('cal-month-title');
    if (titleEl) {
        titleEl.textContent = `${monthNames[month - 1]} ${year}`;
    }

    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = token ? { 'Authorization': 'Bearer ' + token } : {};
        const res = await fetch(`/api/calendar/month?year=${year}&month=${month}`, { headers });
        if (res.ok) {
            calMonthSummary = await res.json();
        } else {
            calMonthSummary = [];
        }
    } catch (e) {
        console.error("Error fetching calendar month:", e);
        calMonthSummary = [];
    }

    renderCalendarGrid(year, month);
}

function renderCalendarGrid(year, month) {
    const grid = document.getElementById('cal-days-grid');
    if (!grid) return;

    grid.innerHTML = '';

    const firstDayOfMonth = new Date(year, month - 1, 1);
    const daysInMonth = new Date(year, month, 0).getDate();

    // Monday-based day of week (0: Mon, 6: Sun)
    let startDay = firstDayOfMonth.getDay() - 1;
    if (startDay === -1) startDay = 6;

    const today = getTodayFormatted();

    // Empty padding cells before start of month
    for (let i = 0; i < startDay; i++) {
        const emptyCell = document.createElement('div');
        emptyCell.className = 'calendar-day-btn other-month';
        grid.appendChild(emptyCell);
    }

    // Month days
    for (let day = 1; day <= daysInMonth; day++) {
        const dateStr = `${year}-${String(month).padStart(2, '0')}-${String(day).padStart(2, '0')}`;
        const daySummary = calMonthSummary.find(d => d.date === dateStr);

        const btn = document.createElement('button');
        btn.className = 'calendar-day-btn';
        if (dateStr === today) btn.classList.add('is-today');
        if (dateStr === calSelectedDate) btn.classList.add('is-selected');

        let dotsHtml = '';
        if (daySummary) {
            dotsHtml = '<div class="calendar-day-dots">';
            if (daySummary.hasNote) dotsHtml += '<span class="cal-dot-note" title="Tiene notas"></span>';
            if (daySummary.handoffCount > 0) dotsHtml += `<span class="cal-dot-handoff" title="${daySummary.handoffCount} handoff(s)"></span>`;
            dotsHtml += '</div>';
        }

        btn.innerHTML = `<span>${day}</span>${dotsHtml}`;
        btn.onclick = () => selectCalendarDate(dateStr);
        grid.appendChild(btn);
    }
}

async function selectCalendarDate(dateStr) {
    calSelectedDate = dateStr;
    const today = getTodayFormatted();

    // Highlight selected button
    document.querySelectorAll('.calendar-day-btn').forEach(btn => btn.classList.remove('is-selected'));
    renderCalendarGrid(calCurrentYear, calCurrentMonth);

    // Format title
    const parts = dateStr.split('-');
    const d = new Date(parseInt(parts[0]), parseInt(parts[1]) - 1, parseInt(parts[2]));
    const options = { weekday: 'long', year: 'numeric', month: 'long', day: 'numeric' };
    const titleStr = d.toLocaleDateString('es-ES', options);

    const titleEl = document.getElementById('cal-active-date-title');
    if (titleEl) {
        titleEl.textContent = `📅 ${titleStr.charAt(0).toUpperCase() + titleStr.slice(1)}`;
    }

    const badgeEl = document.getElementById('cal-active-date-badge');
    if (badgeEl) {
        if (dateStr === today) {
            badgeEl.textContent = 'HOY';
            badgeEl.style.display = 'inline-block';
        } else {
            badgeEl.style.display = 'none';
        }
    }

    await fetchCalendarNote(dateStr);
}

async function fetchCalendarNote(dateStr) {
    isCalEditing = false;
    updateCalEditorVisibility();

    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = token ? { 'Authorization': 'Bearer ' + token } : {};
        const res = await fetch(`/api/calendar/note?date=${dateStr}`, { headers });
        if (res.ok) {
            calActiveNote = await res.json();
            renderCalendarNote(calActiveNote);
        }
    } catch (e) {
        console.error("Error fetching calendar note:", e);
    }
}

function reloadCalendarNote() {
    if (calSelectedDate) fetchCalendarNote(calSelectedDate);
}

function renderCalendarNote(note) {
    calActiveNote = note;
    const renderedEl = document.getElementById('cal-rendered-note');
    const rawEl = document.getElementById('cal-raw-editor');

    if (rawEl) rawEl.value = note.content || '';

    if (renderedEl) {
        if (typeof marked !== 'undefined') {
            renderedEl.innerHTML = marked.parse(note.content || '*Sin notas para este día.*');
        } else {
            renderedEl.textContent = note.content || 'Sin notas para este día.';
        }
    }

    renderHandoffsList();
}

function renderHandoffsList() {
    const handoffsEl = document.getElementById('cal-handoffs-container');
    const countBadge = document.getElementById('cal-handoffs-count-badge');
    const allHandoffs = (calActiveNote && calActiveNote.handoffs) ? calActiveNote.handoffs : [];

    // Filter by status & search query
    let filtered = allHandoffs.filter(h => {
        if (calHandoffStatusFilter !== 'all' && h.status !== calHandoffStatusFilter) {
            return false;
        }
        if (calHandoffSearchQuery) {
            const q = calHandoffSearchQuery.toLowerCase();
            const matchTask = (h.taskName || '').toLowerCase().includes(q);
            const matchSummary = (h.summary || '').toLowerCase().includes(q);
            const matchPending = (h.pending || '').toLowerCase().includes(q);
            const matchBlockers = (h.blockers || '').toLowerCase().includes(q);
            const matchBranch = (h.branch || '').toLowerCase().includes(q);
            const matchCommit = (h.lastCommit || '').toLowerCase().includes(q);
            if (!matchTask && !matchSummary && !matchPending && !matchBlockers && !matchBranch && !matchCommit) {
                return false;
            }
        }
        return true;
    });

    if (countBadge) {
        if (calHandoffStatusFilter !== 'all' || calHandoffSearchQuery) {
            countBadge.textContent = `${filtered.length} de ${allHandoffs.length} registrados`;
        } else {
            countBadge.textContent = `${allHandoffs.length} registrado${allHandoffs.length === 1 ? '' : 's'}`;
        }
    }

    if (!handoffsEl) return;

    if (filtered.length === 0) {
        if (allHandoffs.length === 0) {
            handoffsEl.innerHTML = `
                <div style="color: var(--text-dim); font-size: 12px; text-align: center; padding: 24px;">
                    No hay handoffs registrados para este día. Pulsa "🌙 + Nuevo Handoff" para agregar uno.
                </div>
            `;
        } else {
            handoffsEl.innerHTML = `
                <div style="color: var(--text-dim); font-size: 12px; text-align: center; padding: 24px;">
                    No se encontraron handoffs que coincidan con el filtro o búsqueda actual.
                </div>
            `;
        }
        return;
    }

    handoffsEl.innerHTML = `
        <div class="handoff-list">
            ${filtered.map(h => renderHandoffCardHtml(h)).join('')}
        </div>
    `;
}

function formatHandoffMarkdown(text) {
    if (!text) return '';
    let processed = String(text).trim();

    // 1. Normalize enumerated lists written inline: "1) Item... 2) Item..." or "1. Item... 2. Item..." -> "1. Item...\n2. Item..."
    processed = processed.replace(/^(\d+)[\)\.]\s+/, '$1. ');
    processed = processed.replace(/([^\n])\s+(\d+)[\)\.]\s+([A-Za-zÁ-ÿ0-9¿¡])/g, '$1\n$2. $3');

    // 2. Normalize inline bullet points: "item • item" -> "item\n- item"
    processed = processed.replace(/([^\n])\s+[•\*]\s+([A-Za-zÁ-ÿ0-9¿¡])/g, '$1\n- $2');
    processed = processed.replace(/^[•\*]\s+/, '- ');

    // 3. Render using marked
    if (typeof marked !== 'undefined') {
        try {
            return marked.parse(processed);
        } catch (e) {
            console.warn("Markdown parse error:", e);
        }
    }

    return escapeHtml(processed).replace(/\n/g, '<br/>');
}

function renderHandoffCardHtml(h) {
    let statusClass = 'in_progress';
    let statusText = '🟡 En Progreso';
    let cardStatusClass = 'status-in_progress';
    if (h.status === 'completed') {
        statusClass = 'completed';
        statusText = '🟢 Completado';
        cardStatusClass = 'status-completed';
    } else if (h.status === 'blocked') {
        statusClass = 'blocked';
        statusText = '🔴 Bloqueado';
        cardStatusClass = 'status-blocked';
    }

    let agentIcon = '🤖 Claude Code';
    let agentClass = '';
    if (h.agent === 'agy') {
        agentIcon = '🚀 Antigravity CLI';
        agentClass = 'agy';
    } else if (h.agent === 'developer') {
        agentIcon = '👤 Desarrollador';
        agentClass = 'dev';
    }

    let gitBar = '';
    if (h.branch || h.lastCommit) {
        gitBar = `
            <div class="handoff-git-bar">
                <span>🌿 <strong>${escapeHtml(h.branch || 'HEAD')}</strong></span>
                ${h.lastCommit ? `<span>| 📦 ${escapeHtml(h.lastCommit)}</span>` : ''}
                ${h.dirtyFiles > 0 ? `<span style="color: var(--amber);">| ⚠️ ${h.dirtyFiles} sin commitear</span>` : '<span style="color: var(--green);">| ✓ Árbol limpio</span>'}
            </div>
        `;
    }

    let summaryContent = h.summary || '';
    let pendingContent = h.pending || '';
    if (!calAllHandoffsExpanded) {
        if (summaryContent.length > 280) summaryContent = summaryContent.slice(0, 277) + '...';
        if (pendingContent.length > 280) pendingContent = pendingContent.slice(0, 277) + '...';
    }

    return `
        <div class="handoff-card ${cardStatusClass}" id="card-handoff-${h.id}">
            <div class="handoff-card-header" onclick="openHandoffDetail('${h.id}')" title="Clic para abrir vista detallada">
                <div class="handoff-card-title">
                    <span>💼 ${escapeHtml(h.taskName || 'Tarea')}</span>
                    <span style="font-size: 11px; font-weight: normal; color: var(--text-dim);">(${escapeHtml(h.time || '')})</span>
                </div>
                <div class="handoff-card-badges">
                    <span class="handoff-agent-pill ${agentClass}">${agentIcon}</span>
                    <span class="handoff-status-pill ${statusClass}">${statusText}</span>
                </div>
            </div>
            <div class="handoff-body">
                ${h.summary ? `
                    <div class="handoff-section-row">
                        <span class="handoff-section-label">✓ Lo logrado hoy:</span>
                        <div class="handoff-markdown-body" style="flex: 1;">${formatHandoffMarkdown(summaryContent)}</div>
                    </div>
                ` : ''}
                ${h.pending ? `
                    <div class="handoff-section-row">
                        <span class="handoff-section-label" style="color: var(--accent);">⏳ Pendiente para mañana:</span>
                        <div class="handoff-markdown-body" style="flex: 1; color: var(--text-main);">${formatHandoffMarkdown(pendingContent)}</div>
                    </div>
                ` : ''}
                ${h.blockers ? `
                    <div class="handoff-section-row">
                        <span class="handoff-section-label" style="color: var(--red);">⛔ Puntos de atención:</span>
                        <div class="handoff-markdown-body" style="flex: 1; color: #fb7185;">${formatHandoffMarkdown(h.blockers)}</div>
                    </div>
                ` : ''}
                ${gitBar}
            </div>
            <div class="handoff-actions-row">
                <div class="handoff-action-buttons">
                    <button class="btn-handoff-action primary" onclick="event.stopPropagation(); openHandoffDetail('${h.id}')" title="Ver detalle completo">
                        🔍 Ver Detalle
                    </button>
                    <button class="btn-handoff-action" onclick="event.stopPropagation(); openEditHandoffModal('${h.id}')" title="Modificar este handoff">
                        ✏️ Modificar
                    </button>
                    <button class="btn-handoff-action danger" onclick="event.stopPropagation(); confirmDeleteHandoff('${h.id}', '${escapeHtml(h.taskName || '')}')" title="Eliminar handoff">
                        🗑️ Eliminar
                    </button>
                </div>
                <div>
                    <button class="btn btn-sm btn-ghost" onclick="event.stopPropagation(); resumeTaskFromHandoff('${escapeHtml(h.bubbleId || '')}', '${escapeHtml(h.taskName || '')}', '${escapeHtml(h.pending || '')}')" style="font-size: 11px; color: var(--accent);">
                        ▶ Retomar esta Tarea
                    </button>
                </div>
            </div>
        </div>
    `;
}

function filterHandoffsList() {
    const input = document.getElementById('handoffs-search-input');
    calHandoffSearchQuery = input ? input.value.trim() : '';
    renderHandoffsList();
}

function setHandoffStatusFilter(status, btn) {
    calHandoffStatusFilter = status;
    document.querySelectorAll('.handoffs-filter-btn').forEach(b => b.classList.remove('active'));
    if (btn) btn.classList.add('active');
    renderHandoffsList();
}

function toggleAllHandoffsExpand(expand) {
    calAllHandoffsExpanded = expand;
    renderHandoffsList();
}

function toggleCalendarNotesCollapse() {
    calNotesCollapsed = !calNotesCollapsed;
    const card = document.getElementById('cal-note-card');
    const btn = document.getElementById('btn-toggle-notes');
    if (card) {
        card.classList.toggle('collapsed', calNotesCollapsed);
    }
    if (btn) {
        btn.textContent = calNotesCollapsed ? '🔽 Mostrar Notas' : '🔼 Ocultar Notas';
    }
}

// ================= HANDOFF DETAIL MODAL =================
function openHandoffDetail(id) {
    if (!calActiveNote || !calActiveNote.handoffs) return;
    const h = calActiveNote.handoffs.find(it => it.id === id);
    if (!h) return;

    calCurrentDetailHandoff = h;

    const titleEl = document.getElementById('handoff-detail-title');
    const timeEl = document.getElementById('handoff-detail-time-badge');
    const agentEl = document.getElementById('handoff-detail-agent-pill');
    const statusEl = document.getElementById('handoff-detail-status-pill');
    const dateEl = document.getElementById('handoff-detail-date-badge');
    const summaryEl = document.getElementById('handoff-detail-summary');
    const pendingEl = document.getElementById('handoff-detail-pending');
    const blockersCont = document.getElementById('handoff-detail-blockers-container');
    const blockersEl = document.getElementById('handoff-detail-blockers');
    const gitCont = document.getElementById('handoff-detail-git-container');
    const gitEl = document.getElementById('handoff-detail-git');

    if (titleEl) titleEl.textContent = `💼 ${h.taskName || 'Tarea'}`;
    if (timeEl) timeEl.textContent = `(${h.time || ''})`;
    if (dateEl) dateEl.textContent = `Fecha: ${h.date || calSelectedDate}`;

    if (agentEl) {
        let agentText = '🤖 Claude Code';
        let agentClass = '';
        if (h.agent === 'agy') {
            agentText = '🚀 Antigravity CLI';
            agentClass = 'agy';
        } else if (h.agent === 'developer') {
            agentText = '👤 Desarrollador';
            agentClass = 'dev';
        }
        agentEl.className = `handoff-agent-pill ${agentClass}`;
        agentEl.textContent = agentText;
    }

    if (statusEl) {
        let statusText = '🟡 En Progreso';
        let statusClass = 'in_progress';
        if (h.status === 'completed') {
            statusText = '🟢 Completado';
            statusClass = 'completed';
        } else if (h.status === 'blocked') {
            statusText = '🔴 Bloqueado';
            statusClass = 'blocked';
        }
        statusEl.className = `handoff-status-pill ${statusClass}`;
        statusEl.textContent = statusText;
    }

    if (summaryEl) summaryEl.innerHTML = formatHandoffMarkdown(h.summary || '*Sin descripción de lo completado.*');
    if (pendingEl) pendingEl.innerHTML = formatHandoffMarkdown(h.pending || '*Sin tareas pendientes registradas.*');

    if (blockersCont && blockersEl) {
        if (h.blockers && h.blockers.trim()) {
            blockersCont.style.display = 'block';
            blockersEl.innerHTML = formatHandoffMarkdown(h.blockers);
        } else {
            blockersCont.style.display = 'none';
        }
    }

    if (gitCont && gitEl) {
        if (h.branch || h.lastCommit) {
            gitCont.style.display = 'block';
            let gitLines = [];
            if (h.branch) gitLines.push(`• Rama activa: ${h.branch}`);
            if (h.lastCommit) gitLines.push(`• Último commit: ${h.lastCommit}`);
            if (h.dirtyFiles > 0) gitLines.push(`• Archivos sin commitear: ${h.dirtyFiles}`);
            else gitLines.push(`• Árbol de trabajo: Limpio`);
            gitEl.textContent = gitLines.join('\n');
        } else {
            gitCont.style.display = 'none';
        }
    }

    const modal = document.getElementById('modal-handoff-detail');
    if (modal) modal.classList.add('open');
}

function closeHandoffDetailModal() {
    const modal = document.getElementById('modal-handoff-detail');
    if (modal) modal.classList.remove('open');
}

function copyCurrentHandoffText() {
    if (!calCurrentDetailHandoff) return;
    const h = calCurrentDetailHandoff;
    let text = `*Handoff: ${h.taskName}* (${h.date} ${h.time})\n`;
    text += `Estado: ${h.status === 'completed' ? 'Completado' : h.status === 'blocked' ? 'Bloqueado' : 'En Progreso'}\n\n`;
    if (h.summary) text += `*Logrado hoy:*\n${h.summary}\n\n`;
    if (h.pending) text += `*Pendiente para mañana:*\n${h.pending}\n\n`;
    if (h.blockers) text += `*Bloqueos:*\n${h.blockers}\n\n`;
    if (h.branch) text += `*Git:* Rama ${h.branch} | Commit ${h.lastCommit || 'HEAD'}\n`;

    navigator.clipboard.writeText(text).then(() => {
        showToast("📋 Handoff copiado al portapapeles");
    }).catch(() => {
        showToast("No se pudo copiar al portapapeles");
    });
}

function resumeCurrentHandoffFromDetail() {
    if (!calCurrentDetailHandoff) return;
    const h = calCurrentDetailHandoff;
    closeHandoffDetailModal();
    resumeTaskFromHandoff(h.bubbleId || '', h.taskName || '', h.pending || '');
}

function editCurrentHandoffFromDetail() {
    if (!calCurrentDetailHandoff) return;
    const id = calCurrentDetailHandoff.id;
    closeHandoffDetailModal();
    openEditHandoffModal(id);
}

function deleteCurrentHandoffFromDetail() {
    if (!calCurrentDetailHandoff) return;
    const id = calCurrentDetailHandoff.id;
    const taskName = calCurrentDetailHandoff.taskName;
    closeHandoffDetailModal();
    confirmDeleteHandoff(id, taskName);
}

// ================= HANDOFF EDIT MODAL =================
function openEditHandoffModal(id) {
    if (!calActiveNote || !calActiveNote.handoffs) return;
    const h = calActiveNote.handoffs.find(it => it.id === id);
    if (!h) return;

    document.getElementById('edit-handoff-id').value = h.id;
    document.getElementById('edit-handoff-date').value = h.date || calSelectedDate;
    document.getElementById('edit-handoff-taskname').value = h.taskName || '';
    document.getElementById('edit-handoff-agent').value = h.agent || 'claude';
    document.getElementById('edit-handoff-status').value = h.status || 'in_progress';
    document.getElementById('edit-handoff-summary').value = h.summary || '';
    document.getElementById('edit-handoff-pending').value = h.pending || '';
    document.getElementById('edit-handoff-blockers').value = h.blockers || '';

    const titleEl = document.getElementById('edit-handoff-modal-title');
    if (titleEl) titleEl.textContent = `✏️ Modificar Handoff - ${h.taskName || 'Tarea'}`;

    const modal = document.getElementById('modal-handoff-edit');
    if (modal) modal.classList.add('open');
}

function closeEditHandoffModal() {
    const modal = document.getElementById('modal-handoff-edit');
    if (modal) modal.classList.remove('open');
}

async function submitEditHandoffForm() {
    const id = document.getElementById('edit-handoff-id').value;
    const date = document.getElementById('edit-handoff-date').value || calSelectedDate;
    const taskName = document.getElementById('edit-handoff-taskname').value.trim();
    const agent = document.getElementById('edit-handoff-agent').value;
    const status = document.getElementById('edit-handoff-status').value;
    const summary = document.getElementById('edit-handoff-summary').value.trim();
    const pending = document.getElementById('edit-handoff-pending').value.trim();
    const blockers = document.getElementById('edit-handoff-blockers').value.trim();

    if (!summary && !pending) {
        appAlert("Por favor ingresa al menos lo completado hoy o lo pendiente para mañana.", "Campos Requeridos", "⚠️");
        return;
    }

    const payload = {
        id,
        date,
        taskName: taskName || 'Tarea',
        agent,
        status,
        summary,
        pending,
        blockers
    };

    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': 'Bearer ' + token } : {})
        };
        const res = await fetch('/api/calendar/handoff', {
            method: 'PUT',
            headers,
            body: JSON.stringify(payload)
        });

        if (res.ok) {
            showToast("✅ Handoff modificado exitosamente");
            closeEditHandoffModal();
            await fetchCalendarNote(date);
            await fetchCalendarMonth(calCurrentYear, calCurrentMonth);
            await fetchCalendarStandup();
        } else {
            appAlert("Error al actualizar handoff: " + (await res.text()), "Error", "❌");
        }
    } catch (e) {
        appAlert("Error de conexión: " + e, "Error", "❌");
    }
}

// ================= HANDOFF DELETE ACTION =================
async function confirmDeleteHandoff(id, taskName) {
    const ok = await appConfirm(
        `¿Estás seguro de que deseas eliminar este registro de handoff de la tarea "${taskName}"?\n\nEsta acción actualizará las notas diarias y el registro de standup.`,
        "Eliminar Handoff",
        "Eliminar",
        "Cancelar",
        true,
        "🗑️"
    );
    if (!ok) return;

    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': 'Bearer ' + token } : {})
        };
        const res = await fetch(`/api/calendar/handoff?id=${encodeURIComponent(id)}&date=${encodeURIComponent(calSelectedDate)}`, {
            method: 'DELETE',
            headers
        });

        if (res.ok) {
            showToast("🗑️ Handoff eliminado correctamente");
            await fetchCalendarNote(calSelectedDate);
            await fetchCalendarMonth(calCurrentYear, calCurrentMonth);
            await fetchCalendarStandup();
        } else {
            appAlert("Error al eliminar handoff: " + (await res.text()), "Error", "❌");
        }
    } catch (e) {
        appAlert("Error de conexión: " + e, "Error", "❌");
    }
}

function toggleCalendarEditMode() {
    isCalEditing = !isCalEditing;
    updateCalEditorVisibility();
}

function updateCalEditorVisibility() {
    const renderedEl = document.getElementById('cal-rendered-note');
    const rawEl = document.getElementById('cal-raw-editor');
    const editBtn = document.getElementById('btn-cal-edit-toggle');
    const saveBtn = document.getElementById('btn-cal-save-note');
    const statusEl = document.getElementById('cal-note-status');

    if (isCalEditing) {
        if (renderedEl) renderedEl.style.display = 'none';
        if (rawEl) {
            rawEl.style.display = 'block';
            rawEl.focus();
        }
        if (editBtn) editBtn.textContent = '👁️ Vista Previa';
        if (saveBtn) saveBtn.style.display = 'inline-flex';
        if (statusEl) statusEl.textContent = 'Modo Edición';
    } else {
        if (renderedEl) {
            renderedEl.style.display = 'block';
            if (rawEl && typeof marked !== 'undefined') {
                renderedEl.innerHTML = marked.parse(rawEl.value || '*Sin notas*');
            }
        }
        if (rawEl) rawEl.style.display = 'none';
        if (editBtn) editBtn.textContent = '✏️ Editar Nota';
        if (saveBtn) saveBtn.style.display = 'none';
        if (statusEl) statusEl.textContent = 'Visualización';
    }
}

async function saveCalendarNote() {
    const rawEl = document.getElementById('cal-raw-editor');
    if (!rawEl) return;
    const content = rawEl.value;

    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': 'Bearer ' + token } : {})
        };
        const res = await fetch('/api/calendar/note', {
            method: 'PUT',
            headers,
            body: JSON.stringify({ date: calSelectedDate, content })
        });
        if (res.ok) {
            showToast("Nota diaria guardada con éxito.");
            toggleCalendarEditMode();
            await fetchCalendarMonth(calCurrentYear, calCurrentMonth);
        } else {
            appAlert("Error al guardar nota: " + (await res.text()), "Error", "❌");
        }
    } catch (e) {
        appAlert("Error de red: " + e, "Error", "❌");
    }
}

async function fetchCalendarStandup() {
    const container = document.getElementById('standup-items-container');
    const refDateEl = document.getElementById('standup-ref-date');
    if (!container) return;

    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = token ? { 'Authorization': 'Bearer ' + token } : {};
        const res = await fetch('/api/calendar/standup', { headers });
        if (!res.ok) {
            container.innerHTML = `<div style="font-size: 11px; color: var(--text-dim); text-align: center; padding: 6px;">Sin pendientes registrados.</div>`;
            return;
        }
        const standup = await res.json();
        if (refDateEl && standup.referenceDate) {
            refDateEl.textContent = `Del ${standup.referenceDate}`;
        }

        const items = standup.pendingItems || [];
        if (items.length === 0) {
            container.innerHTML = `
                <div style="font-size: 11px; color: var(--text-dim); text-align: center; padding: 6px;">
                    ¡Todo al día! No hay tareas pendientes del día anterior.
                </div>
            `;
            return;
        }

        container.innerHTML = items.map(it => `
            <div class="standup-item">
                <div class="standup-item-top">
                    <span class="standup-task-name">${escapeHtml(it.taskName || 'Tarea')}</span>
                    <span style="font-size: 10px; color: var(--accent);">${escapeHtml(it.agent === 'agy' ? '🚀 AGY' : '🤖 Claude')}</span>
                </div>
                <div style="color: var(--text-muted); font-size: 11px;">${escapeHtml(it.pending || it.summary)}</div>
                <div style="display: flex; justify-content: flex-end; margin-top: 4px;">
                    <button class="btn btn-sm btn-ghost" onclick="resumeTaskFromHandoff('${escapeHtml(it.bubbleId || '')}', '${escapeHtml(it.taskName || '')}', '${escapeHtml(it.pending || '')}')" style="padding: 1px 6px; font-size: 10px; color: var(--green);">
                        ▶ Retomar
                    </button>
                </div>
            </div>
        `).join('');
    } catch (e) {
        console.error("Error fetching standup:", e);
        container.innerHTML = `<div style="font-size: 11px; color: var(--text-dim); text-align: center; padding: 6px;">No se pudo cargar el standup.</div>`;
    }
}

function resumeTaskFromHandoff(bubbleId, taskName, pendingText) {
    if (bubbleId) {
        const target = allBubbles.find(b => b.id === bubbleId);
        if (target) {
            selectBubble(target);
            switchWorkspaceTab('terminal');
            showToast(`Tarea seleccionada: ${taskName}. Retomando pendientes.`);
            return;
        }
    }
    switchWorkspaceTab('terminal');
    showToast(`Retomando: ${taskName}`);
}

// ================= HANDOFF MODAL LOGIC =================
function openHandoffModalForActive() {
    const selectEl = document.getElementById('handoff-bubble-select');
    if (selectEl) {
        selectEl.innerHTML = allBubbles.map(b => `
            <option value="${b.id}" ${currentBubble && currentBubble.id === b.id ? 'selected' : ''}>
                ${b.workspace === 'aditum' ? '🏢' : '⚡'} ${escapeHtml(b.name)} (${escapeHtml(b.branch)})
            </option>
        `).join('');
    }

    const dateInput = document.getElementById('handoff-date-input');
    const dateHint = document.getElementById('handoff-date-hint');
    const effectiveDate = getEffectiveHandoffDate();
    if (dateInput) {
        dateInput.value = effectiveDate;
    }
    if (dateHint) {
        if (new Date().getHours() < 5) {
            dateHint.textContent = '🌙 Madrugada: asignado al día hábil anterior';
        } else {
            dateHint.textContent = '';
        }
    }

    const modal = document.getElementById('modal-handoff');
    if (modal) modal.classList.add('open');

    // Automatically generate and pre-fill draft from Git and agent terminal
    const targetBubbleId = (currentBubble && currentBubble.id) || (bubbleSelect ? bubbleSelect.value : '');
    if (targetBubbleId) {
        autoFillHandoffDraft(targetBubbleId);
    }
}

function closeHandoffModal() {
    const modal = document.getElementById('modal-handoff');
    if (modal) modal.classList.remove('open');
}

function onHandoffBubbleChange() {
    const bubbleSelect = document.getElementById('handoff-bubble-select');
    if (!bubbleSelect) return;
    document.getElementById('handoff-summary-input').value = '';
    document.getElementById('handoff-pending-input').value = '';
    document.getElementById('handoff-blockers-input').value = '';
    autoFillHandoffDraft(bubbleSelect.value);
}

async function autoFillHandoffDraft(bubbleId) {
    if (!bubbleId) {
        const bubbleSelect = document.getElementById('handoff-bubble-select');
        if (bubbleSelect) bubbleId = bubbleSelect.value;
    }
    if (!bubbleId) return;

    const summaryEl = document.getElementById('handoff-summary-input');
    const pendingEl = document.getElementById('handoff-pending-input');
    const blockersEl = document.getElementById('handoff-blockers-input');
    const agentSelect = document.getElementById('handoff-agent-select');
    const statusSelect = document.getElementById('handoff-status-select');
    const autoBtn = document.getElementById('btn-handoff-autofill');

    if (summaryEl) summaryEl.placeholder = "⚡ Analizando Git y terminal del agente...";
    if (pendingEl) pendingEl.placeholder = "⚡ Analizando cambios pendientes...";
    if (autoBtn) {
        autoBtn.disabled = true;
        autoBtn.innerHTML = '⚡ Analizando Git & Agente...';
    }

    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = token ? { 'Authorization': 'Bearer ' + token } : {};
        const res = await fetch(`/api/calendar/handoff/draft?bubbleId=${encodeURIComponent(bubbleId)}`, { headers });
        if (res.ok) {
            const draft = await res.json();
            if (summaryEl) summaryEl.value = draft.summary || '';
            if (pendingEl) pendingEl.value = draft.pending || '';
            if (blockersEl) blockersEl.value = draft.blockers || '';
            if (agentSelect && draft.agent) agentSelect.value = draft.agent;
            if (statusSelect && draft.status) statusSelect.value = draft.status;
            showToast("✨ Borrador generado automáticamente de Git y la terminal");
        }
    } catch (e) {
        console.warn("Could not load auto draft:", e);
    } finally {
        if (summaryEl) summaryEl.placeholder = "Ej. Se implementó la lógica de cálculo de mora y se actualizaron los modelos Java...";
        if (pendingEl) pendingEl.placeholder = "Ej. Ejecutar migraciones Liquibase, escribir pruebas unitarias de mora y conectar endpoint...";
        if (autoBtn) {
            autoBtn.disabled = false;
            autoBtn.innerHTML = '⚡ Refrescar Borrador';
        }
    }
}

async function requestAgentRedactHandoff() {
    const bubbleSelect = document.getElementById('handoff-bubble-select');
    const bubbleId = bubbleSelect ? bubbleSelect.value : (currentBubble ? currentBubble.id : '');
    if (!bubbleId) {
        showToast("Selecciona una tarea primero.");
        return;
    }

    const btn = document.getElementById('btn-handoff-prompt-agent');
    if (btn) {
        btn.disabled = true;
        btn.innerHTML = '📨 Enviando a la terminal...';
    }

    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': 'Bearer ' + token } : {})
        };
        const res = await fetch('/api/calendar/handoff/prompt-agent', {
            method: 'POST',
            headers,
            body: JSON.stringify({ bubbleId })
        });

        if (res.ok) {
            const data = await res.json();
            const agentName = data.agent === 'agy' ? 'Antigravity CLI' : 'Claude Code';
            showToast(`📨 Instrucción enviada a ${agentName} (${data.sessionName}).`);

            const statusBox = document.getElementById('handoff-agent-live-status');
            if (statusBox) {
                statusBox.style.display = 'block';
                statusBox.innerHTML = `
                    <div style="font-size: 11px; color: var(--accent); display: flex; align-items: center; gap: 8px;">
                        <span>🤖</span>
                        <span><strong>${agentName}</strong> está redactando y guardando el resumen en la terminal... Puedes cambiar a la terminal para verlo en vivo o esperar aquí.</span>
                    </div>
                `;
            }

            // Poll for completion
            const dateInput = document.getElementById('handoff-date-input');
            const targetDate = (dateInput && dateInput.value) ? dateInput.value : getEffectiveHandoffDate();
            const initialCount = (calCurrentNote && calCurrentNote.handoffs) ? calCurrentNote.handoffs.length : 0;
            let attempts = 0;
            const poller = setInterval(async () => {
                attempts++;
                try {
                    const noteRes = await fetch(`/api/calendar/note?date=${targetDate}`, { headers });
                    if (noteRes.ok) {
                        const noteData = await noteRes.json();
                        if (noteData && noteData.handoffs && noteData.handoffs.length > initialCount) {
                            clearInterval(poller);
                            showToast(`✅ ¡${agentName} guardó el handoff exitosamente!`);
                            closeHandoffModal();
                            await fetchCalendarNote(targetDate);
                            await fetchCalendarMonth(calCurrentYear, calCurrentMonth);
                            await fetchCalendarStandup();
                            return;
                        }
                    }
                } catch(e) {}
                if (attempts >= 40) clearInterval(poller);
            }, 2500);

        } else {
            const errData = await res.json().catch(() => ({}));
            appAlert(errData.error || "No hay una terminal de agente activa en esa tarea.", "Aviso", "ℹ️");
        }
    } catch(e) {
        showToast("Error de conexión: " + e);
    } finally {
        if (btn) {
            btn.disabled = false;
            btn.innerHTML = '🤖 Redactar en Terminal';
        }
    }
}

async function submitHandoffForm() {
    const bubbleId = document.getElementById('handoff-bubble-select').value;
    const agent = document.getElementById('handoff-agent-select').value;
    const status = document.getElementById('handoff-status-select').value;
    const summary = document.getElementById('handoff-summary-input').value.trim();
    const pending = document.getElementById('handoff-pending-input').value.trim();
    const blockers = document.getElementById('handoff-blockers-input').value.trim();

    if (!summary && !pending) {
        appAlert("Por favor ingresa al menos lo completado hoy o lo pendiente para mañana.", "Campos Requeridos", "⚠️");
        return;
    }

    const targetBubble = allBubbles.find(b => b.id === bubbleId);
    const taskName = targetBubble ? targetBubble.name : 'Tarea';

    const dateInput = document.getElementById('handoff-date-input');
    const handoffDate = (dateInput && dateInput.value) ? dateInput.value : getEffectiveHandoffDate();

    const payload = {
        date: handoffDate,
        bubbleId,
        taskName,
        agent,
        status,
        summary,
        pending,
        blockers
    };

    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': 'Bearer ' + token } : {})
        };
        const res = await fetch('/api/calendar/handoff', {
            method: 'POST',
            headers,
            body: JSON.stringify(payload)
        });
        if (res.ok) {
            showToast("Handoff registrado con Git snapshot!");
            closeHandoffModal();
            await fetchCalendarNote(payload.date);
            await fetchCalendarMonth(calCurrentYear, calCurrentMonth);
            await fetchCalendarStandup();
        } else {
            appAlert("Error al registrar handoff: " + (await res.text()), "Error", "❌");
        }
    } catch (e) {
        appAlert("Error de red: " + e, "Error", "❌");
    }
}

// ================= END OF DAY MODAL LOGIC =================
function openEndOfDayModal() {
    const container = document.getElementById('eod-active-tasks-list');
    if (container) {
        // Find bubbles that have running sessions
        const activeBubblesWithTerms = allBubbles.filter(b => {
            const terms = allSessions.filter(s => s.bubbleId === b.id && s.isRunning);
            return terms.length > 0;
        });

        if (activeBubblesWithTerms.length === 0) {
            container.innerHTML = `
                <div style="font-size: 12px; color: var(--text-dim); text-align: center; padding: 12px;">
                    No hay terminales activas en este momento.
                </div>
            `;
        } else {
            container.innerHTML = activeBubblesWithTerms.map(b => {
                const terms = allSessions.filter(s => s.bubbleId === b.id && s.isRunning);
                const claudeCount = terms.filter(s => s.role === 'claude').length;
                const agyCount = terms.filter(s => s.role === 'agy').length;
                let agentBadge = '';
                if (claudeCount > 0) agentBadge += ' 🤖 Claude';
                if (agyCount > 0) agentBadge += ' 🚀 AGY';

                return `
                    <div style="background: var(--bg-base); border: 1px solid var(--border-subtle); border-radius: var(--radius-sm); padding: 8px 12px; display: flex; align-items: center; justify-content: space-between;">
                        <div>
                            <strong style="color: var(--text-main); font-size: 12px;">${b.workspace === 'aditum' ? '🏢' : '⚡'} ${escapeHtml(b.name)}</strong>
                            <span style="font-size: 10px; color: var(--accent); margin-left: 6px;">${agentBadge}</span>
                        </div>
                        <button class="btn btn-sm btn-primary" onclick="closeEndOfDayModal(); selectBubbleById('${b.id}'); openHandoffModalForActive();">
                            🌙 Handoff
                        </button>
                    </div>
                `;
            }).join('');
        }
    }

    const notice = document.getElementById('eod-early-morning-notice');
    const targetDateSpan = document.getElementById('eod-target-date');
    const effectiveDate = getEffectiveHandoffDate();
    if (new Date().getHours() < 5) {
        if (notice) notice.style.display = 'block';
        if (targetDateSpan) targetDateSpan.textContent = effectiveDate;
    } else {
        if (notice) notice.style.display = 'none';
    }

    const modal = document.getElementById('modal-end-of-day');
    if (modal) modal.classList.add('open');
}

function closeEndOfDayModal() {
    const modal = document.getElementById('modal-end-of-day');
    if (modal) modal.classList.remove('open');
}

function selectBubbleById(id) {
    const target = allBubbles.find(b => b.id === id);
    if (target) selectBubble(target);
}

async function disconnectVpnFromEod() {
    try {
        showToast("Desconectando Windscribe VPN...");
        await toggleVPN('disconnect');
        const btn = document.getElementById('eod-disconnect-vpn-btn');
        if (btn) {
            btn.textContent = "✓ Desconectado";
            btn.disabled = true;
        }
    } catch (e) {
        showToast("Error al desconectar VPN: " + e);
    }
}

async function autoSaveAllActiveHandoffs() {
    const activeBubblesWithTerms = allBubbles.filter(b => {
        const terms = allSessions.filter(s => s.bubbleId === b.id && s.isRunning);
        return terms.length > 0;
    });

    if (activeBubblesWithTerms.length === 0) {
        showToast("No hay tareas con terminales activas hoy");
        return;
    }

    const btn = document.getElementById('eod-autosave-all-btn');
    if (btn) {
        btn.disabled = true;
        btn.innerHTML = '⚡ Guardando handoffs...';
    }

    let savedCount = 0;
    const token = serverInfo ? serverInfo.authToken : '';
    const headers = {
        'Content-Type': 'application/json',
        ...(token ? { 'Authorization': 'Bearer ' + token } : {})
    };

    const targetDate = getEffectiveHandoffDate();

    for (const b of activeBubblesWithTerms) {
        try {
            const res = await fetch('/api/calendar/handoff/auto', {
                method: 'POST',
                headers,
                body: JSON.stringify({
                    bubbleId: b.id,
                    date: targetDate
                })
            });
            if (res.ok) savedCount++;
        } catch(e) {}
    }

    if (btn) {
        btn.disabled = false;
        btn.innerHTML = '⚡ Auto-guardar Handoff de todas';
    }

    showToast(`Se registraron ${savedCount} handoffs automáticamente en el calendario`);
    closeEndOfDayModal();
    await fetchCalendarNote(targetDate);
    await fetchCalendarMonth(calCurrentYear, calCurrentMonth);
    await fetchCalendarStandup();
    switchWorkspaceTab('calendar');
}

function finishEndOfDayFlow() {
    closeEndOfDayModal();
    showToast("¡Excelente trabajo hoy! Que descanses.");
    switchWorkspaceTab('calendar');
}
