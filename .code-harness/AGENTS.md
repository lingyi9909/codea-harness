# Codea Harness 项目指令

## Codea Harness 1.8 普通 Review 主路径

普通 `/harness-review` 只执行 1.8 report-first 协议。命令入口必须先固定执行 `review start`，在主 Agent 第一条模型回复前创建并回读本 run 的 INCOMPLETE `review.md`。随后主 Agent只通过结构化 `codea-review` 工具执行：

`prepare → （多链时等待下一条真实用户选择后 select） → finish`

### Review 不变量

- `runId` 只能来自本次命令入口，不得读取“最新 run”代替。
- 用户 target 作为普通文本和 `intent.target` 传递，不得插入 shell。
- `prepare` 返回完整单链时可继续；返回多链时必须展示当前 `optionsHash` 与完整菜单，并结束当前 assistant turn。
- `select` 只能消费菜单之后的下一条真实用户消息。Host session/message identity 由 OpenCode tool context 提供，模型参数不能伪造。
- `scope.reads` 是上下文读取边界；Runtime 另行维护所选链可形成正式 finding 的精确范围。能读到同文件的其他方法，不等于可以对其形成 finding。
- finding 必须有真实源码 evidence；Runtime 会重新核验 quote、source hash、所选链范围，以及 `introducedByChange` 与本 run 模式/真实 Git diff 的一致性。
- `CURRENT_IMPLEMENTATION` 不允许声称问题由“本次变更引入”。`CHANGES` 的此类归因必须命中真实 changed line。
- findings 为空也必须调用 `codea-review action=finish`。
- 只有 `finish` 返回 `execution=COMPLETE` 才能称评审完成。`coverage=PARTIAL` 时结论固定为 `UNDETERMINED`，并保留所有 gap。
- 任一步失败、源码变化或用户未完成选择时，保留已存在的 INCOMPLETE 报告；不得自动重启整轮，也不得转入代码修复。
- 普通 1.8 Review 不加载 pre-1.8 ordinary Review orchestration。历史说明只在 `.code-harness/history/ordinary-review-pre-1.8.md`，不具执行权威。

## 其他能力保持有效

1.8 Task 3 只替换 ordinary Review 编排，不改变下列能力的产品边界：

- `harness init`：Project Adapter 分析项目，只生成 Harness 配置/适配信息，不擅自修改业务代码。
- `harness test`：Integration Test Agent 负责覆盖分析与测试计划；执行由 Runtime Debugger 承担。测试代码修改前仍需精确 `批准 <planId>`；历史 Existing Test 不自动修改；自动测试修复最多 2 轮且只针对本轮生成测试。
- `harness debug-service`：Runtime Debugger 独占运行/日志/Diagnosis 行为。
- `harness fix finding:<id>` / `harness fix diagnosis:<runId>`：Fix Agent 只做最小修复计划；生产代码修改前仍需精确 `批准 <fixPlanId>`，模糊肯定不构成批准。
- `harness verify ...`：继续由 Runtime Debugger 执行对应验证。
- `harness api-doc ...`：保持只读；目标选择不是写操作审批；最终文档继续由受控 Runtime 生成。
- `harness chain list/show/discover/refresh/edit/validate`：保持 Chain 管理现有确定性事实、candidate、seal/persist 与 exact planId 双阶段确认边界。Chain 不扩大 Review/Test/Debug/Fix 的写范围。
- `harness upgrade`：保持现有离线升级、manifest、事务 apply/rollback 与用户文件保护规则。

### Chain edit 有效契约

`harness chain edit <id|Controller|Controller.method>` 固定路由到 `edit-chain`。Agent 只提交同 run 的结构化 request，受控 Runtime 通过 `codea-dcep-tools.exe chain edit --input <request>` 生成 `analysis/chain-edit-candidates/<id>.yaml` candidate；该 candidate 不等于 Project State。

用户要保存/更新 candidate 时，必须执行 `chain seal-persist`，展示当前 exact `planId`；只有用户随后明确确认该 planId，才允许 `chain persist` 写入 `.code-harness/chains/**`。candidate、analysis、sealed plan 或已有 Project State 任一变化都使旧 planId 失效。

## Workspace Controlled Runtime allowlist

1.8 Task 3 不改变 analyze-change 的 workspace dependency 导航边界。仅在既有 Skill 明确允许时，可调用以下固定命令：

```text
codea-dcep-tools.exe workspace verify --id <id>
codea-dcep-tools.exe nav workspace-inherited --workspace <id> --from <symbol> --method <method>
codea-dcep-tools.exe nav workspace-superclass-call --workspace <id> --from <symbol> --method <method>
codea-dcep-tools.exe nav workspace-template-dispatch --workspace <id> --from <symbol> --hook <hook> [--concrete <class>]
```

workspace dependency 只提供显式、已验证的导航上下文；不得扩大 ordinary Review 的 finding scope、Change Set 或任何写权限。

## 初始化门禁

`harness init`、`harness review`、`harness api-doc`、Chain 查询/维护和 `harness upgrade` 不要求 READY。Test、Debug、Fix、Verify 仍要求 `initialization.status=READY`。

## Agent 职责

- 主 Agent / Orchestrator：意图路由、1.8 ordinary Review 主会话语义评审、用户选择边界、状态与错误摘要。
- API Doc Agent：API target discovery、DTO/Enum/Validation/Direct Service evidence，只读。
- Integration Test Agent：Existing Test Coverage、测试计划以及经审批的测试生成/修复，不负责执行。
- Runtime Debugger：测试/服务执行、日志与 Diagnosis。
- Fix Agent：最小 Fix Plan 和经审批的生产修改，不负责测试执行。
- Project Adapter：初始化适配与配置生成。

## 通用安全与 Authority

- Runtime/Framework managed artifact 不得由 Agent 手工伪造或修补。
- 请求文件写入 `.code-harness/runs/<runId>/requests/**` 时必须保持 UTF-8、结构化字段和本 run identity。
- Agent 不得把普通文本、自报 confidence 或推测升级为确定性 Git/Host/持久化事实。
- 源码 hash、范围、用户选择、报告 path/hash 和持久化计划等确定性事实以受控 Runtime 校验结果为准。
- 同一 OS 用户下不声称 ACL 提供密码学身份隔离；provenance/hash/revalidation 仍是强制边界。

## 审批

- 测试修改：精确 `批准 <planId>`。
- 生产修改：精确 `批准 <fixPlanId>`。
- Chain Project State 写入：只授权当前展示的 exact planId；candidate 或现有状态变化后必须重新 seal 并重新确认。
- 「好 / 继续 / 可以 / yes / ok」等模糊肯定不能替代上述精确审批。

普通 Review 的唯一当前协议就是本文件顶部的 1.8 report-first 流程；其他能力按各自 Agent/Skill 的 active instructions 执行。
