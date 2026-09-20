# Codea Harness 1.5 Chain Review Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a small, practical Business Chain capability for Codea Harness, persist accepted chains as Project State, and make FULL/TARGETED Review reuse verified chains without weakening existing coverage gates.

**Architecture:** 1.5 adds a Runtime-verified YAML Chain Contract, lazy discovery from production Controller methods, small chain-management intents, and Review integration. Framework files remain replaceable; `.code-harness/chains/**` becomes user-owned Project State and must survive upgrade byte-for-byte.

**Tech Stack:** Go 1.23, existing ast-grep navigation Runtime, JSON Schema Draft 2020-12, YAML v3, Markdown/Agent Skills, Windows x64 release workflow.

**Spec:** `docs/superpowers/specs/2026-08-23-codea-harness-1.5-chain-review-design.md`

## Global Constraints

- Baseline is accepted Codea Harness 1.4.0 commit `bedf2cde3784a6ee15d408271a023a95570c46b8` plus the 1.5 design/plan docs.
- Target version is `1.5.0` only in Task 5; Tasks 1-4 must not prematurely publish 1.5 release artifacts.
- 1.5 Chain EntryPoint is production Controller Method only.
- TestSource/Test Controller must never become a persisted Business Chain EntryPoint.
- Discovery is lazy: current Change Set or explicit target only; never pre-index the whole repository.
- V1/V2 automatic canonicalization is exact verified core-path equality only; no fuzzy similarity.
- No new navigation engine, no JDT LS/JavaParser/Spoon.
- No Test/Debug/Fix/Verify Chain integration in 1.5.
- No generic chain rule engine and no merge/split/edit/ignore user commands.
- Existing FULL/TARGETED Review Coverage gates from 1.4 must not be weakened.
- `.code-harness/chains/**` is Project State and Upgrade must preserve it byte-for-byte.
- Machine enums remain English; user-visible fixed text remains Chinese.
- Use TDD: failing test → minimal implementation → targeted tests → full tests → commit.

---

## File Structure

### New Framework files

```text
.code-harness/contracts/chain.schema.json
.code-harness/contracts/chain-validation-result.schema.json
.code-harness/templates/chain.template.yaml
.code-harness/skills/discover-chain/SKILL.md
.code-harness/skills/validate-chain/SKILL.md
.code-harness/tools-runtime/internal/chain/model.go
.code-harness/tools-runtime/internal/chain/store.go
.code-harness/tools-runtime/internal/chain/validate.go
.code-harness/tools-runtime/internal/chain/discover.go
.code-harness/tools-runtime/internal/chain/canonicalize.go
.code-harness/tools-runtime/internal/chain/chain_test.go
```

Do not add a new Chain Agent unless implementation proves Reviewer/Orchestrator cannot express the flow cleanly. Default is to reuse existing orchestration.

### Existing files likely modified

```text
.code-harness/agents/orchestrator.md
.code-harness/agents/reviewer.md
.code-harness/skills/analyze-change/SKILL.md
.code-harness/skills/review-code/SKILL.md
.code-harness/tools-runtime/cmd/codea-harness-tools/main.go
.code-harness/tools-runtime/internal/report/review.go
.code-harness/tools-runtime/internal/report/review_test.go
.code-harness/tools-runtime/internal/upgrade/upgrade.go
.code-harness/tools-runtime/internal/upgrade/upgrade_15_test.go
.code-harness/upgrade.md
.github/workflows/package-windows-x64.yml
README.md
CHANGELOG.md
.code-harness/VERSION
```

### User-owned Project State

```text
.code-harness/chains/*.yaml
```

Never ship business Chain instances in install/upgrade ZIPs.

---

# Task 1 — Chain Contract & Project State Boundary

**Goal:** Lock the stable YAML format, Runtime data model, schema validation, Project State ownership, and upgrade managed/unmanaged boundary before any discovery logic exists.

**Files:**
- Create: `.code-harness/contracts/chain.schema.json`
- Create: `.code-harness/contracts/chain-validation-result.schema.json`
- Create: `.code-harness/templates/chain.template.yaml`
- Create: `.code-harness/tools-runtime/internal/chain/model.go`
- Create: `.code-harness/tools-runtime/internal/chain/store.go`
- Create/Test: `.code-harness/tools-runtime/internal/chain/chain_test.go`
- Modify/Test: `.code-harness/tools-runtime/internal/upgrade/upgrade.go`
- Create/Test: `.code-harness/tools-runtime/internal/upgrade/upgrade_15_test.go`

