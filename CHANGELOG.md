# Changelog

## 1.3.0 - 2026-08-21

- **Review Report Persistence**：Review/Test 的 Review 阶段新增受控、确定性的 `review.md` 持久化，覆盖 PASSED / FAILED / MANUAL_ACTION_REQUIRED，模型不得自由生成最终报告。
- **Frontend API Documentation**：新增 `harness api-doc Controller / Controller.method / changed`、API target selection、Evidence-backed API Contract 与 deterministic `api-doc.md` renderer。
- **Lightweight Code Navigation**：新增 `get_symbol_info`、`find_by_annotation`、`find_callers`，继续通过固定 ast-grep Runtime Contract 提供 Java 静态源码导航。

## 1.2.0 - 2026-08-20

- **Test Target Selection**：多 Controller 变更时由用户明确选择本次测试目标，Selection 独立于 Approval，并将 selected-only scope 贯穿 Existing Test、计划、生成与执行。
- **Database Evidence**：新增 TEST/LOCAL MySQL 的受控只读数据库证据能力，包含本地配置、AST SQL Safety、查询预算/超时/行数限制、脱敏 Evidence 与升级时 `database.yaml` Project-State 保留。
- **Failure Code Navigation**：失败诊断新增代码导航与 evidence-backed root cause；内部 file:line 必须读取实际实现，interface 解析实现类，外部依赖停止推断，证据不足返回 UNKNOWN。

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
