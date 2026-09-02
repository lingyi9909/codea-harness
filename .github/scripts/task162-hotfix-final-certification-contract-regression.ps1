$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$driverPath = '.github/scripts/task162-release-certification.ps1'
$workflowPath = '.github/workflows/task162-final-release-certification.yml'

foreach ($path in @($driverPath, $workflowPath)) {
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
    'task162-hotfix-task2-invocation-contract-regression.ps1',
    'task162-hotfix-task2-runtime-invocation-regression.ps1',
    'task162-hotfix-task3-real-plain-review-e2e.ps1',
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

Write-Output 'TASK162_HOTFIX_FINAL_CERTIFICATION_CONTRACT PASS'
