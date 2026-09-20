$ErrorActionPreference = 'Stop'

$activeFiles = @(
    '.code-harness/AGENTS.md',
    '.code-harness/tools/README.md'
)
$activeFiles += Get-ChildItem '.code-harness/agents' -Filter '*.md' -File | ForEach-Object { $_.FullName }
$activeFiles += Get-ChildItem '.code-harness/skills' -Filter 'SKILL.md' -File -Recurse | ForEach-Object { $_.FullName }

$legacyInvocation = '(?<![A-Za-z0-9_.-])codea-harness-tools(?:\.exe)?\s+(?:upgrade|validate|workspace|nav|db|chain|analysis|review|report|seal-apply|apply)\b'
$violations = @()
foreach ($file in $activeFiles) {
    $matches = Select-String -Path $file -Pattern $legacyInvocation -AllMatches
    foreach ($match in $matches) {
        $violations += "${file}:$($match.LineNumber): $($match.Line.Trim())"
    }
}
if ($violations.Count -gt 0) {
    Write-Host 'Legacy Runtime invocation remains in Active Agent contract:'
    $violations | ForEach-Object { Write-Host $_ }
    throw 'TASK162_HOTFIX_TASK2_ACTIVE_INVOCATION_AUDIT FAIL'
}

$requiredSchemas = @(
    '.code-harness/contracts/change-set-request.schema.json',
    '.code-harness/contracts/analysis-inventory-request.schema.json',
    '.code-harness/contracts/analysis-certify-request.schema.json',
    '.code-harness/contracts/review-options-request.schema.json'
)
foreach ($schemaPath in $requiredSchemas) {
    if (-not (Test-Path $schemaPath)) { throw "Missing Task 2 request contract: $schemaPath" }
    $schema = Get-Content -Raw $schemaPath | ConvertFrom-Json
    if ($schema.additionalProperties -eq $true) { throw "Request contract permits additionalProperties: $schemaPath" }
}

$reviewOptionsSchema = Get-Content -Raw '.code-harness/contracts/review-options-request.schema.json'
if ($reviewOptionsSchema -match '"baseRef"') { throw 'review-options request contract must not expose baseRef' }

Write-Output 'TASK162_HOTFIX_TASK2_ACTIVE_INVOCATION_AUDIT PASS'
Write-Output 'TASK162_HOTFIX_TASK2_REQUEST_CONTRACTS PASS'
Write-Output 'TASK162_HOTFIX_TASK2_INVOCATION_CONTRACT PASS'
