---
description: Codea Harness 1.8 primary code review with durable report
agent: orchestrator
subtask: false
---

!`./.code-harness/bin/codea-dcep-tools.exe review start`

# Codea Harness 1.8 Review

Review target: $ARGUMENTS

The command output above is the authoritative `review start` result. It has already created and read back the durable **INCOMPLETE** report before this model turn. Keep its `runId` and `reportPath`; never discover a run by taking the newest directory.

## Zero-target and Host identity invariants (mandatory)

- `/harness-review` with **NO arguments** is a valid primary Review command. Never require the user to supply a Controller/class/method before calling prepare. Empty arguments mean `intent={mode:"CHANGES"}` with **no target key**. Do not guess or fabricate a target from the repository.
- `/harness-review OrderController` and `/harness-review OrderController.method` are optional filters, not prerequisites. If there is no relevant Git change or the graph is unresolved, show the concrete Runtime gaps, keep the SAME INCOMPLETE run, and do not turn this into a request for a mandatory target.
- The entrypoint's pre-model `review start` is the **only** shell Runtime invocation authorized in this slash command. After it, **always** use the native OpenCode `codea-review` tool for `prepare`, `resume`, `select` and `finish`. NEVER execute `codea-dcep-tools.exe review prepare/select/finish` through bash, PowerShell, terminal, exec, or a generated script.
- Especially for `select`: native `codea-review` obtains `sessionID`/`messageID` from the actual OpenCode Host context; shell CLI invocations cannot provide the required authenticated user-turn evidence. Never invent IDs, extract another session's IDs, or ask the user to supply them. If the native tool is not advertised or cannot be invoked, fail closed with `CODEA_REVIEW_HOST_TOOL_UNAVAILABLE`, keep the INCOMPLETE report and instruct the user to verify `.opencode/tools/codea-review.ts` and restart OpenCode in a new session. Do not offer a shell workaround.
- When selection is required, display `requiredMenuText` verbatim, STOP this assistant turn, and wait for the user's new message `C1`, `选择C1`, `选择 C1`, or the comma-separated Controller IDs. Runtime verifies the literal real Host user message. Call the native `codea-review` with `action=select` in that next assistant turn, in the same OpenCode conversation.

Use only the `codea-review` structured tool for the ordinary 1.8 review path.
The Runtime CLI accepts both `review prepare --run-id <id> --mode CHANGES` and
`review prepare --run-id <id> --intent CHANGES`; conflicting flags are rejected.
For NO_RELEVANT_CHANGES, retain the SAME runId and the durable INCOMPLETE report;
no replacement `review start` is required. When sources change, rerun prepare on
that original runId. Never switch to CURRENT_IMPLEMENTATION automatically.



## Assistant-turn execution control

A completed `prepare` or `select` tool call is **not** itself a completed assistant turn.

- If the returned `nextAction.type` is `READ_SCOPE_AND_FINISH_THIS_TURN`, this assistant turn is non-terminal. Do **not** emit a user-facing answer and do **not** stop generation after prepare/select. Continue immediately with authorized source reads, perform the semantic review, and call `codea-review finish` in the same assistant turn. This rule is identical when the correct finding list is empty.
- The only normal pre-finish terminal state is `WAIT_FOR_REAL_USER_SELECTION`. In that state copy `nextAction.requiredMenuText` **verbatim** as one plain-text/code block. The first line must remain exactly `<runId> options=<optionsHash>`, followed by the exact `C<n> <name>` lines in returned order. Do not turn this block into a Markdown table or rewrite the header. Then end the turn exactly because a real next user message is required.
- `KEEP_REPORT_INCOMPLETE` or an actual tool/runtime error may end the turn with the concrete failure state.
- Therefore a single-chain, scope-ready review has only one valid normal terminal tool path: `prepare → source reads → finish`. An assistant message produced after only `prepare` is a protocol violation, even if no issue appears obvious.

