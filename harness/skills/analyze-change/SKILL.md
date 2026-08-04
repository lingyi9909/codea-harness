# Analyze Git Change

## Purpose
Analyze the current Git Diff to identify all changed files, understand the affected call chains, and produce a structured change summary for downstream review and test planning.

## Inputs
- `baseRef` (optional): base Git ref, defaults to the merge-base with main/master
- `headRef` (optional): head Git ref, defaults to HEAD

## Allowed tools
- `git_diff` — list changed files and hunks
- `read_code` — read changed source files and directly related callers/callees

## Execution steps

1. **Get the diff**: call `git_diff(baseRef, headRef)` to obtain all changed files and their hunks.
2. **Classify changes**: group changed files by role — Controller, Service, Repository/Mapper/DAO, Entity/DTO/VO, validator, exception handler, config, and shared utilities.
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
6. **Assemble output**: produce a structured summary with affected components, call chains, external dependencies, and risk areas.

## Output
A structured change analysis containing:
- `changedFiles`: list of changed files grouped by role
- `affectedControllers`: Controller classes and their affected endpoints
- `callChains`: for each affected endpoint, the full Controller → Service → Repository chain
- `externalDependencies`: external systems touched by the change
- `riskAreas`: methods or sections flagged for deeper review

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
  - Controller: OrderController.approve() — new endpoint POST /api/order/approve
  - Service chain: OrderController → OrderService.approve() → OrderRepository.updateStatus()
  - External: OrderRpcClient.notifyErp() — ERP notification RPC
  - Risk areas: status transition PENDING→APPROVED, @Transactional boundary, tenant isolation check
```
