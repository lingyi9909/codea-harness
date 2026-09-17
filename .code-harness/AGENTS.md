# Codea Harness 1.8 普通 Review 主路径

普通 `/harness-review` 只执行 1.8 report-first 协议：固定入口先运行 `review start` 创建并回读 INCOMPLETE 报告，然后主 Agent 只通过 `codea-review` 完成 `prepare → (必要时等待真实下一用户选择后 select) → finish`。

- `runId` 必须来自本次入口，禁止“取最新 run”。
- 用户 target 是普通文本/结构化 `intent.target`，不得拼入 shell。
- 多链必须展示当前 `optionsHash` 与完整链菜单并结束当前 assistant turn；只有下一条真实用户选择可授权 `select`。HostTurn 只来自 OpenCode tool context，模型参数不得填写 session/message/userConfirmed。
- 只有 prepare 自动得到完整单链，或 select 成功后的 `scope.reads`，才是允许读取/提交的范围。跨 scope、源码 hash/range 变化必须 fail closed。
- 主 Agent 自己完成语义 Review；普通 1.8 Review 不依赖 1.7 及更早的评审编排、认证分析、分段评审、规则分发或旧 finding certification。
- findings 为空也必须调用 `codea-review action=finish`。只有 finish 返回 `execution=COMPLETE` 才能称“评审完成”。`coverage=PARTIAL` 时结论必须是 `UNDETERMINED`，报告保留所有已知 gap。
- 任一步失败或用户未选择时保留已经存在的 INCOMPLETE 报告，不自动重启整轮，不进入修复代码。

对普通 1.8 Review，以上合同是本文件唯一可执行 Review 协议。

## 历史 Review 协议与其他既有能力

下面保留的版本化内容用于旧 run / 升级兼容和非 Review 能力。**其中 1.7 及更早的 ordinary Review begin / Reviewer / certify / ReviewUnit / RuleDispatch / report-review 步骤全部是历史说明，不得用于新的 1.8 `/harness-review`。** Test、Debug、Fix、API Doc、Chain、Upgrade 等非 ordinary Review 规则若未被 1.8 设计修改，继续有效。

# Codea Harness 项目指令

## 1.6.7 单类 Review 入口

优先使用 `/harness-review OrderController` 或 `/harness-review OrderController.method`。该命令先执行 Runtime `review begin`，将真实 `READY/runId` 注入当前主会话。收到已注入的 runId 时不得再次 begin。自然语言 `harness review <target>` 也必须先 begin，再开始语义分析；普通文本分析不能替代正式 Review。

单类评审先定位目标文件，再沿实际 Controller → Service → Mapper/Repository 读取直接依赖，以及有证据关联的 DTO、SQL、配置。复用本轮已取得的导航结果；不要反复全目录注解检索，也不要递归展开无关模块。Runtime 仍保留完整 Git Snapshot；FULL required coverage 和 TARGETED verified scoped coverage 各自照常认证，未读取或未解析的内容必须如实标记。

`PROPOSAL_PREFLIGHT_FAILED` 表示提交结构需要修正：根据错误位置校对 `chain` 与 `chainRefs` 的逐项 symbol/path/workspace，保留本次 runId 修正后重新提交。不得通过删除歧义引用、伪造证据或重启整轮掩盖错误。认证/导航超时、authority 失败、已 FAILED 的 run 应展示具体失败命令和 Runtime progress 后停止；不自动无限重试或重启。

发现超过一条实际调用链时，必须完整展示 Runtime 菜单并等待下一条真实用户选择。Findings 为空也要完成认证和 `report review`。只有同 run 的 `review.md` 存在且 Runtime `REPORT SUCCEEDED` 才能称完成；在此之前不得转去询问是否修复代码。

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

