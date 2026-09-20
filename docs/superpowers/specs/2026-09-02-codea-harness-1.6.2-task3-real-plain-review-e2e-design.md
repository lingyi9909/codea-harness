# Codea Harness 1.6.2 Post-release Hotfix — Task 3 Design

## Goal

Prove that the ordinary user intent `harness review` can execute through a real Agent Host and the active Codea Harness contracts to a final Runtime-rendered `review.md`, without an Agent → Runtime request-contract drift.

Task 3 is an E2E acceptance layer. It does not change Review, Finding, Chain, Maven, Canonical ChangeSet, or Runtime business semantics.

## Accepted baseline

Task 2 Accepted HEAD:

`2503678e347dc0ba2bc2f0357cefd9306d199480`

## E2E boundary

The user-facing input to the review session is exactly:

```text
harness review
```

The test driver must not pre-create Snapshot, certify, review-options, review-selection, Finding, or report requests and must not manually sequence Runtime commands as a substitute for the Agent.

The E2E uses:

- a real Windows OpenCode headless Agent Host;
- a minimal host adapter whose only bootstrap responsibility is to tell the Agent that `harness ...` intents must read and follow `.code-harness/AGENTS.md`; it contains no Runtime request fields or review business logic;
- the active `.code-harness/AGENTS.md`, Orchestrator, Reviewer, Skills, and JSON contracts from the checkout under test;
- the freshly built `codea-dcep-tools.exe` and pinned ast-grep 0.42.1;
- a local deterministic OpenAI-compatible model stub so CI does not depend on external model credentials or nondeterministic model output.

The model stub is allowed to choose tool calls, but before it creates each formal Agent → Runtime request it must have read the corresponding active contract. OpenCode performs the actual file reads/writes and shell execution.

## Minimal fixture

Use an initialized Maven-shaped fixture with one benign changed file:

`src/main/resources/application.yml`

The framework and project state are committed before the working-tree change. Runtime binaries are injected after commit. The only Review ChangeSet is the intended YML change.

The semantic proposal has no Java symbols/call chains and records the changed YML as `YamlConfig` with COMPLETE coverage. This makes the plain review decision deterministic: no valid business chains → `AUTO_FULL`.

## Required real flow

```text
plain user message: harness review
→ Agent reads Active Contract
→ Agent writes change-set request
→ Runtime analysis snapshot
→ Agent reads snapshot and writes semantic proposal
→ Agent writes canonical analysis-certify request
→ Runtime analysis certify
→ Agent writes review-options request
→ Runtime review options (AUTO_FULL)
→ Agent writes review-selection request using current optionsHash
→ Runtime review select → verified FULL scope
→ Runtime review units
→ Runtime review dispatch
→ Agent writes empty Finding Proposal for the benign change
→ Runtime certify-findings
→ Agent writes report transport
→ Runtime report review
→ .code-harness/runs/<runId>/review.md
```

## Black-box acceptance assertions

The E2E must prove from the resulting workspace and Agent transcript that:

1. the only review user message is plain `harness review`;
2. Active Agent/Orchestrator/Reviewer/required request contracts were actually read;
3. Snapshot request contains only the current allowed fields and does not contain `requestedBaseRef`;
4. canonical certify request references same-run Snapshot + `snapshotSha256` + semantic proposal;
5. review-options request contains no `baseRef`;
6. the Runtime-owned canonical Snapshot, Certified ChangeAnalysis, inventory, review options/origin, review scope, review units, dispatch, certified findings, and final `review.md` are produced by the real flow;
7. no `REQUEST_*_SCHEMA_INVALID`, `REVIEW_OPTIONS_REQUEST_SCHEMA_INVALID`, old executable invocation, retry-after-contract-failure, or manual JSON repair occurs;
8. final report is a Runtime-rendered successful FULL review for the fixture.

## CI

Add a Task 3 Windows workflow that:

1. checks out the exact branch head;
2. builds the Runtime;
3. injects pinned ast-grep 0.42.1;
4. installs a pinned OpenCode CLI version;
5. runs the real plain-review E2E;
6. runs retained Task 1 and Task 2 hotfix gates plus existing 1.6/1.6.1/1.6.2 regressions;
7. ends with an exact-head marker.

## Non-goals

Do not add a product `harness.exe`, change Agent Review semantics, alter Runtime request structs/schemas, change Finding/Chain/Maven behavior, or start Final Release Certification.