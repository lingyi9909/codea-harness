---
description: Run a formal Codea Harness review in the current primary session
subtask: false
---

Runtime bootstrap (executed from the project root):

!`./.code-harness/bin/codea-dcep-tools.exe review begin`

Review target: $ARGUMENTS

Use the injected `READY` result and its fresh `runId` exactly once. Do not call
`review begin` again and do not reuse another run.

Read `.code-harness/AGENTS.md`, `.code-harness/agents/orchestrator.md`, and the
active `analyze-change` and `review-code` skills. Keep all semantic work in this
primary session; do not create a reviewer or other subagent. Create the Runtime
snapshot for this run, then inspect the requested class or method and its direct
dependencies with focused navigation. Preserve every snapshot fact and required
coverage; report unresolved evidence as partial instead of recursively scanning
the whole repository or calling incomplete work complete.

Submit the change-analysis proposal with `codea-reviewer-submit`, certify it,
and obtain Runtime review options. If Runtime requires user selection, show its
complete prompt and end the turn. Continue only from the user's next explicit
selection, submitted through the same tool and same `runId`. Then build review
units and rule dispatch, perform the semantic code review here, submit findings
(including an empty array when there are none), certify them, and request the
Runtime report.

After each stage, show Runtime progress clearly. Correct a preflight-rejected
proposal in this run without restarting. Stop on authority failures and avoid
retry loops. Do not ask to fix product code before the formal review finishes.
The review is complete only when this run has `review.md` and Runtime reports
`REPORT SUCCEEDED`.
