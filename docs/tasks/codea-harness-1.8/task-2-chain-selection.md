# Task 2 — 有界调用链与真实人工选择

状态：未开始。建议负责能力：Go 导航 / Runtime 开发。具体承接人由项目负责人分派。

## 开工条件与必读资料

Task 1 正式验收通过；从其 accepted exact HEAD 开始。

- [任务总入口与统一交付要求](README.md)
- [正式设计](../../superpowers/specs/2026-09-15-codea-harness-1.8-review-design.md)：范围、报告语义与 H01–H17。
- [共享接口与完整实施计划](../../superpowers/plans/2026-09-15-codea-harness-1.8-review-plan.md)：完整阅读 Global Constraints、目录和边界、共享接口。
- [报告样稿](../../examples/2026-09-15-codea-harness-1.8-review-report-example.md)：只作为版式规范，非实际验收结果。

每份文档必须从同一任务包提交读取。本文是总计划 Task 2 的分派版，不另起架构。若与总计划或设计不一致，先向负责人列明冲突；不得自行扩大范围。

## 本任务目标

- 复用批量 AST 导航，生成 nodes[] 调用链、明确范围和缺口。
- 两条及以上实际调用链由真实下一用户消息选择；选择绑定当前 run、菜单、项目与代码内容。

## 研发步骤与文件范围


**Files**

- Create：`internal/reviewrun/prepare.go`、`selection.go`、`prepare_test.go`、`selection_test.go`。
- Create：`internal/reviewauthority/selection_turn_180.go`、`selection_turn_180_test.go`。
- Create：`internal/reviewrun/testdata/controller-review/`，含 OrderController 两个 endpoint、OrderServiceImpl、OrderMapper、OrderMapper.xml、明确项目配置与可复现 Git 基线。
- Modify：`review_run_180.go` 的 prepare/select 路由；仅在必要时给 `internal/nav/extended.go`、`project_calls_163.go` 增加窄适配，保留批处理优化。

**Consumes**：T1 Start/Outcome、nodes[] DTO、现有真实导航与下一用户回合判断。
**Produces**：Prepare/Select、绑定内容与菜单的范围，以及源码读取标识。

- [ ] 建立两条链共享一个 Service 的真实 Java/XML fixture，验证不能合并选项绕过用户选择。
- [ ] 写失败测试：`Test180PrepareTwoEndpointsRequiresSelection`、`Test180UnknownImplementationPreventsAutoSingle`、`Test180NodesRejectLegacyParallelRefs`、`Test180SelectNeedsNextActualUser`、`Test180SelectRejectsStaleMenu`、`Test180SelectedSubsetIsExact`。
- [ ] 用表驱动消息夹具验证下表；fixture 只证明 verifier 判断，最终真实 Host 门禁在 T3/T5 验证。

| Host 消息序列 | 结果 |
|---|---|
| 当前菜单 → user「选择 C1」 | 仅 C1 |
| 当前菜单 → assistant「选择 C1」 | 拒绝 |
| user「选择 C1」→ 当前菜单 | 拒绝 |
| 当前菜单 → user「继续」 | 拒绝 |
| 旧菜单 → user「全部」→ 新菜单 | 拒绝 |
| 本 run 菜单 → 其他 run/项目的 user | 拒绝 |

- [ ] 实现预算内导航与读取复用。缺 ast-grep、超时、坏 JSON、缺 range、范围外路径必须返回缺口/错误，不能产一条“完整”选项。使用执行计数器验证同文件多 symbol 只扫描一次，使用真实 ast 验证结果一致。
- [ ] 提取 Host 消息判断：运行中的 select 工具只验证已存在菜单及下一条 user，不依赖该工具先执行完成。运行 export 使用原生进程参数数组和 timeout，保留 opaque ID 与项目绑定。
- [ ] 完成 Prepare/Select 后把 T1 的合法 scope 保存场景改为调用真实 Prepare/Select；保留必要隔离故障夹具。
- [ ] 运行 `go test ./internal/reviewrun ./internal/reviewauthority ./internal/nav -count=1`；Windows 真 ast 执行小项目计时，超预算定位原因。
- [ ] 提交 T2，交付真实选项与所选范围证据。

## 独立验收清单

- [ ] 真实 Java/Mapper XML fixture 能展示一条及多条入口链，同 Service 的不同入口不合并避选。
- [ ] 只有一条且发现完整时可自动继续；未解析实现或截断不能制造 AUTO_SINGLE。
- [ ] 双链无真实下一用户回复必须阻止评审；选择 C1 只得到 C1，不加入 C2。
- [ ] 旧菜单、未知/重复 ID、跨项目/跨 run、assistant 伪造、用户仅说继续均被拒绝。
- [ ] 真实 ast-grep 的结果、进程数量、超时和小样本时间预算均有证据。

## 不得宣称

- 单凭消息夹具声称原生 Host 已验收。
- 把只发现一条链当成证明不存在第二条链。
- 把未选链当成已评审。

## 交付与停止点

按[统一交付格式](README.md#统一交付与验收记录)提供代码分支、base/exact HEAD、变更文件、实际测试结果、证据位置与剩余限制。证据文件建议为 `docs/verification/1.8/task-2-acceptance.md`；首次提交时填真实数据，不建空白证据冒充执行。

开发完成后提交本任务验收，停止，不自动进入 Task 3。研发自测通过与负责人正式验收分开记录，后续任务只使用 accepted exact HEAD。

## 可直接转发给研发的指令

> 请在 `lingyi9909/codea-harness` 实施 Codea Harness 1.8 Task 2。先完整读取本任务书、任务总入口、正式设计和共享接口，以开工条件要求的 accepted exact HEAD 为基线，只修改本任务范围。按本文逐项自测，交付真实代码和证据后停止等待验收，不继续其他 Task，不恢复 1.7 开发。
