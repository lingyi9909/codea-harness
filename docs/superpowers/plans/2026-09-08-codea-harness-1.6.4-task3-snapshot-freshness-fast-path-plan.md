# Codea Harness 1.6.4 Task 3 Snapshot Freshness Fast Path Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace canonical certify's full `changeset.Compute()` freshness recomputation with a Runtime-owned `changeset.VerifyFreshness()` path that proves the same Git identity and `GitStateSHA256` without rebuilding canonical `Files/Hunks/Snapshot`.

**Architecture:** Keep `changeset.Compute()` unchanged as the sole producer of canonical Snapshot facts. Add `VerifyFreshness(repoRoot, snapshot)` that re-resolves base/HEAD/merge-base/branch and recomputes the exact existing review-scoped committed/staged/unstaged patch hash plus untracked path/content hashes, then compares those identities against the sealed Snapshot. Canonical certify uses this verifier and continues consuming the already-decoded sealed Snapshot for downstream inventory/certification; any mismatch fails closed as `CHANGE_SET_SNAPSHOT_STALE`.

**Tech Stack:** Go 1.23.x, Git CLI, Windows GitHub Actions, existing `changeset`/`analysis` packages, pinned ast-grep 0.42.1 for downstream regression.

**Spec:** `docs/superpowers/specs/2026-09-07-codea-harness-1.6.4-analysis-certification-performance-hotfix-design.md`

## Global Constraints

- Task 2 Accepted Base: `d595cef1d7a2e121c2cd2749697628a6077dfcb3`.
- Formal branch: `hotfix/1.6.4-analysis-cert-performance-task3`.
- `Compute()` remains canonical Snapshot producer; fast path is verification only.
- Freshness must cover resolved base commit, merge base, HEAD, current branch, committed state, staged state, unstaged bytes/state, untracked membership and bytes.
- No `git status`-only shortcut and no path-name-only fingerprint.
- `GitStateSHA256` semantics remain exact; no new semantic authority or certificate hash input.
- Any freshness mismatch fails closed before Entrypoint Inventory / ReviewOptions.
- Task 1 bounded Entrypoint scope and pinned ast-grep 0.42.1 remain unchanged.
- Do not enter Final Certification until Task 3 is separately accepted.

---

### Task 1: Freshness Mutation Matrix

**Files:**
- Create: `.code-harness/tools-runtime/internal/changeset/freshness_164_test.go`
- Create: `.github/workflows/task164-task3-snapshot-freshness.yml`

**Interfaces:**
- Consumes: existing `Snapshot`, `Compute()` and Git test helpers.
- Produces: required API `func VerifyFreshness(repoRoot string, snapshot Snapshot) error`.

- [ ] **Step 1: Write failing mutation-matrix tests**

Cover a fresh pass plus stale rejection after: HEAD move; requested base ref move; branch change; committed-state change; staged bytes change; staged add/delete; unstaged bytes change; unstaged add/restore; untracked add/delete; untracked same-path bytes change; same path/same size/different bytes; staged+unstaged same-path mixed mutation.

- [ ] **Step 2: Run RED on Windows**

Run:

```powershell
go test -count=1 -v ./internal/changeset -run '^Test164VerifyFreshness'
```

Expected: compile/test failure because `VerifyFreshness` does not exist.

- [ ] **Step 3: Record exact RED run and HEAD**

The workflow must print exact `github.sha`; preserve run ID as acceptance evidence.

---

### Task 2: Implement Exact-Authority VerifyFreshness

**Files:**
- Create: `.code-harness/tools-runtime/internal/changeset/freshness_164.go`
- Modify only if needed: `.code-harness/tools-runtime/internal/changeset/git.go`
- Test: `.code-harness/tools-runtime/internal/changeset/freshness_164_test.go`

**Interfaces:**
- Consumes: sealed `Snapshot` from `Compute()`/`DecodeCanonical()`.
- Produces: `func VerifyFreshness(repoRoot string, snapshot Snapshot) error`.

