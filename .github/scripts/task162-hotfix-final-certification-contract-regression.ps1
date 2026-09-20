$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$driverPath = '.github/scripts/task162-release-certification.ps1'
$workflowPath = '.github/workflows/task162-final-release-certification.yml'
$task2FinalContractPath = '.github/scripts/task162-final-task2-invocation-contract-regression.ps1'
$tailAdapterPaths = @(
    '.github/scripts/task162-hotfix-final-entrypoint-inventory-regression.ps1',
    '.github/scripts/task162-hotfix-final-chain-regression.ps1',
    '.github/scripts/task162-hotfix-final-package-cleanup-regression.ps1'
)

foreach ($path in @($driverPath, $workflowPath, $task2FinalContractPath) + $tailAdapterPaths) {
    if (-not (Test-Path $path -PathType Leaf)) { throw "Final certification contract file missing: $path" }
}

$driver = Get-Content -Raw $driverPath
$workflow = Get-Content -Raw $workflowPath

$requiredDriverFragments = @(
    '119c87057718f3d1f6f0286622d32b350f21d64e',
    '2503678e347dc0ba2bc2f0357cefd9306d199480',
    '4a312c4a2c85a202b740d3a1f419b2812e42f866',
    'task162-hotfix-task1-agent-snapshot-request-contract.ps1',
    'task162-hotfix-task1-canonical-changeset-regression.ps1',
    'task162-final-task2-invocation-contract-regression.ps1',
    "go test -count=1 -run 'Test162HotfixTask2' -v ./cmd/codea-dcep-tools",
    'task162-hotfix-task2-runtime-invocation-regression.ps1',
    'task162-hotfix-task3-real-plain-review-e2e.ps1',
    'task162-hotfix-final-entrypoint-inventory-regression.ps1',
    'task162-hotfix-final-chain-regression.ps1',
    'task162-hotfix-final-package-cleanup-regression.ps1',
    'hotfixTask1CanonicalAuthority',
    'hotfixTask2InvocationContract',
    'hotfixTask3RealPlainReview',
    'postTask3CertificationScope'
)
foreach ($fragment in $requiredDriverFragments) {
    if (-not $driver.Contains($fragment)) {
        throw "FINAL_HOTFIX_CERT_CONTRACT_MISSING driver fragment: $fragment"
    }
}

foreach ($legacyDirect in @(
    "Invoke-Regression './.github/scripts/task162-hotfix-task2-invocation-contract-regression.ps1'",
    "Invoke-Regression './.github/scripts/task153-task1-real-entrypoint-inventory.ps1'",
    "Invoke-Regression './.github/scripts/task153-real-review-chain-regression.ps1'",
    '$task2Script = (Resolve-Path ''./.github/scripts/task162-task2-release-package-cleanup-regression.ps1'').Path'
)) {
    if ($driver.Contains($legacyDirect)) {
        throw "FINAL_HOTFIX_CERT_CONTRACT_INVALID final certification still invokes fragile retained gate directly: $legacyDirect"
    }
}

$task2FinalContract = Get-Content -Raw $task2FinalContractPath
foreach ($fragment in @(
    'TASK162_HOTFIX_TASK2_ACTIVE_INVOCATION_AUDIT PASS',
    'TASK162_HOTFIX_TASK2_REQUEST_CONTRACTS PASS',
    'TASK162_HOTFIX_TASK2_INVOCATION_CONTRACT PASS',
    'oneOf',
    'additionalProperties',
    'review-options-request.schema.json',
    '"baseRef"'
)) {
    if (-not $task2FinalContract.Contains($fragment)) {
        throw "FINAL_HOTFIX_CERT_CONTRACT_INVALID Task 2 adapter missing evidence: $fragment"
    }
}

