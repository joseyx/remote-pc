// ==========================================
// Orbit Plugins & Extensions Engine (pear-desktop style)
// ==========================================

let allPlugins = [];
let pendingPluginsRestart = false;
let currentTranscribeFile = null;
let lastTranscriptionResult = null;

// Open & Close Plugins Modal
async function openPluginsModal() {
    const modal = document.getElementById('modal-plugins');
    if (!modal) return;
    modal.classList.add('open');
    modal.style.display = 'flex';
    await loadPlugins();
}

function closePluginsModal() {
    const modal = document.getElementById('modal-plugins');
    if (modal) {
        modal.classList.remove('open');
        modal.style.display = 'none';
    }
}

// Fetch Plugins from Server
async function loadPlugins() {
    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = token ? { 'Authorization': 'Bearer ' + token } : {};
        const res = await fetch('/api/plugins', { headers });
        if (!res.ok) throw new Error("Error HTTP " + res.status);
        allPlugins = await res.json();
        renderPluginsList();
    } catch (err) {
        console.error("Failed to load plugins:", err);
        const container = document.getElementById('plugins-list-container');
        if (container) {
            container.innerHTML = `<div style="text-align: center; color: var(--danger); padding: 20px;">Error al cargar plugins: ${err.message}</div>`;
        }
    }
}

// Render Plugins Cards (pear-desktop style with switch and expandable settings)
function renderPluginsList() {
    const container = document.getElementById('plugins-list-container');
    if (!container) return;

    // Show/hide restart banner
    const banner = document.getElementById('plugins-restart-banner');
    if (banner) {
        banner.style.display = pendingPluginsRestart ? 'flex' : 'none';
    }

    if (!allPlugins || allPlugins.length === 0) {
        container.innerHTML = `<div style="text-align: center; color: var(--text-muted); padding: 30px;">No hay plugins registrados.</div>`;
        return;
    }

    container.innerHTML = allPlugins.map(p => {
        const isEnabled = !!p.enabled;
        const configFields = p.configSchema || [];
        const hasConfig = configFields.length > 0;

        return `
            <div class="plugin-card ${isEnabled ? 'enabled' : ''}" id="plugin-card-${p.id}">
                <div class="plugin-card-header">
                    <div class="plugin-meta">
                        <span class="plugin-icon">${p.icon || '🧩'}</span>
                        <div class="plugin-title-block">
                            <div class="plugin-title-row">
                                <strong class="plugin-title">${p.name}</strong>
                                <span class="badge-ws" style="font-size: 10px;">${p.category || 'Extensión'}</span>
                                <span style="font-size: 11px; color: var(--text-dim);">v${p.version || '1.0'}</span>
                            </div>
                            <p class="plugin-desc">${p.description}</p>
                        </div>
                    </div>
                    <div class="plugin-actions-top">
                        <!-- Toggle Switch -->
                        <label class="plugin-switch" title="${isEnabled ? 'Desactivar plugin' : 'Activar plugin'}">
                            <input type="checkbox" ${isEnabled ? 'checked' : ''} onchange="togglePluginState('${p.id}', this.checked)">
                            <span class="plugin-slider"></span>
                        </label>
                    </div>
                </div>

                <!-- Plugin Toolbar: Quick launch buttons if enabled -->
                <div class="plugin-quick-actions">
                    ${p.id === 'groq_whisper' && isEnabled ? `
                        <button class="btn btn-sm btn-primary" onclick="openTranscribeModal(); closePluginsModal();" style="padding: 2px 10px; font-size: 11px;">
                            🎙️ Abrir Transcriptor Web
                        </button>
                    ` : ''}

                    ${hasConfig ? `
                        <button class="btn btn-sm btn-ghost" onclick="togglePluginConfigPanel('${p.id}')" id="btn-cfg-toggle-${p.id}" style="padding: 2px 8px; font-size: 11px; margin-left: auto;">
                            ⚙️ Ajustes ▼
                        </button>
                    ` : ''}
                </div>

                <!-- Collapsible Settings Panel -->
                ${hasConfig ? `
                    <div class="plugin-config-panel" id="plugin-config-${p.id}" style="display: none;">
                        <div class="plugin-config-form">
                            ${configFields.map(f => renderConfigField(p.id, f, p.config ? p.config[f.key] : '')).join('')}
                        </div>
                        <div style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 12px; border-top: 1px solid var(--border-subtle); padding-top: 10px;">
                            <button class="btn btn-sm" onclick="togglePluginConfigPanel('${p.id}')">Cancelar</button>
                            <button class="btn btn-sm btn-success" onclick="savePluginConfig('${p.id}')">💾 Guardar Configuración</button>
                        </div>
                    </div>
                ` : ''}
            </div>
        `;
    }).join('');
}

