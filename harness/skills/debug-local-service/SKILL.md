---
name: debug-local-service
description: Start the local service, capture its process tree, verify readiness, and collect logs for a debugging time window.
version: 1
agent: runtime-debugger
tools:
  - start_service
  - stop_service
  - read_service_logs
output_schema: docs/contracts/diagnosis.schema.json
---

# Debug Local Service

## Purpose
Start the configured service process, record its `ServiceHandle` (process tree), capture stdout/stderr, verify readiness, record a debugging time window, and collect logs after manual requests.

## When to use
- User says `harness debug-service` or requests to start the service for manual debugging
- Orchestrator routes a service-debug intent to Runtime Debugger
- A developer wants to start the service locally and have Harness capture logs for later diagnosis

## Do not use when
- The user wants to run automated integration tests — use `run-integration-tests` instead
- The service is already running on the target port

## Inputs
- `runId`: unique run identifier
- `harness.yaml` configuration: `service.executable`, `service.args`, `service.startupTimeoutSeconds`, `service.readiness`, `service.logFile`
- `stopService.mode` (must be `processTree`)

## Allowed tools
- `start_service` — start the configured process
- `stop_service` — stop only the process tree recorded in the ServiceHandle
- `read_service_logs` — read captured stdout/stderr and application logs

## Preconditions
- `harness.yaml` has valid `service.executable` and `service.args`
- The configured port is not already in use
- `stopService.mode` is `processTree`

## Execution steps

1. **Start the service**: call `start_service(runId)`. This executes exactly the configured `service.executable` with `service.args`. No shell evaluation. Captures stdout/stderr to `runs/<runId>/service.log`.
2. **Record ServiceHandle**: capture the returned `ServiceHandle`:
   ```json
   {
     "rootPid": 1234,
     "startedAt": "2026-08-04T10:00:00Z",
     "processGroup": 1234
   }
   ```
   Store this handle — it is required for `stop_service` and must not be used for any other run.
3. **Verify readiness**: poll captured stdout/stderr for the configured `readiness.pattern` (e.g., `Started Application`). Wait up to `startupTimeoutSeconds`. If the pattern is not found, classify as `SERVICE_START_ERROR`.
4. **Record the debugging window**: note `windowStart` (time when readiness was confirmed). The window remains open until the user signals completion or `stop_service` is called.
5. **Wait for manual requests**: the developer or frontend triggers requests manually. Harness does NOT send automated HTTP requests in V1.
6. **Collect logs**: when the user signals completion (or on error), call `read_service_logs(runId, windowStart, now)` to collect:
   - Captured stdout/stderr from `runs/<runId>/service.log`
   - Application log file (if configured in `service.logFile`) for the same time window
7. **Stop the service**: call `stop_service(runId, serviceHandle)`. This stops the process tree identified by `serviceHandle.processGroup`. Never stop any other process.

## Output
- `ServiceHandle` (rootPid, startedAt, processGroup)
- Readiness status (ready or timeout)
- Log bundle for the debugging window
- If startup failed: diagnosis with `SERVICE_START_ERROR`

## Stop conditions
- Startup timeout → classify as `SERVICE_START_ERROR`, attempt to stop the process tree, emit diagnosis
- Port already in use → report and stop
- User requests stop → stop the process tree via `stop_service`

## Forbidden actions
- Do not stop any process not recorded in the current ServiceHandle
- Do not send automated HTTP requests
- Do not use `kill` or system commands directly — use only `stop_service`
- Do not start the service if it is already running
- Do not execute shell commands directly — use only controlled tools

## Example

```
Input: runId=debug-20260804-001
Command: ./mvnw spring-boot:run
ServiceHandle: { rootPid: 5678, startedAt: "2026-08-04T10:05:00Z", processGroup: 5678 }
Readiness: confirmed at 10:05:23 (pattern "Started Application" found)
Window: 10:05:23 → 10:15:00 (user signaled stop)
Logs collected: service.log (235 lines), application.log (89 lines in window)
Stop: process tree for processGroup=5678 stopped successfully
```
