$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

# Release-only adapter for the already accepted Task 4 package builder.
# The original remains byte-for-byte unchanged; only its version literals change.
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$source = Join-Path $PSScriptRoot 'task162-task2-package.ps1'
$expectedBlob = '84ff6bc44c06a3dbabd72cd8f807c008acf05594'
$actualBlob = (& git -C $repoRoot hash-object -- $source).Trim()
if ($LASTEXITCODE -ne 0 -or $actualBlob -ne $expectedBlob) { throw "Release builder source changed: $actualBlob" }
$version = (Get-Content (Join-Path $repoRoot '.code-harness/VERSION') -Raw).Trim()
if ($version -ne '1.6.3') { throw "Expected release version 1.6.3, got $version" }
$text = Get-Content $source -Raw
if ([regex]::Matches($text, [regex]::Escape('1.6.2')).Count -lt 4) { throw 'Unexpected retained package builder version contract' }
$adapted = $text.Replace('1.6.2', '1.6.3')
if ($adapted.Contains('1.6.2')) { throw 'Unadapted release version' }
$temp = Join-Path $PSScriptRoot ('.task163-release-package-' + [guid]::NewGuid().ToString('N') + '.tmp.ps1')
try {
    [IO.File]::WriteAllText($temp, $adapted, [Text.UTF8Encoding]::new($false))
    & pwsh -NoProfile -File $temp
    if ($LASTEXITCODE -ne 0) { throw "Release package builder failed: $LASTEXITCODE" }
} finally {
    Remove-Item $temp -Force -ErrorAction SilentlyContinue
    $global:LASTEXITCODE = 0
}
foreach ($kind in @('install','upgrade')) {
    $path = Join-Path $repoRoot "codea-harness-1.6.3-windows-x64-$kind.zip"
    if (-not (Test-Path $path -PathType Leaf)) { throw "Missing release package: $path" }
}
Write-Output 'TASK163_RELEASE_PACKAGE_BUILD PASS version=1.6.3'
