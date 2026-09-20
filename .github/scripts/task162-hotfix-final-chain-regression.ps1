$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$toolsRoot = Join-Path $repoRoot '.code-harness\tools-runtime'
$entrypointAdapter = Join-Path $PSScriptRoot 'task162-hotfix-final-entrypoint-inventory-regression.ps1'

function Invoke-Task153GoGate {
    param(
        [Parameter(Mandatory=$true)][string[]]$Packages,
        [Parameter(Mandatory=$true)][string]$Pattern
    )
    Push-Location $toolsRoot
    try {
        & go test -count=1 @Packages -run $Pattern
        if ($LASTEXITCODE -ne 0) { throw "Task 1.5.3 Go regression failed: $Pattern" }
    }
    finally {
        Pop-Location
    }
}

Push-Location $repoRoot
try {
    $runtime = Join-Path $repoRoot '.code-harness\bin\codea-dcep-tools.exe'
    $astGrep = Join-Path $repoRoot '.code-harness\bin\ast-grep.exe'
    if (!(Test-Path $runtime)) { throw "Windows Runtime missing: $runtime" }
    if (!(Test-Path $astGrep)) { throw "pinned ast-grep missing: $astGrep" }
    if (!(Test-Path $entrypointAdapter)) { throw "Final EntryPoint adapter missing: $entrypointAdapter" }

    $inventoryOutput = (& pwsh -NoProfile -File $entrypointAdapter 2>&1 | Out-String)
    $inventoryExit = $LASTEXITCODE
    Write-Output $inventoryOutput
    if ($inventoryExit -ne 0 -or $inventoryOutput -notmatch 'TASK153_TASK1_REAL_ENTRYPOINT_INVENTORY PASS' -or $inventoryOutput -notmatch 'TASK162_FINAL_ENTRYPOINT_TASK2_REQUEST_CONTRACT_COMPAT PASS') {
        throw "final retained three-Controller inventory regression failed:`n$inventoryOutput"
    }
    $global:LASTEXITCODE = 0
    Write-Output 'CONTROLLER_ENTRYPOINTS 3/3'

    Invoke-Task153GoGate -Packages @('./internal/analysis') -Pattern 'Test153CertRejectsIncompleteEntrypointDraft'
    Write-Output 'INCOMPLETE_DRAFT_REJECTED'

    Invoke-Task153GoGate -Packages @('./internal/analysis') -Pattern 'Test153TamperRejectsChangedAuthoritativeAnalysisBytes'
    Write-Output 'CERTIFIED_ANALYSIS_TAMPER_REJECTED'

    Invoke-Task153GoGate -Packages @('./internal/chain') -Pattern 'Test153CandidateAuthorityRejectsMutatedRuntimeCandidate|Test153CandidateAuthorityRejectsMutationBeforeRuntimeCertification'
    Write-Output 'CHAIN_CANDIDATE_TAMPER_REJECTED'

    Invoke-Task153GoGate -Packages @('./internal/reviewselection','./cmd/codea-dcep-tools') -Pattern 'Test153AutoSingleSelectionIsMachineExecutable|Test153ReviewOptionsAutoSingleExecutesWithoutUserChoice'
    Write-Output 'AUTO_SINGLE_NO_SELECTION'

    Invoke-Task153GoGate -Packages @('./internal/reviewselection') -Pattern 'Test153ReviewOptionsDecisionZeroOneTwo|Test153SelectionRejectsStaleOptionsHash|Test153SelectionRejectsUnknownChain|Test153SelectionAcceptsRuntimeBoundIDs'
    Write-Output 'MULTI_CHAIN_SELECTION_VERIFIED'

    Invoke-Task153GoGate -Packages @('./internal/chain') -Pattern 'Test153EditSupportsAllSemanticOperations'
    Write-Output 'CHAIN_EDIT_VERIFIED'

    Invoke-Task153GoGate -Packages @('./internal/chain') -Pattern 'Test153EditRejectsUnverifiedCodeFacts'
    Write-Output 'UNVERIFIED_EDIT_REJECTED'

    Write-Output 'TASK153_REAL_REVIEW_CHAIN_RELIABILITY PASS'
    Write-Output 'TASK162_FINAL_RETAINED_CHAIN_TASK2_CONTRACT_COMPAT PASS'
}
finally {
    Pop-Location
}
