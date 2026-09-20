# Task 3 — 主 Agent 自主评审并完成报告

状态：未开始。建议负责能力：OpenCode / TypeScript 集成开发，配合 Go 开发。具体承接人由项目负责人分派。

## 开工条件与必读资料

Task 2 正式验收通过（包含 Task 1）；从其 accepted exact HEAD 开始。

- [任务总入口与统一交付要求](README.md)
- [正式设计](../../superpowers/specs/2026-09-15-codea-harness-1.8-review-design.md)：范围、报告语义与 H01–H17。
- [共享接口与完整实施计划](../../superpowers/plans/2026-09-15-codea-harness-1.8-review-plan.md)：完整阅读 Global Constraints、目录和边界、共享接口。
- [报告样稿](../../examples/2026-09-15-codea-harness-1.8-review-report-example.md)：只作为版式规范，非实际验收结果。

每份文档必须从同一任务包提交读取。本文是总计划 Task 3 的分派版，不另起架构。若与总计划或设计不一致，先向负责人列明冲突；不得自行扩大范围。

## 本任务目标

- 正式入口先执行 start；主会话通过一个结构化工具完成准备、选择和结果提交。
- Finish 一次工具调用完成校验、保存、渲染、回读；真正跑通模型自主执行的最小报告闭环。

## 研发步骤与文件范围


**Files**

- Create：`.code-harness/tools/codea-review.ts`、`.github/scripts/review180-host-smoke.py`。
- Modify：`.code-harness/commands/harness-review.md`、`.code-harness/AGENTS.md`、`.code-harness/bootstrap.md`、`.code-harness/agents/orchestrator.md`、`.code-harness/skills/review-code/SKILL.md`。
- Modify：`internal/reviewrun/finish.go`、`finish_test.go`，补齐源码/范围/证据检查；必要时 `review_run_180.go` 增加结构化读取接口供工具内部使用。
- Create：`cmd/codea-dcep-tools/review_entry_180_test.go`。

**Consumes**：T1/T2 完整 Runtime 接口和真实 Host 执行 context。
**Produces**：正式主会话入口与 action 工具；自主模型最小闭环证据。

- [ ] RED：正式命令应在模型第一条回答前已有 review.md；旧入口仅 begin 无文件应失败。命令中的 shell 部分只允许固定 `review start`，用户目标保持普通文本。
- [ ] 工具 args 暴露 action/runId/intent/selection/result；HostTurn 从 context 读取，模型不能填写；工具自行构造 UTF-8 请求、参数数组执行本地 Runtime。对 SDK 使用随正式包已获准的方式，不能新增内网在线安装步骤。
- [ ] 修改当前普通 Review 指令为唯一流程，删除其中互相矛盾的旧 Reviewer/认证步骤，历史说明明确隔离；不得只在一篇长旧指令顶部叠一段“新优先级”。
- [ ] finish 同一次工具调用内部完成校验、写盘和回读；返回固定的 execution/conclusion/path/hash。空 findings 同样调用 finish，部分覆盖不能显示 NO_ISSUES_FOUND。
- [ ] 对 tool 输入写 BOM、缺字段、旧 chainRefs、跨范围、源码变化、opaque Host ID、Windows 路径和取消测试。预检失败修正原 run，禁止自动重跑整轮。
- [ ] 原生 Host 确定性 fixture 测试验证执行传输与 next-user 选择；再接可用真实模型执行单链问题样本。驱动只发 `/harness-review OrderController` 等正常用户输入，监测实际工具调用。
- [ ] 自主执行驱动允许的动作与禁止项：

```python
# 驱动伪代码；只暴露用户会做的交互，不补 Runtime 步骤。
session = start_native_opencode(project, configured_model)
send_user(session, "/harness-review OrderController")
events = observe_until_idle(session)
# 双链场景此处验证尚未 finish，再发送真实明确选择。
# 单链场景不追加“请调用 finish”的人工救场消息。
assert observed_tool(events, "codea-review", action="finish")
assert_report_matches_tool_return(project, events)
```

- [ ] 上述驱动的 helper 在 `review180-host-smoke.py` 定义，复用已验证原生 OpenCode 启动/观察代码；**不得复用 release167-review-e2e.py 中脚本主动 rt(certify/report) 的业务执行方式**。
- [ ] 若真实模型没有自主 finish，记录失败并修入口/工具交互，不通过注入脚本调用绕过。未通过前不进入 T4 扩充。
- [ ] 运行相关 Runtime、Host 与入口测试，提交 T3；明确所用模型与未覆盖的私有模型环境。

## 独立验收清单

- [ ] 模型第一次回答前报告已存在，runId 来自本次入口，不取全局最新 run。
- [ ] 至少单链有问题由真实模型自主调用 finish，返回文件路径和摘要，文件内容可读；无问题场景先完成原生 Host 验证，完整真模型矩阵在 T5 验收。
- [ ] 双链原生 Host 场景在真实用户回复前无正式发现，回复后报告范围与选择一致；T5 再完成真模型重复矩阵。
- [ ] 缺 Reviewer authority 文件不阻止新主路径；模型不能填写 HostTurn/userConfirmed 伪造选择。
- [ ] 预检失败可在原 run 修正；提前退出保留未完成报告，不能算正常成功。
- [ ] 驱动只发送正常用户输入与选择，未替模型执行任何 Runtime 业务步骤。

## 不得宣称

- 用固定响应替代真实模型自主执行。
- 测试脚本补调 finish/report 后声称闭环完成。
- 最小自主闭环未通过便进入 Task 4。

## 交付与停止点

按[统一交付格式](README.md#统一交付与验收记录)提供代码分支、base/exact HEAD、变更文件、实际测试结果、证据位置与剩余限制。证据文件建议为 `docs/verification/1.8/task-3-acceptance.md`；首次提交时填真实数据，不建空白证据冒充执行。

开发完成后提交本任务验收，停止，不自动进入 Task 4。研发自测通过与负责人正式验收分开记录，后续任务只使用 accepted exact HEAD。

## 可直接转发给研发的指令

> 请在 `lingyi9909/codea-harness` 实施 Codea Harness 1.8 Task 3。先完整读取本任务书、任务总入口、正式设计和共享接口，以开工条件要求的 accepted exact HEAD 为基线，只修改本任务范围。按本文逐项自测，交付真实代码和证据后停止等待验收，不继续其他 Task，不恢复 1.7 开发。
