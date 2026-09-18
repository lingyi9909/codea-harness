# Codea Harness 1.8 T3 Acceptance Blocker Repair Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix only the five Task 3 acceptance blockers found at `e3478fa8df653b11baccc598faed70a1a987c8b1`, preserving the accepted T2 baseline and stopping before T4.

**Architecture:** Keep the report-first T3 architecture. Make finish requests invocation-unique at the TypeScript tool boundary; split context read scope from finding-evidence scope in Runtime; validate change attribution from the run intent plus real Git diff; remove legacy ordinary-review procedures from active instruction surfaces; and make the real-model smoke emit reproducible, redacted acceptance artifacts whose hashes are cross-checked against Runtime output.

**Tech Stack:** Go 1.23 Runtime/tests, TypeScript OpenCode tool, Python native-Host/real-model smoke, GitHub Actions Windows x64.

**Spec:** `docs/tasks/codea-harness-1.8/task-3-primary-agent-report.md`

## Global Constraints

- Base remains Task 2 accepted exact HEAD `db2f03cebafa6f72c733286a083ad97e14f41cf5`; repairs stay on `feature/1.8-task3-primary-report`.
- Do not enter Task 4 and do not merge PR #56.
- `/harness-review` still synchronously creates an INCOMPLETE report with fixed `review start` before Agent work.
- Primary Agent still uses only structured `codea-review` prepare/select/finish; scripts never supplement finish.
- Multi-chain selection still requires the actual next user message and Host context.
- PARTIAL coverage remains `UNDETERMINED`.
- T1 durability/idempotency/concurrency and T2 selection semantics must remain green.

---

### Task 1: Isolate concurrent tool finish requests

**Files:**
- Modify: `.code-harness/tools/codea-review.ts`
- Modify: `.github/scripts/review180-host-smoke.py`
- Modify: `.code-harness/tools-runtime/cmd/codea-dcep-tools/review_entry_180_test.go`

**Interfaces:**
- Consumes: `review finish --input <repo-relative-json>` and existing Runtime per-run lock/idempotency.
- Produces: one immutable request file per finish invocation; two distinct concurrent payloads can never both be reported successful for the same run.

- [ ] Add a deterministic tool-layer concurrent-finish regression that issues two different finish payloads through the real TypeScript tool/real Runtime and asserts at most one COMPLETE result and no request-file cross-talk.
- [ ] Verify the regression fails against the shared `requests/finish.json` implementation.
- [ ] Change finish request naming to a Windows-safe invocation-unique name independent of opaque Host IDs, using exclusive creation/no overwrite semantics before Runtime invocation.
- [ ] Verify the regression and existing Host smoke pass.

### Task 2: Separate context read scope from formal finding scope

**Files:**
- Modify: `.code-harness/tools-runtime/internal/reviewrun/types.go`
- Modify: `.code-harness/tools-runtime/internal/reviewrun/prepare.go`
- Modify: `.code-harness/tools-runtime/internal/reviewrun/selection.go`
- Modify: `.code-harness/tools-runtime/internal/reviewrun/finish.go`
- Add/Modify tests under `.code-harness/tools-runtime/internal/reviewrun/*_180_test.go`

**Interfaces:**
- Consumes: selected `Chain.Nodes` and exact source/XML symbol locations discovered during prepare.
- Produces: `scope.json` with full context `reads` plus line-bounded `findingReads`; finish accepts evidence only when the quoted occurrence lies inside the selected chain's finding scope.

- [ ] Add RED regressions for two methods in one Controller, two selected/unselected methods in one shared Service, and two statements in one shared Mapper XML.
- [ ] Carry exact Java method and XML statement line ranges on chain nodes or equivalent persisted scope data.
- [ ] Keep `reads` broad enough for context, but persist separate `findingReads` for selected symbols only.
- [ ] Validate each finding evidence quote occurrence against `findingReads`; reject evidence that is readable context but belongs only to an unselected method/statement.
- [ ] Re-run T2 selection/source-set regressions to prove shared files remain readable without widening formal issue scope.

### Task 3: Validate introducedByChange from run intent and real diff

**Files:**
- Modify: `.code-harness/tools-runtime/internal/reviewrun/finish.go`
- Add/Modify tests under `.code-harness/tools-runtime/internal/reviewrun/*_180_test.go`

