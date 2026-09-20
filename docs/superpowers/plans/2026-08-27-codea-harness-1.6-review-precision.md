# Codea Harness 1.6 — Review Precision Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 Codea Harness 1.5.3 已完成 Certified ChangeAnalysis / Chain / Review Scope 的基础上，把 Finding 侧升级成 deterministic ReviewUnit + Rule Dispatch + Agent Finding Proposal + Runtime Finding Certification，使正式 Code Review 更深、更准、更稳定。

**Architecture:** Controlled Runtime 从同 run Certified ChangeAnalysis 和 verified ReviewScope 构造 ReviewUnit，并根据 framework-owned Spring rule catalog 生成 deterministic RuleDispatch。Reviewer 只能输出 Finding Proposal；Runtime 独立验证 anchor/evidence/scope/introducedByChange/dedup 后生成 Certified Findings，formal `review.md` 只能消费 Certified Findings。1.6 不新增第二套 CI Review pipeline，不做 Resume/Doctor/多语言/通用 SAST。

**Tech Stack:** Go 1.26.5 runtime, JSON Schema Draft 2020-12, YAML v3, pinned ast-grep 0.42.1, existing fixed-argument local Git navigation, PowerShell Windows acceptance scripts, GitHub Actions Windows runners.

**Spec:** `docs/superpowers/specs/2026-08-27-codea-harness-1.6-review-precision-design.md`

## Global Constraints

- Exact 1.6 baseline is `6f4c050783a7ec21f370799c1a8c69c9b51a9e92` (Codea Harness 1.5.3 release merge).
- Platform remains Windows 10/11 x64; no WSL dependency.
- Runtime must remain fully offline; no `git fetch`, Maven/Nexus/Central lookup, source download, SaaS rule fetch, or Internet dependency.
- Pinned Code Navigation remains ast-grep `0.42.1`; do not add regex Java parsing, JDT LS, or a second Java parser.
- 1.5.3 Change Set, Certified ChangeAnalysis, Review Selection, Chain authority, Workspace Dependency isolation and Test Validity semantics are immutable compatibility requirements.
- Workspace Dependency may contribute navigation/context symbols only; dependency paths must never enter ReviewUnit `files[]`, Finding Scope, Certified Findings, Test/Fix/Apply Write Scope.
- Agent/Orchestrator proposal writes remain under `.code-harness/runs/<runId>/requests/**`; Runtime-owned authoritative outputs remain under `.code-harness/runs/<runId>/analysis/**` and `.code-harness/runs/<runId>/review.md`.
- Agent must never directly create authoritative `review-units.json`, `rule-dispatch.json`, `certified-findings.json`, `certified-findings.cert.json` or final `review.md`.
- Every Task starts RED, reaches GREEN, runs its exact gate plus relevant regression packages, and ends in a reviewable commit.
- Do not expand 1.6 into CI adapter, Resume, Doctor, Dashboard, token-cost UI, multi-language support, generic SAST, Test/Fix/Apply feature work, or bundle concurrency.

---

## File Structure / Ownership Map

New Runtime packages:

```text
.code-harness/tools-runtime/internal/reviewunit/
  model.go
  build.go
  canonical.go
  build_test.go
  tamper_test.go

.code-harness/tools-runtime/internal/reviewrules/
  model.go
  catalog.go
  dispatch.go
  dispatch_test.go
  catalog_test.go

.code-harness/tools-runtime/internal/finding/
  model.go
  decode.go
  verify.go
  anchor.go
  evidence.go
  dedup.go
  certify.go
  certificate.go
  verify_test.go
  anchor_test.go
  dedup_test.go
  tamper_test.go
```

New contracts/rules:

```text
.code-harness/contracts/review-unit.schema.json
.code-harness/contracts/rule-dispatch.schema.json
.code-harness/contracts/finding-proposals.schema.json
.code-harness/contracts/certified-findings.schema.json
.code-harness/contracts/certified-findings-cert.schema.json
.code-harness/review-rules/spring-v1.yaml
```

Controlled Runtime command changes:

```text
.code-harness/tools-runtime/cmd/codea-harness-tools/review_precision_command.go
.code-harness/tools-runtime/cmd/codea-harness-tools/review_precision_command_test.go
.code-harness/tools-runtime/cmd/codea-harness-tools/main.go
```

Agent/Skill/Report integration:

```text
.code-harness/agents/reviewer.md
.code-harness/agents/orchestrator.md
.code-harness/skills/review-code/SKILL.md
.code-harness/contracts/review-output.schema.json
.code-harness/tools-runtime/internal/report/review.go
.code-harness/tools-runtime/internal/report/review_precision_test.go
```

Benchmark/release:

```text
.code-harness/tools-runtime/testdata/review-benchmark/**
.code-harness/tools-runtime/internal/finding/benchmark_test.go
.github/workflows/task160-review-precision.yml
.github/scripts/task160-real-review-precision-regression.ps1
.github/workflows/package-windows-x64.yml
.code-harness/VERSION
CHANGELOG.md
```

Do not restructure unrelated 1.5.3 packages while implementing these files.

---

### Task 1: Runtime ReviewUnit

**Objective:** 把 Certified ChangeAnalysis + verified Review Scope 转换成 deterministic、可认证、不会把 dependency workspace 混进来的 ReviewUnit。

