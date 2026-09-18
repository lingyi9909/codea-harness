---
description: Codea Harness 1.8 primary code review with durable report
agent: orchestrator
subtask: false
---

!`./.code-harness/bin/codea-dcep-tools.exe review start`

# Codea Harness 1.8 Review

Review target: $ARGUMENTS

The command output above is the authoritative `review start` result. It has already created and read back the durable **INCOMPLETE** report before this model turn. Keep its `runId` and `reportPath`; never discover a run by taking the newest directory.

Use only the `codea-review` structured tool for the ordinary 1.8 review path:

1. Call `codea-review` with `action=prepare`, this exact `runId`, and intent `CHANGES` by default. Pass the user target as `intent.target` only when it is non-empty. If the user explicitly asks to inspect the current implementation rather than current changes, use `CURRENT_IMPLEMENTATION`.
2. If prepare reports multiple chains and `selectionRequired=true`, present the complete current menu exactly as `optionsHash` plus one line per chain in the form `<id> <name>`. Explain any gaps without claiming they are absent chains. **End this assistant turn. Do not call select or finish.**
3. On the next real user message, call `codea-review` with `action=select`, the same `runId`, the current `optionsHash`, and only the chain IDs the user explicitly selected. The tool obtains Host session/message identity from OpenCode context; never ask the user or model to provide those IDs.
4. When prepare auto-selects one complete chain, or select succeeds, use the returned `scope.reads` as the authority for source reading. Review only that selected scope. Do not add files or ranges that are not present in the scope. After scope is available, do not inspect `.git/**`, `opencode.json`, `.opencode/**`, `.code-harness/**`, prompt/agent/tool implementation, or unrelated project metadata; do not use glob/grep to widen the scope. Read only the authorized source ranges needed for evidence.
5. Perform the code review in this primary Agent. Record only source-grounded findings. Every finding must include severity, problem, impact, recommendation, verification, and evidence whose exact `ref` was actually read. Do not modify production or test code.
6. Once the selected source ranges have been read and findings are source-grounded, immediately call `codea-review` with `action=finish`, even when findings are empty. Do not spend extra turns reverse-engineering Harness/Git internals or re-discovering Runtime authority. Send every actual read, findings, pending risks, and gaps. Only the finish result may authorize the words “评审完成”.
7. If coverage is `PARTIAL`, preserve the disclosed gaps and treat `reviewConclusion=UNDETERMINED` as intentional. Never convert partial coverage into `NO_ISSUES_FOUND`.
8. After successful finish, tell the user the report path and concise conclusion. If any tool call fails, report the concrete error and leave the already-created report INCOMPLETE.

## 历史隔离

The historical certification, ReviewUnit, RuleDispatch/rule-dispatch, reviewer-attestation, `codea-reviewer-submit`, `analysis certify`, `REPORT SUCCEEDED`, and legacy report pipeline are not part of the ordinary 1.8 path above. Do not call `review finish` directly from shell; finish is owned by the structured tool.
