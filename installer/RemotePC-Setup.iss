; Script generated for Inno Setup 6
; Remote PC - Daemon, Terminal & AI MCP Server Suite

#define MyAppName "Remote PC"
#define MyAppVersion "1.0.0"
#define MyAppPublisher "Remote PC"
#define MyAppExeName "remotepc-server.exe"

[Setup]
; Identificador de aplicación único
AppId={{8B264E24-C3D7-48F5-8933-A4CF4D923145}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
DefaultDirName={localappdata}\RemotePC
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
OutputDir=..\dist
OutputBaseFilename=RemotePC-Setup
Compression=lzma2/ultra64
SolidCompression=yes
; Instalación por usuario (no requiere permisos de Administrador ni UAC)
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog
ArchitecturesInstallIn64BitMode=x64compatible
UninstallDisplayIcon={app}\{#MyAppExeName}
SetupLogging=yes
WizardStyle=modern

[Languages]
Name: "spanish"; MessagesFile: "compiler:Languages\Spanish.isl"
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "Crear un acceso directo en el Escritorio"; GroupDescription: "Accesos directos:"; Flags: unchecked
Name: "addtopath"; Description: "Agregar Remote PC al PATH del usuario (para ejecutar 'remotepc-server' en cualquier consola)"; GroupDescription: "Entorno del sistema:";
Name: "setupmcp"; Description: "Configurar servidor MCP para Claude Code CLI y Claude Desktop automáticamente"; GroupDescription: "Integración con IA (Model Context Protocol):";

[Files]
; Ejecutable principal
Source: "..\server\{#MyAppExeName}"; DestDir: "{app}"; Flags: ignoreversion
; Archivo de configuración (solo si no existe para no sobreescribir tokens de sesiones previas)
Source: "..\server\server.config.json"; DestDir: "{app}"; Flags: onlyifdoesntexist uninsneveruninstall
; Scripts y utilidades opcionales
Source: "..\server\scripts\*"; DestDir: "{app}\scripts"; Flags: ignoreversion recursesubdirs createallsubdirs
; Documentación
Source: "..\server\README.md"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{group}\Desinstalar {#MyAppName}"; Filename: "{uninstallexe}"
Name: "{userdesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Run]
; Configuración de variables de entorno y MCP durante la instalación
Filename: "{app}\{#MyAppExeName}"; Parameters: "add-path"; Tasks: addtopath; StatusMsg: "Configurando variable PATH del usuario..."; Flags: runhidden
Filename: "{app}\{#MyAppExeName}"; Parameters: "setup-mcp"; Tasks: setupmcp; StatusMsg: "Configurando integración MCP con Claude Code y Claude Desktop..."; Flags: runhidden
; Opción de iniciar la aplicación al finalizar
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent

[UninstallRun]
; Limpieza limpia al desinstalar
Filename: "{app}\{#MyAppExeName}"; Parameters: "uninstall-mcp"; RunOnceId: "RemotePC_UninstallMCP"; Flags: runhidden
Filename: "{app}\{#MyAppExeName}"; Parameters: "remove-path"; RunOnceId: "RemotePC_RemovePath"; Flags: runhidden
