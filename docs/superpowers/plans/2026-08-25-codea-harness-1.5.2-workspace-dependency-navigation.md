# Codea Harness 1.5.2 Workspace Dependency Navigation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add safe, deterministic navigation through explicitly configured direct Maven sibling-source dependencies so Chain discovery can traverse superclass/template-method code and return to current-project overrides without expanding Change Set, Review Scope, or Write Scope.

**Architecture:** Introduce a focused `internal/workspace` package as the only authority for workspace dependency configuration and Maven source verification. Navigation receives only VERIFIED workspace roots and emits workspace-qualified evidence. ChangeAnalysis and Chain contracts carry logical workspace identity, while Coverage/Review/Apply remain current-repository-only. Release remains Windows-offline and preserves 1.5.1 Project State.

**Tech Stack:** Go 1.23 runtime module, JSON Schema Draft 2020-12, YAML v3, Maven POM XML parsing with Go stdlib, bundled ast-grep 0.42.1, GitHub Actions Windows x64.

**Spec:** `docs/superpowers/specs/2026-08-25-codea-harness-1.5.2-workspace-dependency-navigation-design.md`

## Global Constraints

- Base exactly on `develop@cd020fb319af95d28518b484593e2e0a0a03b7b5`.
- Preserve 1.5.1 direct Chain Discover Bootstrap behavior.
- `Workspace Dependency = Navigation Scope`, never Change Set / Review Scope / Write Scope.
- Only explicitly configured direct sibling Maven source dependency.
- No arbitrary `../**`, JAR decompile, Maven/Nexus/Central network resolution, dependency tests/writes/findings/runs, or generic cross-repo graph.
- Develop strictly Task 1 → 7; each Task independently commits after its RED→GREEN verification.
- Existing `harness.yaml version: 2` remains the config version; `workspaceDependencies` is optional.

---

### Task 1: Workspace Dependency Contract

**Files:**
- Modify: `.code-harness/contracts/harness-config.schema.json`
- Modify: `.code-harness/harness.template.yaml`
- Modify: `.code-harness/agents/project-adapter.md`
- Create: `.code-harness/tools-runtime/internal/workspace/config.go`
- Create: `.code-harness/tools-runtime/internal/workspace/config_test.go`
- Modify: `.code-harness/tools-runtime/cmd/codea-harness-tools/main.go`

**Interfaces:**
- Produces `workspace.Config`, `workspace.Dependency`, `workspace.MavenCoordinate`.
- Produces `workspace.ValidateConfigYAML(repoRoot string, data []byte) ([]workspace.Dependency, error)`.
- `validate --schema .code-harness/contracts/harness-config.schema.json --input .code-harness/harness.yaml` runs both schema and semantic workspace validation.

- [ ] **Step 1: Write RED tests for optional and valid contract**

Tests create a temporary parent directory with `order-service/` as repo root and `company-framework/` as a real sibling. Validate a version-2 harness config with no `workspaceDependencies`, then with:

```yaml
workspaceDependencies:
  - id: company-framework
    root: ../company-framework
    maven:
      groupId: com.company
      artifactId: company-framework
    mode: READ_ONLY
```

Expected: both valid.

- [ ] **Step 2: Write RED rejection matrix**

Table-driven tests must reject exact cases: duplicate id, duplicate canonical root, missing root, current repo root, non-sibling path, `mode: WRITE`, traversal to grandparent, symlink sibling escaping outside workspace parent, and UNC/network path.

- [ ] **Step 3: Run Task 1 RED**

Run:

```text
go test -count=1 ./internal/workspace ./cmd/codea-harness-tools
```

Expected: build/test failure because workspace contract/validator is absent.

- [ ] **Step 4: Add optional JSON Schema field**

Add `workspaceDependencies` array with `additionalProperties:false`; each item requires `id`, `root`, `maven`, `mode`; Maven requires `groupId` and `artifactId`; `mode` const `READ_ONLY`; ids use stable identifier pattern. Do not add it to root-level `required` and do not change config version semantics.

- [ ] **Step 5: Implement filesystem semantic validation**

