$ErrorActionPreference = "Stop"
go test ./internal/... ./tests/...
if ($LASTEXITCODE -ne 0) { throw "Go core tests failed" }
Write-Host "Core tests passed." -ForegroundColor Green
