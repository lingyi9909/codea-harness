# Codea Harness 1.6.3 Runtime Usability Hotfix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 Workspace AST 超时、clean project Chain discover、multi-Chain Review 用户选择缺失和 Upgrade 全量替换四个真实可用性问题。

**Architecture:** 保留 1.6.2 Review Authority 主链；AST 通过 candidate-file narrowing 降低扫描成本；public Chain discovery 新增 source/project discovery 路径，与 Review affected discovery 分离；multi-Chain 用 active Agent hard-stop + real OpenCode two-turn E2E 锁定；Upgrade 保留 backup/stage/rollback 但 apply 改为 hash-aware delta，并移除用户 payload 中的 tools-runtime 源码。

**Tech Stack:** Go 1.23+, PowerShell, Python 3.12, OpenCode 1.18.25, ast-grep 0.42.1, GitHub Actions Windows runners.

**Spec:** `docs/superpowers/specs/2026-09-06-codea-harness-1.6.3-runtime-usability-hotfix-design.md`

## Global Constraints

- 基线必须是 `c81f19cc2fab6925bd278c75b594006ce585f581`。
- 分支必须是 `hotfix/1.6.3-runtime-usability`。
- 不改变 1.6.2 Canonical ChangeSet / ChangeAnalysis Certification / Certified Findings / Report Authority。
- 不新增联网运行依赖。
- 不自动写 `.code-harness/chains/**`。
- `USER_SELECTION` 不得默认 FULL。
- Upgrade 必须保留 backup/schema-validation/rollback。
- 最终版本为 `1.6.3`，只在四项 focused tests 都 GREEN 后 bump。

---

### Task 1: Workspace AST Candidate-File Navigation

**Files:**
- Modify: `.code-harness/tools-runtime/internal/nav/workspace_ast.go`
- Modify if required: `.code-harness/tools-runtime/internal/nav/workspace.go`
- Create: `.code-harness/tools-runtime/internal/nav/workspace_performance_163_test.go`

**Interfaces:**
- Consumes: existing `Navigator`, `Runner`, `workspaceRawMatch`, `workspaceTypeMatch`, `workspaceMethodMatch`.
- Produces: candidate-file helpers used only inside nav; public Navigator method signatures remain unchanged.

- [ ] **Step 1: Write RED large-file/bounded-scan tests**

Create tests with a fake Runner and a 2000-line Java target plus decoy files. Assert `WorkspaceMethod`, `WorkspaceMethodCalls`, and subclass navigation do not pass the whole `src/main/java` directory to ast-grep after a concrete candidate file is known.

- [ ] **Step 2: Run focused RED**

Run:

```bash
cd .code-harness/tools-runtime
go test -count=1 -run 'Test163Workspace' -v ./internal/nav
```

Expected: FAIL because current implementation scans `src/main/java` for every pattern.

- [ ] **Step 3: Implement candidate-file narrowing**

Add deterministic Java file enumeration/prefilter helpers. Text prefilter may only select candidate files; all semantic matches still come from ast-grep. Add a scoped runner helper that receives concrete file paths. Preserve full-scan fallback if no safe candidate set can be established.

- [ ] **Step 4: Run focused GREEN + nav regression**

```bash
cd .code-harness/tools-runtime
go test -count=1 -run 'Test163Workspace' -v ./internal/nav
go test -count=1 ./internal/nav
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add .code-harness/tools-runtime/internal/nav
git commit -m "perf: narrow workspace ast scans"
```

---

### Task 2: Clean-Project Chain Discovery

**Files:**
- Create: `.code-harness/tools-runtime/internal/chain/project_discover_163.go`
- Create: `.code-harness/tools-runtime/internal/chain/project_discover_163_test.go`
- Modify: `.code-harness/tools-runtime/cmd/codea-dcep-tools/chain_command.go`
- Create: `.code-harness/tools-runtime/cmd/codea-dcep-tools/chain_project_discover_163_test.go`
- Modify: `.code-harness/skills/discover-chain/SKILL.md`
- Modify: `.code-harness/agents/orchestrator.md`
- Modify: `.code-harness/agents/reviewer.md`
- Modify/replace affected 1.5.1 bootstrap contract assertions in `.code-harness/tools-runtime/cmd/codea-dcep-tools/chain_discover_bootstrap_151_test.go`

