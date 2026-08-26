$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$toolsRoot = Join-Path $repoRoot '.code-harness\tools-runtime'

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
    $runtime = Join-Path $repoRoot '.code-harness\bin\codea-harness-tools.exe'
    $astGrep = Join-Path $repoRoot '.code-harness\bin\ast-grep.exe'
    if (!(Test-Path $runtime)) { throw "Windows Runtime missing: $runtime" }
    if (!(Test-Path $astGrep)) { throw "pinned ast-grep missing: $astGrep" }

    # Real Git + real Windows Runtime + pinned ast-grep proof of the originally
    # reported omission: two new Controllers plus one modified Controller.
    $inventoryOutput = (& (Join-Path $PSScriptRoot 'task153-task1-real-entrypoint-inventory.ps1') 2>&1 | Out-String)
    if ($LASTEXITCODE -ne 0 -or $inventoryOutput -notmatch 'TASK153_TASK1_REAL_ENTRYPOINT_INVENTORY PASS') {
        throw "real three-Controller inventory regression failed:`n$inventoryOutput"
    }
    Write-Output 'CONTROLLER_ENTRYPOINTS 3/3'

    Invoke-Task153GoGate -Packages @('./internal/analysis') -Pattern 'Test153CertRejectsIncompleteEntrypointDraft'
    Write-Output 'INCOMPLETE_DRAFT_REJECTED'

    Invoke-Task153GoGate -Packages @('./internal/analysis') -Pattern 'Test153TamperRejectsChangedAuthoritativeAnalysisBytes'
    Write-Output 'CERTIFIED_ANALYSIS_TAMPER_REJECTED'

    Invoke-Task153GoGate -Packages @('./internal/chain') -Pattern 'Test153CandidateAuthorityRejectsMutatedRuntimeCandidate|Test153CandidateAuthorityRejectsMutationBeforeRuntimeCertification'
    Write-Output 'CHAIN_CANDIDATE_TAMPER_REJECTED'

    Invoke-Task153GoGate -Packages @('./internal/reviewselection','./cmd/codea-harness-tools') -Pattern 'Test153AutoSingleSelectionIsMachineExecutable|Test153ReviewOptionsAutoSingleExecutesWithoutUserChoice'
    Write-Output 'AUTO_SINGLE_NO_SELECTION'

    Invoke-Task153GoGate -Packages @('./internal/reviewselection') -Pattern 'Test153ReviewOptionsDecisionZeroOneTwo|Test153SelectionRejectsStaleOptionsHash|Test153SelectionRejectsUnknownChain|Test153SelectionAcceptsRuntimeBoundIDs'
    Write-Output 'MULTI_CHAIN_SELECTION_VERIFIED'

    Invoke-Task153GoGate -Packages @('./internal/chain') -Pattern 'Test153EditSupportsAllSemanticOperations'
    Write-Output 'CHAIN_EDIT_VERIFIED'

    Invoke-Task153GoGate -Packages @('./internal/chain') -Pattern 'Test153EditRejectsUnverifiedCodeFacts'
    Write-Output 'UNVERIFIED_EDIT_REJECTED'

    Write-Output 'TASK153_REAL_REVIEW_CHAIN_RELIABILITY PASS'
}
finally {
    Pop-Location
}
