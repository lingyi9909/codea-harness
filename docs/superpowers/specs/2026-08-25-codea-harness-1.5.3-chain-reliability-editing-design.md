# Codea Harness 1.5.3 — Chain Reliability, Review Selection & Editing Design

## Goal

Fix the reliability and authority gaps discovered in real 1.5.1 usage, then make Business Chain interaction practical for developers.

The release focuses on four user-visible outcomes:

1. `harness review` must never silently miss changed Controller entry points;
2. Agent-generated JSON/YAML is proposal data, not authoritative Chain/Review evidence;
3. `harness review` with no target must let the user choose FULL Review or one/more verified business Chains after Chain discovery is proven complete;
4. developers should be able to correct a Chain through natural language instead of hand-editing YAML.

## Baseline

- Exact 1.5.2 release baseline: `6f290d8ff160767bb981278aa123aa1621ea3343`
- Existing 1.5.2 Workspace Dependency Navigation, Review Isolation, Chain Contract, Task 5 real dual-project regression and Windows release behavior are preserved.
- Existing Chain YAML remains `version: 1`; no Project State Chain migration is required.

## Real defects this release addresses

### Defect A — silent Controller omission

Observed in 1.5.1:

```text
Change Set:
- new AController
- new BController
- modified CController

harness review
→ only AController Chain discovered

harness review BController
→ BController Chain can be discovered correctly
```

Current `chain discover` iterates all `ChangeAnalysis.affectedControllers[]`, so the missing Chain can happen earlier when Agent-built ChangeAnalysis silently omits Controller/entrypoint facts. Existing machine Coverage verifies changed files and unresolved symbols, but does not independently prove that all changed Controller entry points are represented.

### Defect B — Agent manufactures a Chain after Runtime did not

Observed behavior:

```text
Runtime does not produce Chain
→ Agent directly edits/creates Chain JSON/YAML
→ Agent continues Review using that Chain
```

This violates the existing design. Reviewer may propose analysis facts, but authoritative discovered/refresh/persisted Chain artifacts must be Runtime-owned and machine validated.

### Defect C — Chain YAML is too difficult to maintain manually

Current Chain YAML contains entryPoints, nodes, workspace, exact path, role, resources, boundaries, status and identity data. The original “developer edits YAML, then validate” assumption is not practical enough for normal usage.

### UX gap — no FULL vs Chain selection for plain `harness review`

1.5.1 semantics are currently:

```text
harness review                → FULL
harness review list           → LIST
harness review <Class>        → TARGETED CLASS
harness review <Class.method> → TARGETED METHOD
```

Plain `harness review` does not currently ask whether the user wants FULL Review or selected business Chains.

## Global trust model

1.5.3 adopts one permanent rule:

```text
Agent proposes.
Runtime certifies.
Only certified/runtime-owned artifacts are authoritative.
```

Agent output is useful for semantic analysis, but cannot by itself authorize Review Scope, Chain facts, Chain persistence or Project State writes.

## Artifact ownership boundary

### Agent/Orchestrator proposal paths

Agent may create request/proposal data only under the current run request area:

```text
.code-harness/runs/<runId>/requests/**
```

Examples:

```text
change-analysis-draft.json
review-selection-request.json
chain-edit-request.json
chain-write-request.json
```

### Runtime-owned paths

Only Controlled Runtime may create authoritative result artifacts:

```text
.code-harness/runs/<runId>/analysis/**
.code-harness/runs/<runId>/review.md
.code-harness/chains/**
```

This includes:

```text
analysis/change-analysis.json
analysis/change-analysis.cert.json
analysis/entrypoint-inventory.json
analysis/discovered-chains/**
analysis/refresh-candidates/**
analysis/chain-edit-candidates/**
analysis/review-options.json
analysis/review-scope.json
chains/**
```

Where the host supports path-scoped write permission, generic Agent write/edit must be denied for Runtime-owned paths. Where the host cannot provide process-level/path-level isolation, Runtime consumers must still treat direct edits as untrusted and fail closed through certification/revalidation. 1.5.3 does not claim OS-security isolation between processes running as the same user.

## Task 1 — Changed Controller / EntryPoint Completeness Gate

### Objective

Eliminate silent missing Controller Chains before Review selection or no-target Chain discovery.

### Machine-generated entrypoint inventory

Controlled Runtime independently derives changed production Controller entrypoint obligations from the real Git Change Set and pinned Code Navigation.

For changed Controller files:

