---
name: fix-bug
description: Design and apply a minimal, approved fix for a confirmed production-code defect.
version: 1
agent: fix-agent
tools:
  - apply_approved_patch
output_schema: docs/contracts/fix-plan.schema.json
---

# Fix Approved Bug

## Purpose
Given a user-selected review finding or a `PRODUCTION_CODE_ERROR` Diagnosis, design a minimal fix, obtain human approval, and apply the change through controlled tooling.

## When to use
- User explicitly says "fix finding F-001" or "fix diagnosis run-001"
- Orchestrator routes `harness fix finding:F-001` or `harness fix diagnosis:<runId>` to Fix Agent
- Runtime Debugger outputs `nextAction: GENERATE_FIX_PLAN`

## Do not use when
- The issue is a test code error → repair tests via Integration Test Agent with `REPAIR_TEST`
- The issue is environment/data → report to user with `REPORT_ENVIRONMENT`
- No specific finding or diagnosis has been selected by the user

## Inputs
- A user-selected review finding (identified by finding `id`, e.g., `F-001`)
- OR a Diagnosis classified as `PRODUCTION_CODE_ERROR` (identified by `runId`)
- Full source files for the affected code

## Allowed tools
- `read_code` — read affected source files
- `apply_approved_patch` — modify production files (only after approval)

## Preconditions
- A review finding or `PRODUCTION_CODE_ERROR` Diagnosis exists
- The user has explicitly selected which issue to fix

## Execution steps

1. **Read the affected code**: use `read_code` to get full content of the file(s) referenced in the finding or diagnosis.
2. **Trace the root cause**: identify the exact line or condition causing the defect. Document in `rootCause`.
3. **Design the fix**: determine the smallest change that resolves the root cause. The change must:
   - Address only the reported defect
   - Not refactor, restructure, reorganize, or "clean up" surrounding code
   - Not weaken any existing validation, assertion, or error check
   - Not delete or disable any tests
   - Not change test code
4. **Produce fix plan**: emit a schema-valid plan with a unique `fixPlanId`:
   ```json
   {
     "fixPlanId": "fix-plan-20260804-001",
     "rootCause": "...",
     "changes": [
       {
         "file": "src/main/java/com/example/OrderService.java",
         "reason": "Missing status guard before state transition",
         "change": "Add precondition check: if (order.getStatus() != PENDING) throw new IllegalOrderStateException()"
       }
     ],
     "verification": [
       "Run OrderControllerIT.shouldApproveOrder — should pass",
       "Run OrderControllerIT.shouldRejectNonPendingOrder — new test for the guard"
     ]
   }
   ```
5. **Wait for human approval**: present the plan prominently. The user must explicitly state "批准 <fixPlanId>" or "approved <fixPlanId>". Generic affirmations ("ok", "继续", "可以") do not count as approval. If the plan is modified, generate a new `fixPlanId` — old approvals do not transfer.
6. **Apply the fix**: after explicit approval, use `apply_approved_patch(fixPlanId, changes)` to modify only the listed files.
7. **Report completion**: output which files were modified and hand off to Orchestrator for verification.

## Output
- Fix plan validating against `docs/contracts/fix-plan.schema.json`
- Modified production files (only those in the approved plan's `changes`)

## Stop conditions
- Fix plan not approved → stop
- A target file is in `deniedPaths` → stop and report
- A target file is outside `allowedProductionPaths` → stop and report
- Verification after fix reveals new issues → emit new diagnosis, do NOT auto-fix

## Forbidden actions
- Do not modify production code before explicit human approval of `fixPlanId`
- Do not refactor, restructure, or "improve" unrelated code
- Do not delete tests, disable tests, weaken assertions, or swallow exceptions
- Do not commit, push, create PRs, or publish Git changes
- Do not re-run tests — verification is handled by Runtime Debugger
- Do not execute shell commands directly — use only controlled tools

## Example

User: fix finding F-001

Agent:
```
Fix Plan: fix-plan-20260804-001

Root cause: OrderService.approve() at line 42 sets status=APPROVED without
checking the current status, allowing re-approval of cancelled/rejected orders.

Change:
  File: src/main/java/com/example/OrderService.java:42
  Add: if (order.getStatus() != Status.PENDING) {
         throw new IllegalOrderStateException("Order is not in PENDING status");
       }

Verification:
  1. Run OrderControllerIT.shouldApprovePendingOrder — should pass
  2. Manually confirm shouldRejectNonPendingOrder catches the new guard

Please reply: 批准 fix-plan-20260804-001
```
