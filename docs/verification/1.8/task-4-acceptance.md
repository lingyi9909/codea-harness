# Codea Harness 1.8 Task 4 — 正式验收记录

日期：2026-09-18  
Task：Codea Harness 1.8 Task 4 — 汇报版报告与历史故障回归  
Branch：`feature/1.8-task4-report-regression`  
Base / T3 Accepted Exact HEAD：`2346bde3d5fe8dda7a372d9c7a08ee0caf3a7557`

## 1. 产品版本与证据提交

- testedProductHead = `52ae4dc2a02d7f124d25a3ee3c02196cae85c82a`
- evidenceCommit = `2f8e7d1d1292d4c1c0bae3d06b9cc9f99dd99124`
- Formal PR：#57（Draft，未合并）
- CI-only PR：#58（仅用于触发 exact-head Windows Runtime Regression，不得合并）
- 本记录是在产品代码、模板与测试已经完成 fresh exact-head 验证后补充的 docs-only 验收证据。
- 后续 docs-only evidence commit 不改变 testedProductHead，也不得拿 evidence HEAD 冒充新的产品测试 HEAD。
- 当前不得进入 Task 5，不得合并 PR #57。

## 2. T4 变更范围

相对 T3 Accepted Exact HEAD `2346bde3d5fe8dda7a372d9c7a08ee0caf3a7557`，T4 产品范围只包含：

- `.code-harness/tools-runtime/internal/reviewrun/report.go`
- `.code-harness/tools-runtime/internal/reviewrun/review.md.tmpl`
- `.code-harness/tools-runtime/internal/reviewrun/finish.go`
- `.code-harness/tools-runtime/internal/reviewrun/report_test.go`
- `.code-harness/tools-runtime/internal/reviewrun/regression_180_test.go`
- `.code-harness/tools-runtime/internal/reviewrun/testdata/reports/blocking.golden.md`
- `.code-harness/tools-runtime/internal/reviewrun/testdata/reports/action-required.golden.md`
- `.code-harness/tools-runtime/internal/reviewrun/testdata/reports/no-issues.golden.md`
- `.code-harness/tools-runtime/internal/reviewrun/testdata/reports/partial.golden.md`
- `.code-harness/tools-runtime/internal/reviewrun/testdata/reports/waiting-selection.golden.md`
- `.code-harness/tools-runtime/internal/reviewrun/testdata/reports/cancelled.golden.md`

未修改普通 Review 的 Prepare / Select / Host 协议；未恢复 Task153 workflow；未进入升级、打包、发布或 Task 5 真模型矩阵。

## 3. 已实现行为

### 3.1 固定五段正式报告

实际 renderer 固定输出以下五段：

1. `评审摘要`
2. `调用链与覆盖范围`
3. `风险清单`
4. `问题明细`
5. `后续处理与评审边界`

报告首屏独立展示：

- execution
- reviewConclusion
- coverage
- mode / target
- selected scope / file count
- CRITICAL / HIGH / MEDIUM / LOW 数量
- pending risks 数量

风险统计由结构化 findings 派生，不接受模型另写一套汇总数字。

### 3.2 结论矩阵

永久测试 `Test180ConclusionMatrix` 固化：

- PARTIAL / 非完整覆盖 → `UNDETERMINED`
- COMPLETE + CRITICAL/HIGH → `BLOCKING`
- COMPLETE + MEDIUM/LOW → `ACTION_REQUIRED`
- COMPLETE + pending risk → `ACTION_REQUIRED`
- COMPLETE + findings/pending 均为空 → `NO_ISSUES_FOUND`

执行完成、风险结论、覆盖状态始终分离；不把流程成功写成上线批准。

### 3.3 调用链与范围边界

- selected chain 展示完整 nodes 顺序、role、path、symbol。
- unselected chain 只展示边界说明，不展示其节点为“已检查”内容。
- waiting-selection 只展示候选链名称，不输出候选链节点为已检查明细，也不形成正式 finding。
- 长链不静默截断。
- coverage gap 与 unselected scope 始终保留。

### 3.4 Markdown / 路径安全

- 普通展示文本转义 Markdown 表格符、链接/图片语法、HTML、反引号等主动内容。
- evidence 以动态长度 fenced code block 保留源码原文，源码中自带反引号不会关闭 fence。
- Windows / 中文 / 特殊字符路径使用安全 code span。
- 路径中的 `|` 会在 Markdown table 内转义，不能新增伪列。
- 生产 `review.md` 持久化统一 canonical UTF-8 / LF，消除 Windows CRLF template 与动态 LF 内容混合造成的跨平台字节漂移。

## 4. 六类 Golden 与补充矩阵

永久 exact golden：

| 场景 | 文件 | Result |
|---|---|---|
| 阻断问题 | `blocking.golden.md` | PASS |
| 非阻断 / ACTION_REQUIRED | `action-required.golden.md` | PASS |
| 无问题 | `no-issues.golden.md` | PASS |
| 覆盖未完成 | `partial.golden.md` | PASS |
| 等待选择 | `waiting-selection.golden.md` | PASS |
| 取消 | `cancelled.golden.md` | PASS |