$entrypointAdapter = Get-Content -Raw $tailAdapterPaths[0]
foreach ($fragment in @('analysis-inventory-request.schema.json','TASK153_TASK1_REAL_ENTRYPOINT_INVENTORY PASS','TASK162_FINAL_ENTRYPOINT_TASK2_REQUEST_CONTRACT_COMPAT PASS')) {
    if (-not $entrypointAdapter.Contains($fragment)) { throw "FINAL_HOTFIX_CERT_CONTRACT_INVALID EntryPoint adapter missing evidence: $fragment" }
}
$chainAdapter = Get-Content -Raw $tailAdapterPaths[1]
foreach ($fragment in @('task162-hotfix-final-entrypoint-inventory-regression.ps1','TASK153_REAL_REVIEW_CHAIN_RELIABILITY PASS','TASK162_FINAL_RETAINED_CHAIN_TASK2_CONTRACT_COMPAT PASS')) {
    if (-not $chainAdapter.Contains($fragment)) { throw "FINAL_HOTFIX_CERT_CONTRACT_INVALID Chain adapter missing evidence: $fragment" }
}
$packageAdapter = Get-Content -Raw $tailAdapterPaths[2]
foreach ($fragment in @('task162-task2-release-package-cleanup-regression.ps1','TASK162_TASK2_RELEASE_PACKAGE_CLEANUP_E2E PASS','TASK162_FINAL_PACKAGE_CLEANUP_EXIT_NORMALIZED PASS')) {
    if (-not $packageAdapter.Contains($fragment)) { throw "FINAL_HOTFIX_CERT_CONTRACT_INVALID package adapter missing evidence: $fragment" }
}

$invokeRegressionMatch = [regex]::Match(
    $driver,
    'function Invoke-Regression\(\[string\]\$Script, \[string\]\$Label\) \{(?<body>[\s\S]*?)\r?\n\}',
    [System.Text.RegularExpressions.RegexOptions]::CultureInvariant
)
if (-not $invokeRegressionMatch.Success) {
    throw 'FINAL_HOTFIX_CERT_CONTRACT_MISSING Invoke-Regression body'
}
$invokeRegressionBody = $invokeRegressionMatch.Groups['body'].Value
if ($invokeRegressionBody -match '&\s+\$Script\b') {
    throw 'FINAL_HOTFIX_CERT_CONTRACT_INVALID accepted regression executes in parent PowerShell scope'
}
foreach ($fragment in @('pwsh', '-NoProfile', '-File', '$childExit')) {
    if (-not $invokeRegressionBody.Contains($fragment)) {
        throw "FINAL_HOTFIX_CERT_CONTRACT_INVALID isolated regression wrapper missing: $fragment"
    }
}
if ($invokeRegressionBody -notmatch '\$childExit\s+-ne\s+0') {
    throw 'FINAL_HOTFIX_CERT_CONTRACT_INVALID isolated regression wrapper does not fail on child process exit code'
}

$requiredWorkflowFragments = @(
    'actions/setup-python@v6',
    "python-version: '3.12'",
    'opencode-ai@1.18.25',
    'task162-hotfix-final-certification-contract-regression.ps1'
)
foreach ($fragment in $requiredWorkflowFragments) {
    if (-not $workflow.Contains($fragment)) {
        throw "FINAL_HOTFIX_CERT_CONTRACT_MISSING workflow fragment: $fragment"
    }
}

if ($workflow -match 'opencode-ai@(latest|\*)') {
    throw 'FINAL_HOTFIX_CERT_CONTRACT_INVALID unpinned OpenCode host'
}

Write-Output 'TASK162_HOTFIX_FINAL_TASK2_ADAPTER_CONTRACT PASS'
Write-Output 'TASK162_HOTFIX_FINAL_TAIL_ADAPTER_CONTRACT PASS'
Write-Output 'TASK162_HOTFIX_FINAL_REGRESSION_ISOLATION_CONTRACT PASS'
Write-Output 'TASK162_HOTFIX_FINAL_CERTIFICATION_CONTRACT PASS'
