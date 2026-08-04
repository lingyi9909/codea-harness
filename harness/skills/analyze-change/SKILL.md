---
name: analyze-change
description: Analyze the current Git Diff to identify all changed files, trace call chains, and produce a structured change analysis for downstream review and test planning.
version: 1
agent: reviewer
tools:
  - git_diff
  - read_code
output_schema: docs/contracts/change-analysis.schema.json
---

# Analyze Git Change

## Purpose
Analyze the current Git Diff to identify all changed files, understand the affected call chains, and produce a structured change summary for downstream review and test planning.

## When to use
- User says `harness review` or `harness test` — this is always the first step
- Before any review, test planning, or code modification
- Whenever the change context is needed by Reviewer, Integration Test Agent, or Fix Agent

## Do not use when
- There is no Git repository
- The working tree has no changes relative to the base ref

## Inputs
- `baseRef` (optional): base Git ref, defaults to the merge-base with main/master
- `headRef` (optional): head Git ref, defaults to HEAD

## Allowed tools
- `git_diff` — list changed files and hunks
- `read_code` — read changed source files and directly related callers/callees

## Execution steps

1. **Get the diff**: call `git_diff(baseRef, headRef)` to obtain all changed files and their hunks.
2. **Classify changes**: group changed files by role — Controller, Service, Repository/Mapper/DAO, Entity/DTO/VO, Validator, ExceptionHandler, Config, Utility, Other.
3. **Read changed files**: for every changed source file matching `scope.sourceIncludes`, call `read_code` to get full file content.
4. **Trace call chains**: for each changed Controller or Service method, identify:
   - Direct callers (upstream) and callees (downstream) within the project
   - Repository/Mapper methods invoked
   - External dependencies (RPC, MQ, third-party APIs, cache)
5. **Identify risk areas**: flag methods that involve:
   - State transitions or status flows
   - Transactional boundaries
   - Authorization, identity, or tenant checks
   - Idempotency mechanisms
   - Exception handling paths
   - Database write operations (INSERT/UPDATE/DELETE)
6. **Assemble output**: produce a structured summary validating against `docs/contracts/change-analysis.schema.json`.

## Output
Must validate against `docs/contracts/change-analysis.schema.json`:
- `changedFiles[]`: path, role, optional hunkSummary
- `affectedControllers[]`: controller class and affected endpoints
- `callChains[]`: entryPoint and full Controller → Service → Repository chain
- `externalDependencies[]`: external systems touched by the change
- `riskAreas[]`: method and list of risk tags (stateTransition, transactional, authorization, tenancy, idempotency, exceptionHandling, databaseWrite)

## Stop conditions
- No changed files matching `scope.sourceIncludes` → report empty diff and stop
- A file cannot be read → report the error and skip that file

## Forbidden actions
- Do not scan unrelated modules or the whole repository
- Do not modify any files
- Do not execute shell commands directly

## Example

```
Input: git_diff for feature branch with 3 changed files
Output:
  changedFiles:
    - { path: "OrderController.java", role: "Controller", hunkSummary: "new approve endpoint" }
    - { path: "OrderService.java", role: "Service", hunkSummary: "approve logic" }
    - { path: "OrderRepository.java", role: "Repository", hunkSummary: "updateStatus method" }
  affectedControllers:
    - { controller: "OrderController", endpoints: ["POST /api/order/approve"] }
  callChains:
    - { entryPoint: "OrderController.approve()", chain: ["OrderService.approve()", "OrderRepository.updateStatus()"] }
  externalDependencies: ["OrderRpcClient.notifyErp()"]
  riskAreas:
    - { method: "OrderService.approve()", risk: ["stateTransition", "transactional", "tenancy"] }
```
