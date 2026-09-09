# Codea Harness 1.6.4 Final Certification / Release Plan

## Baseline

Task 3 Accepted HEAD:

`48158a74a5cbec61ac8936e1c65901013e757101`

Final certification branch:

`release/1.6.4-final-certification`

## Scope

Final Certification is release-only. Do not change Task 1-3 production semantics.

Allowed changes from the accepted Task 3 baseline:

- `.code-harness/VERSION`
- `CHANGELOG.md`
- `.github/scripts/task164-final-certification.ps1`
- `.github/scripts/task164-release-package.ps1`
- `.github/workflows/task164-final-certification.yml`
- this plan

## Required Gates

1. Exact HEAD + release-scope lineage.
2. Windows x64 Runtime/package build using pinned ast-grep 0.42.1.
3. Full `go test -count=1 ./...`.
4. `go vet ./...`.
5. Task 1 exact semantic parity / process-count / 1.6.3 Workspace AST regression.
6. Task 2 telemetry contract and 21/50/100 Windows performance matrix.
7. Task 3 freshness mutation + canonical projection authority + fast path.
8. 1.6.3 Clean Project Chain Discovery regression.
9. 1.6.3 real OpenCode same-session multi-chain USER_SELECTION E2E.
10. 1.6.3 Upgrade V2 regression and package regression.
11. Retained 1.6.2 Review Reliability real-agent/plain-review/authority gates.
12. Authority ordering: uncertified/failed analysis cannot drive downstream review consumers; certified success still reaches ReviewOptions and 2+ Chains USER_SELECTION.
13. Install/upgrade ZIP manifest, managed inventory, Runtime SHA256, whitelist, exact-head release checklist.
14. Final tracked-source clean + exact HEAD marker.

## Performance Hard Gates

- 21-file certify <= 15s
- 50-file certify <= 20s
- 100-file certify <= 30s
- total Entrypoint AST processes <= 4
- Base cat-file processes <= 1
- per-file merge-base = 0
- per-file git show = 0

Target values remain 5s / 10s / 20s respectively.

## Output

The workflow must upload:

- `codea-harness-1.6.4-windows-x64-install.zip`
- `codea-harness-1.6.4-windows-x64-upgrade.zip`
- `codea-harness-1.6.4-release-checklist.json`
- `codea-dcep-tools-whitelist.txt`
- complete gate logs

No merge to `main`, tag, GitHub Release, or generic packaging-lineage update is performed until the exact candidate is certified and explicitly accepted.
