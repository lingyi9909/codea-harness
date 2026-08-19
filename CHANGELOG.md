# Changelog

## 1.1.1 - 2026-08-19

### Fixed

- deterministic Harness upgrade with versioned config compatibility migration
- safe migration of missing `review` configuration without overwriting existing project choices
- rollback when migrated `harness.yaml` fails the new schema
- Review call-chain coverage across Service / Repository / Mapper instead of Controller-only review
- hard failure when internal call-chain symbols cannot be resolved

### Added

- controlled Windows x64 Harness Tool Runtime (`codea-harness-tools.exe`)
- controlled Code Navigation Contract backed by bundled ast-grep
- `find_symbol`, `find_references`, `find_implementations`
- explicit, schema-required Review Coverage reporting
- Upgrade and Review Call Chain golden tests
