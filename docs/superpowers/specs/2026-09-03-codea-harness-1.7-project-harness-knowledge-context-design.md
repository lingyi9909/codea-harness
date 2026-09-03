# Codea Harness 1.7 — Project Harness & Knowledge Context Design

## Status

Design draft for review. No implementation is authorized by this document.

## Goal

Codea Harness 1.7 的目标不是建设一个新的知识库平台，也不是把 TeamAI 变成 Review Authority。

1.7 只做一件事：**让 Codea Harness 在保持现有 Runtime Authority / Evidence / Certification 模型不变的前提下，能够理解“当前代码变更所在的业务项目、业务流程和业务不变量”，并把这些上下文安全地用于 Code Review。**

目标链路：

```text
OpenCode Agent Host
  -> Codea Harness canonical ChangeSet
  -> Project Business Contract
  -> TeamAI candidate recall / code graph (optional)
  -> Runtime current-source verification
  -> Business Context Snapshot
  -> Agent semantic review
  -> Finding Proposal
  -> Runtime Finding Certification
  -> review.md
```

核心原则：

```text
TeamAI helps find what may matter.
Project Harness declares what the business requires.
Runtime proves what is true in the current code.
Agent compares expected behavior with actual behavior.
Runtime certifies whether the reported problem is admissible.
```

## Baseline

- Design branch base: `e23023481edef9f95cdc59938efe5de4840093b8`.
- This base is the accepted Codea Harness 1.6.2 Post-release Hotfix final certification baseline.
- 1.6.x authority semantics remain locked unless this design explicitly extends them:
  - Runtime is the only deterministic Git ChangeSet authority.
  - Agent owns semantic reasoning, not deterministic facts.
  - Agent writes proposal artifacts only.
  - Runtime-owned artifacts require Runtime provenance/certification.
  - Finding must remain bound to real current-source evidence and the certified review scope.
  - Workspace Dependency is navigation/context only and never silently expands Change Set, Finding Scope or Write Scope.

## Product boundary

### OpenCode

OpenCode is the Agent Host.

It owns generic agent capabilities such as:

- conversation and model interaction;
- reading project files;
- invoking approved tools;
- tool-use orchestration;
- generic planning/reasoning.

1.7 does not attempt to replace or reimplement these capabilities.

### Codea Harness

Codea Harness owns the **enterprise development execution protocol**:

- deterministic Runtime authority;
- ChangeSet / scope / evidence / certification;
- project context protocol;
- business-context resolution and verification;
- Review workflow and completion gates;
- controlled tool contracts;
- stale/unresolved knowledge handling;
- formal Finding certification.

Codea Harness does **not** own the business content itself.

### Project Harness

Project Harness is the business repository's versioned **hard business contract**.

It declares only knowledge that is important enough to influence formal engineering judgment, for example:

- business flows;
- states and allowed transitions;
- business invariants;
- key domain semantics;
- authoritative template/entry hints that cannot be reliably inferred;
- table/projection semantics when code alone cannot express the business meaning.

Project Harness lives with the business source repository and versions together with the code.

### TeamAI

TeamAI is an optional **soft knowledge / candidate context provider**.

Based on TeamAI capabilities available at the design date, it can provide team knowledge recall, imported code knowledge graph, project/user scoped knowledge and source-path hints.

Codea Harness may use these capabilities to accelerate context discovery, but:

- TeamAI output is never authoritative by itself;
- TeamAI recall result can never directly create a Certified Finding;
- TeamAI availability must not be required for correctness;
- a stale TeamAI graph must degrade recall quality, not corrupt Runtime authority;
- Codea Harness must work without TeamAI by falling back to Project Harness + local Runtime navigation.

## Architectural decision

### Considered approach A — Put all knowledge into Codea Harness

Codea Harness would implement knowledge storage, graph, indexing, search, sharing, import and learning management.

Advantages:

- single implementation;
- complete control.

Problems:

- duplicates TeamAI capabilities;
- greatly expands Harness scope;
- mixes Review authority with knowledge-platform concerns;
- increases offline distribution and maintenance complexity.

**Rejected.**

