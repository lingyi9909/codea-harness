$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

# Keep the accepted 1.6.4 builder/installer unchanged. Adapt only the release
# version; all package cleanup, binary pins and the three retained Host resource paths stay shared.
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$source = Join-Path $PSScriptRoot 'task164-release-package.ps1'
$expectedBlob = '8d8539c8967afb3f8b5f2285d0afd7ed41f63bcc'
$actualBlob = (& git -C $repoRoot hash-object -- $source).Trim()
if ($LASTEXITCODE -ne 0 -or $actualBlob -ne $expectedBlob) { throw "Retained release builder changed: $actualBlob" }
$version = (Get-Content (Join-Path $repoRoot '.code-harness/VERSION') -Raw).Trim()
if ($version -ne '1.6.6') { throw "Expected 1.6.6, got $version" }
$adapted = (Get-Content $source -Raw).Replace('1.6.4', '1.6.6')
$temp = Join-Path $PSScriptRoot ('.release166-package-' + [guid]::NewGuid().ToString('N') + '.tmp.ps1')
try {
    [IO.File]::WriteAllText($temp, $adapted, [Text.UTF8Encoding]::new($false))
    & pwsh -NoProfile -File $temp
    if ($LASTEXITCODE -ne 0) { throw "1.6.6 package build failed: $LASTEXITCODE" }
} finally {
    Remove-Item $temp -Force -ErrorAction SilentlyContinue
}
