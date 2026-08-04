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

- Do not write or modify test code before an approved test plan exists. Approval is by exact `planId` match — the agent cannot self-approve.
- Do not modify production code before an approved fix plan exists. Approval is by exact `fixPlanId` match — the agent cannot self-approve.
- A newly generated failing test may be repaired and rerun without a second approval when the issue is in test code only.

## Intent routing

The Orchestrator (`harness/agents/orchestrator.md`) routes all user intents:

| Intent | Agents invoked |
|--------|---------------|
| `harness review` | Reviewer |
| `harness test` | Reviewer → Integration Test Agent → Runtime Debugger → (Fix Agent if needed) |
| `harness debug-service` | Runtime Debugger |
| `harness fix finding:<id>` | Fix Agent → Runtime Debugger |
| `harness fix diagnosis:<runId>` | Fix Agent → Runtime Debugger |
| `harness verify test:<class>` | Runtime Debugger |
| `harness verify fix:<fixPlanId>` | Runtime Debugger |
| `harness verify service:<runId>` | Runtime Debugger |

## Agent responsibilities

- **Reviewer**: analyze changes + review code. Read-only.
- **Integration Test Agent**: design test plans + generate/repair test code. Does NOT execute tests or diagnose failures.
- **Runtime Debugger**: execute tests / start services + collect logs + classify failures. Owns diagnosis and nextAction exclusively. Tracks repair round count.
- **Fix Agent**: design minimal fix plans + apply approved changes. Does NOT execute tests.
- **Orchestrator**: routes intents, manages agent handoffs, enforces approval gates, tracks repair rounds, produces user summaries.

## Approval rules

1. The user must explicitly state the exact `planId` or `fixPlanId` to approve. Example: `批准 test-plan-20260804-001` or `批准 fix-plan-20260804-001`.
2. Generic affirmations ("ok", "yes", "继续", "可以", "go ahead") do NOT count as approval.
3. If a plan's content is modified, a new ID must be generated. Old approvals do not transfer.
4. If multiple unapproved plans exist, ask the user which one to act on.

## Tool constraints

- Subagents may only use the controlled tool contracts listed in `harness/tools/README.md`.
- Do not execute arbitrary shell commands. All Maven and service commands must use the exact `executable` and `args` from `harness.yaml`.
- When executing Maven or service commands, the full executable and argument list must be displayed. Shell evaluation (`shell=true`, `eval`, `bash -c`, `sh -c`), pipes, redirection, and command chaining (`&&`, `;`) are prohibited.
- `stop_service` must stop the process tree recorded in the `ServiceHandle` (using `processGroup`), not just a single PID.
- `write_test` requires a `planId` from a human-approved test plan.
- `apply_approved_patch` requires a `fixPlanId` from a human-approved fix plan.

## Test auto-repair limits

- A newly generated failing test may be repaired at most **2 rounds**.
- Repair round count is tracked by the Orchestrator / Runtime Debugger per `planId`.
- After 2 failed repair rounds, stop and emit a schema-valid diagnosis with `nextAction: STOP_UNKNOWN`.
- During repair, the following are prohibited:
  - Deleting tests
  - Adding `@Disabled`
  - Commenting out assertions
  - Weakening assertions (e.g., changing exact value checks to non-null checks)
  - Catching and ignoring exceptions
  - Replacing real internal beans with mocks to bypass production issues

## Result summary format

After every intent completes or stops, output:

```
结果：PASSED | FAILED | WAITING_APPROVAL | MANUAL_ACTION_REQUIRED

完成：
- Review N 个文件
- 生成 M 个测试类
- 执行 K 个场景

发现：
- X 个生产代码问题
- Y 个测试代码问题

下一步：
- 请批准 <planId> | <fixPlanId>
- 或：所有测试通过，无需进一步操作
```

## Prohibited behavior

- No arbitrary shell construction by subagents.
- No production database access.
- No automatic dependency environment provisioning.
- No automatic commit, push, or pull-request creation.
- No unrelated refactoring or weakened assertions.
