# Codea Harness 1.4 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在保持 1.3.2 已验收能力不退化的前提下，交付 Targeted Review、Mapper.xml/YML Review、统一 Human Report UX 和 Runtime Apply Safety，并完成 1.4.0 Windows Release Gate。

**Architecture:** 继续沿用 Agent/Skill/Contract/Controlled Runtime 四层。Targeted Review 通过新的 Review Scope Contract 把“完整 Change Set”与“本次定向 Scope”分离；Resource Review 把 Mapper/YML 纳入 ChangeAnalysis；正式写入通过新的 Runtime apply 子命令强制 diff/hash/path 一致性；所有人读 Markdown 遵守统一 UX 标准。

**Tech Stack:** Go、JSON Schema Draft 2020-12、YAML、ast-grep 0.42.1、Vitess SQL Parser、Windows x64、Java/Spring Boot/Maven。

**Spec:** `docs/superpowers/specs/2026-08-21-codea-harness-1.4-design.md`

## Global Constraints

- Baseline: Codea Harness 1.3.2 `4eb44a2d8bfd7d2f7825815df2d06c49c0c5e48b`。
- 1.4.0 只支持 Windows x64。
- 不做 Maven Doctor/offline 自动适配、Linux/macOS、Gradle、JDT LS、JaCoCo、PIT、SARIF。
- Resource Review 只新增 `*Mapper.xml` 与 `src/main/resources/**/*.yml`。
- `harness review` 默认 FULL，必须保持 1.3.2 语义。
- TARGETED Review 不能声称整个 Change Set 已完整评审。
- 不允许 sampled review 进入 COMPLETE/PASSED。
- Database Evidence、Existing Test 保护、method-level provenance、Upgrade Transaction 均不得退化。
- 所有正式写入必须最终经过 Runtime Apply Gate；Agent/Skill 不得继续把宿主自由写文件视为正式成功路径。
- 每个 Task 必须先写失败测试，再实现，再运行 targeted tests + `go test -count=1 ./...`。

---

## File Structure Map

### Existing files expected to change

- `.code-harness/agents/orchestrator.md` — 路由 FULL/TARGETED/list，并切换正式 apply 流程。
- `.code-harness/agents/reviewer.md` — Targeted Review / Resource Review 规则。
- `.code-harness/skills/analyze-change/SKILL.md` — 资源文件读取、target scope、call-chain relation。
- `.code-harness/skills/review-code/SKILL.md` — Mapper/YML Finding Scope 与定向 Review 限制。
- `.code-harness/contracts/change-analysis.schema.json` — 文件角色与资源文件 evidence。
- `.code-harness/contracts/harness-config.schema.json` — `mapperIncludes` / `configIncludes`。
- `.code-harness/harness.template.yaml` — 新默认 scope。
- `.code-harness/tools-runtime/cmd/codea-harness-tools/main.go` — 新 `apply` 子命令。
- `.code-harness/tools-runtime/internal/coverage/*` — FULL/TARGETED scoped coverage 机器验证。
- `.code-harness/tools-runtime/internal/report/review.go` — UX 标准、target 信息、下一步。
- `.code-harness/tools-runtime/internal/report/*_test.go` — Golden/UX 回归。
- `.code-harness/tools-runtime/internal/upgrade/*` — 1.3.2→1.4 migration/replace tests。
- `.github/workflows/package-windows-x64.yml` — 1.4 release gate。
- `.code-harness/VERSION`、`README.md`、`CHANGELOG.md` — release metadata。

### New files/packages

- `.code-harness/contracts/review-scope.schema.json`
- `.code-harness/contracts/apply-request.schema.json`
- `.code-harness/contracts/apply-result.schema.json`
- `.code-harness/tools-runtime/internal/reviewscope/` — target resolution transport verification / scoped coverage helpers。
- `.code-harness/tools-runtime/internal/apply/` — patch/hash/path/transaction/evidence。
- `.code-harness/tools-runtime/internal/resource/` — Mapper/YML deterministic candidate helpers if implementation needs a Runtime helper boundary。

---

### Task 1: Targeted Review