**Interfaces:**

```go
type Status string
const (
    StatusDiscovered Status = "DISCOVERED"
    StatusAccepted   Status = "ACCEPTED"
    StatusStale      Status = "STALE"
)

type EntryPoint struct {
    Symbol string `json:"symbol" yaml:"symbol"`
    Path   string `json:"path" yaml:"path"`
}

type Node struct {
    Symbol string `json:"symbol" yaml:"symbol"`
    Path   string `json:"path" yaml:"path"`
    Role   string `json:"role" yaml:"role"`
}

type Resource struct {
    Path   string `json:"path" yaml:"path"`
    Symbol string `json:"symbol,omitempty" yaml:"symbol,omitempty"`
    Role   string `json:"role" yaml:"role"`
}

type Boundary struct {
    Symbol string `json:"symbol" yaml:"symbol"`
    Path   string `json:"path" yaml:"path"`
    Role   string `json:"role" yaml:"role"`
}

type Chain struct {
    Version     int          `json:"version" yaml:"version"`
    ID          string       `json:"id" yaml:"id"`
    Name        string       `json:"name" yaml:"name"`
    Status      Status       `json:"status" yaml:"status"`
    EntryPoints []EntryPoint `json:"entryPoints" yaml:"entryPoints"`
    Nodes       []Node       `json:"nodes" yaml:"nodes"`
    Resources   []Resource   `json:"resources,omitempty" yaml:"resources,omitempty"`
    Boundaries  []Boundary   `json:"boundaries,omitempty" yaml:"boundaries,omitempty"`
    Notes       string       `json:"notes,omitempty" yaml:"notes,omitempty"`
}
```

- [ ] **Step 1: Write schema/model failing tests**

Tests must reject:

```text
version != 1
empty/invalid id
empty name
unknown status
empty entryPoints
unknown node/resource/boundary role
additional properties
```

Tests must accept the exact sample from the design spec.

- [ ] **Step 2: Run targeted tests and confirm failure**

Run:

```text
cd .code-harness/tools-runtime
go test -count=1 ./internal/chain
```

Expected: FAIL because model/schema/store do not exist.

- [ ] **Step 3: Implement model + strict schema + template**

Roles for v1:

```text
Node: SERVICE | REPOSITORY | MAPPER | OTHER
Resource: MAPPER_XML | YAML_CONFIG
Boundary: EXTERNAL | CACHE | MQ
```

Template must contain the user-editing comments from the spec.

- [ ] **Step 4: Implement deterministic Chain file storage primitives**

Required functions:

```go
func Load(path string) (Chain, error)
func MarshalYAML(c Chain) ([]byte, error)
func ChainPath(root, id string) (string, error)
func ValidateID(id string) error
```

`ChainPath` must prevent traversal and normalize to `.code-harness/chains/<id>.yaml` only.

- [ ] **Step 5: Add Project State boundary tests before changing upgrade**

Construct target containing:

```text
.code-harness/chains/order-approve.yaml
```

with sentinel bytes. Assert `listManagedFiles`, remove/replace semantics and upgrade never classify it as Framework Managed.

- [ ] **Step 6: Modify Upgrade Project State boundary**

`chains/**` must be excluded from Framework Managed replacement and included in `Result.PreservedFiles` as `chains/**`.

Do not add a 1.5 config migration.

- [ ] **Step 7: Run targeted and full tests**

```text
cd .code-harness/tools-runtime
go test -count=1 ./internal/chain ./internal/upgrade
go test -count=1 ./...
go vet ./...
```

Expected: PASS.

- [ ] **Step 8: Commit Task 1**

Suggested message:

```text
feat: add business chain contract and project state
```

### Task 1 Acceptance Gate

```text
PASS only if:
- exact Chain YAML schema exists and is strict;
- template is developer-readable;
- traversal/duplicate-id primitives are safe;
- chains/** is proven non-managed Project State;
- no discovery/review behavior has been introduced yet;
- full Go tests/vet pass.
```

