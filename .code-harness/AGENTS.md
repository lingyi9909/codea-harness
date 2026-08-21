# Codea Harness 项目指令

## 范围

本仓库定义 Codea Harness V1。1.3 在已验收的 Review/Test/Debug/Fix/DB/Upgrade 主流程上增量增强 Review Report、API Documentation 和 Lightweight Code Navigation，不允许借此重构既有主流程。

## 核心行为

- Review Change Set = merge-base 完整分支差异 + staged + unstaged + untracked。
- `harness review` 与 `harness test` 必须复用相同 Change Set。
- Reviewer 必须读取所有 changed source/test files，并使用 Code Navigation Contract 沿与变更直接相关的内部调用链展开。
- `reviewCoverage.status != COMPLETE` 时，review/test 均停止为 `MANUAL_ACTION_REQUIRED`。
- 集成测试仍以 MockMvc + 真实 Controller/Service/Repository 为主；内部 Bean 默认不 Mock，外部依赖沿用项目测试替代方式。

## 初始化门禁

`harness init`、`harness review`、`harness upgrade` 不要求 READY。`harness test/debug-service/fix/verify` 必须 `initialization.status=READY`。

## Agent 职责

- Reviewer：Change Set + Code Navigation + Review Coverage + Findings，只读。
- Integration Test Agent：Existing Test Coverage、测试计划、生成/修复经审批的测试；不执行测试。
- Runtime Debugger：独占测试/服务执行、日志与 Diagnosis。
- Fix Agent：最小 Fix Plan + 经 fixPlanId 审批的生产修改；不执行测试。
- Project Adapter：init 适配与配置生成。
- Orchestrator：路由、Review Coverage/审批门禁、Agent 交接、测试修复轮次。

## 审批

- 测试代码修改前必须精确 `批准 <planId>`；REUSE_EXISTING 无需审批。
- 生产代码修改前必须精确 `批准 <fixPlanId>`。
- 「好/继续/可以/yes/ok」不算审批；计划变化后旧审批失效。
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
codea-harness-tools report review --input .code-harness/runs/<runId>/requests/<file>.json
```

禁止 `cmd /c`、`powershell -Command`、`bash -c`、shell 求值、管道、重定向或用户命令拼接。Code Navigation 由 Runtime 封装随包 `ast-grep.exe`；Agent/Skill 不得直接调用 ast-grep、raw rule、raw pattern、regex 或 arbitrary query language。

## Upgrade 规则

- 允许**确定性的、版本化的 registered Config Migration**；禁止 AI 猜配置。
- 已存在的 Project State 必须继续保护。
- baseRef 无法按既定优先级识别 → 0 修改 `MANUAL_ACTION_REQUIRED`。
- migration 后必须用新版 Schema 校验；失败完整回滚，`rollbackPerformed=true`。

## 禁止行为

- 不得访问生产数据库或生产资源。
- 不得自动安装依赖、git fetch/pull、commit/push/PR。
- 不得直接执行任意 Shell。
- 不得为让测试通过而删除/禁用测试、弱化断言、吞异常或 Mock 内部 Bean。
