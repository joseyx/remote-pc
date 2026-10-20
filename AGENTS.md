# Reglas y Directrices para Agentes de IA (AGENTS.md)

Este documento contiene directrices obligatorias y de seguridad que **todo asistente o agente de IA (Antigravity, Claude Code, Cursor, etc.) debe acatar estrictamente** al trabajar en el repositorio de **Orbit** (Remote PC).

---

## 🚨 1. Regla Crítica: Protección de la Instancia Instalada de Orbit

> [!CAUTION]
> **NUNCA DETENER, MATAR NI SOBRESCRIBIR LA INSTANCIA DE ORBIT INSTALADA EN LA PC.**
> El usuario utiliza Orbit como su entorno de trabajo diario, manteniendo activas más de 10 terminales críticas (sesiones de Claude Code, servidores de desarrollo, scripts en ejecución). Si Orbit se cierra o reinicia bruscamente, Windows desconecta los pseudoterminales ConPTY y se pierde el estado de todos los procesos hijos en ejecución.

### Prohibiciones Explícitas:
1. **NUNCA ejecutar `Stop-Process`, `taskkill` ni `orbit service stop`** contra procesos de Orbit en ejecución (`orbit.exe` o `remotepc-server.exe`) en el puerto `8080` ni contra procesos que ejecuten desde `%LOCALAPPDATA%\Orbit`.
2. **NUNCA sobrescribir, mover ni eliminar archivos en `%LOCALAPPDATA%\Orbit` o `%APPDATA%\Orbit`** durante fases de desarrollo o pruebas intermedias.
3. **NUNCA asumir que el puerto `8080` está libre:** El puerto `8080` pertenece al servicio de producción activo en la máquina del usuario.

---

## 🧪 2. Protocolo para Pruebas Locales y Desarrollo

Cuando se requiera compilar, probar endpoints o verificar la interfaz web durante el desarrollo:

### A. Pruebas Unitarias Automatizadas
* Ejecutar normalmente con `go test ./...` desde la carpeta `server`.
* Todas las pruebas unitarias deben utilizar directorios temporales (`t.TempDir()` o `os.MkdirTemp`) y puertos libres/efímeros.
* Las pruebas nunca deben escribir en `data/` del proyecto ni en las carpetas de usuario.

### B. Pruebas Manuales / Smoke Tests del Servidor en Vivo
Si es estrictamente necesario levantar el servidor compilado para probar un cambio:
1. **Compilar únicamente en el directorio del proyecto:**
   ```powershell
   Set-Location server
   go build -o orbit-dev.exe ./cmd/server
   ```
2. **Usar SIEMPRE un puerto secundario (distinto de 8080) y un directorio de datos aislado:**
   ```powershell
   $env:ORBIT_DATA_DIR = "$env:TEMP\orbit-dev-data"
   .\server\orbit-dev.exe -port 8089 -no-open
   ```
3. **Probar contra dicho puerto:**
   ```powershell
   curl.exe -s http://localhost:8089/api/health
   ```
4. **Finalizar únicamente el proceso de prueba específico creado (`orbit-dev.exe` en el puerto 8089), sin tocar el proceso principal:**
   ```powershell
   Stop-Process -Name "orbit-dev" -Force
   ```

---

## 📦 3. Actualización de la Instancia en Producción

La instancia instalada en `%LOCALAPPDATA%\Orbit` **SOLO se actualiza si el usuario lo solicita explícitamente** (por ejemplo: *"instala la nueva versión en mi PC"* o *"compila el instalador"*).

Al realizar una actualización autorizada:
1. Compilar el instalador formal mediante el script oficial:
   ```powershell
   .\scripts\build-installer.ps1
   ```
2. O bien, si el usuario pide actualizar el binario directo, avisarle antes de proceder que las terminales deberán ser restauradas tras el reinicio del servicio.

---

## 🌿 4. Flujo de Trabajo en Git

1. **Ramas protegidas:** `main` y `develop` tienen protección de ramas activa contra pushes directos.
2. **Ramas de trabajo:** Todo desarrollo debe realizarse en ramas de características (`feature/...`), corrección (`fix/...`) o mantenimiento (`chore/...`).
3. **Commits convencionales:** Seguir el estándar Conventional Commits (`feat:`, `fix:`, `chore:`, `refactor:`, etc.).
4. **Pull Requests:** Al terminar una tarea, subir la rama remota y abrir un Pull Request hacia `develop`.
