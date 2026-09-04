$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$readmePath = Join-Path $repoRoot '.code-harness/runs/README.md'
$gitignorePath = Join-Path $repoRoot '.code-harness/.gitignore'

if (!(Test-Path $readmePath -PathType Leaf)) {
    throw 'TASK162_REVIEW_RELIABILITY_TASK3_RED: missing .code-harness/runs/README.md'
}
if (!(Test-Path $gitignorePath -PathType Leaf)) {
    throw 'Task 3 requires .code-harness/.gitignore'
}

$readme = Get-Content -Raw -Encoding UTF8 $readmePath
$gitignore = Get-Content -Raw -Encoding UTF8 $gitignorePath

function Assert-Contains([string]$Text, [string]$Pattern, [string]$Message) {
    if ($Text -notmatch $Pattern) { throw $Message }
}

Assert-Contains $readme '[\u4e00-\u9fff]' 'Task 3 README must contain Chinese documentation'
Assert-Contains $gitignore '(?m)^runs/\*$' 'Task 3 must keep runs/* ignored'
Assert-Contains $gitignore '(?m)^!runs/\.gitkeep$' 'Task 3 must keep runs/.gitkeep tracked'
Assert-Contains $gitignore '(?m)^!runs/README\.md$' 'Task 3 must track runs/README.md through a narrow ignore exception'

Assert-Contains $readme '`harness review`' 'Task 3 README must explain harness review'
Assert-Contains $readme '`review begin`' 'Task 3 README must explain review begin'
Assert-Contains $readme '`analysis snapshot`' 'Task 3 README must explain fresh analysis snapshot'
Assert-Contains $readme 'fresh runId|全新.*runId|新的.*runId|新.*runId' 'Task 3 README must state that each top-level review gets a fresh runId'
Assert-Contains $readme '旧.*run|上一.*run|previous.*run|之前.*run' 'Task 3 README must discuss previous-run artifacts'
Assert-Contains $readme '非权威|不具备权威|不得.*复用|不能.*复用|不可.*复用' 'Task 3 README must make previous-run evidence non-authoritative for a new review'
Assert-Contains $readme 'same-run|同一.*runId|同一次.*runId' 'Task 3 README must explain same-run/intra-invocation affinity'

$requiredArtifacts = @(
    'analysis/change-set.json',
    'analysis/entrypoint-inventory.json',
    'analysis/change-analysis.json',
    'analysis/change-analysis.cert.json',
    'analysis/review-options.json',
    'analysis/review-scope.json',
    'analysis/review-units.json',
    'analysis/rule-dispatch.json',
    'analysis/certified-findings.json',
    'analysis/certified-findings.cert.json',
    'review.md'
)
foreach ($artifact in $requiredArtifacts) {
    if (-not $readme.Contains($artifact)) {
        throw "Task 3 README missing canonical artifact name: $artifact"
    }
}

Assert-Contains $readme 'requests/\*\*|requests/' 'Task 3 README must explain requests transport'
Assert-Contains $readme 'transport|传输|请求中转' 'Task 3 README must identify requests as transport-only'
Assert-Contains $readme '不是.*权威|非权威|不属于.*权威' 'Task 3 README must separate transport/documentation from authority'
Assert-Contains $readme '最终.*review\.md|正式.*review\.md|review\.md.*最终|review\.md.*正式' 'Task 3 README must identify review.md as the final formal Review report'
Assert-Contains $readme 'review\.json' 'Task 3 README must explicitly reject review.json as the formal Review report'
Assert-Contains $readme '0.*变更|零变更|无变更' 'Task 3 README must explain zero-change Review'
Assert-Contains $readme 'finding-proposals.*\[\]|空.*finding-proposals|finding-proposals.*空' 'Task 3 README must explain empty finding proposals in zero-change flow'
Assert-Contains $readme 'PASSED' 'Task 3 README must explain PASSED'
Assert-Contains $readme 'FAILED' 'Task 3 README must explain FAILED'
Assert-Contains $readme 'MANUAL_ACTION_REQUIRED' 'Task 3 README must explain MANUAL_ACTION_REQUIRED'
Assert-Contains $readme '不是.*Authority|不是.*权威|非.*Authority|非权威' 'Task 3 README must state that the README itself is not Review Authority'
Assert-Contains $readme '不是.*恢复|不用于.*恢复|非.*恢复' 'Task 3 README must state that runs documentation is not recovery state'
Assert-Contains $readme '不要.*修改|不得.*修改|禁止.*修改' 'Task 3 README must warn against manually editing formal authority artifacts'

Push-Location $repoRoot
try {
    & git check-ignore -q --no-index '.code-harness/runs/review-task3-probe/review.md'
    if ($LASTEXITCODE -ne 0) { throw 'Task 3 must keep real run artifacts ignored' }

    & git check-ignore -q --no-index '.code-harness/runs/README.md'
    if ($LASTEXITCODE -eq 0) { throw 'Task 3 README is still ignored by Git' }
}
finally {
    Pop-Location
}

Write-Output 'TASK162_REVIEW_RELIABILITY_TASK3_RUN_README PASS'
