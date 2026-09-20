# Task 4 — Hash-Aware Delta Apply Implementation Plan

> Execute with Superpowers TDD and task-scoped review. Task 4 only; stop for external acceptance afterward.

**Goal:** Apply only changed managed files while retaining the existing upgrade transaction.

**Accepted base:** `87bf170a2f5969faca531a3996f9299b2d249e10`

**Branch:** `hotfix/1.6.3-runtime-usability-task4`

**Spec:** `docs/superpowers/specs/2026-09-06-codea-harness-1.6.3-runtime-usability-hotfix-design.md`, section 6.

## Scope and decisions

- Production changes belong to `internal/upgrade`, its UpgradeResult schema and upgrade documentation.
- Preserve Backup, Stage, Schema Validation, Apply and Rollback. Compare actual installed/staged file bytes using SHA256; the approved design does not define a new persistent installed-hash manifest authority.
- UNCHANGED files retain identity and modification time. This includes the installed Runtime, ast-grep and unmigrated `harness.yaml`.
- Changed Runtime still uses the existing Windows executable replacement path. Source-package executable cleanup remains distinct from replacement of the installed executable.
- Project State stays protected; `runs/README.md` remains managed documentation.
- Reuse the existing package builder, which already excludes Runtime Go source. Verify its actual ZIPs and manifest hashes/buildCommit on the Task 4 HEAD. Retain repository VERSION until the separately authorized release stage.
- Do not modify Task 1–3 production, reuse the mistaken branch, merge, publish a release or start Final Certification.

## Execution

1. Read the approved design and existing upgrade, migration, Windows replacement, result schema and package tests. Confirm branch/base and clean baseline `go test -count=1 ./internal/upgrade`.
2. Add behavior tests using real files and the existing `Run`/`applyStaged` APIs. Assert unchanged identity/mtime and exact changed-file results, ADD/UPDATE/REMOVE, Project State, managed README, real Windows live executable behavior and rollback after partial apply.
3. Execute RED against unchanged production; record command and expected assertion failures. Keep a baseline-compatible RED file for CI replay against the accepted SHA.
4. Implement deterministic file delta, use it for application/results, skip unchanged config writes, and let rollback compare backup with the partially applied tree. Preserve existing migration and executable replacement semantics.
5. Run focused `go test -count=1 -v ./internal/upgrade -run '^Test163Task4'`, full upgrade regression and full Go regression. Review Task 4 spec coverage and changed-file scope.
6. Commit integrated changes and push the existing Task 4 branch. Run the new Windows workflow on that exact SHA: accepted-base RED replay, focused/upgrade/full regression, built ZIP inspection, pinned ast-grep, retained Task 3 active-contract and real same-session regressions, `go vet`, exact HEAD and scope checks.
7. Deliver final branch/base/SHA, RED evidence, changed files, focused/upgrade/Windows results and run/job URLs. Stop for Task 4 acceptance.
