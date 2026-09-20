# Codea Harness 1.6.4 Task 2 Analysis Certify Performance Evidence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Runtime-owned, non-authoritative `analysis/certify-performance.json` evidence for canonical `analysis certify`, with real stage/process telemetry and dedicated Windows 21/50/100-file performance gates.

**Architecture:** Keep all existing certification authority and output hashes unchanged. Add a run-local recorder around the canonical certify stages, and obtain Entrypoint timing/process facts from instrumentation at the actual `ast-grep` and `git cat-file --batch` launch sites introduced by Task 1. Write telemetry only after/alongside the authoritative path as diagnostic evidence; telemetry write failures never turn a formal failure into success or replace the formal error, and telemetry content never participates in ChangeAnalysis/certificate hashing or correctness decisions.

**Tech Stack:** Go 1.23.x CI toolchain, Git, pinned ast-grep 0.42.1, GitHub Actions `windows-latest`.

**Spec:** `docs/superpowers/specs/2026-09-07-codea-harness-1.6.4-analysis-certification-performance-hotfix-design.md` sections 10–11.

## Global Constraints

- Accepted Task 1 baseline: `f401716f9878e1d5af463f77322f4b15420a47f3`.
- Do not change Runtime Canonical ChangeSet Authority, snapshot freshness fail-closed semantics, Entrypoint completeness, Evidence, Coverage, ReviewOptions, USER_SELECTION, Finding Authority, or certificate semantics.
- Keep pinned ast-grep exactly `0.42.1`.
- Performance artifact is Runtime-owned diagnostic evidence only; it is not semantic authority and must not enter any certificate/hash input.
- Agent/Reviewer cannot supply a performance artifact path or alter correctness based on telemetry.
- Failure telemetry must preserve the original formal error code and must not contain source bytes, secrets/config values, or absolute user paths.
- Process counters are recorded only at actual process launch sites; never estimate from file/pattern counts.
- Do not enter Task 3. Task 3 eligibility is decided only from fresh Task 2 telemetry.

---

### Task 2A: Performance Evidence Contract and Failure Semantics

**Files:**
- Create: `.code-harness/tools-runtime/internal/analysis/certify_performance_164.go`
- Create: `.code-harness/tools-runtime/internal/analysis/certify_performance_164_test.go`
- Modify: `.code-harness/tools-runtime/internal/analysis/certify_canonical_162.go`

**Interfaces:**
- Produces `certifyPerformance164` JSON schema-version-1 model with fixed path `.code-harness/runs/<runId>/analysis/certify-performance.json`.
- Produces internal recorder helpers for stage timing, success/failure completion, error-code extraction, and best-effort atomic write.
- Consumes existing canonical certify flow without changing `CertifyRequest` or `Certificate`.

- [ ] **Step 1: Write failing contract tests**

Add tests that require a successful canonical certify to write a schema-version-1 artifact containing only run/snapshot identity, status, counts, timings, and process counters. Assert the artifact path is derived only from `runId`, `CertifyRequest` has no telemetry path, and changing/deleting telemetry cannot change `AnalysisSHA256`, `EntrypointInventorySHA256`, `ChangeSetSHA256`, or `SnapshotSHA256`.

- [ ] **Step 2: Write failing failure-telemetry tests**

Use a valid canonical request with an injected runtime failure and require `status=FAILED`, the original machine error code in `errorCode`, no certified analysis writes when the formal flow failed before publish, and no source content/absolute path fields in the telemetry JSON.

- [ ] **Step 3: Run RED gate**

Run:

```bash
go test -count=1 -v ./internal/analysis -run 'Test164CertifyPerformance'
```

Expected: FAIL because `certify-performance.json` and recorder types do not yet exist.

- [ ] **Step 4: Implement the minimal recorder and canonical integration**

Create an internal model equivalent to:

```go
type certifyPerformance164 struct {
    SchemaVersion  int                         `json:"schemaVersion"`
    RunID          string                      `json:"runId"`
    SnapshotSHA256 string                      `json:"snapshotSha256"`
    Status         string                      `json:"status"`
    ErrorCode      string                      `json:"errorCode,omitempty"`
    Counts         certifyPerformanceCounts164 `json:"counts"`
    TimingMS       certifyPerformanceTiming164 `json:"timingMs"`
    Processes      certifyPerformanceProcess164 `json:"processes"`
}
```

Instrument canonical stages with monotonic elapsed durations. Use a deferred best-effort writer after the request has a valid run identity. The deferred telemetry write error is intentionally ignored for the formal return path. Never add telemetry fields to `Certificate` or canonical ChangeAnalysis.

- [ ] **Step 5: Run GREEN gate and commit**

Run the same targeted test command and commit only Task 2A files when GREEN.

---

### Task 2B: Real Entrypoint Phase Timing and Process Counters

**Files:**
- Modify: `.code-harness/tools-runtime/internal/analysis/entrypoint_batch_inventory_164.go`
- Modify: `.code-harness/tools-runtime/internal/analysis/entrypoint_bounded_scanner_164.go`
- Modify: `.code-harness/tools-runtime/internal/analysis/certify.go`
- Modify: `.code-harness/tools-runtime/internal/analysis/certify_canonical_162.go`
- Test: `.code-harness/tools-runtime/internal/analysis/certify_performance_164_test.go`

