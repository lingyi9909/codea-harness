from pathlib import Path
import subprocess


def write(path: str, text: str) -> None:
    Path(path).write_text(text, encoding="utf-8", newline="\n")


runtime = Path('.code-harness/tools-runtime/cmd/codea-dcep-tools/review_precision_command.go')
text = runtime.read_text(encoding='utf-8')
if 'func runReviewBegin162' not in text:
    old = '\t"bytes"\n\t"crypto/sha256"'
    new = '\t"bytes"\n\t"crypto/rand"\n\t"crypto/sha256"\n\t"encoding/hex"'
    if old not in text:
        raise RuntimeError('runtime import anchor missing')
    text = text.replace(old, new, 1)
    old = '\t\tcase "units":'
    new = '\t\tcase "begin":\n\t\t\treturn runReviewBegin162(args[1:])\n\t\tcase "units":'
    if old not in text:
        raise RuntimeError('runtime begin switch anchor missing')
    text = text.replace(old, new, 1)
    anchor = 'func runReviewUnits160(args []string) error {'
    begin = '''func runReviewBegin162(args []string) error {
\tif len(args) != 0 {
\t\treturn errors.New("review begin takes no arguments")
\t}
\trunsRoot := filepath.Join(".code-harness", "runs")
\tif err := os.MkdirAll(runsRoot, 0o755); err != nil {
\t\treturn fmt.Errorf("REVIEW_BEGIN_RUNS_DIR_FAILED: %w", err)
\t}
\tfor attempt := 0; attempt < 32; attempt++ {
\t\tentropy := make([]byte, 16)
\t\tif _, err := rand.Read(entropy); err != nil {
\t\t\treturn fmt.Errorf("REVIEW_BEGIN_RANDOM_FAILED: %w", err)
\t\t}
\t\trunID := "review-" + hex.EncodeToString(entropy)
\t\trunPath := filepath.Join(runsRoot, runID)
\t\tif err := os.Mkdir(runPath, 0o755); err != nil {
\t\t\tif errors.Is(err, os.ErrExist) {
\t\t\t\tcontinue
\t\t\t}
\t\t\treturn fmt.Errorf("REVIEW_BEGIN_RUN_DIR_FAILED: %w", err)
\t\t}
\t\treturn writeJSONAndStatus(map[string]any{
\t\t\t"status":  "READY",
\t\t\t"runId":   runID,
\t\t\t"runPath": filepath.ToSlash(runPath),
\t\t}, true)
\t}
\treturn errors.New("REVIEW_BEGIN_RUN_ID_EXHAUSTED")
}

'''
    if anchor not in text:
        raise RuntimeError('review begin function anchor missing')
    text = text.replace(anchor, begin + anchor, 1)
    write(str(runtime), text)

agents = Path('.code-harness/AGENTS.md')
text = agents.read_text(encoding='utf-8')
command = 'codea-dcep-tools.exe review begin'
if command not in text:
    anchor = 'codea-dcep-tools.exe analysis snapshot --input .code-harness/runs/<runId>/requests/<file>.json'
    if anchor not in text:
        raise RuntimeError('AGENTS command anchor missing')
    text = text.replace(anchor, command + '\n' + anchor, 1)
if 'Reliability Hotfix — Fresh Review Lifecycle' not in text:
    text += '''

## 1.6.2 Reliability Hotfix — Fresh Review Lifecycle

每一次新的顶层 `harness review` 都是独立 Review invocation，必须先调用：

```text
codea-dcep-tools.exe review begin
```

固定生命周期是：`review begin` → fresh runId → `analysis snapshot` → 当前 run 的正常 Review Authority Chain。

`review begin` 只负责由 Runtime 生成唯一 fresh runId 并创建 `.code-harness/runs/<runId>/`；它不得读取 Git、不得计算 ChangeSet、不得生成 `analysis/change-set.json`。`analysis snapshot` 仍是唯一 Git ChangeSet Authority。

`same-run` 只约束单次 Review invocation 内部；下一次用户再次输入 `harness review` 时，上一轮 runId、上一轮 Snapshot、上一轮 ChangeAnalysis、上一轮 0 Change 结论和上一轮 `review.md` 对新 invocation 不具备 Authority。Agent session memory 不得替代本次 `review begin` + `analysis snapshot`。
'''
write(str(agents), text)

