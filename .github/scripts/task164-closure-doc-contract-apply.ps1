$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$utf8 = [Text.UTF8Encoding]::new($false)

function Patch-TextFile([string]$RelativePath, [hashtable]$Replacements, [string]$AppendBlock) {
    $path = Join-Path $repoRoot $RelativePath
    if (-not (Test-Path $path -PathType Leaf)) { throw "missing file: $RelativePath" }
    $text = [IO.File]::ReadAllText($path)
    if ($text.Contains('## 1.6.4 Review Host Authority Flow')) {
        return
    }
    foreach ($entry in $Replacements.GetEnumerator()) {
        $old = [string]$entry.Key
        $new = [string]$entry.Value
        $count = ([regex]::Matches($text, [regex]::Escape($old))).Count
        if ($count -ne 1) { throw "expected exactly one patch anchor in ${RelativePath}; count=${count}; anchor=$old" }
        $text = $text.Replace($old, $new)
    }
    $text = $text.TrimEnd("`r","`n") + "`n`n" + $AppendBlock.Trim() + "`n"
    [IO.File]::WriteAllText($path, $text, $utf8)
}

function Patch-ReadmeInstall {
    $path = Join-Path $repoRoot 'README.md'
    $text = [IO.File]::ReadAllText($path)
    if ($text.Contains('codea-harness-1.6.4-windows-x64-install.zip') -and $text.Contains('INSTALL_EXISTING_OPENCODE_CONFLICT')) { return }
    $pattern = '(?s)## 首次安装\r?\n.*?\r?\n## 版本升级'
    $matches = [regex]::Matches($text, $pattern)
    if ($matches.Count -ne 1) { throw "README first-install section match count=$($matches.Count)" }
    $replacement = @'
## 首次安装

1.6.4 首次安装必须使用正式 Release 产物：

```text
codea-harness-1.6.4-windows-x64-install.zip
```

> ⚠ **不要使用 GitHub Source ZIP / `Code → Download ZIP` / `git clone` 目录替代 Release package。** Source 不含正式 Windows Runtime，也不是可安装产品。

解压正式 install ZIP 后，package root 必须同时包含：

```text
install.ps1
.code-harness/
.opencode/agents/reviewer.md
.opencode/commands/harness-review-reviewer.md
.opencode/tools/codea-reviewer-submit.ts
```

**不要只复制 `.code-harness/`。** 1.6.4 Review 的独立 Reviewer Host 还依赖 package root 中的 `.opencode/**` 三个受管资源。

从解压后的 package root 执行：

```powershell
pwsh -NoProfile -File .\install.ps1 -ProjectRoot <项目根目录>
```

安装器先做完整 preflight，再一次性安装 `.code-harness/` 和缺失的 Reviewer Host 资源。1.6.4 的 OpenCode Host contract 固定按 `opencode-ai@1.18.25` 认证；首次安装与正式 Final Certification 都以该版本为兼容基线。

如果项目根目录已经存在以下任一文件：

```text
.opencode/agents/reviewer.md
.opencode/commands/harness-review-reviewer.md
.opencode/tools/codea-reviewer-submit.ts
```

且内容不是 Codea package 中的 exact bytes，安装器必须 fail-closed：

```text
MANUAL_ACTION_REQUIRED
INSTALL_EXISTING_OPENCODE_CONFLICT
0 destructive overwrite
```

不会覆盖用户已有 Host 配置，也不会先写入 `.code-harness/` 或其他 Reviewer Host 文件。请先人工确认/迁移冲突文件，再重新执行安装。若目标已存在 `.code-harness/`，不要用首次安装包覆盖，应走正式 upgrade package。

安装完成后，对工程 Agent 说：

```text
读取 .code-harness/bootstrap.md，执行 harness init
```

初始化或后续使用生成的本机 Project State 包括：

```text
.code-harness/harness.yaml
.code-harness/project.md
.code-harness/database.yaml
.code-harness/runs/**
.code-harness/chains/**
```

正式 install/upgrade ZIP 不包含任何上述 Project State 实例，不会预置业务 `chains/*.yaml`。

## 版本升级
'@
    $text = [regex]::Replace($text, $pattern, $replacement, 1)
    [IO.File]::WriteAllText($path, $text, $utf8)
}

$canonical = @'
## 1.6.4 Review Host Authority Flow

