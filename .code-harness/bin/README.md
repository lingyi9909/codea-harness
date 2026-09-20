# Codea Harness Runtime Binaries

The Windows x64 offline release artifact contains:

- `codea-dcep-tools.exe` — deterministic Go Tool Runtime.
- `ast-grep.exe` — pinned ast-grep 0.42.1, used only behind Code Navigation Contract.

`ast-grep.exe` is assembled into the **release artifact** by `.github/workflows/package-windows-x64.yml`; the large upstream binary is not stored directly in Git source. The workflow verifies the official release ZIP SHA-256 before packaging.

Agents must never invoke `ast-grep.exe` directly.
