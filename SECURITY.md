# Política de Seguridad 🛡️

## Versiones Soportadas

Actualmente mantenemos y publicamos parches de seguridad para la última versión mayor:

| Versión | Soportada |
| :--- | :--- |
| 1.0.x | :white_check_mark: |
| < 1.0 | :x: |

---

## Modelo de Seguridad de Remote PC

Remote PC está diseñado bajo el principio de **cero dependencia en la nube**:
1. **Comunicación Local Directa**: Todo el tráfico entre el móvil y la PC viaja en tu red local (LAN) o por túneles VPN privados cifrados de extremo a extremo (Tailscale/WireGuard).
2. **Tokens Bearer Criptográficos**: Cada instancia genera un token aleatorio seguro de 128 bits guardado en `data/server.config.json` para proteger todos los endpoints REST y WebSockets.
3. **Aislamiento de Entorno**: El ejecutable y las sesiones corren bajo el contexto de tu usuario de Windows, sin necesidad de permisos de administrador ni servicios del sistema expuestos públicamente.

---

## Reportar una Vulnerabilidad

Si descubres una vulnerabilidad de seguridad en Remote PC:
1. **Por favor, NO abras un issue público.**
2. Abre un informe de seguridad privado a través de **GitHub Security Advisories** en el repositorio, o contacta directamente al mantenedor.
3. Incluye detalles técnicos, pasos para reproducir y un script o PoC si es posible.

Agradecemos y reconocemos a todos los investigadores que practican la divulgación responsable.
