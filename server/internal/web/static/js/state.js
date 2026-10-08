// ==========================================
// Global Application State, Dialogs & Toast
// ==========================================

// Global State
let serverInfo = null;
let allBubbles = [];
let allSessions = [];
let activeFilter = 'aditum';
let currentBubble = null;
let activeTab = 'terminal';
let activeBubbleSession = null;

// Bubble Sorting & Priority State
let bubbleSortMode = localStorage.getItem('remotepc_bubble_sort_mode') || 'alpha';
let priorityPickerBubbleId = null;
let draggedBubbleId = null;

const PRIORITY_CONFIG = {
    'P0': { label: 'P0 - Urgente', icon: '🔴', weight: 1, class: 'p0' },
    'P1': { label: 'P1 - Alta',    icon: '🟠', weight: 2, class: 'p1' },
    'P2': { label: 'P2 - Media',   icon: '🟡', weight: 3, class: 'p2' },
    'P3': { label: 'P3 - Baja',    icon: '🟢', weight: 4, class: 'p3' },
    '':   { label: 'Sin prioridad', icon: '⚪', weight: 99, class: 'none' }
};

function getPriorityInfo(p) {
    return PRIORITY_CONFIG[p] || PRIORITY_CONFIG[''];
}

// Universal App Dialog State & Methods (Alert / Confirm / Prompt)
let customDialogResolver = null;

function showAppDialog({ title = 'Aviso', message = '', icon = 'ℹ️', type = 'alert', confirmText = 'Aceptar', cancelText = 'Cancelar', defaultValue = '', isDanger = false }) {
    return new Promise((resolve) => {
        const overlay = document.getElementById('modal-custom-dialog');
        const box = document.getElementById('custom-dialog-box');
        const titleEl = document.getElementById('custom-dialog-title');
        const msgEl = document.getElementById('custom-dialog-message');
        const iconEl = document.getElementById('custom-dialog-icon');
        const inputCont = document.getElementById('custom-dialog-input-container');
        const inputEl = document.getElementById('custom-dialog-input');
        const confirmBtn = document.getElementById('custom-dialog-confirm-btn');
        const cancelBtn = document.getElementById('custom-dialog-cancel-btn');

        if (!overlay) {
            if (type === 'confirm') return resolve(window.confirm(message));
            if (type === 'prompt') return resolve(window.prompt(message, defaultValue));
            window.alert(message);
            return resolve(true);
        }

        titleEl.textContent = title;
        msgEl.textContent = message;
        iconEl.textContent = icon;

        confirmBtn.textContent = confirmText;
        if (isDanger) {
            confirmBtn.className = "btn btn-sm btn-danger";
        } else {
            confirmBtn.className = "btn btn-sm btn-primary";
        }

        if (type === 'alert') {
            cancelBtn.style.display = 'none';
            inputCont.style.display = 'none';
        } else if (type === 'confirm') {
            cancelBtn.textContent = cancelText;
            cancelBtn.style.display = 'inline-flex';
            inputCont.style.display = 'none';
        } else if (type === 'prompt') {
            cancelBtn.textContent = cancelText;
            cancelBtn.style.display = 'inline-flex';
            inputCont.style.display = 'block';
            inputEl.value = defaultValue || '';
        }

        customDialogResolver = {
            type,
            resolve
        };

        overlay.style.display = 'flex';
        requestAnimationFrame(() => {
            overlay.style.opacity = '1';
            overlay.style.pointerEvents = 'auto';
            if (box) box.style.transform = 'translateY(0)';
            if (type === 'prompt' && inputEl) {
                inputEl.focus();
                inputEl.select();
            } else if (confirmBtn) {
                confirmBtn.focus();
            }
        });
    });
}

function closeCustomDialog() {
    const overlay = document.getElementById('modal-custom-dialog');
    const box = document.getElementById('custom-dialog-box');
    if (overlay) {
        overlay.style.opacity = '0';
        overlay.style.pointerEvents = 'none';
        if (box) box.style.transform = 'translateY(-10px)';
        setTimeout(() => {
            overlay.style.display = 'none';
        }, 160);
    }
}

function handleCustomDialogConfirm() {
    if (!customDialogResolver) return;
    const res = customDialogResolver;
    customDialogResolver = null;
    closeCustomDialog();

    if (res.type === 'prompt') {
        const val = document.getElementById('custom-dialog-input')?.value || '';
        res.resolve(val);
    } else if (res.type === 'confirm') {
        res.resolve(true);
    } else {
        res.resolve(true);
    }
}

function handleCustomDialogCancel() {
    if (!customDialogResolver) return;
    const res = customDialogResolver;
    customDialogResolver = null;
    closeCustomDialog();

    if (res.type === 'prompt') {
        res.resolve(null);
    } else if (res.type === 'confirm') {
        res.resolve(false);
    } else {
        res.resolve(false);
    }
}

function appAlert(message, title = 'Notificación', icon = '⚠️') {
    return showAppDialog({ title, message, icon, type: 'alert' });
}

function appConfirm(message, title = 'Confirmar', confirmText = 'Confirmar', cancelText = 'Cancelar', isDanger = false, icon = '❓') {
    return showAppDialog({ title, message, icon, type: 'confirm', confirmText, cancelText, isDanger });
}

function appPrompt(message, defaultValue = '', title = 'Entrada requerida', confirmText = 'Aceptar', icon = '✏️') {
    return showAppDialog({ title, message, icon, type: 'prompt', defaultValue, confirmText });
}

// Global alert override: all alerts use the dark cyber UI modal instead of native browser popup
window.alert = function(msg) {
    appAlert(String(msg));
};



let audioNotifEnabled = true;
let lastTermActivityTime = Date.now();
let hadRecentActiveStream = false;
let outputChunkCount = 0;



function showToast(msg) {
    const t = document.getElementById('toast');
    t.textContent = msg;
    t.style.display = 'block';
    setTimeout(() => { t.style.display = 'none'; }, 3000);
}

function escapeHtml(str) {
    if (!str) return '';
    return String(str).replace(/[&<>"']/g, function(m) {
        return ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[m];
    });
}



function toggleNotifications() {
    audioNotifEnabled = !audioNotifEnabled;
    const btn = document.getElementById('btn-toggle-notif');
    if (btn) {
        btn.textContent = audioNotifEnabled ? '🔔' : '🔕';
        btn.title = audioNotifEnabled ? 'Alertas sonoras (Activado)' : 'Alertas sonoras (Silenciado)';
    }
    if (audioNotifEnabled) {
        playNotificationChime();
        if ('Notification' in window && Notification.permission === 'default') {
            Notification.requestPermission();
        }
        showToast("Alertas sonoras activadas");
    } else {
        showToast("Alertas sonoras silenciadas");
    }
}

function playNotificationChime() {
    if (!audioNotifEnabled) return;
    try {
        const AudioCtx = window.AudioContext || window.webkitAudioContext;
        if (!AudioCtx) return;
        const ctx = new AudioCtx();
        const osc = ctx.createOscillator();
        const gain = ctx.createGain();
        osc.type = 'sine';
        osc.frequency.setValueAtTime(587.33, ctx.currentTime); // D5
        osc.frequency.exponentialRampToValueAtTime(880, ctx.currentTime + 0.12); // A5
        gain.gain.setValueAtTime(0.08, ctx.currentTime);
        gain.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + 0.22);
        osc.connect(gain);
        gain.connect(ctx.destination);
        osc.start();
        osc.stop(ctx.currentTime + 0.23);
    } catch(e) {}
}
