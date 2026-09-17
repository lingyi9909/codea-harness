# Codea Harness 1.8 普通 Review 主路径

普通 `/harness-review` 只执行 1.8 report-first 协议：固定入口先运行 `review start` 创建并回读 INCOMPLETE 报告，然后主 Agent 只通过 `codea-review` 完成 `prepare → (必要时等待真实下一用户选择后 select) → finish`。

- `runId` 必须来自本次入口，禁止“取最新 run”。
- 用户 target 是普通文本/结构化 `intent.target`，不得拼入 shell。
- 多链必须展示当前 `optionsHash` 与完整链菜单并结束当前 assistant turn；只有下一条真实用户选择可授权 `select`。HostTurn 只来自 OpenCode tool context，模型参数不得填写 session/message/userConfirmed。
- 只有 prepare 自动得到完整单链，或 select 成功后的 `scope.reads`，才是允许读取/提交的范围。跨 scope、源码 hash/range 变化必须 fail closed。
- 主 Agent 自己完成语义 Review；普通 1.8 Review 不要求旧 Reviewer 子 Agent、Certified ChangeAnalysis、ReviewUnit、RuleDispatch 或旧 finding certification。
- findings 为空也必须调用 `codea-review action=finish`。只有 finish 返回 `execution=COMPLETE` 才能称“评审完成”。`coverage=PARTIAL` 时结论必须是 `UNDETERMINED`，报告保留所有已知 gap。
- 任一步失败或用户未选择时保留已经存在的 INCOMPLETE 报告，不自动重启整轮，不进入修复代码。

对普通 1.8 Review，以上合同是本文件唯一可执行 Review 协议。

## 历史 Review 协议与其他既有能力

下面保留的版本化内容用于旧 run / 升级兼容和非 Review 能力。**其中 1.7 及更早的 ordinary Review begin / Reviewer / certify / ReviewUnit / RuleDispatch / report-review 步骤全部是历史说明，不得用于新的 1.8 `/harness-review`。** Test、Debug、Fix、API Doc、Chain、Upgrade 等非 ordinary Review 规则若未被 1.8 设计修改，继续有效。

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

## 1.6.7 单类 Review 入口

优先使用 `/harness-review OrderController` 或 `/harness-review OrderController.method`。该命令先执行 Runtime `review begin`，将真实 `READY/runId` 注入当前主会话。收到已注入的 runId 时不得再次 begin。自然语言 `harness review <target>` 也必须先 begin，再开始语义分析；普通文本分析不能替代正式 Review。

单类评审先定位目标文件，再沿实际 Controller → Service → Mapper/Repository 读取直接依赖，以及有证据关联的 DTO、SQL、配置。复用本轮已取得的导航结果；不要反复全目录注解检索，也不要递归展开无关模块。Runtime 仍保留完整 Git Snapshot；FULL required coverage 和 TARGETED verified scoped coverage 各自照常认证，未读取或未解析的内容必须如实标记。

`PROPOSAL_PREFLIGHT_FAILED` 表示提交结构需要修正：根据错误位置校对 `chain` 与 `chainRefs` 的逐项 symbol/path/workspace，保留本次 runId 修正后重新提交。不得通过删除歧义引用、伪造证据或重启整轮掩盖错误。认证/导航超时、authority 失败、已 FAILED 的 run 应展示具体失败命令和 Runtime progress 后停止；不自动无限重试或重启。

发现超过一条实际调用链时，必须完整展示 Runtime 菜单并等待下一条真实用户选择。Findings 为空也要完成认证和 `report review`。只有同 run 的 `review.md` 存在且 Runtime `REPORT SUCCEEDED` 才能称完成；在此之前不得转去询问是否修复代码。

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
