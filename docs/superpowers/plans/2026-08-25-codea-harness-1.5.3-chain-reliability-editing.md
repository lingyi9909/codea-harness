# Codea Harness 1.5.3 — Chain Reliability, Review Selection & Editing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Eliminate silent changed-Controller Chain omissions, make Runtime-certified artifacts the only authoritative Review/Chain facts, add safe Review scope selection with single-option auto execution, and add natural-language Chain editing without direct YAML writes.

**Architecture:** Agent output becomes proposal-only data under `runs/<runId>/requests/**`. Controlled Runtime independently recomputes the Change Set and Controller entrypoint obligations, certifies `ChangeAnalysis`, produces/validates Review options and Chain candidates, and seals immutable Chain write plans before persistence. Existing 1.5.2 Chain/Workspace/Review isolation remains intact; new selection/edit flows reuse the same Certified ChangeAnalysis and sealed persistence boundary.

**Tech Stack:** Go 1.26.5 runtime, JSON Schema Draft 2020-12, YAML v3, pinned ast-grep 0.42.1, Git local CLI through fixed non-shell arguments, PowerShell Windows acceptance scripts, GitHub Actions Windows runners.

**Spec:**
- `docs/superpowers/specs/2026-08-25-codea-harness-1.5.3-chain-reliability-editing-design.md`
- `docs/superpowers/specs/2026-08-25-codea-harness-1.5.3-selection-amendment.md`

## Global Constraints

- Exact release baseline is `6f290d8ff160767bb981278aa123aa1621ea3343` (Codea Harness 1.5.2).
- Platform remains Windows 10/11 x64; no WSL dependency.
- Runtime must work fully offline; no `git fetch`, Maven/Nexus/Central lookup, source download, or Internet dependency.
- Pinned Code Navigation remains ast-grep `0.42.1`; do not add regex Java parsing or a second Java parser.
- Review Change Set semantics remain: merge-base(baseRef, HEAD) → HEAD committed + staged + unstaged + untracked.
- Workspace Dependency remains Navigation/Chain context only; it must never expand Change Set, Review Scope, Finding Scope, Test/Fix/Apply Write Scope.
- Existing Chain YAML remains `version: 1`; no Chain Project State migration.
- Existing `harness.yaml` is not forcibly migrated for 1.5.3.
- Agent/Orchestrator may propose data only under `.code-harness/runs/<runId>/requests/**`; Runtime-owned authoritative artifacts live under `.code-harness/runs/<runId>/analysis/**`, `.code-harness/runs/<runId>/review.md`, and `.code-harness/chains/**`.
- `Agent proposes → Runtime certifies`; Agent-created or Agent-modified Runtime artifacts are never authoritative merely because the path/schema looks correct.
- When a Controller/Chain decision has exactly one valid selectable option, do **not** ask the user; Runtime emits `AUTO_SINGLE` and execution continues directly. User selection appears only for 2+ valid options.
- No Chain integration into Test/Debug/Fix/Verify is added in 1.5.3.
- No project-wide call graph, fuzzy Chain matching, rules engine, JDT LS, JAR decompilation, arbitrary sibling scanning, or new network resolution.
- Every Task starts RED, reaches GREEN, runs its exact gate plus `go test ./...`/`go vet ./...` when required, and ends in a reviewable commit.

---

## File Structure / Ownership Map

New focused runtime packages:

```text
.code-harness/tools-runtime/internal/changeset/
  git.go                fixed-argument local Git Change Set computation
  model.go              Snapshot/File/Hunk canonical model + digest
  git_test.go           committed/staged/unstaged/untracked regressions

.code-harness/tools-runtime/internal/analysis/
  model.go              canonical ChangeAnalysis + Intent types shared by certification/consumers
  entrypoints.go        changed production Controller endpoint inventory
  certify.go            proposal → Certified ChangeAnalysis gate
  evidence.go           symbol/path/role/resource evidence validation
  certificate.go        deterministic hashes + certified loader
  entrypoints_test.go
  certify_test.go
  tamper_test.go

.code-harness/tools-runtime/internal/reviewselection/
  options.go            Runtime ReviewOptions generation
  verify.go             AUTO_FULL/AUTO_SINGLE/USER_SELECTION verification
  options_test.go
  verify_test.go

.code-harness/tools-runtime/internal/chain/
  candidate_provenance.go   candidate identity/hash/analysis binding
  write_plan.go             immutable Chain persistence plan
  edit.go                   semantic Chain edit operations + validation
  *_test.go                 provenance, seal, edit regressions
```

New/updated Controlled Runtime command files:

```text
.code-harness/tools-runtime/cmd/codea-harness-tools/
  analysis_command.go       `analysis inventory|certify`
  review_command.go         `review options|select`
  chain_command.go          add `seal-persist` and `edit`; persist by planId
  main.go                   register `analysis` and `review`
```

New contracts:

```text
.code-harness/contracts/entrypoint-inventory.schema.json
.code-harness/contracts/change-analysis-cert.schema.json
.code-harness/contracts/review-options.schema.json
.code-harness/contracts/review-selection-request.schema.json
.code-harness/contracts/chain-candidate-cert.schema.json
.code-harness/contracts/chain-write-plan.schema.json
.code-harness/contracts/chain-edit-request.schema.json
```

Updated Agent/Skill contracts:

```text
.code-harness/AGENTS.md
.code-harness/agents/orchestrator.md
.code-harness/agents/reviewer.md
.code-harness/skills/analyze-change/SKILL.md
.code-harness/skills/discover-chain/SKILL.md
.code-harness/skills/validate-chain/SKILL.md
.code-harness/skills/edit-chain/SKILL.md          (new)
```

Release/acceptance:

```text
.github/workflows/task153-chain-reliability.yml   (new)
.github/workflows/package-windows-x64.yml         (extend)
.code-harness/tools-runtime/cmd/codea-harness-tools/task153_*_test.go
.github/scripts/task153-real-review-chain-regression.ps1
.code-harness/VERSION
CHANGELOG.md
```

Do not restructure unrelated 1.5.2 packages while implementing these files.

---

### Task 1: Changed Controller / EntryPoint Completeness Gate

**Files:**
- Create: `.code-harness/contracts/entrypoint-inventory.schema.json`
- Create: `.code-harness/tools-runtime/internal/changeset/model.go`
- Create: `.code-harness/tools-runtime/internal/changeset/git.go`
- Create: `.code-harness/tools-runtime/internal/changeset/git_test.go`
- Create: `.code-harness/tools-runtime/internal/analysis/model.go`
- Create: `.code-harness/tools-runtime/internal/analysis/entrypoints.go`
- Create: `.code-harness/tools-runtime/internal/analysis/entrypoints_test.go`
- Create: `.code-harness/tools-runtime/cmd/codea-harness-tools/analysis_command.go`
- Create: `.code-harness/tools-runtime/cmd/codea-harness-tools/analysis_inventory_test.go`
- Modify: `.code-harness/tools-runtime/cmd/codea-harness-tools/main.go`
- Modify only if needed for exact Controller endpoint evidence: `.code-harness/tools-runtime/internal/nav/*`

