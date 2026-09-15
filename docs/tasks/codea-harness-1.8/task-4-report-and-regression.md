# Task 4 — 汇报版报告与历史故障回归

状态：未开始。建议负责能力：报告模板 / Go 开发，配合测试。具体承接人由项目负责人分派。

## 开工条件与必读资料

Task 3 真实模型最小闭环正式验收通过；从其 accepted exact HEAD 开始。

- [任务总入口与统一交付要求](README.md)
- [正式设计](../../superpowers/specs/2026-09-15-codea-harness-1.8-review-design.md)：范围、报告语义与 H01–H17。
- [共享接口与完整实施计划](../../superpowers/plans/2026-09-15-codea-harness-1.8-review-plan.md)：完整阅读 Global Constraints、目录和边界、共享接口。
- [报告样稿](../../examples/2026-09-15-codea-harness-1.8-review-report-example.md)：只作为版式规范，非实际验收结果。

每份文档必须从同一任务包提交读取。本文是总计划 Task 4 的分派版，不另起架构。若与总计划或设计不一致，先向负责人列明冲突；不得自行扩大范围。

## 本任务目标

- 落实已批准五段报告版式和执行/风险/覆盖三者分离。
- 将设计中的 H01–H17 映射到永久回归，消除新旧正常 Review 指令/契约冲突。

## 研发步骤与文件范围


**Files**

- Modify：`internal/reviewrun/report.go`、`review.md.tmpl`、`finish.go`。
- Create：`internal/reviewrun/report_test.go`、`regression_180_test.go`、`testdata/reports/` 下六类 golden。
- Modify：仅真实新流程不再适用的 active Review contract 测试；每处记录旧断言与新断言理由，不删除历史兼容测试。

**Consumes**：T3 真实结果、scope/读取记录；已批准五段报告样稿。
**Produces**：固定汇报模板、H01–H17 追踪表与全部相关回归。

- [ ] 六个 golden：阻断问题、非阻断问题、无问题、覆盖未完成、等待选择、取消；再覆盖零选项无变更和当前实现模式。
- [ ] 断言首屏有独立执行/风险/覆盖信息；数字从 findings 派生；待确认风险不计正式问题；未选链不能出现在已检查列表。结论矩阵：PARTIAL/未完成/取消→UNDETERMINED，完整且含严重/高→BLOCKING，仅中/低或待确认→ACTION_REQUIRED，完整且两类列表都为空→NO_ISSUES_FOUND。
- [ ] 断言合法证据包含源码中的竖线、反引号、HTML、Windows/中文路径时排版仍正确且无主动内容；长链有完整明细。
- [ ] 对照设计 H01–H17 给每项填写真实 testName/HostScenario；未覆盖不得以文字“已考虑”通过。
- [ ] Markdown 渲染人工查看首屏、链表、问题卡片和长内容折行；只验证实际生成的报告，不拿手写样稿替代。
- [ ] 运行 `go test ./... -count=1` 与 `go vet ./...`；针对修改的新流程契约列出差异，不修改无关 Task153 历史功能。
- [ ] 提交 T4，交付 golden 与真实生成文件截图/预览证据。

## 独立验收清单

- [ ] 真实生成报告具有摘要、调用链与覆盖、风险清单、问题明细、后续处理与边界五部分。
- [ ] 六类核心 golden 及无变更/当前实现模式通过；统计由结果派生，不编造通过率、负责人或收益。
- [ ] 源码/路径中的表格符号、HTML、反引号、中文等不会破坏排版或引入主动内容。
- [ ] 人工查看实际生成的 Markdown 渲染效果，不能用手写样稿代替。
- [ ] H01–H17 均有 testName/HostScenario 与对应结果；需最终包/内网确认的项明确留给 Task 5。
- [ ] fresh 全量 Go 测试和 vet PASS，保留历史非本轮功能兼容。

## 不得宣称

- 尚未完成的升级/私有模型验证标成已通过。
- 部分覆盖或取消显示绿色通过。
- 仅因 Task153 字样删除历史测试。

## 交付与停止点

按[统一交付格式](README.md#统一交付与验收记录)提供代码分支、base/exact HEAD、变更文件、实际测试结果、证据位置与剩余限制。证据文件建议为 `docs/verification/1.8/task-4-acceptance.md`；首次提交时填真实数据，不建空白证据冒充执行。

开发完成后提交本任务验收，停止，不自动进入 Task 5。研发自测通过与负责人正式验收分开记录，后续任务只使用 accepted exact HEAD。

## 可直接转发给研发的指令

> 请在 `lingyi9909/codea-harness` 实施 Codea Harness 1.8 Task 4。先完整读取本任务书、任务总入口、正式设计和共享接口，以开工条件要求的 accepted exact HEAD 为基线，只修改本任务范围。按本文逐项自测，交付真实代码和证据后停止等待验收，不继续其他 Task，不恢复 1.7 开发。
