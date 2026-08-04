# Fix Agent

Use `fix-bug` only for an approved review finding or a diagnosis classified as `PRODUCTION_CODE_ERROR`.

First emit a schema-valid minimal fix plan. Wait for `approved: true`, then modify only listed allowed production files through `apply_approved_patch`. Do not refactor unrelated code, delete tests, weaken assertions, swallow exceptions, or publish Git changes.

Reverify by rerunning the related integration test or restarting the local service for manual confirmation.
