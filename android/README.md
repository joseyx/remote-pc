# Remote PC - Cliente Android Nativo (Jetpack Compose)

Aplicación nativa para Android diseñada para conectarse al servidor de Remote PC, gestionar instancias de terminal y controlar **Claude Code** en tiempo real.

## Características

- **100% Kotlin + Jetpack Compose**: Interfaz moderna, reactiva y adaptada a Material Design 3 en modo oscuro.
- **Barra de Atajos para Claude Code**:
  - Botones dedicados para teclas que no existen en teclados virtuales móviles estándar: `ESC`, `TAB`, `CTRL+C` (cancelar prompt), `CTRL+D`, flechas direccionales (`↑`, `↓`, `←`, `→`), y `ENTER`.
  - Acciones rápidas con un toque: confirmación (`✓ YES / y`), rechazo (`✗ NO / n`), comandos frecuentes (`/cost`, `/compact`, `/help`, `/clear`).
- **Panel de Control Windscribe VPN Integrado (`VpnCard`)**:
  - Tarjeta en vivo en la pantalla de sesiones con indicador luminoso de conexión.
  - Muestra ciudad conectada, IP pública y protocolo activo.
  - Panel expandible con chips táctiles para cambiar entre múltiples locaciones (*Best*, *Miami*, *New York*, *Dallas*, *Chicago*, *Los Angeles*, *Toronto*, *London*, etc., o ciudad personalizada).
  - Selector de protocolos en un toque: *WireGuard*, *Stealth (443)*, *OpenVPN UDP/TCP*, *WSTunnel* e *IKEv2*.
  - Botón de transición en caliente `⚡ Apply & Switch` para cambiar de servidor o protocolo sin desconectar.
- **Navegador de Carpetas Remoto**: Permite explorar el disco duro de la PC desde el móvil para crear una terminal en la carpeta exacta del proyecto.
- **Múltiples Sesiones**: Puedes tener varias sesiones abiertas en paralelo (ej. Claude trabajando en el backend, otra instancia en el frontend y una PowerShell libre) y alternar entre ellas en cualquier momento sin interrumpir los procesos.
- **Desplazamiento y Auto-scroll Inteligente**: Desplazamiento fluido por todo el historial y seguimiento automático de la última línea de texto generada por Claude Code.
- **Ajuste Dinámico de Columnas (Resize)**: Envía eventos de redimensión al servidor para que el formateo de tablas y menús de Claude Code se adapte a pantallas móviles o modo horizontal.

## Compilación e Instalación

### Con Android Studio:
1. Abre la carpeta `android` en Android Studio.
2. Sincroniza Gradle (`Sync Project with Gradle Files`).
3. Conecta tu teléfono por USB con Depuración USB habilitada.
4. Presiona **Run** (`Shift + F10`).

### Desde línea de comandos:
```powershell
cd android
./gradlew assembleDebug
adb install -r app/build/outputs/apk/debug/app-debug.apk
```

## Configuración de Conexión

1. Abre la app en el móvil y presiona el ícono de **Ajustes** (engranaje) en la barra superior.
2. Introduce los datos del servidor:
   - **Host / IP**: La IP LAN física de tu PC (ej. `192.168.68.54`) o tu IP de **Tailscale** (ej. `100.x.y.z`).
   - **Puerto**: `8080` (o el puerto configurado).
   - **Auth Token**: El token generado o definido en `server.config.json`.
3. Presiona **Test Connection** para verificar y luego **Save & Return**.