tools = Path('.code-harness/tools/README.md')
text = tools.read_text(encoding='utf-8')
if command not in text:
    anchor = 'codea-dcep-tools.exe analysis snapshot --input .code-harness/runs/<runId>/requests/<file>.json'
    if anchor not in text:
        raise RuntimeError('tools README command anchor missing')
    text = text.replace(anchor, command + '\n' + anchor, 1)
if '### Fresh Review Lifecycle' not in text:
    text += '''

### Fresh Review Lifecycle

每一次新的顶层 `harness review` 都必须先执行 `codea-dcep-tools.exe review begin` 获取 Runtime-owned fresh runId，再用该 runId 创建 Snapshot request 并执行 `analysis snapshot`。旧 run 的 runId、Snapshot、ChangeAnalysis、0 Change 结论和 `review.md` 不能作为新 invocation 的输入事实。

`review begin` 只创建 `.code-harness/runs/<runId>/` 并返回 runId；不读取 Git、不计算 ChangeSet、不创建任何 Authority artifact。Git ChangeSet 仍只能由随后执行的 `analysis snapshot` 计算。
'''
write(str(tools), text)

orchestrator = Path('.code-harness/agents/orchestrator.md')
text = orchestrator.read_text(encoding='utf-8')
if '## 1.6.2 Reliability Hotfix Fresh Review Lifecycle' not in text:
    anchor = '## `harness review`（1.5.3 ReviewOptions）'
    if anchor not in text:
        raise RuntimeError('orchestrator plain review anchor missing')
    section = '''## 1.6.2 Reliability Hotfix Fresh Review Lifecycle

每一次新的顶层 `harness review` 必须视为新的 Review invocation，并先执行 `codea-dcep-tools.exe review begin`。固定入口顺序：`review begin` → fresh runId → `analysis snapshot`。

硬规则：

- `review begin` 只生成 Runtime-owned fresh runId 并创建对应 run directory；不得读取 Git 或生成 ChangeSet。
- `analysis snapshot` 是本流程唯一 Git ChangeSet Authority；每一个 fresh run 都必须重新执行，不能由会话记忆推断“代码没变”。
- `same-run` 只约束单次 Review invocation 内部；Snapshot、Certified ChangeAnalysis、Review Scope、Review Units、Certified Findings 与 `review.md` 必须绑定当前 fresh runId。
- 用户再次输入 `harness review` 时，上一轮 runId、上一轮 Snapshot、上一轮 ChangeAnalysis、上一轮 0 Change 结论、上一轮 `review.md` 对新 invocation 不具备 Authority。
- 不需要用户补充“代码已经变化”；当前代码是否变化只由新 run 的 Runtime `analysis snapshot` 判断。

'''
    text = text.replace(anchor, section + anchor, 1)
old_plain = '''1. 解析 effective baseRef / includeWorkingTree，仅形成 Snapshot request 参数
2. Runtime `analysis snapshot` → same-run `analysis/change-set.json`
'''
new_plain = '''1. 每一次新的顶层 `harness review` 先调用 Runtime `review begin`，创建 fresh runId；不得复用任何 previous run
2. 解析 effective baseRef / includeWorkingTree，仅形成当前 fresh run 的 Snapshot request 参数
3. Runtime `analysis snapshot` → 当前 fresh run 的 `analysis/change-set.json`
'''
if old_plain in text:
    text = text.replace(old_plain, new_plain, 1)
old_target = '''1. 解析 effective baseRef/includeWorkingTree
2. Runtime analysis snapshot 建立完整 Canonical ChangeSet
'''
new_target = '''1. 新的顶层 `harness review <Class|Class.method>` 先调用 Runtime `review begin` 创建 fresh runId
2. 解析 effective baseRef/includeWorkingTree，并用 fresh runId 调用 Runtime analysis snapshot 建立完整 Canonical ChangeSet
'''
if old_target in text:
    text = text.replace(old_target, new_target, 1)
write(str(orchestrator), text)

subprocess.run(['gofmt', '-w', 'cmd/codea-dcep-tools/review_precision_command.go', 'cmd/codea-dcep-tools/review_reliability_task2_fresh_lifecycle_test.go'], cwd='.code-harness/tools-runtime', check=True)
subprocess.run(['go', 'test', '-count=1', '-run', 'Test162ReviewReliabilityTask2Begin', '-v', './cmd/codea-dcep-tools'], cwd='.code-harness/tools-runtime', check=True)
subprocess.run(['pwsh', '-File', './.github/scripts/task162-review-reliability-task2-contract-regression.ps1'], check=True)
print('TASK162_REVIEW_RELIABILITY_TASK2_MINIMAL_GREEN PASS')