- added Controller file → all production endpoint methods are expected entrypoints;
- modified Controller file → endpoint methods whose ranges intersect changed hunks are expected;
- class-level changes outside a specific endpoint that may affect Controller behavior → all production endpoint methods in that Controller are expected;
- deleted Controller endpoint/file → record removal disposition; do not require a new Chain for a deleted entrypoint.

The Runtime must not infer Controller role from class-name suffix. It must use existing machine-verifiable annotation/source evidence.

Example Runtime artifact:

```json
{
  "runId": "r123",
  "status": "COMPLETE",
  "expectedEntryPoints": [
    {"symbol":"AController.create","path":"src/main/java/.../AController.java"},
    {"symbol":"BController.submit","path":"src/main/java/.../BController.java"},
    {"symbol":"CController.update","path":"src/main/java/.../CController.java"}
  ]
}
```

### Required disposition

Every expected entrypoint must have exactly one machine-recognized disposition:

```text
CONFIRMED
→ at least one verified callChain exists for the entrypoint

PARTIAL
→ explicit unresolved/limitation code exists for the entrypoint

REMOVED
→ source entrypoint was deleted by this Change Set
```

Forbidden state:

```text
SILENTLY_MISSING
```

If an expected entrypoint is absent from both confirmed callChains and explicit unresolved facts:

```text
ENTRYPOINT_COMPLETENESS_INCOMPLETE
```

and the no-target Review/Chain flow must stop before showing a Chain selection menu.

### Scope boundary

This gate guarantees changed Controller entrypoint completeness. It does not introduce a project-wide call graph or attempt to prove every possible dynamic branch in the entire application.

### Intent semantics

- `harness review` → full changed-Controller inventory required before Review mode selection;
- `harness review list` → full changed-Controller inventory required before displaying the list;
- `harness chain discover` without target → full changed-Controller inventory required;
- explicit `harness review <target>` / `harness chain discover <target>` → target-specific completeness is sufficient; unrelated changed Controllers do not block the explicitly targeted operation.

## Task 2 — Certified ChangeAnalysis

### Objective

Stop treating Agent-written `ChangeAnalysis` as an authoritative artifact.

### New flow

```text
Agent + Controlled Navigation
↓
requests/change-analysis-draft.json
↓
Controlled Runtime: analysis certify
↓
recompute exact Change Set
+ verify Controller entrypoint inventory
+ verify Schema
+ verify Coverage
+ verify declared symbol/path/role/resource evidence
↓
analysis/change-analysis.json
analysis/change-analysis.cert.json
analysis/entrypoint-inventory.json
↓
Review / Chain consumers
```

The Agent may write the draft. It may not directly create the authoritative `analysis/change-analysis.json`.

### Certification requirements

Runtime certification must verify at minimum:

1. exact Review Change Set matches Runtime-recomputed committed + staged + unstaged + untracked paths under Harness scope;
2. changed Controller entrypoint inventory is complete for the declared intent;
3. every expected entrypoint is CONFIRMED / PARTIAL / REMOVED;
4. each confirmed Controller entrypoint has exact current-workspace path/role evidence;
5. callChain entrypoints and nodes are consistent with declared verified navigation evidence;
6. workspace dependency evidence remains subject to 1.5.2 VERIFIED-only navigation rules;
7. resource relation exact-path/role/source invariants remain valid;
8. FULL/TARGETED machine Coverage remains valid;
9. no dependency workspace expands current Change Set or Review Scope.

Runtime certification must fail; it must not silently “repair” an incomplete Agent draft.

### Certification sidecar

The Runtime emits a sidecar containing deterministic identity data such as:

```text
runId
runtimeVersion
analysisSha256
changeSetSha256
entrypointInventorySha256
```

Consumers must use a shared `loadCertifiedAnalysis` path rather than opening arbitrary Agent-provided JSON directly.

The sidecar is a tamper-detection/provenance aid, not a cryptographic claim against a malicious same-user process. Authority ultimately comes from Runtime recomputation/revalidation plus host write isolation where available.

### Consumer migration

At minimum these flows must accept only Certified ChangeAnalysis:

```text
chain discover
chain validate
chain refresh
chain review-context
reviewscope.Verify / Review mode selection
report review transport construction
```

## Task 3 — Chain Artifact & Write Authority Hardening

### Objective

Make direct Agent modification of Runtime-owned Chain artifacts non-authoritative, and make every Project State write immutable-plan-based.

### Runtime-owned Chain artifacts

