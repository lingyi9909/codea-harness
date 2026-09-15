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

## 1.6.6 主 Agent Review 流程

`harness review` 全程由当前主 Agent 编排，并在同一主会话内执行 `analyze-change` 和 `review-code` 的语义工作。本文此前的 Reviewer.analyze-change / Reviewer.review-code 是评审职责名称，不代表必须派生子 Agent。不得调用 task/generic subagent/harness-review-reviewer 来替代本次主 Agent 评审。

固定顺序：`review begin → analysis snapshot → 主 Agent analyze-change → analysis certify → review options → review select → review units → review dispatch → 主 Agent review-code → review certify-findings → report review`。

主 Agent 使用已经安装的 `codea-reviewer-submit` 工具提交语义 JSON，kind 分别为 `change-analysis` 和 `findings`，runId 必须来自本次 `review begin`。工具名称为升级兼容保留，现支持主 Agent；不得因为名称包含 reviewer 就派生子会话。工具自动写入无 BOM proposal 和 Host receipt；主 Agent 不得手工补造 receipt。Runtime 验证当前项目的主会话、对应 assistant message 和 completed submission，把主会话 ID 绑定到分析证书；后续人工选择和 Findings 必须来自该同一主会话，再执行 Snapshot、证据、Coverage、规则及范围认证。

每次 Runtime 调用后用 `review progress --run-id <runId>` 展示 `events[].display`。没有证据的问题不生成 Finding；空 Findings 也必须走 certification 和正式 report。工具未加载或 Host export 失败时，明确报告安装/会话问题；不能假报成功或改写 Runtime artifact。

### 多调用链必须人工选择

`USER_SELECTION` 时完整原样展示 Runtime 返回的 `selectionPrompt`（全部 C1..Cn、调用链及 exact 路径、当前 runId 和完整 optionsHash），然后立即结束当前 Assistant Turn。明确提示下一条用户消息使用 `选择 C1`、`选择 C1,C2`、`全部`（仅 FULL intent）或 `仅列出`。显式 Controller target 的“全部”应提示用户使用列出的全部 selectionIds，例如 `选择 C1,C2`，仍生成 TARGETED。

收到下一条明确用户回复后，用 `codea-reviewer-submit` 的 kind=`selection` 提交标准 ReviewSelectionRequest JSON（runId/mode/optionsHash/selectionIds），然后以生成的 `requests/review-selection.json` 调用 Runtime `review select`。Runtime 会核验当前主会话中菜单之后真实的用户文本、completed tool call 和当前 optionsHash。不得由 Agent 猜测、默认 ALL、复用旧菜单或手工伪造人工选择。LIST 只列出调用链，不能继续 Findings 或 report。

AUTO_FULL/AUTO_SINGLE 保持自动选择，可直接写 request 调用 `review select`。正式编排先形成 Runtime scope，再执行 units/dispatch；多调用链缺少 scope 绝不能默认为 FULL。

请求文件统一 UTF-8 无 BOM；Runtime 兼容 Windows 工具写入的单个 UTF-8 BOM，但 malformed JSON、UTF-16、重复 BOM、未知字段及篡改 artifact 仍被拒绝。

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

`review progress` 是严格只读接口。不得向 Agent、Reviewer 或 prompt 暴露任何 progress advance/complete 或任意 fail mutation 命令；阶段推进只能发生在 snapshot、Reviewer authority verification、certification、planning/dispatch、finding certification、report publication 等既有 Runtime-owned 成功/失败边界内部。保留的 `review reviewer-unavailable` 是 failure-only 兼容信号，不能推进成功阶段。

---

`bootstrap.md` 是用户第一次接入 Codea Harness 时唯一需要主动指定读取的文件。后续所有操作（`harness review`、`harness test` 等）由 Orchestrator 按 `.code-harness/agents/orchestrator.md` 中的路由自动执行；涉及 Reviewer semantic phase 时，必须同时受上述 1.6.6 主 Agent 提交约定 和 Runtime Review Progress Gate 覆盖。
