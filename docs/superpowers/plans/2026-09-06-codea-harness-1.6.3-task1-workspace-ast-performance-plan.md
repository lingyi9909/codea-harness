# Codea Harness 1.6.3 Task 1 Workspace AST Performance Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove Workspace AST full-tree scan amplification and the default `bufio.Scanner` ~64KB JSON-stream record ceiling without changing Runtime/AST semantic authority.

**Architecture:** Build a deterministic Java-file inventory and use conservative text/file-name prefiltering only to choose candidate files. All Class/Method/inheritance/nested ownership/template-dispatch facts remain produced by ast-grep and Runtime range/ownership checks. Once an owner/subclass is AST-confirmed, downstream AST queries are restricted to those confirmed files. JSON stream parsing keeps line-delimited ast-grep semantics while explicitly supporting records larger than 64KB.

**Tech Stack:** Go 1.23.x, ast-grep 0.42.1, GitHub Actions Windows runner, PowerShell.

**Spec:** `docs/superpowers/specs/2026-09-06-codea-harness-1.6.3-runtime-usability-hotfix-design.md`

## Global Constraints

- Base exactly `1a9a1523c74465ca76b1035b032e2b5bc40012cd`.
- Work only on `Task 1 — Workspace AST Performance Hardening`; do not enter Task 2.
- Do not base work on or cherry-pick from `hotfix/1.6.3-runtime-usability`.
- Candidate narrowing may only reduce AST scan scope; it is never semantic authority.
- Class identity, Method identity, inheritance, nested ownership and template dispatch remain AST/Runtime-owned facts.
- Do not solve the performance issue by timeout increase.
- Required flow: RED → Implementation → Focused GREEN → `internal/nav` Regression GREEN → Fresh exact-head CI.

---

### Task 1A: RED coverage for scan narrowing and large JSON records

**Files:**
- Create: `.code-harness/tools-runtime/internal/nav/workspace_ast_performance_163_test.go`
- Create: `.github/workflows/task163-task1-workspace-ast-performance.yml`

**Interfaces:**
- Consumes existing `Navigator`, `Runner`, `WorkspaceSuperclass`, `WorkspaceMethod`, and `runWorkspaceRaw` behavior.
- Produces regression contracts proving candidate-aware AST targets and >64KB JSON record parsing.

- [ ] **Step 1: Add a recording fake Runner test for large repositories.**

Create a temp `src/main/java` tree with one target class and many unrelated Java files. Invoke `WorkspaceSuperclass("TargetService")`. The Runner returns a valid AST record for the target pattern. Assert every ast-grep invocation targets candidate Java files and does not target the whole `src/main/java` directory.

- [ ] **Step 2: Add a >64KB JSON-stream record test.**

Feed `runWorkspaceRaw` one line-delimited `workspaceSGLine` JSON record whose `text` is >64KB. Assert exactly one full match is returned and no scanner/token error occurs.

- [ ] **Step 3: Add an Authority guard.**

Create a text candidate containing the requested class name but have ast-grep return no match. Assert `WorkspaceSuperclass` remains `ErrSymbolNotFound`; candidate text alone must never create a Runtime fact.

- [ ] **Step 4: Add a real ast-grep Large Class test.**

Under `CODEA_AST_GREP_TEST_PATH`, generate a 1500–3000-line `BigService` and verify `WorkspaceMethod("BigService", "target")` succeeds. Existing nested/inheritance/template tests remain part of the focused gate.

- [ ] **Step 5: Add a Windows Task 1 workflow.**

The workflow must install Go 1.23.x and pinned ast-grep 0.42.1, run the Task 1 focused tests with real ast-grep, then run `go test -count=1 ./internal/nav`.

- [ ] **Step 6: Commit and execute RED.**

Run the Task 1 workflow at the tests-only exact HEAD. Expected RED: scan-target test fails because current code passes the whole source directory; large-record test fails with `bufio.Scanner: token too long`.

### Task 1B: Candidate-file narrowing while preserving AST authority

**Files:**
- Modify: `.code-harness/tools-runtime/internal/nav/workspace_ast.go`
- Test: `.code-harness/tools-runtime/internal/nav/workspace_ast_performance_163_test.go`

**Interfaces:**
- Add internal helpers to inventory Java files, conservatively select candidate files from literal hints, normalize verified candidate paths, and execute ast-grep against candidate files.
- `runWorkspaceRaw` remains the raw AST parser; callers may route through candidate-aware/scoped helpers.

- [ ] **Step 1: Build deterministic Java inventory and candidate narrowing.**

Walk only `<RepoRoot>/src/main/java`, include regular `.java` files, sort paths, and prefilter by safe literal identifier presence/file-name match. Any prefilter miss produces no semantic fact; AST must still match before a result exists.

- [ ] **Step 2: Scope exact class/superclass lookup.**

For `WorkspaceSuperclass` and owner lookup in `WorkspaceMethod`, first narrow by the requested class name, then execute the existing ast-grep patterns only over candidates.

- [ ] **Step 3: Reuse AST-confirmed paths for downstream queries.**

After owner/subclass AST confirmation, execute method, method-call, nested-class inventory and ownership checks only on the AST-confirmed file set. `workspaceAllClassTypes` must no longer rescan the entire Workspace for an already-known owner/subclass file.

- [ ] **Step 4: Preserve fail-closed behavior.**

Empty candidate set or AST mismatch returns the same not-found/partial semantics as before. Ambiguous AST matches remain ambiguous; prefiltering never chooses a winner.

### Task 1C: Remove the Scanner 64KB record ceiling and certify

**Files:**
- Modify: `.code-harness/tools-runtime/internal/nav/workspace_ast.go`
- Test: `.code-harness/tools-runtime/internal/nav/workspace_ast_performance_163_test.go`
- Verify: `.github/workflows/task163-task1-workspace-ast-performance.yml`

**Interfaces:**
- Preserve ast-grep line-delimited JSON semantics.
- Support records materially larger than 64KB with an explicit buffer policy or reader implementation.

- [ ] **Step 1: Replace default Scanner limit.**

Configure the Workspace AST stream parser with an explicit initial buffer and a multi-megabyte maximum large enough for generated 1500–3000-line class records, or use an equivalent reader without Scanner's default token ceiling. Parsing errors must still fail rather than silently certify facts.

- [ ] **Step 2: Run focused GREEN.**

Run:

```text
go test -count=1 ./internal/nav -run 'Test163WorkspaceAST|TestWorkspace|Test152Nested'
```

with `CODEA_AST_GREP_TEST_PATH` set for real-AST cases. Expected: PASS.

- [ ] **Step 3: Run full nav regression.**

Run:

```text
go test -count=1 ./internal/nav
```

Expected: PASS.

- [ ] **Step 4: Run fresh exact-head Windows CI.**

Dispatch `task163-task1-workspace-ast-performance` on the final Task 1 HEAD. Require workflow conclusion `success` and verify workflow `head_sha` equals the branch exact HEAD.

- [ ] **Step 5: Scope review.**

Compare final HEAD to base and confirm changed files are limited to Task 1 implementation/tests/workflow/plan. Record known limitations and stop before Task 2.
