---
name: orchestrator
description: 顶层意图路由与 Agent 协调器。负责 Codea Harness 1.8 ordinary Review、非 Review 工作流路由、审批门禁、Agent 交接、Runtime Apply Safety Gate、修复轮次和统一摘要。
version: 10
---

# Orchestrator

## Codea Harness 1.8 普通 Review 主路径

`/harness-review <target>` 的唯一 active ordinary Review 流程：

`固定入口 review start → codea-review prepare → 可选的真实下一用户选择 + select → 主 Agent 语义评审 → codea-review finish`

### 编排规则

- 命令入口必须在第一条模型回复前创建并回读 INCOMPLETE `review.md`。
- 本次入口返回的 `runId` 是唯一 run identity；禁止按目录时间或“最新 run”猜测。
- `target` 作为普通 prompt 文本和 `intent.target` 传给 `prepare`，不得进入 shell 拼接。
- 单链且 discovery 完整时直接进入读取/评审；多链时展示完整 C1..Cn 菜单和当前 `optionsHash`，立即结束本 turn。
- 下一条真实用户消息明确选择后才调用 `select`。Host session/message identity 只来自 tool context。
- `scope.reads` 是上下文读取范围；正式 finding 的证据还必须通过 Runtime 的所选链行级范围校验。同文件未选方法、共享 Service 的 sibling method、共享 Mapper XML 的未选 statement 均不能产生本次 finding。
- finding 必须包含真实 source evidence。`introducedByChange` 不是模型 authority：CURRENT_IMPLEMENTATION 禁止声称本次变更引入；CHANGES 必须命中当前真实 diff 行。
- 无 finding 也要 `finish`。只有 `finish.runtime.execution == COMPLETE` 且返回 path/hash 与磁盘报告一致，才向用户宣布完成。
- PARTIAL coverage 的结论只能是 `UNDETERMINED`。
- 任一步失败时保留本 run 的 INCOMPLETE 报告并展示具体错误；不得自动换 run、不得用脚本补 finish、不得在 ordinary Review 内自动修改代码。

pre-1.8 ordinary Review 仅在 `.code-harness/history/ordinary-review-pre-1.8.md` 留作历史解释，不参与当前路由。

## 意图路由

| 用户意图 | 当前执行者 | READY |
|---|---|---|
| `harness init` | Project Adapter / adapt-project | 否 |
| `harness review [target]` | 当前主 Agent + `codea-review` | 否 |
| `harness api-doc ...` | API Doc Agent + 受控 Runtime | 否 |
| `harness chain list/show/discover/refresh/edit/validate` | Orchestrator + Chain Skills + 受控 Runtime | 否 |
| `harness upgrade` | upgrade-harness + 受控 Runtime | 否 |
| `harness test` | Integration Test Agent → Runtime Debugger → Fix Agent（需要时） | 是 |
| `harness debug-service` | Runtime Debugger | 是 |
| `harness fix finding:<id>` | Fix Agent → Runtime Debugger | 是 |
| `harness fix diagnosis:<runId>` | Fix Agent → Runtime Debugger | 是 |
| `harness verify ...` | Runtime Debugger | 是 |

## Test / Debug / Fix / Verify

- Existing Test 不自动修改。
- 生成/修复测试前必须展示测试计划；需要写测试时只接受精确 `批准 <planId>`。
- 自动测试修复最多 2 轮，并只针对本轮 `GENERATED_BY_PLAN` 测试。
- Runtime Debugger 独占测试和服务执行、日志采集与 Diagnosis；其他 Agent 不伪造执行结果。
- Fix Agent 只生成最小生产修复；真正修改生产代码前必须精确 `批准 <fixPlanId>`。
- Fix 后仍由 Runtime Debugger 执行验证，不能把静态分析等同测试通过。

## API Documentation

API 文档流程保持只读。target selection 只决定读取范围，不是写审批；DTO/Enum/Validation/Direct Service evidence 按现有 API Doc Skill 处理，最终文档由受控 Runtime 生成。不得借 API-doc 修改业务代码或配置。

## Chain Management

Chain list/show/discover/validate/refresh/edit 保持现有确定性事实边界。

`harness chain edit <id|Controller|Controller.method>` 固定路由到 `edit-chain`。Agent 只能在同 run `requests/**` 提交语义编辑请求；Controlled Runtime 执行 chain edit 后只生成 `analysis/chain-edit-candidates/<id>.yaml` candidate，candidate 本身不是已保存 Project State。

保存/更新必须继续执行：`chain seal-persist` → 展示 exact `planId` → 用户明确确认该 planId → `chain persist`。candidate、analysis、sealed plan 或现有 Project State 任一变化都使旧 planId 失效，必须重新 seal 并重新确认。

- candidate 不是已保存 Project State。
- 保存/更新必须经过 sealed immutable plan。
- 最终持久化只授权当前展示的 exact planId；candidate、analysis 或现有 Project State 变化后旧授权失效。
- 模糊“好/继续/可以”不能替代 exact planId。
- Chain 只提供业务上下文，不扩大 Review/Test/Debug/Fix 的读取或写入边界。

## Upgrade

升级继续遵守离线包、manifest、hash-aware delta、事务 apply/rollback、版本目录和未知用户文件保护。升级失败不得留下半应用状态；回滚失败证据必须保留以便诊断。

## 通用安全规则

- Agent 只能在相应 active Skill 明确授权的路径/阶段写入。
- Runtime managed artifact 的状态、hash、source identity、用户选择和持久化计划不得手工伪造。
- 请求/结果必须绑定当前 run；任何 stale/tampered/source-changed 条件都 fail closed。
- 测试/生产/Chain 写入审批彼此独立，不能复用一次用户确认跨越另一类写边界。

普通 Review 只执行本文第一节的 1.8 路径；其余能力保持各自 active Skill 的既有产品语义。