---

# Task 2 — Lazy Discovery & Exact Canonicalization

**Goal:** Discover only relevant production Controller-method chains using existing navigation evidence, emit DISCOVERED YAML into the current Run, and exactly merge V1/V2 entries only when verified core paths are identical.

**Files:**
- Create: `.code-harness/tools-runtime/internal/chain/discover.go`
- Create: `.code-harness/tools-runtime/internal/chain/canonicalize.go`
- Modify/Test: `.code-harness/tools-runtime/internal/chain/chain_test.go`
- Create: `.code-harness/skills/discover-chain/SKILL.md`
- Modify: `.code-harness/skills/analyze-change/SKILL.md`
- Modify: `.code-harness/agents/reviewer.md`
- Modify: `.code-harness/tools-runtime/cmd/codea-harness-tools/main.go`

**Interfaces:**

```go
type DiscoverInput struct {
    RunID          string
    Target         string // empty | Class | Class.method
    ChangeAnalysis ChangeAnalysisEvidence
}

type DiscoveryResult struct {
    Status      string // COMPLETE | PARTIAL
    Chains      []Chain
    Unresolved  []string
}

func Discover(root string, in DiscoverInput) (DiscoveryResult, error)
func Canonicalize(chains []Chain) []Chain
```

Use existing verified navigation/evidence structures; do not build a second Java parser.

- [ ] **Step 1: Write candidate EntryPoint tests**

Cover:

```text
production @RestController method -> candidate
src/test controller -> excluded
TestSource role -> excluded
Service-only target -> may resolve upward to production Controller method
ambiguous controller symbol -> PARTIAL
```

- [ ] **Step 2: Write lazy-scope tests**

Given unrelated controllers in repo, discovery for `OrderController.approve` must not emit chains for User/Payment controllers.

No-target discovery must only emit EntryPoints affected by current Change Set.

- [ ] **Step 3: Write call-path tests**

Expected verified path:

```text
Controller.method
→ Service
→ ServiceImpl
→ Mapper
→ Mapper.xml
```

Internal unresolved symbol must produce `PARTIAL`, never an ACCEPTED-looking chain.

- [ ] **Step 4: Write exact V1/V2 canonicalization tests**

Case A:

```text
V1 entry -> Service -> Impl -> Mapper
V2 entry -> Service -> Impl -> Mapper
```

Expected: 1 Chain, 2 entryPoints.

Case B:

```text
V1 -> Service -> Impl -> Mapper
V2 -> V2Service -> RiskService -> Mapper
```

Expected: 2 Chains.

Case C: same class naming only, different verified paths -> 2 Chains.

No fuzzy threshold is allowed.

- [ ] **Step 5: Implement minimal discovery on existing evidence**

Rules:

```text
- EntryPoint exact path must come from navigation evidence.
- Core nodes preserve verified order.
- role comes from evidence; unknown -> OTHER.
- Mapper.xml/YML enter only through existing verified resourceRelations.
- stop at existing external boundaries.
```

- [ ] **Step 6: Persist only DISCOVERED run artifacts**

Output directory:

```text
.code-harness/runs/<runId>/analysis/discovered-chains/<id>.yaml
```

Never write `.code-harness/chains/**` in discovery.

- [ ] **Step 7: Add Runtime command plumbing**

Recommended machine command shape:

```text
codea-harness-tools chain discover --input <controlled request json>
```

Do not expose raw ast-grep patterns or arbitrary output paths.

- [ ] **Step 8: Update discover-chain Skill/Reviewer orchestration**

Document user intents:

```text
harness chain discover
harness chain discover OrderController
harness chain discover OrderController.approve
```

Machine facts remain Runtime-verified; Agent only coordinates and explains.

- [ ] **Step 9: Run tests**

```text
cd .code-harness/tools-runtime
go test -count=1 ./internal/chain ./internal/nav ./internal/reviewscope
go test -count=1 ./...
go vet ./...
```

- [ ] **Step 10: Commit Task 2**

Suggested message:

```text
feat: add lazy business chain discovery
```

### Task 2 Acceptance Gate

