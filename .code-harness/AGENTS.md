# Codea Harness 项目指令

## 范围

本仓库定义 Codea Harness V1。1.3 在已验收的 Review/Test/Debug/Fix/DB/Upgrade 主流程上增量增强 Review Report、API Documentation 和 Lightweight Code Navigation，不允许借此重构既有主流程。

## 核心行为

- Review Change Set = merge-base 完整分支差异 + staged + unstaged + untracked。
- `harness review` 与 `harness test` 必须复用相同 Change Set；`harness api-doc changed` 也复用该 Change Set / ChangeAnalysis 的 affectedControllers。
- Reviewer 必须读取所有 changed source/test files，并使用 Code Navigation Contract 沿与变更直接相关的内部调用链展开。
- `reviewCoverage.status != COMPLETE` 时，review/test 均停止为 `MANUAL_ACTION_REQUIRED`。
- `harness api-doc` 是只读流程，最终 `api-doc.md` 只能由 Controlled Runtime deterministic renderer 生成。
- 集成测试仍以 MockMvc + 真实 Controller/Service/Repository 为主；内部 Bean 默认不 Mock，外部依赖沿用项目测试替代方式。
- 1.5 Chain Management 只管理 Business Chain 的发现、读取、验证、刷新和用户确认后的 Project State 持久化；Task 3 不把 Chain 接入 Review/Test/Debug/Fix/Verify。
- 1.5.3 起 Agent/Orchestrator 产生的 ChangeAnalysis 只能先写 `.code-harness/runs/<runId>/requests/change-analysis-draft.json`；它只是 proposal，不是权威事实。
- 权威 ChangeAnalysis 只能由 Controlled Runtime `analysis certify` 重新计算 Change Set、EntryPoint Inventory、Coverage 与 evidence invariants 后生成到同 run 的 `analysis/change-analysis.json`、`analysis/entrypoint-inventory.json`、`analysis/change-analysis.cert.json`。
- Chain/Review 等消费者只能通过 Runtime Certified loader 消费上述权威产物；analysis/certificate/inventory 任一被篡改、Change Set 已变化或 certification 失败都必须 fail closed，进入 `MANUAL_ACTION_REQUIRED` / `PARTIAL`，不得由 Agent 直接修改 Runtime-owned artifact“修复”。

## 初始化门禁

`harness init`、`harness review`、`harness api-doc`、`harness chain list/show/discover/refresh/validate`、`harness upgrade` 不要求 READY。`harness test/debug-service/fix/verify` 必须 `initialization.status=READY`。

## Agent 职责

- Reviewer：Change Set + Code Navigation + Review Coverage + Findings，只读。
- API Doc Agent：API target discovery、DTO/Enum/Validation/Direct Service 一层 evidence、结构化 ApiDoc，只读；不得自由写最终 Markdown。
- Integration Test Agent：Existing Test Coverage、测试计划、生成/修复经审批的测试；不执行测试。
- Runtime Debugger：独占测试/服务执行、日志与 Diagnosis。
- Fix Agent：最小 Fix Plan + 经 fixPlanId 审批的生产修改；不执行测试。
- Project Adapter：init 适配与配置生成。
- Orchestrator：路由、Review Coverage/审批门禁、API target selection、Chain Management、Agent 交接、测试修复轮次。

## 审批

- 测试代码修改前必须精确 `批准 <planId>`；REUSE_EXISTING 无需审批。
- 生产代码修改前必须精确 `批准 <fixPlanId>`。
- `harness api-doc` 的 target selection 仅决定只读文档范围，不构成任何写操作审批，也不得要求 `批准 <planId>`。
- Chain refresh 只有在当前 diff/candidate 已展示后，用户明确确认保存/更新该条 Chain，才允许内部 `chain persist`；该确认只授权该 candidate，不等同 Test/Fix Approval。
- 「好/继续/可以/yes/ok」不算测试/修复审批；计划变化后旧审批失效。对 Chain Project State 写入，也不得在缺少明确保存/更新对象与当前 refresh diff 上下文时把模糊肯定当作确认。
- 自动测试修复最多 2 轮，且仅限本次 `GENERATED_BY_PLAN`；历史 Existing Test 不自动改。

## 受控 Tool Runtime