**Interfaces:**
- Produces:

```go
package changeset

type Source string
const (
    SourceCommitted Source = "COMMITTED"
    SourceStaged    Source = "STAGED"
    SourceUnstaged  Source = "UNSTAGED"
    SourceUntracked Source = "UNTRACKED"
)

type Hunk struct {
    OldStart int `json:"oldStart"`
    OldLines int `json:"oldLines"`
    NewStart int `json:"newStart"`
    NewLines int `json:"newLines"`
}

type File struct {
    Path    string   `json:"path"`
    Status  string   `json:"status"`
    Sources []Source `json:"sources"`
    Hunks   []Hunk   `json:"hunks"`
}

type Snapshot struct {
    BaseRef string `json:"baseRef"`
    Head    string `json:"head"`
    Files   []File `json:"files"`
    SHA256  string `json:"sha256"`
}

func Compute(repoRoot, baseRef string, includeWorkingTree bool) (Snapshot, error)
```

```go
package analysis

type Intent struct {
    Mode   string `json:"mode"`   // FULL | LIST | TARGETED | CHAIN_MAINTENANCE
    Target string `json:"target,omitempty"`
}

type ChangedFile struct {
    Path string `json:"path"`
    Role string `json:"role"`
}

type AffectedController struct {
    Controller    string   `json:"controller"`
    Endpoints     []string `json:"endpoints"`
    ImpactType    string   `json:"impactType"`
    SourceSymbols []string `json:"sourceSymbols"`
}

type CallChain struct {
    EntryPoint string   `json:"entryPoint"`
    Chain      []string `json:"chain"`
}

type SymbolLocation struct {
    Workspace string `json:"workspace,omitempty"`
    Symbol    string `json:"symbol"`
    Path      string `json:"path"`
    Role      string `json:"role"`
    Source    string `json:"source"`
    From      string `json:"from,omitempty"`
}

type ResourceRelation struct {
    Path       string `json:"path"`
    Role       string `json:"role"`
    Resource   string `json:"resource"`
    FromSymbol string `json:"fromSymbol"`
    FromKind   string `json:"fromKind"`
    Source     string `json:"source"`
    Evidence   string `json:"evidence"`
}

type UnresolvedSymbol struct {
    Symbol string `json:"symbol"`
    From   string `json:"from"`
    Reason string `json:"reason"`
}

type ReviewCoverage struct {
    Status            string             `json:"status"`
    ReviewedFiles     []ChangedFile      `json:"reviewedFiles"`
    UnresolvedSymbols []UnresolvedSymbol `json:"unresolvedSymbols"`
}

type ChangeAnalysis struct {
    ChangedFiles         []ChangedFile         `json:"changedFiles"`
    AffectedControllers  []AffectedController  `json:"affectedControllers"`
    CallChains           []CallChain           `json:"callChains"`
    SymbolLocations      []SymbolLocation      `json:"symbolLocations"`
    ResourceRelations    []ResourceRelation    `json:"resourceRelations"`
    ExternalDependencies []string              `json:"externalDependencies"`
    ReviewCoverage       ReviewCoverage        `json:"reviewCoverage"`
}

type EntrypointDisposition string
const (
    DispositionConfirmed EntrypointDisposition = "CONFIRMED"
    DispositionPartial   EntrypointDisposition = "PARTIAL"
    DispositionRemoved   EntrypointDisposition = "REMOVED"
)

type ExpectedEntrypoint struct {
    Symbol      string                `json:"symbol"`
    Path        string                `json:"path"`
    Disposition EntrypointDisposition `json:"disposition,omitempty"`
    Limitation  string                `json:"limitation,omitempty"`
}

type EntrypointInventory struct {
    RunID               string               `json:"runId"`
    Status              string               `json:"status"`
    ExpectedEntrypoints []ExpectedEntrypoint `json:"expectedEntryPoints"`
    ChangeSetSHA256     string               `json:"changeSetSha256"`
}

func BuildEntrypointInventory(repoRoot, runID string, snapshot changeset.Snapshot, intent Intent) (EntrypointInventory, error)
func VerifyEntrypointDispositions(inventory EntrypointInventory, proposal ChangeAnalysis) error
```

- Later Tasks consume the exact `Snapshot.SHA256`, `analysis.ChangeAnalysis`, and `EntrypointInventory` types; do not rename them after Task 1 acceptance.

- [ ] **Step 1: Write failing Change Set tests for all four Git sources**

Create a temp Git repository and assert canonical union semantics:

```go
func Test153ComputeChangeSetIncludesCommittedStagedUnstagedUntracked(t *testing.T) {
    snap, err := Compute(repo, "HEAD~1", true)
    if err != nil { t.Fatal(err) }
    assertSources(t, snap, "src/main/java/acme/AController.java", SourceCommitted)
    assertSources(t, snap, "src/main/java/acme/BController.java", SourceStaged)
    assertSources(t, snap, "src/main/java/acme/CController.java", SourceUnstaged)
    assertSources(t, snap, "src/main/java/acme/DController.java", SourceUntracked)
    if snap.SHA256 == "" { t.Fatal("missing deterministic Change Set digest") }
}
```

Also cover duplicate path source merging and deterministic file/source ordering.

- [ ] **Step 2: Run the Change Set tests and verify RED**

Run from `.code-harness/tools-runtime`:

```powershell
go test -count=1 ./internal/changeset -run Test153
```

Expected: FAIL because `changeset.Compute` does not exist.

- [ ] **Step 3: Implement fixed-argument local Git Change Set computation**

Use `exec.CommandContext` directly; never shell-evaluate strings. Fixed operations are:

```text
git merge-base <baseRef> HEAD
git diff --unified=0 --no-ext-diff <mergeBase>..HEAD -- <Harness scoped paths>
git diff --cached --unified=0 --no-ext-diff -- <Harness scoped paths>
git diff --unified=0 --no-ext-diff -- <Harness scoped paths>
git ls-files --others --exclude-standard
```

Reject missing local `baseRef`; never fetch or substitute another branch. Normalize repository-relative `/` paths, merge duplicate paths, parse only unified hunk headers, and hash canonical JSON of `BaseRef + Head + sorted Files`.

- [ ] **Step 4: Run Change Set tests to GREEN**

```powershell
go test -count=1 ./internal/changeset -run Test153
```

Expected: PASS.

- [ ] **Step 5: Write failing Controller inventory tests for the real reported defect**

Fixture must contain:

```text
AController.java  new, endpoints: create
BController.java  new, endpoints: submit
CController.java  modified only inside update endpoint
PlainService.java changed but not a Controller
FakeController.java class-name suffix only, no Controller annotation
```

Expected inventory:

```go
want := []string{
    "AController.create",
    "BController.submit",
    "CController.update",
}
```

Add regressions:
- class-name suffix without `@RestController`/`@Controller` is excluded;
- modified hunk inside one endpoint expects only that endpoint;
- class-level behavior-affecting hunk outside endpoint method ranges expects all production endpoint methods in that Controller;
- deleted endpoint is represented as `REMOVED`, not required as a new Chain;
- explicit TARGETED intent only requires target-specific inventory.

- [ ] **Step 6: Run inventory tests and verify RED**

```powershell
go test -count=1 ./internal/analysis -run 'Test153.*Entrypoint'
```

Expected: FAIL because inventory builder is absent.

- [ ] **Step 7: Implement machine Controller/endpoint inventory using pinned Navigation evidence**

Do not infer from `XxxController` names. Reuse existing ast-grep runner/Navigator and add only a focused endpoint scanner if the existing public methods cannot return annotation + method range facts:

```go
type ControllerEndpoint struct {
    Controller string
    Symbol     string
    Path       string
    StartLine  int
    EndLine    int
}
```

Recognize production Controller classes from actual Spring controller annotations and endpoint methods from actual mapping annotations. Map Change Set hunks to endpoint ranges deterministically.

- [ ] **Step 8: Implement `analysis inventory` command and schema**

Runtime command:

```text
codea-harness-tools analysis inventory --input .code-harness/runs/<runId>/requests/entrypoint-inventory.json
```

Request body:

```json
{
  "runId":"r153",
  "baseRef":"develop",
  "includeWorkingTree":true,
  "intent":{"mode":"FULL"}
}
```

Runtime writes only:

```text
.code-harness/runs/<runId>/analysis/entrypoint-inventory.json
```

and validates the output against `entrypoint-inventory.schema.json` before success.

- [ ] **Step 9: Run Task 1 exact gate**

```powershell
go test -count=1 ./internal/changeset ./internal/analysis ./cmd/codea-harness-tools -run Test153
go vet ./internal/changeset ./internal/analysis ./cmd/codea-harness-tools
```

Expected: PASS.

- [ ] **Step 10: Commit Task 1**

```powershell
git add .code-harness/contracts/entrypoint-inventory.schema.json `
        .code-harness/tools-runtime/internal/changeset `
        .code-harness/tools-runtime/internal/analysis `
        .code-harness/tools-runtime/cmd/codea-harness-tools/analysis_command.go `
        .code-harness/tools-runtime/cmd/codea-harness-tools/analysis_inventory_test.go `
        .code-harness/tools-runtime/cmd/codea-harness-tools/main.go
git commit -m "feat: add changed controller entrypoint completeness gate"
```

**Task 1 acceptance:** The three-Controller fixture produces exactly 3 expected entrypoints; a Controller-name suffix without machine annotation evidence never appears.

---

### Task 2: Certified ChangeAnalysis

**Files:**
- Create: `.code-harness/contracts/change-analysis-cert.schema.json`
- Create: `.code-harness/tools-runtime/internal/analysis/certify.go`
- Create: `.code-harness/tools-runtime/internal/analysis/evidence.go`
- Create: `.code-harness/tools-runtime/internal/analysis/certificate.go`
- Create: `.code-harness/tools-runtime/internal/analysis/certify_test.go`
- Create: `.code-harness/tools-runtime/internal/analysis/tamper_test.go`
- Create: `.code-harness/tools-runtime/cmd/codea-harness-tools/analysis_certify_test.go`
- Modify: `.code-harness/tools-runtime/cmd/codea-harness-tools/analysis_command.go`
- Modify: `.code-harness/tools-runtime/cmd/codea-harness-tools/chain_command.go`
- Modify: `.code-harness/tools-runtime/cmd/codea-harness-tools/chain_review_context.go`
- Modify: `.code-harness/tools-runtime/cmd/codea-harness-tools/main.go` only if command routing needs adjustment
- Modify: `.code-harness/skills/analyze-change/SKILL.md`
- Modify: `.code-harness/agents/reviewer.md`
- Modify: `.code-harness/AGENTS.md`

**Interfaces:**

```go
package analysis

type Certificate struct {
    RunID                     string `json:"runId"`
    RuntimeVersion            string `json:"runtimeVersion"`
    AnalysisSHA256            string `json:"analysisSha256"`
    ChangeSetSHA256           string `json:"changeSetSha256"`
    EntrypointInventorySHA256 string `json:"entrypointInventorySha256"`
    BaseRef                   string `json:"baseRef"`
    Head                      string `json:"head"`
}

type CertifyRequest struct {
    RunID              string `json:"runId"`
    DraftPath          string `json:"draftPath"`
    BaseRef            string `json:"baseRef"`
    IncludeWorkingTree bool   `json:"includeWorkingTree"`
    Intent             Intent `json:"intent"`
}

