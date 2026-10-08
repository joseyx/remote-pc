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

function getTodayFormatted() {
    const d = new Date();
    const y = d.getFullYear();
    const m = String(d.getMonth() + 1).padStart(2, '0');
    const day = String(d.getDate()).padStart(2, '0');
    return `${y}-${m}-${day}`;
}

async function loadCalendarView() {
    const today = getTodayFormatted();
    if (!calSelectedDate) calSelectedDate = today;
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
    const renderedEl = document.getElementById('cal-rendered-note');
    const rawEl = document.getElementById('cal-raw-editor');
    const handoffsEl = document.getElementById('cal-handoffs-container');
    const countBadge = document.getElementById('cal-handoffs-count-badge');

    if (rawEl) rawEl.value = note.content || '';

    if (renderedEl) {
        if (typeof marked !== 'undefined') {
            renderedEl.innerHTML = marked.parse(note.content || '*Sin notas para este día.*');
        } else {
            renderedEl.textContent = note.content || 'Sin notas para este día.';
        }
    }

    // Render structured handoffs
    const handoffs = note.handoffs || [];
    if (countBadge) {
        countBadge.textContent = `${handoffs.length} registrado${handoffs.length === 1 ? '' : 's'}`;
    }

    if (handoffsEl) {
        if (handoffs.length === 0) {
            handoffsEl.innerHTML = `
                <div style="color: var(--text-dim); font-size: 12px; text-align: center; padding: 18px;">
                    No hay handoffs registrados para este día. Pulsa "🌙 + Nuevo Handoff" para agregar uno.
                </div>
            `;
        } else {
            handoffsEl.innerHTML = `
                <div class="handoff-list">
                    ${handoffs.map(h => renderHandoffCardHtml(h)).join('')}
                </div>
            `;
        }
    }
}

function renderHandoffCardHtml(h) {
    let statusClass = 'in_progress';
    let statusText = '🟡 En Progreso';
    if (h.status === 'completed') {
        statusClass = 'completed';
        statusText = '🟢 Completado';
    } else if (h.status === 'blocked') {
        statusClass = 'blocked';
        statusText = '🔴 Bloqueado';
    }

    let agentIcon = '🤖 Claude Code';
    let agentClass = '';
    if (h.agent === 'agy') {
        agentIcon = '🚀 Antigravity CLI';
        agentClass = 'agy';
    } else if (h.agent === 'developer') {
        agentIcon = '👤 Desarrollador';
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

    return `
        <div class="handoff-card">
            <div class="handoff-card-header">
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
                        <span>${escapeHtml(h.summary)}</span>
                    </div>
                ` : ''}
                ${h.pending ? `
                    <div class="handoff-section-row">
                        <span class="handoff-section-label" style="color: var(--accent);">⏳ Pendiente para mañana:</span>
                        <strong style="color: var(--text-main);">${escapeHtml(h.pending)}</strong>
                    </div>
                ` : ''}
                ${h.blockers ? `
                    <div class="handoff-section-row">
                        <span class="handoff-section-label" style="color: var(--red);">⛔ Puntos de atención:</span>
                        <span style="color: #fb7185;">${escapeHtml(h.blockers)}</span>
                    </div>
                ` : ''}
                ${gitBar}
            </div>
            <div style="display: flex; justify-content: flex-end; margin-top: 4px;">
                <button class="btn btn-sm btn-ghost" onclick="resumeTaskFromHandoff('${escapeHtml(h.bubbleId || '')}', '${escapeHtml(h.taskName || '')}', '${escapeHtml(h.pending || '')}')" style="font-size: 11px; color: var(--accent);">
                    ▶ Retomar esta Tarea
                </button>
            </div>
        </div>
    `;
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

    // Preset agent based on active terminal session role
    const agentSelect = document.getElementById('handoff-agent-select');
    if (agentSelect && activeBubbleSession) {
        if (activeBubbleSession.role === 'agy') agentSelect.value = 'agy';
        else if (activeBubbleSession.role === 'claude') agentSelect.value = 'claude';
        else agentSelect.value = 'developer';
    }

    document.getElementById('handoff-summary-input').value = '';
    document.getElementById('handoff-pending-input').value = '';
    document.getElementById('handoff-blockers-input').value = '';

    const modal = document.getElementById('modal-handoff');
    if (modal) modal.classList.add('open');
}

function closeHandoffModal() {
    const modal = document.getElementById('modal-handoff');
    if (modal) modal.classList.remove('open');
}

function onHandoffBubbleChange() {
    // Can adapt presets if needed
}

function requestAgentSummaryForHandoff() {
    // Find active terminal session in selected bubble
    const bubbleId = document.getElementById('handoff-bubble-select').value;
    const bSessions = allSessions.filter(s => s.bubbleId === bubbleId && s.isRunning);
    const agentSess = bSessions.find(s => s.role === 'claude' || s.role === 'agy') || bSessions[0];

    if (!agentSess) {
        showToast("No hay ninguna terminal activa en esa tarea para solicitar el resumen.");
        return;
    }

    // Send prompt to the terminal socket if connected or notify
    const prompt = "Por favor genera un resumen de cierre de jornada (handoff): 1) Estado del ticket, 2) Lo completado hoy, 3) Lo pendiente para mañana.\r";
    if (ws && ws.readyState === WebSocket.OPEN && currentSessionId === agentSess.id) {
        ws.send(new TextEncoder().encode(prompt));
        showToast("Prompt de handoff enviado a la terminal activa.");
    } else {
        showToast(`Abre la terminal de ${agentSess.name} y pide al agente el resumen de fin de día.`);
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

    const payload = {
        date: calSelectedDate || getTodayFormatted(),
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

function finishEndOfDayFlow() {
    closeEndOfDayModal();
    showToast("¡Excelente trabajo hoy! Que descanses.");
    switchWorkspaceTab('calendar');
}
