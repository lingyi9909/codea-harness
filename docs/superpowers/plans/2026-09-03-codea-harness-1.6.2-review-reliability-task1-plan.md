# Codea Harness 1.6.2 Review Reliability Task 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the active `harness review` contracts expose and safely invoke the complete existing Review Authority Chain, including machine-readable Finding Certification and Report Review request contracts, for both changed and zero-change reviews.

**Architecture:** Keep all existing Review/Finding authority semantics unchanged. Add request schemas at the Agent→Runtime boundary, validate them before strict decode, align the active Agent/Tool contracts with the Runtime commands that already exist, and prove the behavior through focused Go contract tests plus a real Windows OpenCode Agent Host E2E covering changed and zero-change runs.

**Tech Stack:** Go 1.23.x, JSON Schema Draft 2020-12, PowerShell, Python 3.12, OpenCode 1.18.25, GitHub Actions Windows runner.

**Spec:** User-approved `Codea Harness 1.6.2 | harness review Reliability Hotfix` Task 1 specification in the development handoff for this branch.

## Global Constraints

- Development branch must start from exact HEAD `e23023481edef9f95cdc59938efe5de4840093b8`.
- Do not modify Canonical ChangeSet core semantics.
- Do not modify Certified ChangeAnalysis Authority.
- Do not modify Finding Certification business semantics.
- Do not modify ReviewUnit business semantics.
- Do not modify Spring/MyBatis Rule Pack.
- Do not modify Chain business semantics.
- Do not modify existing Review Scope / Selection semantics.
- Task 1 only; stop after fresh exact-head Task 1 verification and wait for external acceptance.

---

### Task 1.1: Add RED Runtime request-contract tests

**Files:**
- Create: `.code-harness/tools-runtime/cmd/codea-dcep-tools/review_reliability_task1_contract_test.go`
- Create: `.github/scripts/task162-review-reliability-task1-contract-regression.ps1`

**Interfaces:**
- Consumes: existing `requestcontract.Validate`, `runReviewCertifyFindings160`, and `runReviewReport` entrypoints.
- Produces: regression expectations for `finding-certify-request.schema.json` and `report-review-request.schema.json` and the full active command inventory.

- [ ] **Step 1: Write failing schema tests**

Add tests that require both new schema files, accept canonical requests, reject unknown fields, and reject non-empty `findings` in the report request.

- [ ] **Step 2: Write failing command-boundary tests**

Invoke schema-invalid Finding Certify and Report Review requests from a temporary run request directory and assert errors contain `FINDING_CERTIFY_REQUEST_SCHEMA_INVALID` and `REPORT_REVIEW_REQUEST_SCHEMA_INVALID` before downstream business processing.

- [ ] **Step 3: Write failing active-contract audit**

Assert `.code-harness/AGENTS.md`, `.code-harness/tools/README.md`, `.code-harness/agents/orchestrator.md`, and `.code-harness/agents/reviewer.md` expose the formal Review command chain and both new request schemas.

- [ ] **Step 4: Run focused RED CI**

Run the focused Go test and PowerShell contract regression on Windows. Expected: FAIL because the two schemas and active contract declarations do not yet exist.

---

### Task 1.2: Add request schemas and schema-before-decode gates

**Files:**
- Create: `.code-harness/contracts/finding-certify-request.schema.json`
- Create: `.code-harness/contracts/report-review-request.schema.json`
- Modify: `.code-harness/tools-runtime/cmd/codea-dcep-tools/review_precision_command.go`
- Modify: `.code-harness/tools-runtime/cmd/codea-dcep-tools/report.go`

**Interfaces:**
- Consumes: existing `findingCertifyRequest160` and `report.ReviewRequest` shapes.
- Produces: `requestcontract.Validate("finding-certify-request.schema.json", data)` and `requestcontract.Validate("report-review-request.schema.json", data)` before strict decode.

- [ ] **Step 1: Add Finding Certify request schema**

Require exactly `runId` and `proposalsPath`, set `additionalProperties:false`, and constrain the proposals path to the same canonical request path form.

