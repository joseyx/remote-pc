# Remote PC - Workspace Cleanup Script
# Cleans temporary build artifacts, distribution packages, and optional cache files.

param (
    [switch]$All,          # Also clean Android build artifacts
    [switch]$ResetSessions # Reset sessions_state.json buffer
)

$ErrorActionPreference = "Continue"
$rootDir = Split-Path -Parent $PSScriptRoot

Write-Host "=====================================================" -ForegroundColor Cyan
Write-Host "            Remote PC - Workspace Cleanup            " -ForegroundColor Cyan
Write-Host "=====================================================" -ForegroundColor Cyan

# 1. Clean compiled binaries
Write-Host "[1] Eliminando binarios de Go compilados..." -ForegroundColor Yellow
$binaries = @(
    (Join-Path $rootDir "remotepc-server.exe"),
    (Join-Path $rootDir "server\remotepc-server.exe")
)
foreach ($b in $binaries) {
    if (Test-Path $b) {
        Remove-Item $b -Force
        Write-Host "    - Eliminado: $b" -ForegroundColor Gray
    }
}

# 2. Clean distribution outputs
Write-Host "[2] Limpiando carpeta dist/..." -ForegroundColor Yellow
$distDir = Join-Path $rootDir "dist"
if (Test-Path $distDir) {
    Remove-Item (Join-Path $distDir "*") -Recurse -Force
    Write-Host "    - Contenido de dist/ eliminado." -ForegroundColor Gray
}

# 3. Clean temporary logs and state
Write-Host "[3] Limpiando archivos temporales (*.tmp, *.log)..." -ForegroundColor Yellow
Get-ChildItem -Path $rootDir -Include "*.tmp", "*.log", "*.out" -Recurse -Depth 2 -File | ForEach-Object {
    Remove-Item $_.FullName -Force
    Write-Host "    - Eliminado: $($_.Name)" -ForegroundColor Gray
}

# 4. Optional: Reset session history buffer
if ($ResetSessions) {
    Write-Host "[4] Reiniciando buffer de sesiones (sessions_state.json)..." -ForegroundColor Yellow
    $sessFile = Join-Path $rootDir "data\sessions_state.json"
    if (Test-Path $sessFile) {
        Remove-Item $sessFile -Force
        Write-Host "    - sessions_state.json eliminado." -ForegroundColor Gray
    }
}

# 5. Optional: Clean Android Gradle builds
if ($All) {
    Write-Host "[5] Limpiando compilaciones de Android..." -ForegroundColor Yellow
    $androidBuilds = @(
        (Join-Path $rootDir "android\build"),
        (Join-Path $rootDir "android\app\build"),
        (Join-Path $rootDir "android\.gradle")
    )
    foreach ($ab in $androidBuilds) {
        if (Test-Path $ab) {
            Remove-Item $ab -Recurse -Force
            Write-Host "    - Eliminado: $ab" -ForegroundColor Gray
        }
    }
}

Write-Host "`n[OK] Limpieza completada exitosamente." -ForegroundColor Green
Write-Host "=====================================================" -ForegroundColor Cyan