- Reviewer：消费 Runtime Canonical ChangeSet Snapshot，只负责 Code Navigation、semantic ChangeAnalysis Proposal、Review Coverage 与 Finding Proposal；不拥有 Git ChangeSet deterministic fact authority，也不拥有整个 Review orchestration。
- API Doc Agent：API target discovery、DTO/Enum/Validation/Direct Service 一层 evidence、结构化 ApiDoc，只读；不得自由写最终 Markdown。
- Integration Test Agent：Existing Test Coverage、测试计划、生成/修复经审批的测试；不执行测试。
- Runtime Debugger：独占测试/服务执行、日志与 Diagnosis。
- Fix Agent：最小 Fix Plan + 经 fixPlanId 审批的生产修改；不执行测试。
- Project Adapter：init 适配与配置生成。
- Orchestrator：拥有 Review 路由、Runtime 调用、主会话语义评审、Runtime progress 展示与 fail-closed 处理，并继续负责 Review Coverage/审批门禁、API target selection、Chain Management、Agent 交接、测试修复轮次；不得独立重算 Git ChangeSet。

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
codea-dcep-tools.exe upgrade
codea-dcep-tools.exe validate ...
codea-dcep-tools.exe nav find-symbol --symbol <symbol> --scope <repo-relative-scope>
codea-dcep-tools.exe nav find-references --symbol <symbol> --scope <repo-relative-scope>
codea-dcep-tools.exe nav find-implementations --symbol <symbol> --scope <repo-relative-scope>
codea-dcep-tools.exe nav get-symbol-info --symbol <symbol> --scope <repo-relative-scope>
codea-dcep-tools.exe nav find-by-annotation --annotation <annotation-name> --scope <repo-relative-scope>
codea-dcep-tools.exe nav find-callers --symbol <method-symbol> --scope <repo-relative-scope>
codea-dcep-tools.exe workspace verify --id <id>
codea-dcep-tools.exe nav workspace-inherited --workspace <id> --from <symbol> --method <method>
codea-dcep-tools.exe nav workspace-superclass-call --workspace <id> --from <symbol> --method <method>
codea-dcep-tools.exe nav workspace-template-dispatch --workspace <id> --from <symbol> --hook <hook> [--concrete <class>]
codea-dcep-tools.exe review begin
codea-dcep-tools.exe review progress --run-id <runId>
codea-dcep-tools.exe review reviewer-unavailable --run-id <runId>
codea-dcep-tools.exe analysis snapshot --input .code-harness/runs/<runId>/requests/<file>.json
codea-dcep-tools.exe analysis inventory --input .code-harness/runs/<runId>/requests/<file>.json
codea-dcep-tools.exe analysis certify --input .code-harness/runs/<runId>/requests/<file>.json
codea-dcep-tools.exe chain list
codea-dcep-tools.exe chain show --target <id|Controller|Controller.method>
codea-dcep-tools.exe chain discover --input .code-harness/runs/<runId>/requests/<file>.json
codea-dcep-tools.exe chain validate --id <chainId> --change-analysis .code-harness/runs/<runId>/analysis/change-analysis.json
codea-dcep-tools.exe chain refresh --input .code-harness/runs/<runId>/requests/<file>.json
codea-dcep-tools.exe chain edit --input .code-harness/runs/<runId>/requests/<file>.json
codea-dcep-tools.exe chain seal-persist --input .code-harness/runs/<runId>/requests/<file>.json
codea-dcep-tools.exe chain persist --input .code-harness/runs/<runId>/requests/<file>.json
codea-dcep-tools.exe report review --input .code-harness/runs/<runId>/requests/<file>.json
codea-dcep-tools.exe report api-doc --input .code-harness/runs/<runId>/requests/<file>.json
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

`harness chain edit` 只能生成 same-run `requests/**` proposal。Controlled Runtime `codea-dcep-tools.exe chain edit --input ...` 基于 Certified ChangeAnalysis 验证后，只能生成 `analysis/chain-edit-candidates/<id>.yaml` + `kind=EDIT` provenance；不得直接修改 `.code-harness/chains/**`。EDIT candidate 的最终保存继续走 `chain seal-persist → exact planId confirmation → chain persist`。
## 1.6.2 Task 2 Agent → Runtime Invocation Contract

Agent/Orchestrator 写入 `requests/**` 后，Controlled Runtime 必须先按对应 machine-readable request contract 校验，再进入 strict decode 与业务处理：

