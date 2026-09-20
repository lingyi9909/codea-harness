# Codea Harness 1.1.1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Harness upgrade deterministic and make Review call-chain coverage provable across changed files and internal Service/Repository/Mapper paths.

**Architecture:** Add a small Go tool runtime under `.code-harness/tools-runtime` and ship a Windows x64 `codea-harness-tools.exe` in the offline release artifact. The runtime owns upgrade/config migration/schema validation and wraps a bundled `ast-grep.exe` behind stable navigation contracts. Existing Orchestrator/Reviewer/Test/Fix agent topology remains unchanged; only contracts, skills, and hard gates are strengthened.

**Tech Stack:** Go stdlib, JSON Schema files already in the Harness package, bundled ast-grep Windows x64 binary.

**Spec:** User-approved Codea Harness 1.1.1 task list from 2026-08-19.

## Global Constraints

- V1 remains Java + Spring Boot + Maven.
- No arbitrary shell execution or user-supplied command execution.
- `harness review` and `harness test` keep the same Review Change Set semantics.
- Existing Agent topology, approval protocol, test reuse strategies, and <=2 generated-test repair rounds remain unchanged.
- Config migration is deterministic and versioned; existing `review` config is never overwritten.
- Review cannot report PASSED when reviewCoverage is PARTIAL.

---

### Task 1: Go Tool Runtime and deterministic upgrade

**Files:**
- Create: `.code-harness/tools-runtime/go.mod`
- Create: `.code-harness/tools-runtime/cmd/codea-harness-tools/main.go`
- Create: `.code-harness/tools-runtime/internal/upgrade/*`
- Create: `.code-harness/tools-runtime/internal/schema/*`
- Test: `.code-harness/tools-runtime/internal/upgrade/upgrade_test.go`

- [x] Write failing upgrade golden tests for missing review migration, existing review preservation, unresolved baseRef, and schema-failure rollback.
- [x] Run tests and verify RED.
- [x] Implement minimal deterministic upgrade transaction and `add-review-config-v1` migration.
- [x] Run tests and verify GREEN.

### Task 2: Controlled code navigation runtime

**Files:**
- Create: `.code-harness/tools-runtime/internal/nav/*`
- Test: `.code-harness/tools-runtime/internal/nav/nav_test.go`
- Modify: `.code-harness/tools/README.md`

- [x] Write failing tests for fixed ast-grep invocation, scope confinement, and shell-metacharacter rejection.
- [x] Run tests and verify RED.
- [x] Implement `nav find-symbol`, `nav find-references`, and `nav find-implementations` with fixed `ast-grep.exe` execution.
- [x] Run tests and verify GREEN.

### Task 3: Review Coverage contracts and hard gates

**Files:**
- Modify: `.code-harness/contracts/change-analysis.schema.json`
- Modify: `.code-harness/skills/analyze-change/SKILL.md`
- Modify: `.code-harness/skills/review-code/SKILL.md`
- Modify: `.code-harness/agents/reviewer.md`
- Modify: `.code-harness/agents/orchestrator.md`
- Modify: `.code-harness/AGENTS.md`

- [x] Add required `reviewCoverage` schema and unresolved symbol model.
- [x] Require reading every changed source/test file and deterministic navigation for internal call chains.
- [x] Require interface implementation resolution and multi-Service traversal to Repository/Mapper/external boundary.
- [x] Add Orchestrator hard gate: PARTIAL => MANUAL_ACTION_REQUIRED and no Integration Test Agent entry.
- [x] Add user-visible Review Coverage before findings.
- [x] Clarify findings may land on Service/Repository/Mapper/DTO/etc.

### Task 4: Upgrade contract/docs/version/package

**Files:**
- Modify: `.code-harness/skills/upgrade-harness/SKILL.md`
- Modify: `.code-harness/tools/README.md`
- Modify: `.code-harness/upgrade.md`
- Modify: `README.md`
- Modify: `.code-harness/VERSION`
- Create: `CHANGELOG.md`
- Create: `.github/workflows/package-windows-x64.yml`

- [x] Replace “never migrate config” with deterministic versioned migration rule.
- [x] Document upgrade transaction and migration/baseRef behavior.
- [x] Cross-build Windows x64 runtime.
- [x] Configure release packaging to bundle pinned ast-grep Windows x64 binary behind the wrapper.
- [x] Set VERSION to 1.1.1 and add changelog.

### Task 5: Verification

- [x] Run `go test ./...` in tools-runtime.
- [x] Run `go vet ./...`.
- [x] Cross-build `GOOS=windows GOARCH=amd64`.
- [x] Validate JSON contract files parse.
- [x] Validate packaging workflow YAML parses.
- [x] Verify the final scope stays within 1.1.1 requirements.
