# Code Harness Web Workbench

The local review workbench serves the React application and Fastify API on the same loopback port. It stores tasks, runs, messages, native checkpoints, permissions and report metadata in MySQL 8.4. Git snapshots and archived report files live under the configured data directory.

## Start the local workbench

Use Node 22 and the locked npm dependencies. Set `HARNESS_MYSQL_HOST`, `HARNESS_MYSQL_PORT`, `HARNESS_MYSQL_DATABASE`, `HARNESS_MYSQL_USER` and `HARNESS_MYSQL_PASSWORD` for the workbench database. Set `HARNESS_REPOSITORY_ROOTS` to the allowed repository locations, `HARNESS_DATA_DIR` to writable storage, and `HARNESS_REF_KEY` and `HARNESS_CURSOR_KEY` to stable secret values of at least 32 characters.

```text
npm ci --ignore-scripts --no-audit --no-fund
npm run web:build
npm run server:start
```

Apply the versioned migrations with the migration account before starting the service. Set `HARNESS_MIGRATION_CONFIRM_DATABASE` to the exact database name, set `HARNESS_MIGRATION_USER` and `HARNESS_MIGRATION_PASSWORD`, then run `npm run migrate`. Open the address printed by the service and enter the pairing code from its console.

The default launcher supports repository registration, preview and browsing persisted evidence. Native dispatch requires the pinned company build integration. Its local launcher can call `startWorkbench(native)` with the verified adapter, version, worker identity, repository roots and data directory. Point the API and worker to the same storage and roots. No HTTP endpoint accepts an adapter, a capability override or an observation decoder.

## Native execution and recovery

Supply build-specific decoders for attributable public observations. Tool summaries, public assistant text and verified terminal/write evidence are persisted separately from raw native frames. Live-only builds also require a message decoder and a terminal-state decoder. They record a continuity gap and reconcile from snapshots. Durable streams resume from a native cursor committed with the corresponding writes.

The worker records control sends before contacting the native service. An uncertain answer or interrupt stays out of the send queue. An expired lease quarantines the slot. Automatic reattachment requires an already accepted input, matching native build, independent read-only proof of the bound session and input, an external-writer audit, and a fresh lease fence. Uncertain sends and unresolved interactions stay quarantined for investigation.

## Verification

- `npm run typecheck`, `npm run web:typecheck` and `npm run web:build` check both applications.
- `npm run test:mysql` migrates and writes fixture records to the dedicated disposable database `harness_ci_test`. It exercises native event replay, Unicode message deltas, control delivery and run reconciliation with a scripted native peer.
- After building the Web app, migrating the disposable database and installing Playwright Chromium, `npm run test:e2e` drives the browser against Fastify, Git and MySQL. The review, permission and model output in this test come from a scripted adapter.
- The Windows workflows run the adapter, storage-contract, Git snapshot and application regressions. The separate Linux job runs the MySQL 8.4 service and Chromium.
- T13 must capture actual company Windows/OpenCode build, authentication, protocol mapping and private-model evidence. See [the Windows runbook](docs/windows-t01-runbook.md) and [the capability matrix](docs/capability-matrix.md).