The only supported product-level review ownership is:

```text
Main Agent / Orchestrator
→ review begin
→ Runtime snapshot
→ independent Reviewer CHANGE_ANALYSIS
→ Runtime certification
→ Runtime planning
→ independent Reviewer FINDINGS
→ Runtime finding certification
→ Runtime report
```

Reviewer owns only the two semantic proposal phases: `CHANGE_ANALYSIS` and `FINDINGS`. Reviewer does not own routing, Runtime execution, certification, planning, progress, final report rendering, or failure recovery.

Main Agent / Orchestrator owns routing, Runtime invocation, Reviewer delegation, Runtime progress rendering, and fail-closed handling. It must use the official Runtime commands `review progress --run-id <runId>` to render `events[].display` and `review reviewer-unavailable --run-id <runId>` when the independent Reviewer Host cannot produce a valid same-run proposal.

The Main Agent / Orchestrator may create only same-run `requests/**` request files. Reviewer proposals must enter the same run only through `codea-reviewer-submit`. `analysis/**`, `review.md`, and `.code-harness/chains/**` remain Runtime/Framework-owned. No semantic fallback to the Main Agent is permitted when Reviewer fails.

OpenCode Host compatibility for 1.6.4 is certified against `opencode-ai@1.18.25`. The resolved Reviewer Host must be a subagent whose effective permissions deny `bash`, `task`, and generic edit/write authority while allowing the dedicated `codea-reviewer-submit` tool; the Reviewer command must resolve to `agent=reviewer` and `subtask=true`.
'@

$agentsReviewBegin = 'codea-dcep-tools.exe review begin'
$agentsReviewBeginNew = "codea-dcep-tools.exe review begin`ncodea-dcep-tools.exe review progress --run-id <runId>`ncodea-dcep-tools.exe review reviewer-unavailable --run-id <runId>"
Patch-TextFile '.code-harness/AGENTS.md' @{
    $agentsReviewBegin = $agentsReviewBeginNew
    '- Reviewer：消费 Runtime Canonical ChangeSet Snapshot，负责 Code Navigation、semantic ChangeAnalysis Proposal、Review Coverage 与 Finding Proposal；不拥有 Git ChangeSet deterministic fact authority。' = '- Reviewer：消费 Runtime Canonical ChangeSet Snapshot，只负责 Code Navigation、semantic ChangeAnalysis Proposal、Review Coverage 与 Finding Proposal；不拥有 Git ChangeSet deterministic fact authority，也不拥有整个 Review orchestration。'
    '- Orchestrator：路由、触发 Runtime Snapshot/Certification、Review Coverage/审批门禁、API target selection、Chain Management、Agent 交接、测试修复轮次；不得独立重算 Git ChangeSet。' = '- Orchestrator：拥有 Review 路由、Runtime 调用、Reviewer delegation、Runtime progress 展示与 fail-closed 处理，并继续负责 Review Coverage/审批门禁、API target selection、Chain Management、Agent 交接、测试修复轮次；不得独立重算 Git ChangeSet。'
} $canonical

Patch-TextFile '.code-harness/bootstrap.md' @{} $canonical

Patch-TextFile '.code-harness/agents/orchestrator.md' @{
    '| `harness review` | Reviewer | 否 |' = '| `harness review` | Orchestrator → Runtime + independent Reviewer phases | 否 |'
    '| `harness review list` | Reviewer（LIST） | 否 |' = '| `harness review list` | Orchestrator → Runtime + independent Reviewer phases（LIST） | 否 |'
    '| `harness review <Class>` | Reviewer（TARGETED CLASS） | 否 |' = '| `harness review <Class>` | Orchestrator → Runtime + independent Reviewer phases（TARGETED CLASS） | 否 |'
    '| `harness review <Class.method>` | Reviewer（TARGETED METHOD） | 否 |' = '| `harness review <Class.method>` | Orchestrator → Runtime + independent Reviewer phases（TARGETED METHOD） | 否 |'
} $canonical

Patch-TextFile '.code-harness/contracts/reviewer-host-contract.md' @{} $canonical
Patch-ReadmeInstall

Write-Output 'TASK164_CLOSURE_DOC_PATCH_APPLIED PASS'
Write-Output 'TASK164_CLOSURE_README_INSTALL_UX PASS'
