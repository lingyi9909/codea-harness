# 1.6.3 Final Certification regression repair

Branch: `release/1.6.3-final-certification`.

Task 1–4 accepted baseline: `7b339fa61745a227202a4a81e6c26a8ffc2ca11f`.
Repair base: `d0d094cfaccc19bef19eaa03349db30500b98f8d`.
Rejected Windows run: `34072313345`, job `101591798442`.

The rejected run passed 16 of 18 certification gates. It failed only
`fullGoRegression` and `goVet`; the detached-HEAD scope fix already passed.
This repair does not change Task 1–4 production, active Agent contracts,
Runtime authority, package contents policy, or product scope.

## Root causes and exact migration scope

| File | Evidence and minimal migration |
| --- | --- |
| `chain_discover_bootstrap_151_test.go` | Its old Reviewer substring required direct analyze-change → discover-chain, bypassing the now explicit certification handoff. Assert the accepted PROJECT/AFFECTED split and complete retained Snapshot → semantic proposal → certification → discovery contract. Preserve ChangeSet/new-entrypoint and forbidden-guidance assertions. |
| `task160_release_test.go` | The VERSION assertion accepted only 1.6.1/1.6.2. Require exactly 1.6.3; retain all historical 1.6.0 changelog and 1.6.1 packaging/whitelist assertions. |
| `workspace_chain_152_test.go` | The fake ast-grep Runner treated its final query argument as a directory and fabricated paths such as `XxxServiceImpl.java/com/company/order/XxxServiceImpl.java`. Task 1 passes one or more candidate files. Adapt only the Runner to honor those targets and preserve directory queries. The original COMPLETE, exact ambiguity/error codes, source attribution, workspace provenance, read-only dependency and no-discovery-on-failure assertions are unchanged. |
| `task163-final-certification.ps1` | Tee-Object does not create a log for a silent successful native command. Initialize each log before invocation and read the empty file as a string. Preserve nonzero exit and exception failures, required markers, per-gate isolation and checklist failure aggregation. |

The scope guard explicitly enumerates only these three historical test paths.
It does not permit a test directory or arbitrary `*_test.go` changes. Regression
probes still reject an adjacent unapproved test and an adjacent production file,
as well as wrong/missing event ref, wrong SHA and arbitrary source changes.
This exact allowance is the documented test-contract migration, not permission
to alter any Task 1–4 production file.

## RED before repair

Fresh reproduction on the unmodified repair base:

```text
go test -count=1 -v ./cmd/codea-dcep-tools -run 'Test151ChainDiscoverBootstrapContractIsSelfContained|TestTask160ReleaseMetadataAndPackageWorkflow|Test152RealDualProjectWorkspaceBusiness'
FAIL Test151ChainDiscoverBootstrapContractIsSelfContained: old Reviewer flow
FAIL TestTask160ReleaseMetadataAndPackageWorkflow: VERSION is 1.6.3
FAIL Test152RealDualProjectWorkspaceBusinessRegression: INHERITED_METHOD_NOT_FOUND
FAIL Test152RealDualProjectWorkspaceBusinessFailureRegressions/ambiguous_override:
     expected AMBIGUOUS_TEMPLATE_DISPATCH, got INHERITED_METHOD_NOT_FOUND
```

The new fixture-target regression first failed for single-current-file,
multiple-candidate and dependency-file inputs, each with an emitted nonexistent
source path. The directory-input control passed. The Runner was then repaired.

The new gate-isolation regression was first run against the original driver and
failed because `silentSuccess.log` did not exist. The production function is
loaded from the driver AST; the regression does not substitute a mock gate.

The final Windows workflow repeats the rejected-base Go assertions and silent-log
RED in a disposable checkout of `d0d094c...`, with compile failures rejected and
the expected failures individually required. Complete RED logs are uploaded in
`task163-final-evidence/rejected-head-*-red.log`.

## Focused GREEN and final verification contract

Local repair validation uses Go 1.23.12, PowerShell 7.4.13 and ast-grep 0.42.1.

The five focused Go tests pass, including all original workspace business cases
and all four new target-input cases. The actual gate runner passes silent-success
and success-after-failure probes and retains FAIL for silent nonzero exit,
PowerShell exception and missing required marker. Persisted checklist results are
checked independently. Detached HEAD and scope rejection regressions pass.

Fresh local full `go test -count=1 ./...` with pinned ast-grep 0.42.1 passes
all 22 packages containing tests (zero failing packages); `go vet ./...` exits 0.
The exact workflow RED-replay script also passes locally. Independent read-only
review found no material issue in the six changed code/test/workflow files.

The Windows certification remains `go test -count=1 ./...` without excludes,
plus `go vet ./...`, Task 1–4 gates, retained 1.6.2 real-agent/same-session
regressions, exact-head package/installed-manifest verification and final scope.
No failing test is skipped or filtered out of the full regression.

The final run must contain `TASK163_FINAL_CERTIFICATION PASS`, an 18-gate PASS
checklist, and release ZIP/Runtime/whitelist identities for the same exact HEAD.
The checklist is uploaded with the release candidate and printed from the same
file in the final log. The final SHA and run/job IDs are supplied in the delivery
message, avoiding a metadata-only commit that would invalidate the run evidence.

Independent acceptance is still required; no merge, release publication,
Accepted Baseline update or Release CLOSED claim is performed by this repair.
