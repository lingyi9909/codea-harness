# Codea Harness 1.6.2 Hotfix Task 1 Canonical ChangeSet Authority Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:test-driven-development and superpowers:verification-before-completion. Execute only Task 1; do not enter Hotfix Task 2.

**Goal:** Make Runtime the single authority for deterministic Git ChangeSet facts and bind Certified ChangeAnalysis to a Runtime-owned canonical snapshot.

**Architecture:** Add a canonical `changeset.Snapshot` containing resolved Git identity plus filtered Review-scope files. `analysis snapshot` publishes the Runtime-owned snapshot. The Agent consumes that artifact for semantic reasoning and submits only semantic proposal data; certification reloads the snapshot, re-resolves the requested ref, recomputes current Git state, compares canonical snapshot identity, then assembles deterministic reviewScope/changedFiles from Runtime facts before semantic evidence validation.

**Tech Stack:** Go 1.23.x, Git CLI, JSON Schema Draft 2020-12, GitHub Actions Windows runner, PowerShell regression scripts.

**Spec:** User-approved 2026-09-02 Codea Harness 1.6.2 Post-release Hotfix / Task 1 requirements.

## Global Constraints

- Base commit: `e737a3b6e77af07df14de4f887ad0b8c9dddcf03`.
- Do not change Review semantic, Finding semantic, Finding Certification semantic, Rule Pack, ReviewUnit business semantics, Workspace Dependency semantics, Chain semantics, Business Flow Review, UI/CLI product behavior.
- Runtime owns deterministic facts; Agent owns semantic reasoning.
- `requestedBaseRef` is provenance only; Git identity is resolved commit + merge base + HEAD + working-tree state + canonical files.
- Stop after Task 1 and hand off for user acceptance.

---

### Task 1A: Canonical Snapshot Model and Compute

**Files:**
- Modify: `.code-harness/tools-runtime/internal/changeset/model.go`
- Modify: `.code-harness/tools-runtime/internal/changeset/git.go`
- Test: `.code-harness/tools-runtime/internal/changeset/canonical_snapshot_162_hotfix_test.go`

**Produces:** `Snapshot{requestedBaseRef,resolvedBaseCommit,mergeBase,headCommit,currentBranch,includeWorkingTree,files,snapshotSha256}` with deterministic identity hash that excludes provenance spelling.

- [ ] Write failing tests for zero diff, working tree, mixed Review/non-Review files, multi-source, includeWorkingTree=false, multi-module paths, equivalent refs, distinct refs.
- [ ] Verify RED on Windows CI.
- [ ] Implement resolved ref/current branch/merge base capture and canonical identity hashing.
- [ ] Verify focused and full changeset tests GREEN.

### Task 1B: Runtime-owned Snapshot Artifact

**Files:**
- Modify: `.code-harness/tools-runtime/cmd/codea-dcep-tools/analysis_command.go`
- Test: `.code-harness/tools-runtime/cmd/codea-dcep-tools/analysis_snapshot_162_hotfix_test.go`

**Produces:** `analysis snapshot --input .code-harness/runs/<runId>/requests/<file>.json` and `.code-harness/runs/<runId>/analysis/change-set.json`.

- [ ] Write failing command test.
- [ ] Verify RED.
- [ ] Implement same-run request validation, Compute, canonical artifact write, deterministic status output.
- [ ] Verify GREEN.

### Task 1C: Semantic Proposal + Certification Snapshot Binding

**Files:**
- Create: `.code-harness/contracts/change-analysis-proposal.schema.json`
- Modify: `.code-harness/contracts/change-analysis-cert.schema.json`
- Modify: `.code-harness/tools-runtime/internal/analysis/model.go`
- Modify: `.code-harness/tools-runtime/internal/analysis/certify.go`
- Modify: `.code-harness/tools-runtime/internal/analysis/certificate.go`
- Test: `.code-harness/tools-runtime/internal/analysis/canonical_snapshot_authority_162_hotfix_test.go`
- Test: `.code-harness/tools-runtime/cmd/codea-dcep-tools/analysis_certify_test.go`

**Produces:** Agent semantic proposal referencing snapshot paths for semantic roles; Runtime assembles authoritative reviewScope and changedFiles from snapshot and rejects stale/mutated Git state.

- [ ] Write failing tests for forged currentBranch/baseCommit/mergeBase, stale snapshot, equivalent base-ref identity and Runtime-owned changedFiles.
- [ ] Verify RED.
- [ ] Implement canonical certification path while retaining explicit legacy compatibility only where required by retained regressions.
- [ ] Verify semantic evidence behavior is unchanged.

### Task 1D: Agent Consumption Contract

**Files:**
- Modify: `.code-harness/AGENTS.md`
- Modify: `.code-harness/agents/reviewer.md`
- Modify: `.code-harness/agents/orchestrator.md`
- Modify: `.code-harness/skills/analyze-change/SKILL.md`
- Modify: `.code-harness/tools/README.md`

**Produces:** Active Review flow starts with Runtime canonical snapshot; Agent does not independently generate base/head/branch/includeWorkingTree/changedFiles sources.

- [ ] Replace Agent-owned deterministic ChangeSet assembly instructions with snapshot consumption.
- [ ] Keep semantic navigation/coverage/role reasoning unchanged.

### Task 1E: Regression and Exact-head Windows Gate

**Files:**
- Create: `.github/workflows/task162-hotfix-task1-canonical-changeset.yml`
- Create: `.github/scripts/task162-hotfix-task1-canonical-changeset-regression.ps1`

- [ ] Run focused canonical snapshot regression.
- [ ] Run `go test -count=1 ./...`.
- [ ] Run `go vet ./...`.
- [ ] Run retained 1.6 Review Precision regression.
- [ ] Run retained 1.6.1 regression gates.
- [ ] Run retained 1.6.2 Maven multi-module regression.
- [ ] Bind final marker to exact Git HEAD.
- [ ] Stop and hand off Task 1 evidence; do not enter Task 2.
