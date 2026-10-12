<div align="center">

# 🚀 Orbit

### The Local-First Developer Cockpit & Native AI MCP Orchestrator

[![CI - Test & Build](https://github.com/joseyx/remote-pc/actions/workflows/ci.yml/badge.svg)](https://github.com/joseyx/remote-pc/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Android](https://img.shields.io/badge/Android-Kotlin%20%2B%20Compose-3DDC84?style=flat&logo=android)](https://developer.android.com)
[![Protocol](https://img.shields.io/badge/AI%20Protocol-MCP%20JSON--RPC%202.0-8A2BE2?style=flat)](https://modelcontextprotocol.io/)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20macOS%20%7C%20Linux-0078D6?style=flat)](https://github.com/joseyx/remote-pc)
[![Installer](https://img.shields.io/badge/Installer-Inno%20Setup%20%2B%20Unix%20Script-orange?style=flat)](dist/Orbit-Setup.exe)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

<p align="center">
  <b>Controla Claude Code, terminales interactivas (ConPTY / POSIX PTY) y flujos multi-proyecto desde tu teléfono Android o navegador web.</b><br>
  Integración nativa con <b>Model Context Protocol (MCP)</b>, Git Worktrees aislados, bóvedas de Obsidian, bases de datos multi-esquema y Windscribe VPN.
</p>

[Instalación Rápida](#-instalación-rápida) •
[Integración MCP](#-servidor-mcp-nativo-para-agentes-ia) •
[Características](#-características-principales) •
[Arquitectura](#-arquitectura-del-sistema) •
[Comandos CLI](#-comandos-cli-de-orbit) •
[App Android](#-aplicación-móvil-android) •
[API REST & WS](#-referencia-de-la-api-rest-y-websockets)

---

</div>

## 💡 ¿Qué es Orbit?

**Orbit** es una suite completa de desarrollo y orquestación diseñada para eliminar la fricción al trabajar con herramientas de inteligencia artificial (**Claude Code**, Antigravity) y proyectos de software complejos con múltiples ramas y worktrees.

A diferencia de clientes SSH genéricos que rompen cursores y secuencias ANSI, Orbit emula consolas interactivas mediante **Windows Pseudo Console (ConPTY)** en Windows y **POSIX pseudo-terminals (`creack/pty`)** en macOS y Linux, persistiendo sesiones en segundo plano al estilo tmux con búfer circular de 2 MB y permitiendo la interacción bidireccional desde **Android**, **Web (Orbit Cockpit)** y **Agentes de IA**.

> [!TIP]
> **100% Autónomo y Privado:** Sin servidores en la nube de terceros, sin telemetría y con latencia ultrabaja en red local (Wi-Fi) o redes privadas virtuales (Tailscale).

---

## ⚡ Instalación Rápida

### Opción 1: Windows (Instalador Gráfico Asistido)
1. Descarga el instalador compilado [`dist/Orbit-Setup.exe`](dist/Orbit-Setup.exe) (**4.7 MB**).
2. Ejecuta el asistente de instalación:
   - Se instala por usuario en `%LOCALAPPDATA%\Orbit` (no requiere permisos de Administrador).
   - `[x]` Agrega automáticamente `orbit` y alias `remotepc-server` al `PATH` del usuario.
   - `[x]` Configura el servidor MCP en **Claude Code** y **Claude Desktop** de forma desatendida.
3. ¡Listo! Abre cualquier terminal y ejecuta `orbit` para iniciar el Cockpit.

### Opción 2: macOS & Linux (Script Automatizado)
Instala y configura el binario y el servidor MCP en un solo comando:
```bash
./scripts/install.sh
```
O directamente con curl:
```bash
curl -fsSL https://raw.githubusercontent.com/joseyx/remote-pc/main/scripts/install.sh | bash
```

### Opción 3: Compilación desde Código Fuente (Multiplataforma)
```bash
# 1. Clonar el repositorio
git clone https://github.com/joseyx/remote-pc.git
cd "remote-pc"

# 2. Compilar el binario Go
cd server
go build -ldflags="-s -w" -o ../bin/orbit ./cmd/server
cd ..

# 3. Registrar MCP en Claude Code y Claude Desktop
./bin/orbit setup-mcp

# 4. Iniciar Orbit Cockpit
./bin/orbit
```

---

## 🤖 Servidor MCP Nativo para Agentes IA

Orbit implementa el protocolo **Model Context Protocol (MCP)** directamente en Go por `stdio`, permitiendo a **Claude Code**, **Claude Desktop** y otros agentes interactuar con tu entorno de desarrollo sin necesidad de instalar Python ni librerías externas.

### Configuración en 1 Clic
```bash
orbit setup-mcp
```
Este comando detecta y configura automáticamente tus clientes:
- **Claude Code CLI**: Agregado a `~/.claude.json`.
- **Claude Desktop**: Configurado en `%APPDATA%\Claude` (Windows), `~/Library/Application Support/Claude` (macOS) o `~/.config/Claude` (Linux).

Verifica el estado en cualquier momento con:
```bash
claude mcp list
# orbit: C:\...\orbit.exe mcp - √ Connected
```

### Catálogo de Herramientas MCP (10 Herramientas Nativas)

| Herramienta | Descripción |
| :--- | :--- |
| `save_daily_handoff` | Registra el cierre de tarea y avances de la jornada en el Calendario con Git snapshot automático. |
| `get_standup_pending` | Consulta los pendientes dejados en el último handoff o fecha previa para standups diarios. |
| `get_today_note` | Lee la nota diaria y los handoffs registrados en el Calendario para hoy. |
| `update_daily_handoff` | Actualiza o modifica un handoff existente de la jornada. |
| `delete_daily_handoff` | Elimina un handoff registrado. |
| `add_daily_todo` | Agrega micro-tareas accionables con checkboxes `[ ]` o `[x]` a la bitácora del día. |
| `append_daily_note` | Anexa notas libres, comandos o apuntes en Markdown a la nota diaria. |
| `list_workspace_bubbles` | Lista todos los workspace bubbles, ramas Git, puertos dev y esquemas de BD. |
| `get_bubble_context` | Obtiene el estado Git detallado (rama, commits ahead/behind, archivos modificados) de la tarea actual. |
| `create_workspace_bubble` | Crea un nuevo worktree y branch aislado para una tarea (soporta Aditum y Brif). |

---

## ✨ Características Principales

```
                  ┌──────────────────────────────────────────┐
                  │              ORBIT COCKPIT               │
                  │   (Terminales, Bubbles, MCP, Obsidian)   │
                  └────────────────────┬─────────────────────┘
                                       │
         ┌─────────────────────────────┼─────────────────────────────┐
         ▼                             ▼                             ▼
┌──────────────────┐          ┌──────────────────┐          ┌──────────────────┐
│   Dual PTY Host  │          │   Agentic Hub    │          │  Workspace Hub   │
│ ConPTY & POSIX   │          │  MCP JSON-RPC    │          │ Git Worktrees    │
│ Buffer 2MB Tmux  │          │ Auto-Handoffs    │          │ Puertos & DBs    │
└──────────────────┘          └──────────────────┘          └──────────────────┘
```

1. **Terminales Persistentes Multiplataforma**: Sesiones de consola interactivas protegidas contra caídas de red y cierres accidentales. El búfer circular almacena los últimos 2 MB de historial.
2. **Workspace Bubbles & Git Worktrees**: Cambia de tarea sin contaminar tu directorio de trabajo. Crea ramas y carpetas de trabajo aisladas en segundos.
3. **Orbit Cockpit (Web Dashboard)**: Interfaz de usuario de alta densidad informativa inspirada en GitHub Dark, con paleta de comandos (`Ctrl+K`), terminal rápida Quake (`F12`), y monitor de recursos.
4. **Orbit Mobile (App Android Nativa)**: Cliente móvil en Kotlin + Jetpack Compose con atajos táctiles para Claude Code (`/compact`, `/pr`, `Ctrl+C`).
5. **Bridge con Obsidian**: Búsqueda e inspección instantánea de notas técnicas en tus bóvedas locales.
6. **Monitoreo de Sistema & VPN**: Estado en tiempo real de RAM, servicios de bases de datos locales (PostgreSQL, Docker) y túnel VPN Windscribe.

---

## 🏗️ Arquitectura del Sistema

```mermaid
graph TD
    subgraph Clientes
        Mobile[App Android Kotlin/Compose]
        Browser[Orbit Cockpit Web Dashboard]
        ClaudeAgent[Claude Code / Claude Desktop]
    end

    subgraph Orbit Daemon [Orbit Server Daemon (Go)]
        API[Enrutador REST / API Server]
        WSHub[Hub WebSockets / Multiplexor PTY]
        MCPServer[Servidor MCP stdio JSON-RPC 2.0]
        SessionMgr[Gestor de Sesiones & PTY Ring Buffer]
        BubbleMgr[Gestor de Workspace Bubbles]
        CalendarMgr[Calendario & Handoffs Registry]
        ObsidianMgr[Obsidian Vault Discovery]
    end

    subgraph SO & Herramientas
        PTY[Motor PTY: ConPTY (Win) / creack/pty (Unix)]
        GitCLI[Git CLI & Monorepo Worktrees]
        ObsidianFS[Bóvedas Markdown de Obsidian]
    end

    ClaudeAgent <-->|stdio JSON-RPC| MCPServer
    MCPServer <-->|Internal API| BubbleMgr
    MCPServer <-->|Internal API| CalendarMgr

    Mobile <-->|HTTP / WebSockets| API
    Browser <-->|HTTP / WebSockets| API
    API --> WSHub
    WSHub --> SessionMgr
    SessionMgr --> PTY
    BubbleMgr --> GitCLI
    ObsidianMgr --> ObsidianFS
```

---

## 💻 Comandos CLI de `orbit`

El ejecutable incluye herramientas completas de autoservicio para terminal:

```text
Uso:
  orbit [comando] [opciones]
  (alias retrocompatible: remotepc-server [comando] [opciones])

Comandos disponibles:
  (sin comando)      Inicia el servidor daemon y el Cockpit web
  mcp                Ejecuta el servidor MCP JSON-RPC 2.0 por stdio (para Claude)
  setup-mcp          Configura el servidor MCP en Claude Code y Claude Desktop
  uninstall-mcp      Remueve la configuración MCP de Claude Code y Claude Desktop
  add-path           Agrega el directorio del binario al PATH del usuario
  remove-path        Remueve el directorio del binario del PATH del usuario
  install            Instalación completa (PATH + MCP + Acceso directo)
  uninstall          Desinstalación limpia del entorno y MCP
  version            Muestra la versión de Orbit
  help               Muestra la ayuda de comandos

Opciones del servidor:
  -open              Abre el Cockpit web en el navegador al iniciar (defecto: true)
  -no-open           No abre el Cockpit web automáticamente
  -port <puerto>     Puerto HTTP a utilizar (defecto: 8080)
  -token <token>     Token de autenticación Bearer
  -host <ip>         Dirección IP para escuchar (defecto: 0.0.0.0)
```

---

## 📁 Estructura del Repositorio

```text
remote-pc/
├── android/                    # Cliente móvil Android Orbit Mobile (Kotlin + Compose)
│   ├── app/src/main/java/com/remotepc/app/
│   │   ├── data/               # Modelos de sesión, VPN y red (OkHttp/WebSocket)
│   │   ├── ui/                 # Interfaz Compose (Terminal, Atajos Claude, VPN Card)
│   │   └── AndroidManifest.xml
│   ├── build.gradle.kts
│   └── README.md
├── server/                     # Backend Go (Dual PTY host, Cockpit Web, Servidor MCP)
│   ├── cmd/server/             # main.go con router de CLI y servidor
│   ├── internal/
│   │   ├── api/                # Enrutador HTTP REST y WebSocket
│   │   ├── bubble/             # Gestor de Bubbles, Worktrees y Dev Servers
│   │   ├── calendar/           # Calendario de Handoffs, Daily Notes y Standup
│   │   ├── config/             # Carga y resolución central de datos (data/)
│   │   ├── git/                # Helpers de Git (status, diff, branch, log)
│   │   ├── installer/          # Registro de Windows y symlinks Unix (~/.local/bin)
│   │   ├── mcp/                # Servidor MCP JSON-RPC 2.0 nativo (10 herramientas)
│   │   ├── obsidian/           # Búsqueda y lectura de Vaults de Obsidian
│   │   ├── pty/                # ConPTY (Windows) & POSIX PTY (macOS/Linux)
│   │   ├── session/            # Gestor de terminales, buffer circular y persistencia
│   │   ├── system/             # Monitor de CPU/RAM multiplataforma (Win32, sysctl, proc)
│   │   ├── vpn/                # Integración con Windscribe CLI
│   │   └── web/                # Orbit Cockpit Web (HTML/CSS/JS + xterm.js + marked)
│   └── README.md
├── installer/                  # Definición del Instalador Gráfico de Windows
│   └── Orbit-Setup.iss         # Script Inno Setup 6 (asistente de instalación por usuario)
├── scripts/                    # Scripts de Automatización y Developer Experience
│   ├── build-installer.ps1     # Compila el binario y empaqueta dist/Orbit-Setup.exe
│   ├── install.sh              # Instalador universal en Bash para macOS y Linux
│   └── clean.ps1               # Limpieza de temporales, builds y logs
├── data/                       # [IGNORADO EN GIT] Almacenamiento central de runtime
│   ├── calendar_notes/         # Notas markdown diarias y handoffs_registry.json
│   ├── bubbles_metadata.json   # Metadatos, prioridades y orden de bubbles
│   ├── sessions_state.json     # Historial persistente de terminales
│   └── server.config.json      # Configuración de puerto y token de la máquina
├── dist/                       # [IGNORADO EN GIT] Instalador empaquetado (Orbit-Setup.exe)
├── start-server.bat            # Lanzador rápido desde Windows
├── server.config.json.example  # Plantilla de configuración de ejemplo
├── .gitignore
├── CONTRIBUTING.md             # Guía de contribución
├── CODE_OF_CONDUCT.md         # Código de conducta
├── SECURITY.md                 # Política de seguridad y reporte de vulnerabilidades
└── CHANGELOG.md                # Historial de cambios y versiones
```

---

## 📱 Aplicación Móvil Android (Orbit Mobile)

La app de Android te da control total de tu estación de trabajo desde cualquier lugar con conexión local o túnel WireGuard/Tailscale.

### Compilar e Instalar APK
```bash
cd android
./gradlew assembleDebug
# El APK se genera en: android/app/build/outputs/apk/debug/app-debug.apk
```

---

## 🔒 Seguridad y Privacidad

- **Autenticación Bearer Obligatoria**: Todas las llamadas REST y Handshakes de WebSocket requieren token secreto.
- **Acceso Directo Sin Nube**: Sin dependencias de servicios externos ni telemetría.
- **Instalación Segura sin Elevación**: Diseñado para instalarse en `%LOCALAPPDATA%\Orbit` o `~/.local/bin` sin requerir permisos de Administrador ni root.

---

## 📜 Licencia

Distribuido bajo la Licencia **MIT**. Consulta el archivo [`LICENSE`](LICENSE) para más detalles.
