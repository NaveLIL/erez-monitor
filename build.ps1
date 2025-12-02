# EREZMonitor Build Script for Windows
# Run with: .\build.ps1

param(
    [switch]$Release,
    [switch]$Clean,
    [switch]$Run,
    [switch]$Test,
    [switch]$Compress
)

$ErrorActionPreference = "Stop"

$AppName = "EREZMonitor"
$OutputDir = "build"
$OutputExe = "$OutputDir\$AppName.exe"

# Colors for output
function Write-Info { Write-Host $args -ForegroundColor Cyan }
function Write-Success { Write-Host $args -ForegroundColor Green }
function Write-Warning { Write-Host $args -ForegroundColor Yellow }

Write-Info "=========================================="
Write-Info "  $AppName Build Script"
Write-Info "=========================================="

# Clean build directory
if ($Clean) {
    Write-Info "Cleaning build directory..."
    if (Test-Path $OutputDir) {
        Remove-Item -Recurse -Force $OutputDir
    }
    Write-Success "Clean complete!"
    if (-not $Release -and -not $Run -and -not $Test) {
        exit 0
    }
}

# Create output directory
if (-not (Test-Path $OutputDir)) {
    New-Item -ItemType Directory -Path $OutputDir | Out-Null
}

# Run tests
if ($Test) {
    Write-Info "Running tests..."
    go test -v -race ./...
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Tests failed!"
        exit 1
    }
    Write-Success "All tests passed!"
    if (-not $Release -and -not $Run) {
        exit 0
    }
}

# Download dependencies
Write-Info "Downloading dependencies..."
go mod tidy
go mod download

# Get version from git tag or default
$Version = "1.0.0"
try {
    $GitTag = git describe --tags --abbrev=0 2>$null
    if ($GitTag) {
        $Version = $GitTag.TrimStart('v')
    }
} catch {
    # Ignore git errors
}

$BuildTime = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
$GitCommit = "unknown"
try {
    $GitCommit = git rev-parse --short HEAD 2>$null
} catch {
    # Ignore git errors
}

Write-Info "Building $AppName v$Version..."

# Build flags
$LDFlags = "-s -w"
$LDFlags += " -X 'main.Version=$Version'"
$LDFlags += " -X 'main.BuildTime=$BuildTime'"
$LDFlags += " -X 'main.GitCommit=$GitCommit'"

if ($Release) {
    Write-Info "Building release version (no console window)..."
    $LDFlags += " -H=windowsgui"
    
    # Check if go-winres is available for icon embedding
    $GoWinres = Get-Command go-winres -ErrorAction SilentlyContinue
    if ($GoWinres) {
        Write-Info "Embedding Windows resources..."
        
        # Create winres.json if it doesn't exist
        if (-not (Test-Path "winres.json")) {
            @{
                "RT_GROUP_ICON" = @{
                    "APP" = "assets/icon.ico"
                }
                "RT_MANIFEST" = @{
                    "#1" = @{
                        "identity" = @{
                            "name" = $AppName
                            "version" = $Version
                        }
                        "description" = "System Resource Monitor"
                        "minimum-os" = "win7"
                        "execution-level" = "asInvoker"
                        "dpi-awareness" = "per-monitor-v2"
                        "ui-access" = $false
                    }
                }
            } | ConvertTo-Json -Depth 10 | Out-File -Encoding utf8 "winres.json"
        }
        
        try {
            go-winres make --in winres.json --product-version="$Version.0" --file-version="$Version.0"
        } catch {
            Write-Warning "Failed to embed resources: $_"
        }
    } else {
        Write-Warning "go-winres not found. Install with: go install github.com/tc-hib/go-winres@latest"
    }
}

# Build the application
$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = "amd64"

go build -ldflags="$LDFlags" -o $OutputExe .

if ($LASTEXITCODE -ne 0) {
    Write-Error "Build failed!"
    exit 1
}

# Get file size
$FileSize = (Get-Item $OutputExe).Length
$FileSizeMB = [math]::Round($FileSize / 1MB, 2)

Write-Success "Build complete: $OutputExe ($FileSizeMB MB)"

# Compress with UPX if available and requested
if ($Compress -and $Release) {
    $Upx = Get-Command upx -ErrorAction SilentlyContinue
    if ($Upx) {
        Write-Info "Compressing with UPX..."
        upx --best --lzma $OutputExe
        
        $CompressedSize = (Get-Item $OutputExe).Length
        $CompressedSizeMB = [math]::Round($CompressedSize / 1MB, 2)
        $Ratio = [math]::Round(($CompressedSize / $FileSize) * 100, 1)
        
        Write-Success "Compressed: $CompressedSizeMB MB ($Ratio% of original)"
    } else {
        Write-Warning "UPX not found. Install from: https://upx.github.io/"
    }
}

# Copy default config
if (-not (Test-Path "$OutputDir\config.yaml")) {
    Copy-Item "config\config.yaml" "$OutputDir\config.yaml"
    Write-Info "Copied default config to $OutputDir\config.yaml"
}

# Run if requested
if ($Run) {
    Write-Info "Starting $AppName..."
    Start-Process -FilePath $OutputExe -ArgumentList "--debug"
}

Write-Info "=========================================="
Write-Success "  Build completed successfully!"
Write-Info "=========================================="
Write-Host ""
Write-Host "To run: .\$OutputExe"
Write-Host "To run in debug mode: .\$OutputExe --debug"
Write-Host "To run minimized: .\$OutputExe --tray-only"
