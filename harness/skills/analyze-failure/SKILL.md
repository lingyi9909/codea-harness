---
name: analyze-failure
description: Correlate test output, Surefire reports, stack traces, and logs to produce a classified Diagnosis with a deterministic nextAction.
version: 1
agent: runtime-debugger
tools:
  - read_test_report
  - read_service_logs
output_schema: docs/contracts/diagnosis.schema.json
---

# Analyze Failure

## Purpose
Correlate all execution output — Maven stdout/stderr, Surefire reports, stack traces, and run-window application logs — to classify the failure and recommend the next action.

## When to use
- After `run-integration-tests` returns a non-zero exit code or failed test assertions
- After `debug-local-service` detects a startup failure or runtime error
- Any time the Orchestrator needs to decide the next step after an execution

## Do not use when
- All tests passed and the service is healthy
- The failure has already been diagnosed in the current run

## Inputs
- Maven stdout/stderr (from `run_maven_test`)
- Surefire XML/TXT reports (from `read_test_report`)
- Application logs from the run window (from `read_service_logs`)
- The execution mode: `integration-test` or `service-debug`

## Allowed tools
- `read_test_report` — re-read reports if needed
- `read_service_logs` — re-read logs with adjusted time window if needed

## Preconditions
- Test execution or service start has completed (success or failure)
- Raw output is available for analysis

## Execution steps

1. **Check for compilation errors**: scan stdout/stderr for compilation failure markers. If found → `TEST_COMPILE_ERROR`, `nextAction: REPAIR_TEST`.
2. **Check for assertion failures**: parse Surefire XML for `<failure>` or `<error>` elements. Extract the assertion message and stack trace. If the failure is in test assertion logic → `TEST_CODE_ERROR`, `nextAction: REPAIR_TEST`.
3. **Check for Spring context failures**: look for `ApplicationContext` load failures, missing bean definitions, configuration errors. **In integration-test mode** → `TEST_CONTEXT_ERROR`, `nextAction: REPAIR_TEST`. **In service-debug mode** → `SERVICE_START_ERROR`, `nextAction: RESTART_SERVICE` or `GENERATE_FIX_PLAN`.
4. **Check for data/environment issues**: look for connection refused, unknown host, missing tables, authentication failures to external services → `TEST_DATA_OR_ENVIRONMENT_ERROR`, `nextAction: REPORT_ENVIRONMENT`.
5. **Check for production code defects**: if the test is correct but the production code returns wrong results, violates business rules, or has unhandled edge cases → `PRODUCTION_CODE_ERROR`, `nextAction: GENERATE_FIX_PLAN`.
6. **Fallback**: if no clear pattern matches → `UNKNOWN`, `nextAction: STOP_UNKNOWN`. Include all raw evidence.

### Classification priority (integration-test mode)
1. `TEST_COMPILE_ERROR` — highest priority, check first
2. `TEST_CONTEXT_ERROR` — Spring context failures (NOT `SERVICE_START_ERROR`)
3. `TEST_DATA_OR_ENVIRONMENT_ERROR` — external connectivity
4. `TEST_CODE_ERROR` — assertion failures
5. `PRODUCTION_CODE_ERROR` — business logic defects
6. `UNKNOWN` — fallback

### Classification priority (service-debug mode)
1. `SERVICE_START_ERROR` — startup failures (valid in this mode only)
2. `TEST_DATA_OR_ENVIRONMENT_ERROR` — external connectivity
3. `PRODUCTION_CODE_ERROR` — runtime business logic defects
4. `UNKNOWN` — fallback

**Critical rule**: `SERVICE_START_ERROR` is only valid in service-debug mode. In integration-test mode, Spring Boot startup failures during `@SpringBootTest` are classified as `TEST_CONTEXT_ERROR`.

## Output
Must validate against `docs/contracts/diagnosis.schema.json`.
- `classification`: exactly one enum value
- `rootCause`: specific, actionable description
- `evidence`: list of concrete log lines, stack traces, or report excerpts
- `nextAction`: exactly one enum value

## Stop conditions
- Evidence is insufficient for classification → classify as `UNKNOWN`, include all available evidence

## Forbidden actions
- Do not modify test code or production code
- Do not re-run tests — that is `run-integration-tests`
- Do not guess the classification — always cite evidence

## Example

```json
{
  "classification": "PRODUCTION_CODE_ERROR",
  "rootCause": "OrderService.approve() does not validate the current order status before transitioning to APPROVED",
  "evidence": [
    "Surefire: shouldApproveOrder FAILED — expected 200 but got 500",
    "Application log: IllegalStateException at OrderService.java:42 — order status is already CANCELLED",
    "Test sent request with orderId=1 which has status=CANCELLED in the test database"
  ],
  "nextAction": "GENERATE_FIX_PLAN"
}
```
