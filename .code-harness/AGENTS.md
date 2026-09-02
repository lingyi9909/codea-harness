# Codea Harness 项目指令

## 范围

本仓库定义 Codea Harness V1。1.3 在已验收的 Review/Test/Debug/Fix/DB/Upgrade 主流程上增量增强 Review Report、API Documentation 和 Lightweight Code Navigation，不允许借此重构既有主流程。

## 核心行为

- Review Change Set 的确定性 Git Fact 只有一个 Authority：Controlled Runtime `analysis snapshot`。Runtime 以 `requestedBaseRef / resolvedBaseCommit / mergeBase / headCommit / currentBranch / includeWorkingTree / canonical files / gitStateSha256 / snapshotSha256` 生成同 run `analysis/change-set.json`；`requestedBaseRef` 只作为 provenance，Git identity 以实际 resolved commit/state 为准。
- Canonical Review Change Set 语义保持 `merge-base` 完整分支差异 + staged + unstaged + untracked，并继续只保留 Harness Review Scope。Agent/Orchestrator 不得再次通过 `git_diff` 独立拼 `baseRef/baseCommit/mergeBase/headCommit/currentBranch/includeWorkingTree/changedFiles.path/changedFiles.sources`。
- `harness review` 与 `harness test` 必须复用 Runtime canonical ChangeSet；`harness api-doc changed` 也复用该 ChangeSet / ChangeAnalysis 的 affectedControllers。
- Reviewer 必须读取 Canonical Snapshot 中所有 required changed source/test files，并使用 Code Navigation Contract 沿与变更直接相关的内部调用链展开。
- `reviewCoverage.status != COMPLETE` 时，review/test 均停止为 `MANUAL_ACTION_REQUIRED`。
- `harness api-doc` 是只读流程，最终 `api-doc.md` 只能由 Controlled Runtime deterministic renderer 生成。
- 集成测试仍以 MockMvc + 真实 Controller/Service/Repository 为主；内部 Bean 默认不 Mock，外部依赖沿用项目测试替代方式。
- 1.5 Chain Management 管理 Business Chain 的发现、读取、验证、刷新和用户确认后的 Project State 持久化；Task 5 新增 `harness chain edit <id|Controller|Controller.method>` 语义编辑，但不得把 Chain 扩成 Review/Test/Debug/Fix/Verify 的写边界。
- Task 5 的 edit 只允许六类 semantic operation；代码事实必须来自 same-run Certified ChangeAnalysis，Runtime 只输出 `analysis/chain-edit-candidates/<id>.yaml` + provenance，edit 本身不得直接写 `.code-harness/chains/**`。
- 1.6.2 Post-release Hotfix 起固定 authority flow：Runtime `analysis snapshot` 先发布 `.code-harness/runs/<runId>/analysis/change-set.json`；Agent 只消费该 Snapshot 做 semantic reasoning，并写 `.code-harness/runs/<runId>/requests/change-analysis-proposal.json`。Proposal 不得包含 Runtime-owned Git identity 或 deterministic changedFiles。
- 权威 ChangeAnalysis 只能由 Controlled Runtime `analysis certify` 重新计算当前 Snapshot identity、重新生成 EntryPoint Inventory、验证 semantic evidence/Coverage，并把 Runtime-owned `reviewScope + changedFiles.path/sources` 与 Agent semantic proposal 组装后生成同 run 的 `analysis/change-analysis.json`、`analysis/entrypoint-inventory.json`、`analysis/change-analysis.cert.json`。
- `analysis certify` 必须重新验证 `resolvedBaseCommit / mergeBase / headCommit / currentBranch / includeWorkingTree / gitStateSha256 / snapshotSha256`。Snapshot 后 Git Review Scope state 发生任何变化都必须 fail closed；错误的 `baseCommit/mergeBase/currentBranch` 不能进入 Certified ChangeAnalysis。
- Chain/Review 等消费者只能通过 Runtime Certified loader 消费上述权威产物；analysis/certificate/inventory/snapshot 任一被篡改、Change Set 已变化或 certification 失败都必须 fail closed，进入 `MANUAL_ACTION_REQUIRED` / `PARTIAL`，不得由 Agent 直接修改 Runtime-owned artifact“修复”。
- 1.5.3 Task 3 起，Generic Agent 的写权限边界是 `requests/**` proposal only；`analysis/**`、`review.md`、`.code-harness/chains/**` 都是 Runtime/Framework Managed artifact。仅把文件放进 Runtime-owned path 不产生 authority，必须有 Runtime provenance 并在消费时重新验证。
- Chain candidate 保存必须经过不可变双阶段授权：Runtime-certified candidate → `chain seal-persist` 生成绑定 candidateHash / analysisHash / expectedExistingHash / previewSha256 的 planId → 用户明确确认当前 planId → `chain persist` 只凭 runId + planId 写入 Project State。candidate、analysis、write plan 或 existing Project State 任一变化都使旧授权失效，必须重新 seal 并重新确认。
- 不声称同一 OS 用户下的 ACL 能提供密码学身份隔离；即使宿主支持路径 ACL，Runtime provenance/hash/certified-analysis revalidation 仍是强制边界。可配置时遵循 `ALLOW requests/**`、`DENY analysis/**|review.md|chains/**` 的 Agent 写权限策略。

