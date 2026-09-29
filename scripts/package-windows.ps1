param(
    [ValidateSet('amd64')]
    [string]$Arch = 'amd64',
    [switch]$SkipTests,
    [switch]$SkipInstaller,
    [switch]$Clean
)

$ErrorActionPreference = 'Stop'

$ProjectRoot = Split-Path -Parent $PSScriptRoot
$BuildBin = Join-Path $ProjectRoot 'build\bin'
$HamlibBin = Join-Path $ProjectRoot 'bin'
$InstallerScript = Join-Path $ProjectRoot 'installer\JTTY-Go.iss'

Write-Host '== JTTY-Go Windows packaging ==' -ForegroundColor Cyan
Write-Host "Project: $ProjectRoot"
Write-Host "Architecture: $Arch"

if (-not (Get-Command wails -ErrorAction SilentlyContinue)) {
    throw 'Wails CLI not found. Install the project-pinned Wails v2 CLI first.'
}
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw 'Go not found.'
}
if (-not (Get-Command npm -ErrorAction SilentlyContinue)) {
    throw 'npm not found.'
}

$rigctld = Join-Path $HamlibBin 'rigctld.exe'
$rigctl = Join-Path $HamlibBin 'rigctl.exe'
$dlls = @(Get-ChildItem -Path $HamlibBin -Filter '*.dll' -File -ErrorAction SilentlyContinue)

if (-not (Test-Path $rigctld)) {
    throw "Missing $rigctld. Put the complete Windows Hamlib runtime into bin\ before packaging."
}
if (-not (Test-Path $rigctl)) {
    throw "Missing $rigctl. The packaged Hamlib tool set should contain rigctl.exe as well as rigctld.exe."
}
if ($dlls.Count -eq 0) {
    throw 'No Hamlib DLLs found in bin\. Refusing to create an incomplete package.'
}

$runtimeFiles = @(Get-ChildItem -Path $HamlibBin -Recurse -File | Where-Object {
    $_.Name -notmatch '^(README\.txt|.*\.md)$' -and $_.Name -notmatch '^\.'
})
if ($runtimeFiles.Count -lt 3) {
    throw 'Hamlib bin directory appears incomplete.'
}

if (-not $SkipTests) {
    Push-Location $ProjectRoot
    try {
        Write-Host '[1/5] Running tests...' -ForegroundColor Yellow
        go test ./internal/... ./tests/...
    }
    finally { Pop-Location }
}

Write-Host '[2/5] Installing frontend dependencies...' -ForegroundColor Yellow
Push-Location (Join-Path $ProjectRoot 'frontend')
try {
    npm install
    npm run build
}
finally { Pop-Location }

Write-Host '[3/5] Building Wails executable with embedded WebView2 bootstrapper and Hamlib...' -ForegroundColor Yellow
Push-Location $ProjectRoot
try {
    $buildArgs = @('-platform', "windows/$Arch", '-webview2', 'embed', '-trimpath')
    if ($Clean) { $buildArgs += '-clean' }
    & wails build @buildArgs
    if ($LASTEXITCODE -ne 0) { throw "wails build failed with exit code $LASTEXITCODE" }
}
finally { Pop-Location }

$exe = Join-Path $BuildBin 'JTTY-Go.exe'
if (-not (Test-Path $exe)) {
    throw "Build completed but executable was not found: $exe"
}

Write-Host '[4/5] Verifying embedded runtime source and release payload...' -ForegroundColor Yellow
Write-Host "Executable: $exe"
Write-Host ("Executable size: {0:N2} MB" -f ((Get-Item $exe).Length / 1MB))
Write-Host "Hamlib runtime files: $($runtimeFiles.Count)"

$releaseDir = Join-Path $ProjectRoot 'dist\windows'
New-Item -ItemType Directory -Force -Path $releaseDir | Out-Null
$portableDir = Join-Path $releaseDir 'JTTY-Go-portable'
if (Test-Path $portableDir) { Remove-Item -Recurse -Force $portableDir }
New-Item -ItemType Directory -Force -Path $portableDir | Out-Null
Copy-Item $exe (Join-Path $portableDir 'JTTY-Go.exe')
# The portable EXE already contains the complete embedded Hamlib runtime.
Copy-Item (Join-Path $ProjectRoot 'LICENSE') (Join-Path $portableDir 'LICENSE.txt')
if (Test-Path (Join-Path $ProjectRoot 'licenses\WSJT-X-GPL-3.0.txt')) {
    New-Item -ItemType Directory -Force -Path (Join-Path $portableDir 'licenses') | Out-Null
    Copy-Item (Join-Path $ProjectRoot 'licenses\WSJT-X-GPL-3.0.txt') (Join-Path $portableDir 'licenses\WSJT-X-GPL-3.0.txt')
}
if (Test-Path (Join-Path $ProjectRoot 'licenses\JTTY-PORT-NOTICE.md')) {
    Copy-Item (Join-Path $ProjectRoot 'licenses\JTTY-PORT-NOTICE.md') (Join-Path $portableDir 'licenses\JTTY-PORT-NOTICE.md')
}

$portableZip = Join-Path $releaseDir 'JTTY-Go-portable-windows.zip'
if (Test-Path $portableZip) { Remove-Item -Force $portableZip }
Compress-Archive -Path (Join-Path $portableDir '*') -DestinationPath $portableZip -CompressionLevel Optimal

if (-not $SkipInstaller) {
    if (-not (Test-Path $InstallerScript)) { throw "Installer script not found: $InstallerScript" }
    $iscc = @(
        (Get-Command ISCC.exe -ErrorAction SilentlyContinue).Source,
        (Join-Path ${env:ProgramFiles(x86)} 'Inno Setup 6\ISCC.exe'),
        (Join-Path ${env:ProgramFiles} 'Inno Setup 6\ISCC.exe')
    ) | Where-Object { $_ -and (Test-Path $_) } | Select-Object -First 1

    if (-not $iscc) {
        Write-Warning 'Inno Setup 6 was not found. Portable package was created, installer step skipped.'
    }
    else {
        Write-Host '[5/5] Building single-file Windows installer...' -ForegroundColor Yellow
        & $iscc "/DArch=$Arch" "/DSourceExe=$exe" $InstallerScript
        if ($LASTEXITCODE -ne 0) { throw "Inno Setup failed with exit code $LASTEXITCODE" }
    }
}
else {
    Write-Host '[5/5] Installer step skipped by parameter.' -ForegroundColor DarkYellow
}

Get-ChildItem $releaseDir -File | ForEach-Object {
    $hash = Get-FileHash $_.FullName -Algorithm SHA256
    "$($hash.Hash)  $($_.Name)" | Add-Content -Path (Join-Path $releaseDir 'SHA256SUMS.txt') -Encoding utf8
}

Write-Host "Packaging complete: $releaseDir" -ForegroundColor Green
