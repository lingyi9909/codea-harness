# Codea Harness 1.6.4 Release Blocker Repair Plan

**Date:** 2026-09-09  
**Repository:** `lingyi9909/codea-harness`  
**Superseded RC baseline:** `6aa5d9dad0623cd60a845360c9b20ab153921e87`  
**Status:** Release blocker repair plan  

> The current 1.6.4 RC is revoked as a release candidate until every task and final certification gate in this document passes on one exact HEAD.

## 1. Scope

This repair is intentionally limited to four release blockers:

1. deterministic `1.6.3 -> 1.6.4` config migration;
2. real Reviewer host registration / independent invocation and fail-closed review authority;
3. Runtime-owned review lifecycle state machine with OpenCode progress rendering;
4. product-level upgrade/review E2E and final re-certification.

Do not use this work to redesign review semantics, chain authority, finding authority, report semantics, Upgrade V2 hash-aware delta semantics, analysis certification performance semantics, or unrelated Runtime/Agent architecture.

Each development task MUST stop for acceptance before the next task starts.

---

# Task 1 — Deterministic Config Migration 1.6.3 -> 1.6.4

## Goal

A real project installed with the exact packaged 1.6.3 release must upgrade to 1.6.4 without being rejected merely because its existing `harness.yaml` conforms to the 1.6.3 schema rather than the 1.6.4 target schema.

The Runtime must migrate source config deterministically before validating it against the target schema.

## First required investigation

Before coding, diff the exact release schemas:

```text
exact 1.6.3 contracts/harness-config.schema.json
vs
current 1.6.4 contracts/harness-config.schema.json
```

Record every backward-incompatible change and determine which root cause is present:

1. the `1.6.3 -> 1.6.4` migration is missing;
2. a migration exists but is not registered in the authoritative version edge;
3. target-schema validation happens before migration;
4. more than one of the above.

Do not fix this by weakening the 1.6.4 schema.

## Authoritative order

```text
1.6.3 harness.yaml
-> detect exact source version
-> deterministic 1.6.3 -> 1.6.4 migration
-> migrated harness.yaml
-> validate against 1.6.4 target schema
-> compute/apply upgrade delta
-> publish upgrade result
```

Forbidden order:

```text
old harness.yaml
-> validate directly against 1.6.4 schema
-> fail before migration
```

## Migration requirements

The migration MUST be:

- deterministic;
- idempotent;
- Runtime-owned;
- explicit by source/target version;
- preserving all compatible user-authored values;
- free of LLM/heuristic inference;
- fail-closed for unsupported or ambiguous source configuration;
- rollback-safe with the existing Upgrade V2 transaction semantics.

No silent deletion or replacement of user configuration is allowed.

## Primary implementation areas

Inspect the actual repository before editing, especially:

```text
contracts/harness-config.schema.json
harness.template.yaml

tools-runtime/internal/upgrade/upgrade.go
tools-runtime/internal/upgrade/inventory.go
tools-runtime/internal/upgrade/migration_14.go
tools-runtime/internal/upgrade/source_runtime.go

skills/upgrade-harness/SKILL.md
upgrade.md
```

`migration_14.go` is an existing deterministic migration pattern; it is not automatically the file that must be changed.

Prefer a 1.6.4-specific migration implementation and tests rather than overloading old migration behavior.

## Mandatory RED/GREEN evidence

First reproduce RED with a real 1.6.3 config:

```text
real 1.6.3 harness.yaml
+
1.6.4 upgrade
-> current failure
```

Then prove GREEN markers:

```text
CONFIG_MIGRATION_163_TO_164_REGISTERED PASS
CONFIG_MIGRATION_BEFORE_TARGET_SCHEMA_VALIDATION PASS
CONFIG_MIGRATION_TARGET_SCHEMA_VALID PASS
CONFIG_MIGRATION_IDEMPOTENT PASS
CONFIG_USER_VALUES_PRESERVED PASS
CONFIG_UNSUPPORTED_MIGRATION_FAIL_CLOSED PASS
```

## Long-term regression gate

Future backward-incompatible changes to `harness-config.schema.json` must require one of:

```text
explicit sourceVersion -> targetVersion migration
```