Authoritative Chain inputs may come only from:

```text
Runtime discovery
Runtime refresh
Runtime chain edit candidate
existing Project State Chain revalidated by Runtime
```

A Chain file merely existing under the right directory does not make it trusted.

### Shared candidate provenance

All discovered / refresh / edit candidates must be consumed through Runtime functions that verify:

```text
same run
expected artifact directory
schema/model validity
candidate hash
Certified ChangeAnalysis identity
current source validation
```

A candidate that changed after Runtime generated its preview must be rejected.

Suggested machine errors:

```text
CHAIN_ARTIFACT_NOT_RUNTIME_OWNED
CHAIN_CANDIDATE_HASH_MISMATCH
CHAIN_CANDIDATE_ANALYSIS_MISMATCH
CHAIN_CANDIDATE_VALIDATION_FAILED
```

### Immutable Chain Write Plan

Replace “candidate path + user confirmation” as the only persistence identity with an immutable Runtime-generated write plan.

Flow:

```text
Runtime candidate
↓
Runtime seal-persist
↓
chain-write-plan.json
  planId
  chainId
  candidateHash
  analysisHash
  expectedExistingHash
  deterministic preview
↓
show preview to user
↓
user explicitly confirms this plan
↓
chain persist(planId)
↓
re-read + re-hash + revalidate
↓
atomic Project State write
```

If any byte/fact changes after sealing, the old plan is invalid.

This reuses the core safety idea already used for Test/Fix immutable plan identity.

### User approval boundary

The Runtime can guarantee that an approval refers to exact bytes/facts, but it cannot cryptographically prove that a human typed the confirmation when the host Agent and Runtime execute as the same OS user. The Orchestrator remains responsible for interpreting the actual user turn. 1.5.3 therefore strengthens byte/fact authority and host path permissions, without making a false OS-level identity claim.

### Host write policy

Where supported, installation/init must configure generic Agent writes so that:

```text
ALLOW .code-harness/runs/<runId>/requests/**
DENY  .code-harness/runs/**/analysis/**
DENY  .code-harness/runs/**/review.md
DENY  .code-harness/chains/**
```

Project Adapter/Upgrade/Runtime-specific Framework Managed writes keep their existing controlled paths.

On hosts without enforceable scoped permissions, startup/doctor may report a non-blocking capability warning, while Runtime certification and candidate validation still prevent edited artifacts from becoming trusted facts.

## Task 4 — `harness review` Mode & Chain Selection UX

### Objective

Make plain `harness review` explicitly user-directed after complete Chain discovery, instead of silently defaulting to FULL.

### New no-target flow

```text
harness review
↓
real Change Set
↓
Agent draft analysis
↓
Runtime certification
↓
Changed Controller / EntryPoint Completeness Gate
↓
resolve Accepted VALID Chains + temporary discovered Chains
↓
Runtime review-options artifact
↓
user chooses Review mode
↓
Runtime verifies selection
↓
FULL or TARGETED Review
```

### Selection must happen after completeness

If changed Controller inventory is incomplete:

```text
3 expected entrypoints
1 confirmed
2 silently missing
```

Runtime returns `ENTRYPOINT_COMPLETENESS_INCOMPLETE` and the system must not show a misleading partial Chain selection list.

### User-visible menu

When at least one verified/temporary business Chain exists:

```text
本次变更：8 个文件
涉及 Controller 入口：3 个
已确认业务链：4 条
入口完整性：3/3

请选择本次评审范围：

1. 全部评审
   覆盖本次完整 Change Set

2. 按业务链评审
   选择一条或多条业务链

3. 仅查看调用链
   不执行代码评审
```

If option 2 is selected:

```text
[ ] OrderController.create
[ ] RefundController.refund
[ ] AdminController.retry
[ ] AdminController.cancel
```

Structured host multi-select is preferred; numbered fallback remains allowed.

### Runtime-generated ReviewOptions

The menu data must be Runtime-generated, not Agent-invented:

```json
{
  "runId":"r123",
  "changeSetDigest":"...",
  "entrypointCompleteness":"COMPLETE",
  "optionsHash":"...",
  "chains":[
    {
      "selectionId":"C1",
      "chainId":"...",
      "entryPoints":["OrderController.create"],
      "source":"ACCEPTED|TEMPORARY",
      "status":"VALID|TEMPORARY"
    }
  ]
}
```

