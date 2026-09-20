# Codea Harness 1.6.4 Task 1 Analysis Certification Performance Hotfix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Entrypoint Inventory certification scale with the Runtime Canonical ChangeSet rather than repository size while preserving certification authority and existing AST matching semantics.

**Architecture:** Controlled Runtime derives an immutable `EntrypointScanPlan` only from the fresh canonical Snapshot plus source existence at the current workspace / `snapshot.MergeBase`. Production entrypoint scanning receives only that plan, validates exact path sets again at the scan boundary, batches all existing type patterns into one ast-grep process and all existing method patterns into one process for controller files, and materializes base sources from one `git cat-file --batch` process into one exact-path temporary tree. General Workspace Navigation keeps its existing behavior; this hotfix adds an Entrypoint-Inventory-specific exact-path path so no unrelated navigation authority changes.

**Tech Stack:** Go, Git plumbing (`git cat-file --batch`), pinned ast-grep 0.42.1, GitHub Actions Windows runners.

**Spec:** `docs/superpowers/specs/2026-09-07-codea-harness-1.6.4-analysis-certification-performance-hotfix-design.md`; amendment authority: `docs/superpowers/specs/2026-09-07-codea-harness-1.6.4-entrypoint-inventory-scope-amendment.md`.

## Global Constraints

- Base is exact `main` HEAD `8a32fdf85a6957961d4631b1d138da316e0860fa`.
- Only Task 1 is implemented; Task 2 is out of scope.
- Current entrypoint scan scope is exactly `snapshot.Files ∩ production Java ∩ current source exists`.
- Base entrypoint scan scope is exactly `snapshot.Files ∩ production Java ∩ source exists at snapshot.MergeBase`.
- `FULL` means complete Canonical ChangeSet review coverage and MUST NOT widen Entrypoint Inventory scan scope.
- Entrypoint Inventory MUST NOT scan `.`, repository/module roots, `src/main/java`, wildcard/glob Java roots, or any full-project fallback.
- Scope widening fails closed with zero certified analysis writes and uses `ENTRYPOINT_SCAN_SCOPE_WIDENED`, `ENTRYPOINT_BASE_SOURCE_OUT_OF_SCOPE`, or `ENTRYPOINT_SCAN_RESULT_OUT_OF_SCOPE` as applicable.
- Base source retrieval uses `snapshot.MergeBase` and one `git cat-file --batch` process; no per-file `git merge-base`, `git show`, per-file temp directory, checkout, worktree, archive, or module copy.
- Entrypoint AST execution is at most four ast-grep processes in the normal current/base path: current type + current method + base type + base method.
- ast-grep remains pinned to 0.42.1 and the existing `allTypePatterns()` / `allMethodPatterns()` matching semantics remain authoritative.
- Runtime Canonical ChangeSet Authority, freshness fail-closed, Entrypoint completeness, evidence validation, coverage validation, Certified ChangeAnalysis, ReviewOptions, USER_SELECTION, and Finding Authority are unchanged.
- The authority chain remains `analysis snapshot → Reviewer proposal → analysis certify → Certified ChangeAnalysis → review options → USER_SELECTION / AUTO_SINGLE / AUTO_FULL`.

---

### Task 1A: Runtime-Owned Snapshot-Bounded Entrypoint Scan Plan

**Files:**
- Create: `.code-harness/tools-runtime/internal/analysis/entrypoint_scan_plan_164.go`
- Modify: `.code-harness/tools-runtime/internal/analysis/entrypoints.go`
- Test: `.code-harness/tools-runtime/internal/analysis/entrypoint_scan_scope_164_test.go`
- Test: `.code-harness/tools-runtime/internal/analysis/certify_test.go`

**Interfaces:**
- Consumes: `changeset.Snapshot{MergeBase, Files, SnapshotSHA256/SHA256}` and production-Java classification from `projectpath.IsMainJava`.
- Produces: `EntrypointScanPlan{RunID, SnapshotSHA256, MergeBase, CurrentPaths, BasePaths, CurrentScopeSHA256, BaseScopeSHA256}` plus exact base blobs, all Runtime-owned.
- Produces: guarded batch scanner input in which callers cannot add paths not present in the plan.