**Files:**
- Create: `.code-harness/contracts/review-unit.schema.json`
- Create: `.code-harness/tools-runtime/internal/reviewunit/model.go`
- Create: `.code-harness/tools-runtime/internal/reviewunit/build.go`
- Create: `.code-harness/tools-runtime/internal/reviewunit/canonical.go`
- Create: `.code-harness/tools-runtime/internal/reviewunit/build_test.go`
- Create: `.code-harness/tools-runtime/internal/reviewunit/tamper_test.go`
- Create: `.code-harness/tools-runtime/cmd/codea-harness-tools/review_precision_command.go`
- Create: `.code-harness/tools-runtime/cmd/codea-harness-tools/review_precision_command_test.go`
- Modify: `.code-harness/tools-runtime/cmd/codea-harness-tools/main.go`

**Interfaces:**

```go
package reviewunit

type Mode string

const (
    ModeFull     Mode = "FULL"
    ModeTargeted Mode = "TARGETED"
)

type FileRef struct {
    Path      string `json:"path"`
    Role      string `json:"role"`
    Changed   bool   `json:"changed"`
    Workspace string `json:"workspace"`
}

type HunkRef struct {
    Path     string `json:"path"`
    NewStart int    `json:"newStart"`
    NewLines int    `json:"newLines"`
}

type Unit struct {
    ID             string    `json:"id"`
    EntryPoint     string    `json:"entryPoint,omitempty"`
    Chain          []string  `json:"chain,omitempty"`
    ContextSymbols []string  `json:"contextSymbols,omitempty"`
    Files          []FileRef `json:"files"`
    ChangedHunks   []HunkRef `json:"changedHunks,omitempty"`
}

type Manifest struct {
    RunID                string `json:"runId"`
    HarnessVersion       string `json:"harnessVersion"`
    Mode                 Mode   `json:"mode"`
    ChangeSetSHA256      string `json:"changeSetSha256"`
    ChangeAnalysisSHA256 string `json:"changeAnalysisSha256"`
    ReviewScopeSHA256    string `json:"reviewScopeSha256,omitempty"`
    Units                []Unit `json:"units"`
    SHA256               string `json:"sha256"`
}

type BuildInput struct {
    RunID          string
    RepoRoot       string
    CertifiedRunID string
}

func Build(input BuildInput) (Manifest, error)
func CanonicalBytes(m Manifest) ([]byte, error)
```

Runtime command:

```text
codea-harness-tools review units --run-id <runId>
```

Success writes exactly:

```text
.code-harness/runs/<runId>/analysis/review-units.json
```

**Required behavior:**

- FULL: every current-project Finding-scope required production file must belong to at least one Unit.
- TARGETED: only verified `scopedFiles` + selectedCallChains may enter Units.
- A confirmed entrypoint branch yields one canonical Unit; two distinct verified core signatures must yield two IDs.
- Identical core branch represented twice canonicalizes to one Unit.
- Changed production file that cannot be bound to a confirmed chain becomes standalone `RU-FILE-*`, not silently omitted.
- Dependency workspace symbol may appear only in `contextSymbols`; dependency path in `files[]` is hard failure `REVIEW_UNIT_SCOPE_VIOLATION`.
- Unit IDs and manifest SHA are deterministic across input ordering.
- Any change in Certified ChangeAnalysis / verified scope invalidates old manifest and returns `REVIEW_UNIT_STALE` to consumers.

- [ ] **Step 1: Write RED tests for FULL/TARGETED construction and deterministic IDs**

Add tests named exactly:

```go
func TestBuildFullCreatesBranchUnitsAndStandaloneChangedFile(t *testing.T)
func TestBuildTargetedUsesOnlyVerifiedScopedFiles(t *testing.T)
func TestBuildDistinctBranchesHaveDistinctDeterministicIDs(t *testing.T)
func TestBuildCanonicalizesDuplicateCoreBranch(t *testing.T)
func TestBuildRejectsDependencyPathInFiles(t *testing.T)
func TestCanonicalBytesStableAcrossInputOrder(t *testing.T)
func TestLoadRejectsStaleCertifiedAnalysis(t *testing.T)
```

Use existing 1.5.3 certified-analysis test fixtures; do not fake authority by constructing an uncertified `change-analysis.json` only.

- [ ] **Step 2: Run RED gate**

Run:

```powershell
cd .code-harness/tools-runtime
go test -count=1 ./internal/reviewunit ./cmd/codea-harness-tools
```

Expected: FAIL because `reviewunit.Build` / `review units` do not exist.

- [ ] **Step 3: Implement schema + model + canonical build**

`review-unit.schema.json` must require manifest hashes and reject additional properties. Build must use existing Certified Analysis loader and existing ReviewScope verifier; do not reimplement Change Set or target selection.

- [ ] **Step 4: Implement command and runtime-owned write**

Register `review units`. Command must use repository-root-safe path joins and atomic Runtime write pattern already used by analysis/chain artifacts.

- [ ] **Step 5: Run GREEN gate**

