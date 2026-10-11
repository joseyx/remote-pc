# Remote PC - Build & Package Installer Script
# Compiles the native Go binary and packages it into dist\RemotePC-Setup.exe using Inno Setup 6.

$ErrorActionPreference = "Stop"

Write-Host "=====================================================" -ForegroundColor Cyan
Write-Host "       Remote PC - Build & Installer Packaging       " -ForegroundColor Cyan
Write-Host "=====================================================" -ForegroundColor Cyan

# 1. Locate Inno Setup Compiler (iscc.exe)
$isccPaths = @(
    "C:\Users\Jose\AppData\Local\Programs\Inno Setup 6\iscc.exe",
    "$env:LOCALAPPDATA\Programs\Inno Setup 6\iscc.exe",
    "$env:ProgramFiles\Inno Setup 6\iscc.exe",
    "${env:ProgramFiles(x86)}\Inno Setup 6\iscc.exe"
)

$isccPath = $null
foreach ($p in $isccPaths) {
    if (Test-Path $p) {
        $isccPath = $p
        break
    }
}

if (-not $isccPath) {
    $cmd = Get-Command "iscc.exe" -ErrorAction SilentlyContinue
    if ($cmd) {
        $isccPath = $cmd.Source
    }
}

if (-not $isccPath) {
    Write-Error "No se encontro el compilador de Inno Setup (iscc.exe). Instalalo o verifica su ruta."
    exit 1
}

Write-Host "[1/4] Compilador Inno Setup encontrado: $isccPath" -ForegroundColor Green

# 2. Compile Go binary
Write-Host "[2/4] Compilando remotepc-server.exe (Go con optimizaciones -s -w)..." -ForegroundColor Yellow
Push-Location "$PSScriptRoot\server"
try {
    go build -ldflags="-s -w" -o remotepc-server.exe ./cmd/server
    if ($LASTEXITCODE -ne 0) {
        throw "Error al compilar el binario de Go."
    }
} finally {
    Pop-Location
}

# Copy to root directory for local project convenience
Copy-Item "$PSScriptRoot\server\remotepc-server.exe" -Destination "$PSScriptRoot\remotepc-server.exe" -Force
Write-Host "      [OK] Binario compilado exitosamente." -ForegroundColor Green

# 3. Create dist output directory
$distDir = "$PSScriptRoot\dist"
if (-not (Test-Path $distDir)) {
    New-Item -ItemType Directory -Path $distDir | Out-Null
}

# 4. Compile Inno Setup Script
Write-Host "[3/4] Compilando instalador con Inno Setup..." -ForegroundColor Yellow
$issPath = "$PSScriptRoot\installer\RemotePC-Setup.iss"

& "$isccPath" "$issPath"
if ($LASTEXITCODE -ne 0) {
    Write-Error "Error durante la compilacion del instalador."
    exit 1
}

# 5. Summary
$setupExe = "$distDir\RemotePC-Setup.exe"
if (Test-Path $setupExe) {
    $item = Get-Item $setupExe
    $hash = (Get-FileHash $setupExe -Algorithm SHA256).Hash
    $sizeMB = [math]::Round($item.Length / 1MB, 2)
    $bytes = $item.Length

    Write-Host ""
    Write-Host "=====================================================" -ForegroundColor Cyan
    Write-Host " [OK] Instalador generado exitosamente!" -ForegroundColor Green
    Write-Host "-----------------------------------------------------" -ForegroundColor Cyan
    Write-Host " Archivo : $setupExe" -ForegroundColor White
    Write-Host " Tamano  : $sizeMB MB ($bytes bytes)" -ForegroundColor White
    Write-Host " SHA256  : $hash" -ForegroundColor Gray
    Write-Host "=====================================================" -ForegroundColor Cyan
} else {
    Write-Error "No se encontro el instalador generado en $setupExe"
    exit 1
}
