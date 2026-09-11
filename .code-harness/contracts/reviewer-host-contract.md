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

## Host attestation boundary

A JSON receipt under `requests/**` is only an index into Host evidence; the receipt itself is not Reviewer authority and is not trusted merely because its fields and proposal hash look valid.

Before Runtime certifies a Reviewer proposal, it MUST resolve the receipt `sessionId` through the local OpenCode Host using `opencode export <sessionId>` and verify all of the following against Host-owned session history:

- the exported session id equals the receipt session id;
- the session has a parent and is therefore an independent child session;
- the relevant user turn is owned by agent `reviewer`;
- the receipt `messageId` identifies the assistant turn for that Reviewer-owned user turn;
- that turn contains a completed `codea-reviewer-submit` submission;
- the submission `runId`, proposal kind, and semantic proposal JSON match the same-run proposal being certified.

A Main Agent / Generic Agent that manually writes `change-analysis-proposal.json` or `finding-proposals.json` plus a forged `*-reviewer-authority.json` MUST be rejected because it cannot manufacture matching Host-owned Reviewer session history.

## Fail closed

If `reviewer` cannot be resolved, started, invoked, or completed, its Host session attestation cannot be verified, or its proposal is missing/malformed/invalid, the only permitted terminal behavior is:

```text
REVIEWER_UNAVAILABLE
MANUAL_ACTION_REQUIRED
HARD STOP
```

After this state there MUST be no analysis certification, review planning/dispatch continuation, Finding certification, or report publication for that review run.

## Forbidden fallback

The Main Agent / Orchestrator MUST NOT perform Reviewer semantic work itself when Reviewer is unavailable. It MUST NOT synthesize `change-analysis-proposal.json` or finding proposals as a fallback, MUST NOT forge Reviewer receipts, and MUST NOT continue the review authority chain.

Reviewer unavailable -> Main Agent semantic fallback is forbidden.

## 1.6.4 Review Host Authority Flow

The only supported product-level review ownership is:

```text
Main Agent / Orchestrator
→ review begin
→ Runtime snapshot
→ independent Reviewer CHANGE_ANALYSIS
→ Runtime certification
→ Runtime planning
→ independent Reviewer FINDINGS
→ Runtime finding certification
→ Runtime report
```

Reviewer owns only the two semantic proposal phases: `CHANGE_ANALYSIS` and `FINDINGS`. Reviewer does not own routing, Runtime execution, certification, planning, progress, final report rendering, or failure recovery.

Main Agent / Orchestrator owns routing, Runtime invocation, Reviewer delegation, Runtime progress rendering, and fail-closed handling. It must use the official Runtime commands `review progress --run-id <runId>` to render `events[].display` and `review reviewer-unavailable --run-id <runId>` when the independent Reviewer Host cannot produce a valid same-run proposal.

The Main Agent / Orchestrator may create only same-run `requests/**` request files. Reviewer proposals must enter the same run only through `codea-reviewer-submit`. `analysis/**`, `review.md`, and `.code-harness/chains/**` remain Runtime/Framework-owned. No semantic fallback to the Main Agent is permitted when Reviewer fails.

OpenCode Host compatibility for 1.6.4 is certified against `opencode-ai@1.18.25`. The resolved Reviewer Host must be a subagent whose effective permissions deny `bash`, `task`, and generic edit/write authority while allowing the dedicated `codea-reviewer-submit` tool; the Reviewer command must resolve to `agent=reviewer` and `subtask=true`.