- [ ] **Step 1: Re-resolve immutable Git identities**

Use the same 30s Runtime timeout and compare `RequestedBaseRef -> ResolvedBaseCommit`, `HEAD`, `merge-base`, and current branch against Snapshot values.

- [ ] **Step 2: Recompute exact existing Git state identity**

For committed/staged/unstaged state use the same Git diff arguments and existing `reviewScopedDiff153()` filtering as `Compute()`, but do not populate `File/Hunk` structures. For untracked state use `git ls-files --others --exclude-standard`, existing scope filtering, and exact content SHA-256; sort identically and call existing `gitStateSHA256162()`.

- [ ] **Step 3: Fail closed**

Any identity/state mismatch returns `CHANGE_SET_SNAPSHOT_STALE`; Git/runtime failures retain specific `CHANGE_SET_*` failure codes where possible and must never be treated as fresh.

- [ ] **Step 4: Verify mutation matrix GREEN**

Run:

```powershell
go test -count=1 -v ./internal/changeset -run '^Test164VerifyFreshness'
```

Expected: all cases PASS.

---

### Task 3: Canonical Certify Integration

**Files:**
- Modify: `.code-harness/tools-runtime/internal/analysis/certify.go`
- Modify: `.code-harness/tools-runtime/internal/analysis/certify_canonical_162.go`
- Create: `.code-harness/tools-runtime/internal/analysis/certify_freshness_fastpath_164_test.go`

**Interfaces:**
- `defaultCertificationRuntime153` gains production freshness verification without changing fake/test runtimes unnecessarily.
- Canonical certify continues downstream with the decoded sealed Snapshot after successful freshness verification.

- [ ] **Step 1: Write RED integration tests**

Prove production canonical certify invokes fast freshness, succeeds with unchanged Git state, rejects post-snapshot mutation before any certified analysis/inventory/cert writes, and does not require a second full `Compute()` to generate a replacement Snapshot.

- [ ] **Step 2: Add optional freshness runtime capability**

Use a narrow interface such as:

```go
type certificationFreshnessRuntime164 interface {
    VerifyFreshness(root string, snapshot changeset.Snapshot) error
}
```

`defaultCertificationRuntime153` delegates to `changeset.VerifyFreshness`. Canonical certify uses the fast path when available; test-only legacy runtimes without the interface may retain existing `Compute()` fallback so unrelated tests remain stable.

- [ ] **Step 3: Preserve authority**

After fast verification, downstream analysis/inventory/publish consumes the original decoded canonical Snapshot. No Snapshot fields, hashes, or ReviewOptions ordering change.

- [ ] **Step 4: Verify focused GREEN**

Run canonical freshness tests plus existing Task 2 performance contract tests.

---

### Task 4: Windows Performance and Final Task 3 Verification

**Files:**
- Extend: `.github/workflows/task164-task3-snapshot-freshness.yml`
- Extend tests as needed in analysis/changeset only.

**Interfaces:**
- Produces Task 3 exact-head Windows acceptance evidence.

- [ ] **Step 1: Performance gate**

Use the existing 21/50/100 canonical certify fixtures and Task 2 telemetry. Require all existing certify SLA/process gates to remain GREEN and log fresh `snapshotFreshness` values. Verify fast path produces a material reduction versus Task 2 accepted evidence (422/442/462 ms) without weakening stale detection.

- [ ] **Step 2: Relevant regression**

```powershell
go test -count=1 ./internal/changeset ./internal/analysis ./internal/nav
```

- [ ] **Step 3: Full regression**

```powershell
go test -count=1 ./...
go vet ./...
```

- [ ] **Step 4: Exact HEAD**

Require workflow checkout SHA == branch exact HEAD and report it.

- [ ] **Step 5: Stop for acceptance**

Report RED/GREEN run IDs, mutation matrix, fresh timing data, regression/vet and exact HEAD. Do not enter Final Certification.