### Considered approach B — TeamAI owns all business knowledge

All project business knowledge would be stored/recalled through TeamAI and consumed directly by Harness.

Advantages:

- minimal Harness storage model;
- strong reuse of TeamAI search/graph.

Problems:

- Recall results are ranked context, not a version-bound business contract;
- code graph may be stale relative to current working tree;
- difficult to prove which code version a critical rule applies to;
- soft knowledge could incorrectly become a Review authority.

**Rejected.**

### Selected approach C — Dual-layer knowledge

```text
TeamAI
Soft Knowledge / Recall / Code Graph / Sharing

        +

Project Harness
Hard Business Contract bound to source Git

        +

Codea Harness Runtime
Current-source Verification / Authority / Evidence
```

This keeps knowledge reuse and Review trust separated.

## Knowledge model

1.7 defines three source classes and three trust states.

### Source class 1 — DECLARED BUSINESS CONTRACT

Source:

```text
.code-harness/project/**
```

Examples:

- `APPROVED` must produce effective data;
- after audit completion, current todo must disappear;
- audit history is append-only;
- a draft deletion must not delete already-effective data;
- `AbstractAuditTemplate` is the intended business flow template.

These facts are human/project declared and versioned with source Git.

They express **what the business requires**, not automatically **what the code currently does**.

### Source class 2 — DISCOVERED CODE FACT

Source:

- current repository source;
- Runtime navigation;
- verified workspace navigation where explicitly allowed;
- source/resource relation evidence.

Examples:

- exact class or method exists;
- exact implementation extends a template;
- exact caller/callee relation exists;
- exact Mapper/XML/resource relation exists;
- exact status assignment occurs in a method;
- exact table/mapper operation is reachable through a verified path.

These express **what current code proves**.

### Source class 3 — CANDIDATE CONTEXT

Source:

- TeamAI recall;
- TeamAI code graph;
- historical team docs;
- model inference;
- other future context providers.

Examples:

- `MerchantEffectiveMapper` is likely related to merchant audit;
- an architecture document says a historical hook is important;
- a code graph points to `AbstractAuditTemplate.execute`.

These are discovery hints only.

### Trust states

Every context item that may enter a Business Context Snapshot must be classified as one of:

```text
CANDIDATE
DECLARED
VERIFIED
```

Additional health status may be:

```text
STALE
UNRESOLVED
REJECTED
```

Rules:

1. `CANDIDATE` may guide further discovery but cannot prove a Finding.
2. `DECLARED` may prove an expected business contract, but not an actual code behavior.
3. `VERIFIED` means Runtime proved the referenced current-source fact.
4. `STALE` means the declaration/reference previously mapped to code but does not match current source identity.
5. `UNRESOLVED` means Runtime cannot prove or disprove the declared/candidate reference.
6. `REJECTED` means current-source verification contradicts the candidate reference.

## Project Harness repository layout

1.7 should introduce a dedicated project contract area rather than continue expanding `project.md` or `harness.yaml` indefinitely.

Recommended layout:

```text
.code-harness/
├── harness.yaml
├── project.md
├── database.yaml
├── chains/
│
└── project/
    ├── manifest.yaml
    ├── domains/
    │   └── merchant.yaml
    ├── flows/
    │   └── merchant-audit.yaml
    ├── invariants/
    │   └── merchant-audit.yaml
    └── knowledge/
        └── merchant.md
```

This is a logical model, not a requirement that every repository create every directory.

A simple project may contain only:

```text
.code-harness/project/
├── manifest.yaml
└── flows/
    └── audit.yaml
```

### `harness.yaml`

Continues to own operational configuration:

- Maven/project shape;
- review baseRef and scope;
- service/test execution;
- write allow/deny paths;
- workspace dependencies;
- Runtime-related settings.

It should not become a business-rule database.

### `project.md`

Remains a human-readable project adaptation summary.

It may summarize discovered project information, but strong business contracts should move into machine-readable `.code-harness/project/**` files.

### `.code-harness/project/**`

Owns project-specific hard business contracts.

## Project manifest

Example:

