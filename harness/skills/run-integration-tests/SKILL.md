---
name: run-integration-tests
description: Execute selected Maven integration tests and collect Surefire reports and logs.
version: 1
agent: runtime-debugger
tools:
  - run_maven_test
  - read_test_report
output_schema: docs/contracts/diagnosis.schema.json
---

# Run Integration Tests

## Purpose
Execute a specific integration test class via the configured Maven command, enforce the timeout, and collect all execution output for downstream diagnosis.

## When to use
- After Integration Test Agent has generated or repaired test classes
- When the Orchestrator hands off test execution to Runtime Debugger
- When the user explicitly requests to run a specific test class

## Do not use when
- The test plan has not been approved yet
- The test class does not exist

## Inputs
- `testClass`: fully qualified test class name (e.g., `com.example.OrderControllerIT`)
- `runId`: unique run identifier for artifact storage

## Allowed tools
- `run_maven_test` — execute the configured Maven command
- `read_test_report` — read stdout/stderr and Surefire XML/TXT

## Preconditions
- `harness.yaml` has valid `integrationTest.executable` and `integrationTest.args`
- The test class exists under `scope.testIncludes`
- The configured `integrationTest.timeoutSeconds` is acceptable

## Execution steps

1. **Build the command**: substitute `${testClass}` in the configured `integrationTest.args` with the actual test class. Display the full `executable` + `args` before execution.
2. **Execute**: call `run_maven_test(testClass, runId)`. This runs exactly the configured executable with the substituted args. No shell evaluation, pipes, redirection, or command chaining.
3. **Check exit code**: record whether the Maven process exited with code 0.
4. **Collect stdout/stderr**: full process output is captured for diagnosis.
5. **Read Surefire reports**: call `read_test_report(runId)` to read XML and TXT reports from the configured `reportDir`.
6. **Return results**: pass all collected output to `analyze-failure` for classification.

## Output
- Maven exit code
- Full stdout/stderr
- Surefire XML and TXT report content (for failed tests)

## Stop conditions
- Maven executable not found → report and stop
- Timeout reached → report as `TEST_DATA_OR_ENVIRONMENT_ERROR`
- No Surefire reports found → flag and continue with stdout/stderr only

## Forbidden actions
- Do not construct arbitrary shell commands — use only the configured executable and args
- Do not run tests that are not in the approved test plan
- Do not modify test code or production code
- Do not run `mvn` directly — always use `run_maven_test`

## Example

```
Input: testClass=com.example.OrderControllerIT, runId=run-20260804-001
Command: ./mvnw -Dspring.profiles.active=test -Dtest=com.example.OrderControllerIT test
Exit code: 1
Surefire: 8 tests run, 6 passed, 2 failed
```