or a test proving the change is backward compatible for every supported source version.

A schema change without migration coverage must fail CI.

## Product E2E required in Task 1

Use the exact packaged 1.6.3 release, not a hand-built mock tree:

```text
exact packaged 1.6.3
-> install/create real project
-> retain real 1.6.3 harness.yaml and user values
-> run candidate 1.6.4 upgrade
-> automatic migration
-> 1.6.4 schema validation PASS
-> upgrade PASS
-> user values preserved
-> run upgrade/migration again
-> idempotency PASS
```

Also prove:

- unknown user files are preserved;
- Upgrade V2 hash-aware delta semantics are unchanged;
- rollback still restores the pre-upgrade state on failure;
- failed migration leaves no half-upgraded installation.

## Task 1 fresh verification

From exact HEAD:

```bash
cd tools-runtime
go test -count=1 ./internal/upgrade/...
go test -count=1 ./...
go vet ./...
```

Also run a fresh Windows exact-head packaged `1.6.3 -> 1.6.4` upgrade E2E.

## Task 1 handoff evidence

Return:

1. exact HEAD;
2. exact 1.6.3/1.6.4 schema diff and root-cause conclusion;
3. RED evidence;
4. migration registration/order evidence;
5. user-value preservation evidence;
6. idempotency evidence;
7. unsupported migration fail-closed evidence;
8. real packaged Windows upgrade E2E;
9. fresh full Go tests;
10. fresh `go vet ./...`;
11. exact-head Windows CI run.

**STOP after Task 1. Do not enter Task 2 before acceptance.**

---

# Task 2 — Reviewer Host Registration + Fail-Closed Review Authority

## Goal

The Reviewer must exist as a real independently invokable host agent in the supported OpenCode product path. Review must never silently fall back to the Main Agent / Orchestrator performing Reviewer semantic work itself.

## Required investigation

Trace the real packaged OpenCode installation path and answer with code evidence:

1. how `agents/reviewer.md` becomes a host-visible agent;
2. which generated/installed file or manifest registers it;
3. how the Orchestrator invokes it;
4. how host/runtime errors are surfaced;
5. whether current packaging tests validate registration or only file presence.

Do not assume that shipping `agents/reviewer.md` means OpenCode can invoke it.

Primary areas include:

```text
agents/orchestrator.md
agents/reviewer.md
skills/review-code/SKILL.md
bootstrap.md
upgrade.md
RELEASE-MANIFEST.json / release packaging inventory if present in the release tree
```

and the actual Runtime/release code that installs host resources.

## Authority invariant

The required review path remains:

```text
Runtime analysis snapshot
-> canonical ChangeSet
-> independent Reviewer semantic analysis proposal
-> Runtime analysis certify
-> Runtime review planning/selection/units/dispatch
-> independent Reviewer finding proposals
-> Runtime finding certify
-> Runtime report
```

The Reviewer proposes semantic content. Runtime owns authoritative artifacts and all certification gates.

## Fail-closed contract

If the independent Reviewer cannot be resolved, started, invoked, or completed:

```text
REVIEWER_UNAVAILABLE
-> MANUAL_ACTION_REQUIRED
-> HARD STOP
```

No authoritative downstream work may continue.

Forbidden fallback:

```text
Reviewer unavailable
-> Main Agent / Orchestrator reads ChangeSet
-> Main Agent performs semantic review itself
-> review continues
```

That must be structurally/testably impossible.

## Mandatory negative cases

Cover at minimum:

- Reviewer registration missing;
- Reviewer file present but not host-invokable;
- Reviewer start/invocation error;
- Reviewer session crash/cancel before proposal completion;
- malformed Reviewer output;
- Reviewer produces no valid proposal;
- Main Agent attempts semantic fallback.

Expected result for every unavailable Reviewer case:

```text
no certified analysis/findings/report publication
no chain/review dispatch continuation
explicit REVIEWER_UNAVAILABLE / MANUAL_ACTION_REQUIRED
```

## Mandatory product E2E

Run through a real packaged OpenCode environment and prove:

