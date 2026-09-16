# Codea Harness 1.8 Task 1 — 验收证据

日期：2026-09-15  
Task：Task 1 — 报告创建与可靠写盘  
分支：`feature/1.8-task1-report-runtime`

## 1. 基线与候选版本

- 任务包基线：`2580ad171687e441a6c4f45d44f5bd3bd307c161`
- 主实现提交：`2608bb1f92b43a4ad6480abdecbbd2f68a973548`
- testedProductHead / candidate exact HEAD：`157ac4312644242df0e037ee684a5de759b1f8ed`
- evidenceCommit：本文件所在 docs-only 提交；产品验证结果固定绑定上述 testedProductHead，不把证据提交冒充新的产品测试 HEAD。
- 前置 Task：无。

最终产品变更严格限定在 Task 1：新增 `internal/reviewrun` 的 DTO、Start/Status/Cancel/Finish、报告模板与可靠保存测试；新增 `review_run_180.go` CLI 映射；在 `review_precision_command.go` 增加 `start/status/cancel/finish` 路由。旧 `review options/select` 与旧 Review 行为保留。

## 2. TDD RED 证据

RED 实现阶段 HEAD：`79a990a0619f356749fc21b40378015b37292171`。

该 HEAD 的 `internal/reviewrun` 目录只有 `run_test.go` 和 `finish_test.go`，测试已经直接调用 `Start`、`Finish`、`Status`、`FinishRequest`，但生产实现文件尚不存在，因此 `go test` 在编译阶段失败。对应 Windows Runtime Regression：

- Run：`34977729412`
- Job：`104409667865`
- `Review regression tests`：PASS
- `Chain regression tests`：PASS
- `Go test`：FAIL
- `Go vet`：因前一步失败被跳过

该 RED 仅证明“测试先于实现存在”；不把编译失败冒充为产品行为失败。

## 3. 最终 exact-head Windows 验证

最终产品 HEAD `157ac4312644242df0e037ee684a5de759b1f8ed` 的原生 Windows 验证：

- Workflow：`Runtime Regression - Windows x64`
- Run：`34985277433`
- Job：`104435671086`
- Runner：`windows-latest`
- Go：`1.23.x`
- ast-grep：固定 Windows x64 `0.42.1`
- OpenCode native smoke：固定 `1.18.25`

实际步骤结果：

| Step | Result |
|---|---|
| Pinned OpenCode native export smoke test | PASS |
| Review regression tests | PASS |
| Chain regression tests | PASS |
| `go test -count=1 ./...` | PASS |
| `go vet ./...` | PASS |

`go test -count=1 ./...` 在原生 Windows runner 执行，因此 `finish_windows_test.go` 的真实占用句柄测试被包含在最终 exact-head 验证中：报告文件被 Windows 独占打开时 Finish 必须失败且 Status 保持 `INCOMPLETE`；释放句柄后同内容重试可完成，不能出现假 `COMPLETE`。

## 4. Task 1 关键回归

最终源码包含并由上述全量 Windows `./...` 执行的 Task 1 回归包括：

- `Test180StartWritesIncompleteReport`：Start 创建真实 `review.md`，状态为 INCOMPLETE，且无 UTF-8 BOM。
- `Test180StartFailsWhenReportUnwritable`：目标冲突/不可写时 Start 失败，不得宣称 COMPLETE。
- `Test180ConcurrentStartsDoNotOverlap`：并发 Start 的 runId 与报告互不覆盖。
- `Test180FinishWithoutScopeKeepsReport`：没有 prepared scope 时 Finish fail-closed，并保留原未完成报告。
- `Test180FinishPreparedStateWritesDurableReport`：合法内部 prepared 状态下 result 与正式报告真实落盘，Status 回读一致。
- `Test180FinishSameContentRetryIsIdempotent`：同内容完成重试幂等。
- `Test180FinishDifferentContentRejectsOverwrite`：完成后不同内容拒绝覆盖。
- `Test180ResultSavedReportFailureSameContentRetryRepairs`：result 已持久化但 report 替换失败时仍保持 INCOMPLETE；同内容重试补齐报告。
- `Test180TempFileFailureKeepsIncompleteReport`：临时文件创建失败不破坏旧报告、不产生假 COMPLETE。
- `Test180ConcurrentFinishSameRunSerializes`：同 run 并发 Finish 串行化并得到同一报告摘要。
- `Test180CancelRejectsFinish`：取消后 Finish 拒绝，状态保持 CANCELLED。
- `Test180FinishRejectsStaleRead`：源码内容与读取 SHA 不一致时拒绝完成。
- `Test180WindowsOccupiedReportPreventsFalseCompleteAndRetryRepairs`：原生 Windows 文件占用/替换失败语义。
- CLI 回归覆盖 `review start`、`status`、`cancel`，以及 Finish request 的 body/path runId 不一致拒绝。