`ValidateConfigYAML` parses only the new field, resolves repo and dependency roots to absolute/evaluated paths, rejects UNC/network roots, enforces dependency resolved parent equals current repo resolved parent, rejects current root, non-directory/missing roots, duplicate id/root, and symlink escape. It must not read dependency source files or POM content.

- [ ] **Step 6: Hook semantic validation into runtime config validate**

After successful YAML schema validation, when schema basename is `harness-config.schema.json`, call `workspace.ValidateConfigYAML(".", ib)` before returning VALID.

- [ ] **Step 7: Update template / Project Adapter boundary**

Template includes a commented workspace dependency example only. Project Adapter states it never auto-scans siblings and only writes this optional field from explicit user configuration after runtime validation.

- [ ] **Step 8: Run Task 1 GREEN**

Run:

```text
go test -count=1 ./internal/workspace ./cmd/codea-harness-tools
go test ./...
go vet ./...
```

Expected: all PASS.

- [ ] **Step 9: Commit Task 1**

Commit message:

```text
feat: add workspace dependency contract
```

---

### Task 2: Maven Dependency → Workspace Source Verification

**Files:**
- Create: `.code-harness/tools-runtime/internal/workspace/maven.go`
- Create: `.code-harness/tools-runtime/internal/workspace/maven_test.go`
- Modify: `.code-harness/tools-runtime/internal/workspace/config.go`
- Modify: `.code-harness/tools-runtime/cmd/codea-harness-tools/main.go`
- Add request/result schema only if CLI structured verification needs an explicit machine artifact.

**Interfaces:**
- Produce `workspace.VerificationStatus` constants: `VERIFIED`, `VERSION_UNRESOLVED`, `COORDINATE_MISMATCH`, `VERSION_MISMATCH`, `SOURCE_NOT_FOUND`.
- Produce `workspace.VerificationResult { DependencyID, Status, WorkspaceRoot, GroupID, ArtifactID, CurrentVersion, SourceVersion, Code }`.
- Produce `workspace.VerifyDirectMavenDependencies(repoRoot string, deps []Dependency) []VerificationResult`.
- Only result `VERIFIED` exposes a confirmed source root to later navigation.

- [ ] **Step 1: Write RED POM fixtures**

Cover direct dependency + sibling POM with matching literal version, `${property}`, local parent property, and local dependencyManagement. Also cover coordinate mismatch, version mismatch, unresolved version, source `pom.xml` missing, and dependency not declared directly by current project.

- [ ] **Step 2: Run Task 2 RED**

```text
go test -count=1 ./internal/workspace -run 'Maven|Verify'
```

Expected: FAIL because verifier is absent.

- [ ] **Step 3: Implement local-only POM model/resolution**

Use `encoding/xml`. Resolve only files already local: current POM, explicit sibling POM, and parent POM referenced by a relativePath that resolves inside the respective local project lineage. Resolve `${property}` from local model/parent properties plus `project.version`, `pom.version`, `project.groupId`, and `pom.groupId`. Resolve a dependency version from direct dependency first, then local dependencyManagement. Never execute Maven and never perform HTTP/network resolution.

- [ ] **Step 4: Enforce exact identity**

Configured group/artifact must match the current direct dependency and sibling effective coordinate. If versions are deterministically known, they must match exactly; mismatch is `VERSION_MISMATCH`. If a required version cannot be locally resolved, result is `VERSION_UNRESOLVED`, not VERIFIED.

- [ ] **Step 5: Emit machine limitation codes**

Map statuses to stable codes:

```text
WORKSPACE_DEPENDENCY_SOURCE_NOT_FOUND
WORKSPACE_DEPENDENCY_COORDINATE_MISMATCH
WORKSPACE_DEPENDENCY_VERSION_UNRESOLVED
WORKSPACE_DEPENDENCY_VERSION_MISMATCH
```

No fallback source is returned for non-VERIFIED states.

- [ ] **Step 6: Run Task 2 GREEN**

```text
go test -count=1 ./internal/workspace
go test ./...
go vet ./...
```