func Certify(root string, req CertifyRequest) (Certificate, error)
func LoadCertified(root, analysisPath string) (ChangeAnalysis, Certificate, error)
```

- `LoadCertified` becomes the only shared loader for Chain/Review consumers after this Task.
- It must verify same-run paths, analysis hash, inventory hash, current recomputed Change Set digest, schema, Coverage, and certificate identity before returning.

- [ ] **Step 1: Write RED regression for incomplete Agent draft**

Use the Task 1 three-Controller fixture, but draft only:

```json
{
  "affectedControllers":[{"controller":"AController","endpoints":["AController.create"]}],
  "callChains":[{"entryPoint":"AController.create","chain":["AController.create","AService.create"]}]
}
```

Expected error must include:

```text
ENTRYPOINT_COMPLETENESS_INCOMPLETE
BController.submit
CController.update
```

- [ ] **Step 2: Write RED regression for exact Change Set mismatch**

Draft omits a staged/untracked changed file while claiming COMPLETE. Runtime must return:

```text
CHANGE_SET_MISMATCH
```

and produce **no** authoritative `analysis/change-analysis.json`.

- [ ] **Step 3: Write RED evidence invariant tests**

Reject:
- confirmed entrypoint missing exact current-workspace Controller symbol/path/role;
- callChain node absent from `symbolLocations`;
- conflicting same-symbol workspace/path/role facts;
- dependency workspace used as changed/reviewed file;
- invalid Mapper relation path/role/source;
- unresolved entrypoint with no explicit limitation code.

Use existing 1.5.2 workspace evidence tests as inputs; do not reopen Task 1–6 accepted semantics.

- [ ] **Step 4: Run certification tests and verify RED**

```powershell
go test -count=1 ./internal/analysis -run 'Test153.*Cert|Test153.*Tamper|Test153.*Evidence'
```

Expected: FAIL.

- [ ] **Step 5: Implement proposal-only input + Runtime certification**

Agent writes only:

```text
.code-harness/runs/<runId>/requests/change-analysis-draft.json
```

Runtime command:

```text
codea-harness-tools analysis certify --input .code-harness/runs/<runId>/requests/analysis-certify.json
```

Certification sequence is fixed:

```text
strict request decode
→ same-run request path validation
→ schema validate draft
→ changeset.Compute
→ BuildEntrypointInventory
→ exact changedFiles comparison
→ VerifyEntrypointDispositions
→ validate symbol/workspace/path/role invariants
→ validate resource relation invariants
→ coverage.VerifyAnalysisJSON
→ canonical marshal authoritative analysis
→ hash analysis + inventory + Change Set
→ atomic write analysis/change-analysis.json
→ atomic write analysis/entrypoint-inventory.json
→ atomic write analysis/change-analysis.cert.json
```

Do not silently add omitted Controller/callChain facts. Certification fails closed.

- [ ] **Step 6: Implement `LoadCertified` tamper/current-state checks**

At minimum:

```go
if sha256(analysisBytes) != cert.AnalysisSHA256 { return CHANGED_ANALYSIS_HASH_MISMATCH }
if sha256(inventoryBytes) != cert.EntrypointInventorySHA256 { return ENTRYPOINT_INVENTORY_HASH_MISMATCH }
if currentChangeSet.SHA256 != cert.ChangeSetSHA256 { return CERTIFIED_CHANGE_SET_STALE }
```

Then rerun schema/Coverage and exact run/path validation before returning parsed facts.

- [ ] **Step 7: Migrate all Chain consumers to `LoadCertified`**

Replace direct `loadVerifiedChainAnalysis` semantics in:

```text
chain discover
chain validate
chain refresh
chain review-context
```

with the shared certified loader. The old loader may remain as a thin wrapper only if it delegates entirely to `analysis.LoadCertified`.

- [ ] **Step 8: Update Reviewer/analyze-change contract**

Required wording:

```text
Agent may create requests/change-analysis-draft.json only.
Agent must not create/overwrite analysis/change-analysis.json or its certificate/inventory.
Runtime certification failure means MANUAL_ACTION_REQUIRED/PARTIAL; Agent must not repair authoritative artifacts itself.
```

Also list `analysis certify` as the mandatory Runtime step before Review/Chain consumption.

- [ ] **Step 9: Run Task 2 exact gate and historical Chain gates**

```powershell
go test -count=1 ./internal/analysis ./internal/coverage ./internal/chain ./internal/reviewscope ./cmd/codea-harness-tools -run 'Test153|Test151|Test152'
go test ./...
go vet ./...
```

Expected: PASS.

- [ ] **Step 10: Commit Task 2**

```powershell
git add .code-harness/contracts/change-analysis-cert.schema.json `
        .code-harness/tools-runtime/internal/analysis `
        .code-harness/tools-runtime/cmd/codea-harness-tools `
        .code-harness/skills/analyze-change/SKILL.md `
        .code-harness/agents/reviewer.md `
        .code-harness/AGENTS.md
git commit -m "feat: certify change analysis before review and chain use"
```

**Task 2 acceptance:** A draft that silently omits 2/3 changed Controller entrypoints can never produce a Certified ChangeAnalysis; editing the authoritative analysis/cert/inventory after certification makes all consumers fail closed.

---

### Task 3: Chain Artifact & Write Authority Hardening

**Files:**
- Create: `.code-harness/contracts/chain-candidate-cert.schema.json`
- Create: `.code-harness/contracts/chain-write-plan.schema.json`
- Create: `.code-harness/tools-runtime/internal/chain/candidate_provenance.go`
- Create: `.code-harness/tools-runtime/internal/chain/write_plan.go`
- Create: `.code-harness/tools-runtime/internal/chain/candidate_provenance_test.go`
- Create: `.code-harness/tools-runtime/internal/chain/write_plan_test.go`
- Create: `.code-harness/tools-runtime/cmd/codea-harness-tools/chain_authority_153_test.go`
- Modify: `.code-harness/tools-runtime/internal/chain/discover.go`
- Modify: `.code-harness/tools-runtime/internal/chain/management.go`
- Modify: `.code-harness/tools-runtime/cmd/codea-harness-tools/chain_command.go`
- Modify: `.code-harness/skills/discover-chain/SKILL.md`
- Modify: `.code-harness/skills/validate-chain/SKILL.md`
- Modify: `.code-harness/agents/orchestrator.md`
- Modify: `.code-harness/AGENTS.md`

**Interfaces:**

```go
package chain

type CandidateCertificate struct {
    RunID          string `json:"runId"`
    Kind           string `json:"kind"` // DISCOVERED | REFRESH | EDIT
    ChainID        string `json:"chainId"`
    CandidatePath  string `json:"candidatePath"`
    CandidateHash  string `json:"candidateHash"`
    AnalysisHash   string `json:"analysisHash"`
}

type WritePlan struct {
    PlanID               string `json:"planId"`
    RunID                string `json:"runId"`
    ChainID              string `json:"chainId"`
    CandidatePath        string `json:"candidatePath"`
    CandidateHash        string `json:"candidateHash"`
    AnalysisHash         string `json:"analysisHash"`
    ExpectedExistingHash string `json:"expectedExistingHash,omitempty"`
    PreviewSHA256        string `json:"previewSha256"`
}

func CertifyCandidate(root string, c Chain, candidatePath, kind string, cert analysis.Certificate) (CandidateCertificate, error)
func LoadRuntimeCandidate(root string, candidatePath string, cert analysis.Certificate) (Chain, CandidateCertificate, error)
func SealWritePlan(root string, runID, candidatePath, expectedExistingHash string) (WritePlan, error)
func PersistWritePlan(root string, runID, planID string) error
```

- [ ] **Step 1: Write RED test: fake Agent-created candidate in the right directory**

Create a syntactically valid:

```text
.code-harness/runs/r153/analysis/discovered-chains/fake.yaml
```

without Runtime candidate certificate. `SealWritePlan` must reject:

```text
CHAIN_ARTIFACT_NOT_RUNTIME_OWNED
```

- [ ] **Step 2: Write RED test: mutate Runtime candidate after discovery**

Flow:

```text
Runtime Discover → candidate + certificate
Agent changes one node byte
SealWritePlan
```

Expected:

```text
CHAIN_CANDIDATE_HASH_MISMATCH
```

- [ ] **Step 3: Write RED test: mutate candidate after sealing**

Flow:

```text
candidate → seal-persist → planId
mutate candidate
persist(planId)
```

Expected: hash mismatch, `0 Project State writes`.

Also change existing `.code-harness/chains/<id>.yaml` after sealing; expected existing hash mismatch, `0 writes`.

- [ ] **Step 4: Run authority tests and verify RED**

```powershell
go test -count=1 ./internal/chain ./cmd/codea-harness-tools -run 'Test153.*Authority|Test153.*WritePlan|Test153.*Candidate'
```

Expected: FAIL.

- [ ] **Step 5: Make Discover/Refresh emit candidate certificates**

For every Runtime-written candidate:

```text
analysis/discovered-chains/<id>.yaml
analysis/discovered-chains/<id>.cert.json

analysis/refresh-candidates/<id>.yaml
analysis/refresh-candidates/<id>.cert.json
```

Certificate hash binds exact candidate bytes + Certified ChangeAnalysis `analysisSha256` + runId + candidate kind.

- [ ] **Step 6: Add `chain seal-persist`**

Runtime command:

```text
codea-harness-tools chain seal-persist --input .code-harness/runs/<runId>/requests/chain-seal-persist.json
```

It loads only Runtime-certified discovered/refresh/edit candidates, revalidates against Certified ChangeAnalysis, computes existing Project State hash, writes:

```text
.code-harness/runs/<runId>/analysis/chain-write-plans/<planId>.json
```

`planId` must be deterministic from immutable plan facts or a collision-safe digest identity; it must change whenever candidate bytes, analysis hash, or expected existing hash changes.

- [ ] **Step 7: Change `chain persist` to consume sealed plan identity**

Request body is only:

```json
{
  "runId":"r153",
  "planId":"chain-write-3e4d5f"
}
```

Persist sequence:

```text
load same-run write plan
→ LoadCertified analysis
→ re-read candidate + candidate certificate
→ verify candidateHash + analysisHash
→ revalidate Chain code facts
→ verify expectedExistingHash
→ atomic SaveAccepted
```

Do not accept arbitrary candidatePath in the final persist request.

- [ ] **Step 8: Update Orchestrator/Skill authority wording**

Lock these rules:

```text
Generic Agent writes: requests/** only.
Runtime result artifacts: analysis/**, review.md, chains/**.
A file under Runtime-owned path without valid Runtime provenance is untrusted.
User confirmation authorizes exactly the currently displayed planId.
Any new candidate/plan bytes require a new confirmation.
```

Do not claim OS-level security when Agent and Runtime run as the same user. Where host path ACLs are configurable, document/request `ALLOW requests/**` and `DENY analysis/**|review.md|chains/**`; Runtime provenance remains mandatory regardless.

- [ ] **Step 9: Run Task 3 exact gate + old Apply safety**

```powershell
go test -count=1 ./internal/chain ./internal/apply ./cmd/codea-harness-tools -run 'Test153|Test152|TestChainPersist|TestApply'
go test ./...
go vet ./...
```

Expected: PASS.

- [ ] **Step 10: Commit Task 3**

```powershell
git add .code-harness/contracts/chain-candidate-cert.schema.json `
        .code-harness/contracts/chain-write-plan.schema.json `
        .code-harness/tools-runtime/internal/chain `
        .code-harness/tools-runtime/cmd/codea-harness-tools/chain_command.go `
        .code-harness/skills/discover-chain/SKILL.md `
        .code-harness/skills/validate-chain/SKILL.md `
        .code-harness/agents/orchestrator.md `
        .code-harness/AGENTS.md
git commit -m "feat: harden runtime ownership of chain artifacts"
```

**Task 3 acceptance:** A hand-created/edited Chain artifact cannot be sealed or persisted; a sealed plan cannot authorize changed bytes; only Runtime-certified candidates can reach Project State.

---

### Task 4: `harness review` Mode & Chain Selection UX

**Files:**
- Create: `.code-harness/contracts/review-options.schema.json`
- Create: `.code-harness/contracts/review-selection-request.schema.json`
- Create: `.code-harness/tools-runtime/internal/reviewselection/options.go`
- Create: `.code-harness/tools-runtime/internal/reviewselection/verify.go`
- Create: `.code-harness/tools-runtime/internal/reviewselection/options_test.go`
- Create: `.code-harness/tools-runtime/internal/reviewselection/verify_test.go`
- Create: `.code-harness/tools-runtime/cmd/codea-harness-tools/review_command.go`
- Create: `.code-harness/tools-runtime/cmd/codea-harness-tools/review_selection_153_test.go`
- Modify: `.code-harness/tools-runtime/cmd/codea-harness-tools/main.go`
- Modify: `.code-harness/tools-runtime/internal/reviewscope/reviewscope.go` only to expose/reuse exact scope derivation; do not weaken its current machine gates
- Modify: `.code-harness/tools-runtime/cmd/codea-harness-tools/chain_review_context.go` to consume Certified ChangeAnalysis and expose Runtime-owned Chain options cleanly
- Modify: `.code-harness/agents/orchestrator.md`
- Modify: `.code-harness/agents/reviewer.md`
- Modify: `.code-harness/skills/analyze-change/SKILL.md`

**Interfaces:**

```go
package reviewselection

type Decision string
const (
    DecisionAutoFull   Decision = "AUTO_FULL"
    DecisionAutoSingle Decision = "AUTO_SINGLE"
    DecisionUser       Decision = "USER_SELECTION"
)

type ChainOption struct {
    SelectionID string   `json:"selectionId"`
    ChainID     string   `json:"chainId"`
    EntryPoints []string `json:"entryPoints"`
    Source      string   `json:"source"` // ACCEPTED | TEMPORARY
    Status      string   `json:"status"` // VALID | TEMPORARY
}

type Options struct {
    RunID                  string        `json:"runId"`
    ChangeSetSHA256        string        `json:"changeSetSha256"`
    EntrypointCompleteness string        `json:"entrypointCompleteness"`
    Decision               Decision      `json:"decision"`
    AutoSelectionIDs       []string      `json:"autoSelectionIds,omitempty"`
    Chains                 []ChainOption `json:"chains"`
    OptionsHash            string        `json:"optionsHash"`
}

type SelectionRequest struct {
    RunID        string   `json:"runId"`
    Mode         string   `json:"mode"` // FULL | TARGETED | LIST
    SelectionIDs []string `json:"selectionIds,omitempty"`
    OptionsHash  string   `json:"optionsHash"`
}

