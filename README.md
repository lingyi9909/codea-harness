# Codea Harness

Codea Harness V1 is a specification package for Java + Spring Boot + Maven projects. It standardizes Git-diff review, Controller-entry integration tests, local service debugging, failure diagnosis, and approved minimal fixes.

V1 is an **Agent-native specification package** — it does not ship a standalone CLI or Harness Engine. An engineering agent (Codex, OpenCode, or similar) executes the contracts, agent instructions, skills, and project configuration in this repository.

## Quick start

1. Copy `AGENTS.md` to the target project root and adapt the project rules.
2. Copy `harness/harness.example.yaml` to `harness/harness.yaml` and replace example values.
3. Ask the engineering agent to run one of these natural-language intents: `harness review`, `harness test`, `harness debug-service`, `harness fix`, `harness verify`.

## Safety gates

- A test plan must be approved before test files are written. The plan is identified by `planId`; the agent cannot self-approve.
- A production fix plan must be approved before production files are modified. The plan is identified by `fixPlanId`; the agent cannot self-approve.
- Agents may only use the controlled tool contracts listed in `harness/tools/README.md`.
- V1 never commits, pushes, creates pull requests, or executes in test/production environments automatically.