## 初始化门禁

`harness init`、`harness review`、`harness api-doc`、`harness chain list/show/discover/refresh/edit/validate`、`harness upgrade` 不要求 READY。`harness test/debug-service/fix/verify` 必须 `initialization.status=READY`。

## Agent 职责

- Reviewer：消费 Runtime Canonical ChangeSet Snapshot，负责 Code Navigation、semantic ChangeAnalysis Proposal、Review Coverage 与 Finding Proposal；不拥有 Git ChangeSet deterministic fact authority。
- API Doc Agent：API target discovery、DTO/Enum/Validation/Direct Service 一层 evidence、结构化 ApiDoc，只读；不得自由写最终 Markdown。
- Integration Test Agent：Existing Test Coverage、测试计划、生成/修复经审批的测试；不执行测试。
- Runtime Debugger：独占测试/服务执行、日志与 Diagnosis。
- Fix Agent：最小 Fix Plan + 经 fixPlanId 审批的生产修改；不执行测试。
- Project Adapter：init 适配与配置生成。
- Orchestrator：路由、触发 Runtime Snapshot/Certification、Review Coverage/审批门禁、API target selection、Chain Management、Agent 交接、测试修复轮次；不得独立重算 Git ChangeSet。

## 审批

- 测试代码修改前必须精确 `批准 <planId>`；REUSE_EXISTING 无需审批。
- 生产代码修改前必须精确 `批准 <fixPlanId>`。
- `harness api-doc` 的 target selection 仅决定只读文档范围，不构成任何写操作审批，也不得要求 `批准 <planId>`。
- Chain refresh/discover/edit candidate 的首次“保存/更新”确认只允许 Runtime 执行 `chain seal-persist` 并展示 exact planId；只有用户随后明确确认该 planId，才允许内部 `chain persist`。任何新 candidate/plan bytes 都必须获得新的确认。
- 「好/继续/可以/yes/ok」不算测试/修复审批；对 Chain Project State 写入，也不能把没有 exact 当前 planId 的模糊肯定解释为 persist 授权。
- 自动测试修复最多 2 轮，且仅限本次 `GENERATED_BY_PLAN`；历史 Existing Test 不自动改。

## 受控 Tool Runtime

