$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Replace-Once([string]$Path, [string]$Old, [string]$New) {
    $text = Get-Content -Raw $Path
    $count = ([regex]::Matches($text, [regex]::Escape($Old))).Count
    if ($count -ne 1) { throw "Expected exactly one anchor in $Path, found $count: $Old" }
    [IO.File]::WriteAllText($Path, $text.Replace($Old, $New), [Text.UTF8Encoding]::new($false))
}

$runtime = '.code-harness/tools-runtime/cmd/codea-dcep-tools/review_precision_command.go'
$text = Get-Content -Raw $runtime
if ($text -notmatch 'func runReviewBegin162') {
    $text = $text.Replace('"bytes"' + "`n" + "`t" + '"crypto/sha256"', '"bytes"' + "`n" + "`t" + '"crypto/rand"' + "`n" + "`t" + '"crypto/sha256"' + "`n" + "`t" + '"encoding/hex"')
    $text = $text.Replace("`t`tcase \"units\":", "`t`tcase \"begin\":`n`t`t`treturn runReviewBegin162(args[1:])`n`t`tcase \"units\":")
    $anchor = 'func runReviewUnits160(args []string) error {'
    $begin = @'
func runReviewBegin162(args []string) error {
	if len(args) != 0 {
		return errors.New("review begin takes no arguments")
	}
	runsRoot := filepath.Join(".code-harness", "runs")
	if err := os.MkdirAll(runsRoot, 0o755); err != nil {
		return fmt.Errorf("REVIEW_BEGIN_RUNS_DIR_FAILED: %w", err)
	}
	for attempt := 0; attempt < 32; attempt++ {
		entropy := make([]byte, 16)
		if _, err := rand.Read(entropy); err != nil {
			return fmt.Errorf("REVIEW_BEGIN_RANDOM_FAILED: %w", err)
		}
		runID := "review-" + hex.EncodeToString(entropy)
		runPath := filepath.Join(runsRoot, runID)
		if err := os.Mkdir(runPath, 0o755); err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return fmt.Errorf("REVIEW_BEGIN_RUN_DIR_FAILED: %w", err)
		}
		return writeJSONAndStatus(map[string]any{
			"status":  "READY",
			"runId":   runID,
			"runPath": filepath.ToSlash(runPath),
		}, true)
	}
	return errors.New("REVIEW_BEGIN_RUN_ID_EXHAUSTED")
}

'@
    if (-not $text.Contains($anchor)) { throw 'review begin insertion anchor missing' }
    $text = $text.Replace($anchor, $begin + $anchor)
    [IO.File]::WriteAllText($runtime, $text, [Text.UTF8Encoding]::new($false))
}

$agents = '.code-harness/AGENTS.md'
$text = Get-Content -Raw $agents
if ($text -notmatch [regex]::Escape('codea-dcep-tools.exe review begin')) {
    $anchor = 'codea-dcep-tools.exe analysis snapshot --input .code-harness/runs/<runId>/requests/<file>.json'
    if (-not $text.Contains($anchor)) { throw 'AGENTS Runtime command anchor missing' }
    $text = $text.Replace($anchor, "codea-dcep-tools.exe review begin`n$anchor")
}
if ($text -notmatch 'Reliability Hotfix — Fresh Review Lifecycle') {
    $text += @'

## 1.6.2 Reliability Hotfix — Fresh Review Lifecycle

每一次新的顶层 `harness review` 都是独立 Review invocation，必须先调用：

```text
codea-dcep-tools.exe review begin
```

固定生命周期是：`review begin` → fresh runId → `analysis snapshot` → 当前 run 的正常 Review Authority Chain。

`review begin` 只负责由 Runtime 生成唯一 fresh runId 并创建 `.code-harness/runs/<runId>/`；它不得读取 Git、不得计算 ChangeSet、不得生成 `analysis/change-set.json`。`analysis snapshot` 仍是唯一 Git ChangeSet Authority。

`same-run` 只约束单次 Review invocation 内部；下一次用户再次输入 `harness review` 时，上一轮 runId、上一轮 Snapshot、上一轮 ChangeAnalysis、上一轮 0 Change 结论和上一轮 `review.md` 对新 invocation 不具备 Authority。Agent session memory 不得替代本次 `review begin` + `analysis snapshot`。
'@
}
[IO.File]::WriteAllText($agents, $text, [Text.UTF8Encoding]::new($false))

