; Script generated for Inno Setup 6
; Orbit - Developer Cockpit & Native AI MCP Orchestrator

#define MyAppName "Orbit"
#ifndef MyAppVersion
#define MyAppVersion "1.2.1"
#endif
#define MyAppPublisher "Orbit"
#define MyAppExeName "orbit.exe"

[Setup]
; Identificador de aplicación único
AppId={{8B264E24-C3D7-48F5-8933-A4CF4D923145}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
DefaultDirName={localappdata}\Orbit
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
OutputDir=..\dist
OutputBaseFilename=Orbit-Setup
Compression=lzma2/ultra64
SolidCompression=yes
; Instalación por usuario (no requiere permisos de Administrador ni UAC)
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog
ArchitecturesInstallIn64BitMode=x64compatible
UninstallDisplayIcon={app}\{#MyAppExeName}
SetupIconFile=orbit.ico
SetupLogging=yes
WizardStyle=modern

[Languages]
Name: "spanish"; MessagesFile: "compiler:Languages\Spanish.isl"
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "Crear un acceso directo en el Escritorio"; GroupDescription: "Accesos directos:"; Flags: unchecked
Name: "addtopath"; Description: "Agregar Orbit al PATH del usuario (para ejecutar 'orbit' o 'remotepc-server' en cualquier consola)"; GroupDescription: "Entorno del sistema:";
Name: "setupmcp"; Description: "Configurar servidor MCP para Claude Code CLI y Claude Desktop automáticamente"; GroupDescription: "Integración con IA (Model Context Protocol):";

[Files]
; Ejecutable principal
Source: "..\server\{#MyAppExeName}"; DestDir: "{app}"; Flags: ignoreversion
; Binario retrocompatible
Source: "..\server\{#MyAppExeName}"; DestDir: "{app}"; DestName: "remotepc-server.exe"; Flags: ignoreversion
; Archivo de configuracion de ejemplo si existe
Source: "..\server.config.json.example"; DestDir: "{app}"; Flags: ignoreversion skipifsourcedoesntexist
; Documentación
Source: "..\server\README.md"; DestDir: "{app}"; Flags: ignoreversion skipifsourcedoesntexist

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{group}\Desinstalar {#MyAppName}"; Filename: "{uninstallexe}"
Name: "{userdesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Run]
; Configuración de variables de entorno, MCP y migración de datos durante la instalación
Filename: "{app}\{#MyAppExeName}"; Parameters: "add-path"; Tasks: addtopath; StatusMsg: "Configurando variable PATH del usuario..."; Flags: runhidden
Filename: "{app}\{#MyAppExeName}"; Parameters: "setup-mcp"; Tasks: setupmcp; StatusMsg: "Configurando integración MCP con Claude Code y Claude Desktop..."; Flags: runhidden
Filename: "{app}\{#MyAppExeName}"; Parameters: "migrate"; StatusMsg: "Migrando datos y proyectos anteriores a la versión abstraída..."; Flags: runhidden
; Opción de iniciar la aplicación al finalizar
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent

[UninstallRun]
; Limpieza limpia al desinstalar
Filename: "{app}\{#MyAppExeName}"; Parameters: "uninstall-mcp"; RunOnceId: "Orbit_UninstallMCP"; Flags: runhidden
Filename: "{app}\{#MyAppExeName}"; Parameters: "remove-path"; RunOnceId: "Orbit_RemovePath"; Flags: runhidden
