# Controlled Tool Contracts

Subagents may use only these operations. Implementations must reject values outside `harness.yaml` scopes.

## `git_diff(baseRef?, headRef?) -> DiffResult`
Returns changed files and hunks. Read-only.

## `read_code(paths, lineRanges?) -> CodeBundle`
Reads repository text files allowed by source/test scopes. Read-only; no glob may escape project root.

## `write_test(path, content, planId) -> WriteResult`
Requires a human-approved test plan identified by `planId`. Path must match `write.allowedTestPaths` and no denied path.

## `run_maven_test(testClass, runId) -> ProcessResult`
Executes exactly `integrationTest.executable` plus configured args with `${testClass}` substitution. Enforces timeout; no shell evaluation.

## `start_service(runId) -> ServiceHandle`
Executes exactly the configured service executable and args, captures stdout/stderr to `runs/<runId>/service.log`, records PID and timestamps, and evaluates readiness.

Returns a `ServiceHandle`:
```json
{
  "rootPid": 1234,
  "startedAt": "2026-08-04T10:00:00Z",
  "processGroup": 1234
}
```

## `stop_service(runId, serviceHandle) -> StopResult`
Stops the process tree identified by `serviceHandle.processGroup`. Only stops PIDs recorded by `start_service` for the same run. Rejects unknown or mismatched handles.

## `read_test_report(runId) -> TestReportBundle`
Reads Maven process output and files under configured `reportDir` only.

## `read_service_logs(runId, from, to) -> LogBundle`
Reads captured process logs and the configured application log for the requested run window.

## `apply_approved_patch(fixPlanId, changes) -> PatchResult`
Requires a human-approved fix plan identified by `fixPlanId`. Every changed path must match `allowedProductionPaths`, avoid denied paths, and appear in the approved plan.
