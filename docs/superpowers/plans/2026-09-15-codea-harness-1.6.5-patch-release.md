# Codea Harness 1.6.5 Patch Release Implementation Plan

**Goal:** Ship the accepted review fixes in an offline Windows package that existing 1.6.4 users can upgrade to.

**Architecture:** Retain strict SemVer upgrade ordering and the existing backup/stage/delta/rollback transaction. Reuse the accepted package builder and installer. Include Reviewer Host resources in the explicit 1.6.4 → 1.6.5 transaction, accepting changed destinations only when their bytes match the installed manifest's ownership hash.

**Tech Stack:** Go 1.23, PowerShell, GitHub Actions Windows x64.

**Spec:** User-approved 1.6.5 patch version; accepted fix baseline `46350e78d9f5d23f3428d4208b7eb4caf25ce958`.

## Constraints

- No new review semantics, no 1.7 work, no retired Task153 workflow restoration.
- Preserve configuration and business state; reject user-modified Host conflicts before mutation.
- Do not treat previous HEAD's successful CI as acceptance of this release.
- Known deferred limitation: trusted human Host-turn binding is not completed by this patch release.

## Implementation and verification

- [x] Add `internal/upgrade/release_165_test.go` regressions for 1.6.4 → 1.6.5 Host updates, preserved state, missing/tampered payload, user conflicts, and partial Host failure rollback. Run `go test -count=1 ./internal/upgrade -run Test165` and observe the old implementation fail.
- [x] Extend the explicit patch upgrade edge using installed Host hashes and target package Host hashes, retaining fixed paths and the existing transaction journal. Run all upgrade tests.
- [x] Set `.code-harness/VERSION` to 1.6.5, update active release metadata assertion, CHANGELOG, README and upgrade instructions. Historical version references remain.
- [x] Add a 1.6.5 adapter around the unchanged 1.6.4 package builder and a release workflow. Build install/upgrade ZIPs, verify every manifest hash and exact HEAD, and test real packaged Windows upgrade against the unmodified 1.6.4 install artifact from Run 34804181203 / build d8327ffe51cecac4a3887366ec847e3aec627923.
- [ ] Windows package gate checks target Runtime, ast-grep, Host hashes, preserved project files, consumed upgrade source and no transaction leaks; full Go tests and vet must pass before artifact upload.
- [ ] Run Linux regression/vet, submit release branch, complete fresh Windows CI, and integrate exact tested HEAD. Deliver Windows upgrade artifact link, SHA256, HEAD and Run/Job evidence.

Local verification: baseline upgrade tests PASS; new Host tests failed on the original implementation, then passed after the patch. Full Linux `go test -count=1 ./...` and `go vet ./...` PASS with real ast-grep 0.42.1. Fresh Windows evidence remains the release gate.
