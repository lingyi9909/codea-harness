# Codea Harness 1.8 — Task 3 Acceptance Record

Status: **CANDIDATE — awaiting independent re-verification**

Base: `feature/1.8-task2-chain-selection`  
Accepted T2 baseline: `db2f03cebafa6f72c733286a083ad97e14f41cf5`  
T3 branch: `feature/1.8-task3-primary-report`  
Formal PR: `#56`

This record belongs only to Task 3. Do not enter Task 4 and do not merge PR #56 until the independent acceptance owner explicitly accepts the candidate.

## Blocker repair scope

1. **Concurrent finish isolation**
   - Every TypeScript `codea-review` finish invocation creates an invocation-unique `requests/finish-<UUID>.json` using exclusive create semantics.
   - Native Host regression sends two different sibling finish calls through the real TypeScript Tool and Go Runtime; exactly one may complete and the other must be rejected by the Runtime overwrite gate.

2. **Selected-chain finding scope**
   - `scope.reads` remains the context-reading boundary.
   - Runtime separately persists and validates the line-level scope that may support formal findings.
   - Permanent regressions cover sibling Controller method, shared Service sibling method, and shared Mapper XML sibling statement.

3. **Change attribution authority**
   - `CURRENT_IMPLEMENTATION` rejects `introducedByChange=true`.
   - `CHANGES` accepts `introducedByChange=true` only when evidence intersects the current run's real changed lines.
   - Model-supplied attribution is never authoritative by itself.

4. **Active instruction cleanup**
   - Ordinary Review uses only the Codea Harness 1.8 report-first path: `review start → codea-review prepare → optional real-user select → finish`.
   - pre-1.8 ordinary Review orchestration is isolated under `.code-harness/history/ordinary-review-pre-1.8.md` and is not active authority.
   - Test/Fix/API-doc/Chain/Upgrade rules remain active. Chain edit retains `harness chain edit <id|Controller|Controller.method> → edit-chain → chain-edit-candidates → chain seal-persist → exact planId → chain persist`.

5. **Real-model acceptance evidence**
   - The real-model smoke must identify the deliberately seeded `IllegalStateException("always fails after successful create")` issue, not merely produce a non-empty findings array.
   - The completed `finish` Tool output's `reportPath` and `reportSha256` must exactly match the durable on-disk `review.md`.
   - The workflow retains artifact `review180-real-model-acceptance` containing a redacted `trajectory.json`, `review.md`, `result.json`, `scope.json`, `run.json`, and SHA-256 `manifest.json`.

## Required exact-head verification

For the final candidate exact HEAD, all of the following must be fresh and green in the same Windows Runtime Regression run:

- secret preflight
- pinned ast-grep bootstrap
- pinned OpenCode native export smoke
- Review regression
- Chain regression
- `go test -count=1 ./...`
- `go vet ./...`
- Codea 1.8 native Host single-chain smoke
- Codea 1.8 native Host multi-chain real-user-selection smoke
- Codea 1.8 native Host concurrent-finish smoke
- Codea 1.8 real-model autonomous single-chain smoke
- retained `review180-real-model-acceptance` artifact with self-consistent manifest and report hash

The exact HEAD, workflow run/job IDs, artifact ID and observed real-model evidence are reported with the re-verification handoff. They are intentionally not hard-coded here so that this file itself remains part of the exact HEAD being tested.
