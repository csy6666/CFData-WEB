[CmdletBinding()]
param(
    [string]$Version = "dev",
    [ValidateSet("Debug", "Release")]
    [string]$BuildType = "Debug",
    [ValidateRange(1, 2147483647)]
    [int]$VersionCode = 1,
    [string]$SdkRoot,
    [string]$JavaHome
)

$ErrorActionPreference = "Stop"

$projectRoot = Split-Path -Parent $PSScriptRoot
$sourceDir = Join-Path $projectRoot "combined_refactor"
$nativeDir = Join-Path $projectRoot "app\src\main\jniLibs\arm64-v8a"
$nativeBackend = Join-Path $nativeDir "libcfdata.so"
$assetDir = Join-Path $projectRoot "release_assets"
$gradleWrapper = Join-Path $projectRoot "gradlew.bat"

if ([string]::IsNullOrWhiteSpace($SdkRoot)) {
    if ($env:ANDROID_SDK_ROOT) {
        $SdkRoot = $env:ANDROID_SDK_ROOT
    } elseif ($env:ANDROID_HOME) {
        $SdkRoot = $env:ANDROID_HOME
    } else {
        $SdkRoot = Join-Path $env:LOCALAPPDATA "Android\Sdk"
    }
}
if (-not (Test-Path -LiteralPath $SdkRoot -PathType Container)) {
    throw "Android SDK directory was not found: $SdkRoot"
}

if ([string]::IsNullOrWhiteSpace($JavaHome)) {
    if ($env:JAVA_HOME) {
        $JavaHome = $env:JAVA_HOME
    } else {
        $JavaHome = "C:\Program Files\Android\Android Studio\jbr"
    }
}
if (-not (Test-Path -LiteralPath (Join-Path $JavaHome "bin\java.exe") -PathType Leaf)) {
    throw "A supported JDK was not found: $JavaHome"
}
if (-not (Test-Path -LiteralPath $gradleWrapper -PathType Leaf)) {
    throw "Gradle Wrapper is missing: $gradleWrapper"
}

if ($BuildType -eq "Release") {
    $requiredSigningVariables = "ANDROID_KEYSTORE_FILE", "ANDROID_KEYSTORE_PASSWORD", "ANDROID_KEY_ALIAS", "ANDROID_KEY_PASSWORD"
    $missingSigningVariables = $requiredSigningVariables | Where-Object { [string]::IsNullOrWhiteSpace([Environment]::GetEnvironmentVariable($_)) }
    if ($missingSigningVariables) {
        throw "Release signing requires environment variables: $($missingSigningVariables -join ', ')"
    }
}

New-Item -ItemType Directory -Force -Path $nativeDir, $assetDir | Out-Null
$env:GOWORK = "off"
$env:GOOS = "android"
$env:GOARCH = "arm64"
$env:CGO_ENABLED = "0"
if (-not $env:GOPROXY) {
    $env:GOPROXY = "https://goproxy.cn,direct"
}

Push-Location $sourceDir
try {
    go build -trimpath -ldflags "-s -w -X main.appVersion=$Version" -o $nativeBackend .
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
} finally {
    Pop-Location
}

$env:JAVA_HOME = $JavaHome
$env:ANDROID_SDK_ROOT = $SdkRoot
$env:ANDROID_HOME = $SdkRoot
$env:ANDROID_VERSION_NAME = $Version
$env:ANDROID_VERSION_CODE = $VersionCode

Push-Location $projectRoot
try {
    & $gradleWrapper ":app:assemble$BuildType" "--no-daemon"
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
} finally {
    Pop-Location
}

$variant = $BuildType.ToLowerInvariant()
$apkPath = Join-Path $projectRoot "app\build\outputs\apk\$variant\app-$variant.apk"
if (-not (Test-Path -LiteralPath $apkPath -PathType Leaf)) {
    throw "Gradle did not create the expected APK: $apkPath"
}

$safeVersion = ($Version -replace '[^0-9A-Za-z._-]', '_').Trim('_')
if ([string]::IsNullOrWhiteSpace($safeVersion)) {
    throw "Version cannot be converted to an APK filename"
}
$assetPath = Join-Path $assetDir "CFData-Android-$safeVersion-arm64-v8a-$variant.apk"
Copy-Item -LiteralPath $apkPath -Destination $assetPath -Force

Write-Host "Built $nativeBackend"
Write-Host "Packaged $assetPath"
Write-Host "SHA256 $((Get-FileHash -LiteralPath $assetPath -Algorithm SHA256).Hash)"