**Interfaces:**
- Consumes: persisted `preparedOptions180.Intent`, current source fingerprint, and real Git working-tree/index diff against HEAD.
- Produces: fail-closed change attribution: CURRENT_IMPLEMENTATION cannot claim introduced-by-change; CHANGES may claim it only when finding evidence intersects an actual changed/new-file line range.

- [ ] Add RED regression: CURRENT_IMPLEMENTATION + `introducedByChange=true` is rejected.
- [ ] Add RED regressions: CHANGES + true without diff-line evidence is rejected; true with evidence intersecting a real changed hunk is accepted; untracked file lines count as changed.
- [ ] Revalidate the prepared source snapshot at finish, load the run intent, derive changed line ranges from Git, and reject unsupported attribution.
- [ ] Preserve nil/false attribution without manufacturing a causal claim.

### Task 4: Remove legacy ordinary-review procedures from active instructions

**Files:**
- Modify: `.code-harness/AGENTS.md`
- Modify: `.code-harness/bootstrap.md`
- Modify: `.code-harness/agents/orchestrator.md`
- Modify: `.code-harness/skills/review-code/SKILL.md`
- Modify: `.code-harness/tools-runtime/cmd/codea-dcep-tools/review_entry_180_test.go`
- Create: `docs/history/codea-harness-pre-1.8-ordinary-review.md` only if legacy explanatory text must be retained.

**Interfaces:**
- Produces: each active instruction surface has exactly one executable ordinary Review flow: `review start` then `codea-review`; Test/Debug/Fix/API Doc/Chain/Upgrade rules remain available and unchanged in authority.

- [ ] Add a RED full-file regression that scans the complete active instruction surfaces, not only text before a `历史` heading, and rejects executable legacy ordinary-review commands/steps (`review begin`, ordinary `analysis certify`, old Reviewer/ReviewUnit/RuleDispatch/report-review completion flow).
- [ ] Move legacy ordinary-review explanation out of the active loaded files; do not merely add another priority banner.
- [ ] Preserve non-ordinary Review Test/Debug/Fix/API Doc/Chain/Upgrade contracts and links.
- [ ] Verify all active instruction files still expose 1.8 `codea-review` semantics and no executable legacy flow.

### Task 5: Make real-model evidence self-verifying and retained

**Files:**
- Modify: `.github/scripts/review180-real-model-smoke.py`
- Modify: `.github/workflows/runtime-regression-windows-x64.yml`
- Modify: `.code-harness/tools-runtime/cmd/codea-dcep-tools/review_entry_180_test.go`
- Create after a successful final run: `docs/verification/1.8/task-3-acceptance.md`

**Interfaces:**
- Consumes: OpenCode exported session/tool return and durable run files.
- Produces: redacted evidence directory containing model trajectory, `review.md`, `result.json`, `scope.json`, run state, manifest/hashes; GitHub Actions retains it as a T3 artifact.

- [ ] Add RED assertions that the real model finding specifically cites the injected `always fails after successful create` defect and its source location/quote.
- [ ] Parse the completed finish tool return and require its runId/reportPath/reportSha256 to match the actual durable report and computed SHA-256.
- [ ] Export a redacted model trajectory plus `review.md`, `result.json`, `scope.json`, state/run metadata and a SHA-256 manifest into a stable evidence directory; never include secrets.
- [ ] Upload that directory on success and failure with short retention suitable for acceptance review.
- [ ] Run fresh Windows Runtime Regression at the final exact HEAD and record run/job/artifact/model/actions/findings/conclusion/hashes in `docs/verification/1.8/task-3-acceptance.md`.
- [ ] Re-run after the evidence-record commit if needed so the recorded exact HEAD itself is green.

### Final verification

- [ ] Windows secret preflight PASS.
- [ ] ast-grep/bootstrap PASS.
- [ ] OpenCode native export smoke PASS.
- [ ] Review regression PASS.
- [ ] Chain regression PASS.
- [ ] `go test -count=1 ./...` PASS.
- [ ] `go vet ./...` PASS.
- [ ] Native Host single-chain/multi-chain/concurrent-finish regressions PASS.
- [ ] Real-model autonomous single-chain smoke PASS with seeded-defect assertion and retained artifact.
- [ ] Evidence record points to the final exact HEAD and matching Windows run/job/artifact.
- [ ] Stop for T3 re-verification; do not enter T4 and do not merge PR #56.
