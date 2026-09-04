# Codea Harness 1.6.2 Review Reliability Task 3 — Review Run 中文 README Design

## 1. Goal

为 `.code-harness/runs/` 增加一份随 Harness 一起分发、可直接阅读的中文 `README.md`，解释一次正式 `harness review` Run 的生命周期、权威产物、临时 transport、0 变更行为和跨 Run 边界。

Task 3 只解决“用户/开发者如何正确理解 Review Run 目录”的可读性问题，不引入新的 Review Authority、状态机或业务语义。

## 2. Baseline

- Task 2 Accepted Baseline: `ab2f42f53f472aebf2b18b11a1c9166feee2a20c`
- Task 3 branch must start from that exact commit.
- Task 1 zero-change authority/report chain and Task 2 fresh Review lifecycle are accepted behavior and must not be changed.

## 3. Deliverable

Create and track:

```text
.code-harness/runs/README.md
```

Because `.code-harness/.gitignore` currently ignores `runs/*`, add only the narrow exception:

```text
!runs/README.md
```

Keep `runs/*` ignored so real execution runs remain local artifacts.

## 4. Required Chinese Content

The README must make these points explicit:

1. `.code-harness/runs/` stores execution artifacts only; the README is explanatory documentation, not Review Authority or recovery state.
2. Every new top-level `harness review` starts a fresh invocation: `review begin` creates a new Runtime-owned `runId`, followed by a fresh `analysis snapshot`.
3. Previous runId, Snapshot, ChangeAnalysis, `review.md`, or a prior zero-change conclusion is non-authoritative for a later top-level Review invocation.
4. Within one invocation, formal Review phases share the same runId; “same-run” is intra-invocation only.
5. Explain the formal artifact chain and the main files under `analysis/`, including:
   - `change-set.json`
   - `entrypoint-inventory.json`
   - `change-analysis.json`
   - `change-analysis.cert.json`
   - `review-options.json`
   - `review-scope.json`
   - `review-units.json`
   - `rule-dispatch.json`
   - `certified-findings.json`
   - `certified-findings.cert.json`
6. Explain that `requests/**` is Agent→Runtime transport only, not formal authority; consumed report transport may be deleted by Runtime.
7. Explain that final user-facing formal Review report is `<runId>/review.md`; do not invent/use `review.json` as the formal report.
8. Explain zero-change behavior: `changedFiles=[]` does not shortcut the chain; Review Units, dispatch, empty proposals, Finding Certification, and final `review.md` still complete, ending with PASSED / 0 changed files / 0 findings when coverage is complete.
9. Explain report outcomes at a user-readable level: `PASSED`, `FAILED`, `MANUAL_ACTION_REQUIRED`.
10. Warn users not to manually edit formal authority artifacts to make a Review pass.

## 5. Non-goals / No Scope Creep

Task 3 must not:

- modify Canonical ChangeSet semantics;
- modify Certified ChangeAnalysis Authority;
- modify Finding Certification semantics;
- modify ReviewUnit, Rule Dispatch, Chain, Review Scope, or Selection semantics;
- change `review begin` or fresh lifecycle behavior;
- add another formal report such as `review.json`;
- add task state/recovery behavior under `runs/`;
- change Runtime product code unless a failing documentation contract proves it is strictly necessary (not expected for this task).

## 6. Verification

Task 3 certification must prove:

- the README is tracked despite `runs/*` ignore;
- required Chinese concepts and canonical artifact names are present;
- README does not claim authority or cross-run reuse;
- Task 1 Review Reliability regression remains green;
- Task 2 fresh lifecycle + same-session E2E remains green;
- retained real plain `harness review` E2E remains green;
- final workflow checks out and certifies its exact `${{ github.sha }}`.

Stop after Task 3 fresh exact-head verification and wait for external acceptance. Do not enter Final Certification automatically.
