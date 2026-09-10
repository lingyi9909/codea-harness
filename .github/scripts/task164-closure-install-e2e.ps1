$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$zip = Join-Path $repoRoot 'codea-harness-1.6.4-windows-x64-install.zip'
if (-not (Test-Path $zip -PathType Leaf)) { throw "missing install package: $zip" }
$requiredHost = @(
    '.opencode/agents/reviewer.md',
    '.opencode/commands/harness-review-reviewer.md',
    '.opencode/tools/codea-reviewer-submit.ts'
)

function Assert-SameBytes([string]$Expected, [string]$Actual, [string]$Label) {
    if (-not (Test-Path $Actual -PathType Leaf)) { throw "$Label missing: $Actual" }
    $expectedHash = (Get-FileHash -Algorithm SHA256 $Expected).Hash
    $actualHash = (Get-FileHash -Algorithm SHA256 $Actual).Hash
    if ($expectedHash -ne $actualHash) { throw "$Label hash mismatch expected=$expectedHash actual=$actualHash" }
}

$fixture = Join-Path $env:RUNNER_TEMP ('task164-install-e2e-' + [guid]::NewGuid().ToString('N'))
$package = Join-Path $fixture 'package'
$cleanTarget = Join-Path $fixture 'clean-target'
$conflictTarget = Join-Path $fixture 'conflict-target'
try {
    New-Item -ItemType Directory -Force $package,$cleanTarget,$conflictTarget | Out-Null
    Expand-Archive -Path $zip -DestinationPath $package -Force
    $installer = Join-Path $package 'install.ps1'
    if (-not (Test-Path $installer -PathType Leaf)) { throw 'INSTALL_PACKAGE_MISSING_INSTALLER: install.ps1 must be at package root' }
    if (-not (Test-Path (Join-Path $package '.code-harness') -PathType Container)) { throw 'install package missing .code-harness root' }
    foreach ($rel in $requiredHost) {
        if (-not (Test-Path (Join-Path $package $rel) -PathType Leaf)) { throw "install package missing $rel" }
    }

    $cleanOutput = (& pwsh -NoProfile -File $installer -ProjectRoot $cleanTarget 2>&1 | Out-String)
    if ($LASTEXITCODE -ne 0) { throw "clean install failed exit=$LASTEXITCODE`n$cleanOutput" }
    if (-not (Test-Path (Join-Path $cleanTarget '.code-harness') -PathType Container)) { throw 'clean install did not install .code-harness' }
    foreach ($rel in $requiredHost) {
        Assert-SameBytes (Join-Path $package $rel) (Join-Path $cleanTarget $rel) "installed $rel"
    }
    if (-not $cleanOutput.Contains('INSTALL_REVIEWER_HOST_RESOURCES PASS')) { throw 'clean install marker missing' }
    Write-Output 'INSTALL_REVIEWER_HOST_RESOURCES PASS'

    # Conflict must be detected before *any* framework/Host write. Keep two
    # sentinels so the gate proves zero destructive overwrite, not only a
    # non-zero exit code.
    $conflictRel = '.opencode/agents/reviewer.md'
    $conflictPath = Join-Path $conflictTarget $conflictRel
    New-Item -ItemType Directory -Force (Split-Path -Parent $conflictPath) | Out-Null
    [IO.File]::WriteAllText($conflictPath, "user-owned-reviewer`n", [Text.UTF8Encoding]::new($false))
    $unrelatedPath = Join-Path $conflictTarget 'user-owned.txt'
    [IO.File]::WriteAllText($unrelatedPath, "keep-me`n", [Text.UTF8Encoding]::new($false))
    $conflictBefore = (Get-FileHash -Algorithm SHA256 $conflictPath).Hash
    $unrelatedBefore = (Get-FileHash -Algorithm SHA256 $unrelatedPath).Hash

    $stderr = Join-Path $fixture 'install-conflict.stderr.log'
    $stdout = (& pwsh -NoProfile -File $installer -ProjectRoot $conflictTarget 2> $stderr | Out-String)
    $exit = $LASTEXITCODE
    $conflictOutput = $stdout + (if (Test-Path $stderr) { Get-Content -Raw $stderr } else { '' })
    if ($exit -eq 0) { throw "conflicting install unexpectedly succeeded`n$conflictOutput" }
    foreach ($marker in @('MANUAL_ACTION_REQUIRED','INSTALL_EXISTING_OPENCODE_CONFLICT','0 destructive overwrite')) {
        if (-not $conflictOutput.Contains($marker)) { throw "conflict output missing marker: $marker`n$conflictOutput" }
    }
    if (Test-Path (Join-Path $conflictTarget '.code-harness')) { throw 'framework tree was written despite Host collision' }
    if ((Get-FileHash -Algorithm SHA256 $conflictPath).Hash -ne $conflictBefore) { throw 'conflicting Host file was overwritten' }
    if ((Get-FileHash -Algorithm SHA256 $unrelatedPath).Hash -ne $unrelatedBefore) { throw 'unrelated user file was modified' }
    foreach ($rel in $requiredHost) {
        if ($rel -ne $conflictRel -and (Test-Path (Join-Path $conflictTarget $rel))) { throw "Host resource was partially installed before collision stop: $rel" }
    }
    Write-Output 'INSTALL_EXISTING_OPENCODE_CONFLICT_FAIL_CLOSED PASS'
}
finally {
    Remove-Item -Recurse -Force $fixture -ErrorAction SilentlyContinue
}
