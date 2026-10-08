// ==========================================
// Secondary Views (Live Preview, Git Diff, Obsidian)
// ==========================================

// Obsidian State
let currentVault = 'aditum';
let currentNotePath = '';
let isNoteEditing = false;

// Git State
let currentGitStatus = null;
let selectedDiffFile = null;



// Live Preview Controls
function reloadPreviewFrame() {
    const iframe = document.getElementById('preview-iframe');
    const url = document.getElementById('preview-url-input').value;
    iframe.src = url;
}

function navigateToPreviewUrl() {
    reloadPreviewFrame();
}

function openExternalPreview() {
    const url = document.getElementById('preview-url-input').value;
    window.open(url, '_blank');
}

// Git & Diff Tab
async function loadGitStatus() {
    if (!currentBubble) return;
    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = token ? { 'Authorization': 'Bearer ' + token } : {};
        const res = await fetch(`/api/bubbles/${currentBubble.id}/git/status`, { headers });
        if (!res.ok) return;
        currentGitStatus = await res.json();
        renderGitFilesList();
    } catch (e) {
        console.error("Error al obtener git status:", e);
    }
}

function renderGitFilesList() {
    const container = document.getElementById('git-files-container');
    if (!currentGitStatus || !currentGitStatus.changes || currentGitStatus.changes.length === 0) {
        container.innerHTML = `
            <div style="padding: 24px 12px; text-align: center; color: var(--text-dim); font-size: 12px;">
                Árbol de trabajo limpio.<br>No hay archivos modificados.
            </div>
        `;
        document.getElementById('git-diff-container').innerHTML = `
            <div style="color: var(--text-dim); text-align: center; padding: 40px;">
                Árbol limpio (Branch <strong>${escapeHtml(currentGitStatus?.branch || '')}</strong>).
            </div>
        `;
        return;
    }

    container.innerHTML = currentGitStatus.changes.map(ch => {
        const isSel = selectedDiffFile === ch.path;
        let badgeClass = 'git-status-badge';
        let badgeColor = 'var(--accent)';
        if (ch.status === 'M') badgeColor = 'var(--amber)';
        else if (ch.status === 'A') badgeColor = 'var(--green)';
        else if (ch.status === 'D') badgeColor = 'var(--red)';

        return `
            <div class="git-file-item ${isSel ? 'active' : ''}" onclick="viewFileDiff('${escapeHtml(ch.path)}')">
                <span style="overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 210px;" title="${escapeHtml(ch.path)}">${escapeHtml(ch.path)}</span>
                <span class="${badgeClass}" style="color: ${badgeColor};">${ch.status}</span>
            </div>
        `;
    }).join('');
}

async function viewFileDiff(filePath) {
    selectedDiffFile = filePath;
    renderGitFilesList();

    const diffContainer = document.getElementById('git-diff-container');
    diffContainer.innerHTML = `<div style="color: var(--text-dim); padding: 20px;">Cargando diff de ${escapeHtml(filePath)}...</div>`;

    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = token ? { 'Authorization': 'Bearer ' + token } : {};
        const res = await fetch(`/api/bubbles/${currentBubble.id}/git/diff?file=${encodeURIComponent(filePath)}`, { headers });
        if (!res.ok) {
            diffContainer.textContent = "Error al obtener diff.";
            return;
        }
        const data = await res.json();
        renderFormattedDiff(data.diff);
    } catch (e) {
        diffContainer.textContent = "Error: " + e;
    }
}

function renderFormattedDiff(diffText) {
    const container = document.getElementById('git-diff-container');
    if (!diffText || diffText.trim() === '') {
        container.innerHTML = `<div style="color: var(--text-dim); padding: 20px;">Sin diferencias con HEAD en este archivo.</div>`;
        return;
    }

    const lines = diffText.split('\n');
    const formatted = lines.map(line => {
        if (line.startsWith('+') && !line.startsWith('+++')) {
            return `<span class="diff-add">${escapeHtml(line)}</span>`;
        } else if (line.startsWith('-') && !line.startsWith('---')) {
            return `<span class="diff-del">${escapeHtml(line)}</span>`;
        } else if (line.startsWith('@@')) {
            return `<span class="diff-header">${escapeHtml(line)}</span>`;
        }
        return `<span>${escapeHtml(line)}</span>`;
    }).join('\n');

    container.innerHTML = formatted;
}

// Obsidian Tab
async function switchObsidianVault(vault) {
    currentVault = vault;
    document.getElementById('btn-vault-aditum').className = `ws-tab-btn ${vault === 'aditum' ? 'active' : ''}`;
    document.getElementById('btn-vault-brif').className = `ws-tab-btn ${vault === 'brif' ? 'active' : ''}`;
    await searchObsidianNotes('');
}