## 5. 真实写盘样本

### 5.1 Start 未完成报告

由 `Start` 实际生成：

- Run ID：`review-ff587e85aa6188906599a35679e018bb`
- Execution：`INCOMPLETE`
- `review.md` SHA256：`469a715d3b8c05db5864807c88e9560e5f7fa371fce98d392a192c5e4a719c58`
- `run.json` SHA256：`d699d579eb487422eabbd75ba01c37646d472f0f00784dbc17e149e7e896da86`
- 文件：`artifacts/task-1-start-review.md`、`artifacts/task-1-start-run.json`

报告明确包含“评审未完成，尚无评审结论”，并且文件首部无 BOM。

### 5.2 Finish 完成报告

为了只验证 Task 1 的保存机制，使用**包内合法 prepared-state 测试夹具**把同一 run 标记为 scope ready，然后调用生产 `Finish`。这不是 Prepare/Select、主 Agent 或真实模型端到端证据。

- Run ID：`review-716dce90ae4988ef32bf527676c3daae`
- Execution：`COMPLETE`
- Coverage：`COMPLETE`
- fixture ReviewConclusion：`NO_ISSUES_FOUND`（仅保存机制夹具结论，不代表对本仓代码的真实评审）
- `review.md` SHA256：`b2406103bc7278098e2dcccb7e2015bc2882a90823eaf86dd89c4c68a1e45014`
- `result.json` SHA256：`d14ec113e3b848d93bc7838afd0ff86a3ff6b3be63cdbbb6fac6b07cda5a3c1f`
- `run.json` SHA256：`53983fb5a8df6afbeb67fc2fb90500be3977b411dbe193504cd7c122fcdc11cc`
- 报告尾部元数据中的 `resultSha256` 与实际 `result.json` SHA256 一致。
- 文件：`artifacts/task-1-complete-review.md`、`artifacts/task-1-complete-result.json`、`artifacts/task-1-complete-run.json`

## 6. 历史故障覆盖

| ID | Task 1 证据 |
|---|---|
| H01 BOM | Start 测试 + 实际 Start 报告，无 BOM |
| H04 阶段错位 | 新 run 状态与 result/report 回读核对；失败保持 INCOMPLETE |
| H05 缺 runId | Runtime 随机分配 runId；CLI status/cancel 精确绑定 run；Finish 校验 request path/body runId |
| H10 Windows 路径/schema/文件占用 | 原生 Windows full Go test + `Test180WindowsOccupiedReportPreventsFalseCompleteAndRetryRepairs` |
| H16 中断/并发/重复覆盖 | result/report 故障、temp-file 故障、并发 Start/Finish、幂等重试、不同内容覆盖拒绝 |
| H17 误删历史逻辑 | Review regression、Chain regression、full Go test、Go vet 均 PASS |

## 7. 非 Task 1 阻塞项与边界

同一最终产品 HEAD 的旧 `Release Package - Windows x64` Run `34985277431` 在 `Detect release version` 失败。原因是当前仓库 `.code-harness/VERSION` 为 `1.6.7`，而该旧 workflow 的版本白名单只接受 `1.6.1`–`1.6.4`；它在实际打包/产品测试前即停止。Task 1 明确不修改 VERSION、发布流程或发布系统，因此本任务不越界修复该历史发布 gate，也不把它记为 Runtime 报告机制失败。

Task 1 尚未实现或证明：

- Prepare / 调用链发现；
- Select / 真实下一用户回合人工选择；
- 主 Agent / 私有模型自主评审与自主 Finish；
- 五段最终汇报版完整模板；
- 1.8 Windows 正式候选包、升级与发布验收。

这些属于后续 Task，当前不得宣称完成。

## 8. 结论

研发自测结论：**PASS — Runtime 报告机制通过。**

负责人正式验收：**待验收。**

Task 2：**未开始。**