```yaml
version: 1
projectId: merchant-service

contextProviders:
  teamai:
    enabled: true
    mode: OPTIONAL

businessFlows:
  - id: merchant-audit
    source: flows/merchant-audit.yaml
```

`mode: OPTIONAL` is mandatory for the first TeamAI integration design. TeamAI failure cannot make ordinary Harness Review structurally impossible unless the user later explicitly chooses a stricter organization policy.

## Business Flow Contract

A flow contract should be deliberately small. Configuration by Exception remains the product principle.

Example:

```yaml
version: 1
id: merchant-audit
name: 商户审核

discovery:
  template:
    symbol: com.company.audit.AbstractAuditTemplate

states:
  - DRAFT
  - PENDING
  - APPROVED
  - REJECTED

invariants:
  - AUDIT_SUBMIT_CREATES_TODO
  - AUDIT_APPROVE_CREATES_EFFECTIVE
  - AUDIT_COMPLETE_REMOVES_TODO
  - AUDIT_HISTORY_APPEND_ONLY
```

The project should not manually enumerate every Controller, Service, Mapper and method when Runtime can discover them from the template/entry hints.

## Business Invariant Contract

Example:

```yaml
version: 1
flowId: merchant-audit

invariants:
  - id: AUDIT_APPROVE_CREATES_EFFECTIVE
    description: 审核通过后必须产生生效数据
    severity: high
    when:
      state: APPROVED
    expected:
      effect: CREATE_EFFECTIVE_RECORD

  - id: AUDIT_COMPLETE_REMOVES_TODO
    description: 审核完成后当前待办必须消失
    severity: high
    when:
      stateAnyOf: [APPROVED, REJECTED]
    expected:
      effect: REMOVE_CURRENT_TODO
```

The first version should prefer semantic business language over embedding Java implementation details into invariants.

The mapping from semantic effect such as `CREATE_EFFECTIVE_RECORD` to current implementation is resolved through Runtime discovery/evidence rather than hardcoding every method into the invariant.

## TeamAI Context Provider

### Purpose

TeamAI is used to reduce search space and surface relevant historical/team context.

Example query generated from a changed symbol:

```text
MerchantAuditService approveMerchant merchant audit flow
```

Possible TeamAI result:

```text
MerchantAuditController.approve
MerchantAuditService.approveMerchant
AbstractAuditTemplate.execute
MerchantAuditMapper.updateStatus
MerchantEffectiveMapper.insert
MerchantTodoMapper.delete
MerchantAuditHistoryMapper.insert
```

This result becomes only `CandidateContext[]`.

### Provider contract

Codea Harness should integrate TeamAI behind a provider abstraction rather than scattering raw `teamai` commands through Skills.

Logical contract:

```text
ContextProvider
  recall(query, scope) -> CandidateContext[]
```

Candidate item should preserve at least:

```json
{
  "provider": "TEAMAI",
  "query": "...",
  "kind": "CODE_GRAPH_SOURCE_HINT",
  "path": "merchant-service/src/main/java/.../MerchantEffectiveMapper.java",
  "symbol": "MerchantEffectiveMapper.insert",
  "providerScore": 18.5,
  "matchedTerms": ["merchant", "audit"],
  "missingTerms": [],
  "status": "CANDIDATE"
}
```

Provider score is ranking metadata only. It is not confidence for a Certified Finding.

### Invocation rules

TeamAI should not be invoked for every Review by default.

Suggested policy:

1. Use existing Certified ChangeAnalysis and local Runtime facts first.
2. Load Project Harness contract when changed code maps to a declared domain/flow.
3. Use local Runtime navigation where the next evidence hop is deterministic.
4. Invoke TeamAI when context is broad/ambiguous, cross-module, historically complex, or when local navigation has multiple plausible branches.
5. Use TeamAI result only to prioritize verification candidates.

This avoids making TeamAI a mandatory latency/cost step for simple changes.

## Code Graph candidate verification

This is the key trust boundary.

TeamAI Code Graph answer:

```text
MerchantEffectiveMapper.insert
```

must never become:

```text
VERIFIED because TeamAI says so
```

The required flow is:

