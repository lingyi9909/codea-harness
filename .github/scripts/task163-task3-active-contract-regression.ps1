$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$required = @{
  '.code-harness/AGENTS.md' = @(
    'TASK163_USER_SELECTION_TURN_HARD_STOP',
    'USER_SELECTION',
    'review select',
    '下一条用户消息'
  )
  '.code-harness/agents/orchestrator.md' = @(
    'TASK163_USER_SELECTION_TURN_HARD_STOP',
    '立即结束当前 Assistant Turn',
    'review select',
    'review units',
    'review dispatch',
    '下一条用户消息'
  )
  '.code-harness/agents/reviewer.md' = @(
    'TASK163_USER_SELECTION_TURN_HARD_STOP',
    'USER_SELECTION',
    '不得进入 Finding Proposal'
  )
  '.code-harness/skills/review-code/SKILL.md' = @(
    'TASK163_USER_SELECTION_TURN_HARD_STOP',
    'USER_SELECTION',
    '不得执行本 Skill'
  )
}

foreach ($path in $required.Keys) {
  if (!(Test-Path $path -PathType Leaf)) { throw "missing active contract: $path" }
  $text = Get-Content -Raw $path
  foreach ($needle in $required[$path]) {
    if (-not $text.Contains($needle)) {
      throw "TASK163_ACTIVE_CONTRACT_HARD_STOP_MISSING path=$path needle=$needle"
    }
  }
}

$orchestrator = Get-Content -Raw '.code-harness/agents/orchestrator.md'
$hardStop = $orchestrator.IndexOf('TASK163_USER_SELECTION_TURN_HARD_STOP')
if ($hardStop -lt 0) { throw 'Task 3 hard-stop section missing' }
$section = $orchestrator.Substring($hardStop)
foreach ($forbidden in @('review select','review units','review dispatch','finding-proposals','report review')) {
  if (-not $section.Contains($forbidden)) { throw "Task 3 hard-stop section does not name forbidden same-turn action: $forbidden" }
}
foreach ($requiredPhrase in @('立即结束当前 Assistant Turn','下一条用户消息','不得自动构造 FULL','不得自动构造 TARGETED')) {
  if (-not $section.Contains($requiredPhrase)) { throw "Task 3 hard-stop section missing phrase: $requiredPhrase" }
}

Write-Output 'MULTI_CHAIN_REVIEW_REQUIRES_USER_SELECTION CONTRACT PASS'
Write-Output 'MULTI_CHAIN_NO_SELECTION_NO_REVIEW_UNITS CONTRACT PASS'
Write-Output 'TASK163_TASK3_ACTIVE_CONTRACT_HARD_STOP PASS'
