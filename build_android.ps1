# Builds the Quest / Android APK.
#
# The build applies a patch to Fyne's Android input handling (see
# patches/fyne-<version>/android.go): without it, a Quest controller's pointer
# ray is read as a finger dragging across the screen, and the UI scrolls on its
# own and turns every click into a drag.
#
# Go will not patch files in the module cache in place, so this copies Fyne
# into .build/ (git-ignored), applies the patch there, and points Fyne at that
# copy for the length of the build. go.mod and go.sum are restored afterwards,
# so desktop builds are unaffected.
#
# Usage:  .\build_android.ps1 [-Ndk <path to NDK>] [-Sdk <path to Android SDK>]
# Defaults come from ANDROID_NDK_HOME and ANDROID_HOME.

param(
    [string]$Sdk = $env:ANDROID_HOME,
    [string]$Ndk = $env:ANDROID_NDK_HOME
)

$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

if (-not $Sdk) { throw "Set ANDROID_HOME or pass -Sdk <path to the Android SDK>." }
if (-not $Ndk) { throw "Set ANDROID_NDK_HOME or pass -Ndk <path to the NDK>." }

$fyneDir = (go list -m -f '{{.Dir}}' fyne.io/fyne/v2).Trim()
$fyneVer = (go list -m -f '{{.Version}}' fyne.io/fyne/v2).Trim()
$patchDir = Join-Path $PSScriptRoot "patches\fyne-$fyneVer"
if (-not (Test-Path $patchDir)) {
    throw "No Fyne input patch for $fyneVer. The patch in patches\ was written for a different Fyne version; port it before building, or Quest input will be broken."
}

# 1. A patched copy of Fyne, made once per version.
$build = Join-Path $PSScriptRoot '.build'
$fyneCopy = Join-Path $build "fyne-$fyneVer"
if (-not (Test-Path $fyneCopy)) {
    New-Item -ItemType Directory -Force $build | Out-Null
    Copy-Item -Recurse $fyneDir $fyneCopy
    # The module cache is read-only; the copy must not be.
    Get-ChildItem -Recurse -File $fyneCopy | ForEach-Object { $_.IsReadOnly = $false }
}
Copy-Item -Force (Join-Path $patchDir 'android.go') (Join-Path $fyneCopy 'internal\driver\mobile\app\android.go')

# 2. Point Fyne at the patched copy for the length of the build only. Fyne
#    runs Go commands outside this module too, which rules out -modfile, so
#    go.mod gets a replace directive that is always taken out again, even if
#    the build fails.
$modBackup = [IO.File]::ReadAllBytes((Join-Path $PSScriptRoot 'go.mod'))
$sumBackup = [IO.File]::ReadAllBytes((Join-Path $PSScriptRoot 'go.sum'))
$env:ANDROID_HOME = $Sdk
$env:ANDROID_NDK_HOME = $Ndk

$fyne = Join-Path (go env GOPATH) 'bin\fyne.exe'
if (-not (Test-Path $fyne)) { throw "fyne CLI not found; install it with: go install fyne.io/tools/cmd/fyne@latest" }

try {
    go mod edit "-replace=fyne.io/fyne/v2=$($fyneCopy -replace '\\', '/')"
    & $fyne package -os android/arm64 -app-id com.echotools.cosmeticeditor
    if ($LASTEXITCODE -ne 0) { throw "fyne package failed" }
}
finally {
    [IO.File]::WriteAllBytes((Join-Path $PSScriptRoot 'go.mod'), $modBackup)
    [IO.File]::WriteAllBytes((Join-Path $PSScriptRoot 'go.sum'), $sumBackup)
}
Write-Host "Built EchoVR_Cosmetics.apk with the Quest input patch (Fyne $fyneVer)."
