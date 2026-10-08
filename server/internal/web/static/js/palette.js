// ==========================================
// Command Palette (Ctrl+K)
// ==========================================

// Command Palette State
let cmdPaletteItems = [];
let cmdPaletteFiltered = [];
let cmdPaletteSelectedIndex = 0;

// Notifications & Audio


// ==========================================
// QoL 2: Command Palette (Ctrl+K)
// ==========================================
function openCommandPalette() {
    const modal = document.getElementById('command-palette-modal');
    const input = document.getElementById('cmd-palette-input');
    if (!modal) return;
    modal.style.display = 'flex';
    buildCmdPaletteItems();
    filterCommandPalette('');
    if (input) {
        input.value = '';
        setTimeout(() => input.focus(), 60);
    }
}

function closeCommandPalette() {
    const modal = document.getElementById('command-palette-modal');
    if (modal) modal.style.display = 'none';
}

function buildCmdPaletteItems() {
    cmdPaletteItems = [];

    // 1. Bubbles / Worktrees
    allBubbles.forEach(b => {
        cmdPaletteItems.push({
            cat: 'Burbujas y Tareas',
            title: `${b.name} (${b.workspace.toUpperCase()})`,
            desc: `Rama: ${b.branch || 'main'} · Puerto :${b.port || 8080}`,
            icon: '🫧',
            action: () => {
                selectBubble(b.id);
                closeCommandPalette();
            }
        });
    });

    // 2. Navigation
    cmdPaletteItems.push(
        { cat: 'Vistas', title: 'Ir a Terminales', desc: 'Panel interactivo de terminales', icon: '>_', shortcut: '', action: () => { switchWorkspaceTab('terminal'); closeCommandPalette(); } },
        { cat: 'Vistas', title: 'Ir a Live Preview', desc: 'Visualizador embebido de la app local', icon: '🌐', shortcut: '', action: () => { switchWorkspaceTab('preview'); closeCommandPalette(); } },
        { cat: 'Vistas', title: 'Ir a Git & Diff', desc: 'Revisión de archivos modificados y commits', icon: '🔀', shortcut: '', action: () => { switchWorkspaceTab('git'); closeCommandPalette(); } },
        { cat: 'Vistas', title: 'Ir a Obsidian Runbooks', desc: 'Notas de arquitectura y runbooks', icon: '📓', shortcut: '', action: () => { switchWorkspaceTab('obsidian'); closeCommandPalette(); } }
    );

    // 3. Workspaces
    cmdPaletteItems.push(
        { cat: 'Workspaces', title: 'Filtrar: Solo con Terminales Abiertas', desc: 'Ver únicamente tareas con consolas o Claude activos', icon: '💻', shortcut: '', action: () => { setTaskTermFilter(true); closeCommandPalette(); } },
        { cat: 'Workspaces', title: 'Filtrar: Ver Todas las Tareas', desc: 'Quitar filtro de terminales abiertas', icon: '📋', shortcut: '', action: () => { setTaskTermFilter(false); closeCommandPalette(); } },
        { cat: 'Workspaces', title: 'Filtrar por Aditum', desc: 'Ver solo tareas de Aditum', icon: '🏢', shortcut: '', action: () => { setWorkspaceFilter('aditum'); closeCommandPalette(); } },
        { cat: 'Workspaces', title: 'Filtrar por Brif', desc: 'Ver solo tareas de Brif', icon: '⚡', shortcut: '', action: () => { setWorkspaceFilter('brif'); closeCommandPalette(); } },
        { cat: 'Workspaces', title: 'Ver Todos los Workspaces', desc: 'Mostrar todas las tareas', icon: '🌐', shortcut: '', action: () => { setWorkspaceFilter('all'); closeCommandPalette(); } }
    );

    // 4. Terminal & Shell actions
    cmdPaletteItems.push(
        { cat: 'Terminal', title: 'Buscar en Buffer de Terminal', desc: 'Resalta texto en la pantalla actual', icon: '🔍', shortcut: 'Ctrl+F', action: () => { closeCommandPalette(); openTerminalSearch(); } },
        { cat: 'Terminal', title: 'Dividir Horizontalmente', desc: 'Panel lado a lado', icon: '🪟', shortcut: '', action: () => { closeCommandPalette(); triggerSplit('horizontal'); } },
        { cat: 'Terminal', title: 'Dividir Verticalmente', desc: 'Panel arriba y abajo', icon: '🪜', shortcut: '', action: () => { closeCommandPalette(); triggerSplit('vertical'); } },
        { cat: 'Terminal', title: 'Unir / Cerrar Split', desc: 'Volver a terminal única', icon: '🔄', shortcut: '', action: () => { closeCommandPalette(); closeSplit(); } },
        { cat: 'Terminal', title: 'Lanzar Claude Code', desc: 'Nueva sesión de Claude', icon: '🤖', shortcut: '', action: () => { closeCommandPalette(); spawnClaudeInBubble(); } },
        { cat: 'Terminal', title: 'Lanzar Antigravity CLI', desc: 'Nueva sesión de Google Antigravity (agy)', icon: '🚀', shortcut: '', action: () => { closeCommandPalette(); spawnAgyInBubble(); } },
        { cat: 'Terminal', title: 'Lanzar Shell', desc: 'Nueva sesión PowerShell/Bash', icon: '🐚', shortcut: '', action: () => { closeCommandPalette(); spawnShellInBubble(); } },
        { cat: 'Terminal', title: 'Limpiar Buffer', desc: 'Clear pantalla', icon: '🧹', shortcut: '', action: () => { closeCommandPalette(); clearActiveTerminal(); } },
        { cat: 'Workspaces', title: 'Ver Calendario & Handoffs', desc: 'Diario de trabajo y pendientes del día', icon: '📅', shortcut: '', action: () => { closeCommandPalette(); switchWorkspaceTab('calendar'); } },
        { cat: 'Workspaces', title: 'Finalizar Jornada (Cierre del Día)', desc: 'Checklist de cierre y handoffs', icon: '🌙', shortcut: '', action: () => { closeCommandPalette(); openEndOfDayModal(); } }
    );

    // 5. System & VPN
    cmdPaletteItems.push(
        { cat: 'Sistema', title: 'Ocultar / Mostrar Sidebar', desc: 'Colapsar barra lateral izquierda', icon: '📂', shortcut: 'Ctrl+B', action: () => { closeCommandPalette(); toggleSidebar(); } },
        { cat: 'Sistema', title: 'Alternar VPN Windscribe', desc: 'Abrir diálogo de conexión VPN', icon: '🛡️', shortcut: '', action: () => { closeCommandPalette(); openVPNModal(); } },
        { cat: 'Sistema', title: 'Conectar Teléfono Móvil', desc: 'Ver QR y credenciales de pairing', icon: '📱', shortcut: '', action: () => { closeCommandPalette(); openPairModal(); } },
        { cat: 'Sistema', title: 'Copiar Token de Acceso', desc: 'Copiar token de autorización', icon: '📋', shortcut: '', action: () => { closeCommandPalette(); copyToken(); } },
        { cat: 'Sistema', title: 'Alternar Alertas Sonoras', desc: 'Activar o desactivar sonido de terminal', icon: '🔔', shortcut: '', action: () => { closeCommandPalette(); toggleNotifications(); } }
    );
}