$tools = '.code-harness/tools/README.md'
$text = Get-Content -Raw $tools
if ($text -notmatch [regex]::Escape('codea-dcep-tools.exe review begin')) {
    $anchor = 'codea-dcep-tools.exe analysis snapshot --input .code-harness/runs/<runId>/requests/<file>.json'
    if (-not $text.Contains($anchor)) { throw 'tools README Runtime command anchor missing' }
    $text = $text.Replace($anchor, "codea-dcep-tools.exe review begin`n$anchor")
}
if ($text -notmatch 'Fresh Review Lifecycle') {
    $text += @'

### Fresh Review Lifecycle

每一次新的顶层 `harness review` 都必须先执行 `codea-dcep-tools.exe review begin` 获取 Runtime-owned fresh runId，再用该 runId 创建 Snapshot request 并执行 `analysis snapshot`。旧 run 的 runId、Snapshot、ChangeAnalysis、0 Change 结论和 `review.md` 不能作为新 invocation 的输入事实。

`review begin` 只创建 `.code-harness/runs/<runId>/` 并返回 runId；不读取 Git、不计算 ChangeSet、不创建任何 Authority artifact。Git ChangeSet 仍只能由随后执行的 `analysis snapshot` 计算。
'@
}
[IO.File]::WriteAllText($tools, $text, [Text.UTF8Encoding]::new($false))

$orchestrator = '.code-harness/agents/orchestrator.md'
$text = Get-Content -Raw $orchestrator
if ($text -notmatch 'Reliability Hotfix Fresh Review Lifecycle') {
    $anchor = '## `harness review`（1.5.3 ReviewOptions）'
    if (-not $text.Contains($anchor)) { throw 'orchestrator plain review anchor missing' }
    $section = @'
## 1.6.2 Reliability Hotfix Fresh Review Lifecycle

每一次新的顶层 `harness review` 必须视为新的 Review invocation，并先执行 `codea-dcep-tools.exe review begin`。固定入口顺序：`review begin` → fresh runId → `analysis snapshot`。

硬规则：

- `review begin` 只生成 Runtime-owned fresh runId 并创建对应 run directory；不得读取 Git 或生成 ChangeSet。
- `analysis snapshot` 是本流程唯一 Git ChangeSet Authority；每一个 fresh run 都必须重新执行，不能由会话记忆推断“代码没变”。
- `same-run` 只约束单次 Review invocation 内部；Snapshot、Certified ChangeAnalysis、Review Scope、Review Units、Certified Findings 与 `review.md` 必须绑定当前 fresh runId。
- 用户再次输入 `harness review` 时，上一轮 runId、上一轮 Snapshot、上一轮 ChangeAnalysis、上一轮 0 Change 结论、上一轮 `review.md` 对新 invocation 不具备 Authority。
- 不需要用户补充“代码已经变化”；当前代码是否变化只由新 run 的 Runtime `analysis snapshot` 判断。

'@
    $text = $text.Replace($anchor, $section + $anchor)
}
$oldPlain = @'
1. 解析 effective baseRef / includeWorkingTree，仅形成 Snapshot request 参数
2. Runtime `analysis snapshot` → same-run `analysis/change-set.json`
'@
$newPlain = @'
1. 每一次新的顶层 `harness review` 先调用 Runtime `review begin`，创建 fresh runId；不得复用任何 previous run
2. 解析 effective baseRef / includeWorkingTree，仅形成当前 fresh run 的 Snapshot request 参数
3. Runtime `analysis snapshot` → 当前 fresh run 的 `analysis/change-set.json`
'@
if ($text.Contains($oldPlain)) {
    $text = $text.Replace($oldPlain, $newPlain)
}
$oldTarget = @'
1. 解析 effective baseRef/includeWorkingTree
2. Runtime analysis snapshot 建立完整 Canonical ChangeSet
'@
$newTarget = @'
1. 新的顶层 `harness review <Class|Class.method>` 先调用 Runtime `review begin` 创建 fresh runId
2. 解析 effective baseRef/includeWorkingTree，并用 fresh runId 调用 Runtime analysis snapshot 建立完整 Canonical ChangeSet
'@
if ($text.Contains($oldTarget)) {
    $text = $text.Replace($oldTarget, $newTarget)
}
[IO.File]::WriteAllText($orchestrator, $text, [Text.UTF8Encoding]::new($false))

Push-Location '.code-harness/tools-runtime'
try {
    gofmt -w 'cmd/codea-dcep-tools/review_precision_command.go' 'cmd/codea-dcep-tools/review_reliability_task2_fresh_lifecycle_test.go'
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    go test -count=1 -run 'Test162ReviewReliabilityTask2Begin' -v ./cmd/codea-dcep-tools
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}
finally { Pop-Location }

./.github/scripts/task162-review-reliability-task2-contract-regression.ps1
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Output 'TASK162_REVIEW_RELIABILITY_TASK2_MINIMAL_GREEN PASS'