```text
Reviewer listed/resolvable by host
Reviewer receives the intended review input
Reviewer runs in an independent session/agent identity
Reviewer produces proposal only
Runtime certifies/rejects proposal
Main Agent is not Reviewer authority
```

Also run a real negative E2E with Reviewer intentionally unavailable and prove the hard stop.

## Task 2 acceptance markers

```text
REVIEWER_HOST_REGISTRATION PASS
REVIEWER_INDEPENDENT_INVOCATION PASS
REVIEWER_IDENTITY_EVIDENCE PASS
REVIEWER_RUNTIME_AUTHORITY_SEPARATION PASS
REVIEWER_UNAVAILABLE_FAIL_CLOSED PASS
MAIN_AGENT_REVIEWER_FALLBACK_FORBIDDEN PASS
```

**STOP after Task 2. Do not enter Task 3 before acceptance.**

---

# Task 3 — Runtime Review State Machine + OpenCode Progress Events

## Goal

A top-level `harness review` must expose deterministic, Runtime-owned stage progress so the user can see where the review is and failures can be attributed to a concrete stage.

OpenCode renders Runtime state; it does not invent state.

## Canonical eight stages

```text
1/8 REVIEW_BEGIN
2/8 SNAPSHOT
3/8 CHANGE_ANALYSIS
4/8 CERTIFICATION
5/8 REVIEW_PLANNING
6/8 REVIEW_EXECUTION
7/8 FINDING_CERTIFICATION
8/8 REPORT
```

Each stage must have one authoritative status model, for example:

```text
PENDING
RUNNING
SUCCEEDED
FAILED
BLOCKED
```

The exact wire schema may differ, but semantics must remain deterministic.

## Runtime ownership

Runtime owns:

- runId;
- stage identity/order;
- stage transition legality;
- timestamps/durations if recorded;
- terminal status;
- failure code and failing stage;
- artifact identity associated with completed stages.

Agent/prompt layers may not arbitrarily advance a stage or mark success.

## Lifecycle requirements

Every top-level review starts fresh:

```text
harness review
-> review begin
-> fresh Runtime runId
-> stage 1/8
```

A prior OpenCode session must not reuse the previous review run's stage/artifact state.

Legal ordering must be fail-closed. Examples:

- `CERTIFICATION` cannot succeed before `CHANGE_ANALYSIS`;
- `REVIEW_EXECUTION` cannot begin before certified planning/selection exists;
- `REPORT` cannot begin before finding certification terminal success;
- failure/block in an earlier stage prevents later authoritative stages.

## OpenCode rendering contract

OpenCode should display Runtime progress events such as:

```text
[1/8] REVIEW_BEGIN          PASS
[2/8] SNAPSHOT              PASS
[3/8] CHANGE_ANALYSIS       RUNNING
...
```

The visual formatting can be adapted to the host, but the event source must be Runtime-owned state.

No prompt-only progress text may be treated as authority.

## Persistence / observability

Persist enough per-run state under the existing Runtime-owned run tree to diagnose interruptions without making the progress file part of semantic certificate hashes unless explicitly designed that way.

Recommended evidence includes:

- runId;
- current/terminal stage;
- per-stage status;
- start/end time or duration;
- stable failure code;
- associated artifact identifiers/hashes where relevant.

Do not persist source code or secrets merely for progress display.

## Mandatory interruption tests

At minimum fail/interrupt each of:

- snapshot;
- change analysis Reviewer call;
- analysis certification;
- review planning;
- review execution Reviewer call;
- finding certification;
- report.

Each must prove:

```text
correct failing stage
correct terminal state
no later authoritative stage executed
no false PASS emitted
```

## Task 3 acceptance markers

```text
REVIEW_STAGE_ORDER_RUNTIME_OWNED PASS
REVIEW_STAGE_TRANSITION_FAIL_CLOSED PASS
REVIEW_FRESH_RUN_STATE PASS
REVIEW_STAGE_FAILURE_ATTRIBUTION PASS
OPENCODE_PROGRESS_FROM_RUNTIME_EVENTS PASS
PROMPT_ONLY_PROGRESS_NOT_AUTHORITY PASS
```

**STOP after Task 3. Do not enter Task 4 before acceptance.**