```text
TeamAI Code Graph
      |
      v
Candidate symbol/path
      |
      v
Runtime exact current-source lookup
      |
      +-- symbol/path does not exist ------> REJECTED / STALE
      |
      +-- exists but relation unresolved --> UNRESOLVED
      |
      v
Runtime verifies exact relation/evidence
      |
      v
VERIFIED CODE FACT
```

Runtime must verify against the current working source, not TeamAI's imported graph revision.

Verification may reuse existing controlled navigation primitives such as:

- exact symbol lookup;
- references;
- implementations;
- callers;
- template dispatch;
- current-project resource relations;
- explicitly verified workspace navigation.

1.7 should add only the minimum new deterministic primitives required to represent business-flow evidence. It should not build a second general-purpose code graph engine.

## Business Context Resolver

The central new 1.7 concept should be a **Business Context Resolver**, not a TeamAI-specific feature.

Logical input:

```json
{
  "runId": "r123",
  "changeAnalysisPath": ".code-harness/runs/r123/analysis/change-analysis.json",
  "reviewUnitIds": ["RU-..."],
  "flowIds": ["merchant-audit"]
}
```

Resolver inputs:

```text
Certified ChangeAnalysis
Project Harness Contract
Current Source Navigation
Verified Workspace Context (when allowed)
Optional TeamAI Candidate Context
```

Resolver output:

```text
Runtime-owned Business Context Snapshot
```

The resolver owns source classification, verification status and provenance. Agent must not combine arbitrary recall output and project YAML into an authority artifact by itself.

## Business Context Snapshot

Proposed artifact:

```text
.code-harness/runs/<runId>/analysis/business-context.json
.code-harness/runs/<runId>/analysis/business-context.cert.json
```

Conceptual shape:

```json
{
  "runId": "r123",
  "changeAnalysisSha256": "...",
  "projectContractSha256": "...",
  "flows": [
    {
      "id": "merchant-audit",
      "declared": {
        "states": ["DRAFT", "PENDING", "APPROVED", "REJECTED"],
        "invariants": [
          "AUDIT_APPROVE_CREATES_EFFECTIVE"
        ]
      },
      "verified": {
        "templates": [],
        "entrypoints": [],
        "implementations": [],
        "transitions": [],
        "repositories": [],
        "resources": [],
        "sideEffects": []
      },
      "candidates": [
        {
          "provider": "TEAMAI",
          "status": "VERIFIED",
          "path": "...",
          "symbol": "MerchantEffectiveMapper.insert"
        }
      ],
      "unresolved": []
    }
  ],
  "snapshotSha256": "..."
}
```

### Snapshot identity

Business Context Snapshot must bind at least:

- same-run Certified ChangeAnalysis hash;
- Project Harness contract digest;
- verified source evidence identity;
- selected flow IDs;
- resolver version;
- canonical context output digest.

If any relevant Project Harness contract changes after resolution, the old Business Context Snapshot becomes stale.

If current source changes invalidate the underlying Certified ChangeAnalysis, normal 1.6.x fail-closed behavior already invalidates downstream context.

## Dual Snapshot model

1.7 should conceptually expose two independent Runtime facts:

```text
ChangeSet Snapshot
What changed?

Business Context Snapshot
What does this changed code mean in this project's business context?
```

Review consumes both:

```text
ChangeSet / Certified ChangeAnalysis
             +
Business Context Snapshot
             |
             v
Agent Semantic Review
             |
             v
Finding Proposal
             |
             v
Runtime Finding Certification
```

The Business Context Snapshot does not replace Certified ChangeAnalysis or ReviewUnit. It augments semantic context.

## End-to-end `harness review` execution chain

### Step 1 — Canonical change authority

Existing 1.6.2 behavior remains:

```text
harness review
 -> Runtime analysis snapshot
 -> Canonical ChangeSet
 -> Agent ChangeAnalysis Proposal
 -> Runtime analysis certify
 -> Certified ChangeAnalysis / EntryPoint Inventory
```

TeamAI is not involved in Git authority.

### Step 2 — ReviewUnit and ordinary rule dispatch

