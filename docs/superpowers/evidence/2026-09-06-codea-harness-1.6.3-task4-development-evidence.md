# Task 4 development evidence

- Branch: `hotfix/1.6.3-runtime-usability-task4`
- Accepted base: `87bf170a2f5969faca531a3996f9299b2d249e10`
- Scope: approved design section 6, Hash-Aware Delta Apply only.
- Final acceptance remains external. This is not Final Certification.

## RED before implementation

The original upgrade package passed `go test -count=1 ./internal/upgrade` before adding Task 4 tests.

Two new behavior assertions then failed against unchanged production: byte-identical Runtime replacement and reporting all package files as updated. The tests were subsequently named with the `Test163Task4` prefix used by the focused gate.

The baseline-compatible test file is also replayed by the Task 4 Windows workflow against a `git archive` of the exact accepted base. The current implementation is never used for the expected-failure side of that comparison. A successful baseline run or a build error rejects the RED gate.

Fresh local replay command, from the archived accepted Runtime with the new test file copied in:

```text
go test -count=1 -v ./internal/upgrade -run '^Test163Task4'
```

Observed assertions:

```text
--- FAIL: Test163Task4UpgradeDeltaLeavesUnchangedRuntimeAndFrameworkFilesUntouched
unchanged file was replaced: .../.code-harness/bin/codea-dcep-tools.exe
--- FAIL: Test163Task4UpgradeDeltaAddsUpdatesAndRemovesOnlyChangedManagedFiles
updatedFiles=[AGENTS.md VERSION agents/x.md bin/ast-grep.exe bin/codea-dcep-tools.exe bootstrap.md contracts/added.schema.json contracts/harness-config.schema.json contracts/upgrade-result.schema.json harness.template.yaml project.template.md skills/x/SKILL.md tools/README.md upgrade.md]
want=[VERSION contracts/added.schema.json skills/x/SKILL.md]
```

Independent review found a regression in the first delta implementation for managed directory/file transitions. Both directions were reproduced as RED (`is a directory` / `not a directory` during comparison), then fixed by classifying paths absent from the old managed-file inventory as ADD before attempting a hash comparison. Both tests run the complete upgrade transaction. No Task 1–3 compatibility change was needed.

## Exact-head verification contract

Workflow: `.github/workflows/task163-task4-delta-upgrade.yml`

The workflow checks `HEAD == github.sha`, accepted-base ancestry, accepted-base RED, focused delta/Windows tests, complete upgrade regression, complete internal-package regression with pinned ast-grep, retained discovery/review command gates, Task 3 active contract/negative control/real OpenCode same-session E2E, Runtime build, actual install/upgrade ZIP contents and manifest hashes, `go vet ./...`, and a final changed-file scope audit.

Run/job IDs and the final exact SHA are reported after the workflow completes; they are intentionally not embedded by a later commit that would invalidate that HEAD's evidence.

The package gate reuses the existing package builder and retains the repository's current VERSION. Its artifacts are Task 4 regression candidates, not published 1.6.3 releases. Final Certification and release versioning remain outside this task.

## Pre-existing full-repository test failure

An additional local `go test -count=1 ./...` run found failures outside Upgrade. They were reproduced independently from an archive of accepted base `87bf170a2f5969faca531a3996f9299b2d249e10`:

```text
go test -count=1 -v ./cmd/codea-dcep-tools -run '^Test152RealDualProjectWorkspaceBusiness'

Test152RealDualProjectWorkspaceBusinessRegression:
expected COMPLETE, got PARTIAL / INHERITED_METHOD_NOT_FOUND

Test152RealDualProjectWorkspaceBusinessFailureRegressions/ambiguous_override:
expected PARTIAL / AMBIGUOUS_TEMPLATE_DISPATCH,
got PARTIAL / INHERITED_METHOD_NOT_FOUND
```

The legacy `businessAstRunner152` test fixture builds result paths by appending `com/...` to the AST input as though it were always a directory. Accepted Task 1 narrowing can pass a concrete candidate file. No Task 4 production change is involved in reproducing these failures. The fixture and Task 1–3 production remain unchanged under the user's scope restriction. Do not interpret the scoped Task 4 Windows workflow as a full `go test ./...` PASS.