```text
PASS only if:
- production Controller Method is the only persisted EntryPoint type;
- test/demo source cannot become a chain entry by role/path trick;
- discovery is change/target bounded;
- unresolved internal navigation is PARTIAL;
- V1/V2 merge is exact verified core-path equality only;
- discovered chains stay under runs/**;
- full tests/vet pass.
```

---

# Task 3 — Chain Management, Validation, Refresh & Persistence

**Goal:** Make Chain usable by developers: list/show/discover/refresh/validate, allow direct YAML edits, safely accept a discovered Chain into Project State, and detect stale saved chains.

**Files:**
- Create: `.code-harness/tools-runtime/internal/chain/validate.go`
- Extend: `.code-harness/tools-runtime/internal/chain/store.go`
- Extend/Test: `.code-harness/tools-runtime/internal/chain/chain_test.go`
- Create: `.code-harness/skills/validate-chain/SKILL.md`
- Modify: `.code-harness/agents/orchestrator.md`
- Modify: `.code-harness/tools-runtime/cmd/codea-harness-tools/main.go`

**Interfaces:**

```go
type ValidationResult struct {
    ChainID  string   `json:"chainId"`
    Status   string   `json:"status"` // VALID | STALE | INVALID
    Errors   []string `json:"errors"`
    Warnings []string `json:"warnings"`
}

func Validate(root string, c Chain, evidence EvidenceProvider) ValidationResult
func SaveAccepted(root string, c Chain, expectedExistingHash string) error
func Refresh(root string, existing Chain, discovered Chain) RefreshResult
```

`SaveAccepted` must refuse silent overwrite. Existing Chain replacement requires an explicit expected hash/version from the user-confirmed refresh/update flow.

- [ ] **Step 1: Write validation tests**

Reject/STALE cases:

```text
entryPoint missing
entryPoint in wrong exact path
entryPoint is TestSource
node missing
node path mismatch
node order/call relation invalid
Mapper.xml missing
resource role/path mismatch
resource relation absent
boundary symbol missing
id != filename
duplicate project chain id
```

- [ ] **Step 2: Write notes isolation test**

Changing `notes` must never affect code-fact validation outcome.

- [ ] **Step 3: Implement `Validate` using existing navigation/resource evidence**

Do not trust YAML because status says ACCEPTED.

Result semantics:

```text
VALID   -> all core facts still true
STALE   -> previously meaningful chain no longer matches current source
INVALID -> malformed/unsafe/contradictory chain
```

- [ ] **Step 4: Write list/show tests**

User-visible result must be Chinese and stable.

`show` must display entryPoints, verified role labels, nodes, resources, boundaries, notes, and status.

No class-name suffix guessing for roles.

- [ ] **Step 5: Write safe persistence tests**

Cover:

```text
DISCOVERED -> explicit user accept -> validate -> ACCEPTED file
validation failure -> 0 Project State writes
same id exists + no explicit update token/hash -> reject
explicit refresh with matching existing hash -> atomic replace
```

- [ ] **Step 6: Implement chain intents**

User intents:

```text
harness chain list
harness chain show <id|target>
harness chain discover [target]
harness chain refresh <id>
harness chain validate [id]
```

Do not add merge/split/edit/ignore commands.

- [ ] **Step 7: Implement refresh as diff-first flow**

Runtime produces old/new facts; Orchestrator shows deterministic change summary.

No Project State overwrite before explicit user confirmation.

After confirmation: validate candidate, then atomically replace.

- [ ] **Step 8: Add stale-chain tests**

If saved Chain references renamed/removed Service, `validate` returns STALE and Review-facing consumer can distinguish it from INVALID.

- [ ] **Step 9: Run tests**

```text
cd .code-harness/tools-runtime
go test -count=1 ./internal/chain
go test -count=1 ./...
go vet ./...
```

- [ ] **Step 10: Commit Task 3**

Suggested message:

```text
feat: add business chain management
```

### Task 3 Acceptance Gate

```text
PASS only if:
- developers can understand/edit one-chain-per-YAML;
- validate proves code facts rather than trusting YAML;
- stale saved chains are detectable;
- user acceptance is required before runs/** becomes chains/**;
- overwrite/update is atomic and explicit;
- no rule engine is added;
- full tests/vet pass.
```

---

# Task 4 — Review Consumes Verified Chains