**Files:**
- Create: `.code-harness/contracts/review-scope.schema.json`
- Create: `.code-harness/tools-runtime/internal/reviewscope/reviewscope.go`
- Create: `.code-harness/tools-runtime/internal/reviewscope/reviewscope_test.go`
- Modify: `.code-harness/contracts/change-analysis.schema.json`
- Modify: `.code-harness/tools-runtime/internal/coverage/coverage.go`
- Modify: `.code-harness/tools-runtime/internal/coverage/coverage_test.go`
- Modify: `.code-harness/agents/orchestrator.md`
- Modify: `.code-harness/agents/reviewer.md`
- Modify: `.code-harness/skills/analyze-change/SKILL.md`
- Modify: `.code-harness/tools-runtime/internal/report/review.go`
- Test: `.code-harness/tools-runtime/internal/report/review_test.go`

**Interfaces:**
- Consumes: existing `ChangeAnalysis.callChains[]`, `changedFiles[]`, `reviewCoverage.reviewedFiles[]`。
- Produces: `ReviewScopeSelection` with `mode`, optional `target`, `selectedCallChains`, `scopedFiles`；machine verifier `Verify(selection, changeAnalysis)`。

- [ ] **Step 1: Write failing Schema tests for FULL/TARGETED Review Scope**

Add tests that accept:

```json
{
  "mode": "TARGETED",
  "target": {"symbol":"OrderController.approve","kind":"METHOD"},
  "selectedCallChains":[{"entryPoint":"OrderController.approve","chain":["OrderController.approve","OrderService.approve"]}],
  "scopedFiles":["src/main/java/OrderController.java","src/main/java/OrderService.java"]
}
```

and reject TARGETED without `target`, unknown kind, empty scopedFiles, unknown fields.

- [ ] **Step 2: Run the new schema tests and confirm FAIL**

Run from `.code-harness/tools-runtime`:

```powershell
go test -count=1 ./internal/schema
```

Expected: FAIL because `review-scope.schema.json` does not exist / contract not implemented.

- [ ] **Step 3: Add `review-scope.schema.json`**

Contract requirements:

```text
mode = FULL | TARGETED
FULL: target must be absent/null; selectedCallChains/scopedFiles may represent full scope
TARGETED: target required
kind = CLASS | METHOD
selectedCallChains >= 1
scopedFiles >= 1
additionalProperties = false
```

- [ ] **Step 4: Write failing machine verification tests**

Cover:

1. selected chain not present in validated ChangeAnalysis → reject.
2. scoped file not justified by selected chain / changed target relation → reject.
3. TARGETED reviewedFiles missing one scoped file → coverage PARTIAL.
4. unrelated changed file omitted from TARGETED → allowed, but not counted as reviewed.
5. FULL still requires every changed file.

- [ ] **Step 5: Implement `internal/reviewscope.Verify` and scoped coverage calculation**

Expected conceptual API:

```go
type Selection struct {
    Mode string
    Target *Target
    SelectedCallChains []CallChain
    ScopedFiles []string
}

func Verify(selectionJSON, changeAnalysisJSON []byte) (Selection, error)
func ComputeCoverage(selection Selection, reviewedFiles []string) CoverageResult
```

Do not trust Agent-declared COMPLETE.

- [ ] **Step 6: Update Orchestrator/Reviewer/Analyze Change intent rules**

Document exact intents:

```text
harness review                -> FULL
harness review list           -> list chains only
harness review Class          -> TARGETED CLASS
harness review Class.method   -> TARGETED METHOD
```

For Service target resolving to 2+ call chains, require host multi-select or numbered fallback; never default ALL.

- [ ] **Step 7: Update Review report transport/renderer for mode/target**

Header must show:

```text
评审模式：完整评审 / 定向评审
评审目标：<symbol>（TARGETED only）
Change Set 文件：N
本次 Scope 文件：M
```

TARGETED summary must include:

```text
本结论只覆盖本次定向评审范围，不代表整个 Change Set 已完成评审。
```

- [ ] **Step 8: Add golden tests**

Required cases:

