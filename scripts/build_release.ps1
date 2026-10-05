# ============================================================================
# AERO Production Release Build Script
#
# Usage:
#   .\scripts\build_release.ps1
#   .\scripts\build_release.ps1 -UseUPX     # opt-in UPX compression (see note below)
#
# Code signing (strongly recommended for public releases - unsigned apps trigger
# Windows SmartScreen warnings). Provide a code-signing certificate either as a
# PFX file or by thumbprint from the Windows certificate store:
#   $env:AERO_SIGN_PFX = "C:\certs\aero.pfx"; $env:AERO_SIGN_PASSWORD = "..."
#   or
#   $env:AERO_SIGN_THUMBPRINT = "ABCDEF..."
# signtool.exe comes with the Windows SDK.
#
# The version comes from wails.json (info.productVersion) - the single source
# of truth. Bump it there before releasing.
#
# Requirements: Go, Node.js, Wails CLI v2, NSIS (for the installer)
# ============================================================================

param(
    [switch]$UseUPX = $false
)

$ErrorActionPreference = "Stop"

$PROJECT_ROOT = Split-Path -Parent $PSScriptRoot
$BUILD_DIR = Join-Path $PROJECT_ROOT "build\bin"
$EXE_PATH = Join-Path $BUILD_DIR "Aero.exe"
$WAILS_INSTALLER = Join-Path $BUILD_DIR "Aero-amd64-installer.exe"
$INSTALLER_PATH = Join-Path $BUILD_DIR "Aero_Setup.exe"
$CHECKSUM_FILE = Join-Path $BUILD_DIR "checksum.sha256"
$NSIS_SCRIPT = Join-Path $PROJECT_ROOT "build\windows\installer\project.nsi"

function Write-Step { param([string]$m) Write-Host "`n[->] $m" -ForegroundColor Cyan }
function Write-Ok   { param([string]$m) Write-Host "    OK  $m" -ForegroundColor Green }
function Write-Warn { param([string]$m) Write-Host "    !!  $m" -ForegroundColor Yellow }
function Fail       { param([string]$m) Write-Host "    XX  $m" -ForegroundColor Red; exit 1 }

function Find-Tool {
    param([string]$Name, [string[]]$Fallbacks = @())
    $cmd = Get-Command $Name -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    foreach ($p in $Fallbacks) {
        $hit = Get-ChildItem -Path $p -ErrorAction SilentlyContinue | Sort-Object FullName -Descending | Select-Object -First 1
        if ($hit) { return $hit.FullName }
    }
    return $null
}

Set-Location $PROJECT_ROOT
$Version = (Get-Content (Join-Path $PROJECT_ROOT "wails.json") -Raw | ConvertFrom-Json).info.productVersion

Write-Host "`n============================================" -ForegroundColor Cyan
Write-Host "  AERO Production Build v$Version" -ForegroundColor Cyan
Write-Host "============================================" -ForegroundColor Cyan

# ---------------------------------------------------------------------------
Write-Step "Pre-flight checks"
foreach ($tool in @("go", "node", "npm", "wails")) {
    if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) { Fail "$tool not found on PATH" }
}
Write-Ok "Go, Node.js and Wails found"

$makensis = Find-Tool "makensis" @("C:\Program Files (x86)\NSIS\makensis.exe", "C:\Program Files\NSIS\makensis.exe")
if ($makensis) { Write-Ok "NSIS: $makensis" } else { Write-Warn "NSIS not found - the installer will be skipped (https://nsis.sourceforge.io)" }

$signtool = Find-Tool "signtool" @("C:\Program Files (x86)\Windows Kits\10\bin\*\x64\signtool.exe")
$signArgs = $null
if ($env:AERO_SIGN_PFX) {
    $signArgs = @("/f", $env:AERO_SIGN_PFX, "/p", $env:AERO_SIGN_PASSWORD)
} elseif ($env:AERO_SIGN_THUMBPRINT) {
    $signArgs = @("/sha1", $env:AERO_SIGN_THUMBPRINT)
}
if ($signArgs -and -not $signtool) { Fail "Signing requested but signtool.exe was not found (install the Windows SDK)" }
if ($signArgs) { Write-Ok "Code signing enabled" } else { Write-Warn "Code signing NOT configured - users will see SmartScreen warnings" }

