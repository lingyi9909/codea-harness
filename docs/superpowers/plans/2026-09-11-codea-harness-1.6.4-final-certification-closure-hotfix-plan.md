# Codea Harness 1.6.4 Final Certification Closure Hotfix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Repair only the OpenCode Host Adapter, Agent Contract, Product E2E, install UX, and final certification gaps that invalidate the 6605916 1.6.4 release candidate.

**Architecture:** Keep the accepted Runtime authority/state-machine architecture unchanged. Generate OpenCode 1.18.25-compatible Reviewer host resources, prove them through OpenCode's own resolved configuration/debug interfaces, make root-session product E2Es obey the Harness Agent contract, and add a deterministic first-install preflight/copy layer for `.opencode` host collisions. Final certification is rebuilt around a new exact HEAD and cannot inherit 6605916 evidence.

**Tech Stack:** PowerShell 7, Python 3.12 deterministic OpenAI-compatible test model, OpenCode `opencode-ai@1.18.25`, Go Runtime tests, GitHub Actions Windows.

**Spec:** User-approved Codea Harness 1.6.4 Final Certification Closure Hotfix requirements from 2026-09-11.

## Global Constraints

- Work only on `release/1.6.4-final-certification`; never merge/publish to `main` in this task.
- Base `6605916b4929434ea3362ab5b4fc6325ca117a2f` is hotfix input only, not a releasable RC.
- Never restore revoked RC `6aa5d9dad0623cd60a845360c9b20ab153921e87`.
- Do not redesign accepted Runtime ChangeSet/certification/review-planning/finding/report/progress/performance semantics.
- All OpenCode product gates pin `opencode-ai@1.18.25`.
- New final certification evidence must be generated from one new exact HEAD.

---

### Task 1: OpenCode 1.18.25 resolved Reviewer host contract

**Files:**
- Modify: `.github/scripts/task164-release-package.ps1`
- Create: `.github/scripts/task164-closure-opencode-resolved-contract.ps1`
- Modify: `.github/scripts/task164-final-certification-contract.py`

**Interfaces:**
- Consumes: install package and real `opencode` 1.18.25 CLI.
- Produces: resolved-agent/command evidence and required PASS markers.

- [ ] Write RED gate that extracts the install package and executes `opencode debug agent reviewer` plus `opencode debug config`; assert `permission`/tool resolution, `agent=reviewer`, `subtask=true`, denied `bash/task/edit`, allowed `codea-reviewer-submit`.
- [ ] Verify RED on current package (`permissions`, `shell`, `subagent` must fail).
- [ ] Generate 1.18.25 frontmatter using singular `permission`, real keys, deny-by-default/minimal read/search plus custom submit tool; generate command with `subtask: true`.
- [ ] Verify GREEN through actual resolved OpenCode output; no grep-only acceptance.

### Task 2: Agent contract consistency

**Files:**
- Modify: `.code-harness/AGENTS.md`
- Modify: `.code-harness/bootstrap.md`
- Modify: `.code-harness/agents/orchestrator.md`
- Modify: `.code-harness/contracts/reviewer-host-contract.md`
- Create: `.github/scripts/task164-closure-agent-contract.ps1`

**Interfaces:**
- Produces one canonical review authority flow and a machine gate for the two official progress/failure Runtime commands.

- [ ] Add RED consistency checks for missing `review progress` / `review reviewer-unavailable` and ambiguous `harness review | Reviewer` ownership.
- [ ] Update all four documents so Main Agent/Orchestrator owns routing/Runtime/delegation/progress/fail-closed and Reviewer owns only CHANGE_ANALYSIS + FINDINGS semantic proposal phases.
- [ ] Verify `HARNESS_164_AGENT_CONTRACT_CONSISTENT PASS`.

### Task 3: Contract-compliant positive OpenCode product E2E and Runtime progress rendering

**Files:**
- Modify: `.github/scripts/task164-task4-plain-review-server.py`
- Modify: `.github/scripts/task164-task4-packaged-plain-review-e2e.ps1`

