# Codea Harness 1.6.2 Post-hotfix Final Release Certification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Certify the final 1.6.2 release candidate from the accepted Task 3 baseline and prove that all accepted Hotfix authority/invocation/plain-review behavior plus retained 1.6.x release/package regressions remain green on one exact HEAD.

**Architecture:** Keep Task 1–3 product semantics frozen. Extend release-certification infrastructure only: bind the release to the accepted Task 1/2/3 commits, run the accepted Hotfix gates in addition to the existing retained release gates, run the real plain `harness review` E2E using the pinned OpenCode host, and emit the official packages/checklist/whitelist from the same exact release HEAD.

**Tech Stack:** PowerShell, Go 1.23.x CI toolchain, Windows GitHub Actions, OpenCode 1.18.25, Python 3.12, ast-grep 0.42.1.

**Spec:** `docs/superpowers/specs/2026-09-02-codea-harness-1.6.2-task3-real-plain-review-e2e-design.md`

## Global Constraints

- Accepted Task 1 HEAD: `119c87057718f3d1f6f0286622d32b350f21d64e`.
- Accepted Task 2 HEAD: `2503678e347dc0ba2bc2f0357cefd9306d199480`.
- Accepted Task 3 HEAD: `4a312c4a2c85a202b740d3a1f419b2812e42f866`.
- Do not change Review, Finding, Chain, Workspace, Maven, Canonical ChangeSet, Runtime request, or Rule Pack business semantics.
- Final certification changes are limited to release/certification test infrastructure and documentation.
- Official artifacts and release checklist must be generated from one fresh exact HEAD.

---

### Task 1: Final-certification contract RED gate

**Files:**
- Create: `.github/scripts/task162-hotfix-final-certification-contract-regression.ps1`
- Modify: `.github/workflows/task162-final-release-certification.yml`

- [ ] Add a contract regression that requires the accepted Task 1/2/3 SHAs, the three Hotfix gate invocations, Hotfix PASS fields in the release checklist, and pinned OpenCode/Python setup in the workflow.
- [ ] Run it on the current pre-hotfix-aware release driver and verify it fails because the required final-certification contract is missing.

### Task 2: Minimal final-certification implementation

**Files:**
- Modify: `.github/scripts/task162-release-certification.ps1`
- Modify: `.github/workflows/task162-final-release-certification.yml`

- [ ] Bind certification to the accepted Task 1/2/3 commits using Git ancestry checks.
- [ ] Add a post-Task3 scope guard so certification-only commits cannot silently change product/runtime semantics after Task 3 acceptance.
- [ ] Run Task 1 canonical authority gates, Task 2 Agent→Runtime invocation gates, and Task 3 real plain `harness review` E2E in the final certification run.
- [ ] Preserve all existing package/no-Go/upgrade/retained 1.6/1.6.1/1.6.2 gates.
- [ ] Extend the release checklist with accepted Hotfix baselines and PASS fields for Task 1, Task 2, Task 3, post-Task3 scope, and exact-head certification.
- [ ] Install pinned OpenCode 1.18.25 and Python 3.12 before the release driver executes.

### Task 3: Fresh exact-head release evidence

**Files:**
- No product files.

- [ ] Run the contract regression on the final committed source and verify PASS.
- [ ] Run the full final release certification on the final release HEAD.
- [ ] Verify Full Go regression, Go vet, Runtime build, Hotfix Task 1/2/3 gates, retained regressions, package cleanup, install/upgrade artifacts, whitelist, checklist, and exact-head marker all PASS.
- [ ] Verify the final release branch differs from accepted Task 3 only by the approved certification infrastructure/docs files.
- [ ] Hand the exact HEAD, workflow run/job IDs, and official artifact/checklist evidence to the external reviewer; do not self-declare user acceptance.
