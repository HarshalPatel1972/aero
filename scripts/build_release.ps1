# ============================================================================
# AERO Production Release Build Script
# scripts/build_release.ps1
#
# Term-Phase 11: "Portable Perfection"
# One-click fabrication of the final distributable asset.
#
# Usage:
#   .\scripts\build_release.ps1
#   .\scripts\build_release.ps1 -SkipUPX    # Skip UPX compression
#   .\scripts\build_release.ps1 -Version "1.1.0"
#
# Requirements:
#   - Go 1.21+
#   - Wails CLI v2.x
#   - Node.js 18+
#   - (Optional) UPX for binary compression
# ============================================================================

param(
    [string]$Version = "1.0.0",
    [switch]$SkipUPX = $false,
    [switch]$SkipClean = $false
)

$ErrorActionPreference = "Stop"

# ============================================================================
# CONFIGURATION
# ============================================================================

$PROJECT_ROOT = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
if (-not $PROJECT_ROOT) { $PROJECT_ROOT = Get-Location }

$BUILD_DIR = Join-Path $PROJECT_ROOT "build\bin"
$OUTPUT_NAME = "Aero.exe"
$OUTPUT_PATH = Join-Path $BUILD_DIR $OUTPUT_NAME
$OUTPUT_NAME = "Aero.exe"
$OUTPUT_PATH = Join-Path $BUILD_DIR $OUTPUT_NAME
$INSTALLER_NAME = "Aero_Setup.exe"
$INSTALLER_PATH = Join-Path $BUILD_DIR $INSTALLER_NAME
$CHECKSUM_FILE = Join-Path $BUILD_DIR "checksum.sha256"

# Colors for output
$COLOR_INFO = "Cyan"
$COLOR_SUCCESS = "Green"
$COLOR_WARN = "Yellow"
$COLOR_ERROR = "Red"

# ============================================================================
# FUNCTIONS
# ============================================================================

function Write-Step {
    param([string]$Message)
    Write-Host "`n[$([char]0x2192)] $Message" -ForegroundColor $COLOR_INFO
}

function Write-Success {
    param([string]$Message)
    Write-Host "    $([char]0x2713) $Message" -ForegroundColor $COLOR_SUCCESS
}

function Write-Warn {
    param([string]$Message)
    Write-Host "    $([char]0x26A0) $Message" -ForegroundColor $COLOR_WARN
}

function Write-Fail {
    param([string]$Message)
    Write-Host "    $([char]0x2717) $Message" -ForegroundColor $COLOR_ERROR
}

function Get-FileSize {
    param([string]$Path)
    $size = (Get-Item $Path).Length
    if ($size -gt 1MB) {
        return "{0:N2} MB" -f ($size / 1MB)
    } elseif ($size -gt 1KB) {
        return "{0:N2} KB" -f ($size / 1KB)
    }
    return "$size B"
}

# ============================================================================
# PRE-FLIGHT CHECKS
# ============================================================================

Write-Host ""
Write-Host "============================================" -ForegroundColor $COLOR_INFO
Write-Host "  AERO Production Build v$Version" -ForegroundColor $COLOR_INFO
Write-Host "  $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')" -ForegroundColor DarkGray
Write-Host "============================================" -ForegroundColor $COLOR_INFO

Write-Step "Pre-flight checks..."

# Check Go
try {
    $goVersion = go version 2>&1
    Write-Success "Go: $goVersion"
} catch {
    Write-Fail "Go not found. Install from https://go.dev"
    exit 1
}

# Check Wails
try {
    $wailsVersion = wails version 2>&1 | Select-Object -First 1
    Write-Success "Wails: $wailsVersion"
} catch {
    Write-Fail "Wails CLI not found. Run: go install github.com/wailsapp/wails/v2/cmd/wails@latest"
    exit 1
}

# Check Node
try {
    $nodeVersion = node --version 2>&1
    Write-Success "Node.js: $nodeVersion"
} catch {
    Write-Fail "Node.js not found. Install from https://nodejs.org"
    exit 1
}

# Check UPX (optional)
$hasUPX = $false
if (-not $SkipUPX) {
    try {
        $upxVersion = upx --version 2>&1 | Select-Object -First 1
        Write-Success "UPX: $upxVersion"
        $hasUPX = $true
    } catch {
        Write-Warn "UPX not found. Binary compression will be skipped."
        Write-Warn "Install from: https://upx.github.io/"
    }
}

# Check NSIS (makensis)
$hasNSIS = $false
$makensisPath = "makensis"

try {
    $nsisVersion = & $makensisPath /VERSION 2>&1 | Select-Object -First 1
    if ($nsisVersion) {
        Write-Success "NSIS (PATH): $nsisVersion"
        $hasNSIS = $true
    }
} catch {
    # Try user-provided path
    $fallbackPath = "C:\Program Files (x86)\NSIS\makensis.exe"
    if (Test-Path $fallbackPath) {
        $makensisPath = $fallbackPath
        # Verify it works
        try {
            $nsisVersion = & $makensisPath /VERSION 2>&1 | Select-Object -First 1
            if ($nsisVersion) {
                Write-Success "NSIS (Local): $nsisVersion"
                $hasNSIS = $true
            }
        } catch {
             Write-Warn "Found NSIS at default path but execution failed."
        }
    } else {
        Write-Warn "NSIS (makensis) not found. Installer generation will be skipped."
    }
}

# ============================================================================
# STEP 1: CLEAN
# ============================================================================

