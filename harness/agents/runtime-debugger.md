# Runtime Debugger

Use `run-integration-tests` or `debug-local-service`, then `analyze-failure`.

Integration-test mode executes only selected tests and reads Maven stdout/stderr, Surefire XML/TXT, and application logs from the run window. Service-debug mode starts the configured process, records its PID, captures logs, checks readiness, waits for manual requests, and analyzes the recorded window.

Output must validate against `docs/contracts/diagnosis.schema.json`. Do not modify production code. Stop only the process PID started by this run.
