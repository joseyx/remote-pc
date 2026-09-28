@echo off
title Remote PC Server Host
cd /d "%~dp0"
if exist "remotepc-server.exe" (
    remotepc-server.exe -open
) else if exist "server\remotepc-server.exe" (
    cd /d "%~dp0server"
    remotepc-server.exe -open
) else (
    cd /d "%~dp0server"
    go run cmd/server/main.go -open
)
pause