if (-not $SkipClean) {
    Write-Step "Cleaning previous builds..."
    
    if (Test-Path $BUILD_DIR) {
        Remove-Item -Path $BUILD_DIR -Recurse -Force
        Write-Success "Removed: $BUILD_DIR"
    }
    
    # Clean Go cache for fresh build
    go clean -cache 2>&1 | Out-Null
    Write-Success "Go cache cleared"
}

# ============================================================================
# STEP 2: BUILD
# ============================================================================

Write-Step "Building production binary..."

Set-Location $PROJECT_ROOT

$buildArgs = @(
    "build",
    "-platform", "windows/amd64",
    "-clean"
)

# Add ldflags for version injection (simplified to avoid shell quoting issues)
$ldflags = "-s -w" 
$buildArgs += "-ldflags"
$buildArgs += $ldflags

Write-Host "    Command: wails $($buildArgs -join ' ')" -ForegroundColor DarkGray

# Run directly to stream output
& wails @buildArgs

if ($LASTEXITCODE -ne 0) {
    Write-Fail "Build failed!"
    exit 1
}

Write-Success "Build completed"

# Verify output exists
if (-not (Test-Path $OUTPUT_PATH)) {
    Write-Fail "Output binary not found: $OUTPUT_PATH"
    exit 1
}

$originalSize = Get-FileSize $OUTPUT_PATH
Write-Success "Binary size: $originalSize"

# ============================================================================
# STEP 3: COMPRESS (UPX)
# ============================================================================

if ($hasUPX -and -not $SkipUPX) {
    Write-Step "Compressing binary with UPX..."
    
    # NOTE: UPX is generally safe with Wails binaries, but test thoroughly
    # If issues occur, use --skip-upx flag
    
    $upxArgs = @(
        "--best",
        "--lzma",
        "-q",
        $OUTPUT_PATH
    )
    
    $upxResult = & upx @upxArgs 2>&1
    
    if ($LASTEXITCODE -eq 0) {
        $compressedSize = Get-FileSize $OUTPUT_PATH
        Write-Success "Compressed: $originalSize -> $compressedSize"
    } else {
        Write-Warn "UPX compression failed (binary still usable)"
        Write-Host $upxResult -ForegroundColor Yellow
    }
} else {
    Write-Warn "Skipping UPX compression"
}

# ============================================================================
# STEP 4: GENERATE INSTALLER (NSIS)
# ============================================================================

if ($hasNSIS) {
    Write-Step "Generating NSIS Installer..."
    
    $nsisScript = Join-Path $PROJECT_ROOT "build\windows\installer.nsi"
    if (-not (Test-Path $nsisScript)) {
        Write-Warn "NSIS script not found at: $nsisScript"
    } else {
        # Run makensis
        # /V2 = Verbosity level 2 (errors/warnings)
        $nsisArgs = @("/V2", $nsisScript)
        
        Write-Host "    Command: & `"$makensisPath`" $nsisScript" -ForegroundColor DarkGray
        $nsisResult = & $makensisPath @nsisArgs 2>&1
        
        if ($LASTEXITCODE -eq 0 -and (Test-Path $INSTALLER_PATH)) {
            $installerSize = Get-FileSize $INSTALLER_PATH
            Write-Success "Installer created: $INSTALLER_NAME ($installerSize)"
        } else {
            Write-Warn "Installer generation failed"
            Write-Host $nsisResult -ForegroundColor Yellow
        }
    }
} else {
    Write-Warn "Skipping Installer generation (NSIS missing)"
}

# ============================================================================
# STEP 5: GENERATE CHECKSUM
# ============================================================================

Write-Step "Generating SHA256 checksum..."

$hash = Get-FileHash -Path $OUTPUT_PATH -Algorithm SHA256
$hashString = $hash.Hash.ToLower()
$checksumContent = "$hashString  $OUTPUT_NAME"

if (Test-Path $INSTALLER_PATH) {
    $instHash = Get-FileHash -Path $INSTALLER_PATH -Algorithm SHA256
    $instHashString = $instHash.Hash.ToLower()
    $checksumContent += "`r`n$instHashString  $INSTALLER_NAME"
}

Set-Content -Path $CHECKSUM_FILE -Value $checksumContent -NoNewline
Write-Success "Checksum: $hashString"
Write-Success "Saved to: $CHECKSUM_FILE"

# ============================================================================
# SUMMARY
# ============================================================================

Write-Host ""
Write-Host "============================================" -ForegroundColor $COLOR_SUCCESS
Write-Host "  BUILD SUCCESSFUL!" -ForegroundColor $COLOR_SUCCESS
Write-Host "============================================" -ForegroundColor $COLOR_SUCCESS
Write-Host ""
Write-Host "  Output:   $OUTPUT_PATH" -ForegroundColor White
Write-Host "  Size:     $(Get-FileSize $OUTPUT_PATH)" -ForegroundColor White
Write-Host "  Version:  $Version" -ForegroundColor White
Write-Host "  Checksum: $hashString" -ForegroundColor DarkGray
Write-Host "  Installer: $INSTALLER_PATH" -ForegroundColor White
Write-Host "  Size:      $(if (Test-Path $INSTALLER_PATH) { Get-FileSize $INSTALLER_PATH } else { "N/A" })" -ForegroundColor White
Write-Host ""
Write-Host "  Ready for distribution!" -ForegroundColor $COLOR_SUCCESS
Write-Host ""

# Return to original directory
Set-Location $PROJECT_ROOT
