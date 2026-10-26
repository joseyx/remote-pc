# Changelog 📋

Todos los cambios notables en este proyecto serán documentados en este archivo.
El formato se basa en [Keep a Changelog](https://keepachangelog.com/es-ES/1.0.0/) y sigue [Semantic Versioning](https://semver.org/lang/es/).

## [1.5.2] - 2026-10-26

### 🐛 Correcciones y Mejoras (Bug Fixes & Enhancements)
- **Restauración de Selección de Texto y Copiado con Ratón en Terminal**:
  - Corrección en `terminal.js` evitando que el protocolo de captura de ratón (Mouse Tracking activado por ConPTY o Claude Code) deshabilite el servicio de selección de texto de xterm.js.
  - Arrastre normal de ratón ahora siempre selecciona texto; se requiere `Alt` únicamente si se desea pasar eventos de clic a aplicaciones TUI.
  - Copiado inteligente con clic derecho: copia el texto seleccionado al portapapeles y limpia la selección; si no hay texto seleccionado, pega el contenido del portapapeles (comportamiento idéntico a Windows Terminal y PuTTY).
  - Resaltado visual acentuado de selección (`rgba(56, 189, 248, 0.35)`) para clara visibilidad del texto marcado.
- **Resiliencia y Auto-Reconexión de Terminales Web**:
  - Implementación de reconexión automática en los sockets WebSocket de terminal (`termWs` y `termWs2`) tras desconexiones inesperadas o reinicios del demonio Orbit.
  - Detección de sockets cerrados en el ciclo de polling (`fetchSessions`), reanudando terminales activas sin necesidad de recargar la página.
- **Optimización de Latencia INP (Interaction to Next Paint)**:
  - Actualización visual inmediata de clases activas en `selectBubble` cediendo un micro-frame (`requestAnimationFrame`) antes de inicializar la terminal, reduciendo la latencia de respuesta de 776ms a menos de 20ms.

---

## [1.5.1] - 2026-10-22

### ⚡ Optimización y Rendimiento (Performance)
- **Eliminación del Bucle de Reflow en Terminal (`ResizeObserver`)**:
  - Corrección del bucle infinito de recálculo de layout en `terminal.js` incorporando detección y memorización de dimensiones con tolerancia de micro-jitter (< 2px) en `initTerminalResizeObserver()`.
  - Asignación condicional de desplazamiento de scroll (`scrollLeft = 0`) para evitar ensuciar y forzar la reevaluación síncrona del layout (*forced synchronous reflow*) antes de llamar a `fitAddon.fit()`.
- **Prevención de Destrucción Masiva del DOM en Polling (`fetchSessions`)**:
  - Implementación de un mecanismo de huella digital (*snapshot fingerprint*) en `fetchSessions()`, eliminando la reescritura innecesaria de `#bubbles-container` y subtabs de terminales cada 3.5 segundos cuando no hay cambios de estado.
  - Pausa inteligente del ciclo de polling cuando la pestaña se encuentra oculta o en segundo plano (`document.hidden`), con sincronización inmediata al recuperar el foco (`visibilitychange`).
- **Control de Versiones de Recursos Estáticos (Cache Busting)**:
  - Sufijo de control de versiones actualizado a `?v=2.3.1` en `index.html` (incluyendo `calendar.js`) para garantizar que los navegadores invaliden la caché previa y carguen los scripts optimizados.

---

## [1.5.0] - 2026-10-22

### 🚀 Añadido (Added)
- **Asignación Inteligente de Handoffs en Madrugada**:
  - Lógica automática para registrar handoffs realizados entre las `00:00:00` y las `04:59:59` (12:00 AM a 5:00 AM) en el calendario del **día hábil anterior**.
  - Días hábiles laborales de Martes a Viernes retroceden al día inmediatamente anterior (Lunes a Jueves).
  - Handoffs de madrugada en Sábado, Domingo o Lunes retroceden y se consolidan en el **Viernes hábil anterior**, evitando registrar jornadas en fines de semana.
  - Horario diurno regular (`>= 05:00:00`) mantiene la fecha del día actual.
- **Control de Fecha y Visibilidad en el Web Cockpit**:
  - Selector de fecha incorporado en el modal de Handoff (`#modal-handoff`) permitiendo confirmar o modificar el día de registro con retroalimentación visual (`🌙 Madrugada: asignado al día hábil anterior`).
  - Aviso contextual dinámico en el modal de Finalizar Jornada (`#modal-end-of-day`) indicando el día de registro objetivo para auto-guardados masivos.
  - Inicialización del calendario diario enfocando la jornada hábil efectiva al ingresar en horas de la madrugada.
- **Mejoras en el Protocolo MCP y CLI de Agentes**:
  - Herramienta MCP `save_daily_handoff` actualizada con parámetro opcional `date` y directrices para agentes Claude Code y Antigravity.
  - Auto-detección del día hábil anterior en `update_daily_handoff` y `delete_daily_handoff` al omitirse la fecha en horas de madrugada.
  - Script CLI `remotepc-handoff.py` actualizado con soporte de fecha hábil.

---

## [1.4.0] - 2026-10-21

### 🚀 Añadido (Added)
- **Arquitectura de Plugins y Extensiones (Estilo Pear Desktop)**:
  - Sistema modular y dinámico de plugins gestionado desde el Web Cockpit y persistido en `data/plugins.json`.
  - Panel visual de plugins con interruptores ON/OFF inmediatos, tarjetas de información, metadatos y formulario de configuración expandible con soporte para credenciales enmascaradas.
  - Notificación de reinicio con un solo clic (`Restart Required`) que reinicia el demonio Orbit restaurando de forma transparente todas las terminales y pseudoterminales PTY activas.
- **Plugin de Transcripción Groq Whisper LPU (`groq_whisper`)**:
  - Extracción y compresión de audio mediante FFmpeg a formato MP3 mono de 16 kHz (48 kbps) optimizado para speech-to-text.
  - Integración nativa con la API de Groq Whisper (`whisper-large-v3-turbo` y `whisper-large-v3`) con velocidad de inferencia ultra-rápida.
  - Soporte inteligente para archivos de audio de gran tamaño (> 24 MB) mediante segmentación automática en fragmentos temporales y ensamblado secuencial con marcas de tiempo relativas.
  - Herramientas MCP dinámicas (`transcribe_video` y `extract_audio`) expuestas automáticamente a Claude Code, Antigravity y asistentes compatibles solo cuando el plugin está habilitado.
- **Transcriptor Web Interactivo (Web Cockpit)**:
  - Modal accesible desde la cabecera, la paleta de comandos (`Ctrl+K`) y el menú de acciones contextuales.
  - Soporte para arrastrar y soltar (Drag & Drop) archivos multimedia (`.mp4`, `.mkv`, `.mov`, `.webm`, `.mp3`, `.wav`) o especificar rutas locales absolutas en Windows.
  - Selector de idioma, marcas de tiempo opcionales `[MM:SS]`, copia al portapapeles, descarga en `.txt` y anexo directo a las Notas Diarias de Hoy en el calendario.
- **Flujo de Trabajo CI y Plantillas de GitHub**:
  - Nuevo workflow de validación automática de Pull Requests (`validate-pr.yml`).
  - Plantillas interactivas en formato YAML para reporte de bugs, solicitudes de características y tareas de refactorización.
  - Reglas y directrices de seguridad para agentes de IA (`AGENTS.md`).

### 🐛 Corregido (Fixed)
- **Desplazamiento Horizontal y Desbordamiento en Terminal ConPTY**:
  - Activación de `windowsMode: true` en todas las instancias de xterm.js (`termObj`, `termObj2`, `quakeTermObj`) para delegar el control de salto y buffer al ConPTY nativo de Windows, eliminando el desplazamiento cíclico de texto hacia la izquierda.
  - Reubicación de márgenes/paddings a contenedores envolventes (`.term-pane`), garantizando que `FitAddon` mida con precisión matemática de píxel las columnas reales del pseudoterminal.
  - Bloqueo estricto de desplazamiento horizontal con `overflow-x: hidden !important` y reseteo forzado de `scrollLeft = 0` en eventos de redimensionamiento y limpieza (`Ctrl+L`).
  - Sincronización automática de dimensiones PTY mediante `ResizeObserver` reactivo al colapsar/expandir la barra lateral o redimensionar paneles divididos.

---

## [1.3.1] - 2026-10-15

### 🐛 Corregido (Fixed)
- **Renderizado de Notas de la Versión (Patch Notes)**:
  - Integración completa con el parser `marked.js` en el modal de actualización del Web Cockpit (`#modal-update`), renderizando el Markdown de GitHub Releases en HTML estructurado.
  - Nueva clase tipográfica `.update-changelog-body` en `modals.css` con diseño dedicado para títulos, listas con viñetas legibles, badges de código (`<code>`) en cian y contenedor responsivo ampliado con scroll suave.
  - Corrección de secuencias de escape y comillas invertidas en las notas publicadas en GitHub Releases.

---

## [1.3.0] - 2026-10-15

### 🚀 Añadido (Added)
- **Herramientas MCP para Servidores de Desarrollo (`orbit mcp`)**:
  - Nuevas herramientas nativas para Claude Code y asistentes de IA:
    - `start_project_server`: Inicia el servidor de desarrollo del workspace vinculándolo a una sesión PTY interactiva (`devserver-<id>`).
    - `get_project_server_status`: Inspección en tiempo real de estado, PID, puerto y últimas líneas de salida.
    - `stop_project_server`: Detención ordenada del servidor y liberación del puerto.
  - Sincronización transparente: los logs de arranque y compilación se visualizan en vivo tanto en el Web Cockpit como en la App Móvil.
- **Rediseño Integral de Orbit Mobile (4 Pestañas)**:
  - Nueva interfaz moderna con Jetpack Compose y navegación por pestañas:
    - 🪐 **Cockpit**: Gestión de workspaces, burbujas/tareas activas y diálogo de creación rápida.
    - ⚡ **Servers**: Switch on/off para servidores de desarrollo, visualización de puertos y logs.
    - 📋 **Daily Standup**: Generación automática de resumen de tareas y avance diario.
    - 📊 **Monitor**: Telemetría del host (CPU, memoria, disco, IP de red/Tailscale y versión de Orbit).
- **Integración Avanzada de Repositorios Git y Workspaces**:
  - Detección y vinculación de repositorios Git en workspaces.
  - Creación de tareas clonando ramas existentes con buscador predictivo en tiempo real (probado sobre más de 1,200 ramas).
  - Selector segmentado interactivo (Nueva Rama / Rama Existente) con caché inteligente y botón de refresco forzado.
  - Soporte para inicialización de bases de datos mediante dumps SQL personalizados (`.sql`/`.dump`).

### 🐛 Corregido (Fixed)
- **Compatibilidad con Ramas Jerárquicas (`feature/TAR-xxx`)**:
  - Soporte completo para ramas con barras en el nombre durante la creación de worktrees.
  - Emparejamiento resiliente de bubbles en el backend incluso si el directorio o schema ya existían localmente.
- **Ajuste Visual y Anti-Wrap en Tarjetas de Tareas**:
  - Rediseño en 2 filas del encabezado de las tarjetas en la barra lateral del Web Cockpit.
  - Prevención de desbordamiento y saltos de línea en badges de estado (`nowrap`), eliminando estiramientos verticales.

---

## [1.2.1] - 2026-10-13

### 🐛 Corregido (Fixed)
- **Desincronización de Terminal ConPTY**:
  - Deduplicación y debounce en eventos de resize (`fitActiveTerminal` y `fitSplitTerminal`) para prevenir spam de `ResizePseudoConsole` a Windows ConPTY y evitar sobreescritura de caracteres en TUIs interactivas como Claude Code.
  - Sincronización precisa de 250ms tras animaciones de la barra lateral.
- **Repintado de Pantalla (`Ctrl+L`)**:
  - La acción de limpiar buffer (`clearActiveTerminal`) ahora envía Form Feed (`\x0c` / `Ctrl+L`) al PTY para forzar el repintado íntegro de la aplicación interactiva activa.
  - Añadido botón de acceso rápido `Ctrl+L` en la barra inferior del terminal.
- **Ocultamiento de Barra Lateral**:
  - Corrección de visibilidad estricta (`visibility: hidden !important`) en `.sidebar.collapsed` evitando filtrado de botones en el borde de la pantalla.

---

## [1.2.0] - 2026-10-12

### 🚀 Añadido (Added)
- **Abstracción Universal de Workspaces**:
  - Soporte multi-proyecto para cualquier repositorio Git del usuario persistido en `data/workspaces.json`.
  - Gestor visual de proyectos en el Cockpit con explorador de carpetas locales del host y pestañas dinámicas.
  - Aislamiento automático de Git worktrees (`.worktrees/<taskId>`) y configuración de dev servers por proyecto.
  - Endpoints CRUD (`/api/workspaces`) y verificación integral de workspaces en `orbit doctor`.
- **Pipeline de Migración Automática (`orbit migrate`)**:
  - Migración desatendida y sin pérdida de datos para instalaciones existentes y upgrades directos.
  - Auto-descubrimiento y consolidación de proyectos previos (Aditum, Brif, etc.), handoffs y notas diarias en `%LOCALAPPDATA%\Orbit`.
  - Gancho de migración automático al iniciar el servidor y paso integrado en `Orbit-Setup.exe`.
- **Rediseño Limpio del Web Cockpit Header**:
  - Reorganización de la barra superior en 3 bloques despejados: Telemetría (RAM + VPN), Herramientas Rápidas (`Ctrl+K`, `Quake`, `Cierre de Día`) y Menú Unificado `⚙️ Sistema`.
  - Menú desplegable centralizado con accesos a QR Pairing, Seguridad/PIN, copia rápida de Token y URL LAN, alternador de alertas sonoras, diagnóstico Doctor y recarga.

---

## [1.1.0] - 2026-10-12

### 🚀 Añadido (Added)
- **Auto-Actualizador Integrado (`orbit update` & Web Cockpit)**:
  - Detección automática y periódica de nuevas versiones desde GitHub Releases cada 15 minutos en el Web Cockpit.
  - Modal interactivo de actualización con visualización del changelog oficial y botón para aplicar en caliente.
  - Reemplazo seguro in-place del ejecutable en ejecución (`ApplyUpdate`) con rotación `.old` en Windows y limpieza en el siguiente arranque.
  - Subcomandos de consola: `orbit check-update` y `orbit update`.
- **Orbit Doctor - Diagnósticos del Sistema (`orbit doctor`)**:
  - Suite de diagnóstico integral de conectividad (LAN, Tailscale, ZeroTier, VPN).
  - Verificación de puertos y listeners locales, detección de reglas del Firewall de Windows.
  - Chequeo de salud del gestor de terminales PTY, shell predeterminado y persistencia de datos.
  - Salida formateada con colores en consola y endpoint API (`GET /api/system/doctor`).
- **Servicio en Segundo Plano & Autoarranque (Daemon)**:
  - Subcomandos completos: `orbit service install`, `orbit service start`, `orbit service stop`, `orbit service status`, `orbit service uninstall`.
  - Soporte multiplataforma: Windows Startup Registry / Scheduled Tasks, macOS Launchd (`LaunchAgents`) y Linux Systemd user services.
  - Control de servicio y estado en tiempo real desde el Web Cockpit y Command Palette (`Ctrl+K`).
- **Emparejamiento Rápido por Código QR (QR Pairing)**:
  - Generación de código QR dinámico en el Cockpit para conectar instantáneamente la app móvil Orbit Mobile sin tipear IPs ni tokens manualmente.
  - Sincronización automática del QR en pantalla ante rotaciones de token o cambios de red.
- **Hardening de Seguridad, Control de Tokens & PIN Maestro**:
  - Rotación dinámica de tokens en caliente (`orbit token rotate`, `POST /api/system/token/rotate`) con invalidación inmediata del token previo y sincronización thread-safe (`sync.RWMutex`).
  - Configuración de PIN maestro de seguridad (`orbit pin set/status/remove`, `POST /api/system/pin/set`) necesario para consultar o rotar el token.
  - Bloqueo de pantalla interactivo del Web Cockpit (`Alt + L` / `cockpit-lock-overlay`) con efecto glassmorphism, desbloqueable mediante PIN o token.
- **Migración Automática & Robustez de Almacenamiento**:
  - Lógica inteligente de resolución de carpetas (`config.GetDataDir()`): priorización estricta de carpetas con datos reales sobre directorios vacíos.
  - Migración automática y transparente de handoffs, notas y configuración al instalar `Orbit-Setup.exe` en `%LOCALAPPDATA%\Orbit`.
  - Script oficial de desinstalación limpia para Unix (`scripts/uninstall.sh`) empaquetado en todas las releases de Linux y macOS.
- **Rebranding Oficial a Orbit & Assets de Marca**:
  - Rebranding completo del ecosistema a **Orbit** con alias retrocompatible `remotepc-server`.
  - Logotipo oficial vectorial SVG, favicon y assets de marca integrados en el Cockpit, instalador (`orbit.ico`) y app móvil.

### 🐛 Corregido (Fixed)
- **Aislamiento de Almacenamiento en Subdirectorios**: Corrección del escenario donde ejecutar el binario desde subcarpetas (`server/`) creaba una carpeta `data` huérfana vacía que ocultaba los handoffs y notas del usuario.
- **Fallbacks en Calendario y Metadatos**: Búsqueda recursiva de respaldo para notas Markdown y `bubbles_metadata.json` ante cambios de directorio de ejecución.

---

## [1.0.0] - 2026-10-11

### Añadido (Added)
- **Servidor MCP Nativo en Go (`remotepc-server mcp`)**:
  - Implementación completa del protocolo Model Context Protocol (JSON-RPC 2.0 sobre stdio) sin dependencia de Python.
  - 10 herramientas para agentes de IA: `save_daily_handoff`, `get_standup_pending`, `get_today_note`, `update_daily_handoff`, `delete_daily_handoff`, `add_daily_todo`, `append_daily_note`, `list_workspace_bubbles`, `get_bubble_context`, `create_workspace_bubble`.
  - Auto-arranque silencioso del daemon en segundo plano si Claude invoca una herramienta cuando el servidor está inactivo.
- **Subcomandos de Autoservicio CLI**:
  - `setup-mcp`: Configuración desatendida para Claude Code CLI (`~/.claude.json`) y Claude Desktop (`%APPDATA%\Claude`).
  - `uninstall-mcp`: Eliminación limpia de las configuraciones MCP.
  - `add-path` y `remove-path`: Manipulación directa del `PATH` del usuario en el registro de Windows con notificación broadcast (`WM_SETTINGCHANGE`).
  - `install` y `uninstall`: Procedimientos de auto-instalación completa en un comando.
- **Instalador Gráfico (`RemotePC-Setup.exe`)**:
  - Empaquetador Inno Setup 6 con compresión LZMA2 (~4.7 MB).
  - Instalación por usuario en `%LOCALAPPDATA%\RemotePC` sin requerir permisos de Administrador ni elevación UAC.
  - Asistente multilenguaje (Español / Inglés) con tareas seleccionables para `PATH`, `MCP` y acceso directo.
- **Herramientas de Build y Limpieza**:
  - `scripts/build-installer.ps1`: Automatización de compilación y empaquetado del instalador.
  - `scripts/clean.ps1`: Purgado de binarios, logs y reseteo de buffers de sesiones.
- **Windows Pseudo Console (ConPTY) Nativo**:
  - Emulación completa de terminal interactivo para Claude Code (colores 24-bit ANSI, spinners, cursores).
  - Persistencia de sesiones en segundo plano tipo tmux con buffer circular de 2 MB.
- **Eco Workspace Hub**:
  - Gestión de tareas y bubbles para múltiples espacios (Aditum y Brif).
  - Soporte para Git Worktrees y clones ultraligeros de bases de datos MySQL multi-esquema.
- **Integración con Windscribe VPN**:
  - Conexión, desconexión y cambio en caliente de servidores y protocolos vía CLI sin bloqueos de proceso.
  - Priorización de interfaces físicas para garantizar conectividad LAN con el móvil.
- **Cliente Android Nativo**:
  - Interfaz en Jetpack Compose con scroll cinético sub-píxel.
  - Barra de atajos rápidos para Claude Code (`ESC`, `TAB`, `Ctrl+C`, `Yes/No`, comandos slash).
  - Explorador remoto de directorios de la PC.

### Modificado (Changed)
- **Aislamiento de Datos de Runtime**: Migración de todos los archivos generados en tiempo de ejecución (`calendar_notes`, `bubbles_metadata.json`, `sessions_state.json`) a una carpeta centralizada `data/` totalmente ignorada por git.
- **Arquitectura Monorepo Limpia**: Estructuración en `server/`, `android/`, `installer/`, `scripts/` y `data/`.