**Interfaces:**
- Extend internal `entrypointExecutionMetrics164` with current/base type/method process counts and durations, Base source-load duration, and controller-file counts.
- Add optional internal `InventoryWithPerformance(...) (EntrypointInventory, entrypointExecutionMetrics164, error)` capability to the default certification runtime; do not widen the existing required `certificationRuntime153` interface used by legacy/fake tests.

- [ ] **Step 1: Write failing actual-launch counter tests**

Require one Controller on both sides to report exactly one Current Type, Current Method, Base Type, Base Method AST process and one Base `cat-file` process. Require a no-Controller side to report zero Method processes. Assertions must use actual runner/process instrumentation, not formulas.

- [ ] **Step 2: Run RED gate**

Run:

```bash
go test -count=1 -v ./internal/analysis -run 'Test164CertifyPerformance|Test164EntrypointPerformance'
```

Expected: FAIL because phase-specific process/timing fields are not available.

- [ ] **Step 3: Instrument actual process launch sites**

At `countingEntrypointRunner164.Run`, classify only the concrete Task 1 inline rule id (`codea-entrypoint-types-164` vs `codea-entrypoint-methods-164`) and the scanner side supplied by the caller; increment the corresponding counter immediately before launching the actual process and add elapsed duration after return. At `gitBatchBaseSourceReader164.ReadBaseSources`, time and count the single real `git cat-file --batch` invocation only after all protocol/path validation has passed.

- [ ] **Step 4: Feed metrics into canonical certify telemetry**

Production `defaultCertificationRuntime153` exposes the optional performance-aware inventory method using `buildEntrypointInventoryWithMetrics164`. Canonical certify type-asserts this optional capability; non-production/fake runtimes continue through the existing `Inventory` method without interface breakage. Copy only aggregate counts/durations into `certify-performance.json`.

- [ ] **Step 5: Run GREEN gate and commit**

Run targeted analysis tests and Task 1 regression tests. Verify Task 1 process-count/scope behavior remains unchanged.

---

### Task 2C: Dedicated Windows 21/50/100-file Performance Gate

**Files:**
- Create: `.code-harness/tools-runtime/internal/analysis/certify_performance_windows_164_test.go`
- Create: `.github/workflows/task164-task2-analysis-cert-performance-evidence.yml`

**Interfaces:**
- Produces three real Git/Java fixtures and invokes actual canonical `Certify` with pinned ast-grep 0.42.1.
- Reads the emitted performance artifact and asserts wall-clock, stage telemetry, and actual process counters.

- [ ] **Step 1: Build deterministic fixture/proposal helpers**

Fixture A: 21 changed files, 15–20 production Java, 3–5 Controllers, Current+Base Modified coverage.

Fixture B: 50 changed production Java, 8–10 Controllers, Current+Base, multi-module.

Fixture C: 100 changed production Java, 10–20 Controllers.

Generate valid Reviewer proposal evidence from the precomputed expected Entrypoint inventory outside the timed certify interval so the timed region is only Runtime `analysis certify`.

- [ ] **Step 2: Add hard wall-clock/process gates**

Assert:

```text
21 files: target <=5s, hard <=15s
50 files: target <=10s, hard <=20s
100 files: target <=20s, hard <=30s
AST processes <=4 for every fixture
Base cat-file processes <=1
per-file merge-base ==0
per-file git show ==0
```

Also assert the telemetry `total` is non-zero and that required timing keys (`snapshotFreshness`, `entrypointInventory`, Current/Base Type/Method AST, `baseSourceLoad`, `evidenceValidation`, `coverageValidation`) are present/non-negative.

- [ ] **Step 3: Add dedicated Windows workflow**

Use `windows-latest`, pin ast-grep 0.42.1 with the accepted checksum, run telemetry contract tests plus A/B/C performance gates, then run full relevant analysis/nav regression, full `go test -count=1 ./...`, `go vet ./...`, and exact-HEAD checks.

- [ ] **Step 4: Run fresh Windows workflow and commit only necessary gate changes**

No timeout increase is permitted as a fix.

---

### Task 2D: Final Task 2 Verification and Task 3 Eligibility Evidence

**Files:**
- No production changes unless verification finds a Task 2 regression.

- [ ] **Step 1: Re-read spec sections 10–13 and compare every requirement to tests/artifact fields**

Confirm telemetry is diagnostic only, failures remain fail-closed, and no forbidden scheme was introduced.

- [ ] **Step 2: Fresh full verification on the final exact HEAD**

Run through Windows CI:

```bash
go test -count=1 ./...
go vet ./...
```

Require all dedicated Task 2 Windows jobs GREEN on one exact SHA.

- [ ] **Step 3: Report Task 3 eligibility without entering Task 3**

From Fixture A/B telemetry report `snapshotFreshness` milliseconds and percentage of total. Apply the design threshold only as evidence:

```text
snapshotFreshness >= 40% of total
or snapshotFreshness > 2s sustained
```

State `Task 3 candidate` or `Task 3 not required by current evidence`; do not implement Task 3 until separate user approval.