**Goal:** Make 1.4 FULL/TARGETED Review consume valid accepted chains first, lazy-discover when missing, safely handle stale chains, and expose chain provenance in review.md without changing coverage semantics.

**Files:**
- Modify: `.code-harness/agents/orchestrator.md`
- Modify: `.code-harness/agents/reviewer.md`
- Modify: `.code-harness/skills/analyze-change/SKILL.md`
- Modify: `.code-harness/skills/review-code/SKILL.md`
- Modify: `.code-harness/tools-runtime/internal/reviewscope/reviewscope.go`
- Modify/Test: `.code-harness/tools-runtime/internal/reviewscope/reviewscope_test.go`
- Modify: `.code-harness/tools-runtime/internal/report/review.go`
- Modify/Test: `.code-harness/tools-runtime/internal/report/review_test.go`

**Interfaces:**

Add a small Review transport field; do not redesign all Review contracts:

```json
{
  "chainContext": {
    "id": "order-approve",
    "name": "订单审批",
    "source": "ACCEPTED | DISCOVERED",
    "status": "VALID | TEMPORARY"
  }
}
```

Runtime scope still ultimately validates exact scoped files/symbols using existing 1.4 evidence.

- [ ] **Step 1: Write accepted-chain lookup tests**

TARGETED:

```text
accepted valid matching chain -> reuse
no accepted chain -> lazy discover
accepted stale chain -> must not silently reuse
```

FULL:

```text
accepted chains may provide context
but every changed file still participates in FULL coverage
```

- [ ] **Step 2: Write stale fallback tests**

For stale chain, valid options are:

```text
use current temporary rediscovered chain for this review
refresh persisted chain
stop
```

No automatic Project State mutation.

- [ ] **Step 3: Write coverage regression tests first**

Lock:

```text
FULL remains changedFiles subset reviewedFiles
TARGETED remains scopedFiles subset reviewedFiles
chain presence never changes COMPLETE rules
scope-out Finding still rejected
```

- [ ] **Step 4: Integrate chain lookup/discovery with Review flow**

Reuse existing `reviewscope` exact-path evidence; do not let Chain YAML bypass `symbolLocations/resourceRelations` verification.

- [ ] **Step 5: Update review.md Renderer**

Top summary adds:

```text
业务链：订单审批
Chain ID：order-approve
Chain 来源：项目已确认 / 本次临时发现
```

Temporary chain warning:

```text
⚠️ 本次评审使用临时发现的业务链，尚未沉淀到项目 Chain。
```

TARGETED disclaimer from 1.4 must remain.

- [ ] **Step 6: Add post-review persistence suggestion**

If Review used a new DISCOVERED Chain, Orchestrator may ask whether user wants to save it. It must not auto-save.

- [ ] **Step 7: Regression test Task 1-3 / 1.4 semantics**

At minimum:

```text
Targeted controller all-chain semantics
resource exact evidence
TEST_VALIDITY scope
Chinese UX
deterministic renderer
Runtime role evidence
```

- [ ] **Step 8: Run tests**

```text
cd .code-harness/tools-runtime
go test -count=1 ./internal/chain ./internal/reviewscope ./internal/coverage ./internal/report
go test -count=1 ./...
go vet ./...
```

- [ ] **Step 9: Commit Task 4**

Suggested message:

```text
feat: make review consume verified business chains
```

### Task 4 Acceptance Gate

```text
PASS only if:
- accepted valid chains are reused;
- missing chains are lazily discovered;
- stale chains never silently pass;
- users can review using temporary chain without forced persistence;
- chain never weakens 1.4 Coverage/Scope gates;
- review.md shows chain provenance clearly;
- full tests/vet pass.
```

---

# Task 5 — 1.5 Release / Upgrade / Windows Gate

**Goal:** Publish 1.5.0 as a Windows x64 accepted release and prove 1.4→1.5 upgrade preserves user chains byte-for-byte while installing all new Framework capabilities.

**Files:**
- Modify: `.code-harness/VERSION`
- Modify: `README.md`
- Modify: `CHANGELOG.md`
- Modify: `.code-harness/upgrade.md`
- Modify: `.github/workflows/package-windows-x64.yml`
- Create/Extend: `.code-harness/tools-runtime/internal/upgrade/upgrade_15_test.go`

