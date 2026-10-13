---
description: Codea Harness 1.8 primary code review with durable report
agent: orchestrator
subtask: false
---

!`./.code-harness/bin/codea-dcep-tools.exe review start`

# Codea Harness 1.8 Review

Review target: $ARGUMENTS

The command output above is the authoritative `review start` result. It has already created and read back the durable **INCOMPLETE** report before this model turn. Keep its `runId` and `reportPath`; never discover a run by taking the newest directory.

## 自动评审范围与原生工具边界

- `/harness-review` 没有参数时，`intent={mode:"CHANGES"}`（没有 target），自动评审**全部本次 Git 变更实际影响的 Controller**及其方法级下游调用链，不做 C1/C2/C3 人工选择。没有有效变更时保持 INCOMPLETE，不能自行改成全仓当前实现。
- `/harness-review OrderController` 仅评审该 Controller 的受影响方法和下游；`OrderController.method` 可进一步限定方法。对于 Maven 同名 Controller，优先使用完整 Java 文件路径指定目标以消除歧义。不得因同名误合并或扩大范围。
- 用户明确提出“当前实现”时才使用 `CURRENT_IMPLEMENTATION`。Use `CURRENT_IMPLEMENTATION` **only** when the user's request itself explicitly asks to inspect the current implementation. Never switch or retry from `CHANGES` to `CURRENT_IMPLEMENTATION` after `REVIEW_TARGET_NO_RELEVANT_CHANGES`, `NO_RELEVANT_CHANGES` or any prepare failure. CHANGES must remain fail-closed with the durable report INCOMPLETE.
- 唯一允许的 Shell Runtime 调用是命令最开始的 `review start`。此后 `prepare`、授权源码 `read` 和 `finish` 必须通过**原生** OpenCode `codea-review` 工具调用；不得通过 PowerShell、bash、脚本或伪造 Host 身份调用 Runtime。若原生工具未加载，报告 `CODEA_REVIEW_HOST_TOOL_UNAVAILABLE`，保持 INCOMPLETE。
- **不再支持普通 Review 的人工菜单、select、resume 或要求用户输入“选择 C1”**。内部 `C1` 等 ID 仅用于自动 scope 的稳定身份与报告归属，不能向用户索要选择。即使受影响 Controller 有多个，也应自动包含全部，不创建新 run。
- Runtime 负责源码/Git 快照、optionsHash、所有子链范围及未解析项校验。Agent 不得补读 scope 外方法来掩盖归因缺口；`PARTIAL` 必须对应 `UNDETERMINED`；模型说明性 gap 不得误降级真实 `COMPLETE/BLOCKING`。

## Assistant-turn execution control

`prepare` 返回已授权 `scope` 后，主 Agent **non-terminal**：必须在同一 turn 中读源码、评审并 `finish`。返回 `nextAction.type=READ_SCOPE_AND_FINISH_THIS_TURN` 是强制续行，不得在 prepare 后先给用户自然语言答复或结束 turn；没有 finding 也必须 `finish`。

1. 使用本次 `review start` 的 exact `runId`，通过 `codea-review action=prepare` 执行 `CHANGES`；只有用户提供 Controller 时传 `intent.target`。
2. `prepare` 的 `options.chains` 按 Controller 分组，仅供查看本次自动纳入的范围；实际源码范围以 `scope.reads` 为准，报告 finding 证据仍受 `findingReads` 方法级行范围限制。多个 Controller 也不得展示人工选择菜单或等待用户回复。
3. 如果 `nextAction.type=KEEP_REPORT_INCOMPLETE`，说明没有有效范围或安全条件不足，显示 Runtime 具体原因，保留现有 run 的 INCOMPLETE 报告。不能伪造完成态。
4. 仅通过 `codea-review action=read`，传入 `scope.reads` 中完整、原样的 `path/sha256/startLine/endLine` 元组读取源码；禁止内置 `read/glob/grep/list/bash`，拒绝任何范围外读取、路径枚举及试探。禁止为了扩大范围探索 `.git/**`、`opencode.json`、`.opencode/**`、`.code-harness/**`、Prompt 和无关元数据。用户指定 Controller 不允许混入其他 Controller，未指定则自动覆盖**所有已发现且受变更影响**的 Controller。
5. 每项 finding 必须提供 severity、problem、impact、recommendation、verification、以及实际读取过且被 Runtime 许可的 evidence。保持数据库说明性 gap 的既有安全语义，不要修改业务代码。
   - **introducedByChange=true requires proof**：该 finding 自身对应的缺陷必须能被真实 Git changed lines 证明为本次引入；Controller/方法被 CHANGES 纳入评审，不代表其所有发现都是新引入的。未修改的 Controller 鉴权注解缺失、租户隔离缺失等历史问题，即使下游 SQL 本次修改，也必须 **introducedByChange=false**。证据不足时省略这个可选字段或明确为 false，不能猜测为 true 再靠失败重试修正。Runtime 的 REVIEW_FINISH_CHANGE_ATTRIBUTION_UNPROVEN 必须保持严格拒绝。
6. `pendingRisks` 只描述 current unresolved risk in the selected implementation。Do not add a pending risk for hypothetical future edits, future endpoint repurposing, someone could later bind sensitive SQL, or generic best-practice concerns. 不存在现实待核实风险时传 `pendingRisks=[]`。
7. 不论是否发现问题都通过原生 `codea-review action=finish` 提交 reads、findings、pendingRisks、gaps。若模型已读取更精确 evidence ref，必须把该 ref 原样纳入 result.reads。仅 `finish` 返回 COMPLETE 后才允许宣布评审完成。

### 证据引用严格规则（CI #195 永久修复）

- **single physical source line（单个源码物理行）**：每条 `evidence.quote` 必须从本轮 `read` 返回的源码行原样复制连续片段；`ref.startLine = ref.endLine` 为该行真实行号，SHA/路径与授权范围一致。允许在同一 finding 中添加多条独立单行证据来支撑跨行结论。
- **严禁模型重排代码块**：不得对多行 Java/XML 片段去缩进、重缩进、合并行或改写空白；不得把几行拼接成一条 quote。Runtime 只兼容 CRLF/LF 的换行差异，并不会忽略原文缩进。
- 如果 `finish` 返回 `REVIEW_FINISH_EVIDENCE_QUOTE_MISMATCH`，重新读取失败文件的已授权精确源码行，替换不匹配的 quote；**仅修改 ref 行号无法修复被改写的 quote**。不能删除 evidence、放宽校验或绕开 Runtime。

## 历史隔离

The historical certification, ReviewUnit, RuleDispatch/rule-dispatch, reviewer-attestation, `codea-reviewer-submit`, `analysis certify`, `REPORT SUCCEEDED`, and legacy report pipeline are not part of the ordinary 1.8 path above. Do not call `review finish` directly from shell; finish is owned by the structured tool.