Existing 1.6 behavior remains:

```text
Certified ChangeAnalysis
 -> ReviewScope
 -> ReviewUnit
 -> existing Rule Dispatch
```

### Step 3 — Determine whether business context is needed

Business context should be requested when at least one condition is met, for example:

- changed ReviewUnit intersects a declared Project Harness flow;
- changed symbol is a template/subclass/hook associated with a declared flow;
- changed code mutates declared business state/effect;
- business-flow rule pack requires Project Context;
- Agent cannot disambiguate relevant business branch using already verified local evidence.

Simple unrelated utility changes should skip this phase.

### Step 4 — Load Project Business Contract

Runtime loads and validates relevant `.code-harness/project/**` contract.

Invalid schema -> business-context feature fails closed for that flow.

The ordinary non-business Review may continue if safe; the formal report must state business-context coverage is unavailable rather than silently pretending the flow was checked.

### Step 5 — Deterministic local discovery first

Runtime resolves declared template/entry/domain hints against current source and builds initial verified graph/evidence.

### Step 6 — Optional TeamAI candidate recall

If context remains broad or ambiguous, a controlled TeamAI provider call returns candidate paths/symbols/docs.

The result remains `CANDIDATE`.

### Step 7 — Runtime candidate verification

Runtime verifies each selected candidate against current source.

Only successful current-source mappings enter `verified` context.

### Step 8 — Publish Business Context Snapshot

Runtime publishes `business-context.json` and certificate.

Agent consumes this artifact instead of manually merging raw TeamAI output with YAML and source guesses.

### Step 9 — Agent semantic business review

Agent evaluates:

```text
Declared business invariant
        versus
Verified current implementation evidence
        versus
Current ChangeSet causality
```

Example:

```text
DECLARED:
APPROVED must create effective data

VERIFIED IMPLEMENTATION:
approve -> history insert -> todo delete
no verified effective write in the relevant branch

CHANGE CAUSALITY:
current change removed/bypassed the effective-write path
```

Only then may Agent produce a Finding Proposal.

### Step 10 — Runtime Finding Certification

Existing 1.6 Finding authority remains mandatory.

For a Business Flow Finding, Runtime additionally verifies:

- referenced invariant exists in the current Project Harness digest;
- Business Context Snapshot is current and certified;
- all claimed actual-code evidence is VERIFIED;
- Finding anchor remains inside allowed Finding Scope;
- introduced-by-change / causal evidence is sufficient for the applicable rule;
- candidate-only TeamAI evidence is not being used as proof.

Only then may the issue enter Certified Findings and `review.md`.

## Business Flow Review as first consumer

Project Harness infrastructure should not be built as a generic knowledge platform first.

Its first concrete consumer should be B-end business flow Review.

Initial semantic categories may include:

```text
STATE_TRANSITION
FLOW_COMPLETENESS
PROJECTION_CONSISTENCY
SIDE_EFFECT_CONSISTENCY
TEMPLATE_HOOK
```

Examples:

- illegal state transition introduced by the change;
- APPROVED path no longer creates required effective projection;
- completed flow leaves current todo behind;
- template override bypasses a mandatory hook;
- history/effective/todo projections become inconsistent after a changed transition.

These categories remain Agent-semantic unless Runtime can fully prove a specific deterministic condition.

## Candidate context must never expand authority scope

TeamAI may return code outside the current ReviewUnit or even outside the current repository.

Rules:

1. Candidate path does not automatically enter Review Scope.
2. Candidate path does not automatically enter Finding Scope.
3. Candidate path does not automatically become Write Scope.
4. Workspace candidate must still pass existing explicit workspace allowlist/verification rules.
5. External/team documentation may explain context but cannot act as line/symbol evidence.
6. A Finding must remain anchored and causally related to the certified current-project change according to existing 1.6 rules.

This preserves the existing distinction:

```text
Context Scope != Change Scope != Finding Scope != Write Scope
```

## Staleness model

Knowledge staleness is a first-class problem.

### Project Harness staleness

A declared source reference may become stale when:

- class/method is renamed;
- template is removed;
- module/path moves;
- declared state no longer exists;
- flow mapping contradicts current code structure.

Runtime resolution should mark such references `STALE` or `UNRESOLVED`.

Critical unresolved business contracts must not silently participate in Certified Findings.

### TeamAI staleness

TeamAI graph/docs may refer to an older source revision.

1.7 does not require TeamAI to prove graph revision equality with current working tree because Runtime always re-verifies candidates.

Therefore TeamAI staleness causes:

```text
more REJECTED / UNRESOLVED candidates
```

not:

```text
incorrect VERIFIED evidence
```

## Failure and degradation behavior

### TeamAI unavailable

Behavior:

```text
TEAMAI_PROVIDER_UNAVAILABLE
```

Harness falls back to:

- Project Harness contract;
- local Runtime navigation;
- existing Workspace Dependency navigation where allowed.

Do not fail an otherwise complete ordinary Review solely because optional TeamAI is unavailable.

### TeamAI returns no relevant candidate

Continue local discovery. No Finding consequence.

### Project Harness missing

Ordinary existing 1.6 Review continues.

Business Flow Review status is `NOT_CONFIGURED`, not PASS.

### Project Harness invalid

Business-flow review for that contract is blocked/fail-closed and surfaced clearly.

### Critical declared reference unresolved

Business-flow coverage becomes PARTIAL / MANUAL_ACTION_REQUIRED according to final 1.7 gate design; do not manufacture a PASS.

## Knowledge lifecycle

### Soft knowledge lifecycle

TeamAI owns:

```text
team docs
session learnings
shared rules/skills
code graph/teamwiki
historical explanations
cross-project knowledge
```

### Hard knowledge lifecycle

Project Harness owns:

```text
business flow contract
business invariants
project-specific semantic facts that affect formal Review judgment
```

### Knowledge promotion

A useful future workflow is **Knowledge Promotion**:

```text
TeamAI recalled learning
        |
        v
Human determines it is a durable business rule
        |
        v
Proposal to .code-harness/project/**
        |
        v
Human review/confirmation
        |
        v
Versioned Project Business Contract
        |
        v
Runtime validation
```

Important: 1.7 initial scope may define this concept but does not need to implement an automatic promotion workflow.

TeamAI must never auto-promote session knowledge into hard business contract without an explicit human-reviewed change.

## Dynamic facts

Dynamic facts should not be copied into Project Harness or TeamAI as current truth.

Examples:

- current test environment status;
- current logs;
- database rows;
- CI state;
- work-item status;
- service health.

Future providers may expose these through controlled CLI/MCP/internal API adapters.

They should enter as run-scoped evidence, not durable business contract.

1.7 Business Flow Review does not require broad dynamic-fact integration.

## TeamAI/OpenCode integration boundary

The design does not depend on TeamAI natively injecting itself into OpenCode.

Preferred initial integration:

```text
OpenCode
  -> Codea Harness skill/agent flow
  -> controlled Context Provider invocation
  -> teamai recall / future TeamAI machine-readable command
```

This keeps the architecture stable even if TeamAI changes its supported Agent integrations.

If TeamAI later offers a first-class OpenCode integration, Codea Harness may use it for UX optimization, but the provider/authority boundary must remain unchanged.

## Security / offline constraints

1.7 must preserve existing enterprise constraints:

- Windows 10/11 x64;
- no WSL requirement;
- no automatic internet access;
- no `git fetch/pull` performed by Harness;
- no arbitrary shell command construction;
- TeamAI invocation must be configured for the company's approved local/team knowledge source;
- TeamAI absence must be a supported state;
- Project Harness contract is local Git content and remains usable fully offline;
- no production DB/resource access as part of Business Flow Review.

## Data ownership summary

| Data | Owner | Trust / Use |
|---|---|---|
| Canonical ChangeSet | Codea Harness Runtime | authoritative |
| Certified ChangeAnalysis | Codea Harness Runtime | authoritative |
| Project business invariant | Project Harness | declared expected behavior |
| Current symbol/call/resource evidence | Codea Harness Runtime | verified actual behavior |
| TeamAI recall result | TeamAI | candidate context only |
| TeamAI code graph source hint | TeamAI | candidate navigation only |
| Business Context Snapshot | Codea Harness Runtime | certified combined context |
| Finding Proposal | Agent | semantic proposal |
| Certified Finding | Codea Harness Runtime | formal report authority |