- [ ] **Step 1: Write failing exact-scope tests**

Add tests that construct Snapshot files equivalent to:

```go
[]changeset.File{
    {Path: "src/main/java/acme/A.java", Status: "M"},
    {Path: "src/main/java/acme/B.java", Status: "M"},
    {Path: "src/main/java/acme/C.java", Status: "D"},
    {Path: "src/main/java/acme/D.java", Status: "A"},
    {Path: "src/test/java/acme/TestOnly.java", Status: "M"},
}
```

with current `A/B/D` and merge-base `A/B/C`, then assert exact `CurrentPaths=A/B/D` and `BasePaths=A/B/C`. Add an explicit `Intent{Mode:"FULL"}` regression proving no unrelated repository Java file enters either set.

- [ ] **Step 2: Run Task 1A RED gate**

Run from `.code-harness/tools-runtime`:

```text
go test -count=1 ./internal/analysis -run 'Test164EntrypointScanPlan|Test164EntrypointScope'
```

Expected before implementation: FAIL because the Runtime-owned plan/exact-scope batch path does not exist and production Base scanning still performs per-file Git operations.

- [ ] **Step 3: Implement the plan and exact base-source authority**

Implement:

```go
type EntrypointScanPlan struct {
    RunID              string
    SnapshotSHA256     string
    MergeBase          string
    CurrentPaths       []string
    BasePaths          []string
    CurrentScopeSHA256 string
    BaseScopeSHA256    string
}
```

`buildEntrypointScanPlan164` sorts/dedupes canonical paths, includes only production Java paths in `snapshot.Files`, uses exact current-file existence, and retrieves merge-base blobs only for those Snapshot Java candidates. Base retrieval invokes one `git cat-file --batch` process and parses responses in request order; only returned blobs for requested Snapshot paths may be materialized/scanned.

- [ ] **Step 4: Add scope-widening and result-leak fail-closed tests**

Deliberately request `src/main/java`, `.`, an unrelated Java file, and a fabricated scan result outside the planned set, asserting `ENTRYPOINT_SCAN_SCOPE_WIDENED`, `ENTRYPOINT_BASE_SOURCE_OUT_OF_SCOPE`, and `ENTRYPOINT_SCAN_RESULT_OUT_OF_SCOPE` as appropriate. For canonical certify, inject an Inventory failure and assert `change-analysis.json`, `entrypoint-inventory.json`, and `change-analysis.cert.json` are not created/overwritten.

- [ ] **Step 5: Run Task 1A GREEN gate**

```text
go test -count=1 ./internal/analysis -run 'Test164EntrypointScanPlan|Test164EntrypointScope|Test164CertifyEntrypointScopeWidening'
```

Expected: PASS. Do not proceed to Task 1B until this gate is green.

---

### Task 1B: Two-Phase Exact-Path Batch AST Execution

**Files:**
- Create: `.code-harness/tools-runtime/internal/nav/entrypoint_batch_164.go`
- Modify: `.code-harness/tools-runtime/internal/analysis/entrypoints.go`
- Modify: `.code-harness/tools-runtime/internal/nav/controller_endpoints_153.go`
- Test: `.code-harness/tools-runtime/internal/nav/entrypoint_batch_164_test.go`
- Test: `.code-harness/tools-runtime/internal/analysis/entrypoint_scan_scope_164_test.go`

**Interfaces:**
- Consumes: exact `[]string` Java files supplied by the Runtime-owned plan; no directory/root/glob input.
- Produces: `FindControllerEndpointsExact(ctx, paths)` preserving current controller/mapping annotation filtering and declaration range semantics.
- Produces: at most two ast-grep invocations per snapshot side: all type rules over exact planned files, then all method rules over only AST-confirmed controller files.

- [ ] **Step 1: Write failing process-amplification tests**

Use a recording `nav.Runner` and assert one exact-file set triggers exactly two calls when a controller exists. Assert the first call receives every exact requested file and the second receives only files whose type-phase AST result proves `@Controller`/`@RestController`. Assert no argument equals `.`, `src/main/java`, a module root, repository root, `*`, or `**/*.java`.