// Render dynamic configuration fields
function renderConfigField(pluginId, field, currentValue) {
    const fieldId = `cfg-${pluginId}-${field.key}`;
    const val = currentValue !== undefined ? currentValue : (field.default || '');

    let inputHtml = '';
    if (field.type === 'select') {
        const options = (field.options || []).map(opt => {
            const selected = opt === val ? 'selected' : '';
            return `<option value="${opt}" ${selected}>${opt}</option>`;
        }).join('');
        inputHtml = `<select id="${fieldId}" class="form-control" style="font-size: 12px; height: 30px; background: var(--bg-surface);">${options}</select>`;
    } else if (field.type === 'checkbox') {
        const checked = val ? 'checked' : '';
        inputHtml = `
            <label style="display: flex; align-items: center; gap: 8px; cursor: pointer; font-size: 12px; color: var(--text-main);">
                <input type="checkbox" id="${fieldId}" ${checked}> ${field.label}
            </label>
        `;
    } else if (field.type === 'password') {
        inputHtml = `
            <div style="position: relative; display: flex; align-items: center;">
                <input type="password" id="${fieldId}" value="${val}" placeholder="${field.placeholder || ''}" class="form-control" style="font-size: 12px; padding-right: 32px; font-family: monospace;">
                <button type="button" class="btn btn-sm btn-ghost" onclick="togglePasswordVisibility('${fieldId}')" style="position: absolute; right: 2px; padding: 2px 6px; font-size: 12px;" title="Mostrar / Ocultar">👁️</button>
            </div>
        `;
    } else {
        inputHtml = `<input type="${field.type || 'text'}" id="${fieldId}" value="${val}" placeholder="${field.placeholder || ''}" class="form-control" style="font-size: 12px;">`;
    }

    if (field.type === 'checkbox') {
        return `
            <div class="plugin-field-row" style="margin-bottom: 8px;">
                ${inputHtml}
                ${field.help ? `<small style="color: var(--text-dim); font-size: 11px; display: block; margin-top: 2px;">${field.help}</small>` : ''}
            </div>
        `;
    }

    return `
        <div class="plugin-field-row" style="margin-bottom: 10px;">
            <label style="display: block; font-size: 11px; font-weight: 600; color: var(--text-muted); margin-bottom: 4px;">
                ${field.label} ${field.required ? '<span style="color: var(--danger);">*</span>' : ''}
            </label>
            ${inputHtml}
            ${field.help ? `<small style="color: var(--text-dim); font-size: 11px; display: block; margin-top: 3px;">${field.help}</small>` : ''}
        </div>
    `;
}

// Toggle password input visibility
function togglePasswordVisibility(inputId) {
    const el = document.getElementById(inputId);
    if (!el) return;
    el.type = el.type === 'password' ? 'text' : 'password';
}

// Toggle expandable config panel
function togglePluginConfigPanel(pluginId) {
    const panel = document.getElementById(`plugin-config-${pluginId}`);
    const btn = document.getElementById(`btn-cfg-toggle-${pluginId}`);
    if (!panel) return;

    if (panel.style.display === 'none' || panel.style.display === '') {
        panel.style.display = 'block';
        if (btn) btn.innerHTML = '⚙️ Ajustes ▲';
    } else {
        panel.style.display = 'none';
        if (btn) btn.innerHTML = '⚙️ Ajustes ▼';
    }
}

