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
- Go 1.22+.

## Ejecución

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
