# Codea Harness 1.6.2 Task 2 Fresh Review Lifecycle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every new top-level `harness review` invocation create a fresh Runtime-owned runId and recompute the Canonical Snapshot, even inside the same OpenCode session.

**Architecture:** Add a minimal `codea-dcep-tools.exe review begin` lifecycle entry that only creates a unique Review run directory and returns its runId. Update active Agent contracts so every new top-level `harness review` must call `review begin` before `analysis snapshot`; `same-run` remains strictly intra-invocation. `analysis snapshot` remains the only Git ChangeSet Authority and no existing Review/ChangeAnalysis/Finding semantics change.

**Tech Stack:** Go 1.23, PowerShell 7, Windows GitHub Actions, OpenCode 1.18.25, deterministic local OpenAI-compatible model stub.

**Spec:** User-approved Codea Harness 1.6.2 `harness review` Reliability Hotfix Task 2 — Fresh Review Lifecycle.

## Global Constraints

- Accepted Task 1 baseline is `1141a240529ea3fedcc8df0d3750db31f9fb1104`.
- Do not modify Canonical ChangeSet semantics, Certified ChangeAnalysis Authority, Finding Certification semantics, ReviewUnit semantics, Rule Pack, Chain semantics, or Review Scope/Selection semantics.
- `review begin` must not inspect Git or compute ChangeSet.
- Every new top-level `harness review` must use a new runId and then execute `analysis snapshot` for that run.
- `same-run` means only artifacts inside one Review invocation share one runId.
- Task 3 README generation is out of scope.

---

### Task 1: RED — Fresh lifecycle contract and same-session regression

**Files:**
- Create: `.code-harness/tools-runtime/cmd/codea-dcep-tools/review_reliability_task2_fresh_lifecycle_test.go`
- Create: `.github/scripts/task162-review-reliability-task2-contract-regression.ps1`
- Create: `.github/workflows/task162-review-reliability-task2-red.yml`

**Interfaces:**
- Consumes: existing `run([]string)` / `runReview160([]string)` command routing.
- Produces: failing expectations that `review begin` exists, returns distinct valid runIds, creates run directories, and active contracts require begin-before-snapshot.

- [ ] **Step 1: Write failing Go tests**
  - `review begin` accepts no arguments.
  - Two calls return different non-empty runIds.
  - Each returned runId has `.code-harness/runs/<runId>/` immediately created.
  - No `analysis/change-set.json` exists after begin alone.

- [ ] **Step 2: Write failing active-contract regression**
  - `.code-harness/AGENTS.md`, `.code-harness/tools/README.md`, and `.code-harness/agents/orchestrator.md` must expose `review begin`.
  - Plain `harness review` must say each new top-level invocation starts with fresh `review begin` then fresh `analysis snapshot`.
  - Previous runId/Snapshot/ChangeAnalysis/review.md/zero-change conclusion must be non-authoritative for a new invocation.

- [ ] **Step 3: Run RED Windows gate**
  - Expected failure: unknown `review begin` and/or missing lifecycle contract.

---

### Task 2: GREEN — Minimal Runtime `review begin`

**Files:**
- Modify: `.code-harness/tools-runtime/cmd/codea-dcep-tools/review_precision_command.go`
- Test: `.code-harness/tools-runtime/cmd/codea-dcep-tools/review_reliability_task2_fresh_lifecycle_test.go`

**Interfaces:**
- Produces: `codea-dcep-tools.exe review begin` → JSON `{status:"READY", runId:"...", runPath:".code-harness/runs/<runId>"}`.
- Side effect: creates only `.code-harness/runs/<runId>/`.

- [ ] **Step 1: Route `review begin` through `runReview160`**
- [ ] **Step 2: Generate collision-resistant Runtime-owned runId using stdlib only**
- [ ] **Step 3: Atomically claim a new run directory; retry on collision**
- [ ] **Step 4: Do not create Snapshot or any authority artifact**
- [ ] **Step 5: Run focused Go tests and full Go regression**

---

### Task 3: GREEN — Active Agent lifecycle contract

**Files:**
- Modify: `.code-harness/AGENTS.md`
- Modify: `.code-harness/tools/README.md`
- Modify: `.code-harness/agents/orchestrator.md`
- Modify only if needed for current-run consumption wording: `.code-harness/agents/reviewer.md`

**Interfaces:**
- New top-level plain `harness review` flow begins `review begin → fresh runId → analysis snapshot`.
- Existing explicit targeted review must also create a fresh runId before its snapshot because it is a new top-level `harness review` invocation.

- [ ] **Step 1: Add `review begin` to the active fixed command set**
- [ ] **Step 2: Define fresh-invocation hard rule**
- [ ] **Step 3: Define previous-run artifacts and conversation memory as non-authoritative**
- [ ] **Step 4: Keep `same-run` explicitly intra-invocation**
- [ ] **Step 5: Run active-contract regression**

---

### Task 4: Real OpenCode same-session E2E

**Files:**
- Create: `.github/scripts/task162-review-reliability-task2/mock_openai_server.py`
- Create: `.github/scripts/task162-review-reliability-task2-real-agent-e2e.ps1`
- Create: `.github/workflows/task162-review-reliability-task2.yml`

**Interfaces:**
- OpenCode first call: plain `harness review` in a clean fixture → run-A → 0 Change.
- Same OpenCode session second call: plain `harness review` after fixture mutation → run-B → fresh Snapshot.

- [ ] **Step 1: Build stateful deterministic model that discovers runId from `review begin` output; never hard-code a global runId**
- [ ] **Step 2: Scenario A — same session, working-tree mutation**
  - First review produces 0 Change.
  - Mutate review-relevant working tree.
  - Continue exact same OpenCode session with `opencode run --continue` (or exact returned session id).
  - Assert run-A != run-B and Snapshot B contains the mutation.
- [ ] **Step 3: Scenario B — same session, HEAD mutation**
  - First review produces 0 Change.
  - Create a new Git commit whose state remains review-relevant against a stable base ref.
  - Continue same OpenCode session.
  - Assert a new runId and a fresh Snapshot reflecting the new HEAD/committed change.
- [ ] **Step 4: Assert no user hint such as “代码变化了” is present**
- [ ] **Step 5: Retain Task 1 and Task 3 real-agent regressions**

---

### Task 5: Fresh exact-head Windows certification

**Files:**
- Modify: `.github/workflows/task162-review-reliability-task2.yml` only as needed for final gate.

- [ ] **Step 1: Run focused Task 2 tests**
- [ ] **Step 2: Run `go test -count=1 ./...`**
- [ ] **Step 3: Run `go vet ./...`**
- [ ] **Step 4: Build exact-head Windows Runtime**
- [ ] **Step 5: Run Task 2 same-session working-tree E2E**
- [ ] **Step 6: Run Task 2 same-session HEAD E2E**
- [ ] **Step 7: Run retained Task 1 complete invocation contract E2E and Task 3 real plain review E2E**
- [ ] **Step 8: Verify checkout HEAD equals `${{ github.sha }}`**
- [ ] **Step 9: Stop at Task 2 awaiting human acceptance; do not enter Task 3**
