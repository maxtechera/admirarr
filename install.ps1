# ─── Admirarr Installer (Windows) ─────────────────────────────────────
# Usage: irm https://get.admirarr.dev/windows | iex
#
# Options:
#   $env:ADMIRARR_INSTALL_DIR  — install location (default: ~\.local\bin)
#   $env:ADMIRARR_VERSION      — pin to a specific version (default: latest)
# ──────────────────────────────────────────────────────────────────────

$ErrorActionPreference = "Stop"

$Repo = "maxtechera/admirarr"
$Binary = "admirarr.exe"

Write-Host ""
Write-Host "  ⚓ " -ForegroundColor DarkYellow -NoNewline
Write-Host "ADMIRARR" -NoNewline -ForegroundColor White
Write-Host " installer" -ForegroundColor DarkGray
Write-Host "  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor DarkGray
Write-Host ""

# ── Detect architecture ──

$Arch = if ([Environment]::Is64BitOperatingSystem) {
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
} else {
    Write-Host "  ✗ 32-bit systems are not supported" -ForegroundColor Red
    exit 1
}

Write-Host "  → Platform: windows/$Arch" -ForegroundColor DarkGray

# ── Check Docker ──

$docker = Get-Command docker -ErrorAction SilentlyContinue
if ($docker) {
    $dv = & docker --version 2>$null
    Write-Host "  ✓ Docker found: $dv" -ForegroundColor Green
} else {
    Write-Host "  ! Docker not found — required for 'admirarr setup'" -ForegroundColor DarkYellow
    Write-Host "    Install: https://docs.docker.com/desktop/install/windows/" -ForegroundColor DarkGray
}

# ── Determine version ──

if ($env:ADMIRARR_VERSION) {
    $Version = $env:ADMIRARR_VERSION
    Write-Host "  → Version: $Version (pinned)" -ForegroundColor DarkGray
} else {
    Write-Host "  → Fetching latest release..." -ForegroundColor DarkGray
    try {
        $release = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
        $Version = $release.tag_name
    } catch {
        Write-Host "  ✗ Cannot reach GitHub API" -ForegroundColor Red
        exit 1
    }
    Write-Host "  → Version: $Version" -ForegroundColor DarkGray
}

$VersionNum = $Version.TrimStart("v")

# ── Download ──

$Filename = "admirarr_${VersionNum}_windows_${Arch}.zip"
$Url = "https://github.com/$Repo/releases/download/$Version/$Filename"

$TmpDir = Join-Path ([System.IO.Path]::GetTempPath()) "admirarr-install"
if (Test-Path $TmpDir) { Remove-Item $TmpDir -Recurse -Force }
New-Item -ItemType Directory -Path $TmpDir | Out-Null

Write-Host "  → Downloading $Filename..." -ForegroundColor DarkGray
try {
    Invoke-WebRequest -Uri $Url -OutFile (Join-Path $TmpDir $Filename) -UseBasicParsing
} catch {
    Write-Host "  ✗ Download failed: $Url" -ForegroundColor Red
    exit 1
}

# ── Extract ──

Write-Host "  → Extracting..." -ForegroundColor DarkGray
Expand-Archive -Path (Join-Path $TmpDir $Filename) -DestinationPath $TmpDir -Force

# Find binary
$Found = Get-ChildItem -Path $TmpDir -Filter $Binary -Recurse | Select-Object -First 1
if (-not $Found) {
    Write-Host "  ✗ Binary not found in archive" -ForegroundColor Red
    exit 1
}

# ── Install ──

if ($env:ADMIRARR_INSTALL_DIR) {
    $InstallDir = $env:ADMIRARR_INSTALL_DIR
} else {
    $InstallDir = Join-Path $env:USERPROFILE ".local\bin"
}

if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$Target = Join-Path $InstallDir $Binary
Copy-Item $Found.FullName $Target -Force

Write-Host "  ✓ Installed to $Target" -ForegroundColor Green

# ── Verify ──

try {
    $ver = & $Target --version 2>&1 | Select-Object -First 1
    Write-Host "  ✓ $ver" -ForegroundColor Green
} catch {}

# ── PATH check ──

$UserPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($UserPath -notlike "*$InstallDir*") {
    Write-Host ""
    Write-Host "  ! $InstallDir is not in your PATH" -ForegroundColor DarkYellow
    Write-Host ""

    $addToPath = Read-Host "  Add to PATH now? (y/n)"
    if ($addToPath -eq "y") {
        [Environment]::SetEnvironmentVariable("PATH", "$UserPath;$InstallDir", "User")
        $env:PATH = "$env:PATH;$InstallDir"
        Write-Host "  ✓ Added to PATH (restart your terminal to take effect)" -ForegroundColor Green
    } else {
        Write-Host "  Add it manually:" -ForegroundColor DarkGray
        Write-Host "    `$env:PATH += `";$InstallDir`"" -ForegroundColor DarkGray
    }
}

# ── Cleanup ──

Remove-Item $TmpDir -Recurse -Force -ErrorAction SilentlyContinue

# ── Done ──

Write-Host ""
Write-Host "  ⚓ " -ForegroundColor DarkYellow -NoNewline
Write-Host "Ready." -ForegroundColor White -NoNewline
Write-Host " Run " -NoNewline
Write-Host "admirarr setup" -ForegroundColor DarkYellow -NoNewline
Write-Host " to deploy your stack."
Write-Host "  Or " -ForegroundColor DarkGray -NoNewline
Write-Host "admirarr doctor" -ForegroundColor DarkYellow -NoNewline
Write-Host " if you already have one running." -ForegroundColor DarkGray
Write-Host ""