Agent/Orchestrator submits only selected IDs + `optionsHash`; Runtime verifies the selection has not gone stale and derives the final `ReviewScopeSelection`.

Suggested machine errors:

```text
REVIEW_OPTIONS_STALE
REVIEW_SELECTION_UNKNOWN_CHAIN
REVIEW_SELECTION_SCOPE_INVALID
```

### One Chain

If exactly one Chain exists, still offer a lightweight choice:

```text
1. 全部评审
2. 仅评审该业务链
3. 仅查看调用链
```

FULL and single-Chain TARGETED may happen to include similar files, but their report semantics remain different.

### Zero Chain

If the complete inventory proves there are zero business Chains for the current Change Set, do not invent TARGETED options. Plain `harness review` keeps FULL Review available and clearly reports that there is no selectable business Chain.

### Existing explicit intents remain

```text
harness review <Controller>
harness review <Controller.method>
harness review <Service/downstream target>
harness review list
```

remain supported.

Explicit target Review does not show the initial FULL-vs-Chain menu. Existing Service/downstream multi-upstream selection is retained, but should reuse the same Runtime ReviewOptions/selection verification path.

## Task 5 — Human-friendly Chain Edit Skill

### Objective

Users should correct Chain business flow through natural language without manually editing Chain YAML fields.

### New intent

```text
harness chain edit <id|Controller|Controller.method>
```

Conversation continuation after `chain show` may also enter the same edit flow when the user clearly refers to the displayed Chain.

### User examples

```text
把 OldService.process 换成 NewService.process
```

```text
OrderServiceImpl.submit 后面应该先进入 AbstractTemplate.execute，
再回到 OrderServiceImpl.doExecute，然后到 OrderMapper.updateStatus
```

```text
删除 Redis 这一步
```

```text
把这条链名字改成“订单审核提交”
```

### Agent responsibility

The new `edit-chain` Skill may interpret natural language into a semantic edit proposal, but may not directly edit YAML.

Agent-produced semantic operations are limited in 1.5.3 to:

```text
REPLACE_NODE
ADD_NODE
REMOVE_NODE
REORDER_NODE
RENAME_CHAIN
UPDATE_NOTES
```

EntryPoint add/remove is intentionally deferred because it changes Chain identity/canonicalization semantics more deeply. Resource/boundary facts should normally be recomputed from verified source evidence rather than manually authored by the user.

### Runtime edit flow

```text
existing ACCEPTED Chain or current same-run candidate
+
user semantic edit request
+
fresh Certified ChangeAnalysis / targeted source validation
↓
Runtime chain edit
↓
validate proposed resulting facts
↓
analysis/chain-edit-candidates/<id>.yaml
+ deterministic human diff
↓
user reviews preview
↓
Runtime seal-persist
↓
explicit confirmation of exact plan
↓
Runtime persist
```

### Validation rule

Natural-language business knowledge may guide what to check, but saved code facts must still be machine-verifiable.

If the user requests a node/order that current source cannot prove:

```text
CHAIN_EDIT_FACT_NOT_VERIFIED
```

and no Project State write occurs.

Business context that cannot be encoded as a code fact may be stored in `notes`.

### Human preview

Users should see a readable diff instead of YAML:

```text
业务链修改预览

原：
OrderController.submit
↓
OldService.process
↓
OrderMapper.update

新：
OrderController.submit
↓
NewService.process
↓
OrderMapper.update

代码验证：
NewService.process   ✅
调用关系             ✅
Mapper relation      ✅

保存计划：CEDIT-7F21A3
```

The exact plan identity must bind the candidate hash, current Chain hash and Certified ChangeAnalysis hash.

## Task 6 — Real Project, Windows, Upgrade & Release Gate

Release version: `1.5.3`.

### Required real-project regressions

#### A. Three changed Controllers

Fixture contains:

```text
new AController
new BController
modified CController
```

Plain `harness review` must machine-prove `3/3` changed Controller entrypoint completeness before exposing Review mode selection.

Negative regression:

- draft intentionally omits B/C Controller facts;
- Runtime certification must fail with `ENTRYPOINT_COMPLETENESS_INCOMPLETE`;
- no review-options artifact;
- no Review execution.

#### B. Agent-tampered Chain artifact

Generate a real Runtime Chain candidate, then mutate its YAML outside Runtime before use.

Expected:

```text
CHAIN_CANDIDATE_HASH_MISMATCH / equivalent fail-closed error
0 Project State writes
Review does not consume the tampered Chain
```

#### C. Review mode selection

