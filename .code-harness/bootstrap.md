# Codea Harness 初始化入口

当用户要求执行 `harness init` 时：

1. 读取 `.code-harness/AGENTS.md`，了解 Harness 通用规则和安全约束。
2. 读取 `.code-harness/agents/orchestrator.md`，了解完整意图路由。
3. 由 Orchestrator 调用 Project Adapter（`.code-harness/agents/project-adapter.md`）。
4. Project Adapter 使用 `adapt-project` Skill（`.code-harness/skills/adapt-project/SKILL.md`）分析目标项目。
5. 根据识别结果自动生成：
   - `harness.yaml`——项目可执行配置
   - `project.md`——项目适配信息
6. 输出初始化结果摘要。如果存在无法从项目文件中确定的信息，列出未确定项。
7. **只询问无法从项目文件中确定的信息**。能从代码中自动识别的内容必须自动填写，不得询问用户。
8. 不得修改业务代码、测试代码、`pom.xml` 或 `application` 配置文件。
9. 未经用户明确同意，不得修改目标项目根目录的 `AGENTS.md`。

## OpenCode Reviewer Host Gate（1.6.4）

首次接入后，所有 `harness review` 语义阶段还必须读取：

```text
.code-harness/contracts/reviewer-host-contract.md
```

正式 release 必须把 canonical Reviewer 注册为项目级 OpenCode subagent：

```text
.opencode/agents/reviewer.md
mode: subagent
```

`harness review` 的 semantic analysis 与 finding proposal 必须由该独立 Reviewer child session 执行；Main Agent / Orchestrator 只负责路由、Runtime 调用和门禁，不得代替 Reviewer 做 semantic review。

`.code-harness/agents/orchestrator.md` 中已有的 `Reviewer.analyze-change` / `Reviewer.review-code` 仅是 semantic shorthand。1.6.4 支持的 OpenCode 产品路径必须把这两个语义阶段绑定到项目级 `harness-review-reviewer` command，由它委派 `independent reviewer subagent child session`；不得把 shorthand 解释为 Main Agent / Orchestrator 本地执行 Reviewer 语义工作。

固定 Host binding：

```text
CHANGE_ANALYSIS
-> harness-review-reviewer
-> independent reviewer child session
-> codea-reviewer-submit
-> requests/change-analysis-proposal.json + Reviewer Host authority receipt
-> Runtime analysis certify

FINDINGS
-> harness-review-reviewer
-> independent reviewer child session
-> codea-reviewer-submit
-> requests/finding-proposals.json + Reviewer Host authority receipt
-> Runtime review certify-findings
```

如果 Reviewer 无法 resolve/start/invoke/complete，child session crash/cancel，或者没有产出可供 Runtime 校验的有效 proposal，OpenCode Host 必须先调用唯一的 failure-only Runtime 信号：

```text
codea-dcep-tools.exe review reviewer-unavailable --run-id <runId>
```

该命令不接受 `--stage`、自定义 failure code、advance 或 complete 参数。Runtime 必须自行读取当前 review progress，并且只允许当前 `CHANGE_ANALYSIS` 或 `REVIEW_EXECUTION` 失败为 `REVIEWER_UNAVAILABLE`；它不能产生 PASS、不能推进阶段、不能修改其他阶段。

命令返回后固定输出并立即停止：

```text
REVIEWER_UNAVAILABLE
MANUAL_ACTION_REQUIRED
HARD STOP
```

Host failure 上报后不得继续 Runtime `analysis certify` 或 `review certify-findings`，也不得继续 review planning/selection/units/dispatch、finding certification 或 report publication；不得由 Main Agent / Orchestrator 生成 semantic proposal 作为 fallback。可再次调用只读 `review progress --run-id <runId>` 展示 Runtime 已记录的失败事件，但不得尝试恢复或跳过失败阶段。

## Runtime Review Progress Gate（1.6.4 Task 3）

`harness review` 的阶段状态只能由 Controlled Runtime 的 8-stage review state machine 决定。OpenCode / Main Agent / Orchestrator 只负责展示状态，不能选择、跳过、补写或宣告阶段结果。

在 `review begin` 获得同一 `runId` 后，以及每个 Runtime/Reviewer 阶段返回后，使用只读命令读取当前状态：

```text
codea-dcep-tools.exe review progress --run-id <runId>
```

OpenCode 只展示该命令返回的 Runtime-derived `events[].display`，并据此告诉用户当前阶段、已完成阶段或阻断阶段。不得自行生成或宣告阶段 PASS/FAIL/RUNNING；prompt 文本、Reviewer 输出、Main Agent 叙述均不是 progress authority。

固定 authority 规则：

```text
Runtime state/event -> 可展示、可决定是否继续
Agent/Reviewer prompt text -> 仅语义内容，不得改变 progress
Reviewer proposal -> 只有 Reviewer Host receipt + session attestation 被 Runtime 验证后，才允许推进对应 Reviewer stage
Runtime command failure -> 当前 stage FAILED，后续 stage BLOCKED
Runtime terminal success -> REPORT SUCCEEDED
```

`review progress` 是严格只读接口。不得向 Agent、Reviewer 或 prompt 暴露任何 progress advance/complete 或任意 fail mutation 命令；阶段推进只能发生在 snapshot、Reviewer authority verification、certification、planning/dispatch、finding certification、report publication 等既有 Runtime-owned 成功/失败边界内部。唯一例外是上述 `review reviewer-unavailable` failure-only Host 信号：它没有成功权威，只能要求 Runtime 对当前 Reviewer-dependent stage 执行 fail-closed。

---

`bootstrap.md` 是用户第一次接入 Codea Harness 时唯一需要主动指定读取的文件。后续所有操作（`harness review`、`harness test` 等）由 Orchestrator 按 `.code-harness/agents/orchestrator.md` 中的路由自动执行；涉及 Reviewer semantic phase 时，必须同时受上述 1.6.4 Host binding 和 Runtime Review Progress Gate 覆盖。

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