- FULL unchanged behavior.
- TARGETED one chain.
- TARGETED class with multiple methods.
- Service target multiple chains → selection required contract text present.
- TARGETED PARTIAL when a scoped file is missing.
- `review list` shows confirmed chains and separates unresolved/candidate entries.

- [ ] **Step 9: Run tests**

```powershell
go test -count=1 ./internal/reviewscope ./internal/coverage ./internal/report ./internal/schema
go test -count=1 ./...
```

Expected: PASS.

- [ ] **Step 10: Commit**

```bash
git add .code-harness
 git commit -m "feat: add targeted review scopes"
```

**Task 1 Acceptance Gate:** FULL semantics unchanged; TARGETED coverage is machine verified; report cannot imply full Change Set coverage.

---

### Task 2: Mapper.xml and YML Resource Review

**Files:**
- Modify: `.code-harness/harness.template.yaml`
- Modify: `.code-harness/contracts/harness-config.schema.json`
- Modify: `.code-harness/contracts/change-analysis.schema.json`
- Modify: `.code-harness/agents/reviewer.md`
- Modify: `.code-harness/skills/analyze-change/SKILL.md`
- Modify: `.code-harness/skills/review-code/SKILL.md`
- Create if needed: `.code-harness/tools-runtime/internal/resource/resource.go`
- Create if needed: `.code-harness/tools-runtime/internal/resource/resource_test.go`
- Modify: `.code-harness/tools-runtime/internal/coverage/coverage_test.go`

**Interfaces:**
- Consumes: Review Change Set and Targeted Review `scopedFiles`.
- Produces: changed/reviewed resource files with roles `MapperXml` and `YamlConfig`; evidence-backed Findings using existing review-output schema.

- [ ] **Step 1: Write failing harness config schema tests**

Require:

```yaml
scope:
  sourceIncludes: [src/main/java/**/*.java]
  testIncludes: [src/test/java/**/*.java]
  mapperIncludes: [src/main/resources/**/*Mapper.xml]
  configIncludes: [src/main/resources/**/*.yml]
```

Reject missing new fields only after 1.4 migration/schema version is active.

- [ ] **Step 2: Update template and schema**

Add only `mapperIncludes` and `configIncludes`; do not add SQL migration/pom/Gradle patterns.

- [ ] **Step 3: Add ChangeAnalysis roles**

Extend `fileRole` with explicit resource roles, e.g.:

```text
MapperXml
YamlConfig
```

Do not overload `Other` for the new official scope.

- [ ] **Step 4: Write failing FULL coverage tests**

Cases:

- changed Mapper.xml not reviewed → PARTIAL.
- changed yml not reviewed → PARTIAL.
- both read → COMPLETE when all other rules pass.

- [ ] **Step 5: Write failing Targeted resource relation tests**

Cases:

- selected chain contains `OrderMapper.updateStatus` and changed `OrderMapper.xml#updateStatus` → Mapper.xml included in scopedFiles.
- unrelated changed `UserMapper.xml` → excluded from TARGETED and explicitly counted as outside scope.
- YML only enters TARGETED if there is evidence relation to target (for example changed property consumed by a target-chain class); otherwise stays outside scope.

- [ ] **Step 6: Add Reviewer/Skill rules for Mapper.xml**

Allow Findings only for high-value change-related risks:

```text
UPDATE/DELETE missing or weakened WHERE
removed/weakened tenant/org/user isolation predicate
dynamic SQL weakening a required filter
statement id / Mapper method mismatch
parameter mismatch
resultMap/resultType mismatch caused by current change
unbounded batch update/delete
```

Explicitly forbid XML style/indent/name nitpicks.

- [ ] **Step 7: Add Reviewer/Skill rules for YML**

Focus on changed keys affecting:

```text
datasource/pool
timeout/thread/queue
Redis/MQ/RPC
log level
profiles/feature switches
hard-coded secrets
@Value / @ConfigurationProperties key rename/delete mismatch
```

Do not review unchanged config globally.

- [ ] **Step 8: Add deterministic candidate helpers only where reliable**

If `internal/resource` is created, keep it bounded:

```go
type MapperCandidate struct {
    StatementID string
    Operation string
    Risk string
    Evidence string
}
```

