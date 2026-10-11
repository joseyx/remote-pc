@echo off
title Remote PC Server Host
echo =====================================================
echo           Remote PC Server Host Launcher             
echo =====================================================
cd /d "%~dp0"

if exist "remotepc-server.exe" (
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
