---
name: review-code
description: Review analyzed changes for correctness, state transitions, transactions, authorization, tenancy, idempotency, exception handling, and data consistency. Produce evidence-backed findings.
version: 1
agent: reviewer
tools:
  - read_code
output_schema: docs/contracts/review-output.schema.json
---

# Review Changed Code

## Purpose
Review the analyzed change for correctness and safety, producing evidence-backed findings that validate against `review-output.schema.json`.

## When to use
- After `analyze-change` completes — this is the second step in `harness review` and `harness test`
- Whenever changed code needs correctness verification
- Always runs before test plan generation in `harness test`

## Do not use when
- No change analysis exists yet — run `analyze-change` first
- The diff is empty (no changes to review)
- User explicitly requests to skip review

## Inputs
- Change analysis from `analyze-change` (validated against `change-analysis.schema.json`)
- Full content of all changed source files and directly related callers/callees

## Allowed tools
- `read_code` — read source files referenced in findings for evidence extraction

## Execution steps

1. **Parameter validation**: for every changed method, check:
   - Required parameters are validated (null, empty, range)
   - Input types match the expected schema
   - DTO/VO fields have appropriate constraints

2. **Business rules**: verify that changed logic correctly implements:
   - Stated business requirements from the diff context
   - Edge cases (empty lists, null values, boundary values)

3. **State transitions**: for any status/state changes, check:
   - All valid transitions are explicitly handled
   - Invalid transitions are rejected with appropriate errors
   - Concurrent state modifications are considered

4. **Transactions**: for database writes, verify:
   - `@Transactional` boundaries are appropriate (not too wide, not too narrow)
   - Rollback conditions are correct
   - Read-only operations are not wrapped in write transactions unnecessarily

5. **Authorization and tenancy**: confirm:
   - Identity/role checks exist before sensitive operations
   - Tenant/org data isolation is enforced in queries
   - User context is propagated through the call chain

6. **Idempotency**: for mutating operations, check:
   - Duplicate requests are handled safely
   - Unique constraints or idempotency keys are present

7. **Exception handling**: verify:
   - Exceptions are caught at the right level
   - Error responses follow the project's unified response format
   - No sensitive information leaks in error messages
   - Resources (connections, streams) are closed in finally/try-with-resources

8. **Data consistency**: check:
   - Query conditions match business intent (especially soft-delete filters, status filters)
   - Update statements include appropriate WHERE conditions
   - Cache invalidation happens after successful writes

9. **For each finding**, record: file, line, concrete evidence from the diff or code, impact, a minimal recommendation, severity, whether the issue was introduced by this change, confidence level, and whether additional integration testing is needed.

## Output
Must validate against `docs/contracts/review-output.schema.json`. Every finding requires:
- `id`, `severity`, `file`, `line`, `evidence`, `impact`, `recommendation`
- `needsTest` (boolean), `introducedByChange` (boolean), `confidence` (0-1)

## Stop conditions
- No findings → emit empty findings array with a summary noting no issues found
- A file needed for evidence cannot be read → note the limitation in the finding

## Forbidden actions
- Do not modify any files
- Do not propose unrelated refactoring or style changes
- Do not report issues without concrete evidence from the diff or referenced code
- Do not scan the entire repository

## Example

```json
{
  "id": "F-001",
  "severity": "high",
  "file": "src/main/java/com/example/OrderService.java",
  "line": 42,
  "evidence": "OrderService.approve() sets status=APPROVED without checking current status is PENDING",
  "impact": "An already-approved or cancelled order could be re-approved, causing duplicate ERP notifications",
  "recommendation": "Add a status guard: if (order.getStatus() != Status.PENDING) throw new IllegalOrderStateException()",
  "needsTest": true,
  "introducedByChange": true,
  "confidence": 0.95
}
```
