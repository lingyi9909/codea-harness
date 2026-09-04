$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$driverPath = '.github/scripts/task162-release-certification.ps1'
$packagePath = '.github/scripts/task162-task2-package.ps1'
$workflowPath = '.github/workflows/task162-review-reliability-final-certification-contract.yml'

foreach ($path in @($driverPath, $packagePath, $workflowPath)) {
    if (-not (Test-Path $path -PathType Leaf)) {
        throw "REVIEW_RELIABILITY_FINAL_CERT_CONTRACT_MISSING file: $path"
    }
}

$driver = Get-Content -Raw $driverPath
$package = Get-Content -Raw $packagePath
$workflow = Get-Content -Raw $workflowPath

# The previous 1.6.2 release/hotfix certification remains retained evidence.
$requiredRetainedDriverFragments = @(
    '119c87057718f3d1f6f0286622d32b350f21d64e',
    '2503678e347dc0ba2bc2f0357cefd9306d199480',
    '4a312c4a2c85a202b740d3a1f419b2812e42f866',
    'task162-hotfix-task1-canonical-changeset-regression.ps1',
    'task162-final-task2-invocation-contract-regression.ps1',
    'task162-hotfix-task2-runtime-invocation-regression.ps1',
    'task162-hotfix-task3-real-plain-review-e2e.ps1',
    'hotfixTask1CanonicalAuthority',
    'hotfixTask2InvocationContract',
    'hotfixTask3RealPlainReview'
)
foreach ($fragment in $requiredRetainedDriverFragments) {
    if (-not $driver.Contains($fragment)) {
        throw "REVIEW_RELIABILITY_FINAL_CERT_CONTRACT_REGRESSION retained driver fragment missing: $fragment"
    }
}

# This Review Reliability Hotfix must be certified from its accepted base and all three accepted task heads.
$requiredReviewReliabilityFragments = @(
    'e23023481edef9f95cdc59938efe5de4840093b8',
    '1141a240529ea3fedcc8df0d3750db31f9fb1104',
    'ab2f42f53f472aebf2b18b11a1c9166feee2a20c',
    '6c5af8908edf73a5fb772069e9bc2d91d2ebe289',
    'acceptedReviewReliability',
    'task162-review-reliability-task1-contract-regression.ps1',
    'task162-review-reliability-task1-real-agent-e2e-v2.ps1',
    'task162-review-reliability-task2-contract-regression.ps1',
    "go test -count=1 -run 'Test162ReviewReliabilityTask(1|2)' -v ./cmd/codea-dcep-tools",
    'task162-review-reliability-task2-real-agent-e2e.ps1',
    'task162-review-reliability-task3-run-readme-regression.ps1',
    'reviewReliabilityTask1InvocationContract',
    'reviewReliabilityTask1RealAgent',
    'reviewReliabilityTask2FreshLifecycle',
    'reviewReliabilityTask2SameSession',
    'reviewReliabilityTask3RunReadme',
    'postReviewReliabilityTask3CertificationScope'
)
foreach ($fragment in $requiredReviewReliabilityFragments) {
    if (-not $driver.Contains($fragment)) {
        throw "REVIEW_RELIABILITY_FINAL_CERT_CONTRACT_RED driver fragment missing: $fragment"
    }
}

if (-not $driver.Contains('git diff --name-only "$acceptedReviewReliabilityTask3..HEAD"')) {
    throw 'REVIEW_RELIABILITY_FINAL_CERT_CONTRACT_RED post-Task3 scope is not bound to the accepted Review Reliability Task 3 head'
}

# Task 3 explicitly requires the Chinese Run README to ship with Harness. Real run state must still be excluded.
foreach ($fragment in @(
    'runs/README.md',
    'TASK162_REVIEW_RELIABILITY_TASK3_PACKAGE_README PASS'
)) {
    if (-not $package.Contains($fragment)) {
        throw "REVIEW_RELIABILITY_FINAL_CERT_PACKAGE_RED package fragment missing: $fragment"
    }
}
foreach ($fragment in @(
    'Assert-RunReadmeOnly',
    'runs/README.md',
    'reviewReliabilityTask3PackageReadme'
)) {
    if (-not $driver.Contains($fragment)) {
        throw "REVIEW_RELIABILITY_FINAL_CERT_PACKAGE_RED release driver package assertion missing: $fragment"
    }
}

foreach ($fragment in @(
    'release/1.6.2-review-reliability-final-certification',
    'task162-review-reliability-final-certification-contract-regression.ps1',
    'ref: ${{ github.sha }}',
    'fetch-depth: 0',
    'TASK162_REVIEW_RELIABILITY_FINAL_CERT_CONTRACT_EXACT_HEAD PASS'
)) {
    if (-not $workflow.Contains($fragment)) {
        throw "REVIEW_RELIABILITY_FINAL_CERT_CONTRACT_INVALID workflow fragment missing: $fragment"
    }
}

Write-Output 'TASK162_REVIEW_RELIABILITY_FINAL_CERTIFICATION_CONTRACT PASS'
exit 0
