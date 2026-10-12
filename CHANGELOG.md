# Changelog 📋

Todos los cambios notables en este proyecto serán documentados en este archivo.
El formato se basa en [Keep a Changelog](https://keepachangelog.com/es-ES/1.0.0/) y sigue [Semantic Versioning](https://semver.org/lang/es/).

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