补充永久测试：

- `Test180ReportZeroOptionsAndCurrentImplementationMode` — 零选项无变更、CURRENT_IMPLEMENTATION：PASS
- `Test180ReportSummaryCountsAreDerivedFromFindings` — 统计派生 / pending risk 分离：PASS
- `Test180ReportEscapesDisplayContentAndKeepsEvidenceLiteral` — Markdown/HTML/backtick/Windows 中文特殊路径：PASS
- `Test180ReportLongChainIsCompleteAndUnselectedNodesStayOutOfCheckedList` — 长链完整、未选节点隔离：PASS
- `Test180ReportWaitingSelectionShowsCandidatesWithoutCheckedNodes` — 等待人工选择边界：PASS
- `Test180ConclusionMatrix` — 固定风险结论矩阵：PASS

这些测试由 fresh `go test -count=1 ./...` 在 testedProductHead 上执行，`internal/reviewrun` package 结果：

`ok codea-harness-tools/internal/reviewrun 15.674s`

## 5. 实际生成报告预览证据

T4 的 blocking renderer output 由 `Test180ReportGoldens/blocking` 在 Windows exact-head run 中真实生成，并与保留文件逐字节比较。

保留预览：

`.code-harness/tools-runtime/internal/reviewrun/testdata/reports/blocking.golden.md`

在 repository UTF-8/LF 表示下：

- bytes：`3276`
- report SHA256：`b4b1880107df09f100de96f1a3769779dc614287d03511012c24a24af14a4ab0`

该文件不是手写样稿替代品：验收测试实际调用生产 `renderReport()` 生成 Markdown，再与此文件 exact-byte 比较；本次 Windows fresh run 已 PASS。

人工预览已核对：

- 首屏同时显示 execution / risk / coverage，三者含义独立。
- 调用链表只展开 selected C1 的 4 个节点；C2 明确标记“未选择”，不把 C2 节点冒充已检查内容。
- HIGH finding 在摘要、风险表、问题明细三处一致。
- evidence 使用 fenced code，长 SQL 不塞进表格。
- 后续处理与边界明确写明未选择链和外部依赖不属于本报告验证范围。
- 未编造负责人、期限、通过率、事故金额或收益。

说明：上述 retained report 是 deterministic renderer / golden 类组件证据；Native Host 固定响应证据属于下一节，二者不混称为真实模型语义验收。T3 已完成真实模型最小自主闭环；T5 仍负责三类真模型矩阵与正式 artifact 保留。

## 6. Fresh Windows Runtime Regression

正式 exact-head 验证：

- Workflow：`Runtime Regression - Windows x64`
- Run：`35328142099`
- Job：`105545960713`
- testedProductHead：`52ae4dc2a02d7f124d25a3ee3c02196cae85c82a`
- Run conclusion：`SUCCESS`
- OpenCode Host：`1.18.25`

门禁结果：

| Gate | Result |
|---|---|
| Report identity regression | PASS |
| Pinned OpenCode native export | PASS |
| Review regression | PASS |
| Chain regression | PASS |
| `go test -count=1 ./...` | PASS |
| `go vet ./...` | PASS |
| Native Host single-chain | PASS |
| Native Host multi-chain real-user-selection | PASS |
| Native Host concurrent-finish | PASS |
| Real-model autonomous smoke | NOT RUN — T4 branch intentionally excluded; T3 evidence remains authoritative, T5 runs full matrix |

Native Host 日志：

- single-chain：`REVIEW180_HOST_SINGLE PASS runId=review-96a0a20e33c6e38a07893b486aee185e actions=['prepare', 'finish']`
- multi-chain：`REVIEW180_HOST_MULTI PASS runId=review-c78b3b8783bdc012d5dd3f8d381764db actions=['prepare', 'select', 'finish'] actualUserReply=true`
- concurrent-finish：`REVIEW180_HOST_CONCURRENT_FINISH PASS runId=review-e9b05cbb3a4205464a00ed909f6d4559 completed=1 rejected=1 uniqueRequests=2`
- Host 总结：`REVIEW180_NATIVE_HOST_SMOKE PASS opencode=1.18.25 deterministic=true`

Native Host 固定响应只证明 Host/tool/runtime 文件链路，不替代真实模型语义质量。

## 7. H01–H17 永久追踪

