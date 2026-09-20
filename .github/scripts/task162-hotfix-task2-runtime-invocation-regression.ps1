$ErrorActionPreference = 'Stop'

$exe = Join-Path (Get-Location) '.code-harness/bin/codea-dcep-tools.exe'
if (-not (Test-Path $exe)) {
    throw "built Runtime not found: $exe"
}

function Get-RepoRelativePath {
    param([string]$Path)
    return [System.IO.Path]::GetRelativePath((Get-Location).Path, (Resolve-Path $Path).Path).Replace('\','/')
}

function Invoke-ExpectedFailure {
    param(
        [string[]]$Arguments,
        [string]$Expected,
        [string]$Forbidden = ''
    )
    $output = (& $exe @Arguments 2>&1 | Out-String)
    $exitCode = $LASTEXITCODE
    $global:LASTEXITCODE = 0
    if ($exitCode -eq 0) {
        throw "expected Runtime failure for args [$($Arguments -join ' ')], got success: $output"
    }
    if (-not $output.Contains($Expected)) {
        throw "expected Runtime output to contain '$Expected' for args [$($Arguments -join ' ')], got: $output"
    }
    if ($Forbidden -and $output.Contains($Forbidden)) {
        throw "Runtime output unexpectedly contains '$Forbidden': $output"
    }
    return $output
}

$zeroArg = (& $exe 2>&1 | Out-String)
$zeroExit = $LASTEXITCODE
$global:LASTEXITCODE = 0
if ($zeroExit -eq 0) { throw 'zero-arg Runtime unexpectedly succeeded' }
if (-not $zeroArg.Contains('usage: codea-dcep-tools')) { throw "zero-arg usage missing Runtime name: $zeroArg" }
if ($zeroArg.Contains('codea-harness-tools')) { throw "zero-arg usage contains legacy Runtime name: $zeroArg" }
Write-Output 'TASK162_HOTFIX_TASK2_ZERO_ARG_RUNTIME PASS'

$runId = "task2-runtime-contract-$PID"
$requestsDir = Join-Path '.code-harness/runs' "$runId/requests"
New-Item -ItemType Directory -Path $requestsDir -Force | Out-Null
try {
    $snapshotPath = Join-Path $requestsDir 'snapshot-invalid.json'
    [System.IO.File]::WriteAllText($snapshotPath, "{`"runId`":`"$runId`",`"requestedBaseRef`":`"HEAD`",`"includeWorkingTree`":true}")
    Invoke-ExpectedFailure @('analysis','snapshot','--input',(Get-RepoRelativePath $snapshotPath)) 'CHANGE_SET_REQUEST_SCHEMA_INVALID' | Out-Null
    Write-Output 'TASK162_HOTFIX_TASK2_SNAPSHOT_UNKNOWN_FIELD_REJECTED PASS'

    $inventoryPath = Join-Path $requestsDir 'inventory-invalid.json'
    [System.IO.File]::WriteAllText($inventoryPath, "{`"runId`":`"$runId`",`"baseRef`":`"HEAD`",`"includeWorkingTree`":true,`"intent`":{`"mode`":`"FULL`"},`"unexpected`":true}")
    Invoke-ExpectedFailure @('analysis','inventory','--input',(Get-RepoRelativePath $inventoryPath)) 'ANALYSIS_INVENTORY_REQUEST_SCHEMA_INVALID' | Out-Null
    Write-Output 'TASK162_HOTFIX_TASK2_INVENTORY_UNKNOWN_FIELD_REJECTED PASS'

    $certifyPath = Join-Path $requestsDir 'certify-invalid.json'
    [System.IO.File]::WriteAllText($certifyPath, "{`"runId`":`"$runId`",`"snapshotPath`":`".code-harness/runs/$runId/analysis/change-set.json`",`"snapshotSha256`":`"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa`",`"proposalPath`":`".code-harness/runs/$runId/requests/change-analysis-proposal.json`",`"intent`":{`"mode`":`"FULL`"},`"unexpected`":true}")
    Invoke-ExpectedFailure @('analysis','certify','--input',(Get-RepoRelativePath $certifyPath)) 'ANALYSIS_CERTIFY_REQUEST_SCHEMA_INVALID' | Out-Null
    Write-Output 'TASK162_HOTFIX_TASK2_CERTIFY_UNKNOWN_FIELD_REJECTED PASS'

    $reviewInvalidPath = Join-Path $requestsDir 'review-options-invalid.json'
    [System.IO.File]::WriteAllText($reviewInvalidPath, "{`"runId`":`"$runId`",`"changeAnalysisPath`":`".code-harness/runs/$runId/analysis/change-analysis.json`",`"baseRef`":`"HEAD`"}")
    Invoke-ExpectedFailure @('review','options','--input',(Get-RepoRelativePath $reviewInvalidPath)) 'REVIEW_OPTIONS_REQUEST_SCHEMA_INVALID' | Out-Null
    Write-Output 'TASK162_HOTFIX_TASK2_REVIEW_OPTIONS_BASEREF_REJECTED PASS'

    $reviewValidPath = Join-Path $requestsDir 'review-options-valid.json'
    [System.IO.File]::WriteAllText($reviewValidPath, "{`"runId`":`"$runId`",`"changeAnalysisPath`":`".code-harness/runs/$runId/analysis/change-analysis.json`"}")
    $validOutput = (& $exe review options --input (Get-RepoRelativePath $reviewValidPath) 2>&1 | Out-String)
    $validExit = $LASTEXITCODE
    $global:LASTEXITCODE = 0
    if ($validExit -eq 0) {
        throw 'valid review options unexpectedly completed without a Certified ChangeAnalysis fixture'
    }
    if ($validOutput.Contains('REVIEW_OPTIONS_REQUEST_SCHEMA_INVALID')) {
        throw "valid review options was rejected by request schema: $validOutput"
    }
    Write-Output 'TASK162_HOTFIX_TASK2_REVIEW_OPTIONS_VALID_SCHEMA PASS'
} finally {
    Remove-Item -Recurse -Force (Join-Path '.code-harness/runs' $runId) -ErrorAction SilentlyContinue
}

Write-Output 'TASK162_HOTFIX_TASK2_RUNTIME_INVOCATION_REGRESSION PASS'
