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

$ErrorActionPreference = 'Stop'

# When run via `irm ... | iex`, calling `exit` would terminate the whole
# PowerShell window. All control flow below therefore lives inside a function
# and uses `return`, never `exit`.

# PowerShell 7.3+ turns a non-zero native exit code into a terminating error
# when $ErrorActionPreference is 'Stop'. We check exit codes ourselves (wsl,
# curl), so disable that behaviour where it exists.
if (Test-Path variable:PSNativeCommandUseErrorActionPreference) {
    $PSNativeCommandUseErrorActionPreference = $false
}

# Ask wsl.exe to emit UTF-8 instead of UTF-16; otherwise captured output is
# riddled with NUL bytes and string matching silently fails.
$env:WSL_UTF8 = '1'

# Windows PowerShell 5.1 may default to TLS 1.0/1.1, which GitHub rejects.
try { [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12 } catch {}

$Repo       = if ($env:CRABBY_REPO) { $env:CRABBY_REPO } else { 'marioolf/crabby' }
$InstallDir = Join-Path $env:LOCALAPPDATA 'Crabby'
$Binary     = 'crabby.exe'

function Write-Ok   ($m) { Write-Host "OK   $m" -ForegroundColor Green }
function Write-Warn ($m) { Write-Host "WARN $m" -ForegroundColor Yellow }
function Write-Err  ($m) { Write-Host "ERR  $m" -ForegroundColor Red }

# Download a URL to a file, tolerating corporate proxies and the redirect chain
# GitHub uses for release assets (github.com -> release-assets.githubusercontent.com).
#
# Strategy: prefer curl.exe (ships with Windows 10 1803+/11) because it follows
# redirects and handles TLS much like a browser. Fall back to Invoke-WebRequest
# with a browser-like User-Agent and the system proxy's default credentials.
function Get-CrabbyFile {
    param(
        [Parameter(Mandatory)] [string] $Url,
        [Parameter(Mandatory)] [string] $OutFile
    )

    $curl = Get-Command curl.exe -ErrorAction SilentlyContinue
    if ($curl) {
        & $curl.Source --fail --location --silent --show-error --retry 3 `
            --proto '=https' --tlsv1.2 --output $OutFile $Url 2>$null
        if ($LASTEXITCODE -eq 0 -and (Test-Path $OutFile)) { return }
    }

    # Fallback: Invoke-WebRequest, made as browser-like as possible.
    try {
        [System.Net.WebRequest]::DefaultWebProxy.Credentials = [System.Net.CredentialCache]::DefaultCredentials
    } catch {}

    Invoke-WebRequest -Uri $Url -OutFile $OutFile -UseBasicParsing `
        -MaximumRedirection 5 -ErrorAction Stop `
        -UserAgent 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) crabby-installer'
}

function Invoke-CrabbyInstall {
    Write-Host "Installing Crabby..." -ForegroundColor Cyan

    # --- Detect architecture -----------------------------------------------
    switch ($env:PROCESSOR_ARCHITECTURE) {
        'AMD64' { $arch = 'amd64' }
        'ARM64' { $arch = 'arm64' }
        default {
            Write-Err "Unsupported architecture: $($env:PROCESSOR_ARCHITECTURE)"
            return
        }
    }

    # --- Download and install crabby.exe -----------------------------------
    $asset = "crabby_windows_$arch.zip"
    $url   = "https://github.com/$Repo/releases/latest/download/$asset"
    $tmp   = Join-Path $env:TEMP ("crabby_" + [System.Guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $tmp -Force | Out-Null

    try {
        Write-Host "   Downloading $asset..."
        Get-CrabbyFile -Url $url -OutFile (Join-Path $tmp $asset)
        Expand-Archive -Path (Join-Path $tmp $asset) -DestinationPath $tmp -Force

        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
        Copy-Item -Path (Join-Path $tmp $Binary) -Destination (Join-Path $InstallDir $Binary) -Force
        Write-Ok "Installed crabby.exe to $InstallDir"
    }
    catch {
        $status = $null
        if ($_.Exception.Response) { $status = [int]$_.Exception.Response.StatusCode }
        Write-Err "Download or installation failed: $url"
        if ($status) { Write-Err "HTTP status: $status" }
        Write-Err $_.Exception.Message
        Write-Warn "If you are behind a corporate proxy, download the file in your browser and run:"
        Write-Warn "  Expand-Archive <downloaded.zip> `"$InstallDir`" -Force"
        return
    }
    finally {
        Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
    }

    # --- Ensure it is on PATH ----------------------------------------------
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if ($userPath -notlike "*$InstallDir*") {
        [Environment]::SetEnvironmentVariable('Path', "$userPath;$InstallDir", 'User')
        $env:Path = "$env:Path;$InstallDir"
        Write-Ok "Added $InstallDir to your user PATH (restart your terminal to pick it up)"
    }
    else {
        Write-Ok "$InstallDir is already on your PATH"
    }

    # --- Verify WSL --------------------------------------------------------
    Write-Host "Checking WSL..." -ForegroundColor Cyan
    if (-not (Get-Command wsl -ErrorAction SilentlyContinue)) {
        Write-Warn "WSL is not installed. Install it with:  wsl --install"
        Write-Warn "Then re-run this installer to set up the Linux side."
        return
    }
    Write-Ok "WSL is installed"

    # --- Verify an Ubuntu distribution -------------------------------------
    # Strip any stray NUL bytes in case an older WSL ignores WSL_UTF8.
    $distros = ((wsl -l -q) -join "`n") -replace "`0", ""
    if ($distros -notmatch 'Ubuntu') {
        Write-Warn "No Ubuntu distribution found. Install one with:  wsl --install -d Ubuntu"
        Write-Warn "Then re-run this installer to set up the Linux side."
        return
    }
    Write-Ok "Ubuntu distribution found"

    # --- Install / update the Linux binary inside WSL ----------------------
    # install.sh always fetches the latest release and overwrites in place, so
    # run it unconditionally. Skipping when crabby was already present is what
    # used to leave existing installs stuck on the old version.
    Write-Host "Setting up the Linux crabby binary inside WSL..." -ForegroundColor Cyan
    wsl bash -lc "curl -fsSL https://raw.githubusercontent.com/$Repo/main/install.sh | bash"
    if ($LASTEXITCODE -eq 0) {
        Write-Ok "Linux crabby is installed and up to date in WSL"
    }
    else {
        Write-Warn "Could not auto-install inside WSL. Run this inside WSL:"
        Write-Warn "  curl -fsSL https://raw.githubusercontent.com/$Repo/main/install.sh | bash"
    }

    Write-Host ""
    Write-Ok "Done! Open a new terminal and run:  crabby doctor"
    Write-Host "   Crabby · by marioolf · github.com/marioolf/crabby" -ForegroundColor DarkGray
}

Invoke-CrabbyInstall
