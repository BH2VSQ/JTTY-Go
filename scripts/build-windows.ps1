$ErrorActionPreference = "Stop"

Write-Host "== JTTY-Go Windows build ==" -ForegroundColor Cyan

go version
node --version
npm --version

if (-not (Get-Command wails -ErrorAction SilentlyContinue)) {
  throw "Wails CLI not found. Install Wails v2 first."
}

wails doctor
wails build -platform windows/amd64 -clean

Write-Host "Build completed. Check build/bin for JTTY-Go.exe" -ForegroundColor Green
