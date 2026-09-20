# Review Run 目录说明

`.code-harness/runs/` 用来保存 Codea Harness 的**执行产物**。本 README 只用于帮助人阅读和定位一次 Review 的产物，**不是 Review Authority（权威来源），也不是任务状态或恢复机制**。

> 核心原则：一次新的顶层 `harness review`，就是一次新的 Review invocation。不要拿旧 Run 的结论替代新的 Review。

## 1. 每次 `harness review` 都从 fresh runId 开始

新的顶层 `harness review` 必须按下面的生命周期启动：

```text
harness review
  -> review begin
  -> fresh runId
  -> analysis snapshot
  -> inventory / semantic proposal / analysis certify
  -> review options / review scope / review units / rule dispatch
  -> finding proposals / certify-findings
  -> report review
  -> review.md
```

其中：

- `review begin` 只负责由 Runtime 创建一个新的 runId 和对应 Run 目录；它本身不读取 Git，也不生成 ChangeSet。
- `analysis snapshot` 必须在本次新 Run 中重新执行，它生成本次 Review 的 Canonical ChangeSet Snapshot。
- 同一次 invocation 内的后续阶段共享同一个 runId，这就是这里的 **same-run**；same-run 只表示“同一次 Review 内的产物绑定”，绝不表示后续顶层 `harness review` 可以复用旧 runId。
- 旧 runId、旧 Snapshot、旧 ChangeAnalysis、旧 `review.md`，以及旧 Run 的“无变更/0 问题”结论，对新的顶层 Review 都是**非权威**信息，不得复用来跳过 fresh `review begin` 和 fresh `analysis snapshot`。

## 2. 一个正式 Review Run 里有什么

典型目录结构如下：

```text
.code-harness/runs/<runId>/
├─ analysis/
│  ├─ change-set.json
│  ├─ entrypoint-inventory.json
│  ├─ change-analysis.json
│  ├─ change-analysis.cert.json
│  ├─ review-options.json
│  ├─ review-scope.json
│  ├─ review-units.json
│  ├─ rule-dispatch.json
│  ├─ certified-findings.json
│  └─ certified-findings.cert.json
├─ requests/
│  └─ ... Agent -> Runtime transport JSON ...
└─ review.md
```

下面是主要文件的含义。

| 文件 | 作用 |
|---|---|
| `analysis/change-set.json` | Runtime 生成的 Canonical ChangeSet Snapshot，是本次 Review 的 Git ChangeSet Authority。 |
| `analysis/entrypoint-inventory.json` | Runtime 认可的入口点清单，用于后续语义分析与 Coverage 认证。 |
| `analysis/change-analysis.json` | 经 Runtime certify 后发布的正式 ChangeAnalysis。 |
| `analysis/change-analysis.cert.json` | ChangeAnalysis 对应的机器认证信息。 |
| `analysis/review-options.json` | 本次 Run 的正式 Review Options。 |
| `analysis/review-scope.json` | Runtime 确认的 Review Scope。 |
| `analysis/review-units.json` | 本次 Review 要评审的 Review Units；0 变更时可以是空集合。 |
| `analysis/rule-dispatch.json` | 对 Review Units 的正式规则分发结果；0 变更时可以为空。 |
| `analysis/certified-findings.json` | Runtime 对本次 Agent finding proposals 认证后的 Findings。 |
| `analysis/certified-findings.cert.json` | Certified Findings 对应的机器认证信息。 |
| `review.md` | 本次 Run 面向用户的最终正式 Review 报告。 |

这些文件属于同一个 runId 的完整证据链。不要把不同 Run 的 Authority Artifact 混在一起使用。

## 3. `requests/**` 只是 transport，不是 Authority

`.code-harness/runs/<runId>/requests/**` 是 Agent → Runtime 的结构化请求传输区（transport）。例如 semantic proposal、Finding proposal、report request 会先通过这里交给 Controlled Runtime。

需要注意：

- `requests/**` **不是正式权威产物**，不能用 request JSON 替代 Runtime 发布的 `analysis/**` 产物。
- Runtime 成功消费某些 report transport 后，可以删除对应请求文件；请求文件不存在不等于正式 Review 结果丢失。
- Agent 不得直接修改正式 Authority Artifact 来“修正”或“让 Review 通过”；需要变化时，应重新走对应 Runtime contract。

## 4. 最终报告只看 `review.md`

Review 的最终正式报告是：

```text
.code-harness/runs/<runId>/review.md
```

`review.md` 由 Controlled Runtime 的 deterministic renderer 生成。不要让 Agent 自由拼接最终 Markdown，也不要创建或使用 `review.json` 作为正式 Review 报告。

报告结果主要有三种：

- `PASSED`：Coverage 完整，且没有阻断 Review 的已认证问题。
- `FAILED`：存在需要阻断通过的已认证 Findings。
- `MANUAL_ACTION_REQUIRED`：Coverage 为 PARTIAL，机器证据不足，需要人工处理后再决定下一步。

## 5. 0 变更不是“提前结束”

当 `analysis/change-set.json` 中 `changedFiles=[]`（也就是零变更/无变更）时，Review **仍然必须完成正式链路**，不能因为“没有代码变化”就直接在对话里返回成功。

0 变更 Run 仍需要继续完成：

```text
review units -> empty units
rule dispatch -> empty dispatch
finding-proposals = []
certify-findings -> empty certified findings + certificate
report review -> review.md
```

如果 Coverage 完整，最终应得到 `PASSED`，并在 `review.md` 中体现 **0 个变更文件 / 0 个问题**。

这条规则确保“无变更”也是一个可以审计的正式 Review Run，而不是 Agent 的会话记忆结论。

## 6. 新 Review 不读取旧 Run 来决定是否需要重算

例如：

```text
Run A: harness review -> 0 变更
随后工作区发生变化或 HEAD 产生新提交
Run B: 再次 harness review
```

Run B 必须创建自己的新 runId，并重新执行 `analysis snapshot`。即使当前 Agent session 中仍然保留 Run A 的聊天上下文，Run A 的“0 变更”结论也不得阻止 Run B 读取当前 Git 状态。

因此，排查 Review 结果时可以查看旧 Run 做历史参考，但**不能把旧 Run 作为新 Review 的 Authority**。

## 7. 不要手工修改正式产物

为了保持 Review 可验证：

- 不要手工修改 `analysis/change-set.json`、`analysis/change-analysis.json`、`analysis/certified-findings.json` 或其证书文件；
- 不要把不同 runId 的 Authority Artifact 复制到当前 Run；
- 不要为了得到 `PASSED` 而修改 `review.md`；
- 不要把 `.code-harness/runs/` 当作 Harness 的任务状态机或恢复数据库。

如果源码/Git 状态已经变化，请直接重新执行顶层 `harness review`，让 Harness 创建 fresh runId 并重新生成本次 Review 的完整证据链。
