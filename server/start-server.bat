@echo off
title Remote PC Server Host
echo Starting Remote PC Server Host...
cd /d "%~dp0"
if exist "remotepc-server.exe" (
    remotepc-server.exe -open
) else (
    go run cmd/server/main.go -open
)
pause
