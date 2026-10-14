param([Parameter(Mandatory=$true)][string]$Destination)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$source = Join-Path $projectRoot 'bin'
Push-Location $projectRoot
try {
    go run ./cmd/runtimecheck -dir $source
    if ($LASTEXITCODE -ne 0) { throw 'Hamlib runtime verification failed' }
} finally { Pop-Location }
$manifest = Get-Content (Join-Path $source 'runtime-manifest.json') -Raw | ConvertFrom-Json
New-Item -ItemType Directory -Force -Path $Destination | Out-Null
foreach ($entry in $manifest.files.PSObject.Properties) {
    Copy-Item -LiteralPath (Join-Path $source $entry.Name) -Destination (Join-Path $Destination $entry.Name)
}
Copy-Item -LiteralPath (Join-Path $source 'runtime-manifest.json') -Destination $Destination