- `analysis snapshot` → `change-set-request.schema.json`
- `analysis inventory` → `analysis-inventory-request.schema.json`
- `analysis certify` → `analysis-certify-request.schema.json`
- `review options` → `review-options-request.schema.json`

Active Agent 只能调用 `.code-harness/bin/codea-dcep-tools.exe`；`codea-harness-tools` 仅可作为 Go module/import 的历史内部名称存在，不是可执行文件调用名。

`review options` 的 Agent-facing request 固定为：

```json
{
  "runId": "<runId>",
  "changeAnalysisPath": ".code-harness/runs/<runId>/analysis/change-analysis.json"
}
```

需要显式 target 时只允许额外加入可选 `target`。`baseRef`、`requestedBaseRef`、Snapshot identity 以及其他 Git fact 均不是 `review options` request 字段；`baseRef` 只在 `analysis snapshot` / retained legacy analysis request 的既定 Contract 中出现。Unknown field 必须由 request schema fail closed。

`analysis certify` 的 Active Agent 形态仍固定为 canonical request：`runId / snapshotPath / snapshotSha256 / proposalPath / intent`。Schema 中保留的 legacy certify shape 仅用于 Runtime upgrade compatibility，Active Agent 不得生成 legacy `draftPath/baseRef` 形态。

## 1.6.2 Reliability Hotfix — Complete Review Invocation Contract

正式 `harness review` 的 Active Runtime command set 是一个整体，Agent/Orchestrator 不得因为旧白名单遗漏而声称 Runtime 缺少 Finding Certification 或 Report 接口：

```text
codea-dcep-tools.exe review options --input .code-harness/runs/<runId>/requests/<file>.json
codea-dcep-tools.exe review select --input .code-harness/runs/<runId>/requests/<file>.json
codea-dcep-tools.exe review units --run-id <runId>
codea-dcep-tools.exe review dispatch --run-id <runId>
codea-dcep-tools.exe review certify-findings --input .code-harness/runs/<runId>/requests/<file>.json
codea-dcep-tools.exe report review --input .code-harness/runs/<runId>/requests/<file>.json
```

在 Agent 创建正式 request 之前必须先读取对应 machine-readable contract：

```text
finding-certify-request.json → .code-harness/contracts/finding-certify-request.schema.json
report-review.json           → .code-harness/contracts/report-review-request.schema.json
```

`review-output.schema.json` 只描述既有 Review 输出结构，不能作为本请求 schema。
正式 `report review` 的 Agent-facing request contract 只能是 `report-review-request.schema.json`。正式 report request 的 `findings` 固定为 `[]`；Agent raw Finding 只能进入 `requests/finding-proposals.json`，正式 Finding 必须由 Runtime `review certify-findings` 生成 same-run `analysis/certified-findings.json` + `certified-findings.cert.json` 后再由 `report review` 加载。

`changedFiles=[]` 不是提前成功返回条件。0 Change 仍必须执行 `review units → review dispatch → finding-proposals.json=[] → review certify-findings → report review`，并生成 0 Change / 0 Finding 的正式 `review.md`。


## 1.6.2 Reliability Hotfix — Fresh Review Lifecycle

每一次新的顶层 `harness review` 都是独立 Review invocation，必须先调用：

```text
codea-dcep-tools.exe review begin
```

固定生命周期是：`review begin` → fresh runId → `analysis snapshot` → 当前 run 的正常 Review Authority Chain。

`review begin` 只负责由 Runtime 生成唯一 fresh runId 并创建 `.code-harness/runs/<runId>/`；它不得读取 Git、不得计算 ChangeSet、不得生成 `analysis/change-set.json`。`analysis snapshot` 仍是唯一 Git ChangeSet Authority。

`same-run` 只约束单次 Review invocation 内部；下一次用户再次输入 `harness review` 时，上一轮 runId、上一轮 Snapshot、上一轮 ChangeAnalysis、上一轮 0 Change 结论和上一轮 `review.md` 对新 invocation 不具备 Authority。Agent session memory 不得替代本次 `review begin` + `analysis snapshot`。

## 1.6.3 Multi-Chain Review Assistant Turn Gate

`TASK163_USER_SELECTION_TURN_HARD_STOP`