// Toggle Plugin ON / OFF
async function togglePluginState(pluginId, isChecked) {
    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': 'Bearer ' + token } : {})
        };

        const res = await fetch(`/api/plugins/${pluginId}/toggle`, {
            method: 'POST',
            headers,
            body: JSON.stringify({ enabled: isChecked })
        });

        if (!res.ok) {
            const errData = await res.json().catch(() => ({}));
            throw new Error(errData.error || `Error HTTP ${res.status}`);
        }

        const data = await res.json();
        if (data.requiresRestart) {
            pendingPluginsRestart = true;
        }

        // Update local object
        const p = allPlugins.find(x => x.id === pluginId);
        if (p) p.enabled = isChecked;

        renderPluginsList();

        if (typeof showToast === 'function') {
            showToast(isChecked ? `Plugin "${p ? p.name : pluginId}" activado` : `Plugin desactivado`, "info");
        }
    } catch (err) {
        console.error("Error toggling plugin:", err);
        alert("Error al cambiar estado del plugin: " + err.message);
        loadPlugins(); // revert checkbox
    }
}

// Save Plugin Configuration
async function savePluginConfig(pluginId) {
    const plugin = allPlugins.find(x => x.id === pluginId);
    if (!plugin) return;

    const fields = plugin.configSchema || [];
    const newConfig = {};

    for (const f of fields) {
        const fieldEl = document.getElementById(`cfg-${pluginId}-${f.key}`);
        if (!fieldEl) continue;

        if (f.type === 'checkbox') {
            newConfig[f.key] = fieldEl.checked;
        } else if (f.type === 'number') {
            newConfig[f.key] = parseFloat(fieldEl.value) || 0;
        } else {
            newConfig[f.key] = fieldEl.value.trim();
        }
    }

    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': 'Bearer ' + token } : {})
        };

        const res = await fetch(`/api/plugins/${pluginId}/config`, {
            method: 'POST',
            headers,
            body: JSON.stringify({ config: newConfig })
        });

        if (!res.ok) {
            const errData = await res.json().catch(() => ({}));
            throw new Error(errData.error || `Error HTTP ${res.status}`);
        }

        const data = await res.json();
        if (data.requiresRestart) {
            pendingPluginsRestart = true;
        }

        // Close panel and refresh
        togglePluginConfigPanel(pluginId);
        await loadPlugins();

        if (typeof showToast === 'function') {
            showToast("Configuración guardada exitosamente", "success");
        }
    } catch (err) {
        console.error("Error saving plugin config:", err);
        alert("Error al guardar configuración: " + err.message);
    }
}

// Restart Orbit daemon to apply MCP changes immediately
async function restartOrbitServiceFromPlugins() {
    const btn = document.getElementById('btn-restart-from-plugins');
    if (btn) {
        btn.disabled = true;
        btn.innerHTML = '🔄 Reiniciando...';
    }

    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': 'Bearer ' + token } : {})
        };

        await fetch('/api/system/service', {
            method: 'POST',
            headers,
            body: JSON.stringify({ action: 'restart' })
        });

        if (typeof showToast === 'function') {
            showToast("Reiniciando Orbit Daemon... Las sesiones se restaurarán automáticamente", "info");
        }

        setTimeout(() => {
            window.location.reload();
        }, 1800);
    } catch (err) {
        console.error("Error restarting Orbit:", err);
        alert("Error al reiniciar Orbit: " + err.message);
        if (btn) {
            btn.disabled = false;
            btn.innerHTML = '🔄 Reiniciar Orbit ahora';
        }
    }
}


// ==========================================
// Web Video/Audio Transcriber (Groq Whisper)
// ==========================================

function openTranscribeModal() {
    const modal = document.getElementById('modal-transcribe');
    if (!modal) return;
    modal.classList.add('open');
    modal.style.display = 'flex';
    resetTranscribeModal();
}

function closeTranscribeModal() {
    const modal = document.getElementById('modal-transcribe');
    if (modal) {
        modal.classList.remove('open');
        modal.style.display = 'none';
    }
}

