<#
.SYNOPSIS
    Crabby installer for Windows.

.DESCRIPTION
    Installs the crabby.exe wrapper on Windows and, when possible, the Linux
    crabby binary inside WSL. Usage:

        irm https://raw.githubusercontent.com/marioolf/crabby/main/install.ps1 | iex

    Environment overrides:
        CRABBY_REPO   owner/repo to download from (default: marioolf/crabby)
#>

#Requires -Version 5
$ErrorActionPreference = 'Stop'

$Repo       = if ($env:CRABBY_REPO) { $env:CRABBY_REPO } else { 'marioolf/crabby' }
$InstallDir = Join-Path $env:LOCALAPPDATA 'Crabby'
$Binary     = 'crabby.exe'

function Write-Ok   ($m) { Write-Host "OK   $m" -ForegroundColor Green }
function Write-Warn ($m) { Write-Host "WARN $m" -ForegroundColor Yellow }
function Write-Err  ($m) { Write-Host "ERR  $m" -ForegroundColor Red }

Write-Host "Installing Crabby..." -ForegroundColor Cyan

# --- Detect architecture ---------------------------------------------------
switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { $arch = 'amd64' }
    'ARM64' { $arch = 'arm64' }
    default {
        Write-Err "Unsupported architecture: $($env:PROCESSOR_ARCHITECTURE)"
        exit 1
    }
}

# --- Download and install crabby.exe ---------------------------------------
$asset = "crabby_windows_$arch.zip"
$url   = "https://github.com/$Repo/releases/latest/download/$asset"
$tmp   = Join-Path $env:TEMP ("crabby_" + [System.Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $tmp -Force | Out-Null

try {
    Write-Host "   Downloading $asset..."
    Invoke-WebRequest -Uri $url -OutFile (Join-Path $tmp $asset) -UseBasicParsing
    Expand-Archive -Path (Join-Path $tmp $asset) -DestinationPath $tmp -Force

    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    Copy-Item -Path (Join-Path $tmp $Binary) -Destination (Join-Path $InstallDir $Binary) -Force
    Write-Ok "Installed crabby.exe to $InstallDir"
}
catch {
    Write-Err "Download or installation failed: $url"
    Write-Err $_.Exception.Message
    Write-Err "Check that a release exists at https://github.com/$Repo/releases"
    exit 1
}
finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}

# --- Ensure it is on PATH ---------------------------------------------------
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if ($userPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable('Path', "$userPath;$InstallDir", 'User')
    Write-Ok "Added $InstallDir to your user PATH (restart your terminal to pick it up)"
}
else {
    Write-Ok "$InstallDir is already on your PATH"
}

# --- Verify WSL -------------------------------------------------------------
Write-Host "Checking WSL..." -ForegroundColor Cyan
if (-not (Get-Command wsl -ErrorAction SilentlyContinue)) {
    Write-Warn "WSL is not installed. Install it with:  wsl --install"
    Write-Warn "Then re-run this installer to set up the Linux side."
    exit 0
}
Write-Ok "WSL is installed"

# --- Verify an Ubuntu distribution -----------------------------------------
$distros = (wsl -l -q 2>$null) -join "`n"
if ($distros -notmatch 'Ubuntu') {
    Write-Warn "No Ubuntu distribution found. Install one with:  wsl --install -d Ubuntu"
    Write-Warn "Then re-run this installer to set up the Linux side."
    exit 0
}
Write-Ok "Ubuntu distribution found"

# --- Install the Linux binary inside WSL if missing ------------------------
Write-Host "Setting up the Linux crabby binary inside WSL..." -ForegroundColor Cyan
wsl bash -lc "command -v crabby >/dev/null 2>&1"
if ($LASTEXITCODE -eq 0) {
    Write-Ok "Linux crabby is already installed in WSL"
}
else {
    Write-Host "   Installing Linux crabby inside WSL..."
    wsl bash -lc "curl -fsSL https://raw.githubusercontent.com/$Repo/main/install.sh | bash"
    if ($LASTEXITCODE -eq 0) {
        Write-Ok "Linux crabby installed in WSL"
    }
    else {
        Write-Warn "Could not auto-install inside WSL. Run this inside WSL:"
        Write-Warn "  curl -fsSL https://raw.githubusercontent.com/$Repo/main/install.sh | bash"
    }
}

Write-Host ""
Write-Ok "Done! Open a new terminal and run:  crabby doctor"
