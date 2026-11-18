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

    $running = Get-Process ytmusic -ErrorAction SilentlyContinue
    if ($running) {
        Write-Host "==> Stopping running ytmusic instance before build..." -ForegroundColor Yellow
        $running | Stop-Process -Force
        Start-Sleep -Milliseconds 300
    }

    if ($Clean) {
        Write-Host "==> Cleaning previous build artifacts..." -ForegroundColor Yellow
        if (Test-Path $binDir) { Remove-Item -Recurse -Force $binDir }
        if (Test-Path $rootExe) { Remove-Item -Force $rootExe }
        $sysoFile = Join-Path $RootDir "cmd\ytmusic\rsrc_windows_amd64.syso"
        if (Test-Path $sysoFile) { Remove-Item -Force $sysoFile }
    }

    Write-Host "==> Syncing Go module dependencies (go mod tidy)..." -ForegroundColor Cyan
    & go mod tidy
    if ($LASTEXITCODE -ne 0) { throw "go mod tidy failed." }

    Write-Host "==> Generating embedded app icon & PE resources..." -ForegroundColor Cyan
    & go run ./cmd/genicon
    if ($LASTEXITCODE -ne 0) { throw "icon resource generation failed." }

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

    # Notify Windows Explorer to refresh cached executable icons immediately.
    try {
        Add-Type -TypeDefinition @"
using System;
using System.Runtime.InteropServices;
public static class ShellIconRefresh {
    [DllImport("shell32.dll")]
    public static extern void SHChangeNotify(uint wEventId, uint uFlags, IntPtr dwItem1, IntPtr dwItem2);
}
"@ -ErrorAction SilentlyContinue
        [ShellIconRefresh]::SHChangeNotify(0x08000000, 0x0000, [IntPtr]::Zero, [IntPtr]::Zero)
        if (Get-Command ie4uinit.exe -ErrorAction SilentlyContinue) {
            Start-Process -FilePath "ie4uinit.exe" -ArgumentList "-show" -NoNewWindow -ErrorAction SilentlyContinue
        }
    } catch {}

    $sizeMB = [Math]::Round(((Get-Item $binExe).Length / 1MB), 2)
    Write-Host "==> Build succeeded!" -ForegroundColor Green
    Write-Host "    Binary: $binExe ($sizeMB MB)" -ForegroundColor Green
    Write-Host "    Copy:   $rootExe" -ForegroundColor Green

    Write-Host ""
    Write-Host "==> Lines of Code Summary" -ForegroundColor Cyan
    $langMap = @{
        ".go"   = "Golang"
        ".js"   = "JavaScript"
        ".css"  = "CSS"
        ".html" = "HTML"
        ".ps1"  = "PowerShell"
        ".md"   = "Markdown"
        ".mod"  = "Go Module"
    }
    $stats = @{}
    $sourceFiles = @(Get-ChildItem -Path $RootDir -Recurse -File | Where-Object {
        $rel = $_.FullName.Substring($RootDir.Length).TrimStart('\', '/')
        -not ($rel -match '^(?:\.git|bin)(?:\\|/|$)') -and $langMap.ContainsKey($_.Extension.ToLowerInvariant())
    })
    foreach ($f in $sourceFiles) {
        $lang = $langMap[$f.Extension.ToLowerInvariant()]
        if (-not $stats.ContainsKey($lang)) {
            $stats[$lang] = [PSCustomObject]@{
                Language = $lang
                Files    = 0
                Code     = 0
                Blank    = 0
                Total    = 0
            }
        }
        $lines = [System.IO.File]::ReadAllLines($f.FullName)
        $nonEmpty = 0
        foreach ($line in $lines) {
            if (-not [string]::IsNullOrWhiteSpace($line)) {
                $nonEmpty++
            }
        }
        $entry = $stats[$lang]
        $entry.Files += 1
        $entry.Total += $lines.Length
        $entry.Code += $nonEmpty
        $entry.Blank += ($lines.Length - $nonEmpty)
    }

    $rows = @($stats.Values | Sort-Object -Property Total -Descending)
    $totalFiles = 0
    $totalCode = 0
    $totalBlank = 0
    $totalLines = 0
    foreach ($r in $rows) {
        $totalFiles += $r.Files
        $totalCode += $r.Code
        $totalBlank += $r.Blank
        $totalLines += $r.Total
    }

    Write-Host ("    {0,-14} {1,6} {2,8} {3,8} {4,8}" -f "Language", "Files", "Code", "Blank", "Total") -ForegroundColor DarkGray
    Write-Host ("    {0,-14} {1,6} {2,8} {3,8} {4,8}" -f ("-" * 14), ("-" * 6), ("-" * 8), ("-" * 8), ("-" * 8)) -ForegroundColor DarkGray
    foreach ($r in $rows) {
        Write-Host ("    {0,-14} {1,6} {2,8} {3,8} {4,8}" -f $r.Language, $r.Files, $r.Code, $r.Blank, $r.Total)
    }
    Write-Host ("    {0,-14} {1,6} {2,8} {3,8} {4,8}" -f ("-" * 14), ("-" * 6), ("-" * 8), ("-" * 8), ("-" * 8)) -ForegroundColor DarkGray
    Write-Host ("    {0,-14} {1,6} {2,8} {3,8} {4,8}" -f "TOTAL", $totalFiles, $totalCode, $totalBlank, $totalLines) -ForegroundColor White
    Write-Host ""

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