Use XML parsing and existing SQL parser where SQL can be deterministically extracted. Dynamic SQL that cannot be normalized must return `UNKNOWN/CANDIDATE`, not a fake confirmed issue.

- [ ] **Step 9: Run tests**

```powershell
go test -count=1 ./internal/schema ./internal/coverage ./internal/resource
go test -count=1 ./...
```

If `internal/resource` is not created, omit that package.

- [ ] **Step 10: Commit**

```bash
git add .code-harness
 git commit -m "feat: review mapper xml and yaml resources"
```

**Task 2 Acceptance Gate:** FULL Review cannot silently skip changed Mapper/YML; Targeted Review only includes evidence-related resource files; style-only resource findings are forbidden.

---

### Task 3: Human Report UX Standard

**Files:**
- Modify: `.code-harness/tools-runtime/internal/report/review.go`
- Modify: `.code-harness/tools-runtime/internal/report/review_test.go`
- Create: `.code-harness/tools-runtime/internal/report/review_14_ux_test.go`
- Modify relevant human-facing Agent/Skill summaries: `.code-harness/agents/orchestrator.md`, `.code-harness/agents/runtime-debugger.md` if present
- Modify: `README.md` if screenshots/examples are documented as text

**Interfaces:**
- Consumes: normalized Review report request from Tasks 1–2.
- Produces: deterministic Chinese Markdown with unified first-screen summary, call-chain roles, Finding blocks, next-step block.

- [ ] **Step 1: Write failing UX golden tests**

Lock the first-screen table for FULL/TARGETED/PARTIAL.

TARGETED expected fragments:

```markdown
# 🔍 代码评审报告
| 评审模式 | 🎯 定向评审 |
| 评审目标 | `OrderController.approve` |
```

- [ ] **Step 2: Add fixed Chinese UI test**

Forbid fixed UI strings such as:

```text
Review Scope
Review Coverage
Summary
Evidence
Manual Action Required
```

Code/third-party names remain allowed.

- [ ] **Step 3: Add call-chain role presentation**

Runtime maps known roles for display only:

```text
Controller -> 🌐 接口入口
Service interface -> ⚙️ 业务接口
ServiceImpl -> 🧠 业务实现
Repository/Mapper -> 🗄 数据访问
MapperXml -> 📄 Mapper XML
```

Unknown role falls back to `🔹 代码节点`; never invent a machine symbol.

- [ ] **Step 4: Standardize Finding block**

Render:

```markdown
### 🟠 F-001｜高

📍 **位置**
...

❗ **问题**
...

🔎 **证据**
...

💥 **影响**
...

🛠 **修复建议**
...

🧪 **是否需要测试**
是
```

Keep deterministic severity/file/line/id sorting.

- [ ] **Step 5: Add Next Step block**

Rules:

- FAILED → `下一步：优先处理阻断问题；可使用 harness fix finding:<id>`.
- PASSED → `下一步：无需处理阻断问题。`
- MANUAL_ACTION_REQUIRED → exact unresolved/missing action.
- TARGETED → scope disclaimer always present.

- [ ] **Step 6: Verify deterministic rendering**

Render same normalized request twice and require byte-for-byte equality.

- [ ] **Step 7: Run tests**

```powershell
go test -count=1 ./internal/report
go test -count=1 ./...
```

- [ ] **Step 8: Commit**

```bash
git add .code-harness README.md
 git commit -m "feat: standardize human report ux"
```

**Task 3 Acceptance Gate:** first screen communicates result/scope/problem count/next action; all fixed UI is Chinese; FULL/TARGETED/PARTIAL golden reports pass.

---

### Task 4: Runtime Apply Safety Gate

