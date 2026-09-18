# Codea Harness 1.8 Task 3 — 正式验收记录

日期：2026-09-18  
Task：Codea Harness 1.8 Task 3 — 主 Agent 自主评审并完成正式报告  
Branch：`feature/1.8-task3-primary-report`  
Base / T2 Accepted Baseline：`db2f03cebafa6f72c733286a083ad97e14f41cf5`

## 1. 产品版本与证据提交

- testedProductHead = `05b1bf27ee21bbe94198c0a7d2c1f29af09f9943`
- evidenceCommit = `572fbe8bec314ba09cab9ef93454f7d41f362796`
- 本记录是在已经完成产品代码与测试验证的 testedProductHead 之后补充的 docs-only 验收证据。
- evidenceCommit 只用于固化验收证据，不改变 testedProductHead，也不把证据提交冒充新的产品测试 HEAD。
- 本轮不得进入 Task 4，不得合并 PR #56。

## 2. 正式 Windows Runtime Regression

正式 exact-head 验证绑定：

- Workflow：`Runtime Regression - Windows x64`
- Run：`35317540750`
- Job：`105512414862`
- testedProductHead：`05b1bf27ee21bbe94198c0a7d2c1f29af09f9943`
- Branch：`feature/1.8-task3-primary-report`
- Base：`feature/1.8-task2-chain-selection`
- T2 Accepted Baseline：`db2f03cebafa6f72c733286a083ad97e14f41cf5`
- Run conclusion：`SUCCESS`

正式门禁结果：

| Gate | Result |
|---|---|
| Review regression | PASS |
| Chain regression | PASS |
| `go test -count=1 ./...` | PASS |
| `go vet ./...` | PASS |
| Native Host single-chain | PASS |
| Native Host multi-chain real-user-selection | PASS |
| Native Host concurrent-finish | PASS |
| Real-model autonomous smoke | PASS |

Native Host 日志证据：

- single-chain：`REVIEW180_HOST_SINGLE PASS`，动作 `prepare → finish`
- multi-chain：`REVIEW180_HOST_MULTI PASS`，动作 `prepare → select → finish`，且 `actualUserReply=true`
- concurrent-finish：`REVIEW180_HOST_CONCURRENT_FINISH PASS`，同一 run 两个不同请求中恰好一个完成、一个被拒绝，`uniqueRequests=2`
- OpenCode Host：`1.18.25`

## 3. Real-model 自主闭环证据

正式保留 Artifact：

- Artifact：`review180-real-model-acceptance`
- Artifact ID：`10536475343`
- Artifact SHA256：`a9bbfa0370915f240e505e3ae69a20e2246039b62d3c10f66b89dc1aa913f5a0`
- Model：`task15secret/deepseek-v4-pro`
- OpenCode Host：`1.18.25`
- Real-model runId：`review-242e2da0eb53fd36da2ffdf6256d1f64`

真实模型自主执行路径：

`prepare → read ×4 → finish`

该路径为模型自主执行。驱动只发送正常用户 Review 输入并观察真实 Tool / Host 事件；没有脚本代替模型执行 `prepare`、`select` 或 `finish`。本场景为 single-chain，因此没有 `select` 动作；若需要选择，选择必须来自真实下一用户回合，不能由脚本伪造。

Real-model smoke 明确命中预置问题：

`IllegalStateException("always fails after successful create")`

最终 Runtime 结果：

- `execution = COMPLETE`
- `coverage = COMPLETE`
- `reviewConclusion = BLOCKING`
- report SHA256：`450ea44f5bb4da38f1ecc97016db2a82817fa4eb1a050efe0adf9c816a4e9e28`

正式日志记录：

`REVIEW180_REAL_MODEL PASS model=task15secret/deepseek-v4-pro runId=review-242e2da0eb53fd36da2ffdf6256d1f64 actions=['prepare', 'finish'] findings=2 seededIssue=true conclusion=BLOCKING reportSha256=450ea44f5bb4da38f1ecc97016db2a82817fa4eb1a050efe0adf9c816a4e9e28`

其中日志中的 Tool action 摘要记录 `prepare` 与 `finish`；保留 trajectory 中另外存在 4 次真实 source `read`，因此完整自主路径记为 `prepare → read ×4 → finish`。

## 4. Artifact 与持久化结果

`review180-real-model-acceptance` 为 Task 3 正式 real-model acceptance artifact。验收要求的保留内容包括：

- `trajectory.json`
- `review.md`
- `result.json`
- `scope.json`
- `run.json`
- `manifest.json`

Artifact 用于证明真实模型轨迹、正式报告、结果、选择范围与 run 元数据来自同一自主闭环；`manifest.json` 用于固化文件 SHA-256。正式 report SHA256 与 Runtime finish 返回值一致：

`450ea44f5bb4da38f1ecc97016db2a82817fa4eb1a050efe0adf9c816a4e9e28`

## 5. Task 3 已验证能力

当前 testedProductHead 已验证：

- 正式 Review 入口进入 1.8 report-first 流程。
- single-chain Native Host 可执行 `prepare → finish`。
- multi-chain Native Host 必须经过真实下一用户回复完成选择，验证 `prepare → select → finish`。
- concurrent finish 使用不同请求时，Runtime 只允许一个结果完成，另一个请求被正式拒绝。
- finding 必须受 selected-chain 的精确结构边界约束；同一物理行中的未选 Java sibling method / Mapper sibling statement 不能借共享行号进入正式报告。
- Java method declaration 定位支持合法的 `void create ()` 空格写法，并忽略普通注释中的 `create()` 文本；overload 歧义 fail closed。
- real-model single-chain 有问题样本可自主完成 `prepare → read ×4 → finish`，并真实命中 seeded issue。
- 完成结果、coverage、reviewConclusion、report path/hash 形成持久化闭环。

## 6. Known limits / 未验证范围

以下内容不属于当前 Task 3 已完成证明范围，不得由本验收记录扩大宣称：

1. **真实模型矩阵**：当前正式 real-model acceptance 只证明 `task15secret/deepseek-v4-pro` 的 single-chain 有问题样本自主闭环。其他私有模型、其他 provider、无问题场景以及更完整的真实模型重复矩阵未在本 Task 3 证据中证明。
2. **真实模型 multi-chain 矩阵**：multi-chain 的“真实下一用户选择”已由原生 Native Host fixture 验证；完整 real-model multi-chain 重复矩阵属于后续最终验收范围，本记录不宣称已覆盖。
3. **Task 4 范围**：老板汇报版式、Task 4 扩展报告呈现以及对应历史问题回归不属于本次验收；当前不得进入 Task 4。
4. **最终 Windows 发布/升级认证**：正式候选包、离线升级、发布与最终 Certification 不属于 Task 3，本记录不宣称已经完成。
5. **跨任务能力**：本记录只接受 Task 3 主 Agent 自主评审与正式报告闭环，不替代后续 Task 的独立测试和验收。

这些限制不否定上述 Task 3 exact-head 验证结果，但后续任务不得直接复用本记录去证明尚未覆盖的能力。

## 7. 验收状态

研发自验证：**PASS。**

依据 testedProductHead `05b1bf27ee21bbe94198c0a7d2c1f29af09f9943` 的正式 Windows Runtime Regression、Native Host 与 real-model evidence，Task 3 当前代码与测试已完成研发侧自验证。

独立验收：**尚待验收人确认。**

本文件只记录研发侧已完成的正式验收证据，不替代独立验收人的最终 ACCEPTED 决定。

Task 4：**未进入。**  
PR #56：**保持未合并。**