**Interfaces:**
- Root model may use OpenCode `write` only for same-run `requests/**`, `task` only to invoke Reviewer, and `bash` only for exact allowlisted Runtime executable commands with no shell evaluation/pipeline/redirection.
- Transcript must carry Runtime `events[].display` progress from 1/8 through 8/8.

- [ ] RED: reject legacy `TASK4_STAGE_xx` as progress authority and reject illegal shell constructs in model-issued bash calls.
- [ ] Refactor deterministic model to parse tool results itself, create request JSON via `write`, invoke only exact Runtime commands via `bash`, delegate semantic phases via `task`/command, and call `review progress` after each authority boundary.
- [ ] Assert literal `harness review` root session transcript contains Runtime-derived `[1/8] ... [8/8] REPORT PASS` in order.
- [ ] Emit `OPENCODE_RUNTIME_PROGRESS_RENDERED PASS`, `OPENCODE_RUNTIME_PROGRESS_1_TO_8 PASS`, `PROMPT_ONLY_PROGRESS_NOT_AUTHORITY PASS`.

### Task 4: Product-level Reviewer interruption E2E

**Files:**
- Modify: `.github/scripts/task164-task4-progress-interruption-e2e.ps1`
- Extend deterministic model from Task 3 with controlled Reviewer-unavailable mode.

**Interfaces:**
- Literal `harness review` through real OpenCode must fail during CHANGE_ANALYSIS and surface Runtime failure events.

- [ ] RED: require real root session/transcript instead of direct Runtime-only mutation.
- [ ] Drive `review begin -> snapshot -> Reviewer failure -> review reviewer-unavailable -> review progress` through OpenCode.
- [ ] Assert visible CHANGE_ANALYSIS FAILED, later stages BLOCKED, `REVIEWER_UNAVAILABLE / MANUAL_ACTION_REQUIRED / HARD STOP`, and zero downstream certification/planning/finding/report/fallback artifacts.
- [ ] Emit interruption PASS markers.

### Task 5: Safe first-install UX and host collision handling

**Files:**
- Modify: `.github/scripts/task164-release-package.ps1`
- Create: `.github/scripts/task164-install.ps1` (packaged as root `install.ps1`)
- Modify: `README.md`
- Create: `.github/scripts/task164-closure-install-e2e.ps1`

**Interfaces:**
- Installer consumes extracted release package + target project root.
- It preflights all three `.opencode` host destinations before any write; non-identical existing bytes produce `MANUAL_ACTION_REQUIRED` and zero destructive writes.

- [ ] RED: package must expose complete host resources and deterministic installer; conflict fixture must remain byte-for-byte unchanged.
- [ ] Add package-root installer with full preflight then copy; exact-byte existing host files are idempotent, conflicting bytes hard-stop before framework/host mutation.
- [ ] Rewrite README 1.6.4 install instructions: use Release package, run full package-root install, never copy only `.code-harness`, never use Source ZIP, OpenCode 1.18.25 supported/pinned contract, collision behavior.
- [ ] Emit install PASS markers.

### Task 6: Fresh exact-head Final Certification closure

**Files:**
- Modify: `.github/scripts/task164-final-certification.ps1`
- Modify: `.github/scripts/task164-final-certification-contract.py`
- Modify: `.github/workflows/task164-final-certification.yml`

**Interfaces:**
- Produces fresh 1.6.4 install/upgrade packages, checklist, evidence artifact, release-candidate artifact for one exact HEAD.

- [ ] Add new closure gates/markers and exact allowed-file scope; explicitly reject inheriting 6605916 certification evidence.
- [ ] Fresh exact packaged 1.6.3 -> candidate 1.6.4 upgrade.
- [ ] Fresh positive literal OpenCode review, unavailable/interruption, real progress, resolved config, install conflict gates.
- [ ] Fresh retained performance/authority, `go test -count=1 ./...`, `go vet ./...`, package SHA256, exact-head guard.
- [ ] Inspect final checklist/artifacts and confirm all gates PASS on the same new HEAD.
- [ ] Stop for user acceptance; do not publish or merge main.
