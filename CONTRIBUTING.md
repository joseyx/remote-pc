# Guía de Contribución a Remote PC 🤝

¡Gracias por tu interés en contribuir a **Remote PC**! Este proyecto es un monorepo que combina un backend en **Go** para Windows, una aplicación móvil en **Kotlin + Jetpack Compose** para Android, y un instalador empaquetado con **Inno Setup**.

---

## 🏗️ Arquitectura del Repositorio

- `server/`: Backend Go con Windows Pseudo Console (ConPTY), WebSocket streaming, servidor MCP nativo y Web Dashboard embebido.
- `android/`: Cliente Android nativo en Kotlin con Jetpack Compose y Material 3.
- `installer/`: Script Inno Setup 6 (`RemotePC-Setup.iss`) para generar el instalador `.exe`.
- `scripts/`: Herramientas de automatización (`build-installer.ps1`, `clean.ps1`).
- `data/`: Almacenamiento local de estado y notas (ignorado en git).

---

## 🛠️ Flujo de Trabajo Local

### 1. Clonar el repositorio
```bash
git clone https://github.com/joseyx/remote-pc.git
cd remote-pc
```

### 2. Backend Go
```powershell
cd server
go test ./...              # Ejecutar tests
go run cmd/server/main.go  # Iniciar servidor en modo desarrollo
```

### 3. Cliente Android
Abre la carpeta `android/` en **Android Studio (Ladybug o superior)**:
- Requiere JDK 17 y Android SDK 34+.
- Compila con:
  ```bash
  cd android
  ./gradlew assembleDebug
  ```

### 4. Compilación del Instalador
Para empaquetar el instalador para usuarios finales:
```powershell
.\scripts\build-installer.ps1
```
Generará `dist\RemotePC-Setup.exe`.

---

## 📐 Convenciones de Commits

Seguimos la convención de [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` Nuevas funcionalidades (ej: `feat(mcp): add calendar notes tool`)
- `fix:` Correcciones de errores (ej: `fix(terminal): handle resize debounce`)
- `perf:` Mejoras de rendimiento o memoria (ej: `perf: reduce buffer allocations`)
- `refactor:` Cambios de código sin alteración funcional (ej: `refactor(repo): isolate runtime data`)
- `docs:` Cambios en la documentación (ej: `docs: update CLI reference`)
- `chore:` Tareas de mantenimiento o CI (ej: `chore(ci): add windows test runner`)

---

## 🧪 Pruebas y Validación

Antes de abrir un Pull Request, asegúrate de:
1. Pasar todas las pruebas de Go: `cd server && go test -v ./...`
2. Verificar que no se suban archivos de datos locales (`data/` y `dist/` deben permanecer ignorados).
3. Mantener el formato idiomático de Go: `go fmt ./...`.