let searchDebounceTimer = null;
function onObsidianSearch(query) {
    clearTimeout(searchDebounceTimer);
    searchDebounceTimer = setTimeout(() => {
        searchObsidianNotes(query);
    }, 250);
}

async function searchObsidianNotes(q) {
    const container = document.getElementById('obsidian-notes-container');
    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = token ? { 'Authorization': 'Bearer ' + token } : {};
        const res = await fetch(`/api/vaults/${currentVault}/notes?q=${encodeURIComponent(q)}`, { headers });
        if (!res.ok) return;
        const notes = await res.json();
        renderObsidianNotes(notes);
    } catch (e) {
        container.innerHTML = `<div style="color: var(--red); padding: 10px;">Error al buscar notas</div>`;
    }
}

function renderObsidianNotes(notes) {
    const container = document.getElementById('obsidian-notes-container');
    if (!notes || notes.length === 0) {
        container.innerHTML = `
            <div style="padding: 20px 10px; text-align: center; color: var(--text-dim); font-size: 12px;">
                No se encontraron notas en esta vault.
            </div>
        `;
        return;
    }

    container.innerHTML = notes.map(n => {
        const isSel = currentNotePath === n.relative;
        return `
            <div class="obsidian-note-item ${isSel ? 'active' : ''}" onclick="readObsidianNote('${escapeHtml(n.relative)}')">
                <span class="obsidian-note-title">${escapeHtml(n.title)}</span>
                <span class="obsidian-note-path" title="${escapeHtml(n.relative)}">${escapeHtml(n.relative)}</span>
            </div>
        `;
    }).join('');
}

async function readObsidianNote(relPath) {
    currentNotePath = relPath;
    isNoteEditing = false;
    document.getElementById('btn-obsidian-edit-mode').textContent = "✏️ Editar";
    document.getElementById('btn-obsidian-save').style.display = "none";
    document.getElementById('obsidian-raw-editor').style.display = "none";
    document.getElementById('obsidian-markdown-body').style.display = "block";

    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = token ? { 'Authorization': 'Bearer ' + token } : {};
        const res = await fetch(`/api/vaults/${currentVault}/note?path=${encodeURIComponent(relPath)}`, { headers });
        if (!res.ok) return;
        const note = await res.json();

        document.getElementById('obsidian-current-title').textContent = note.title;
        document.getElementById('obsidian-current-path').textContent = note.relative;

        // Render markdown via marked
        if (typeof marked !== 'undefined') {
            document.getElementById('obsidian-markdown-body').innerHTML = marked.parse(note.content);
        } else {
            document.getElementById('obsidian-markdown-body').innerText = note.content;
        }

        document.getElementById('obsidian-raw-editor').value = note.content;
    } catch (e) {
        appAlert("Error al abrir nota: " + e, "Error en Obsidian", "❌");
    }
}

function toggleObsidianEditMode() {
    isNoteEditing = !isNoteEditing;
    const mdBody = document.getElementById('obsidian-markdown-body');
    const rawEdit = document.getElementById('obsidian-raw-editor');
    const saveBtn = document.getElementById('btn-obsidian-save');
    const editBtn = document.getElementById('btn-obsidian-edit-mode');

    if (isNoteEditing) {
        mdBody.style.display = 'none';
        rawEdit.style.display = 'block';
        saveBtn.style.display = 'inline-flex';
        editBtn.textContent = "👁️ Vista Previa";
        rawEdit.focus();
    } else {
        if (typeof marked !== 'undefined') {
            mdBody.innerHTML = marked.parse(rawEdit.value);
        }
        mdBody.style.display = 'block';
        rawEdit.style.display = 'none';
        saveBtn.style.display = 'none';
        editBtn.textContent = "✏️ Editar";
    }
}

async function saveCurrentObsidianNote() {
    if (!currentNotePath) return;
    const content = document.getElementById('obsidian-raw-editor').value;
    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': 'Bearer ' + token } : {})
        };
        const res = await fetch(`/api/vaults/${currentVault}/note`, {
            method: 'POST',
            headers,
            body: JSON.stringify({ path: currentNotePath, content })
        });
        if (res.ok) {
            showToast("Nota guardada en Obsidian!");
            toggleObsidianEditMode();
        } else {
            appAlert("Error al guardar nota: " + (await res.text()), "Error en Obsidian", "❌");
        }
    } catch (e) {
        appAlert("Error de red: " + e, "Error de Conexión", "❌");
    }
}
