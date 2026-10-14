param(
    [Parameter(Mandatory=$true)][string]$SourceDir,
    [Parameter(Mandatory=$true)][string]$BuildDir,
    [string]$FortranCompiler = 'gfortran'
)
$ErrorActionPreference = 'Stop'
$SourceDir = (Resolve-Path -LiteralPath $SourceDir).Path
$harness = Join-Path (Split-Path -Parent $PSScriptRoot) 'tests\oracle\jtty_oracle.f90'
$reference = Join-Path (Split-Path -Parent $PSScriptRoot) 'reference\wsjtx-jtty'
foreach ($core in @('lib/jtty/jtty_mod.f90','lib/jtty/jtty_source_codec.f90','lib/jtty/jtty_mdecode.f90','lib/jtty/jtty_tbcc_code_profile.f90','lib/jtty/gen_jttywave.f90')) {
    $actual = (Get-FileHash -LiteralPath (Join-Path $SourceDir $core) -Algorithm SHA256).Hash
    $expected = (Get-FileHash -LiteralPath (Join-Path $reference $core) -Algorithm SHA256).Hash
    if ($actual -ne $expected) { throw "WSJT-X source does not match pinned reference: $core" }
}
New-Item -ItemType Directory -Force -Path $BuildDir | Out-Null
$sources = @(
    'lib/77bit/packjt77_schema.f90','lib/77bit/packjt77_grammar.f90','lib/77bit/packjt77.f90',
    'lib/jtty/jtty_source_codec.f90','lib/jtty/jtty_mod.f90',
    'lib/jtty/jtty_tbcc_code_profile.f90','lib/jtty/tbcc.f90','lib/jtty/jtty_fec_mod.f90',
    'lib/jtty/jtty_tbcc_list_decoder.f90','lib/jtty/jtty_tbcc_decoder.f90',
    'lib/jtty/jtty_payload_correlators.f90','lib/fftw3mod.f90','lib/four2a.f90',
    'lib/jtty/ana64a.f90','lib/jtty/gen_syncwave.f90','lib/jtty/genjtty.f90',
    'lib/gfsk_pulse.f90','lib/jtty/gen_jttywave.f90','lib/jtty/jtty_decode.f90',
    'lib/twkfreq.f90','lib/db.f90','lib/chkcall.f90','lib/fmtmsg.f90',
    'lib/jtty/jtty_peakup.f90','lib/jtty/subtract_jtty.f90','lib/jtty/jtty_mdecode.f90','lib/jtty/rjtty_sub.f90'
) | ForEach-Object { Join-Path $SourceDir $_ }
Push-Location $BuildDir
try {
    & $FortranCompiler -O2 -ffree-line-length-none -fallow-argument-mismatch -ffunction-sections -fdata-sections "-I$SourceDir/lib" @sources $harness '-Wl,--gc-sections' -lfftw3f -lfftw3f_threads -o jtty-oracle.exe
    if ($LASTEXITCODE -ne 0) { throw 'Independent WSJT-X oracle build failed' }
} finally { Pop-Location }