当任何 `harness review`（包括显式 Controller CLASS/METHOD）的 Runtime `review options` 返回 `decision=USER_SELECTION`（2+ 去重后的实际调用链，按 entryPoint + 完整 chain + optional exact refs 计数）时，当前 Assistant Turn 必须只展示 Runtime 生成的选择并询问用户，然后立即结束；只有**下一条用户消息**提供明确选择后才允许继续 same-run Review。

在下一条用户消息到达前，禁止调用 `review select`、`review units`、`review dispatch`、创建 `finding-proposals.json`、执行 Finding Certification 或 `report review`。不得把 Agent 自己选择的 FULL / TARGETED / LIST 冒充用户选择，也不得默认 ALL。AUTO_FULL 与 AUTO_SINGLE 的既有机器直通规则保持不变。显式 target 始终保持 TARGETED；ALL 使用全部当前 Runtime selectionIds，不提供 FULL。Controller CLASS/METHOD 可选择 confirmed 子集，但每条选中链入口必须属于原 target，Runtime scope/units 不得补回未选分支。

## 1.6.6 主 Agent Review 流程

`harness review` 全程由当前主 Agent 编排，并在同一主会话内执行 `analyze-change` 和 `review-code` 的语义工作。本文此前的 Reviewer.analyze-change / Reviewer.review-code 是评审职责名称，不代表必须派生子 Agent。不得调用 task/generic subagent/harness-review-reviewer 来替代本次主 Agent 评审。

固定顺序：`review begin → analysis snapshot → 主 Agent analyze-change → analysis certify → review options → review select → review units → review dispatch → 主 Agent review-code → review certify-findings → report review`。

主 Agent 使用已经安装的 `codea-reviewer-submit` 工具提交语义 JSON，kind 分别为 `change-analysis` 和 `findings`，runId 必须来自本次 `review begin`。工具名称为升级兼容保留，现支持主 Agent；不得因为名称包含 reviewer 就派生子会话。工具自动写入无 BOM proposal 和 Host receipt；主 Agent 不得手工补造 receipt。Runtime 验证当前项目的主会话、对应 assistant message 和 completed submission，把主会话 ID 绑定到分析证书；后续人工选择和 Findings 必须来自该同一主会话，再执行 Snapshot、证据、Coverage、规则及范围认证。

每次 Runtime 调用后用 `review progress --run-id <runId>` 展示 `events[].display`。没有证据的问题不生成 Finding；空 Findings 也必须走 certification 和正式 report。工具未加载或 Host export 失败时，明确报告安装/会话问题；不能假报成功或改写 Runtime artifact。

### 多调用链必须人工选择

`USER_SELECTION` 时完整原样展示 Runtime 返回的 `selectionPrompt`（全部 C1..Cn、调用链及 exact 路径、当前 runId 和完整 optionsHash），然后立即结束当前 Assistant Turn。明确提示下一条用户消息使用 `选择 C1`、`选择 C1,C2`、`全部`（仅 FULL intent）或 `仅列出`。显式 Controller target 的“全部”应提示用户使用列出的全部 selectionIds，例如 `选择 C1,C2`，仍生成 TARGETED。

收到下一条明确用户回复后，用 `codea-reviewer-submit` 的 kind=`selection` 提交标准 ReviewSelectionRequest JSON（runId/mode/optionsHash/selectionIds），然后以生成的 `requests/review-selection.json` 调用 Runtime `review select`。Runtime 会核验当前主会话中菜单之后真实的用户文本、completed tool call 和当前 optionsHash。不得由 Agent 猜测、默认 ALL、复用旧菜单或手工伪造人工选择。LIST 只列出调用链，不能继续 Findings 或 report。

AUTO_FULL/AUTO_SINGLE 保持自动选择，可直接写 request 调用 `review select`。正式编排先形成 Runtime scope，再执行 units/dispatch；多调用链缺少 scope 绝不能默认为 FULL。

请求文件统一 UTF-8 无 BOM；Runtime 兼容 Windows 工具写入的单个 UTF-8 BOM，但 malformed JSON、UTF-16、重复 BOM、未知字段及篡改 artifact 仍被拒绝。
