# Codea Harness 1.6.2 Hotfix — Task 3 Implementation Plan

## Scope

Implement only the real plain `harness review` E2E and its CI acceptance gate on top of Task 2 Accepted HEAD `2503678e347dc0ba2bc2f0357cefd9306d199480`.

## Task 1 — RED acceptance

Create a Task 3 Windows E2E script/workflow that requires a real OpenCode run, active-contract reads, real Runtime artifacts, and final `review.md`. Record a failing RED run before the model-host fixture exists.

## Task 2 — Deterministic Agent Host fixture

Add test-only assets under `.github/scripts/task162-hotfix-task3/`:

- local OpenAI-compatible model server;
- minimal OpenCode project-agent bootstrap with no Runtime request fields;
- fixture preparation and post-run black-box verifier.

The model server must issue one tool action at a time and may not bypass OpenCode tool execution.

## Task 3 — Real plain review path

Prepare an initialized Maven-shaped Git fixture, commit framework/project state, inject fresh Runtime + pinned ast-grep, create one benign `application.yml` working-tree change, then execute:

`opencode run --auto --agent codea-harness-e2e --model codea-task3/task3 "harness review"`

Assert the full Runtime-owned authority chain and final `review.md`.

## Task 4 — Contract-drift assertions

Validate produced Agent requests and transcript:

- snapshot: no `requestedBaseRef`;
- certify: canonical request shape;
- review options: no `baseRef`;
- active request schemas read before use;
- no schema-invalid/retry/manual-repair escape path;
- no legacy executable invocation.

## Task 5 — Retained regressions + exact head

Run Task 2 focused/audit gates, Task 1 canonical gates, full Go regression/vet, Windows Runtime build, real Runtime invocation regression, retained 1.6 Review Precision, 1.6.1 rename, Maven multi-module, duplicate-symbol authority, then an exact-head marker.

## Completion

Task 3 is development-complete only when a fresh Actions run at the exact candidate HEAD is green and contains a dedicated `TASK162_HOTFIX_TASK3_REAL_PLAIN_REVIEW_E2E PASS` marker. Do not start Final Release Certification.