---

# Task 4 — Product E2E + Final Re-Certification

## Goal

Re-certify one exact candidate HEAD as a new 1.6.4 RC only after Tasks 1-3 are accepted.

The superseded `6aa5d9dad0623cd60a845360c9b20ab153921e87` RC must never be restored as the final certified 1.6.4 merely by changing documentation.

## Gate A — Real 1.6.3 -> 1.6.4 Upgrade E2E

Using official packaged artifacts:

```text
install exact 1.6.3
-> retain real user config + unknown files
-> upgrade to exact 1.6.4 candidate
-> deterministic migration
-> target schema PASS
-> managed delta PASS
-> unknown files preserved
-> release manifest correct
-> second upgrade idempotent
```

Run Windows first-class evidence.

## Gate B — Real OpenCode Review E2E

Use a real supported OpenCode host with the packaged candidate:

```text
harness review
-> fresh runId
-> Runtime 1/8..8/8 stages
-> canonical snapshot
-> independent Reviewer change analysis
-> Runtime certification
-> Runtime review planning
-> independent Reviewer review execution
-> Runtime finding certification
-> Runtime report
```

Evidence must prove independent Reviewer identity/session and Runtime authority boundaries.

## Gate C — Reviewer unavailable negative E2E

Intentionally remove/break Reviewer host registration:

```text
harness review
-> Reviewer resolution/invocation failure
-> REVIEWER_UNAVAILABLE
-> MANUAL_ACTION_REQUIRED
-> terminal hard stop
```

Must prove no fallback semantic review and no final report.

## Gate D — Progress / interruption E2E

At least one controlled mid-review failure must prove the exact Runtime stage is shown and later stages never run.

## Gate E — Retained 1.6.4 performance/authority regressions

Re-run the previously accepted 1.6.4 analysis certification performance and authority gates on the final exact HEAD, including the large-repo/small-ChangeSet Windows P0 gate:

- snapshot-bounded Entrypoint scan;
- no repo/module/source-root scan;
- AST process count upper bound;
- Base git batch upper bound;
- no per-file merge-base/show;
- repository-size invariance;
- freshness fast-path invariants;
- existing ReviewOptions / USER_SELECTION authority invariants.

These repair tasks must not regress prior 1.6.4 work.

## Gate F — Full exact-head verification

From one exact commit:

```bash
cd tools-runtime
go test -count=1 ./...
go vet ./...
```

Also run all release/package/Windows workflows required by the existing release design.

No result from an older commit may be used as final certification evidence.

## Final certification evidence

The release record must contain:

1. exact candidate HEAD;
2. exact package hashes;
3. Task 1 migration markers;
4. Task 2 Reviewer markers;
5. Task 3 progress/state-machine markers;
6. real packaged 1.6.3 -> candidate upgrade evidence;
7. real OpenCode positive review evidence;
8. real Reviewer-unavailable negative evidence;
9. retained 1.6.4 performance/authority evidence;
10. fresh full Go test/vet evidence;
11. Windows exact-head workflow links/artifacts.

Only after all gates pass may the new candidate be called **1.6.4 Release Candidate** and proceed toward `main`.

---

# Global prohibitions

Throughout Tasks 1-4, do not:

1. weaken the target schema to hide a missing migration;
2. ask users to manually rewrite an otherwise supported 1.6.3 config before upgrade;
3. use LLM/heuristics for deterministic config migration;
4. allow Main Agent / Orchestrator to replace an unavailable Reviewer;
5. treat prompt instructions as Reviewer registration;
6. treat prompt progress text as Runtime state;
7. skip failed review stages and continue downstream;
8. reuse stale runId/snapshot/analysis/report for a new top-level review;
9. change USER_SELECTION semantics before certification;
10. reopen already accepted analysis-certification performance architecture without a concrete regression caused by this repair;
11. declare release success from unit tests alone;
12. certify different gates on different HEADs and combine them into one release claim.

# Development sequence

```text
Task 1
-> acceptance
Task 2
-> acceptance
Task 3
-> acceptance
Task 4 Final Re-Certification
```

No parallel task completion is accepted where it makes per-task authority or evidence ambiguous.
