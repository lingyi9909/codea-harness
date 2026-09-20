# Task 5 — 离线升级与最终真实验收

状态：未开始。建议负责能力：Windows 发布 / 集成测试负责人。具体承接人由项目负责人分派。

## 开工条件与必读资料

Task 4 正式验收通过（包含 Task 1–3）；从其 accepted exact HEAD 开始。

- [任务总入口与统一交付要求](README.md)
- [正式设计](../../superpowers/specs/2026-09-15-codea-harness-1.8-review-design.md)：范围、报告语义与 H01–H17。
- [共享接口与完整实施计划](../../superpowers/plans/2026-09-15-codea-harness-1.8-review-plan.md)：完整阅读 Global Constraints、目录和边界、共享接口。
- [报告样稿](../../examples/2026-09-15-codea-harness-1.8-review-report-example.md)：只作为版式规范，非实际验收结果。

每份文档必须从同一任务包提交读取。本文是总计划 Task 5 的分派版，不另起架构。若与总计划或设计不一致，先向负责人列明冲突；不得自行扩大范围。

## 本任务目标

- 制作正式机制的 1.8.0 Windows 候选安装/升级包，并验证真实 1.6.7→1.8.0 升级。
- 完成 Windows 全量、真实模型自主矩阵和内网私有模型验收，给出精确代码与制品证据。

## 研发步骤与文件范围


**Files**

- Create：`.github/workflows/release-1.8.0-windows-x64.yml`、`.github/scripts/release180-package.ps1`、`release180-upgrade-regression.ps1`、`review180-real-model-e2e.py`。
- Modify：`.code-harness/tools-runtime/internal/upgrade/` 中 managed Host inventory 和版本迁移的窄适配；增加 `release_180_test.go`。
- Modify：`.code-harness/VERSION`→1.8.0、README、CHANGELOG、`.code-harness/upgrade.md`、相关现行 release contract。
- Reuse：1.6.7 真实发布包下载/hash/文件清单/升级事务验证代码；旧 installer/builder 不重写。

**Consumes**：T1–T4 已验收产品、新 Host 文件清单。
**Produces**：与 exact HEAD 绑定的候选包、真实模型/Windows/内网分层证据；所有门槛满足后才是正式交付。

- [ ] 固定 1.6.7 baseline：HEAD `970573a6d2147733b773c0de0c124d9324914ae8`、Release Run `34950443217`、install artifact `10389138981`、checklist `10389298093`。下载时验证存续状态与官方 digest；过期时从该 exact HEAD 正式重建并标注来源，不能冒充原 artifact。
- [ ] 将新 `codea-review.ts` 和更新入口纳入 installer/upgrade manifest；真实 1.6.7→1.8.0 更新工具、命令和指令。验证用户 Host 文件冲突零覆盖、未知文件保留、apply 后故障全回滚、同版本 noop。
- [ ] 包内运行 ast-grep --version，校验 pinned 下载 SHA；临时目录含空格、中文、`&`、`#`、`%`。升级后重开原生 Host 会话再调用正式入口。
- [ ] Windows fresh 全量 `go test -count=1 ./...`、`go vet ./...`，所有相关集成测试使用真实工具；无包时 fail，不能 skip。
- [ ] 真模型矩阵：单链有问题、单链无问题、双链选 C1 各 3 个 fresh run；当前实现无 diff、目标无相关变更、模型提前停止、工具失败/超时作为额外场景。
- [ ] 种入问题至少包括明确的租户隔离缺口和无条件 SQL 更新；干净对照包含真实保护条件。真实模型必须找到预期高风险证据，不通过空结果绕开验证。
- [ ] 驱动不注入预制 findings 或工具调用指令，不替模型执行 prepare/select/finish/report。双链测试在回复前观测无 finding review/finish、仅有未完成报告；回复后选中范围必须精确。
- [ ] CI 对无真实模型配置的运行输出 `REAL_MODEL_NOT_VERIFIED` 并阻断正式发布标记；fixture transport PASS 不能顶替。凭据只来自已批准环境，不提交或输出 secret。
- [ ] 产出候选包供内网同模型同 Windows 实测；无法直连内网时该层保持未验证，请用户回传脱敏 run/report/实际工具输出，不上传业务源码。
- [ ] 每个场景证据结构至少为：

```json
{
  "schemaVersion": 1,
  "head": "actual-40-character-head",
  "scenario": "two-chains-select-c1",
  "testKind": "REAL_MODEL_AUTONOMOUS",
  "model": "actual-configured-model",
  "hostVersion": "actual-host-version",
  "runId": "actual-runtime-id",
  "userSelectionObserved": true,
  "modelInvokedFinish": true,
  "driverInvokedRuntimeBusinessSteps": false,
  "execution": "COMPLETE",
  "reportPath": "actual-report-path",
  "reportSha256": "actual-sha256",
  "expectedFindingsFound": true,
  "timings": {"modelMs": 0, "navigationMs": 0, "finishMs": 0, "totalMs": 0}
}
```

以上是字段示意，实际证据的 head/hash/时间必须采集，不能照抄示意值。fixture 测试标 `HOST_TRANSPORT_FIXTURE`；内网标 `PRIVATE_MODEL_ACCEPTANCE`。

- [ ] 提交最终产品 HEAD 后获取 fresh Run ID/Job ID 和制品 SHA；确认报告在模型会话中显示路径且磁盘可打开，才向用户提交验收。

## 独立验收清单

- [ ] 新工具、命令、指令完整进入安装/升级清单；冲突保护、回滚、未知文件保留及同版本 noop PASS。
- [ ] 正式包内真实 Runtime/ast-grep/OpenCode 流程在 Windows 路径矩阵通过。
- [ ] 单链有问题、单链无问题、双链选子集各 3 个 fresh 真模型 run 全部正确生成正式报告。
- [ ] 种入高风险问题被正确发现；提前退出/工具错误不能假完成，脚本不补最后一步。
- [ ] 最终 exact HEAD 的 fresh go test -count=1 ./...、go vet ./... 与发布验收有 Run/Job 证据。
- [ ] 候选包由实际内网私有模型验收；无法验证时明确阻塞正式完成声明，不冒充已完成。

## 不得宣称

- 候选包可下载等同最终验收通过。
- 固定响应耗时等同真实模型时延。
- 复用旧 HEAD 的绿色 CI 作为新版本验收。

## 交付与停止点

按[统一交付格式](README.md#统一交付与验收记录)提供代码分支、base/exact HEAD、变更文件、实际测试结果、证据位置与剩余限制。证据文件建议为 `docs/verification/1.8/task-5-acceptance.md`；首次提交时填真实数据，不建空白证据冒充执行。

开发完成后提交本任务验收，停止，不自动启动后续版本或发布。研发自测通过与负责人正式验收分开记录，后续任务只使用 accepted exact HEAD。

## 可直接转发给研发的指令

> 请在 `lingyi9909/codea-harness` 实施 Codea Harness 1.8 Task 5。先完整读取本任务书、任务总入口、正式设计和共享接口，以开工条件要求的 accepted exact HEAD 为基线，只修改本任务范围。按本文逐项自测，交付真实代码和证据后停止等待验收，不继续其他 Task，不恢复 1.7 开发。
