# Remote PC - Servidor Host (Windows ConPTY, WebSockets & Windscribe VPN)

Servidor daemon de alto rendimiento para Windows que permite crear y administrar múltiples instancias de terminal con **Windows Pseudo Console (ConPTY)** de forma remota, ideal para controlar **Claude Code** y shells interactivas.

## Características

- **ConPTY Nativo**: Soporte completo para aplicaciones TUI/raw terminal (secuencias ANSI, colores de 24 bits, menús y cursores de Claude Code).
- **Persistencia tipo tmux**: Las sesiones continúan ejecutándose en segundo plano aunque el cliente móvil se desconecte o pierda cobertura.
- **Búfer Circular**: Guarda el historial de salida reciente (configurable en KB) para reenviarlo instantáneamente al reconectar.
- **Desktop Web Dashboard**: Interfaz web embebida en `http://localhost:8080/` con espejo de terminal en vivo (xterm.js), monitor de clientes conectados y estadísticas.
- **Controlador Nativo de Windscribe VPN**: Permite conectar, desconectar y cambiar locaciones y protocolos (WireGuard, Stealth:443, UDP, TCP, WSTunnel, IKEv2) vía API sin bloqueos de proceso.
- **Navegación de Directorios Remota**: Permite al cliente Android inspeccionar carpetas en la PC para seleccionar el directorio de trabajo exacto de cada proyecto.
- **Aislamiento de Red**: Prioriza las interfaces físicas Wi-Fi/Ethernet sobre los adaptadores virtuales VPN para asegurar que el emparejamiento con el móvil sea siempre confiable.
- **Cifrado y Seguridad**: Autenticación por token Bearer persistente en `server.config.json`.

## Requisitos

- Windows 10 (versión 1809 o superior) o Windows 11.
- Inno Setup 6 (opcional, solo para compilar el instalador gráfico).
- Go 1.22+ (para compilar desde código fuente).

## Instalador para Usuarios (`RemotePC-Setup.exe`)

Para desplegar Remote PC en cualquier computadora sin necesidad de instalar Go ni Python:
1. Ejecuta el instalador gráfico `dist\RemotePC-Setup.exe`.
2. El asistente permite:
   - Instalar por usuario en `%LOCALAPPDATA%\RemotePC` (no requiere permisos de Administrador).
   - Agregar automáticamente al `PATH` del usuario.
   - Configurar el servidor MCP para **Claude Code CLI** y **Claude Desktop** de forma desatendida.
   - Crear acceso directo en el Escritorio y Menú Inicio.

### Generar el Instalador
Para recompilar el binario y empaquetar el instalador con Inno Setup:
```powershell
.\build-installer.ps1
```
Generará `dist\RemotePC-Setup.exe` optimizado con compresión LZMA2.

## Comandos CLI de `remotepc-server.exe`

El binario compilado incluye herramientas de autoservicio para terminal:

| Comando | Descripción |
| :--- | :--- |
| `remotepc-server` | Inicia el servidor daemon y la interfaz web en segundo plano |
| `remotepc-server mcp` | Ejecuta el servidor MCP JSON-RPC 2.0 por stdio (utilizado por Claude) |
| `remotepc-server setup-mcp` | Configura el servidor MCP en Claude Code y Claude Desktop para el usuario actual |
| `remotepc-server uninstall-mcp` | Remueve la configuración MCP de Claude Code y Claude Desktop |
| `remotepc-server add-path` | Agrega el directorio del binario al `PATH` de Windows del usuario |
| `remotepc-server remove-path` | Remueve el directorio del binario del `PATH` del usuario |
| `remotepc-server install` | Autoinstalación completa (PATH + MCP + Acceso directo) |
| `remotepc-server uninstall` | Desinstalación limpia de entorno y MCP |
| `remotepc-server version` | Muestra la versión actual |
| `remotepc-server help` | Muestra la ayuda de comandos |

## Servidor MCP Nativo (Model Context Protocol)

Remote PC implementa el protocolo MCP directamente en Go, permitiendo a Claude Code y Claude Desktop interactuar con:
- `save_daily_handoff`: Registra cierres de jornada y avances diarios directamente en el Calendario.
- `get_standup_pending`: Consulta pendientes del día anterior o de fechas específicas.
- `get_today_note`: Consulta notas y tareas del día.
- `update_daily_handoff`: Edita un handoff existente.
- `delete_daily_handoff`: Elimina un handoff.
- `add_daily_todo`: Agrega tareas accionables con checkboxes `[ ]` o `[x]`.
- `append_daily_note`: Anexa notas libres a la bitácora del día.
- `list_workspace_bubbles`: Consulta todos los entornos de trabajo y branches activos.
- `get_bubble_context`: Obtiene el estado Git, commits y puerto asignado de la tarea actual.
- `create_workspace_bubble`: Crea un nuevo worktree y branch aislado para una tarea (Aditum, Brif).

Si el servidor daemon no está en ejecución cuando Claude invoca una herramienta MCP, `remotepc-server mcp` inicia automáticamente el servicio en segundo plano sin bloquear al agente.

## Ejecución Manual

### Modo directo
```powershell
go run cmd/server/main.go
```

### Con argumentos personalizados
```powershell
go run cmd/server/main.go -port 8080 -token MiClaveSegura123 -host 0.0.0.0 -open
```

### Compilar binario ejecutable
```powershell
go build -ldflags="-s -w" -o remotepc-server.exe ./cmd/server
.\remotepc-server.exe -open
```

## Configuración Persistente (`server.config.json`)

Si existe un archivo `server.config.json` en el directorio de trabajo, el servidor cargará automáticamente sus parámetros:

```json
{
  "port": "8080",
  "authToken": "tu_token_seguro",
  "host": "0.0.0.0",
  "defaultShell": "powershell.exe",
  "maxHistoryKB": 64
}
```

## Endpoints de la API

| Método | Endpoint | Descripción |
| :--- | :--- | :--- |
| `GET` | `/` | Desktop Web Dashboard embebido con Live Terminal Mirror |
| `GET` | `/api/health` | Comprobación de estado del servidor |
| `GET` | `/api/server-info` | Información general (hostname, uptime, IPs físicas, estado de VPN) |
| `GET` | `/api/connections` | Lista de clientes móviles/web conectados en vivo |
| `GET` | `/api/fs/directories?path=...` | Listar carpetas de la PC para elegir proyectos |
| `GET` | `/api/sessions` | Listar sesiones activas (PID, estado, carpeta) |
| `POST` | `/api/sessions` | Crear nueva sesión (`{ name, cwd, command, cols, rows }`) |
| `DELETE` | `/api/sessions/{id}` | Terminar sesión y matar proceso |
| `WS` | `/ws/session/{id}?token=...` | Canal bidireccional de terminal (I/O, resize) |
| `GET` | `/api/vpn/status` | Estado actual de Windscribe VPN (conectado, IP, protocolo, firewall) |
| `GET` | `/api/vpn/options` | Catálogo de locaciones y protocolos con puertos válidos |
| `POST` | `/api/vpn/connect` | Conectar VPN (`{ location, protocol, port }`) |
| `POST` | `/api/vpn/disconnect` | Desconectar túnel VPN |
