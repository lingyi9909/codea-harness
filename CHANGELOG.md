# Changelog

## 1.5.3 - 2026-08-26

- **Controller EntryPoint Completeness**：Runtime 独立重算 Change Set，并对 changed production Controller 的 endpoint obligation 做机器清单；新增/修改 Controller 不允许因 Agent draft 遗漏而静默跳过，缺失项以 `ENTRYPOINT_COMPLETENESS_INCOMPLETE` fail closed。
- **Certified ChangeAnalysis**：Agent 只提交 proposal；Runtime 绑定 Change Set、EntrypointInventory、证据与原始 intent 后发布 Certified ChangeAnalysis，tamper/stale artifact 不能继续驱动 Review、Chain 或 Chain Edit。
- **Runtime-owned Chain authority**：discover/edit candidate 必须具备 Runtime provenance；持久化统一复用 immutable `seal-persist → planId → persist`，candidate/analysis/existing Chain 任一变化都会拒绝旧 plan。
- **Review selection**：plain `harness review` 在 0 个有效 Chain 时 `AUTO_FULL`，exact 1 个时 `AUTO_SINGLE` 直接执行且不询问用户，2+ 时才进入 Runtime-bound `USER_SELECTION`；FULL/TARGETED/LIST 权限边界保持机器校验。
- **Natural-language Chain Edit**：新增受控 Chain Edit，支持 verified node replace/add/remove/reorder、rename 和 notes；代码事实必须来自 Certified ChangeAnalysis，Workspace Dependency 仅保留真实 navigation identity，未验证关系拒绝成为 Chain 事实。
- **No Chain YAML migration**：继续使用 Chain YAML version 1；升级不会迁移、重写或自动生成既有 `chains/**` Project State。
- **1.5.2 isolation preserved**：Workspace Dependency 仍只扩展 Navigation/Chain Context，不扩展 Change Set、Review/Finding/Test/Fix/Apply Write Scope；1.5.2 Workspace/Review isolation 回归继续作为 release gate。
- **1.5.2 → 1.5.3 Release Gate**：Windows x64 live upgrade byte-for-byte 保持 `harness.yaml`、`project.md`、`database.yaml`、`runs/**`、`chains/**`，安装新增 1.5.3 contracts/skills/runtime，并发布 install / upgrade 双 ZIP。

## 1.5.2 - 2026-08-25

- **Workspace Dependency Chain Navigation**：支持显式配置的 direct sibling Maven source dependency；只有 Maven identity 机器验证为 `VERIFIED` 后才进入跨 workspace 代码导航，workspace dependency 始终只是 **Navigation Scope**，不是 Change Set、Review Scope 或 Write Scope。
- **Maven version safety**：本地解析 current/sibling POM、local parent property 与 dependencyManagement；coordinate/version 无法精确确认时返回稳定 machine code，`VERSION_MISMATCH`、`VERSION_UNRESOLVED` 等状态绝不回退到猜测或公网/JAR 解析。
- **Inheritance / template dispatch**：调用链可以从 current project 进入 dependency superclass/template method，再确定性返回 current concrete override；存在两个真实 override 且无法唯一确认时返回 `AMBIGUOUS_TEMPLATE_DISPATCH`，不选择任意分支。
- **Review Isolation**：workspace dependency evidence 可以作为业务 Chain Context，但 FULL/TARGETED coverage、`scopedFiles`、`Finding.file` 和 Fix/Test write path 均保持 current repository only；dependency 不产生 Finding，也不能被写入。
- **1.5.1 → 1.5.2 Release Gate**：无 harness config migration；正式 Windows x64 升级 byte-for-byte 保持既有 `harness.yaml`、`project.md`、`database.yaml`、`runs/**`、`chains/**`，且不会向既有 `harness.yaml` 自动注入 `workspaceDependencies`；发布 1.5.2 install / upgrade 双 ZIP。

## 1.5.1 - 2026-08-24

- **Chain Discover Bootstrap Fix**：`harness chain discover [target]` 变为自包含流程，直接从 current Change Set 自动 `analyze-change → Schema validate → Runtime machine coverage verify → chain discover`；无需先执行 harness review，也不要求历史 Chain 或既有 Review Run。
- **Freshness-safe reuse**：仅当当前 run 的 verified ChangeAnalysis 与当前 source revision / Change Set 完全一致时才复用；不存在、无法证明一致或已过期时自动重新 analyze-change。
- **Fresh production stack**：当前 Change Set 新增 Controller / Service / ServiceImpl / Mapper / Mapper.xml 可直接发现；committed / staged / unstaged / untracked 全部继续属于 Change Set，新 production Controller Method 可直接成为 Candidate EntryPoint。
- **Machine-fact failure output**：真实 discovery 为 PARTIAL 时直接展示 unresolved symbol 与机器 reason（如 `IMPLEMENTATION_NOT_FOUND`），不再输出“可能需要额外 Code Navigation”等模糊说明。
- **1.5.0 → 1.5.1 Release Gate**：无新增 harness config migration；Windows x64 正式升级继续 byte-for-byte 保持 `harness.yaml`、`project.md`、`database.yaml`、`runs/**`、`chains/**`，并发布 1.5.1 install / upgrade 双 ZIP。

## 1.5.0 - 2026-08-24

- **Chain Management**：新增一链一 YAML 的业务 Chain Project State、lazy discovery、exact V1/V2 canonicalization、`list/show/discover/refresh/validate`、STALE 检测以及用户明确确认后的安全持久化；`chains/**` 永远不是 Framework Managed。
- **Review Consumes Verified Chains**：FULL/TARGETED Review 优先复用当前代码事实仍然 VALID 的 ACCEPTED Chain；缺失时只在当前 Run lazy discover 临时 Chain；STALE Chain 必须由用户明确选择临时使用、刷新或停止，绝不静默复用。
- **Review provenance**：`review.md` 增加业务链名称、Chain ID、来源与状态；临时 Chain 明确提示尚未沉淀。Chain 只补充业务上下文，不改变既有 Change Set、Review Scope、Coverage 或 Finding Gate。
- **1.4.0 → 1.5.0 Upgrade Gate**：正式 Windows x64 升级验证 `harness.yaml`、`project.md`、`database.yaml`、`runs/**`、`chains/**` Project State 保持；其中 `chains/**` byte-for-byte 保持，stale Framework 正常删除，新 Chain Framework 正常安装。
- **Release boundary**：1.5.0 的 Chain 仅接入 Review；**不支持 Test/Debug/Fix Chain**，也不新增 generic Chain rule engine、merge/split/edit/ignore 命令。

## 1.4.0 - 2026-08-22

- **Targeted Review**：新增 `harness review <Class>` / `<Class.method>` 与 `harness review list`；定向 Scope 由 Runtime 基于已验证调用链和 exact path evidence 机器校验，不能把定向结论冒充整个 Change Set 已完成评审。
- **Mapper.xml / YML Review**：`*Mapper.xml` 与 `src/main/resources/**/*.yml` 纳入正式 Review Change Set、FULL Coverage 和 evidence-related TARGETED Scope，仅检查与本次变化相关的高价值 Mapper/配置风险。
- **Human Report UX Standard**：`review.md` 统一中文首屏、机器 role evidence 驱动的调用链角色、🔴🟠🟡🟢 严重级别与确定性排序，并将测试代码 Finding 限定为 `TEST_VALIDITY`，普通测试代码质量问题不再产生 Finding。
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