```powershell
cd .code-harness/tools-runtime
go test -count=1 ./internal/reviewunit ./cmd/codea-harness-tools
go test -count=1 ./...
go vet ./...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```text
git commit -m "feat: add deterministic review units"
```

**Task 1 acceptance:** Reviewer can be given a machine-owned, same-run ReviewUnit manifest; scope/dependency/canonicalization failures are fail-closed before any Finding Review.

---

### Task 2: Deterministic Spring Rule Catalog & Rule Dispatch

**Objective:** Runtime 决定“每个 ReviewUnit 应检查哪些规则”，避免 Agent 仅靠长 Prompt 自己选择检查项。

**Files:**
- Create: `.code-harness/contracts/rule-dispatch.schema.json`
- Create: `.code-harness/review-rules/spring-v1.yaml`
- Create: `.code-harness/tools-runtime/internal/reviewrules/model.go`
- Create: `.code-harness/tools-runtime/internal/reviewrules/catalog.go`
- Create: `.code-harness/tools-runtime/internal/reviewrules/dispatch.go`
- Create: `.code-harness/tools-runtime/internal/reviewrules/catalog_test.go`
- Create: `.code-harness/tools-runtime/internal/reviewrules/dispatch_test.go`
- Modify: `.code-harness/tools-runtime/cmd/codea-harness-tools/review_precision_command.go`
- Modify: `.code-harness/tools-runtime/cmd/codea-harness-tools/review_precision_command_test.go`

**Interfaces:**

```go
package reviewrules

type Kind string

const (
    KindAgent   Kind = "AGENT"
    KindMachine Kind = "MACHINE"
)

type Rule struct {
    ID               string   `yaml:"id" json:"id"`
    Version          int      `yaml:"version" json:"version"`
    Kind             Kind     `yaml:"kind" json:"kind"`
    SeverityDefault  string   `yaml:"severityDefault" json:"severityDefault"`
    Roles            []string `yaml:"roles" json:"roles"`
    RequiredEvidence []string `yaml:"requiredEvidence" json:"requiredEvidence"`
    Prompt           string   `yaml:"prompt" json:"prompt"`
}

type Dispatch struct {
    ReviewUnitID      string   `json:"reviewUnitId"`
    RuleID            string   `json:"ruleId"`
    RuleVersion       int      `json:"ruleVersion"`
    Kind              Kind     `json:"kind"`
    SeverityDefault   string   `json:"severityDefault"`
    RequiredEvidence  []string `json:"requiredEvidence"`
    DispatchReason    []string `json:"dispatchReason"`
}

type Manifest struct {
    RunID            string     `json:"runId"`
    ReviewUnitsSHA256 string    `json:"reviewUnitsSha256"`
    RuleCatalogSHA256 string    `json:"ruleCatalogSha256"`
    Dispatches       []Dispatch `json:"dispatches"`
    SHA256           string     `json:"sha256"`
}

func LoadCatalog(path string) ([]Rule, string, error)
func BuildDispatch(units reviewunit.Manifest, rules []Rule, catalogSHA string) (Manifest, error)
```

Runtime command:

```text
codea-harness-tools review dispatch --run-id <runId>
```

Writes:

```text
.code-harness/runs/<runId>/analysis/rule-dispatch.json
```

**Initial exact rule IDs:**

```text
MYBATIS-SQL-001
MYBATIS-ISOLATION-001
MYBATIS-BIND-001
MYBATIS-CONTRACT-001
SPRING-TX-001
SPRING-TX-002
SPRING-TX-003
SPRING-AUTH-001
SPRING-VALIDATION-001
SPRING-CONFIG-001
```

1.6 的 10 条规则默认全部 `kind=AGENT`，除非实现过程中某个规则能由现有结构化证据 100% 判定；不得为了“确定性”把 heuristic matcher hit 直接升级成 Finding。

- [ ] **Step 1: Write RED catalog/dispatch tests**

```go
func TestCatalogLoadsExactlyTenSpringV1Rules(t *testing.T)
func TestCatalogRejectsDuplicateRuleID(t *testing.T)
func TestDispatchMapperXmlGetsOnlyRelevantMyBatisRules(t *testing.T)
func TestDispatchTransactionalServiceGetsTxRules(t *testing.T)
func TestDispatchYamlGetsConfigRule(t *testing.T)
func TestDispatchDoesNotUseClassNameSuffixAsRoleFact(t *testing.T)
func TestDispatchStableAcrossUnitOrder(t *testing.T)
func TestDispatchRejectsStaleReviewUnits(t *testing.T)
```

Role matching must use machine role/resource evidence already stored in ReviewUnit; `XxxServiceImpl` / `XxxMapper` suffix is not authoritative.

- [ ] **Step 2: Run RED gate**

```powershell
cd .code-harness/tools-runtime
go test -count=1 ./internal/reviewrules ./cmd/codea-harness-tools
```

Expected: FAIL.

- [ ] **Step 3: Add framework-owned Spring v1 rule catalog**

Each rule must contain Chinese `prompt`, exact roles, required evidence and severity default. Do not add rule configuration UI or project override in 1.6.

- [ ] **Step 4: Implement deterministic dispatcher**

Dispatcher consumes ReviewUnit facts only. It must not read arbitrary sibling files or infer role from names.

- [ ] **Step 5: Run GREEN gate**

```powershell
cd .code-harness/tools-runtime
go test -count=1 ./internal/reviewrules ./internal/reviewunit ./cmd/codea-harness-tools
go test -count=1 ./...
go vet ./...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```text
git commit -m "feat: add spring review rule dispatch"
```

**Task 2 acceptance:** 相同 ReviewUnit + catalog 必须得到 byte-stable RuleDispatch；规则只被机器分发，不等于机器已经判定有问题。

---

