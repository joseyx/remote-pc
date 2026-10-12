@echo off
title Orbit - Developer Cockpit & MCP Host
echo =====================================================
echo            Orbit - Cockpit & MCP Launcher            
echo =====================================================
cd /d "%~dp0"

if exist "orbit.exe" (
    echo Iniciando orbit.exe...
    orbit.exe -open
) else if exist "server\orbit.exe" (
    echo Iniciando server\orbit.exe...
    server\orbit.exe -open
) else if exist "remotepc-server.exe" (
    echo Iniciando remotepc-server.exe...
    remotepc-server.exe -open
) else if exist "server\remotepc-server.exe" (
    echo Iniciando server\remotepc-server.exe...
    server\remotepc-server.exe -open
) else (
    echo Binario no encontrado. Iniciando via Go...
    cd server
    go run ./cmd/server -open
)

pause