1. Call `codea-review` with `action=prepare`, this exact `runId`, and intent `CHANGES` by default. Pass the user target as `intent.target` only when it is non-empty. Use `CURRENT_IMPLEMENTATION` **only** when the user's request itself explicitly asks to inspect the current implementation rather than current changes. Never switch or retry from `CHANGES` to `CURRENT_IMPLEMENTATION` because CHANGES returns no relevant source changes, `REVIEW_TARGET_NO_RELEVANT_CHANGES`, an incomplete scope, or any other prepare failure. Those states must remain fail-closed with the durable report INCOMPLETE.
2. If prepare reports multiple Controllers and `selectionRequired=true`, present the complete current menu exactly as `optionsHash` plus one line per chain in the form `<id> <name>`. Explain any gaps without claiming they are absent chains. **End this assistant turn. Do not call select or finish.**
3. On the next real user message, call `codea-review` with `action=select`, the same `runId`, the current `optionsHash`, and only the Controller IDs the user explicitly selected. `C1`, `选择C1` and `选择 C1` are all accepted; never demand a fixed prefix. The tool obtains Host session/message identity from OpenCode context; never ask the user or model to provide those IDs.
4. When prepare auto-selects one Controller (with its exact affected method subchains), or select succeeds, use the returned `scope.reads` as the authority for source reading. Review only that selected scope. Do not add files or ranges that are not present in the scope. After scope is available, do not inspect `.git/**`, `opencode.json`, `.opencode/**`, `.code-harness/**`, prompt/agent/tool implementation, or unrelated project metadata; do not use glob/grep to widen the scope. Read only the authorized source ranges needed for evidence.
5. Perform the code review in this primary Agent. Record only source-grounded findings. Every finding must include severity, problem, impact, recommendation, verification, and evidence whose exact `ref` was actually read. Do not modify production or test code.
6. `pendingRisks` is only for a **current unresolved risk in the selected implementation**: the current source must support a plausible harmful condition, but confirming whether it actually applies requires evidence outside the authorized scope. Do not add a pending risk for hypothetical future edits, future endpoint repurposing, "someone could later bind sensitive SQL", generic best-practice reminders, or policy/configuration that the current implementation does not depend on. If the selected current code has no finding and no such current unresolved harmful condition, send `pendingRisks=[]`.
7. Once the selected source ranges have been read and findings are source-grounded, immediately call `codea-review` with `action=finish`, even when findings are empty. Do not spend extra turns reverse-engineering Harness/Git internals or re-discovering Runtime authority. Send every actual read, findings, pending risks, and gaps. Only the finish result may authorize the words “评审完成”.
8. If coverage is `PARTIAL`, preserve the disclosed gaps and treat `reviewConclusion=UNDETERMINED` as intentional. Never convert partial coverage into `NO_ISSUES_FOUND`.
9. After successful finish, tell the user the report path and concise conclusion. If any tool call fails, report the concrete error and leave the already-created report INCOMPLETE.

## 辅助对话后的安全恢复（必须）

- Controller 菜单展示后，若下一条真实用户消息是明确 Controller 选择，直接调用原生 `codea-review select`；不要求用户输入固定“选择”前缀，不从模型文本伪造选择。
- 若是辅助问题或确认而非选择，先回答该问题；然后在同一 assistant turn 调用原生 `codea-review` 的 `action=resume`，使用 **同一 runId**，读取其 fresh `nextAction.requiredMenuText`，原样重展完整 Controller 菜单并结束 turn，等真正的下一条用户选择。禁止重新 `review start`。
- `resume` 使用存储的原始 intent 在 Runtime 重新核验源码指纹、Controller optionsHash 和 CHANGES Git snapshot。若已变化则生成新菜单，旧菜单必然失效，不允许复用旧 hash/IDs；若未变化则保留原 runId/hash。
- Runtime 继续严格检查 Host session/message 身份及 user reply，辅助消息本身不得授权 select。工具不可用时保持 INCOMPLETE，不使用 shell 绕过。
- Controller 分组只折叠 UI：CHANGES 仍须精确到受影响方法；选中 Controller 后必须覆盖其全部受影响方法子链；部分未解析就 PARTIAL。
- 说明性外部信息与 Runtime 覆盖缺口分别保存，不能掩盖 CRITICAL/HIGH findings。

## 历史隔离

The historical certification, ReviewUnit, RuleDispatch/rule-dispatch, reviewer-attestation, `codea-reviewer-submit`, `analysis certify`, `REPORT SUCCEEDED`, and legacy report pipeline are not part of the ordinary 1.8 path above. Do not call `review finish` directly from shell; finish is owned by the structured tool.