- [ ] **Step 2: Add Report Review request schema**

Mirror the existing `report.ReviewRequest` transport fields and nested structures. Require `findings` to be an empty array (`maxItems:0`) so raw Agent findings cannot bypass same-run Certified Findings.

- [ ] **Step 3: Validate Finding Certify request before strict decode**

Read bytes → request schema validation → existing strict JSON decode → existing path/business checks.

- [ ] **Step 4: Validate Report Review request before strict decode**

Read bytes → request schema validation → existing strict JSON decode → existing Certified ChangeAnalysis/Scope/Coverage/Certified Findings authority flow.

- [ ] **Step 5: Run focused GREEN tests**

Run the Task 1 contract tests. Expected: PASS without changing Finding Certification or ReviewUnit logic.

---

### Task 1.3: Align active Agent and Tool contracts

**Files:**
- Modify: `.code-harness/AGENTS.md`
- Modify: `.code-harness/tools/README.md`
- Modify: `.code-harness/agents/orchestrator.md`
- Modify: `.code-harness/agents/reviewer.md`

**Interfaces:**
- Consumes: the existing Runtime commands and new request schemas.
- Produces: one consistent discoverable formal Review command inventory and schema-read-before-write instructions.

- [ ] **Step 1: Expose all formal Review Runtime commands**

List `review options`, `review select`, `review units`, `review dispatch`, `review certify-findings`, and `report review` in the active fixed-command contracts.

- [ ] **Step 2: Require request schema reads**

Before creating `finding-certify-request.json`, read `finding-certify-request.schema.json`. Before creating `report-review.json`, read `report-review-request.schema.json`. Do not use `review-output.schema.json` as the report request contract.

- [ ] **Step 3: Add canonical FULL and TARGETED report request examples**

Both examples contain `findings:[]`; TARGETED includes `target`, `scopedFiles`, and selected `callChains`.

- [ ] **Step 4: Make zero-change behavior explicit**

State that `changedFiles=[]` still executes Review Units → Rule Dispatch → `finding-proposals=[]` → Finding Certification → Report Review; no early success shortcut.

- [ ] **Step 5: Re-run active-contract regression**

Expected: PASS.

---

### Task 1.4: Real OpenCode Agent Host changed/zero-change E2E

**Files:**
- Create: `.github/scripts/task162-review-reliability-task1/mock_openai_server.py`
- Create: `.github/scripts/task162-review-reliability-task1-real-agent-e2e.ps1`
- Create: `.github/workflows/task162-review-reliability-task1.yml`

**Interfaces:**
- Consumes: active repository contracts, real `codea-dcep-tools.exe`, pinned ast-grep 0.42.1, pinned OpenCode 1.18.25.
- Produces: host protocol evidence and exact-head CI proving changed and zero-change complete Review Authority chains.

- [ ] **Step 1: Build deterministic Agent-host scenario**

The model may issue normal OpenCode tool calls only. Its first stage reads AGENTS, Tool README, Orchestrator, Reviewer, and both new request schemas before formal Finding/Report requests are written.

- [ ] **Step 2: Run changed Review scenario**

Create one benign review-relevant working-tree YML change, execute the plain user intent `harness review`, and assert all authoritative artifacts including Certified Findings and `review.md` exist.

- [ ] **Step 3: Run zero-change Review scenario**

Use the same active contracts in a clean fixture with no review-relevant change and assert empty Review Units/Dispatch/Proposals/Certified Findings still lead to a PASSED `review.md` with zero changed files and zero findings.

- [ ] **Step 4: Assert forbidden failure text is absent**

Reject transcripts containing `unknown field`, `cannot unmarshal array`, missing Certified Findings, or claims that Runtime lacks `certify-findings`.

- [ ] **Step 5: Run full Task 1 verification**

Run focused Task 1 tests, full Go tests, `go vet ./...`, static contract regression, both real Agent scenarios, and exact-head marker. Expected: all PASS.

- [ ] **Step 6: Stop at Task 1 acceptance gate**

Report branch, exact HEAD, CI run/job, and evidence. Do not begin Task 2.