### Task 3: Finding Proposal Contract + Runtime Anchor/Evidence Verification

**Objective:** Reviewer 的输出从 formal Finding 降为 proposal；Runtime 独立证明 proposal 的 rule、scope、path、symbol、line/range、evidence 与 introducedByChange。

**Files:**
- Create: `.code-harness/contracts/finding-proposals.schema.json`
- Create: `.code-harness/tools-runtime/internal/finding/model.go`
- Create: `.code-harness/tools-runtime/internal/finding/decode.go`
- Create: `.code-harness/tools-runtime/internal/finding/anchor.go`
- Create: `.code-harness/tools-runtime/internal/finding/evidence.go`
- Create: `.code-harness/tools-runtime/internal/finding/verify.go`
- Create: `.code-harness/tools-runtime/internal/finding/anchor_test.go`
- Create: `.code-harness/tools-runtime/internal/finding/verify_test.go`
- Modify: `.code-harness/skills/review-code/SKILL.md`
- Modify: `.code-harness/agents/reviewer.md`

**Interfaces:**

```go
package finding

type AnchorKind string

const (
    AnchorLine      AnchorKind = "LINE"
    AnchorSymbol    AnchorKind = "SYMBOL"
    AnchorFile      AnchorKind = "FILE"
    AnchorChangeSet AnchorKind = "CHANGESET"
)

type Anchor struct {
    Kind   AnchorKind `json:"kind"`
    Path   string     `json:"path,omitempty"`
    Line   int        `json:"line,omitempty"`
    Symbol string     `json:"symbol,omitempty"`
}

type EvidenceRef struct {
    Kind      string `json:"kind"`
    Value     string `json:"value,omitempty"`
    Path      string `json:"path,omitempty"`
    StartLine int    `json:"startLine,omitempty"`
    EndLine   int    `json:"endLine,omitempty"`
}

type Proposal struct {
    ProposalID        string        `json:"proposalId"`
    ReviewUnitID      string        `json:"reviewUnitId"`
    RuleID            string        `json:"ruleId"`
    Category          string        `json:"category"`
    Severity          string        `json:"severity"`
    Anchor            Anchor        `json:"anchor"`
    EvidenceRefs      []EvidenceRef `json:"evidenceRefs"`
    Problem           string        `json:"problem"`
    Impact            string        `json:"impact"`
    Recommendation    string        `json:"recommendation"`
    NeedsTest         bool          `json:"needsTest"`
    IntroducedByChange bool         `json:"introducedByChange"`
    Confidence        float64       `json:"confidence"`
}

type VerifiedProposal struct {
    Proposal Proposal
    AnchorDigest string
    EvidenceDigest string
}

func DecodeProposals(data []byte) ([]Proposal, error)
func Verify(ctx VerifyContext, p Proposal) (VerifiedProposal, error)
```

`VerifyContext` must be created from Runtime-owned loaders for Certified Analysis, ReviewUnit and RuleDispatch; it must not accept arbitrary Agent-provided trusted hashes.

**Fixed rejection codes:**

```text
RULE_NOT_DISPATCHED
FINDING_PROPOSAL_INVALID
FINDING_ANCHOR_NOT_VERIFIED
FINDING_EVIDENCE_NOT_VERIFIED
FINDING_SCOPE_VIOLATION
FINDING_DEPENDENCY_SCOPE_FORBIDDEN
FINDING_INTRODUCED_BY_CHANGE_NOT_VERIFIED
```

**Anchor semantics:**

- LINE: exact current-project path + actual line + optional symbol; Runtime verifies line belongs to declared symbol/source range when symbol supplied.
- SYMBOL: exact symbol must exist in current-project Certified Analysis/navigation evidence and belong to current Finding Scope.
- FILE: exact current-project Finding-scope path; permitted only when no more specific line/symbol is claimed.
- CHANGESET: at least two verified evidence refs from current Finding Scope; never a fallback for an unverifiable line.

Runtime must reject an invalid line; it must **not** silently search for a “nearby correct line” and approve it.

- [ ] **Step 1: Write RED proposal and anchor tests**

```go
func TestVerifyRejectsRuleNotDispatchedForUnit(t *testing.T)
func TestVerifyAcceptsExactLineAnchorInCurrentSymbol(t *testing.T)
func TestVerifyRejectsInventedLine(t *testing.T)
func TestVerifyRejectsInventedSymbol(t *testing.T)
func TestVerifyAcceptsSymbolAnchorForMissingConstraintFinding(t *testing.T)
func TestVerifyAcceptsFileAnchorWhenRuleAllowsFileEvidence(t *testing.T)
func TestVerifyChangeSetRequiresTwoVerifiedEvidenceRefs(t *testing.T)
func TestVerifyRejectsScopeOutsideFile(t *testing.T)
func TestVerifyRejectsWorkspaceDependencyPath(t *testing.T)
func TestVerifyRejectsIntroducedByChangeWithoutHunkOrContractEvidence(t *testing.T)
func TestVerifyPreservesExistingTestValidityBoundary(t *testing.T)
```

- [ ] **Step 2: Run RED gate**

```powershell
cd .code-harness/tools-runtime
go test -count=1 ./internal/finding ./internal/report
```

Expected: FAIL.

- [ ] **Step 3: Implement schema/decoder/anchor/evidence verification**