**Files:**
- Create: `.code-harness/contracts/apply-request.schema.json`
- Create: `.code-harness/contracts/apply-result.schema.json`
- Create: `.code-harness/tools-runtime/internal/apply/apply.go`
- Create: `.code-harness/tools-runtime/internal/apply/apply_test.go`
- Create: `.code-harness/tools-runtime/internal/apply/path_policy.go`
- Create: `.code-harness/tools-runtime/internal/apply/patch.go`
- Modify: `.code-harness/tools-runtime/cmd/codea-harness-tools/main.go`
- Modify: `.code-harness/agents/orchestrator.md`
- Modify: `.code-harness/agents/fix-agent.md` if present
- Modify test-generation/fix Skills that currently imply direct host writes
- Modify: `.code-harness/contracts/fix-plan.schema.json`
- Modify test plan contract if one exists and needs `unifiedDiff/diffSha256/baseSha256`

**Interfaces:**
- Consumes: `apply-request.json` under `.code-harness/runs/<runId>/requests/` and `harness.yaml.write` path policy.
- Produces: atomic file changes and `.code-harness/runs/<runId>/evidence/apply/<planId>.json` validated by `apply-result.schema.json`.

- [ ] **Step 1: Write failing request/result schema tests**

Request minimum:

```json
{
  "runId":"run-1",
  "planType":"FIX",
  "planId":"fix-1",
  "diffSha256":"<sha256>",
  "files":[{"path":"src/main/java/A.java","baseSha256":"<sha256>"}],
  "unifiedDiff":"--- a/src/main/java/A.java\n+++ b/src/main/java/A.java\n..."
}
```

Reject unknown fields, empty diff, duplicate file paths, invalid planType.

- [ ] **Step 2: Write failing hash/path safety tests**

Cases:

1. diff hash mismatch → 0 writes.
2. baseSha mismatch → `BASE_CHANGED`, 0 writes.
3. FIX touches test path → reject.
4. TEST touches production path → reject.
5. deniedPaths overrides allowlist.
6. patch includes file not declared in files[] → reject.
7. path traversal / absolute path → reject.
8. Framework Managed `.code-harness/agents/**` etc. → reject.

- [ ] **Step 3: Implement path policy loader**

Read `harness.yaml` using existing schema/YAML parser. Apply glob semantics consistently with current config. Deny takes precedence.

Expected API:

```go
type Policy struct {
    AllowedTest []string
    AllowedProduction []string
    Denied []string
}

func (p Policy) Allow(planType, path string) error
```

- [ ] **Step 4: Implement diff normalization and validation**

Requirements:

- SHA256 calculated over exact UTF-8 unifiedDiff bytes from request.
- Parse touched paths; no rename outside allowed policy.
- touched paths set == declared files[] set.
- no binary patch in 1.4 unless explicitly implemented and tested; safest default is reject.

- [ ] **Step 5: Write failing atomic multi-file apply test**

Scenario: first file patch valid, second file intentionally fails. Expected: both original files remain byte-identical and result says `rollbackPerformed=true`.

- [ ] **Step 6: Implement transactional apply**

Use temp/stage/backup under run-scoped or OS temp location. Sequence:

```text
validate all inputs
read and hash all originals
stage all outputs
verify staged hashes/paths
replace files
on any failure restore all originals
```

Never partially commit a multi-file patch.

- [ ] **Step 7: Add evidence and duplicate-plan protection**

Success evidence:

```json
{
  "runId":"run-1",
  "planType":"FIX",
  "planId":"fix-1",
  "diffSha256":"...",
  "status":"APPLIED",
  "files":[{"path":"...","beforeSha256":"...","afterSha256":"..."}],
  "rollbackPerformed":false
}
```

If success evidence already exists for same planId, reject repeat apply.

- [ ] **Step 8: Wire CLI**

Add:

```text
codea-harness-tools apply --input .code-harness/runs/<runId>/requests/apply.json
```

`main.go` usage becomes:

```text
<upgrade|validate|nav|db|report|apply>
```

Input must stay under correct run request directory; no patch CLI args.

- [ ] **Step 9: Update Fix/Test Plans and Agent/Skill contracts**

Fix Plan must carry the exact approved patch identity:

```text
unifiedDiff
diffSha256
files[].baseSha256
```

Plan mutation changes diffSha256 and invalidates prior approval. Remove any statement that host `write_file` alone constitutes completed formal apply.

- [ ] **Step 10: Run tests**

```powershell
go test -count=1 ./internal/apply ./cmd/codea-harness-tools ./internal/schema
go test -count=1 ./...
go vet ./...
```