- [ ] **Step 1: Write 1.4→1.5 upgrade tests before VERSION bump**

Build accepted-1.4-like target with:

```text
harness.yaml
project.md
database.yaml
runs/**
chains/order-approve.yaml
```

Record exact bytes/hash of all Project State.

- [ ] **Step 2: Assert Framework install and Project State preservation**

After upgrade expect:

```text
contracts/chain.schema.json exists
chain validation contract exists
templates/chain.template.yaml exists
chain runtime/skills exist
chains/order-approve.yaml byte-for-byte unchanged
project.md/database.yaml/runs unchanged
no business chains introduced by release package
```

- [ ] **Step 3: Prove managed replace does not delete `chains/**`**

Create stale Framework file plus user chain. Expected:

```text
stale Framework removed
user chain preserved and absent from removedFiles
```

- [ ] **Step 4: Bump version and document only delivered scope**

```text
.code-harness/VERSION = 1.5.0
```

README/CHANGELOG must explicitly say 1.5 is Chain-for-Review only and does not yet chain-enable Test/Debug/Fix.

- [ ] **Step 5: Extend release package completeness**

Both install and upgrade ZIP require:

```text
contracts/chain.schema.json
contracts/chain-validation-result.schema.json
templates/chain.template.yaml
chain skills/runtime
```

Both ZIPs must reject/leak no:

```text
chains/*.yaml
harness.yaml
project.md
database.yaml
runs/**
```

- [ ] **Step 6: Add real Windows 1.4→1.5 live upgrade smoke**

Use accepted 1.4 baseline `bedf2cde3784a6ee15d408271a023a95570c46b8`.

Before upgrade create sentinel chain and compute SHA256.

After upgrade assert:

```text
VERSION=1.5.0
chain SHA unchanged
other Project State unchanged
new chain framework installed
stale Framework removed
source/stage/backup cleaned
installed runtime hash == release runtime hash
```

- [ ] **Step 7: Probe installed `chain validate` capability**

After live upgrade invoke installed runtime in a deterministic failure/success mode proving the `chain` subcommand exists and comes from the accepted release runtime.

- [ ] **Step 8: Run exact release gate**

Workflow must include:

```text
go test -count=1 ./internal/chain ./internal/reviewscope ./internal/coverage ./internal/report
go test -count=1 ./internal/apply ./internal/schema ./internal/upgrade
go test -count=1 ./...
go vet ./...
Windows x64 build
pinned ast-grep smoke
formal dual ZIP layout
real 1.4 -> 1.5 live upgrade
artifact upload
```

- [ ] **Step 9: Verify exact-head artifacts**

Required:

```text
codea-harness-1.5.0-windows-x64-install
codea-harness-1.5.0-windows-x64-upgrade
```

Record Run ID, exact head SHA, artifact IDs and SHA256 digests for manual acceptance.

- [ ] **Step 10: Commit Task 5**

Suggested message:

```text
feat: release Codea Harness 1.5.0
```

### Task 5 Acceptance Gate

```text
PASS only if:
- exact-head package-windows-x64 is completed/success;
- real accepted 1.4 -> 1.5 live upgrade succeeds;
- user chains/** is byte-for-byte preserved;
- install/upgrade ZIPs contain Chain Framework but no business Chain instances;
- exact-head formal artifacts exist;
- README/CHANGELOG do not claim Test/Debug/Fix Chain support.
```

---

# Final 1.5 Acceptance Checklist

```text
Chain YAML Contract                         PASS
Developer-readable one-chain-per-file      PASS
Production Controller Method entry only    PASS
TestSource exclusion                       PASS
Lazy discovery                             PASS
Exact V1/V2 canonicalization               PASS
DISCOVERED run state                       PASS
ACCEPTED Project State                     PASS
STALE detection                            PASS
Direct YAML edit + Runtime validation      PASS
Safe refresh/persistence                   PASS
Review accepted-chain reuse                PASS
Review lazy fallback                       PASS
Coverage semantics unchanged               PASS
Chain provenance in review.md              PASS
chains/** upgrade preservation             PASS
1.4 -> 1.5 Windows live upgrade            PASS
Formal install/upgrade artifacts           PASS
No non-goal expansion                      PASS
```