**Interfaces:**
- Existing `chain.Discover(DiscoverInput)` remains affected/ChangeAnalysis discovery for Review.
- Add a separate project-source discovery API and Runtime command/request shape; public `harness chain discover [target]` uses it.
- Output remains `DiscoveryResult` and run-scoped Runtime-certified `DISCOVERED` candidates.

- [ ] **Step 1: Write RED clean-project tests**

Fixture requirements:

```text
main == HEAD
working tree clean
Controller -> Service -> Impl -> Mapper -> Mapper XML
```

Assert public project discovery returns a DISCOVERED candidate, writes only under current run, and does not require non-empty ChangeSet.

- [ ] **Step 2: Run focused RED**

```bash
cd .code-harness/tools-runtime
go test -count=1 -run 'Test163.*ProjectDiscover|Test151ChainDiscoverBootstrapContractIsSelfContained' -v ./cmd/codea-dcep-tools ./internal/chain
```

Expected: FAIL because current public bootstrap requires `current Change Set`.

- [ ] **Step 3: Implement separate project-source discovery**

Use deterministic source navigation. Do not synthesize `analysis/change-set.json`. Keep candidates in `runs/<runId>/analysis/discovered-chains/**` and certify exact bytes/source provenance before they can enter any persistence path.

- [ ] **Step 4: Update active Agent/Skill contract**

Document exactly:

```text
harness chain discover [target] = PROJECT DISCOVERY
review lazy discovery = AFFECTED DISCOVERY
chain refresh = EXISTING CHAIN REFRESH
```

Remove public discover wording that requires a non-empty current Change Set.

- [ ] **Step 5: Run GREEN + chain regression**

```bash
cd .code-harness/tools-runtime
go test -count=1 -run 'Test163.*ProjectDiscover|Test151ChainDiscoverBootstrapContractIsSelfContained' -v ./cmd/codea-dcep-tools ./internal/chain
go test -count=1 ./internal/chain ./cmd/codea-dcep-tools
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add .code-harness/tools-runtime .code-harness/skills/discover-chain .code-harness/agents
git commit -m "feat: discover chains from clean project source"
```

---

### Task 3: Multi-Chain Review User-Selection Hard Stop

**Files:**
- Modify: `.code-harness/AGENTS.md`
- Modify: `.code-harness/tools/README.md`
- Modify: `.code-harness/agents/orchestrator.md`
- Modify: `.code-harness/agents/reviewer.md`
- Create: `.github/scripts/task163-review-user-selection-contract.ps1`
- Create: `.github/scripts/task163-review-user-selection-real-agent-e2e.ps1`
- Create or extend: `.github/scripts/task163-review-user-selection/mock_openai_server.py`

**Interfaces:**
- Runtime `review options` and `review select` schemas remain compatible.
- `decision=USER_SELECTION` becomes an Agent invocation hard-stop contract.

- [ ] **Step 1: Write RED active-contract regression**

The script must require all active contract surfaces to state that `USER_SELECTION` ends the current assistant turn and forbids `review select/units/dispatch` until another user message provides the choice.

- [ ] **Step 2: Write RED real OpenCode two-turn E2E**

Turn 1 exact user input: `harness review`. Model fixture must produce 2+ Chains. Assert no Runtime `review select`, units or dispatch calls occur in turn 1.

Turn 2 exact user input: `全部`. Assert `review select FULL` occurs and the Review authority chain completes.

- [ ] **Step 3: Run RED in branch CI**

Expected: contract and/or E2E fails against current prompts because the same turn may auto-submit FULL.

- [ ] **Step 4: Harden active contracts / deterministic model fixture**

Add explicit MUST STOP / MUST NOT CALL language and same-session continuation behavior. Do not add a fake Runtime claim that it can cryptographically prove human origin.

