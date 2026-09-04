# Codea Harness 1.6.2 Review Reliability Task 3 Run README Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a tracked Chinese `.code-harness/runs/README.md` that accurately explains the accepted Review Run lifecycle and artifacts without creating any new Review Authority.

**Architecture:** Treat the README as static explanatory documentation shipped with Harness. Keep actual `runs/*` execution directories ignored, add only a narrow Git ignore exception for `runs/README.md`, and enforce the documentation contract with a Windows PowerShell regression. Retain all accepted Review Runtime semantics unchanged.

**Tech Stack:** Markdown, Git ignore rules, PowerShell 7, GitHub Actions Windows runner.

**Spec:** `docs/superpowers/specs/2026-09-04-codea-harness-1.6.2-review-reliability-task3-run-readme-design.md`

## Global Constraints

- Accepted Task 2 baseline is `ab2f42f53f472aebf2b18b11a1c9166feee2a20c`.
- Do not modify Runtime Review product semantics.
- Do not modify Canonical ChangeSet, Certified ChangeAnalysis, Finding Certification, ReviewUnit, Rule Dispatch, Chain, Review Scope, or Selection semantics.
- Do not modify Task 2 fresh Review lifecycle semantics.
- `.code-harness/runs/README.md` is explanatory only; it must not become Authority or recovery state.
- Task 3 only; after fresh exact-head verification, stop for external acceptance.

---

### Task 1: RED — Review Run README contract

**Files:**
- Create: `.github/scripts/task162-review-reliability-task3-run-readme-regression.ps1`
- Create: `.github/workflows/task162-review-reliability-task3.yml`

**Interfaces:**
- Consumes: repository files at the checked-out exact HEAD.
- Produces: a deterministic documentation gate for the Review Run README contract.

- [ ] **Step 1: Write failing README regression**

The PowerShell regression must fail when `.code-harness/runs/README.md` is absent and, once present, assert all of the following:

```text
README is UTF-8 readable and contains Chinese text
.code-harness/.gitignore contains !runs/README.md
runs/* remains ignored
README names review begin and analysis snapshot
README states every new top-level harness review uses a fresh runId
README states previous-run artifacts/conclusions are non-authoritative
README explains same-run is intra-invocation
README names the canonical analysis artifact set
README describes requests/** as transport-only
README names review.md as final formal Review report
README rejects review.json as formal report
README describes zero-change full chain and PASSED / 0 / 0 outcome
README mentions PASSED / FAILED / MANUAL_ACTION_REQUIRED
README states it is documentation, not authority/recovery state
```

- [ ] **Step 2: Create RED workflow**

The Task 3 workflow must run the README regression before any retained regression. On the RED commit it should fail specifically because `.code-harness/runs/README.md` does not exist.

- [ ] **Step 3: Run RED CI and record expected failure**

Expected: Task 3 README contract FAIL; no production/runtime change has been made.

---

### Task 2: GREEN — Track and write the Chinese Run README

**Files:**
- Modify: `.code-harness/.gitignore`
- Create: `.code-harness/runs/README.md`

**Interfaces:**
- Produces: tracked static documentation while preserving ignore behavior for all real run directories/artifacts.

- [ ] **Step 1: Add narrow Git ignore exception**

Keep:

```text
runs/*
!runs/.gitkeep
```

and add exactly:

```text
!runs/README.md
```

Do not unignore execution run directories.

- [ ] **Step 2: Add Chinese README**

Document the accepted flow:

```text
new top-level harness review
  -> review begin
  -> fresh runId
  -> analysis snapshot
  -> inventory / semantic proposal / certify
  -> review options / scope / units / dispatch
  -> finding proposals / certify-findings
  -> report review
  -> review.md
```

List the formal artifacts and clearly separate `requests/**` transport from authority artifacts.

- [ ] **Step 3: Document zero-change and cross-run rules**

State that zero changes still execute the complete formal chain and that old runs never authorize a new top-level Review.

- [ ] **Step 4: Run focused README contract**

Expected: PASS.

---

### Task 3: Retained regression and exact-head certification

**Files:**
- Modify only as needed: `.github/workflows/task162-review-reliability-task3.yml`

**Interfaces:**
- Consumes: existing accepted Task 1/Task 2/old plain-review regression scripts.
- Produces: fresh exact-head Windows certification for Task 3.

- [ ] **Step 1: Run Task 3 README contract**
- [ ] **Step 2: Run focused/full Go regression and `go vet ./...`**
- [ ] **Step 3: Build exact-head Windows Runtime**
- [ ] **Step 4: Run Task 1 retained real Review Reliability E2E**
- [ ] **Step 5: Run Task 2 fresh lifecycle contract and same-session E2E**
- [ ] **Step 6: Run retained old Task 3 real plain `harness review` E2E**
- [ ] **Step 7: Verify checkout HEAD equals `${{ github.sha }}`**
- [ ] **Step 8: Report branch, exact HEAD, run/job and evidence; stop awaiting human acceptance**
