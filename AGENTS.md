# Project Instructions for Codea Harness

## Scope

This repository defines Codea Harness V1. Keep changes limited to specification, contracts, agent instructions, skill instructions, example configuration, and package validation.

## V1 behavior

- Review starts from Git Diff and reads only directly related call-chain code.
- Integration tests enter through MockMvc and use real Controller, Service, Repository, and the project's existing test database configuration.
- Internal Service and Repository beans are not mocked by default.
- External systems, third-party APIs, MQ, and RPC follow the target project's existing test substitution method.
- Local service debugging is independent from integration-test execution.

## Required gates

- Do not write or modify test code before an approved test plan exists. Approval is by `planId` — the agent cannot set its own approval.
- Do not modify production code before an approved fix plan exists. Approval is by `fixPlanId` — the agent cannot set its own approval.
- A newly generated failing test may be repaired and rerun without a second approval when the issue is in test code only.

## Tool constraints

- Subagents may only use the controlled tool contracts listed in `harness/tools/README.md`.
- Do not execute arbitrary shell commands. All Maven and service commands must use the exact `executable` and `args` from `harness.yaml`.
- When executing Maven or service commands, the full executable and argument list must be displayed. Shell evaluation (`shell=true`, `eval`, `bash -c`, `sh -c`), pipes, redirection, and command chaining (`&&`, `;`) are prohibited.
- `stop_service` must stop the process tree recorded in the `ServiceHandle`, not just a single PID.
- `write_test` requires a `planId` from a human-approved test plan.
- `apply_approved_patch` requires a `fixPlanId` from a human-approved fix plan.

## Test auto-repair limits

- A newly generated failing test may be repaired at most **2 rounds**.
- After 2 failed repair rounds, stop and emit a schema-valid diagnosis.
- During repair, the following are prohibited:
  - Deleting tests
  - Adding `@Disabled`
  - Commenting out assertions
  - Weakening assertions (e.g., changing exact value checks to non-null checks)
  - Catching and ignoring exceptions
  - Replacing real internal beans with mocks to bypass production issues

## Prohibited behavior

- No arbitrary shell construction by subagents.
- No production database access.
- No automatic dependency environment provisioning.
- No automatic commit, push, or pull-request creation.
- No unrelated refactoring or weakened assertions.