- [ ] **Step 5: Run GREEN plus retained Task 1/2 real-agent E2Es**

Expected: new two-turn E2E and retained 1.6.2 E2Es PASS.

- [ ] **Step 6: Commit**

```bash
git add .code-harness/AGENTS.md .code-harness/tools/README.md .code-harness/agents .github/scripts
git commit -m "fix: require user choice for multi-chain review"
```

---

### Task 4: Upgrade V2 Hash-Aware Delta Apply

**Files:**
- Modify: `.code-harness/tools-runtime/internal/upgrade/upgrade.go`
- Modify existing helpers in: `.code-harness/tools-runtime/internal/upgrade/*.go`
- Create: `.code-harness/tools-runtime/internal/upgrade/delta_163_test.go`
- Modify: `.code-harness/skills/upgrade-harness/SKILL.md`
- Modify: `.code-harness/upgrade.md`
- Modify packaging workflow/script if it currently copies `tools-runtime/**` into user payload.

**Interfaces:**
- `upgrade.Run(Options) Result` remains the transaction entrypoint.
- Project State preservation remains unchanged.
- Managed payload no longer includes `tools-runtime/**`.

- [ ] **Step 1: Write RED delta tests**

Cover:

1. source package missing `tools-runtime/**` is valid;
2. unchanged `bin/codea-dcep-tools.exe` is not replaced/touched;
3. changed Runtime is replaced;
4. framework-only change touches only changed framework files;
5. removed managed files are removed;
6. rollback still restores bytes after a forced apply failure.

- [ ] **Step 2: Run focused RED**

```bash
cd .code-harness/tools-runtime
go test -count=1 -run 'Test163Upgrade' -v ./internal/upgrade
```

Expected: FAIL due current full managed-tree replacement and `tools-runtime` ownership.

- [ ] **Step 3: Implement hash-aware delta apply**

Compute actual staged/installed file bytes or SHA-256 immediately before apply. Skip identical files. Preserve add/update/remove semantics. Only invoke running executable replacement when Runtime bytes differ.

- [ ] **Step 4: Remove tools-runtime from user managed payload**

Remove it from upgrade managed dirs/package requirements, not from repository source. Update package tests/workflow accordingly.

- [ ] **Step 5: Run GREEN + full upgrade regression**

```bash
cd .code-harness/tools-runtime
go test -count=1 -run 'Test163Upgrade' -v ./internal/upgrade
go test -count=1 ./internal/upgrade
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add .code-harness/tools-runtime/internal/upgrade .code-harness/skills/upgrade-harness .code-harness/upgrade.md .github
git commit -m "feat: apply harness upgrades by content delta"
```

---

### Task 5: 1.6.3 Exact-Head Certification

**Files:**
- Modify: `.code-harness/VERSION`
- Create: `.github/workflows/task163-runtime-usability.yml`
- Add only certification scripts/evidence required by Tasks 1–4.

**Interfaces:**
- Version becomes `1.6.3` only after focused task tests are green.

- [ ] **Step 1: Add exact-head Windows workflow**

Required steps:

```text
checkout github.sha
Go 1.23.x
Python 3.12
Node 22
Task 1 focused tests
Task 2 focused tests
Task 3 contract + real OpenCode two-turn E2E
Task 4 focused tests
full go test -count=1 ./...
go vet ./...
Windows codea-dcep-tools.exe build
pinned ast-grep 0.42.1
pinned OpenCode 1.18.25
retained 1.6.2 real Review invocation E2E
retained 1.6.2 same-session freshness E2E
exact HEAD == github.sha
```

- [ ] **Step 2: Bump VERSION to 1.6.3**

- [ ] **Step 3: Push and inspect fresh workflow**

Do not claim completion until the workflow for the exact final SHA is `success`.

- [ ] **Step 4: Run final compare against base**

Verify only intended four Hotfix areas + docs/tests/CI/version changed; no accidental 1.6.2 authority regressions.

- [ ] **Step 5: Await external acceptance**

Do not merge to `main` automatically.