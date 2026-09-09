# Reviewer Host Contract

This contract is part of the supported OpenCode review path for Codea Harness 1.6.4.

## Registration

The release package MUST expose the canonical Reviewer as:

```text
.opencode/agents/reviewer.md
```

OpenCode must resolve `reviewer` as `mode: subagent`. The packaged host file is generated from `.code-harness/agents/reviewer.md`; it is not an independent semantic implementation.

## Independent invocation

For every semantic analysis or finding-review phase, the Main Agent / Orchestrator MUST delegate to the OpenCode `reviewer` subagent. A Reviewer run is a child session with an identity distinct from the parent session.

The Reviewer may create proposal material only under same-run:

```text
.code-harness/runs/<runId>/requests/**
```

It MUST NOT create or overwrite Runtime-owned authority under:

```text
.code-harness/runs/<runId>/analysis/**
.code-harness/runs/<runId>/review.md
.code-harness/chains/**
```

Runtime remains the only authority for ChangeAnalysis certification, review planning/dispatch, Finding certification, and report publication.

## Fail closed

If `reviewer` cannot be resolved, started, invoked, or completed, or its proposal is missing/malformed/invalid, the only permitted terminal behavior is:

```text
REVIEWER_UNAVAILABLE
MANUAL_ACTION_REQUIRED
HARD STOP
```

After this state there MUST be no analysis certification, review planning/dispatch continuation, Finding certification, or report publication for that review run.

## Forbidden fallback

The Main Agent / Orchestrator MUST NOT perform Reviewer semantic work itself when Reviewer is unavailable. It MUST NOT synthesize `change-analysis-proposal.json` or finding proposals as a fallback, and MUST NOT continue the review authority chain.

Reviewer unavailable -> Main Agent semantic fallback is forbidden.