With multiple valid Chains:

- FULL choice → full current Change Set Coverage;
- one Chain → TARGETED verified scopedFiles only;
- multiple selected Chains → union derived by Runtime;
- LIST → no Finding Review;
- stale `optionsHash` → rejection.

#### D. Chain Edit

Real source + existing Chain:

- valid REPLACE_NODE → preview + sealed plan + explicit persist path succeeds;
- unsupported/unverified node → rejected;
- candidate modified after seal → old plan rejected;
- metadata-only RENAME/NOTES preserves verified code facts;
- dependency workspace remains READ_ONLY and cannot become Write Scope.

### Required CI

```text
go test ./...
go vet ./...
Windows x64 build
pinned real ast-grep smoke
Task 1 Controller completeness regression
Task 2 Certified ChangeAnalysis tamper regression
Task 3 Chain authority regression
Task 4 Review selection regression
Task 5 Chain edit regression
1.5.2 Workspace Dependency + Task 5 real business regression
1.5.2 Review Isolation regression
real 1.5.2 → 1.5.3 Windows live upgrade
```

### Upgrade

1.5.3 must preserve byte-for-byte Project State unless the user explicitly invokes a later Chain edit/save operation:

```text
harness.yaml
project.md
database.yaml
runs/**
chains/**
```

No automatic Chain rewrite or `harness.yaml` migration is required for this design. New contracts/skills/runtime behavior are Framework Managed.

Release artifacts:

```text
codea-harness-1.5.3-windows-x64-install.zip
codea-harness-1.5.3-windows-x64-upgrade.zip
```

## User-visible machine failure translation

Machine enums remain internal. User-facing summaries remain Chinese.

Examples:

```text
ENTRYPOINT_COMPLETENESS_INCOMPLETE
→ 本次变更的 Controller 入口识别不完整，暂不能选择调用链评审范围。

ANALYSIS_NOT_CERTIFIED / ANALYSIS_TAMPERED
→ 本次变更分析未通过运行时认证，需要重新分析。

CHAIN_CANDIDATE_HASH_MISMATCH
→ 调用链候选内容已变化，需要重新生成修改预览。

CHAIN_EDIT_FACT_NOT_VERIFIED
→ 你要求的调用链修改无法从当前代码中确认，未保存。

REVIEW_OPTIONS_STALE
→ 调用链列表已变化，请重新选择评审范围。
```

## Non-goals

1.5.3 does not add:

- project-wide Call Graph database;
- fuzzy Chain matching;
- automatic sibling workspace scanning;
- JAR decompilation;
- JDT language server;
- Chain integration into Test/Debug/Fix/Verify;
- arbitrary user-authored resource/boundary facts;
- OS-level process isolation or a privileged broker service;
- automatic repair of incomplete Agent analysis.

## Acceptance gates

```text
G1  Every changed production Controller entrypoint is CONFIRMED / PARTIAL / REMOVED; never silently missing
G2  Plain harness review does not show Chain choices until no-target entrypoint completeness is proven
G3  Authoritative ChangeAnalysis is Runtime certified, not Agent-written directly
G4  Runtime consumers reject uncertified/tampered analysis artifacts
G5  Runtime Chain candidates are hash/source/analysis bound and direct edits are non-authoritative
G6  Chain Project State persistence uses immutable sealed write plans + revalidation + expected hash
G7  Plain harness review supports FULL / selected Chain(s) / LIST after complete discovery
G8  Explicit targeted review keeps existing direct behavior and uses machine-verified selection
G9  Users can edit Chain semantics through edit-chain Skill without hand-editing YAML
G10 Unverified Chain edits fail closed with 0 Project State writes
G11 1.5.2 Workspace Navigation and Review/Write isolation do not regress
G12 Real Windows regressions + 1.5.2 → 1.5.3 live upgrade + formal release artifacts are green
```

## Fixed development order

```text
Task 1 Changed Controller / EntryPoint Completeness Gate
→ Task 2 Certified ChangeAnalysis
→ Task 3 Chain Artifact & Write Authority Hardening
→ Task 4 Review Mode & Chain Selection UX
→ Task 5 Human-friendly Chain Edit Skill
→ Task 6 Real Project / Windows / Upgrade / Release Gate
```

Task 4 must not be implemented before Tasks 1–3 are complete: the product must never present a user with a partial/untrusted Chain list and ask them to choose from it.

Task 5 must reuse Task 3 sealed persistence; it must not create a second Chain write path.