Use current source bytes, existing navigation ranges and changeset hunks; do not use OCR, fuzzy text search, or model confidence as proof.

- [ ] **Step 4: Change Reviewer Skill contract from Finding to Finding Proposal**

`review-code/SKILL.md` must say explicitly:

```text
Reviewer 只提出 Finding Proposal。
Proposal 不等于正式 Finding。
只有 Runtime certify-findings 成功后的 Certified Finding 才能进入 review.md。
```

Skill output schema must point to `finding-proposals.schema.json`. Keep Chinese user-facing fields and existing Java/Mapper/YML/Test Validity review boundaries.

- [ ] **Step 5: Run GREEN gate**

```powershell
cd .code-harness/tools-runtime
go test -count=1 ./internal/finding ./internal/report ./cmd/codea-harness-tools
go test -count=1 ./...
go vet ./...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```text
git commit -m "feat: verify review finding proposals"
```

**Task 3 acceptance:** 一个 Agent proposal 即使 schema 正确，只要 line/symbol/scope/evidence/rule 任何一项无法由 Runtime 证明，就不能成为正式 Finding。

---

### Task 4: Certified Findings Authority + Formal Report Integration

**Objective:** 把 1.5.3 的 “Agent proposes -> Runtime certifies” trust model 完整延伸到 Finding；formal `review.md` 不再直接信任 Agent `findings[]`。

**Files:**
- Create: `.code-harness/contracts/certified-findings.schema.json`
- Create: `.code-harness/contracts/certified-findings-cert.schema.json`
- Create: `.code-harness/tools-runtime/internal/finding/dedup.go`
- Create: `.code-harness/tools-runtime/internal/finding/certify.go`
- Create: `.code-harness/tools-runtime/internal/finding/certificate.go`
- Create: `.code-harness/tools-runtime/internal/finding/dedup_test.go`
- Create: `.code-harness/tools-runtime/internal/finding/tamper_test.go`
- Modify: `.code-harness/tools-runtime/cmd/codea-harness-tools/review_precision_command.go`
- Modify: `.code-harness/tools-runtime/cmd/codea-harness-tools/review_precision_command_test.go`
- Modify: `.code-harness/tools-runtime/internal/report/review.go`
- Create: `.code-harness/tools-runtime/internal/report/review_precision_test.go`
- Modify: `.code-harness/contracts/review-output.schema.json`
- Modify: `.code-harness/agents/orchestrator.md`

**Interfaces:**

```go
package finding

type CertifiedFinding struct {
    ID                 string        `json:"id"`
    RuleID             string        `json:"ruleId"`
    ReviewUnitID       string        `json:"reviewUnitId"`
    Category           string        `json:"category"`
    Severity           string        `json:"severity"`
    Anchor             Anchor        `json:"anchor"`
    EvidenceRefs       []EvidenceRef `json:"evidenceRefs"`
    Problem            string        `json:"problem"`
    Impact             string        `json:"impact"`
    Recommendation     string        `json:"recommendation"`
    NeedsTest          bool          `json:"needsTest"`
    IntroducedByChange bool          `json:"introducedByChange"`
    Confidence         float64       `json:"confidence"`
}

type CertifiedSet struct {
    RunID                string             `json:"runId"`
    HarnessVersion       string             `json:"harnessVersion"`
    ChangeSetSHA256      string             `json:"changeSetSha256"`
    ChangeAnalysisSHA256 string             `json:"changeAnalysisSha256"`
    ReviewUnitsSHA256    string             `json:"reviewUnitsSha256"`
    RuleDispatchSHA256   string             `json:"ruleDispatchSha256"`
    FindingProposalsSHA256 string           `json:"findingProposalsSha256"`
    Findings             []CertifiedFinding `json:"findings"`
    SHA256               string             `json:"sha256"`
}

type Certificate struct {
    RunID                  string `json:"runId"`
    CertifiedFindingsSHA256 string `json:"certifiedFindingsSha256"`
    ChangeSetSHA256         string `json:"changeSetSha256"`
    ChangeAnalysisSHA256    string `json:"changeAnalysisSha256"`
    ReviewUnitsSHA256       string `json:"reviewUnitsSha256"`
    RuleDispatchSHA256      string `json:"ruleDispatchSha256"`
    FindingProposalsSHA256  string `json:"findingProposalsSha256"`
    Mode                    string `json:"mode"`
    ScopeSHA256             string `json:"scopeSha256,omitempty"`
}

