# Changelog 📋

Todos los cambios notables en este proyecto serán documentados en este archivo.
El formato se basa en [Keep a Changelog](https://keepachangelog.com/es-ES/1.0.0/) y sigue [Semantic Versioning](https://semver.org/lang/es/).

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
