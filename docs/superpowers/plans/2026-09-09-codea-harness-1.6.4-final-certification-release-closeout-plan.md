# Codea Harness 1.6.4 Final Certification / Release Closeout Plan

Date: 2026-09-09

## Accepted baseline

- Task 3 Accepted HEAD: `48158a74a5cbec61ac8936e1c65901013e757101`
- Release branch: `release/1.6.4-final-certification`
- Final Certification must remain a fast-forward descendant of the accepted baseline.

## Scope

Release closeout only. Do not modify accepted Task 1–3 production logic unless a fresh release gate exposes a concrete blocker.

Allowed release-closeout changes:

1. `.code-harness/VERSION` -> `1.6.4`
2. `task164-release-package.ps1`
3. `task164-final-certification.ps1`
4. `task164-final-certification.yml`
5. this plan

No package/schema/authority relaxation is permitted.

## Final certification gates

### A. Exact release identity and scope

- exact `github.sha`
- release ref is `refs/heads/release/1.6.4-final-certification`
- accepted Task 3 baseline is an ancestor
- VERSION is exactly 1.6.4
- diff from accepted baseline contains release-closeout files only

### B. Package / Windows x64

- build Runtime Windows amd64
- pinned ast-grep 0.42.1
- produce install + upgrade ZIPs for 1.6.4
- verify VERSION, RELEASE-MANIFEST build commit, Runtime SHA256, ast-grep SHA256/version and managed-file inventory
- generate release checklist and runtime whitelist evidence

### C. 1.6.4 functional authority

- Task 1 snapshot-bounded exact semantic parity and batch AST behavior
- Task 2 Runtime telemetry schema/process counters
- Task 3 freshness mutation matrix and canonical projection authority
- no fallback to full `changeset.Compute()` in Task 3 fast path

### D. 1.6.4 performance

Fresh Windows exact-head evidence:

- 21-file certify <= 15s hard limit
- 50-file certify <= 20s hard limit
- 100-file certify <= 30s hard limit
- total Entrypoint AST processes <= 4
- Base cat-file processes <= 1
- per-file merge-base == 0
- per-file git show == 0

### E. Gate order authority

Fresh evidence must preserve:

- certification failure -> no downstream authoritative review execution
- uncertified analysis cannot be consumed by chain/review context
- multi-chain without explicit selection cannot create review units
- certification success -> ReviewOptions / multi-chain USER_SELECTION path remains available

### F. Retained 1.6.3 gates

- Workspace AST regression
- Clean Project Chain Discovery
- real OpenCode same-session USER_SELECTION E2E
- Upgrade V2 / package rollback regression

### G. Retained 1.6.2 Review Reliability

- Task 1 real-agent E2E
- Task 2 same-session E2E
- real plain `harness review`
- retained authority contracts

### H. Whole repository verification

- `go test -count=1 ./...`
- `go vet ./...`
- final exact-head / tracked-source-clean check

## Deliverables

- `codea-harness-1.6.4-windows-x64-install.zip`
- `codea-harness-1.6.4-windows-x64-upgrade.zip`
- `codea-harness-1.6.4-release-checklist.json`
- `codea-dcep-tools-whitelist.txt`
- complete `task164-final-evidence/`

## Stop condition

After one exact release HEAD passes every gate, stop and submit Final Certification for acceptance. Do not merge to `main`, create a tag, or publish a GitHub Release without separate user instruction.