## Responsibility summary

```text
OpenCode
  Agent Host / generic tool orchestration

TeamAI
  Soft knowledge / recall / code graph / sharing

Project Harness
  Version-bound hard business contract

Codea Harness Agent
  Semantic reasoning over certified context

Codea Harness Runtime
  Current-source facts / verification / provenance / certification / gates
```

## Non-goals for initial 1.7

Explicitly out of scope:

- building a new search engine;
- building a new vector database;
- building a general organization knowledge portal;
- replacing TeamAI code graph;
- letting TeamAI create formal Findings directly;
- project-wide always-on whole-repository graph certification;
- automatically turning every session learning into a business rule;
- PRD/solution/test/archive full lifecycle platform;
- production database/log integration;
- generic multi-language Project Harness expansion;
- changing the accepted 1.6 ChangeSet/Finding authority model;
- making TeamAI mandatory for `harness review` correctness.

## Initial 1.7 scope proposal

The first implementation plan should be decomposed around these capabilities only:

1. Project Harness contract root and schemas.
2. Business Flow + Business Invariant v1 contracts.
3. Runtime contract loader, digest and staleness validation.
4. Business Context Resolver.
5. Business Context Snapshot + certificate.
6. Optional TeamAI Context Provider returning candidate-only context.
7. Runtime verification of TeamAI code/source candidates.
8. Business Flow Review rule dispatch and semantic Finding integration.
9. Retained 1.6.x scope/authority regressions.
10. Real plain `harness review` E2E proving optional TeamAI and no-TeamAI paths.

This list is design decomposition, not yet the implementation Task plan.

## Acceptance principles for later planning

The eventual implementation should prove at least:

1. Removing TeamAI does not change canonical ChangeSet authority or make ordinary Review incorrect.
2. A TeamAI result pointing to a nonexistent/stale symbol cannot become VERIFIED.
3. A high-scoring TeamAI recall cannot directly produce a Certified Finding.
4. A valid Project Harness invariant plus VERIFIED contradictory implementation evidence can support a business Finding.
5. An unresolved critical business contract cannot silently manufacture PASS.
6. TeamAI candidate files cannot expand Change Set / Finding Scope / Write Scope.
7. Project Harness contract changes invalidate stale Business Context Snapshot.
8. Multi-module and duplicate-symbol identity continue to use exact path-qualified evidence.
9. `harness review` still works without TeamAI installed/configured.
10. A configured TeamAI path demonstrably reduces/targets context discovery without weakening Runtime verification.

## Design questions intentionally left for review

The architecture above fixes component boundaries. The following implementation-level choices should be reviewed before writing the 1.7 plan:

1. Whether `flows/*.yaml` and `invariants/*.yaml` remain separate files or invariants are embedded directly in the flow for V1 simplicity.
2. Exact minimal semantic vocabulary for effects such as `CREATE_EFFECTIVE_RECORD`, `REMOVE_CURRENT_TODO`, `APPEND_HISTORY`.
3. Whether Business Context Snapshot is one artifact per run or one artifact per ReviewUnit/flow with a run-level index.
4. Exact machine-readable TeamAI invocation/output adapter for the company environment.
5. What threshold makes unresolved Project Harness context `PARTIAL` versus `MANUAL_ACTION_REQUIRED`.
6. Which existing Runtime navigation primitives are sufficient and which minimal business-flow-specific evidence primitives must be added.
7. Whether Knowledge Promotion belongs to 1.7.x or a later knowledge-evolution version.

These questions should be decided during design review rather than silently encoded during development.

## One-sentence architecture

> **TeamAI helps Codea Harness find relevant candidates; Project Harness declares the versioned business contract; Codea Harness Runtime verifies current-code truth and publishes certified Business Context; the Agent performs semantic comparison; Runtime remains the final Finding authority.**
