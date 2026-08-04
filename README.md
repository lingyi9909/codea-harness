# Codea Harness

Codea Harness V1 is a specification package for Java + Spring Boot + Maven projects. It standardizes Git-diff review, Controller-entry integration tests, local service debugging, failure diagnosis, and approved minimal fixes.

V1 intentionally does **not** ship a standalone Harness Engine. Codex, OpenCode, or another engineering agent executes the contracts, agent instructions, skills, and project configuration in this repository.

## Quick start

1. Copy `AGENTS.md` to the target project root and adapt the project rules.
2. Copy `harness/harness.example.yaml` to `harness/harness.yaml` and replace example values.
3. Install Python 3.10+ dependencies: `python -m pip install pyyaml jsonschema`.
4. Validate the package: `python scripts/validate_package.py`.
5. Ask the engineering agent to run one of: `harness review`, `harness test`, `harness debug-service`, `harness fix`, `harness verify`.

## Safety gates

- A test plan must be approved before test files are written.
- A production fix plan must be approved before production files are modified.
- Agents may only use the controlled tool contracts listed in `harness/tools/README.md`.
- V1 never commits, pushes, creates pull requests, or executes in test/production environments automatically.
