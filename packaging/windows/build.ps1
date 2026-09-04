# Builds a deployable WhatsAppGo folder, a portable ZIP and, when Inno Setup is
# installed, an installer.
#
# Run on Windows with Qt 6.5 or newer, the Go toolchain and a C++ compiler.
# Point -QtDir at the Qt installation if windeployqt is not on PATH, e.g.
#   powershell -File packaging\windows\build.ps1 -QtDir C:\Qt\6.8.2\msvc2022_64
#
# Signing is skipped unless -SignThumbprint names a certificate in the
# certificate store. Unsigned builds run, but SmartScreen warns about them.
[CmdletBinding()]
param(
    [string]$QtDir = "",
    [string]$SignThumbprint = "",
    [string]$Configuration = "Release"
)

$ErrorActionPreference = "Stop"
$projectRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$buildDir = Join-Path $projectRoot "desktop\build-windows"
$stageDir = Join-Path $projectRoot "build\windows"

Push-Location $projectRoot
try {
    New-Item -ItemType Directory -Force -Path (Join-Path $projectRoot "bin"), $stageDir | Out-Null

    # The daemon is pure Go: no cgo, no C toolchain, no SQLite DLL to ship.
    $env:CGO_ENABLED = "0"
    & go build -trimpath -ldflags "-s -w" -o (Join-Path $projectRoot "bin\whatsappd.exe") ./cmd/whatsappd
    if ($LASTEXITCODE -ne 0) { throw "building whatsappd failed" }
    & go build -trimpath -ldflags "-s -w" -o (Join-Path $projectRoot "bin\whatsappctl.exe") ./cmd/whatsappctl
    if ($LASTEXITCODE -ne 0) { throw "building whatsappctl failed" }

    $cmakeArgs = @("-S", (Join-Path $projectRoot "desktop"), "-B", $buildDir,
                   "-DCMAKE_BUILD_TYPE=$Configuration",
                   "-DCMAKE_INSTALL_PREFIX=$stageDir",
                   "-DBUILD_TESTING=OFF")
    if ($QtDir -ne "") { $cmakeArgs += "-DCMAKE_PREFIX_PATH=$QtDir" }
    & cmake @cmakeArgs
    if ($LASTEXITCODE -ne 0) { throw "cmake configure failed" }
    & cmake --build $buildDir --config $Configuration --parallel
    if ($LASTEXITCODE -ne 0) { throw "cmake build failed" }

    if (Test-Path $stageDir) { Remove-Item -Recurse -Force $stageDir }
    & cmake --install $buildDir --config $Configuration
    if ($LASTEXITCODE -ne 0) { throw "cmake install failed" }

    # windeployqt copies the Qt DLLs, the platform plugin and the QML modules
    # the application imports. Without --qmldir it ships no QML plugins and the
    # window comes up empty.
    $windeployqt = if ($QtDir -ne "") { Join-Path $QtDir "bin\windeployqt.exe" } else { "windeployqt" }
    & $windeployqt --qmldir (Join-Path $projectRoot "desktop\qml") --release `
                   (Join-Path $stageDir "whatsappgo.exe")
    if ($LASTEXITCODE -ne 0) { throw "windeployqt failed" }

    if ($SignThumbprint -ne "") {
        # Every executable is signed, not only the launcher: the daemon is a
        # separate process and SmartScreen judges it on its own.
        Get-ChildItem -Path $stageDir -Filter *.exe -Recurse | ForEach-Object {
            & signtool sign /sha1 $SignThumbprint /fd SHA256 /tr http://timestamp.digicert.com /td SHA256 $_.FullName
            if ($LASTEXITCODE -ne 0) { throw "signing $($_.Name) failed" }
        }
    } else {
        Write-Warning "SignThumbprint was not given; the build is unsigned and SmartScreen will warn about it."
    }

    $zip = Join-Path $projectRoot "build\WhatsAppGo-windows-x64.zip"
    if (Test-Path $zip) { Remove-Item -Force $zip }
    Compress-Archive -Path (Join-Path $stageDir "*") -DestinationPath $zip
    Write-Host "Built $zip"

    $iscc = Get-Command iscc.exe -ErrorAction SilentlyContinue
    if ($null -ne $iscc) {
        & $iscc.Source (Join-Path $PSScriptRoot "whatsappgo.iss") "/DStageDir=$stageDir" "/DOutputDir=$(Join-Path $projectRoot 'build')"
        if ($LASTEXITCODE -ne 0) { throw "Inno Setup failed" }
        Write-Host "Built the installer in $(Join-Path $projectRoot 'build')"
    } else {
        Write-Warning "Inno Setup (iscc.exe) was not found; only the portable ZIP was produced."
    }
}
finally {
    Pop-Location
}