func BuildOptions(root string, certifiedAnalysisPath string, target string) (Options, error)
func VerifyAndBuildScope(root string, req SelectionRequest) (reviewscope.Selection, error)
```

**Single-option rule is mandatory:**

```text
0 valid Chains  → AUTO_FULL for plain `harness review`; no Chain prompt
1 valid Chain   → AUTO_SINGLE TARGETED; no Controller/Chain prompt
2+ valid Chains → USER_SELECTION; show menu/multi-select
```

For explicit Service/downstream target:

```text
1 upstream Chain   → AUTO_SINGLE, no prompt
2+ upstream Chains → USER_SELECTION
```

Explicit `harness review Controller` / `Controller.method` continues directly and includes all machine-required branches for that target; do not introduce a redundant Controller prompt.

- [ ] **Step 1: Write RED options tests for 0/1/2+ Chains**

Assertions:

```go
if got.Decision != DecisionAutoFull { ... }      // zero
if got.Decision != DecisionAutoSingle { ... }    // exactly one
if len(got.AutoSelectionIDs) != 1 { ... }
if got.Decision != DecisionUser { ... }          // two or more
```

Also assert options are blocked entirely when entrypoint inventory is incomplete.

- [ ] **Step 2: Write RED stale/forged selection tests**

Reject:

```text
REVIEW_OPTIONS_STALE                 wrong optionsHash / changed Change Set
REVIEW_SELECTION_UNKNOWN_CHAIN       selectionId not in Runtime options
REVIEW_SELECTION_SCOPE_INVALID       derived TARGETED scope fails reviewscope.Verify
```

- [ ] **Step 3: Run selection tests and verify RED**

```powershell
go test -count=1 ./internal/reviewselection ./cmd/codea-harness-tools -run 'Test153.*ReviewOption|Test153.*Selection'
```

Expected: FAIL.

- [ ] **Step 4: Implement Runtime ReviewOptions generation**

Command:

```text
codea-harness-tools review options --input .code-harness/runs/<runId>/requests/review-options-request.json
```

Runtime must:

```text
LoadCertified
→ require complete relevant EntrypointInventory
→ resolve ACCEPTED+VALID / DISCOVERED+TEMPORARY Chains using existing Chain rules
→ canonicalize/sort options
→ assign C1..Cn only after stable sorting
→ compute optionsHash over certified analysis hash + option facts
→ set AUTO_FULL/AUTO_SINGLE/USER_SELECTION
→ write analysis/review-options.json
```

Agent must never invent option IDs.

- [ ] **Step 5: Implement single-option auto execution as machine behavior**

If exactly one valid option:

```json
{
  "decision":"AUTO_SINGLE",
  "autoSelectionIds":["C1"]
}
```

Orchestrator must not ask “which Controller/Chain?” and must immediately invoke Runtime selection verification with `C1` + current `optionsHash`. Plain `harness review` therefore proceeds directly as single-Chain TARGETED Review when there is exactly one valid Chain.

This is the normative amendment and must have both unit and command-level regression coverage.

- [ ] **Step 6: Implement 2+ user selection and final scope generation**

Command:

```text
codea-harness-tools review select --input .code-harness/runs/<runId>/requests/review-selection-request.json
```

For `mode=TARGETED`, Runtime maps selected option IDs to exact callChains and derives current-workspace scopedFiles through existing `reviewscope` logic, then writes:

```text
.code-harness/runs/<runId>/analysis/review-scope.json
```

For `mode=FULL`, Runtime produces FULL scope without Chain IDs. For `mode=LIST`, no Finding review is authorized.

- [ ] **Step 7: Update plain `harness review` Orchestrator flow**

New flow text must be exact:

```text
certify ChangeAnalysis
→ completeness gate
→ review options
→ AUTO_FULL/AUTO_SINGLE execute directly
→ USER_SELECTION only when 2+ valid options
→ Runtime select/verify
→ FULL or TARGETED Review
```

For 2+ options, show:

```text
1. 全部评审
2. 按业务链评审
3. 仅查看调用链
```

Only if the user chooses business Chain review, show the 2+ Runtime options as structured multi-select/numbered fallback.

- [ ] **Step 8: Run Task 4 exact gate + historical Review isolation**

```powershell
go test -count=1 ./internal/reviewselection ./internal/reviewscope ./internal/coverage ./internal/chain ./internal/report ./cmd/codea-harness-tools -run 'Test153|Test152'
go test ./...
go vet ./...
```

Expected: PASS.

- [ ] **Step 9: Commit Task 4**

```powershell
git add .code-harness/contracts/review-options.schema.json `
        .code-harness/contracts/review-selection-request.schema.json `
        .code-harness/tools-runtime/internal/reviewselection `
        .code-harness/tools-runtime/internal/reviewscope `
        .code-harness/tools-runtime/cmd/codea-harness-tools/review_command.go `
        .code-harness/tools-runtime/cmd/codea-harness-tools/review_selection_153_test.go `
        .code-harness/tools-runtime/cmd/codea-harness-tools/main.go `
        .code-harness/agents/orchestrator.md `
        .code-harness/agents/reviewer.md `
        .code-harness/skills/analyze-change/SKILL.md
git commit -m "feat: add certified review chain selection"
```

**Task 4 acceptance:** No selection UI is shown for exactly one valid Chain/upstream Controller; 2+ options require Runtime-bound selection; incomplete inventory cannot present a misleading menu.

---

### Task 5: Human-friendly Chain Edit Skill

**Files:**
- Create: `.code-harness/contracts/chain-edit-request.schema.json`
- Create: `.code-harness/tools-runtime/internal/chain/edit.go`
- Create: `.code-harness/tools-runtime/internal/chain/edit_test.go`
- Create: `.code-harness/tools-runtime/cmd/codea-harness-tools/chain_edit_153_test.go`
- Create: `.code-harness/skills/edit-chain/SKILL.md`
- Modify: `.code-harness/tools-runtime/cmd/codea-harness-tools/chain_command.go`
- Modify: `.code-harness/agents/orchestrator.md`
- Modify: `.code-harness/agents/reviewer.md` only if handoff wording references editing
- Modify: `.code-harness/skills/validate-chain/SKILL.md`
- Modify: `.code-harness/AGENTS.md`

**Interfaces:**

```go
package chain

type EditOperation struct {
    Type   string   `json:"type"` // REPLACE_NODE | ADD_NODE | REMOVE_NODE | REORDER_NODE | RENAME_CHAIN | UPDATE_NOTES
    From   string   `json:"from,omitempty"`
    To     string   `json:"to,omitempty"`
    Symbol string   `json:"symbol,omitempty"`
    After  string   `json:"after,omitempty"`
    Name   string   `json:"name,omitempty"`
    Notes  string   `json:"notes,omitempty"`
    Order  []string `json:"order,omitempty"`
}

type EditRequest struct {
    RunID              string          `json:"runId"`
    ChainID            string          `json:"chainId"`
    ChangeAnalysisPath string          `json:"changeAnalysisPath"`
    Operations         []EditOperation `json:"operations"`
}

type EditResult struct {
    Status        string   `json:"status"` // EDIT_READY
    CandidatePath string   `json:"candidatePath"`
    Added         []string `json:"added"`
    Removed       []string `json:"removed"`
    Changed       []string `json:"changed"`
}

func ApplyVerifiedEdit(root string, req EditRequest) (EditResult, error)
```

No `ADD_ENTRYPOINT`/`REMOVE_ENTRYPOINT` in 1.5.3.

- [ ] **Step 1: Write RED tests for supported semantic operations**

Cover all six:

```text
REPLACE_NODE
ADD_NODE
REMOVE_NODE
REORDER_NODE
RENAME_CHAIN
UPDATE_NOTES
```

For code-fact operations, the resulting ordered core path must match a callChain/path/role sequence in Certified ChangeAnalysis. Name/notes may change without code-fact inference.

- [ ] **Step 2: Write RED unverified-fact tests**

Examples:

```text
replace OldService.process → InventedService.process
add node that exists as a symbol but is not on any verified chain edge
reorder nodes into a sequence not present in certified callChains
use dependency workspace node as if it were current workspace
```

Expected:

```text
CHAIN_EDIT_FACT_NOT_VERIFIED
```

and no edit candidate file.

- [ ] **Step 3: Run edit tests and verify RED**

```powershell
go test -count=1 ./internal/chain ./cmd/codea-harness-tools -run 'Test153.*Edit'
```

Expected: FAIL.

- [ ] **Step 4: Implement strict edit request/schema decoding**

Command:

```text
codea-harness-tools chain edit --input .code-harness/runs/<runId>/requests/chain-edit-request.json
```

Runtime must:

```text
Load existing exact project Chain
→ LoadCertified target-specific CHAIN_MAINTENANCE analysis
→ apply operations in memory
→ preserve Chain version/id/entryPoints
→ verify resulting code facts against Certified ChangeAnalysis
→ compute deterministic diff
→ write analysis/chain-edit-candidates/<id>.yaml
→ write matching candidate certificate kind=EDIT
```

Do not write `.code-harness/chains/**` in this command.

- [ ] **Step 5: Implement `edit-chain` Skill as proposal-only natural-language translator**

Skill tools may read/show Chain and create request proposal, but must never edit Chain YAML directly. Its contract is:

```text
user natural language
→ resolve exact Chain id
→ build semantic operations only
→ target-specific analyze-change draft (intent=CHAIN_MAINTENANCE)
→ Runtime analysis certify
→ Runtime chain edit
→ show deterministic candidate diff
→ ask explicit save confirmation for exact candidate
→ chain seal-persist
→ show planId preview
→ explicit confirmation of that plan
→ chain persist(planId)
```

If the user says a code relationship that Runtime cannot verify, explain it as unverified; offer to store it in `notes` only if the user explicitly wants business context rather than code fact.

- [ ] **Step 6: Add Orchestrator route**

Add:

```text
harness chain edit <id|Controller|Controller.method>
```

Exact target resolution only. Multiple matches → ambiguity; no fuzzy selection. `chain show` continuation may reuse the displayed exact id when the user's referent is unambiguous.

- [ ] **Step 7: Prove sealed persistence is reused, not duplicated**

Command-level regression:

```text
chain edit
→ candidate certificate
→ seal-persist
→ mutate candidate
→ persist old planId
```

must reject with `0 Project State writes`.

- [ ] **Step 8: Run Task 5 exact gate**

```powershell
go test -count=1 ./internal/chain ./internal/analysis ./cmd/codea-harness-tools -run 'Test153.*Edit|Test153.*WritePlan'
go test ./...
go vet ./...
```

Expected: PASS.

- [ ] **Step 9: Commit Task 5**

```powershell
git add .code-harness/contracts/chain-edit-request.schema.json `
        .code-harness/tools-runtime/internal/chain/edit.go `
        .code-harness/tools-runtime/internal/chain/edit_test.go `
        .code-harness/tools-runtime/cmd/codea-harness-tools/chain_edit_153_test.go `
        .code-harness/tools-runtime/cmd/codea-harness-tools/chain_command.go `
        .code-harness/skills/edit-chain/SKILL.md `
        .code-harness/skills/validate-chain/SKILL.md `
        .code-harness/agents/orchestrator.md `
        .code-harness/agents/reviewer.md `
        .code-harness/AGENTS.md
