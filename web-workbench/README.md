# Code Harness Web Workbench — T01 development gate

**This is an engineering scaffold, NOT a working Web product.** The source design is PR #67 (`b92549ad...`). T01 must not be marked PASS without the actual company Windows/OpenCode private-model run.

## T01 commands

From `web-workbench/` with preinstalled Node 22 LTS and offline-cached fixed npm dependencies:

```text
npm ci --offline --ignore-scripts --no-audit --no-fund
npm run gen:api
npm run typecheck
npm test
npm run test:windows-smoke -- --manifest <approved-local-company-manifest>
```

The last command deliberately exits nonzero unless the company's live native capabilities can be evidenced. See `docs/windows-t01-runbook.md` and `docs/capability-matrix.md`.

The `probe` CLI uses a loopback-only, redirect-free native HTTP transport and **does not** silently retry a native command. Merely accepting `/command` is never proof of Review completion; there is no simulated native permission, skill, or report success. The required routes and payload field mapping must be established from the company's exact OpenCode build; do not copy routes from a different v2 build without testing.

T02–T14 are **not implemented** in this branch. They remain subject to the independent T01 gate. The design previews in `docs/superpowers/specs/assets/` remain illustrative only.