Expected: all PASS.

- [ ] **Step 7: Commit Task 2**

```text
feat: verify Maven workspace source identity
```

---

### Task 3: Cross-Workspace Inheritance Navigation

**Files:**
- Create: `.code-harness/tools-runtime/internal/nav/workspace.go`
- Create: `.code-harness/tools-runtime/internal/nav/workspace_test.go`
- Modify: `.code-harness/tools-runtime/internal/nav/nav.go`
- Modify: `.code-harness/tools-runtime/internal/nav/extended.go` only where needed to reuse existing symbol parsing/navigation.
- Modify: `.code-harness/tools-runtime/cmd/codea-harness-tools/main.go` for structured workspace navigation requests.

**Interfaces:**
- Consume only `workspace.VerificationResult{Status:VERIFIED}` roots.
- Produce workspace-qualified navigation facts: `{Workspace, Symbol, Path, Role, Source, From}`.
- Produce deterministic limitation `{Code, Symbol, From}` instead of guessed matches.

- [ ] **Step 1: RED inherited method test**

Fixture current subclass extends dependency `AbstractTemplate`; current `submit()` calls `execute()` and `super.execute()` variants. Expected resolved edge to dependency `AbstractTemplate.execute` only after VERIFIED source is supplied.

- [ ] **Step 2: RED superclass internal method test**

`AbstractTemplate.execute → validate` must resolve within dependency workspace.

- [ ] **Step 3: RED template override dispatch test**

`AbstractTemplate.execute → doExecute()` must dispatch back to the concrete current subclass override when current call context uniquely identifies the subclass, then continue to current Mapper.

- [ ] **Step 4: RED ambiguity test**

Two possible concrete overrides without unique current context must return `AMBIGUOUS_TEMPLATE_DISPATCH`; no arbitrary branch may be emitted.

- [ ] **Step 5: Implement verified workspace scope wrapper**

Existing `Navigator` continues rejecting `../` scope. New workspace navigation constructs a separate `Navigator{RepoRoot: verifiedRoot}` and passes only dependency-relative `src/main/java` scope. This preserves the existing path guard rather than weakening it.

- [ ] **Step 6: Implement inheritance resolver**

Resolve exact single `extends` owner, locate method declarations in the exact class, distinguish inherited/super call, and walk superclass-local calls. Template hook dispatch requires exact hook name + unique concrete override in the current invocation context. Emit the specified `SUPERCLASS_*`, `INHERITED_METHOD_*`, `TEMPLATE_*`, and ambiguity codes.

- [ ] **Step 7: Run Task 3 GREEN**

```text
go test -count=1 ./internal/nav
go test ./...
go vet ./...
```

Expected: all PASS.

- [ ] **Step 8: Commit Task 3**

```text
feat: navigate verified workspace inheritance
```

---

### Task 4: ChangeAnalysis + Chain Workspace Integration

**Files:**
- Modify: `.code-harness/contracts/change-analysis.schema.json`
- Modify: `.code-harness/contracts/chain.schema.json`
- Modify: `.code-harness/templates/chain.template.yaml`
- Modify: `.code-harness/tools-runtime/internal/chain/model.go`
- Modify: `.code-harness/tools-runtime/internal/chain/discover.go`
- Modify: `.code-harness/tools-runtime/internal/chain/canonicalize.go`
- Modify: `.code-harness/tools-runtime/internal/chain/validate.go` (or current chain validation implementation)
- Modify/add chain tests for workspace identity/backward compatibility.
- Modify coverage/reviewscope only to enforce current-workspace filtering where their existing assumptions would otherwise include dependency evidence.

**Interfaces:**
- `workspace` optional on symbol locations and Chain entry/node/resource/boundary; omitted means `current`.
- `WORKSPACE_INHERITANCE` becomes an allowed symbol-location source.
- Chain core signature normalizes missing workspace to `current` and includes workspace in ordered node identity.

- [ ] **Step 1: RED schema compatibility tests**

