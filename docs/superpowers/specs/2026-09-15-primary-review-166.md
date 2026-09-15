# 1.6.6 primary Agent review

Approved scope: main Agent owns semantic ChangeAnalysis and Findings; Runtime keeps canonical Git facts, coverage/evidence certification, scope and report rendering. No mandatory Reviewer child session. Preserve 1.6.5 and ship a new Windows upgrade.

Reuse the installed submission tool name `codea-reviewer-submit` for upgrade compatibility. Version 2 receipts attest the primary OpenCode session and completed submission, without requiring agent=reviewer. Legacy version 1 receipts retain their independent-child validation for old releases; 1.6.6 CLI requires version 2 and binds the primary session into the analysis certificate for later selection/Findings. No hand-written authority receipt bypass.

For 1.6.6, multi-chain selection requires an actual primary-session user turn after the complete Runtime-generated selectionPrompt displaying every option, exact paths, current runId and optionsHash. Accepted replies are `选择 C1,C2`, `全部`, or `仅列出`. The submission tool records selection; Runtime verifies the exact completed call, user turn, current options and resulting scope. Missing scope or LIST cannot become FULL. Downstream consumers revalidate selection; report scope is derived from the confirmed Runtime artifact, including exact chain refs. Single-chain and zero-change reviews remain automatic.

Request ingress accepts one UTF-8 BOM, without rewriting certified artifacts. Malformed JSON, invalid encodings and altered evidence still fail closed.

Acceptance: primary-session analysis and nonempty findings reach report; child/synthetic/stale/self-selected multi-chain requests fail; exact selected branch persists; actual Windows 1.6.5→1.6.6 upgrade preserves project state; real ast-grep, full Go tests and vet pass.
