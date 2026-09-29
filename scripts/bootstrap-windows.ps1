$ErrorActionPreference = 'Stop'

Write-Host '== JTTY-Go Windows bootstrap ==' -ForegroundColor Cyan
Write-Host 'Go:' (go version)
Write-Host 'Node:' (node --version)
Write-Host 'npm:' (npm --version)

Write-Host "`n[1/4] Resolving Go modules..." -ForegroundColor Yellow
go mod download
go mod tidy

Write-Host "`n[2/4] Running core tests..." -ForegroundColor Yellow
go test ./internal/... ./tests/...

Write-Host "`n[3/4] Installing frontend dependencies..." -ForegroundColor Yellow
Push-Location frontend
npm install
npm run build
Pop-Location

Write-Host "`n[4/4] Checking Wails..." -ForegroundColor Yellow
wails doctor

Write-Host "`nBootstrap completed. Run: wails dev" -ForegroundColor Green