git commit -m "feat: add verified natural language chain editing"
```

**Task 5 acceptance:** Developers never need to hand-edit Chain YAML for the supported operations; Agent cannot make an unverified code relation authoritative; all saves reuse Task 3 immutable plan persistence.

---

### Task 6: Real Project Regression, Windows Gate, Upgrade & 1.5.3 Release

**Files:**
- Create: `.github/workflows/task153-chain-reliability.yml`
- Create: `.code-harness/tools-runtime/cmd/codea-harness-tools/task153_controller_completeness_test.go`
- Create: `.code-harness/tools-runtime/cmd/codea-harness-tools/task153_artifact_authority_test.go`
- Create: `.code-harness/tools-runtime/cmd/codea-harness-tools/task153_review_selection_test.go`
- Create: `.code-harness/tools-runtime/cmd/codea-harness-tools/task153_chain_edit_test.go`
- Create: `.github/scripts/task153-real-review-chain-regression.ps1`
- Modify: `.github/workflows/package-windows-x64.yml`
- Modify: `.code-harness/tools-runtime/internal/upgrade/upgrade_15_test.go`
- Modify: `.code-harness/VERSION`
- Modify: `CHANGELOG.md`
- Modify release/manifest contract files already used by `package-windows-x64`

**Interfaces:** No new production interface after Task 5. Task 6 freezes and proves the release contract.

- [ ] **Step 1: Add the exact reported three-Controller real-project regression**

Windows fixture must contain real Git state with:

```text
new AController.java
new BController.java
modified CController.java
```

and real downstream Service/Impl/Mapper/Mapper.xml code. Use pinned real `ast-grep.exe` and built `codea-harness-tools.exe`.

First provide intentionally incomplete Agent draft with only `AController.create`; invoke `analysis certify`; assert:

```text
ENTRYPOINT_COMPLETENESS_INCOMPLETE
BController.submit
CController.update
0 authoritative change-analysis writes
```

Then provide complete proposal evidence derived from real Runtime Navigation and assert Certified inventory is exactly `3/3`.

- [ ] **Step 2: Add real single-option auto execution regression**

Create a Change Set whose certified inventory resolves exactly one valid business Chain. Run:

```text
analysis certify
review options
```

Assert:

```json
{"decision":"AUTO_SINGLE","autoSelectionIds":["C1"]}
```

and the acceptance driver does not create a user-selection request before executing `review select` with the machine auto-selection.

- [ ] **Step 3: Add real 2+ Chain selection regression**

Create at least three verified Chains. Assert:
- `decision=USER_SELECTION`;
- selecting `C1,C3` generates only those exact selectedCallChains/current scopedFiles;
- `optionsHash` mismatch is rejected;
- unknown `C9` is rejected;
- FULL still covers complete Change Set;
- LIST creates no Finding review authorization.

- [ ] **Step 4: Add artifact tamper regressions**

Real driver cases:

```text
Runtime discover → mutate candidate → seal-persist = REJECT
Agent creates fake analysis/discovered-chains YAML → seal-persist = REJECT
Runtime seal-persist → mutate candidate → persist(planId) = REJECT
Runtime certify → mutate analysis/change-analysis.json → chain discover = REJECT
```

Every rejected case must assert no `.code-harness/chains/**` Project State mutation.

- [ ] **Step 5: Add real Chain Edit regression**

Use a verified Chain where current source contains an alternate verified node path. Assert:

```text
REPLACE_NODE valid       → EDIT_READY
REORDER_NODE valid       → EDIT_READY
RENAME_CHAIN             → EDIT_READY
UPDATE_NOTES              → EDIT_READY
InventedService.process   → CHAIN_EDIT_FACT_NOT_VERIFIED
```

Then run `seal-persist` + explicit approval-driver `persist(planId)` and verify exact final Project State bytes/model.

- [ ] **Step 6: Add dedicated fresh CI workflow**

`.github/workflows/task153-chain-reliability.yml` must run on Windows and include named steps:

```text
Task 1 Controller EntryPoint completeness gate
Task 2 Certified ChangeAnalysis gate
Task 3 Chain artifact authority gate
Task 4 Review options and AUTO_SINGLE gate
Task 5 Chain edit gate
Full Go test
Go vet
```

Use `go test -count=1` for Task-specific gates; do not rely on Go test cache.

- [ ] **Step 7: Extend package Windows workflow with real 1.5.3 acceptance**

After pinned ast-grep/runtime build and existing 1.5.2 Task 5/6 regressions, run:

```text
Task 1.5.3 real review/chain reliability regression
```

The PowerShell script `.github/scripts/task153-real-review-chain-regression.ps1` must finish with:

```text
TASK153_REAL_REVIEW_CHAIN_RELIABILITY PASS
```

and explicit PASS evidence for:

```text
CONTROLLER_ENTRYPOINTS 3/3
INCOMPLETE_DRAFT_REJECTED
CERTIFIED_ANALYSIS_TAMPER_REJECTED
CHAIN_CANDIDATE_TAMPER_REJECTED
AUTO_SINGLE_NO_SELECTION
MULTI_CHAIN_SELECTION_VERIFIED
CHAIN_EDIT_VERIFIED
UNVERIFIED_EDIT_REJECTED
```

- [ ] **Step 8: Update version and release notes**

Set:

```text
.code-harness/VERSION = 1.5.3
```

CHANGELOG entry must describe:
- changed Controller completeness;
- Certified ChangeAnalysis;
- Runtime-owned Chain artifacts + sealed persistence;
- plain `harness review` selection behavior;
- exact-one Chain/Controller auto execution;
- natural-language Chain edit;
- no Chain YAML migration;
- 1.5.2 Workspace/Review isolation preserved.

- [ ] **Step 9: Add real 1.5.2 → 1.5.3 live upgrade gate**

Upgrade source baseline must be exact accepted `6f290d8ff160767bb981278aa123aa1621ea3343` or a formal release artifact built from that tree.

Before upgrade hash and after upgrade compare byte-for-byte:

```text
harness.yaml
project.md
database.yaml
runs/**
chains/**
```

Assert:

```text
VERSION becomes 1.5.3
new contracts/skills/runtime installed
existing chains/** byte-identical
existing runs/** byte-identical
no workspaceDependencies auto-injection
no Agent proposal promoted to Runtime state by upgrade
upgrade staging/source cleanup succeeds
```

- [ ] **Step 10: Build final artifacts and record digests**

Required names:

```text
codea-harness-1.5.3-windows-x64-install.zip
codea-harness-1.5.3-windows-x64-upgrade.zip
```

Release checklist must record workflow run ID, exact head SHA, artifact IDs, sizes, and SHA256 digests.

- [ ] **Step 11: Run final exact-head gate**

Required workflows on the same exact HEAD:

```text
task153-chain-reliability
package-windows-x64
task152-workspace-navigation
task152-review-isolation
task4-review-chain-consumption
task3-chain-management
```

All must be `completed/success`. A pending, skipped required step, or red future-test workflow means the release baseline is not accepted.

- [ ] **Step 12: Commit Task 6**

```powershell
git add .github/workflows `
        .github/scripts/task153-real-review-chain-regression.ps1 `
        .code-harness/tools-runtime/cmd/codea-harness-tools/task153_* `
        .code-harness/tools-runtime/internal/upgrade/upgrade_15_test.go `
        .code-harness/VERSION `
        CHANGELOG.md
git commit -m "feat: release Codea Harness 1.5.3"
```

**Task 6 acceptance:** Exact-head Windows CI proves the original 3-Controller omission is impossible to pass silently, Runtime-owned artifacts reject tampering, single-option selection auto-executes, 2+ Chain selection is machine-bound, Chain edit is verified/sealed, 1.5.2 regressions stay green, and live upgrade preserves all Project State.

---

## Fixed Development / Review Order

```text
Task 1 Controller / EntryPoint Completeness
→ acceptance + lock exact SHA
Task 2 Certified ChangeAnalysis
→ acceptance + lock exact SHA
Task 3 Chain Artifact / Write Authority
→ acceptance + lock exact SHA
Task 4 Review Selection UX + AUTO_SINGLE
→ acceptance + lock exact SHA
Task 5 Natural-language Chain Edit
→ acceptance + lock exact SHA
Task 6 Real Windows / Upgrade / Release
→ final 1.5.3 release baseline
```

Do not start the next Task on a red acceptance HEAD. Do not reopen an accepted Task unless a later real regression proves a concrete defect.

## Final Acceptance Gates

```text
G1  Real Git Change Set independently recomputed by Runtime
G2  Every changed production Controller entrypoint is CONFIRMED/PARTIAL/REMOVED; no silent missing
G3  Agent draft cannot become authoritative without Runtime certification
G4  Certified ChangeAnalysis tamper/staleness is rejected by all consumers
G5  Runtime-generated Chain candidate provenance is required
G6  Immutable Chain write plan binds candidate + analysis + existing hash
G7  Exactly one valid Controller/Chain option auto-executes with no user selection
G8  Two or more options require Runtime-bound user selection and optionsHash verification
G9  FULL/TARGETED/LIST Review semantics and 1.5.2 isolation remain correct
G10 Natural-language Chain edits cannot introduce unverified code facts
G11 Chain Edit persistence reuses the same sealed write path; no second write authority exists
G12 Workspace Dependency remains Navigation/Chain context only
G13 Existing Chain YAML version 1 and Project State survive 1.5.2→1.5.3 byte-for-byte
G14 Real Windows + pinned ast-grep + exact-head release workflows are green
```

## Plan Self-Review Checklist

- Spec coverage: Tasks 1–6 cover every normative section of the design plus the single-option amendment.
- No placeholder implementation steps: every new interface, command, error class, test fixture, and CI gate is named above.
- Type consistency: `changeset.Snapshot.SHA256`, `analysis.ChangeAnalysis`, `analysis.EntrypointInventory`, `analysis.Certificate`, `analysis.LoadCertified`, `chain.CandidateCertificate`, `chain.WritePlan`, and `reviewselection.Options.OptionsHash` are the shared identities used by downstream Tasks.
- Repository path consistency: real acceptance scripts live under `.github/scripts/**`, matching the existing 1.5.2 layout.
- Scope guard: no Test/Debug/Fix/Verify Chain integration, project-wide graph, fuzzy matching, JDT, network source resolution, or Chain YAML migration is introduced.