- [ ] **Step 11: Commit**

```bash
git add .code-harness
 git commit -m "feat: enforce runtime apply safety gate"
```

**Task 4 Acceptance Gate:** approved diff identity, base hashes and write path policy are Runtime enforced; multi-file apply is atomic/rollback-safe; direct host write is no longer the formal success path.

---

### Task 5: 1.4 Release / Upgrade / Windows Gate

**Files:**
- Modify: `.code-harness/VERSION`
- Modify: `.code-harness/tools-runtime/internal/upgrade/*`
- Modify: `.github/workflows/package-windows-x64.yml`
- Modify: `CHANGELOG.md`
- Modify: `README.md`
- Test: existing upgrade/release suites plus new 1.4 golden cases

**Interfaces:**
- Consumes: Tasks 1–4 accepted commits.
- Produces: exact-head formal install/upgrade artifacts for 1.4.0.

- [ ] **Step 1: Add failing 1.3.2→1.4 migration test**

Verify old config with only `sourceIncludes/testIncludes` is migrated to include:

```yaml
mapperIncludes:
  - src/main/resources/**/*Mapper.xml
configIncludes:
  - src/main/resources/**/*.yml
```

Existing user values remain byte/semantic preserved except registered migration edits.

- [ ] **Step 2: Add upgrade preservation assertions**

Must remain unchanged:

```text
harness.yaml user custom values except registered migration
project.md
database.yaml
runs/**
```

New contracts/runtime files installed; stale framework removed.

- [ ] **Step 3: Bump version to 1.4.0 and update changelog/readme**

Document only shipped features:

- Targeted Review
- Mapper.xml/YML Review
- Human Report UX Standard
- Runtime Apply Safety

Do not advertise Doctor/Linux/JDT LS/etc.

- [ ] **Step 4: Extend Windows workflow targeted suites**

Add explicit commands:

```powershell
go test -count=1 ./internal/reviewscope ./internal/coverage ./internal/report
go test -count=1 ./internal/apply ./internal/schema ./internal/upgrade
go test -count=1 ./...
go vet ./...
```

Keep real ast-grep smoke and package layout checks.

- [ ] **Step 5: Add live upgrade scenario**

Run 1.3.2 → 1.4.0 on Windows, verify:

```text
VERSION=1.4.0
new review/apply contracts present
runtime contains apply subcommand
project state preserved
upgrade source cleaned
stage/backup cleaned
runtime exe replaced
```

- [ ] **Step 6: Build and validate formal ZIPs**

Install root must be `.code-harness/`; upgrade root `.code-harness-upgrade/`; no `harness.yaml/project.md/database.yaml/runs` in release packages.

- [ ] **Step 7: Run final exact-head gate**

Do not accept a prior SHA's green run. Required final evidence:

```text
develop HEAD SHA
workflow run ID
status=completed
conclusion=success
job/step results
install artifact ID + digest
upgrade artifact ID + digest
```

- [ ] **Step 8: Commit release**

```bash
git add .
 git commit -m "release: complete Codea Harness 1.4.0 gate"
```

**Task 5 Acceptance Gate:** all exact-head tests green, Windows live upgrade passes, both formal artifacts generated from the accepted 1.4 HEAD.

---

## Final Acceptance Checklist

- [ ] `harness review` remains FULL and regression-free.
- [ ] `harness review list` lists current-change call chains without Findings.
- [ ] `harness review <Class>` and `<Class.method>` produce scoped, machine-verifiable TARGETED review.
- [ ] Multi-chain Service target never defaults ALL.
- [ ] Mapper.xml and YML changed files participate in FULL coverage.
- [ ] Targeted resource files require evidence relation to target.
- [ ] Reports are Chinese-first, readable, deterministic, and include next action.
- [ ] Targeted report explicitly says it does not cover the full Change Set.
- [ ] Runtime Apply enforces diff hash/base hash/path policy/atomic rollback.
- [ ] 1.3.2 → 1.4.0 upgrade preserves Project State.
- [ ] Exact-head Windows Release Gate succeeds and artifacts are locked to the accepted SHA.