func Certify(ctx CertifyContext, proposals []Proposal) (CertifiedSet, Certificate, []Rejection, error)
func LoadCertified(repoRoot, runID string) (CertifiedSet, error)
```

Runtime command:

```text
codea-harness-tools review certify-findings --input .code-harness/runs/<runId>/requests/finding-certify-request.json
```

Writes only:

```text
.code-harness/runs/<runId>/analysis/certified-findings.json
.code-harness/runs/<runId>/analysis/certified-findings.cert.json
```

**Dedup canonical identity:**

```text
ruleId + anchor.kind + normalizedPath + canonicalSymbol/resource + evidenceDigest
```

Different中文措辞 / proposalId must not create duplicate Certified Findings.

**Formal report rule:**

`report.ReviewRequest.Findings` can no longer be an authoritative free-form array from Agent. Report generation must load same-run Certified Findings and map them into renderer transport. If legacy field remains temporarily for transport compatibility, Runtime must compare it byte-for-byte to loaded certified data and reject mismatch; preferred implementation is to remove it from Agent-facing request entirely.

- [ ] **Step 1: Write RED authority/dedup/tamper tests**

```go
func TestCertifyProducesOneFindingForSemanticDuplicates(t *testing.T)
func TestCertifyRejectsUnverifiedProposalWithoutBlockingOtherValidProposal(t *testing.T)
func TestLoadCertifiedRejectsChangedAnalysisBytes(t *testing.T)
func TestLoadCertifiedRejectsChangedReviewUnitBytes(t *testing.T)
func TestLoadCertifiedRejectsChangedRuleDispatchBytes(t *testing.T)
func TestLoadCertifiedRejectsChangedProposalBytes(t *testing.T)
func TestReportRejectsRawAgentFindingWithoutCertifiedSet(t *testing.T)
func TestReportUsesCertifiedFindingAnchor(t *testing.T)
func TestReportCanPassWithZeroCertifiedFindings(t *testing.T)
```

- [ ] **Step 2: Run RED gate**

```powershell
cd .code-harness/tools-runtime
go test -count=1 ./internal/finding ./internal/report ./cmd/codea-harness-tools
```

Expected: FAIL.

- [ ] **Step 3: Implement certification + dedup + certificate loader**

Single proposal rejection must be recorded as machine rejection and omitted; authoritative artifact stale/hash mismatch must fail closed for the entire formal report.

- [ ] **Step 4: Wire Orchestrator pipeline**

Exact order becomes:

```text
analysis certify
-> review scope/selection verify
-> review units
-> review dispatch
-> reviewer review-code produces requests/finding-proposals.json
-> review certify-findings
-> report render from Certified Findings
```

Orchestrator must not render report between proposal and certification.

- [ ] **Step 5: Update report transport**

Renderer should display LINE as `path:line`; SYMBOL as `path + symbol`; FILE as path; CHANGESET as cross-file evidence summary. It must not invent a line for non-LINE anchors.

- [ ] **Step 6: Run GREEN gate**

```powershell
cd .code-harness/tools-runtime
go test -count=1 ./internal/finding ./internal/report ./cmd/codea-harness-tools ./internal/reviewscope ./internal/reviewselection
go test -count=1 ./...
go vet ./...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```text
git commit -m "feat: certify formal review findings"
```

**Task 4 acceptance:** formal report authority 完全从 Agent raw Finding 切换到 same-run Certified Findings；任何上游 authoritative artifact tamper 都 fail closed。

---

### Task 5: Spring Rule Pack v1 Behavior + Deep Review Agent Guidance

**Objective:** 让 10 条规则真正提升 Spring/MyBatis Review 深度，同时严格控制误报，不把 Codea 变成泛化 SAST。

**Files:**
- Modify: `.code-harness/review-rules/spring-v1.yaml`
- Modify: `.code-harness/skills/review-code/SKILL.md`
- Modify: `.code-harness/agents/reviewer.md`
- Create: `.code-harness/tools-runtime/internal/reviewrules/spring_v1_contract_test.go`
- Create: `.code-harness/tools-runtime/internal/report/spring_v1_review_contract_test.go`

**Rule acceptance requirements:**

1. `MYBATIS-SQL-001` — 本次 UPDATE/DELETE WHERE 缺失/弱化；无明确 SQL 证据不报。
2. `MYBATIS-ISOLATION-001` — tenant/org/user 隔离条件删除/弱化；必须有 before/change evidence 或 verified contract evidence。
3. `MYBATIS-BIND-001` — 本次新增/扩大 `${}` 拼接风险；不能把所有 `${}` 一律当漏洞。
4. `MYBATIS-CONTRACT-001` — Mapper Java/XML statement/param/result contract mismatch；必须关联 verified resource relation。
5. `SPRING-TX-001` — transactional self-invocation；必须证明 caller/callee 同 Bean 且代理语义受影响。
6. `SPRING-TX-002` — checked exception rollback；必须证明异常路径和 rollback 默认行为冲突。
7. `SPRING-TX-003` — readOnly write path；必须证明事务 annotation + verified write path。
8. `SPRING-AUTH-001` — auth weakening；不得用固定注解名缺失作为唯一证据，必须基于 changed/new endpoint 与项目内 verified auth pattern/explicit evidence。
9. `SPRING-VALIDATION-001` — key input validation omission；必须能沿 verified chain 到达有风险操作，普通 DTO 无注解不能直接报。
10. `SPRING-CONFIG-001` — changed datasource/pool/timeout/retry/log-level/feature-switch；只审 changed key，不泛扫整个 YML。

**Noise rules remain forbidden:**

```text
命名
格式
缩进
重复代码
“建议重构”
普通测试代码风格
未变化配置
scope 外潜在问题
workspace dependency finding
```

- [ ] **Step 1: Write RED rule contract tests**

Tests must assert all 10 IDs exist, all prompts explicitly require current-change evidence, all rules forbid evidence-free certainty, and no style rule appears.

```go
func TestSpringV1ContainsExactTenHighValueRules(t *testing.T)
func TestSpringV1RulesRequireCurrentChangeEvidence(t *testing.T)
func TestSpringV1HasNoStyleOrNamingRules(t *testing.T)
func TestAuthRuleDoesNotRequireOneHardCodedAnnotationName(t *testing.T)
func TestValidationRuleRequiresVerifiedDangerousPath(t *testing.T)
```

