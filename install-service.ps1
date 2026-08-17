# install-service.ps1
# Instala FprinterAgent como servicio nativo de Windows.
# Ejecutar como Administrador.
#
# Uso:
#   .\install-service.ps1 install
#   .\install-service.ps1 uninstall
#   .\install-service.ps1 restart
#   .\install-service.ps1 status

param(
    [string]$Action = "install",
    [string]$InstallDir = "C:\FprinterAgent",
    [string]$ServiceName = "FprinterAgent"
)

$ErrorActionPreference = "Stop"

function Require-Admin {
    $current = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($current)
    if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        Write-Host "Este script requiere permisos de Administrador" -ForegroundColor Red
        exit 1
    }
}

function Install-Agent {
    Require-Admin

    $existing = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if ($existing) {
        Write-Host "Servicio existente, deteniendo..." -ForegroundColor Yellow
        if ($existing.Status -eq 'Running') { Stop-Service $ServiceName -Force }
        sc.exe delete $ServiceName | Out-Null
        Start-Sleep -Seconds 2
    }

    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir | Out-Null
    }

    $sourceExe = Join-Path $PSScriptRoot "FprinterAgent.exe"
    $sourceCfg = Join-Path $PSScriptRoot "config.json"

    if (-not (Test-Path $sourceExe)) {
        Write-Host "No se encuentra FprinterAgent.exe junto al script. Ejecuta build.sh primero." -ForegroundColor Red
        exit 1
    }

    Copy-Item $sourceExe $InstallDir -Force
    # No sobrescribir config existente (preserva token y configuración local)
    if (-not (Test-Path (Join-Path $InstallDir "config.json"))) {
        Copy-Item $sourceCfg $InstallDir -Force
        Write-Host "config.json copiado. EDITALO antes de iniciar." -ForegroundColor Yellow
    } else {
        Write-Host "config.json existente preservado en $InstallDir" -ForegroundColor Cyan
    }

    $exePath = Join-Path $InstallDir "FprinterAgent.exe"
    sc.exe create $ServiceName binPath= "`"$exePath`"" start= auto DisplayName= "Print Agent (ESC/POS)"
    sc.exe description $ServiceName "Agente local de impresion termica ESC/POS"

    # Reinicio automatico ante crash: 1er fallo tras 5s, 2do tras 10s, demas tras 30s
    sc.exe failure $ServiceName reset= 86400 actions= restart/5000/restart/10000/restart/30000 | Out-Null

    Write-Host ""
    Write-Host "Servicio instalado, iniciando..." -ForegroundColor Green
    Start-Service $ServiceName
    Start-Sleep -Seconds 2
    Show-Status
}

function Uninstall-Agent {
    Require-Admin
    $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if (-not $svc) {
        Write-Host "El servicio no existe" -ForegroundColor Yellow
        return
    }
    if ($svc.Status -eq 'Running') { Stop-Service $ServiceName -Force }
    sc.exe delete $ServiceName
    Write-Host "Servicio desinstalado" -ForegroundColor Green
}

function Restart-Agent {
    Require-Admin
    Restart-Service $ServiceName -Force
    Start-Sleep -Seconds 2
    Show-Status
}

function Show-Status {
    $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if (-not $svc) {
        Write-Host "Servicio no instalado" -ForegroundColor Yellow
        return
    }
    Write-Host ""
    Write-Host "Servicio: $($svc.Name)" -ForegroundColor Cyan
    Write-Host "Estado:   $($svc.Status)" -ForegroundColor $(if ($svc.Status -eq 'Running') { 'Green' } else { 'Yellow' })

    try {
        $r = Invoke-RestMethod -Uri "http://127.0.0.1:8765/health" -TimeoutSec 2
        Write-Host "Health:   OK (impresora: $($r.printer))" -ForegroundColor Green
    } catch {
        Write-Host "Health:   no responde" -ForegroundColor Yellow
    }
}

switch ($Action.ToLower()) {
    "install"   { Install-Agent }
    "uninstall" { Uninstall-Agent }
    "restart"   { Restart-Agent }
    "status"    { Show-Status }
    default     { Write-Host "Accion desconocida: $Action. Usa install/uninstall/restart/status" }
}