Old ChangeAnalysis/Chain without workspace remains valid. New evidence with `workspace: company-framework` and source `WORKSPACE_INHERITANCE` is valid. `changedFiles` stays current-repo-only and does not gain workspace field.

- [ ] **Step 2: RED canonicalization test**

Two chains differing only by `current/Foo.execute` versus `company-framework/Foo.execute` must not canonicalize together or share a deterministic discovered ID.

- [ ] **Step 3: RED discovery mixed-workspace test**

A confirmed call chain whose symbol locations alternate current → dependency → current builds a DISCOVERED Chain retaining exact workspace identities and dependency-relative paths.

- [ ] **Step 4: Implement contract/model normalization**

Add workspace field with `omitempty`; helper `normalizeWorkspace("") == "current"`. Keep Chain schema version 1 and no Project State migration.

- [ ] **Step 5: Update chain identity/discovery/validation**

Resolve symbols by `(workspace,symbol)` rather than symbol alone where workspace evidence exists. Core signatures and IDs use normalized workspace + symbol + path + role + order.

- [ ] **Step 6: Enforce Coverage boundary**

Coverage required set remains derived exclusively from `changedFiles`. TARGETED `scopedFiles` must include only current-workspace repository paths; workspace dependency evidence may inform call-chain selection but is filtered from file scope.

- [ ] **Step 7: Run Task 4 GREEN**

```text
go test -count=1 ./internal/chain ./internal/coverage ./internal/reviewscope
go test ./...
go vet ./...
```

Expected: all PASS.

- [ ] **Step 8: Commit Task 4**

```text
feat: add workspace identity to chain evidence
```

---

### Task 5: Real Dual-Project Business Regression

**Files:**
- Create: `.code-harness/tools-runtime/cmd/codea-harness-tools/workspace_chain_152_test.go`
- Modify bootstrap test helpers only if needed to reuse real Git fixture creation; never pre-write final Chain.

**Interfaces:**
- Direct user-intent acceptance driver starts from `harness chain discover XxxController` equivalent orchestration with no historical Chain and no prewritten discovered Chain.
- It may create ChangeAnalysis as the analyze-change stage output, but the test must prove the end-to-end driver generated it from the fixture rather than pre-seeding it.

- [ ] **Step 1: Build real workspace fixture**

Create sibling `company-framework` and `order-service` repositories/POMs. Current project working tree includes staged/unstaged/untracked business changes. Framework contains `AbstractTemplate`; current project contains Controller/Service/ServiceImpl override/Mapper/XML.

- [ ] **Step 2: RED direct discover**

From empty current `runs/**`/`chains/**`/ChangeAnalysis, execute the direct discover acceptance driver. Expected pre-implementation failure.

- [ ] **Step 3: Complete bootstrap integration**

Driver performs current Change Set analysis, config semantic validation, Maven workspace verification, verified inheritance navigation, ChangeAnalysis schema+coverage verification, then Chain discovery.

- [ ] **Step 4: Assert success invariants**

Change Set contains only `order-service`; Maven identity VERIFIED; dependency `AbstractTemplate.execute` found; override returns to `XxxServiceImpl.doExecute`; Mapper/XML present; discovery COMPLETE; one DISCOVERED YAML contains both workspaces; no dependency finding/write/run/state.

- [ ] **Step 5: Add four failure regressions**

Exact outcomes:

```text
VERSION_MISMATCH           → PARTIAL / WORKSPACE_DEPENDENCY_VERSION_MISMATCH
NOT_CONFIGURED             → PARTIAL / WORKSPACE_DEPENDENCY_NOT_CONFIGURED
AMBIGUOUS_OVERRIDE         → PARTIAL / AMBIGUOUS_TEMPLATE_DISPATCH
SOURCE_MISSING             → PARTIAL / WORKSPACE_DEPENDENCY_SOURCE_NOT_FOUND
```

No fallback guesses or historical-Chain guidance.

- [ ] **Step 6: Run Task 5 GREEN**

```text
go test -count=1 ./cmd/codea-harness-tools -run '152|Workspace'
go test ./...
go vet ./...
```

Expected: all PASS.

