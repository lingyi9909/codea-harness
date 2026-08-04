# Codea Harness

Codea Harness V1 is a specification package for Java + Spring Boot + Maven projects. It standardizes Git-diff review, Controller-entry integration tests, local service debugging, failure diagnosis, and approved minimal fixes.

V1 is an **Agent-native specification package** — it does not ship a standalone CLI or Harness Engine. An engineering agent (Codex, OpenCode, or similar) executes the contracts, agent instructions, skills, and project configuration in this repository.

## Quick start

1. Copy `AGENTS.md` to the target project root and adapt the project rules.
2. Copy `harness/harness.example.yaml` to `harness/harness.yaml` and replace example values.
3. Ask the engineering agent to run one of these natural-language intents:

| Intent | What it does |
|--------|-------------|
| `harness review` | Analyze Git Diff, review all changed code, output findings |
| `harness test` | Review → design test plan → wait approval → generate tests → execute → diagnose |
| `harness debug-service` | Start service, capture logs, wait for manual requests, diagnose failures |
| `harness fix finding:<id>` | Generate fix plan for a review finding → wait approval → apply fix → verify |
| `harness fix diagnosis:<runId>` | Generate fix plan for a diagnosis → wait approval → apply fix → verify |
| `harness verify test:<class>` | Re-run a specific test class and diagnose |
| `harness verify fix:<fixPlanId>` | Verify a fix by re-running associated tests |
| `harness verify service:<runId>` | Check health of a running service instance |

## Approval protocol

Two formal gates require explicit user approval:

1. **Test Plan**: the agent outputs a plan with a `planId`. Reply `批准 <planId>` to approve.
2. **Fix Plan**: the agent outputs a plan with a `fixPlanId`. Reply `批准 <fixPlanId>` to approve.

Generic affirmations ("ok", "yes", "继续") do **not** count as approval. If a plan is modified, a new ID is generated and the old approval is void.

## Architecture

```
User Intent
    ↓
Orchestrator (routes, manages handoffs, tracks repair rounds)
    ├── Reviewer (analyze-change → review-code)
    ├── Integration Test Agent (design tests → wait approval → generate tests)
    ├── Runtime Debugger (execute tests / start service → analyze failures)
    └── Fix Agent (design fix → wait approval → apply fix)
```

See `harness/agents/orchestrator.md` for the full routing and handoff specification.

## Safety gates

- A test plan must be approved before test files are written. The plan is identified by `planId`; the agent cannot self-approve.
- A production fix plan must be approved before production files are modified. The plan is identified by `fixPlanId`; the agent cannot self-approve.
- Agents may only use the controlled tool contracts listed in `harness/tools/README.md`.
- Test code may be automatically repaired at most 2 rounds before stopping.
- V1 never commits, pushes, creates pull requests, or executes in test/production environments automatically.