- [ ] **Step 2: Run Task 1B RED gate**

```text
go test -count=1 ./internal/nav -run 'Test164EntrypointBatch'
```

Expected before implementation: FAIL because `runRaw()` starts one process per existing pattern rather than one process per phase.

- [ ] **Step 3: Implement multi-rule ast-grep batch execution**

Build inline YAML from the existing pattern slices, one rule document per existing pattern separated by `---`, and invoke pinned ast-grep as one process per phase:

```text
ast-grep scan --inline-rules <rules> --json=stream <exact-file-1> ... <exact-file-N>
```

Parse the same `file/text/range` fields currently consumed from ast-grep JSON. Do not replace/simplify pattern lists. Reject every output file not in the exact target set with `ENTRYPOINT_SCAN_RESULT_OUT_OF_SCOPE`.

- [ ] **Step 4: Batch current and base sides through one scanner call**

Production entrypoint scanning consumes `EntrypointScanPlan`, scans current exact paths once, writes all base blobs into one temporary tree containing only `BasePaths`, and scans that exact base set once. Remove production per-file `git merge-base`, `git show`, and per-file temp-directory behavior from Entrypoint Inventory.

- [ ] **Step 5: Run Task 1B GREEN and semantic regressions**

```text
go test -count=1 ./internal/nav -run 'Test164EntrypointBatch|Test153'
go test -count=1 ./internal/analysis -run 'Test164|Test153Entrypoint'
```

Expected: PASS with unchanged expected entrypoint symbols/dispositions and `astGrepProcessCount <= 4` for current+base normal execution.

---

### Task 1C: Windows Large-Repository Performance / Invariance Gate and Final Verification

**Files:**
- Create: `.code-harness/tools-runtime/internal/analysis/entrypoint_performance_164_test.go`
- Create: `.github/workflows/task164-task1-analysis-cert-performance.yml`

**Interfaces:**
- Consumes: the same production Entrypoint Inventory/certification path and pinned `.code-harness/bin/ast-grep.exe`.
- Produces: machine-visible evidence for requested/scanned sets, ast-grep counts, base Git batch count, absence of per-file merge-base/show, and elapsed time.

- [ ] **Step 1: Add a real Windows P0 fixture test**

Create a temporary Git repository with at least 5000 production Java files and a canonical 4–5-file ChangeSet containing one modified Controller, one modified Service, and one deleted/renamed Java case. Execute the production inventory/certification path with real pinned ast-grep and fail if elapsed time exceeds 15 seconds while logging the target <=10 second measurement. Assert exact current/base scanned files, `astGrepProcessCount <= 4`, per-file merge-base/show counts `==0`, full-project Entrypoint Base scan `==0`, and one base Git batch process.

- [ ] **Step 2: Add repository-size invariance test**

Run the same five-file ChangeSet against 100 and 5000 unrelated Java files and assert identical `currentRequestedFiles`, `baseRequestedFiles`, `currentScannedFiles`, `baseScannedFiles`, `astGrepProcessCount`, and `baseGitBatchProcessCount`.

- [ ] **Step 3: Add dedicated Windows workflow**

Create `task164-task1-analysis-cert-performance.yml` on `windows-latest`, vendor ast-grep 0.42.1 using the already-pinned Windows x64 SHA-256, run Task 1A/1B focused tests, the 5000+ P0/invariance tests, full relevant `internal/analysis` and `internal/nav` regression, `go test -count=1 ./...`, `go vet ./...`, and exact-HEAD assertion.

- [ ] **Step 4: Run fresh exact-head verification**

```text
go test -count=1 ./internal/analysis ./internal/nav
go test -count=1 ./...
go vet ./...
```

Run the dedicated Windows workflow at the final exact branch HEAD and record run URL/ID, elapsed P0 evidence, exact file sets, `astGrepProcessCount`, `baseGitBatchProcessCount`, and exact HEAD.

- [ ] **Step 5: Stop at Task 1**

Do not modify or begin Task 2. Return exact HEAD plus Task 1A RED/GREEN, Task 1B RED/GREEN, process counts, 5000+ Windows performance, full regression, `go test`, `go vet`, and fresh exact-head Windows CI evidence for acceptance.