- [ ] **Step 7: Commit Task 5**

```text
test: add real workspace chain regression
```

---

### Task 6: Review Isolation Regression

**Files:**
- Create: `.code-harness/tools-runtime/internal/reviewscope/workspace_isolation_152_test.go`
- Create: `.code-harness/tools-runtime/internal/coverage/workspace_isolation_152_test.go`
- Extend apply path-policy tests only if required to prove `../dependency` remains rejected.
- Add reviewer/orchestrator contract regression only if dependency evidence could leak into prompt-visible Finding rules.

**Interfaces:**
- Workspace Chain may be consumed as business context.
- Review scope output contains current repository paths only.

- [ ] **Step 1: RED FULL/TARGETED isolation tests**

Use a mixed-workspace verified Chain/ChangeAnalysis. Assert FULL required coverage is only current changedFiles and TARGETED scopedFiles omit dependency workspace paths.

- [ ] **Step 2: RED finding/write isolation**

Assert dependency workspace cannot become Finding.file and any `../company-framework` Fix/Apply path remains rejected by existing apply path policy.

- [ ] **Step 3: Implement minimal filters/guards**

Filter by normalized workspace `current` at boundaries where file scopes/findings are materialized; never strip dependency nodes from business Chain context itself.

- [ ] **Step 4: Run Task 6 GREEN**

```text
go test -count=1 ./internal/coverage ./internal/reviewscope ./internal/apply ./internal/chain
go test ./...
go vet ./...
```

Expected: all PASS, including prior Task 1–4 Chain/Review tests.

- [ ] **Step 5: Commit Task 6**

```text
test: lock workspace review isolation
```

---

### Task 7: 1.5.2 Release Gate

**Files:**
- Modify: `.code-harness/VERSION`
- Modify: `CHANGELOG.md`
- Modify: `.github/workflows/package-windows-x64.yml`
- Modify/add `.code-harness/tools-runtime/internal/upgrade/release_152_contract_test.go`
- Update generic historical release tests only where they incorrectly bind the current version/artifact name.

**Interfaces:**
- VERSION exact `1.5.2`.
- Formal artifacts: `codea-harness-1.5.2-windows-x64-install` and `codea-harness-1.5.2-windows-x64-upgrade`.

- [ ] **Step 1: RED release contract**

Require VERSION/CHANGELOG/workflow tokens for 1.5.2, workspace/template/review-isolation gates, accepted 1.5.1 baseline `cd020fb319af95d28518b484593e2e0a0a03b7b5`, and formal install/upgrade artifact names.

- [ ] **Step 2: Update version/changelog**

Document Workspace Dependency Chain Navigation, strict Navigation-only boundary, Maven version safety, template dispatch ambiguity behavior, and no harness config migration.

- [ ] **Step 3: Extend Windows formal release workflow**

Run full Go test/vet, Windows build, real ast-grep smoke, workspace identity regression, template inheritance regression, Chain regression, Review isolation regression, package assembly, and real accepted `1.5.1 → 1.5.2` live upgrade.

- [ ] **Step 4: Live-upgrade Project State assertions**

Inject representative `harness.yaml` without workspaceDependencies, `project.md`, `database.yaml`, `runs/**`, `chains/**`; SHA256 compare byte-for-byte after upgrade. Upgrade package must not inject workspaceDependencies into existing config.

- [ ] **Step 5: Build formal artifacts**

Exact ZIP names:

```text
codea-harness-1.5.2-windows-x64-install.zip
codea-harness-1.5.2-windows-x64-upgrade.zip
```

- [ ] **Step 6: Run full verification**

```text
go test ./...
go vet ./...
```

Then require exact-head GitHub Actions Windows gate success and inspect every required step/artifact digest.

- [ ] **Step 7: Commit Task 7**

```text
feat: release Codea Harness 1.5.2
```

- [ ] **Step 8: Final acceptance review**

Re-check G1–G8, PR diff scope, exact HEAD, all CI conclusions, Windows artifact IDs/digests, and confirm no dependency workspace writes/findings/review scope expansion.