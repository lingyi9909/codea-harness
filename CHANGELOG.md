# Changelog

## 1.5.0 - 2026-08-24

- **Chain Management**：新增一链一 YAML 的业务 Chain Project State、lazy discovery、exact V1/V2 canonicalization、`list/show/discover/refresh/validate`、STALE 检测以及用户明确确认后的安全持久化；`chains/**` 永远不是 Framework Managed。
- **Review Consumes Verified Chains**：FULL/TARGETED Review 优先复用当前代码事实仍然 VALID 的 ACCEPTED Chain；缺失时只在当前 Run lazy discover 临时 Chain；STALE Chain 必须由用户明确选择临时使用、刷新或停止，绝不静默复用。
- **Review provenance**：`review.md` 增加业务链名称、Chain ID、来源与状态；临时 Chain 明确提示尚未沉淀。Chain 只补充业务上下文，不改变既有 Change Set、Review Scope、Coverage 或 Finding Gate。
- **1.4.0 → 1.5.0 Upgrade Gate**：正式 Windows x64 升级验证 `harness.yaml`、`project.md`、`database.yaml`、`runs/**`、`chains/**` Project State 保持；其中 `chains/**` byte-for-byte 保持，stale Framework 正常删除，新 Chain Framework 正常安装。
- **Release boundary**：1.5.0 的 Chain 仅接入 Review；**不支持 Test/Debug/Fix Chain**，也不新增 generic Chain rule engine、merge/split/edit/ignore 命令。

## 1.4.0 - 2026-08-22

- **Targeted Review**：新增 `harness review <Class>` / `<Class.method>` 与 `harness review list`；定向 Scope 由 Runtime 基于已验证调用链和 exact path evidence 机器校验，不能把定向结论冒充整个 Change Set 已完成评审。
- **Mapper.xml / YML Review**：`*Mapper.xml` 与 `src/main/resources/**/*.yml` 纳入正式 Review Change Set、FULL Coverage 和 evidence-related TARGETED Scope，仅检查与本次变化相关的高价值 Mapper/配置风险。
- **Human Report UX Standard**：`review.md` 统一中文首屏、机器 role evidence 驱动的调用链角色、标准 Finding 块和明确下一步；无可靠 role evidence 时降级为普通代码节点。
- **Runtime Apply Safety**：Fix/Test 正式写入统一经过 Controlled Runtime；审批前 seal exact plan，审批后校验 diff/base hash/path/hard-deny，执行原子多文件 apply/rollback 并生成机器 evidence。
- **1.3.2 → 1.4.0 Upgrade Gate**：registered migration 将旧 `harness.yaml version=1` 升为 v2，并在保留用户现有配置的同时补充 Mapper/YML scope；正式升级由目标版本 Runtime 执行，继续保护 `project.md`、`database.yaml`、`runs/**` 和非 migration 用户配置。

## 1.3.2 - 2026-08-21

- **Review Report UX Fix**：`review.md` 固定 UI 中文化，支持多条真实 `callChains[]` 展示、🔴🟠🟡🟢 严重级别与确定性排序，并将测试代码 Finding 限定为 `TEST_VALIDITY`，普通测试代码质量问题不再产生 Finding。

## 1.3.1 - 2026-08-21

- **Release Packaging Fix**：Windows Release 拆分为 install / upgrade 双 ZIP，新增 `RELEASE-MANIFEST.json`，Upgrade Preflight 对不完整正式包一次性列出全部缺失项并提示不要使用 GitHub Source Code；不改变既有 staged upgrade / Project State / rollback / running-exe replacement 核心语义。

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
