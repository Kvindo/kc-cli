# Install the latest `kc` (Kvindo Cloud CLI) for Windows.
#   irm https://raw.githubusercontent.com/Kvindo/kc-cli/main/install.ps1 | iex
# Override the install directory with $env:KC_INSTALL_DIR (default: %LOCALAPPDATA%\kc).
$ErrorActionPreference = "Stop"
$repo = "Kvindo/kc-cli"

# ── detect architecture ────────────────────────────────────────────────────
$arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
$asset = "kc-windows-$arch.exe"
$url = "https://github.com/$repo/releases/latest/download/$asset"

# ── install directory ──────────────────────────────────────────────────────
$dir = if ($env:KC_INSTALL_DIR) { $env:KC_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "kc" }
New-Item -ItemType Directory -Force -Path $dir | Out-Null
$dest = Join-Path $dir "kc.exe"

# ── download ───────────────────────────────────────────────────────────────
Write-Host "Downloading $asset from the latest release..."
Invoke-WebRequest -Uri $url -OutFile $dest

# ── add to the user PATH if missing ────────────────────────────────────────
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$dir*") {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$dir", "User")
    Write-Host "Added $dir to your PATH — restart your terminal to use 'kc'."
}

Write-Host "Installed kc -> $dest"
& $dest version
