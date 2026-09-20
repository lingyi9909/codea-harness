$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Has-Property([object]$Object, [string]$Name) {
    return $null -ne $Object.PSObject.Properties[$Name]
}

function Assert-ClosedObjectContract([object]$Schema, [string]$Context) {
    if (Has-Property $Schema 'additionalProperties') {
        if ($Schema.additionalProperties -ne $false) {
            throw "Request contract is not closed at ${Context}: additionalProperties must be false"
        }
        return
    }

    if (Has-Property $Schema 'oneOf') {
        $branches = @($Schema.oneOf)
        if ($branches.Count -eq 0) { throw "Request contract oneOf is empty at $Context" }
        for ($i = 0; $i -lt $branches.Count; $i++) {
            Assert-ClosedObjectContract $branches[$i] "${Context}.oneOf[$i]"
        }
        return
    }

    throw "Request contract does not close object properties at $Context"
}

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
    if (-not (Test-Path $schemaPath -PathType Leaf)) { throw "Missing Task 2 request contract: $schemaPath" }
    $schema = Get-Content -Raw $schemaPath | ConvertFrom-Json
    Assert-ClosedObjectContract $schema $schemaPath
}

$analysisCertifySchema = Get-Content -Raw '.code-harness/contracts/analysis-certify-request.schema.json' | ConvertFrom-Json
if (-not (Has-Property $analysisCertifySchema 'oneOf')) {
    throw 'analysis-certify request contract must preserve canonical/legacy oneOf compatibility'
}
if (@($analysisCertifySchema.oneOf).Count -ne 2) {
    throw 'analysis-certify request contract must have exactly two approved oneOf branches'
}

$reviewOptionsSchema = Get-Content -Raw '.code-harness/contracts/review-options-request.schema.json'
if ($reviewOptionsSchema -match '"baseRef"') { throw 'review-options request contract must not expose "baseRef"' }

Write-Output 'TASK162_HOTFIX_TASK2_ACTIVE_INVOCATION_AUDIT PASS'
Write-Output 'TASK162_HOTFIX_TASK2_REQUEST_CONTRACTS PASS'
Write-Output 'TASK162_HOTFIX_TASK2_INVOCATION_CONTRACT PASS'
Write-Output 'TASK162_FINAL_TASK2_ONEOF_CONTRACT_ADAPTER PASS'