function resetTranscribeModal() {
    currentTranscribeFile = null;
    lastTranscriptionResult = null;
    const fileInput = document.getElementById('transcribe-file-input');
    if (fileInput) fileInput.value = '';
    const pathInput = document.getElementById('transcribe-path-input');
    if (pathInput) pathInput.value = '';
    const dropzone = document.getElementById('transcribe-dropzone');
    if (dropzone) {
        dropzone.classList.remove('has-file');
        dropzone.innerHTML = `
            <div style="font-size: 28px; margin-bottom: 6px;">🎬</div>
            <strong style="color: var(--text-main); font-size: 13px;">Arrastra un video o audio aquí</strong>
            <span style="color: var(--text-muted); font-size: 11px; margin-top: 2px;">Soporta .mp4, .mkv, .mov, .webm, .mp3, .wav, .m4a o haz clic para explorar</span>
        `;
    }
    const statusBox = document.getElementById('transcribe-status-box');
    if (statusBox) statusBox.style.display = 'none';
    const resultBox = document.getElementById('transcribe-result-box');
    if (resultBox) resultBox.style.display = 'none';
    const btn = document.getElementById('btn-start-transcribe');
    if (btn) {
        btn.disabled = false;
        btn.innerHTML = '⚡ Iniciar Transcripción con Groq Whisper';
    }
}

function handleTranscribeFileSelect(files) {
    if (!files || files.length === 0) return;
    currentTranscribeFile = files[0];
    const dropzone = document.getElementById('transcribe-dropzone');
    if (dropzone) {
        dropzone.classList.add('has-file');
        const sizeMB = (currentTranscribeFile.size / (1024 * 1024)).toFixed(1);
        dropzone.innerHTML = `
            <div style="font-size: 28px; margin-bottom: 6px;">✅</div>
            <strong style="color: var(--accent); font-size: 13px;">${currentTranscribeFile.name}</strong>
            <span style="color: var(--text-dim); font-size: 11px; margin-top: 2px;">Tamaño: ${sizeMB} MB · Listo para procesar</span>
        `;
    }
    const pathInput = document.getElementById('transcribe-path-input');
    if (pathInput) pathInput.value = '';
}

function handleTranscribeDragOver(e) {
    e.preventDefault();
    e.stopPropagation();
    const dropzone = document.getElementById('transcribe-dropzone');
    if (dropzone) dropzone.classList.add('dragover');
}

function handleTranscribeDragLeave(e) {
    e.preventDefault();
    e.stopPropagation();
    const dropzone = document.getElementById('transcribe-dropzone');
    if (dropzone) dropzone.classList.remove('dragover');
}

function handleTranscribeDrop(e) {
    e.preventDefault();
    e.stopPropagation();
    const dropzone = document.getElementById('transcribe-dropzone');
    if (dropzone) dropzone.classList.remove('dragover');
    if (e.dataTransfer && e.dataTransfer.files && e.dataTransfer.files.length > 0) {
        handleTranscribeFileSelect(e.dataTransfer.files);
    }
}

async function startTranscribe() {
    const pathInput = document.getElementById('transcribe-path-input');
    const localPath = pathInput ? pathInput.value.trim() : '';

    if (!currentTranscribeFile && !localPath) {
        alert("Por favor selecciona o arrastra un archivo de video/audio, o escribe su ruta local.");
        return;
    }

    const timestampsToggle = document.getElementById('transcribe-timestamps-toggle');
    const withTimestamps = timestampsToggle ? timestampsToggle.checked : true;
    const langSelect = document.getElementById('transcribe-lang-select');
    const language = langSelect ? langSelect.value : 'auto';
    const promptInput = document.getElementById('transcribe-prompt-input');
    const prompt = promptInput ? promptInput.value.trim() : '';

    const btn = document.getElementById('btn-start-transcribe');
    const statusBox = document.getElementById('transcribe-status-box');
    const statusMsg = document.getElementById('transcribe-status-msg');
    const resultBox = document.getElementById('transcribe-result-box');

    if (btn) {
        btn.disabled = true;
        btn.innerHTML = '⏳ Procesando con FFmpeg y Groq...';
    }
    if (statusBox) statusBox.style.display = 'block';
    if (statusMsg) statusMsg.innerText = 'Extrayendo audio con FFmpeg y transcribiendo con Groq Whisper LPU...';
    if (resultBox) resultBox.style.display = 'none';

    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = token ? { 'Authorization': 'Bearer ' + token } : {};

        let response;
        if (currentTranscribeFile) {
            const formData = new FormData();
            formData.append('file', currentTranscribeFile);
            formData.append('withTimestamps', withTimestamps ? 'true' : 'false');
            if (language) formData.append('language', language);
            if (prompt) formData.append('prompt', prompt);

            response = await fetch('/api/plugins/groq-whisper/transcribe', {
                method: 'POST',
                headers,
                body: formData
            });
        } else {
            headers['Content-Type'] = 'application/json';
            response = await fetch('/api/plugins/groq-whisper/transcribe', {
                method: 'POST',
                headers,
                body: JSON.stringify({
                    filePath: localPath,
                    withTimestamps,
                    language,
                    prompt
                })
            });
        }

        if (!response.ok) {
            const errData = await response.json().catch(() => ({}));
            throw new Error(errData.error || `Error HTTP ${response.status}`);
        }

        const data = await response.json();
        lastTranscriptionResult = data;
        renderTranscriptionResult(data);

        if (typeof showToast === 'function') {
            showToast("Transcripción completada con éxito", "success");
        }
    } catch (err) {
        console.error("Transcribe failed:", err);
        alert("Error en la transcripción: " + err.message);
    } finally {
        if (btn) {
            btn.disabled = false;
            btn.innerHTML = '⚡ Iniciar Transcripción con Groq Whisper';
        }
        if (statusBox) statusBox.style.display = 'none';
    }
}

