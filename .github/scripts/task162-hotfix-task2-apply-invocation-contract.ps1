$ErrorActionPreference = 'Stop'

$activeFiles = @(
    '.code-harness/AGENTS.md',
    '.code-harness/tools/README.md'
)
$activeFiles += Get-ChildItem '.code-harness/agents' -Filter '*.md' -File | ForEach-Object { $_.FullName }
$activeFiles += Get-ChildItem '.code-harness/skills' -Filter 'SKILL.md' -File -Recurse | ForEach-Object { $_.FullName }

$legacyInvocation = '(?<![A-Za-z0-9_.-])codea-harness-tools(?:\.exe)?(?=\s+(?:upgrade|validate|workspace|nav|db|chain|analysis|review|report|seal-apply|apply)\b)'
foreach ($file in $activeFiles) {
    $content = [System.IO.File]::ReadAllText($file)
    $updated = [regex]::Replace($content, $legacyInvocation, 'codea-dcep-tools.exe')
    if ($updated -ne $content) {
        [System.IO.File]::WriteAllText($file, $updated)
    }
}

$mainPath = '.code-harness/tools-runtime/cmd/codea-dcep-tools/main.go'
$main = [System.IO.File]::ReadAllText($mainPath)
$oldUsage = 'usage: codea-harness-tools <upgrade|validate|workspace|nav|db|chain|analysis|review|report|seal-apply|apply>'
$newUsage = 'usage: codea-dcep-tools <upgrade|validate|workspace|nav|db|chain|analysis|review|report|seal-apply|apply>'
if (-not $main.Contains($oldUsage) -and -not $main.Contains($newUsage)) {
    throw 'Runtime zero-arg usage contract not found'
}
$main = $main.Replace($oldUsage, $newUsage)
[System.IO.File]::WriteAllText($mainPath, $main)

$sharedSection = @'

## 1.6.2 Task 2 Agent → Runtime Invocation Contract

Agent/Orchestrator 写入 `requests/**` 后，Controlled Runtime 必须先按对应 machine-readable request contract 校验，再进入 strict decode 与业务处理：

- `analysis snapshot` → `change-set-request.schema.json`
- `analysis inventory` → `analysis-inventory-request.schema.json`
- `analysis certify` → `analysis-certify-request.schema.json`
- `review options` → `review-options-request.schema.json`

Active Agent 只能调用 `.code-harness/bin/codea-dcep-tools.exe`；`codea-harness-tools` 仅可作为 Go module/import 的历史内部名称存在，不是可执行文件调用名。

`review options` 的 Agent-facing request 固定为：

```json
{
  "runId": "<runId>",
  "changeAnalysisPath": ".code-harness/runs/<runId>/analysis/change-analysis.json"
}
```

需要显式 target 时只允许额外加入可选 `target`。`baseRef`、`requestedBaseRef`、Snapshot identity 以及其他 Git fact 均不是 `review options` request 字段；`baseRef` 只在 `analysis snapshot` / retained legacy analysis request 的既定 Contract 中出现。Unknown field 必须由 request schema fail closed。

`analysis certify` 的 Active Agent 形态仍固定为 canonical request：`runId / snapshotPath / snapshotSha256 / proposalPath / intent`。Schema 中保留的 legacy certify shape 仅用于 Runtime upgrade compatibility，Active Agent 不得生成 legacy `draftPath/baseRef` 形态。
'@

foreach ($file in @('.code-harness/AGENTS.md', '.code-harness/tools/README.md', '.code-harness/agents/orchestrator.md')) {
    $content = [System.IO.File]::ReadAllText($file)
    if (-not $content.Contains('## 1.6.2 Task 2 Agent → Runtime Invocation Contract')) {
        [System.IO.File]::WriteAllText($file, $content.TrimEnd() + $sharedSection + "`n")
    }
}
