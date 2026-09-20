$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$legacy = Join-Path $PSScriptRoot 'task162-task2-release-package-cleanup-regression.ps1'
if (-not (Test-Path $legacy -PathType Leaf)) { throw "retained package cleanup regression missing: $legacy" }

Push-Location $repoRoot
try {
    # The retained regression intentionally executes native negative-path probes.
    # Its final PASS markers are authoritative, but a successful script can leave
    # LASTEXITCODE=1 behind. Run it in a child command that explicitly returns 0
    # only after the script itself completes without a terminating error.
    $escapedLegacy = $legacy.Replace("'", "''")
    $command = "& '$escapedLegacy'; exit 0"
    $lines = @(& pwsh -NoProfile -Command $command 2>&1)
    $childExit = $LASTEXITCODE
    $lines | ForEach-Object { Write-Output $_ }
    if ($childExit -ne 0) { throw "retained package cleanup regression failed with child exit code $childExit" }

    $text = ($lines | Out-String)
    foreach ($marker in @(
        'TASK162_TASK2_ARTIFACT_CLEAN PASS',
        'TASK162_TASK2_NEW_INSTALL_NO_GO_ANALYSIS_REVIEW PASS',
        'TASK162_TASK2_REAL_161_TO_162_UPGRADE PASS',
        'TASK162_TASK2_PROJECT_STATE_PRESERVATION PASS',
        'TASK162_TASK2_RELEASE_PACKAGE_CLEANUP_E2E PASS'
    )) {
        if ($text -notmatch [regex]::Escape($marker)) {
            throw "retained package cleanup missing marker: $marker`n$text"
        }
    }

    $global:LASTEXITCODE = 0
    Write-Output 'TASK162_FINAL_PACKAGE_CLEANUP_EXIT_NORMALIZED PASS'
}
finally {
    Pop-Location
}