| ID | testName / HostScenario | T4 状态 |
|---|---|---|
| H01 | `Test180ReviewFinishIngressIsBOMCompatibleAndStrict`; `Test180StartWritesIncompleteReport` | COVERED |
| H02 | `Test180PrimaryReviewInstructionsDoNotRequireLegacyReviewerAuthority` | COVERED |
| H03 | `Test180PrimaryReviewFinishPathDoesNotUseOpaqueHostID`; `Test180SelectedSubsetIsExact` | COVERED |
| H04 | `Test180FinishWithoutScopeKeepsReport`; `Test180ReviewFinishCommandRejectsRunIDMismatch` | COVERED |
| H05 | `Test180ReviewStartCommandWritesIncompleteReport`; `Test180ReviewFinishCommandRejectsRunIDMismatch` | COVERED |
| H06 | `Test180FinishPreparedStateWritesDurableReport`; T3 real-model autonomous finish evidence | COVERED |
| H07 | `Test180NodesRejectLegacyParallelRefs`; finish ingress legacy `chainRefs` rejection | COVERED |
| H08 | `Test180PrepareTwoEndpointsRequiresSelection`（15s fixture budget / navigation process count） | COVERED |
| H09 | `Test180PrepareMissingAstGrepFailsClosed`; 正式包 SHA / version 留 T5 | COVERED_WITH_TASK5_FINAL |
| H10 | `Test180WindowsOccupiedReportPreventsFalseCompleteAndRetryRepairs`; report identity regression | COVERED_WITH_TASK5_FINAL |
| H11 | `Test180SelectNeedsNextActualUser`; `Test180SelectedSubsetIsExact`; Native Host multi-chain | COVERED_WITH_TASK5_FINAL |
| H12 | 正式 1.8.0 package + 1.6.7→1.8.0 transaction upgrade | TASK5_REQUIRED |
| H13 | `Test180CurrentWorkflowDoesNotRestoreTask153`; `Test180HistoricalCompatibilityAssetsRemain` | COVERED |
| H14 | `Test180SmokeDriversUseNativeCommandPathAndDoNotScriptFinish`; T3 real-model evidence | COVERED_WITH_TASK5_FINAL |
| H15 | `Test180ReportGoldens`; `Test180ConclusionMatrix`; zero-option/current-implementation regression | COVERED |
| H16 | concurrent start/finish、result-save retry、cancel terminal-state regressions | COVERED |
| H17 | `Test180HistoricalCompatibilityAssetsRemain`; fresh full Go/vet | COVERED |

`regression_180_test.go` 额外机械断言：

- 恰好存在 H01–H17 共 17 项。
- 每项必须有真实 testName / HostScenario 描述。
- 禁止用“已考虑”类文字代替证据。
- H12 必须保持 `TASK5_REQUIRED`，T4 不得伪造升级通过。
- 废弃 `.github/workflows/task153-chain-reliability.yml` 必须保持不存在。
- Task153 历史兼容测试、1.6.7 upgrade 回归、upgrade skill 必须保持存在。

## 8. 本轮发现并修复的失败

T4 fresh Windows 回归过程中真实发现：

1. 两个含 finding 的 golden 与 renderer 在结尾字节存在差异，exact golden 首轮失败。
2. 进一步复验定位到 Windows checkout 使用 CRLF，而生产 template 与动态 evidence helper 的 LF 混合，使有 evidence 的报告跨平台字节不稳定。
3. 最终修复为生产 `renderReport()` 统一 canonical UTF-8/LF，golden 读取同样规范化 CRLF→LF 后再 exact-byte 比较。
4. 修复后 testedProductHead `52ae4dc2a02d7f124d25a3ee3c02196cae85c82a` fresh 全量 Windows Runtime Regression SUCCESS。

没有通过放宽 golden、删除失败测试或恢复旧 Task153 workflow 来换取绿色结果。

## 9. Known limits / 未验证范围

以下能力不由 T4 宣称完成：

1. **正式 1.8.0 install / upgrade package**：属于 T5。
2. **真实 1.6.7→1.8.0 升级与 rollback**：属于 T5。
3. **H09/H10/H11/H14 最终发行包/真实环境矩阵**：T4 保留组件/Host 回归，最终包与完整真实环境验证属于 T5。
4. **三类真实模型各 3 次 fresh run**：属于 T5；本轮 T4 branch 的 real-model step按现有 Workflow 条件明确 NOT RUN，不以 T3 旧结果冒充 T4 新运行。
5. **实际公司内网私有模型 / 正式 Windows 包验收**：属于 T5，当前未验证。
6. **发布可用性与合并**：T4 只形成验收候选，不代表 1.8.0 已可发布。

## 10. 验收状态

研发自验证：**PASS。**

依据 testedProductHead `52ae4dc2a02d7f124d25a3ee3c02196cae85c82a` 的：

- 六类 exact golden 与补充报告矩阵
- H01–H17 永久回归追踪
- fresh `go test -count=1 ./...`
- fresh `go vet ./...`
- Windows Native Host single / multi / concurrent smoke
- 实际生成 Markdown exact preview 检查

Task 4 当前已满足研发侧交付门槛。

独立验收：**尚待验收人确认。**

本记录不替代验收人的最终 ACCEPTED 决定。

Task 5：**未进入。**  
PR #57：**保持 Draft / 未合并。**
