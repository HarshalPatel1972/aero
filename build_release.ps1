# AERO Release Build Script
# Creates an optimized, production-ready Windows executable
#
# Requirements:
#   - Go 1.21+
#   - Wails CLI (go install github.com/wailsapp/wails/v2/cmd/wails@latest)
#   - UPX (optional, for compression): https://upx.github.io/
#
# Usage:
#   .\build_release.ps1
#   .\build_release.ps1 -SkipUPX    # Skip UPX compression

param(
    [switch]$SkipUPX = $false,
    [switch]$Verbose = $false
)

$ErrorActionPreference = "Stop"

# ═══════════════════════════════════════════════════════════════
# Configuration
# ═══════════════════════════════════════════════════════════════

$ProjectName = "AERO"
$OutputDir = "build\bin"
$OutputFile = "$OutputDir\$ProjectName.exe"

# Build flags for production
# -s: Strip symbol table
# -w: Strip DWARF debugging info
# -H windowsgui: Hide console window on Windows
$LDFlags = "-s -w -H windowsgui"

# ═══════════════════════════════════════════════════════════════
# Helper Functions
# ═══════════════════════════════════════════════════════════════

function Write-Step {
    param([string]$Message)
    Write-Host "`n[AERO] " -ForegroundColor Cyan -NoNewline
    Write-Host $Message
}

function Write-Success {
    param([string]$Message)
    Write-Host "[✓] " -ForegroundColor Green -NoNewline
    Write-Host $Message
}

function Write-Warning {
    param([string]$Message)
    Write-Host "[!] " -ForegroundColor Yellow -NoNewline
    Write-Host $Message
}

function Write-Error {
    param([string]$Message)
    Write-Host "[✗] " -ForegroundColor Red -NoNewline
    Write-Host $Message
}

function Get-FileSize {
    param([string]$Path)
    if (Test-Path $Path) {
        $size = (Get-Item $Path).Length
        if ($size -gt 1MB) {
            return "{0:N2} MB" -f ($size / 1MB)
        } else {
            return "{0:N2} KB" -f ($size / 1KB)
        }
    }
    return "N/A"
}

# ═══════════════════════════════════════════════════════════════
# Pre-flight Checks
# ═══════════════════════════════════════════════════════════════

Write-Host ""
Write-Host "╔══════════════════════════════════════════════════════════╗" -ForegroundColor Cyan
Write-Host "║           AERO Release Build Script v1.0                 ║" -ForegroundColor Cyan
Write-Host "║           Production-Ready Binary Generator              ║" -ForegroundColor Cyan
Write-Host "╚══════════════════════════════════════════════════════════╝" -ForegroundColor Cyan

Write-Step "Checking prerequisites..."

# Check Go
try {
    $goVersion = (go version) 2>&1
    Write-Success "Go: $goVersion"
} catch {
    Write-Error "Go is not installed or not in PATH"
    exit 1
}

# Check Wails
try {
    $wailsVersion = (wails version) 2>&1
    Write-Success "Wails: $wailsVersion"
} catch {
    Write-Error "Wails CLI is not installed. Run: go install github.com/wailsapp/wails/v2/cmd/wails@latest"
    exit 1
}

# Check UPX (optional)
$upxAvailable = $false
if (-not $SkipUPX) {
    try {
        $upxVersion = (upx --version 2>&1 | Select-Object -First 1)
        Write-Success "UPX: $upxVersion"
        $upxAvailable = $true
    } catch {
        Write-Warning "UPX not found. Binary will not be compressed."
        Write-Warning "Install UPX from https://upx.github.io/ for smaller binaries."
    }
}

# ═══════════════════════════════════════════════════════════════
# Build Frontend
# ═══════════════════════════════════════════════════════════════

Write-Step "Building frontend..."

Push-Location frontend
try {
    npm run build 2>&1 | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "Frontend build failed"
    }
    Write-Success "Frontend built successfully"
} catch {
    Write-Error "Frontend build failed: $_"
    Pop-Location
    exit 1
}
Pop-Location

# ═══════════════════════════════════════════════════════════════
# Build Wails Application
# ═══════════════════════════════════════════════════════════════

Write-Step "Building Wails application..."
Write-Host "    Platform: windows/amd64"
Write-Host "    Flags: $LDFlags"

try {
    $buildOutput = wails build -platform windows/amd64 -ldflags "$LDFlags" 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "Wails build failed: $buildOutput"
    }
    Write-Success "Wails build completed"
} catch {
    Write-Error "Build failed: $_"
    exit 1
}

$sizeBeforeUPX = Get-FileSize $OutputFile
Write-Host "    Size: $sizeBeforeUPX"

# ═══════════════════════════════════════════════════════════════
# UPX Compression
# ═══════════════════════════════════════════════════════════════

if ($upxAvailable -and -not $SkipUPX) {
    Write-Step "Compressing with UPX..."
    
    try {
        $upxOutput = upx --best --lzma "$OutputFile" 2>&1
        if ($LASTEXITCODE -ne 0) {
            Write-Warning "UPX compression failed, continuing with uncompressed binary"
        } else {
            $sizeAfterUPX = Get-FileSize $OutputFile
            Write-Success "Compressed: $sizeBeforeUPX -> $sizeAfterUPX"
        }
    } catch {
        Write-Warning "UPX compression failed: $_"
    }
}

# ═══════════════════════════════════════════════════════════════
# Summary
# ═══════════════════════════════════════════════════════════════

Write-Host ""
Write-Host "╔══════════════════════════════════════════════════════════╗" -ForegroundColor Green
Write-Host "║                    BUILD COMPLETE                        ║" -ForegroundColor Green
Write-Host "╚══════════════════════════════════════════════════════════╝" -ForegroundColor Green
Write-Host ""
Write-Host "Output: " -NoNewline
Write-Host "$OutputFile" -ForegroundColor Yellow
Write-Host "Size:   " -NoNewline
Write-Host (Get-FileSize $OutputFile) -ForegroundColor Yellow
Write-Host ""

# Verify the binary exists
if (Test-Path $OutputFile) {
    Write-Host "Run with: " -NoNewline
    Write-Host ".\$OutputFile" -ForegroundColor Cyan
} else {
    Write-Error "Binary not found at expected location"
    exit 1
}

Write-Host ""