`.code-harness/bin/codea-dcep-tools.exe` 是 Harness 背后的确定性工具实现，不是新的产品 CLI。Agent 只可调用固定子命令：

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
codea-harness-tools analysis snapshot --input .code-harness/runs/<runId>/requests/<file>.json
codea-harness-tools analysis inventory --input .code-harness/runs/<runId>/requests/<file>.json
codea-harness-tools analysis certify --input .code-harness/runs/<runId>/requests/<file>.json
codea-harness-tools chain list
codea-harness-tools chain show --target <id|Controller|Controller.method>
codea-harness-tools chain discover --input .code-harness/runs/<runId>/requests/<file>.json
codea-harness-tools chain validate --id <chainId> --change-analysis .code-harness/runs/<runId>/analysis/change-analysis.json
codea-harness-tools chain refresh --input .code-harness/runs/<runId>/requests/<file>.json
codea-harness-tools chain edit --input .code-harness/runs/<runId>/requests/<file>.json
codea-harness-tools chain seal-persist --input .code-harness/runs/<runId>/requests/<file>.json
codea-harness-tools chain persist --input .code-harness/runs/<runId>/requests/<file>.json
codea-harness-tools report review --input .code-harness/runs/<runId>/requests/<file>.json
codea-harness-tools report api-doc --input .code-harness/runs/<runId>/requests/<file>.json
```

Workspace 依赖导航只允许 `analyze-change` 在 current-project superclass/template inheritance 确定性断链时使用；候选只来自显式 `harness.yaml.workspaceDependencies`。必须先 `workspace verify --id <id>`，且只有 `VERIFIED` 才允许三个 `workspace-*` nav 子命令。不得扫描任意 sibling，不得把 dependency workspace 扩成 Change Set、Review Scope 或 Write Scope。

`analysis snapshot` 是 Git ChangeSet authority boundary。Agent → Runtime 的 request JSON 字段固定为：

```json
{
  "runId": "<runId>",
  "baseRef": "<baseRef>",
  "includeWorkingTree": true
}
```

这里的 `baseRef` 是请求参数；Runtime 解析该本地 ref 后，在 Runtime-owned Snapshot artifact 中以 `requestedBaseRef` 保存原始请求值作为 provenance。`requestedBaseRef` 不是 Agent-facing request 字段。Runtime 继续计算 merge-base/HEAD/currentBranch、过滤 canonical Review paths、合并 committed/staged/unstaged/untracked sources，并发布同 run Runtime-owned `analysis/change-set.json`。Agent 不得自行生成替代 Snapshot。

`analysis certify` 是 ChangeAnalysis authority boundary。canonical request 固定为：

```json
{
  "runId": "<runId>",
  "snapshotPath": ".code-harness/runs/<runId>/analysis/change-set.json",
  "snapshotSha256": "<sha256>",
  "proposalPath": ".code-harness/runs/<runId>/requests/change-analysis-proposal.json",
  "intent": {
    "mode": "FULL"
  }
}
```

Runtime 必须重新计算 live Snapshot 并验证 resolved identity/state 完全一致，再把 Runtime-owned deterministic facts 与 Agent semantic proposal 组装为 Certified ChangeAnalysis。Agent 不得直接创建/覆盖 `.code-harness/runs/<runId>/analysis/change-set.json`、`change-analysis.json`、`entrypoint-inventory.json` 或 `change-analysis.cert.json`。

`chain seal-persist` 只接受同 run 的 Runtime-certified candidate，重新加载 Certified ChangeAnalysis、验证 candidate provenance/bytes/code facts 与当前 existing Project State hash，并生成不可变 write plan；它本身不得修改 `.code-harness/chains/**`。

`chain persist` 是内部 Controlled Runtime 写入动作，只接受同 run 的 exact `planId`。Runtime 必须重新验证 sealed plan、Certified ChangeAnalysis、candidate hash、preview hash 与 existing Project State hash 后才可原子写入；最终 persist request 不得重新携带 candidatePath/changeAnalysisPath/expectedExistingHash 改写已确认计划。

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
- Agent/Skill/Orchestrator 不得把 `git_diff` 或自行执行 Git 推导作为 Review ChangeSet Authority，也不得自行填写/覆盖 Runtime-owned Git identity、changedFiles.path 或 changedFiles.sources。
- 不得为让测试通过而删除/禁用测试、弱化断言、吞异常或 Mock 内部 Bean。


### Chain edit authority（1.5.3 Task 5）

`harness chain edit` 只能生成 same-run `requests/**` proposal。Controlled Runtime `codea-harness-tools chain edit --input ...` 基于 Certified ChangeAnalysis 验证后，只能生成 `analysis/chain-edit-candidates/<id>.yaml` + `kind=EDIT` provenance；不得直接修改 `.code-harness/chains/**`。EDIT candidate 的最终保存继续走 `chain seal-persist → exact planId confirmation → chain persist`。