`.code-harness/bin/codea-harness-tools.exe` 是 Harness 背后的确定性工具实现，不是新的产品 CLI。Agent 只可调用固定子命令：

```text
codea-harness-tools upgrade
codea-harness-tools validate ...
codea-harness-tools nav find-symbol --symbol <symbol> --scope <repo-relative-scope>
codea-harness-tools nav find-references --symbol <symbol> --scope <repo-relative-scope>
codea-harness-tools nav find-implementations --symbol <symbol> --scope <repo-relative-scope>
codea-harness-tools nav get-symbol-info --symbol <symbol> --scope <repo-relative-scope>
codea-harness-tools nav find-by-annotation --annotation <annotation-name> --scope <repo-relative-scope>
codea-harness-tools nav find-callers --symbol <method-symbol> --scope <repo-relative-scope>
codea-harness-tools workspace verify --id <id>
codea-harness-tools nav workspace-inherited --workspace <id> --from <symbol> --method <method>
codea-harness-tools nav workspace-superclass-call --workspace <id> --from <symbol> --method <method>
codea-harness-tools nav workspace-template-dispatch --workspace <id> --from <symbol> --hook <hook> [--concrete <class>]
codea-harness-tools analysis inventory --input .code-harness/runs/<runId>/requests/<file>.json
codea-harness-tools analysis certify --input .code-harness/runs/<runId>/requests/<file>.json
codea-harness-tools chain list
codea-harness-tools chain show --target <id|Controller|Controller.method>
codea-harness-tools chain discover --input .code-harness/runs/<runId>/requests/<file>.json
codea-harness-tools chain validate --id <chainId> --change-analysis .code-harness/runs/<runId>/analysis/change-analysis.json
codea-harness-tools chain refresh --input .code-harness/runs/<runId>/requests/<file>.json
codea-harness-tools chain persist --input .code-harness/runs/<runId>/requests/<file>.json
codea-harness-tools report review --input .code-harness/runs/<runId>/requests/<file>.json
codea-harness-tools report api-doc --input .code-harness/runs/<runId>/requests/<file>.json
```

Workspace 依赖导航只允许 `analyze-change` 在 current-project superclass/template inheritance 确定性断链时使用；候选只来自显式 `harness.yaml.workspaceDependencies`。必须先 `workspace verify --id <id>`，且只有 `VERIFIED` 才允许三个 `workspace-*` nav 子命令。不得扫描任意 sibling，不得把 dependency workspace 扩成 Change Set、Review Scope 或 Write Scope。

`analysis certify` 是 ChangeAnalysis authority boundary：请求必须位于同 run 的 `requests/**`，Runtime 独立重算并验证后才发布 authoritative artifacts。Agent 不得直接创建/覆盖 `.code-harness/runs/<runId>/analysis/change-analysis.json`、`entrypoint-inventory.json` 或 `change-analysis.cert.json`。

`chain persist` 是内部 Controlled Runtime 写入动作，只能在 `validate-chain` / Orchestrator 已满足用户确认、candidate validation 与 expected-hash 门禁后调用；它不是独立用户意图。

禁止 `cmd /c`、`powershell -Command`、`bash -c`、shell 求值、管道、重定向或用户命令拼接。Code Navigation 由 Runtime 封装随包 `ast-grep.exe`；Agent/Skill 不得直接调用 ast-grep、raw rule、raw pattern、regex 或 arbitrary query language。

## Upgrade 规则

- 允许**确定性的、版本化的 registered Config Migration**；禁止 AI 猜配置。
- 已存在的 Project State 必须继续保护。
- baseRef 无法按既定优先级识别 → 0 修改 `MANUAL_ACTION_REQUIRED`。
- migration 后必须用新版 Schema 校验；失败完整回滚，`rollbackPerformed=true`。

## 禁止行为

- 不得访问生产数据库或生产资源。
- API Documentation 不得读取真实数据库，也不得超过 Controller → DTO/VO → Enum/Validation → Direct Service Method 一层的分析深度。
- 不得自动安装依赖、git fetch/pull、commit/push/PR。
- 不得直接执行任意 Shell。
- 不得为让测试通过而删除/禁用测试、弱化断言、吞异常或 Mock 内部 Bean。