- [ ] **Step 2: Run RED gate**

```powershell
cd .code-harness/tools-runtime
go test -count=1 ./internal/reviewrules ./internal/report
```

Expected: FAIL until prompts/contracts are complete.

- [ ] **Step 3: Finalize rule prompts and Reviewer guidance**

For each dispatched rule, Reviewer must output either zero or more proposals; it must not output “rule passed” as Finding. Evidence text stays Chinese, source identifiers remain original.

- [ ] **Step 4: Run GREEN gate**

```powershell
cd .code-harness/tools-runtime
go test -count=1 ./internal/reviewrules ./internal/report ./internal/finding
go test -count=1 ./...
go vet ./...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```text
git commit -m "feat: deepen spring review rules"
```

**Task 5 acceptance:** 10 条规则能被 deterministic dispatch，Agent guidance 强调高证据/低噪音，且没有把 matcher hit 直接等价为 bug。

---

### Task 6: 24-case Review Benchmark + 1.6 Quality Gate

**Objective:** 用固定 Spring 变更集证明 1.6 的 Review Precision，而不是靠单次人工观感。

**Files:**
- Create fixtures under: `.code-harness/tools-runtime/testdata/review-benchmark/`
- Create: `.code-harness/tools-runtime/internal/finding/benchmark_test.go`
- Create: `.github/workflows/task160-review-precision.yml`
- Create: `.github/scripts/task160-real-review-precision-regression.ps1`

**Benchmark layout:**

```text
review-benchmark/
  positive/
    01-mybatis-weak-where/
    02-mybatis-tenant-removed/
    03-mybatis-dollar-bind/
    04-mapper-contract-mismatch/
    05-tx-self-invocation/
    06-tx-checked-rollback/
    07-tx-readonly-write/
    08-auth-weakened/
    09-validation-omitted/
    10-dangerous-config/
    11-test-validity/
    12-cross-file-contract/
  negative/
    13-format-only/
    14-naming-only/
    15-test-naming/
    16-valid-dynamic-sql/
    17-valid-cross-bean-tx/
    18-valid-readonly-read/
    19-unchanged-config/
    20-dependency-context-only/
  contract/
    21-invalid-line-anchor/
    22-invented-symbol/
    23-semantic-duplicate/
    24-deterministic-repeat/
```

Each fixture must include checked-in input source/diff metadata and expected machine ground truth; no Internet/model call is required to run the deterministic benchmark gate.

**Metrics:**

```text
Precision >= 0.90
MustFindRecall >= 0.85
AnchorRate = 1.00
DuplicateRate = 0
DeterministicArtifactStability = 1.00
```

The deterministic benchmark is not allowed to fake Agent success by hardcoding final findings. Positive cases must exercise real ReviewUnit + Dispatch + proposal verification/certification fixtures; Agent proposal fixture is treated as model output input, while Runtime gates are real.

- [ ] **Step 1: Add 24 fixtures and RED benchmark assertions**

Create exact test names:

```go
func TestBenchmarkPositiveMustFindCases(t *testing.T)
func TestBenchmarkNegativeMustNotFindCases(t *testing.T)
func TestBenchmarkAnchorRateIsOne(t *testing.T)
func TestBenchmarkDuplicateRateIsZero(t *testing.T)
func TestBenchmarkRuntimeArtifactsAreDeterministic(t *testing.T)
```

- [ ] **Step 2: Run local benchmark RED/GREEN**

```powershell
cd .code-harness/tools-runtime
go test -count=1 ./internal/finding -run Benchmark
```

Expected after implementation: PASS and log metric summary containing all five metric names and numeric values.

- [ ] **Step 3: Add Windows workflow**

`task160-review-precision.yml` must run on `windows-latest` and execute:

```powershell
go test -count=1 ./internal/reviewunit
go test -count=1 ./internal/reviewrules
go test -count=1 ./internal/finding
go test -count=1 ./internal/report
go test -count=1 ./cmd/codea-harness-tools
go test -count=1 ./...
go vet ./...
```

- [ ] **Step 4: Add real Windows regression script**

`task160-real-review-precision-regression.ps1` must create a real temporary Spring/Maven-style repository and exercise real Windows `codea-harness-tools.exe` through:

```text
analysis certify
review units
review dispatch
review certify-findings
formal report render
```

Required sentinels:

```text
TASK160_REVIEW_UNIT_VERIFIED PASS
TASK160_RULE_DISPATCH_VERIFIED PASS
TASK160_INVALID_LINE_REJECTED PASS
TASK160_INVENTED_SYMBOL_REJECTED PASS
TASK160_DEPENDENCY_SCOPE_REJECTED PASS
TASK160_DUPLICATE_FINDING_COLLAPSED PASS
TASK160_CERTIFIED_FINDING_REPORT PASS
TASK160_REVIEW_PRECISION_REGRESSION PASS
```

- [ ] **Step 5: Run full gate**

```powershell
cd .code-harness/tools-runtime
go test -count=1 ./...
go vet ./...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```text
git commit -m "test: add review precision benchmark gate"
```

**Task 6 acceptance:** 仓库中存在固定、可重复、可度量的 Review Precision 基准；后续 Prompt/Rule/Runtime 修改都必须通过它。

---

### Task 7: 1.6 Release Integration & Regression Certification