function filterCommandPalette(q) {
    const query = (q || '').toLowerCase().trim();
    if (!query) {
        cmdPaletteFiltered = [...cmdPaletteItems];
    } else {
        cmdPaletteFiltered = cmdPaletteItems.filter(item => 
            item.title.toLowerCase().includes(query) ||
            (item.desc && item.desc.toLowerCase().includes(query)) ||
            item.cat.toLowerCase().includes(query)
        );
    }
    cmdPaletteSelectedIndex = 0;
    renderCmdPaletteList();
}

function renderCmdPaletteList() {
    const container = document.getElementById('cmd-palette-list');
    if (!container) return;

    if (cmdPaletteFiltered.length === 0) {
        container.innerHTML = `<div style="padding: 16px; text-align: center; color: var(--text-dim); font-size: 13px;">No se encontraron comandos o tareas</div>`;
        return;
    }

    let html = '';
    let currentCat = '';
    cmdPaletteFiltered.forEach((item, idx) => {
        if (item.cat !== currentCat) {
            currentCat = item.cat;
            html += `<div class="cmd-palette-category">${currentCat}</div>`;
        }
        const isSelected = idx === cmdPaletteSelectedIndex;
        html += `
            <div class="cmd-palette-item ${isSelected ? 'selected' : ''}" onclick="executeCmdPaletteItem(${idx})">
                <div class="cmd-palette-item-left">
                    <span style="font-size: 14px;">${item.icon}</span>
                    <div>
                        <div style="font-weight: 600;">${escapeHtml(item.title)}</div>
                        ${item.desc ? `<div style="font-size: 11px; color: var(--text-dim);">${escapeHtml(item.desc)}</div>` : ''}
                    </div>
                </div>
                ${item.shortcut ? `<span class="cmd-palette-shortcut">${escapeHtml(item.shortcut)}</span>` : ''}
            </div>
        `;
    });
    container.innerHTML = html;

    const selectedEl = container.querySelector('.cmd-palette-item.selected');
    if (selectedEl) {
        selectedEl.scrollIntoView({ block: 'nearest' });
    }
}

function handleCmdPaletteKey(e) {
    if (e.key === 'ArrowDown') {
        e.preventDefault();
        if (cmdPaletteSelectedIndex < cmdPaletteFiltered.length - 1) {
            cmdPaletteSelectedIndex++;
            renderCmdPaletteList();
        }
    } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        if (cmdPaletteSelectedIndex > 0) {
            cmdPaletteSelectedIndex--;
            renderCmdPaletteList();
        }
    } else if (e.key === 'Enter') {
        e.preventDefault();
        executeCmdPaletteItem(cmdPaletteSelectedIndex);
    } else if (e.key === 'Escape') {
        closeCommandPalette();
    }
}

function executeCmdPaletteItem(idx) {
    const item = cmdPaletteFiltered[idx];
    if (item && item.action) {
        item.action();
    }
}
