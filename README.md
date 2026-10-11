# 🚀 Remote PC: Eco Workspace Hub & Control Remoto para Claude Code

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Platform](https://img.shields.io/badge/Platform-Windows%2010%20%2F%2011-0078D6?style=flat&logo=windows)](https://microsoft.com)
[![Android](https://img.shields.io/badge/Android-Kotlin%20%2B%20Compose-3DDC84?style=flat&logo=android)](https://developer.android.com)
[![Protocol](https://img.shields.io/badge/Streaming-WebSockets%20%2B%20ConPTY-FF6B6B?style=flat)]()
[![VPN Integration](https://img.shields.io/badge/VPN-Windscribe%20CLI-0066FF?style=flat)]()
[![Obsidian](https://img.shields.io/badge/Obsidian-Vaults%20Sync-7C3AED?style=flat&logo=obsidian)]()

**Remote PC** es una suite completa de desarrollo remoto y centro de operaciones de ingeniería (**Eco Workspace Hub**) diseñada para operar todos tus proyectos de software (**Aditum** y **Brif**) con aislamiento por tareas (*bubbles*), soporte nativo para **Git Worktrees**, bases de datos locales multiesquema en **MySQL 8.0**, integración directa con tus bóvedas de **Obsidian**, previsualización web en vivo, y ejecución supervisada de **Claude Code** y shells interactivas tanto desde tu **teléfono Android** como desde un **Dashboard Web en tiempo real**.

Sin servidores en la nube de terceros, sin telemetría y con latencia ultra baja en red local (LAN) o VPN privada (Tailscale).

---

## 🌟 Características Principales

### 🫧 Eco Workspace Hub & Gestión de Bubbles
- **Organización por Workspaces**: Separación de contextos para **Aditum** (Java / Spring Boot + MySQL 8.0 multi-esquema + Liquibase) y **Brif** (Django + Scrapy + Celery + GitLab).
- **Flujo de Trabajo Paralelo de Aditum con Worktrees**:
  - Creación automatizada de worktrees aislados (`C:\Users\Jose\bin\start-worktree.sh <TAR-ID>`).
  - Clonado de base de datos MySQL local en ~25 segundos desde la semilla ultraligera `aditum_slim` hacia el esquema `aditum_<branch>` sin necesidad de Docker.
  - Asignación inteligente de puertos dinámicos (`8080+N`, ej. `:8081` a `:8099`) y lectura de variables `.worktree.env`.
  - Control de Dev Server con arranque/parada en un clic (`start-backend.sh` o Maven).
- **Flujo de Trabajo Lineal y Worktrees en Brif**:
  - Soporte para trabajo lineal ágil en el repositorio principal (`brif-scrapper`) mediante cambio de rama en caliente (`feat/<slug>`, `fix/<slug>`).
  - Modo opcional de worktree aislado (`brif-scrapper-<slug>`) para tareas de largo aliento.
  - Control de servidor Django (`python manage.py runserver`).
- **Eliminación y Limpieza Segura**:
  - Destrucción limpia de worktrees con `remove-worktree.sh` que purga la carpeta y el esquema de base de datos asociado de forma segura.

### 📓 Integración Nativa con Bóvedas de Obsidian (Runbooks)
- Detección automática de las bóvedas locales de Obsidian desde `%APPDATA%\obsidian\obsidian.json`.
- Acceso directo a las bóvedas de **Aditum** (`Vaultsa\Aditum`) y **Brif** (`Vaultsa\Brif\Brif`).
- Búsqueda instantánea de runbooks, incidentes, guías de despliegue y documentación de arquitectura.
- Lector de Markdown integrado (`marked.js`) y editor en caliente para consultar guías de trabajo y comandos sin abandonar la consola.

### 🔀 Git & Diff Viewer Integrado
- Vista en tiempo real del estado de Git para cada bubble: rama actual, commits pendientes (ahead / behind) y árbol sucio.
- Lista de archivos modificados, staged y sin rastrear con códigos de estado (`M`, `A`, `D`, `??`).
- Visor de diferencias unificadas (*diff*) línea por línea con resaltado visual de inserciones y eliminaciones.

### 🌐 Embedded Live Preview
- Pestaña de previsualización en vivo mediante un iframe embebido apuntando a `http://localhost:<PORT>`.
- Barra de navegación con recarga rápida y apertura en nueva pestaña para inspeccionar APIs, Swagger UI o paneles administrativos.

### 🖥️ Windows Pseudo Console (ConPTY) Nativo & Multi-Terminal
- Emulación de terminal raw completa en Windows a través de la API oficial `CreatePseudoConsole` de Windows (`kernel32.dll`).
- Soporte íntegro para secuencias de escape ANSI, colores de 24 bits, menús interactivos, spinners y cursores dinámicos de **Claude Code**.
- Pestañas multi-terminal por tarea: subpestañas independientes para `Claude Code`, `Dev Server` y `Shells (Git Bash / PowerShell)`.
- **VPN Guard para Claude Code**: Verificación previa del estado de Windscribe VPN antes de lanzar sesiones de Claude Code con advertencia y conexión en un clic si el túnel está inactivo.
- Persistencia de sesiones estilo **tmux**: los procesos continúan ejecutándose en tu PC en segundo plano aunque cierres la app del teléfono o pierdas la conexión Wi-Fi temporalmente.
- Búfer circular de terminal (2 MB configurable) que envía instantáneamente el historial reciente a cualquier cliente al reconectar.

### 📱 Aplicación Móvil Android (100% Nativa)
- Construida en **Kotlin** con **Jetpack Compose** y **Material Design 3**.
- **Barra de Atajos Especializada para Claude Code**:
  - Teclas rápidas: `ESC`, `TAB`, `CTRL+C`, `CTRL+D`, flechas de navegación (`↑`, `↓`, `←`, `→`) y `ENTER`.
  - Botones de acción rápida: `✓ YES / y`, `✗ NO / n`, `/cost`, `/compact`, `/help`, `/clear`.
- **Explorador de Archivos Remoto**: Navega por los discos y carpetas de tu PC desde el móvil para crear sesiones en la ruta exacta de cualquier proyecto.
- **Scroll Táctil Cinético Sub-píxel**: Desplazamiento ultra fluido con inercia física y botón flotante de retorno rápido al cursor.

### 🛡️ Integración Nativa con Windscribe VPN
- Diseñado para superar bloqueos geográficos de Claude Code (errores HTTP 403 Forbidden).
- **Selector de Locaciones**: Conexión a *Best Location (Auto)*, *Miami*, *New York*, *Dallas*, *Chicago*, *Los Angeles*, *Seattle*, *Toronto*, *London*, *Frankfurt*, *Paris*, *Amsterdam*, *Zurich*, *Tokyo* o cualquier ciudad/código personalizado.
- **Selector de Protocolos**: *WireGuard* (más rápido), *Stealth:443* (anti-DPI / TLS ofuscado), *OpenVPN UDP*, *OpenVPN TCP*, *WSTunnel* e *IKEv2*.
- **Cambio en Caliente (On-the-fly Switch)**: Cambia de servidor o protocolo sin necesidad de desconectarte manualmente primero.
- **Priorización de IP Física**: El servidor detecta y filtra adaptadores virtuales para que la app del móvil siempre apunte a tu IP Wi-Fi física (`192.168.x.x`), manteniendo la comunicación teléfono-PC intacta (con *Allow LAN Traffic* activo en Windscribe).

---

## 🏛️ Arquitectura del Sistema

```mermaid
flowchart TD
    subgraph PC["🖥️ Workstation Windows (Host)"]
        subgraph ServerDaemon["remotepc-server (Go Backend)"]
            BubbleMgr["🫧 Bubble Manager (Aditum & Brif Worktrees)"]
            GitPkg["🔀 Git Status, Diff & Branches"]
            ObsidianPkg["📓 Obsidian Vault Discovery & Search"]
            ConPTY["ConPTY Engine (kernel32.dll)"]
            SessionMgr["Session Manager (tmux-style persist)"]
            RingBuf["Ring Buffer (2MB History)"]
            WsServer["WebSocket Server (/ws/session/:id)"]
            RestAPI["REST API (/api/...)"]
            WebDash["Embedded Eco Dashboard (xterm.js + marked)"]
            VpnMgr["Windscribe VPN Controller (CLI Bridge)"]
        end

        Scripts["📜 bin/start-worktree.sh & remove-worktree.sh"]
        MySQL["🗄️ MySQL 8.0 Local (aditum_slim seed)"]
        Vaults["📚 Vaultsa/Aditum & Vaultsa/Brif"]
        ClaudeCLI["🤖 Claude Code CLI / PowerShell"]
        WindscribeApp["🛡️ Windscribe Service & CLI"]

        BubbleMgr --> Scripts
        BubbleMgr --> MySQL
        ObsidianPkg --> Vaults
        ConPTY --> ClaudeCLI
        VpnMgr --> WindscribeApp
    end

    subgraph Mobile["📱 Dispositivo Android (Cliente)"]
        ComposeUI["Jetpack Compose UI (Material 3)"]
        TermView["Terminal Screen + Kinetic Sub-pixel Scroll"]
        ClaudeBar["Claude Shortcut Bar (ESC, Tab, Ctrl+C, Yes/No)"]
        VpnCard["VPN Controller Card (Locations & Protocols)"]
        ApiClient["OkHttp / Coroutines WebSocket Client"]
    end

    subgraph Browser["🌐 Navegador Web (Dashboard Local/Remoto)"]
        EcoWeb["Eco Liquid Glass Hub (/index.html)"]
        EcoTabs["Terminales · Live Preview · Git Diff · Obsidian"]
    end

    ApiClient <-->|Local Wi-Fi / Tailscale| WsServer
    ApiClient <-->|HTTP REST Requests| RestAPI
    Browser <-->|HTTP / WebSockets| ServerDaemon
```

---

## 📁 Estructura del Repositorio

```text
Remote PC/
├── server/                     # Servidor Daemon & MCP en Go
│   ├── cmd/server/             # Punto de entrada CLI, Daemon y MCP (main.go)
│   ├── internal/
│   │   ├── api/                # Enrutador HTTP REST y WebSocket
│   │   ├── bubble/             # Gestor de Bubbles, Worktrees y Dev Servers
│   │   ├── calendar/           # Calendario de Handoffs, Daily Notes y Standup
│   │   ├── config/             # Carga y resolución central de datos (data/)
│   │   ├── git/                # Helpers de Git (status, diff, branch, log)
│   │   ├── installer/          # Registro de Windows (PATH), accesos directos y uninstaller
│   │   ├── mcp/                # Servidor MCP JSON-RPC 2.0 nativo (10 herramientas)
│   │   ├── obsidian/           # Búsqueda y lectura de Vaults de Obsidian
│   │   ├── pty/                # Windows ConPTY API (CreatePseudoConsole)
│   │   ├── session/            # Gestor de terminales, buffer circular y persistencia
│   │   ├── vpn/                # Integración con Windscribe CLI
│   │   └── web/                # Dashboard Web embebido (HTML/CSS/JS + xterm.js + marked)
│   ├── scripts/                # Scripts auxiliares en Python (fallback MCP y CLI)
│   └── README.md
├── android/                    # Cliente Nativo Android (Kotlin + Compose)
│   ├── app/src/main/java/com/remotepc/app/
│   │   ├── data/               # Modelos de Sesión, VPN y Red (WebSocket/OkHttp)
│   │   ├── ui/                 # Interfaz Compose (Terminal, Atajos Claude, VPN Card)
│   │   └── AndroidManifest.xml
│   ├── build.gradle.kts
│   └── README.md
├── installer/                  # Definición del Instalador Gráfico
│   └── RemotePC-Setup.iss      # Script Inno Setup 6 (asistente de instalación por usuario)
├── scripts/                    # Scripts de Automatización y Developer Experience
│   ├── build-installer.ps1     # Compila el binario y empaqueta dist/RemotePC-Setup.exe
│   └── clean.ps1               # Limpieza de temporales, builds y logs
├── data/                       # [IGNORADO EN GIT] Almacenamiento central de runtime
│   ├── calendar_notes/         # Notas markdown diarias y handoffs_registry.json
│   ├── bubbles_metadata.json   # Metadatos, prioridades y orden de bubbles
│   ├── sessions_state.json     # Historial persistente de terminales
│   └── server.config.json      # Configuración de puerto y token de la máquina
├── dist/                       # [IGNORADO EN GIT] Instalador empaquetado (RemotePC-Setup.exe)
├── start-server.bat            # Script de inicio rápido en Windows
├── server.config.json.example  # Plantilla de configuración de ejemplo
├── .gitignore
└── README.md
```

---

## ⚡ Guía de Inicio Rápido

### 1. Requisitos Previos

* **En la PC**:
  * Windows 10 (Build 1809+) o Windows 11.
  * [Go 1.22+](https://go.dev/dl/) instalado.
  * [Claude Code](https://docs.anthropic.com/en/docs/agents-and-tools/claude-code/overview) (`npm install -g @anthropic-ai/claude-code`).
  * *(Opcional)* [Windscribe VPN](https://windscribe.com/) instalado en Windows con la opción **Allow LAN Traffic** activada en sus preferencias.
* **En el Móvil**:
  * Teléfono o tablet con **Android 8.0 (API 26)** o superior.
  * Conectado a la misma red Wi-Fi que la PC (o conectado a tu VPN de Tailscale).

---

### 2. Iniciar el Servidor en Windows

1. Clona el repositorio:
   ```bash
   git clone https://github.com/joseyx/remote-pc.git
   cd "Remote PC"
   ```

2. Ejecuta el script de inicio rápido:
   ```cmd
   start-server.bat
   ```
   *O compila e inicia manualmente:*
   ```powershell
   cd server
   go build -ldflags="-s -w" -o ..\remotepc-server.exe .\cmd\server
   cd ..
   .\remotepc-server.exe -open
   ```

3. El servidor se iniciará en `http://localhost:8080/` y mostrará en consola:
   ```
   =====================================================
          Remote PC Daemon - Terminal & Claude Host     
   =====================================================
    Listening on : 0.0.0.0:8080
    Dashboard UI : http://localhost:8080/
    Auth Token   : 387fca53171520740b591975dcaec681
   -----------------------------------------------------
    Connect from Android with:
      Host/IP : 192.168.x.x
      Port    : 8080
      Token   : 387fca53171520740b591975dcaec681
   =====================================================
   ```

---

### 3. Compilar e Instalar la App Android

Si dispones de Android Studio o el Android SDK configurado:

```bash
cd android
./gradlew assembleDebug
adb install -r app/build/outputs/apk/debug/app-debug.apk
```

---

### 4. Emparejar el Teléfono con la PC

1. Abre la aplicación **Remote PC** en tu teléfono Android.
2. Toca el icono de **Ajustes** (engranaje) en la barra superior.
3. Ingresa los datos mostrados por tu servidor:
   * **Host / IP**: La IP LAN de tu PC (ej. `192.168.68.54`).
   * **Puerto**: `8080`.
   * **Auth Token**: Tu token seguro.
4. Toca **Test Connection** para verificar la comunicación y luego **Save & Return**.

---

### 5. Lanzar Sesiones de Claude Code

1. En la pantalla principal de la app, pulsa el botón flotante **`+`**.
2. **Nombre de sesión**: Ingresa un alias (ej. `Mi Proyecto`).
3. **Directorio de trabajo (CWD)**: Toca el icono de carpeta para navegar visualmente por las carpetas de tu PC y elegir tu proyecto.
4. **Comando**:
   * Selecciona el preset **Claude Code** (`claude`), o
   * Usa una shell directa (`powershell.exe` / `cmd.exe`).
5. Pulsa **Create Session**. ¡Tu sesión de Claude Code comenzará a transmitirse en vivo con todos sus colores, opciones interactivas y atajos móviles listos para usar!

---

## 🛡️ Control de Windscribe VPN

Tanto en la parte superior de la aplicación Android como en la columna lateral del Dashboard Web dispones de una tarjeta dedicada a **Windscribe VPN**:

| Control | Descripción |
| :--- | :--- |
| **Indicador Luminoso** | **Verde**: Túnel activo y cifrado. **Gris**: Desconectado / Conexión directa. **Ámbar**: En proceso de conexión. |
| **Detalles del Túnel** | Muestra en vivo la ciudad conectada, IP pública asignada y protocolo activo (ej. `Miami - Vice · 135.136.5.74 · Stealth:443`). |
| **Desplegable de Ajustes (`Tune` / `^`)** | Permite seleccionar entre múltiples ciudades de USA, Canadá, Europa y Asia, o escribir una ciudad personalizada. |
| **Selector de Protocolos** | Permite elegir entre WireGuard, Stealth:443, OpenVPN UDP/TCP, WSTunnel e IKEv2. |
| **⚡ Apply & Switch** | Permite cambiar de locación o protocolo directamente sobre la marcha sin necesidad de desconectarse primero. |

> [!IMPORTANT]
> Para que el teléfono continúe comunicándose con la PC por Wi-Fi mientras el VPN está conectado, debes habilitar la opción **Allow LAN Traffic** en Windscribe:
> `Windscribe > Preferencias (⚙️) > Connection > Allow LAN Traffic: ON`.

---

## ⚙️ Configuración del Servidor (`server.config.json`)

Puedes personalizar los parámetros del servidor creando un archivo `server.config.json` en la raíz (puedes basarte en `server.config.json.example`):

```json
{
  "port": "8080",
  "authToken": "tu_token_secreto_personalizado",
  "host": "0.0.0.0",
  "defaultShell": "powershell.exe",
  "maxHistoryKB": 64
}
```

* **`port`**: Puerto de escucha HTTP/WebSocket (por defecto: `8080`).
* **`authToken`**: Token Bearer de seguridad. Si se omite, se generará uno aleatorio de 128 bits al iniciar.
* **`host`**: Interfaz de enlace (`0.0.0.0` para escuchar en toda la red local).
* **`defaultShell`**: Shell predeterminada (`powershell.exe`, `cmd.exe`, `bash.exe`).
* **`maxHistoryKB`**: Tamaño máximo del búfer circular por sesión (en KB).

---

## 📡 Referencia de la API REST y WebSockets

Todos los endpoints (excepto `/api/health` y `/`) requieren la cabecera `Authorization: Bearer <TOKEN>` o el parámetro `?token=<TOKEN>`.

### Sesiones de Terminal
* `GET /api/sessions`: Devuelve la lista de sesiones activas (ID, nombre, CWD, PID, estado de ejecución, timestamp).
* `POST /api/sessions`: Crea una nueva terminal ConPTY.
  * *Body*: `{"name": "Claude", "cwd": "C:\\Proyectos", "command": "claude", "cols": 100, "rows": 30}`
* `DELETE /api/sessions/{id}`: Termina el proceso y libera los recursos de la consola virtual.
* `WS /ws/session/{id}`: Conexión WebSocket bidireccional para transmisión binaria de entrada/salida de terminal y redimensionamiento dinámico.

### Sistema de Archivos
* `GET /api/fs/directories?path=C:\`: Explora las subcarpetas del sistema para el selector visual de directorios.

### Windscribe VPN
* `GET /api/vpn/status`: Devuelve el estado actual de la conexión VPN (`installed`, `connected`, `state`, `location`, `ip`, `protocol`, `firewall`).
* `GET /api/vpn/options`: Devuelve el catálogo de locaciones recomendadas y protocolos soportados con sus puertos.
* `POST /api/vpn/connect`: Conecta al VPN con parámetros opcionales `{ "location": "Miami", "protocol": "stealth", "port": "443" }`.
* `POST /api/vpn/disconnect`: Desconecta el túnel VPN.

### Servidor y Diagnóstico
* `GET /api/server-info`: Devuelve hostname, uptime, IPs locales físicas, estado de VPN y contador de clientes conectados.
* `GET /api/connections`: Lista los dispositivos actualmente conectados al servidor (IP, User-Agent, sesión asociada).
* `GET /api/health`: Comprobación rápida de disponibilidad (`status: ok`).

---

## 🔒 Seguridad y Privacidad

* **Totalmente Autónomo**: Ningún dato de tus proyectos, comandos de terminal o claves de Claude viaja a servidores externos; todo permanece estrictamente dentro de tu red local.
* **Tokens Criptográficos**: Todas las llamadas de la API REST y WebSockets están autenticadas con tokens Bearer generados aleatoriamente o configurados por el usuario.
* **Aislamiento de Interfaces**: El servidor filtra los adaptadores de red virtuales (Windscribe, TAP, WSL, Hyper-V) para asegurar que solo se expongan las interfaces Wi-Fi/Ethernet físicas de tu PC.

---

## 📄 Licencia

Este proyecto está bajo la Licencia **MIT**. Consulta el archivo `LICENSE` para más detalles.
