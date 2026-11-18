<#
.SYNOPSIS
    Build script for ytmusic (Windows 11 GUI YouTube Music & Video Player).

.PARAMETER Test
    Run Go unit and integration tests before building.

.PARAMETER Clean
    Remove generated binaries (bin\ and ytmusic.exe) before building.

.PARAMETER DebugMode
    Build with a console window and WebView2 DevTools enabled for debugging.

.PARAMETER Run
    Launch ytmusic.exe after a successful build.
#>
param(
    [switch]$Test,
    [switch]$Clean,
    [switch]$DebugMode,
    [switch]$Run
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$RootDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Push-Location $RootDir

try {
    Write-Host "==> ytmusic build script" -ForegroundColor Cyan

    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        throw "Go toolchain not found in PATH. Please install Go 1.22+ from https://go.dev/dl/."
    }

    $goVer = & go version
    Write-Host "    Go: $goVer" -ForegroundColor DarkGray

    $binDir = Join-Path $RootDir "bin"
    $binExe = Join-Path $binDir "ytmusic.exe"
    $rootExe = Join-Path $RootDir "ytmusic.exe"

    if ($Clean) {
        Write-Host "==> Cleaning previous build artifacts..." -ForegroundColor Yellow
        if (Test-Path $binDir) { Remove-Item -Recurse -Force $binDir }
        if (Test-Path $rootExe) { Remove-Item -Force $rootExe }
        if (-not $Test -and -not $Run) {
            # Still proceed to build unless only clean was desired? Standard build.ps1 cleans then builds.
        }
    }

    Write-Host "==> Syncing Go module dependencies (go mod tidy)..." -ForegroundColor Cyan
    & go mod tidy
    if ($LASTEXITCODE -ne 0) { throw "go mod tidy failed." }

    if ($Test) {
        Write-Host "==> Running Go tests..." -ForegroundColor Cyan
        & go test -v ./...
        if ($LASTEXITCODE -ne 0) { throw "go test failed." }
    }

    New-Item -ItemType Directory -Force -Path $binDir | Out-Null

    $env:GOOS = "windows"
    $env:GOARCH = "amd64"

    if ($DebugMode) {
        Write-Host "==> Building debug binary (console enabled)..." -ForegroundColor Cyan
        & go build -trimpath -o $binExe ./cmd/ytmusic
    } else {
        Write-Host "==> Building release Windows GUI binary..." -ForegroundColor Cyan
        & go build -trimpath -ldflags="-s -w -H=windowsgui" -o $binExe ./cmd/ytmusic
    }
    if ($LASTEXITCODE -ne 0) { throw "go build failed." }

    Copy-Item -Path $binExe -Destination $rootExe -Force

    $sizeMB = [Math]::Round(((Get-Item $binExe).Length / 1MB), 2)
    Write-Host "==> Build succeeded!" -ForegroundColor Green
    Write-Host "    Binary: $binExe ($sizeMB MB)" -ForegroundColor Green
    Write-Host "    Copy:   $rootExe" -ForegroundColor Green

    $ytdlp = Get-Command yt-dlp -ErrorAction SilentlyContinue
    if (-not $ytdlp) {
        $wingetLink = Join-Path $env:LOCALAPPDATA "Microsoft\WinGet\Links\yt-dlp.exe"
        if (-not (Test-Path $wingetLink)) {
            Write-Warning "yt-dlp not detected in PATH; ytmusic will fall back to built-in InnerTube player API. Installing yt-dlp (`winget install yt-dlp.yt-dlp`) is recommended for maximum stream compatibility."
        }
    }

    if ($Run) {
        Write-Host "==> Launching ytmusic.exe..." -ForegroundColor Cyan
        if ($DebugMode) {
            Start-Process -FilePath $binExe -ArgumentList "--debug"
        } else {
            Start-Process -FilePath $binExe
        }
    }
} finally {
    Pop-Location
}