function renderTranscriptionResult(data) {
    const resultBox = document.getElementById('transcribe-result-box');
    const metaEl = document.getElementById('transcribe-result-meta');
    const textEl = document.getElementById('transcribe-result-text');
    if (!resultBox || !textEl) return;

    resultBox.style.display = 'block';

    const durMin = (data.duration / 60).toFixed(1);
    const words = (data.text || '').trim().split(/\s+/).filter(Boolean).length;

    if (metaEl) {
        metaEl.innerHTML = `
            <span>🌐 Idioma: <strong>${data.language || 'Detectado'}</strong></span> · 
            <span>⏱️ Duración: <strong>${data.duration ? data.duration.toFixed(1) + 's (~' + durMin + ' min)' : 'N/A'}</strong></span> · 
            <span>📝 Palabras: <strong>${words}</strong></span>
        `;
    }

    textEl.value = data.formattedText || data.text || '';
    resultBox.scrollIntoView({ behavior: 'smooth' });
}

function copyTranscribeResult() {
    const textEl = document.getElementById('transcribe-result-text');
    if (!textEl || !textEl.value) return;

    navigator.clipboard.writeText(textEl.value).then(() => {
        if (typeof showToast === 'function') {
            showToast("Transcripción copiada al portapapeles", "success");
        } else {
            alert("Transcripción copiada al portapapeles");
        }
    }).catch(err => {
        console.error("Copy failed:", err);
    });
}

function downloadTranscribeResult() {
    const textEl = document.getElementById('transcribe-result-text');
    if (!textEl || !textEl.value) return;

    const blob = new Blob([textEl.value], { type: 'text/plain;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `transcripcion_${new Date().toISOString().slice(0, 10)}.txt`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
}

async function saveTranscribeToTodayNote() {
    const textEl = document.getElementById('transcribe-result-text');
    if (!textEl || !textEl.value) return;

    const fileName = currentTranscribeFile ? currentTranscribeFile.name : 'Video / Audio';
    const noteText = `\n\n### 🎙️ Transcripción: ${fileName}\n\n${textEl.value}\n`;

    try {
        const token = serverInfo ? serverInfo.authToken : '';
        const headers = {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': 'Bearer ' + token } : {})
        };

        const res = await fetch('/api/calendar/note/append', {
            method: 'POST',
            headers,
            body: JSON.stringify({ content: noteText })
        });

        if (!res.ok) throw new Error("Error HTTP " + res.status);

        if (typeof showToast === 'function') {
            showToast("Transcripción guardada en la Nota Diaria de Hoy", "success");
        } else {
            alert("Transcripción guardada en la Nota Diaria de Hoy");
        }
    } catch (err) {
        console.error("Save to calendar failed:", err);
        alert("Error al guardar en el diario: " + err.message);
    }
}