function Sign-File {
    param([string]$Path)
    if (-not $signArgs) { return }
    & $signtool sign @signArgs /fd SHA256 /tr "http://timestamp.digicert.com" /td SHA256 /d "Aero" $Path
    if ($LASTEXITCODE -ne 0) { Fail "Signing failed for $Path" }
    Write-Ok "Signed $(Split-Path -Leaf $Path)"
}

# ---------------------------------------------------------------------------
Write-Step "Running tests"
go test ./internal/security/ ./internal/transfer/
if ($LASTEXITCODE -ne 0) { Fail "Tests failed" }
Write-Ok "Tests passed"

# ---------------------------------------------------------------------------
Write-Step "Building frontend"
# The Go binary embeds frontend/dist, which does not exist in a fresh clone.
Push-Location (Join-Path $PROJECT_ROOT "frontend")
npm ci
if ($LASTEXITCODE -ne 0) { Pop-Location; Fail "npm ci failed" }
npm run build
if ($LASTEXITCODE -ne 0) { Pop-Location; Fail "Frontend build failed" }
Pop-Location
Write-Ok "Frontend built"

# ---------------------------------------------------------------------------
Write-Step "Applying app icon"
# build/ is generated and not committed; without this Wails falls back to its
# default icon. The committed source of truth is assets/.
New-Item -ItemType Directory -Force (Join-Path $PROJECT_ROOT "build\windows") | Out-Null
Copy-Item -Force (Join-Path $PROJECT_ROOT "assets\appicon.png") (Join-Path $PROJECT_ROOT "build\appicon.png")
Copy-Item -Force (Join-Path $PROJECT_ROOT "assets\icon.ico") (Join-Path $PROJECT_ROOT "build\windows\icon.ico")
Write-Ok "Icon copied from assets/"


# ---------------------------------------------------------------------------
Write-Step "Building application"
$buildArgs = @("build", "-clean", "-platform", "windows/amd64", "-ldflags", "-s -w", "-trimpath")
if ($makensis) { $buildArgs += "-nsis" }
& wails @buildArgs
if ($LASTEXITCODE -ne 0) { Fail "Wails build failed" }
if (-not (Test-Path $EXE_PATH)) { Fail "Output binary not found: $EXE_PATH" }
Write-Ok "Built $EXE_PATH"

# UPX-packed executables are frequently flagged by antivirus software, which
# is a real problem for a public release. Only use it if you have tested that.
if ($UseUPX) {
    if (-not (Get-Command upx -ErrorAction SilentlyContinue)) { Fail "-UseUPX given but upx not found" }
    upx --best --lzma -q $EXE_PATH
    if ($LASTEXITCODE -ne 0) { Fail "UPX failed" }
    Write-Ok "Compressed with UPX"
}

Sign-File $EXE_PATH

# ---------------------------------------------------------------------------
if ($makensis) {
    Write-Step "Packaging installer"
    # Repackage so the installer contains the final (compressed/signed) exe.
    & $makensis "-DARG_WAILS_AMD64_BINARY=$EXE_PATH" $NSIS_SCRIPT
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path $WAILS_INSTALLER)) { Fail "Installer packaging failed" }
    Move-Item -Force $WAILS_INSTALLER $INSTALLER_PATH
    Sign-File $INSTALLER_PATH
    Write-Ok "Installer: $INSTALLER_PATH"
}

# ---------------------------------------------------------------------------
Write-Step "Generating SHA256 checksums"
$lines = @()
foreach ($f in @($EXE_PATH, $INSTALLER_PATH)) {
    if (Test-Path $f) {
        $lines += "$((Get-FileHash -Path $f -Algorithm SHA256).Hash.ToLower())  $(Split-Path -Leaf $f)"
    }
}
Set-Content -Path $CHECKSUM_FILE -Value ($lines -join "`r`n") -NoNewline
Write-Ok "Saved $CHECKSUM_FILE"

Write-Host "`n  BUILD SUCCESSFUL - Aero v$Version" -ForegroundColor Green
Write-Host "  $BUILD_DIR`n"
