# Review Host Contract — 1.6.6

The primary Agent owns semantic analysis and Finding proposals. Runtime owns canonical Git facts, coverage/evidence certification, scope, progress and report rendering. No Reviewer child session is required.

The installed tool retains the name `codea-reviewer-submit` for upgrade compatibility. `kind=change-analysis|findings|selection` writes only canonical same-run request files plus a receipt. Main-session receipts use version 2; Runtime verifies the primary session (no parent), assistant/agent identity, current user parent, completed tool call, runId, kind and exact JSON proposal. Receipt fields and file hash alone do not establish authority. Session/message IDs remain opaque. Export uses native process arguments, never shell interpolation.

For multi-chain selection, the complete Runtime-generated `selectionPrompt` must be displayed before the next real user turn and include runId and optionsHash. Accepted exact replies: `选择 C1`, `选择 C1,C2`, `全部` (FULL intent only), `仅列出`. The primary Agent submits the matching SelectionRequest using kind=selection, then invokes Runtime review select. Runtime independently rebuilds options and rechecks this attestation downstream. Synthetic user text, Agent self-selection, stale menus, missing scope, and LIST cannot authorize Findings.

Legacy version 1 receipts remain supported for older releases with their original independent Reviewer child-session attestation. The 1.6.6 CLI requires version 2, binds the verified primary session ID into the analysis certificate, and requires subsequent selection and Findings to use that same session and project directory. Packaged Reviewer resources remain for historical commands; the normal review route never invokes them.

Only requests/** may be Agent-authored. analysis/**, review.md and chains/** remain Runtime-owned. Invalid/missing Host submission fails closed; do not hand-write authority JSON or replace certified artifacts.
