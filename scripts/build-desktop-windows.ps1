param(
    [string]$Version = "dev-desktop",
    [switch]$Portable
)

$ErrorActionPreference = "Stop"
$projectRoot = Split-Path -Parent $PSScriptRoot
$sourceDir = Join-Path $projectRoot "combined_refactor"
$assetDir = Join-Path $projectRoot "release_assets"
$output = Join-Path $assetDir "cfdata-desktop-windows-amd64.exe"
$portableReadme = Join-Path $PSScriptRoot "CFData-Desktop-README.txt"

New-Item -ItemType Directory -Force -Path $assetDir | Out-Null
$env:GOWORK = "off"
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"

Push-Location $sourceDir
try {
    go build -trimpath -ldflags "-s -w -H=windowsgui -X main.appVersion=$Version -X main.desktopBuild=true" -o $output .
} finally {
    Pop-Location
}

Write-Host "Built $output"

if ($Portable) {
    if (-not (Test-Path -LiteralPath $portableReadme -PathType Leaf)) {
        throw "Portable package README is missing: $portableReadme"
    }
    $safeVersion = ($Version -replace '[^0-9A-Za-z._-]', '_').Trim('_')
    if ([string]::IsNullOrWhiteSpace($safeVersion)) {
        throw "Version cannot be converted to a portable package filename"
    }

    $portableName = "CFData-Desktop-$safeVersion-windows-amd64"
    $archivePath = Join-Path $assetDir "$portableName.zip"
    $stagingRoot = Join-Path $env:TEMP ("cfdata-portable-" + [guid]::NewGuid().ToString("N"))
    $stagingDir = Join-Path $stagingRoot "CFData-Desktop"
    New-Item -ItemType Directory -Force -Path $stagingDir | Out-Null
    try {
        Copy-Item -LiteralPath $output -Destination (Join-Path $stagingDir "CFData-Desktop.exe")
        Copy-Item -LiteralPath $portableReadme -Destination (Join-Path $stagingDir "README.txt")
        Compress-Archive -LiteralPath $stagingDir -DestinationPath $archivePath -Force
    } finally {
        if (Test-Path -LiteralPath $stagingRoot) {
            [System.IO.Directory]::Delete($stagingRoot, $true)
        }
    }
    $hash = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash
    Write-Host "Packaged $archivePath"
    Write-Host "SHA256 $hash"
}
