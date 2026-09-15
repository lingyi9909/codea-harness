# Task 1 — 报告创建与可靠写盘

状态：未开始。建议负责能力：Go Runtime 开发。具体承接人由项目负责人分派。

## 开工条件与必读资料

无产品任务前置；从任务包文档提交所对应的 main 基线开始。

- [任务总入口与统一交付要求](README.md)
- [正式设计](../../superpowers/specs/2026-09-15-codea-harness-1.8-review-design.md)：范围、报告语义与 H01–H17。
- [共享接口与完整实施计划](../../superpowers/plans/2026-09-15-codea-harness-1.8-review-plan.md)：完整阅读 Global Constraints、目录和边界、共享接口。
- [报告样稿](../../examples/2026-09-15-codea-harness-1.8-review-report-example.md)：只作为版式规范，非实际验收结果。

每份文档必须从同一任务包提交读取。本文是总计划 Task 1 的分派版，不另起架构。若与总计划或设计不一致，先向负责人列明冲突；不得自行扩大范围。

## 本任务目标

- 实现 Start/Status/Cancel 和最小报告保存机制；入口先有可读的未完成报告。
- 定义共享 DTO，Finish 未具备合法范围时拒绝；合法内部状态下验证实际写盘、幂等、错误保留。

## 研发步骤与文件范围


**Files**

- Create：`internal/reviewrun/types.go`、`run.go`、`finish.go`、`report.go`、`internal/reviewrun/run_test.go`、`finish_test.go`、最小 `review.md.tmpl`。
- Create：`cmd/codea-dcep-tools/review_run_180.go` 与 `review_run_180_test.go`。
- Modify：`cmd/codea-dcep-tools/review_precision_command.go` 中 review 子命令路由；新增 start/status/cancel/finish 分支，旧行为不替换。
- Reuse：`internal/requestjson/read.go`、`internal/schema/schema.go`。

**Consumes**：现有配置/路径校验、随机 ID 和文件 I/O 能力；不依赖 Reviewer。
**Produces**：Start/Status/Cancel、严格 Finish 保存机制和最小中文报告；Prepare/Select 此时不得伪装可用。

- [ ] 写 `Test180StartWritesIncompleteReport`、`Test180StartFailsWhenReportUnwritable`、`Test180FinishWithoutScopeKeepsReport`。首个测试的核心断言：

```go
func Test180StartWritesIncompleteReport(t *testing.T) {
    root := t.TempDir()
    got, err := Start(root)
    if err != nil { t.Fatal(err) }
    if got.Execution != "INCOMPLETE" || got.RunID == "" { t.Fatal(got) }
    data, err := os.ReadFile(got.ReportPath)
    if err != nil { t.Fatal(err) }
    if !bytes.Contains(data, []byte("评审未完成")) { t.Fatal(string(data)) }
    if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) { t.Fatal("BOM") }
}
```

- [ ] 运行 `go test ./internal/reviewrun -run '^Test180(Start|FinishWithoutScope)' -count=1 -v`，确认失败原因是缺实现/真实行为，不能忽略编译错误并宣称产品红灯。
- [ ] 实现共享 DTO、最小 Start/Status/Cancel，finish 暂时没有已准备 scope 时明确拒绝。包内测试使用真实临时文件验证已经准备的合法内部状态下的渲染路径；测试夹具不进入生产编译。
- [ ] 增加保存故障测试：result 成功/report 失败、临时文件写入失败、Windows 文件被占用、两并发 run、同 run 两次 finish 并发、同内容 finish 重试、不同内容覆盖拒绝；核对旧报告可读且无假 COMPLETE。单 run 短期互斥只覆盖本地读写，人工等待不占锁。
- [ ] 运行 `go test ./internal/reviewrun ./internal/requestjson ./internal/schema -count=1`，在 Windows 用真实文件句柄完成占用测试。
- [ ] 提交 T1，只报告“Runtime 报告机制通过”，不得称主 Agent 链路完成。

## 独立验收清单

- [ ] 新建 run 返回路径指向真实 review.md，明确未完成，不含 BOM。
- [ ] 不可写/空间不足/替换失败返回错误，不能出现假 COMPLETE。
- [ ] 两个 run 互不覆盖；同 run 并发 finish 有互斥；同内容重试幂等，不同完成结果拒绝覆盖。
- [ ] 取消后不能继续 finish；不存在范围时不得伪造范围或生成正式成功报告。
- [ ] Windows 实际文件占用/替换测试及本任务相关回归 PASS。

## 不得宣称

- 主 Agent/真实模型评审完成。
- 调用链发现与人工选择完成。
- 1.8 已可发布。

## 交付与停止点

按[统一交付格式](README.md#统一交付与验收记录)提供代码分支、base/exact HEAD、变更文件、实际测试结果、证据位置与剩余限制。证据文件建议为 `docs/verification/1.8/task-1-acceptance.md`；首次提交时填真实数据，不建空白证据冒充执行。

开发完成后提交本任务验收，停止，不自动进入 Task 2。研发自测通过与负责人正式验收分开记录，后续任务只使用 accepted exact HEAD。

## 可直接转发给研发的指令

> 请在 `lingyi9909/codea-harness` 实施 Codea Harness 1.8 Task 1。先完整读取本任务书、任务总入口、正式设计和共享接口，以开工条件要求的 accepted exact HEAD 为基线，只修改本任务范围。按本文逐项自测，交付真实代码和证据后停止等待验收，不继续其他 Task，不恢复 1.7 开发。