**Objective:** 在不破坏 1.5.3 已验收行为的前提下，把 Review Precision 正式发布为 Codea Harness 1.6.0。

**Files:**
- Modify: `.code-harness/VERSION`
- Modify: `CHANGELOG.md`
- Modify: `.github/workflows/package-windows-x64.yml`
- Add only if needed for release proof: `.code-harness/tools-runtime/cmd/codea-harness-tools/task160_release_test.go`

**Version:**

```text
1.6.0
```

**Required regression gates before release:**

1. Existing 1.5.3 changed Controller/EntryPoint completeness gate.
2. Existing Certified ChangeAnalysis tamper/stale gate.
3. Existing Review Selection AUTO_FULL/AUTO_SINGLE/USER_SELECTION behavior.
4. Existing Chain authority/edit/persist gates.
5. Existing Workspace Dependency navigation + Review isolation gates.
6. Existing Test Validity behavior.
7. New ReviewUnit gate.
8. New RuleDispatch gate.
9. New Finding anchor/evidence/certification gate.
10. New 24-case benchmark gate.
11. Real Windows Task160 regression.
12. Formal Windows install/upgrade package validation.

**Real upgrade:**

Package workflow must use accepted 1.5.3 release commit `6f4c050783a7ec21f370799c1a8c69c9b51a9e92` as old baseline and perform real:

```text
1.5.3 -> 1.6.0
```

Upgrade must byte-preserve existing Project State exactly as 1.5.3 release gate already does:

```text
harness.yaml
project.md
database.yaml
runs/**
chains/**
```

The new framework-owned `.code-harness/review-rules/spring-v1.yaml` must be installed/upgraded as Framework State and must not be copied into Project State.

Installed 1.6 runtime capability probe must execute:

```text
codea-harness-tools.exe review units --run-id __missing__
```

and receive the expected required-run/input failure from the **new** command, proving the packaged runtime contains 1.6 Review Precision capability.

- [ ] **Step 1: Extend package workflow with Task160 gates before staging**

Formal ZIP must not be created until Task160 and all retained 1.5.3 regressions pass.

- [ ] **Step 2: Update VERSION and CHANGELOG**

CHANGELOG 1.6.0 section must state only:

- deterministic ReviewUnit;
- deterministic Spring Rule Dispatch;
- Finding Proposal -> Runtime Verified/Certified Finding;
- Spring Rule Pack v1 (10 high-value rules);
- 24-case Review Precision benchmark;
- 1.5.3 behavior preserved.

Do not claim CI/resume/doctor/new languages.

- [ ] **Step 3: Run full local Go gate**

```powershell
cd .code-harness/tools-runtime
go test -count=1 ./...
go vet ./...
```

Expected: PASS.

- [ ] **Step 4: Run/observe fresh exact-head Windows workflows**

Required success workflows at exact Task 7 HEAD:

```text
task160-review-precision
package-windows-x64
retained task153-chain-reliability
task3-chain-management
task4-review-chain-consumption
task152-review-isolation
task152-workspace-navigation
```

If workflow names are consolidated later, the release checklist must still show explicit PASS evidence for every retained gate above.

- [ ] **Step 5: Verify formal artifacts**

Required artifacts:

```text
codea-harness-1.6.0-windows-x64-install.zip
codea-harness-1.6.0-windows-x64-upgrade.zip
codea-harness-1.6.0-release-checklist
```

Release checklist records exact HEAD SHA and SHA256 of install/upgrade ZIPs.

- [ ] **Step 6: Commit**

```text
git commit -m "release: prepare Codea Harness 1.6.0"
```

**Task 7 acceptance:** Task 1-6 新能力 + 1.5.3 全部关键 regression + real Windows 1.5.3 -> 1.6.0 upgrade + formal package 全部通过后，才允许把该 HEAD 标记为 1.6 accepted baseline。

---

## Task dependency order

```text
Task 1 ReviewUnit
  -> Task 2 Rule Dispatch
    -> Task 3 Finding Proposal Verify
      -> Task 4 Certified Findings / Report Authority
        -> Task 5 Spring Rule Pack Deep Review
          -> Task 6 Benchmark / Windows Precision Gate
            -> Task 7 Release Certification
```

禁止并行实现 Task 3/4 来绕过 Task 1/2 的接口稳定；每个 Task 验收通过后再进入下一 Task。

## Final acceptance checklist

- [ ] Task 1: ReviewUnit deterministic / scope-safe / dependency-safe
- [ ] Task 2: RuleDispatch deterministic / role-evidence based
- [ ] Task 3: Proposal line/symbol/evidence/scope machine verified
- [ ] Task 4: formal report only consumes Certified Findings
- [ ] Task 5: 10 Spring high-value rules, no style noise
- [ ] Task 6: 24-case benchmark meets all fixed thresholds
- [ ] Task 7: 1.5.3 regressions + Windows package + real upgrade all PASS
- [ ] VERSION = `1.6.0`
- [ ] CHANGELOG matches actual shipped scope
- [ ] exact accepted HEAD recorded in release checklist

## Developer handoff rule

研发执行时，每次只开发一个 Task。提交后由 Reviewer 按该 Task 的 accepted baseline -> current HEAD 做独立验收；**不要因为后续 Task 的代码“顺便修好了”就跳过当前 Task 的验收，也不要在没有 concrete regression 的情况下重新打开已验收的 1.5.3 Task。**
