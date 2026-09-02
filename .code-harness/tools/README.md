# 受控工具契约

Subagent 只能使用本文件定义的操作。**禁止任意 Shell。** Upgrade / Schema Validate / Code Navigation / Database Evidence / Review Report / API Documentation Report 均有确定性 Go Runtime 实现：

```text
.code-harness/bin/codea-dcep-tools.exe
```

它不是新的 Harness CLI 产品；用户仍然只表达 `harness review/test/api-doc/debug-service/fix/verify/upgrade`。Agent 只能映射到固定子命令，禁止 `cmd /c`、PowerShell、`bash -c`、管道、重定向、命令链接或用户输入命令拼接。

## Runtime 固定入口

```text
codea-dcep-tools.exe upgrade
codea-dcep-tools.exe validate --schema <under .code-harness/contracts> --input <under .code-harness> [--format auto|yaml|json]
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
codea-dcep-tools.exe analysis snapshot --input .code-harness/runs/<runId>/requests/<file>.json
codea-dcep-tools.exe analysis inventory --input .code-harness/runs/<runId>/requests/<file>.json
codea-dcep-tools.exe analysis certify --input .code-harness/runs/<runId>/requests/<file>.json
codea-dcep-tools.exe db ping --run-id <id>
codea-dcep-tools.exe db list-tables --schema <schema> --run-id <id>
codea-dcep-tools.exe db describe-table --schema <schema> --table <table> --run-id <id>
codea-dcep-tools.exe db query --input .code-harness/runs/<runId>/requests/<file>.json
codea-dcep-tools.exe report review --input .code-harness/runs/<runId>/requests/<file>.json
codea-dcep-tools.exe report api-doc --input .code-harness/runs/<runId>/requests/<file>.json
```

未知子命令、目录逃逸、非法 symbol/identifier/annotation name 必须拒绝。`nav` 由 Runtime 以固定参数调用 `.code-harness/bin/ast-grep.exe`；Agent/Skill 不得直接调用或生成 ast-grep 命令。`db query` 不接受 raw SQL CLI 参数。

### Canonical ChangeSet Snapshot

`analysis snapshot` 是 Review/Test/API-doc changed 的唯一 Git ChangeSet authority。Agent-facing request JSON 固定为：

```json
{
  "runId": "<runId>",
  "baseRef": "<baseRef>",
  "includeWorkingTree": true
}
```

`baseRef` 是 Agent → Runtime 请求字段。Runtime 本地解析该 ref 后，在 Snapshot output 中把原始请求值保存为 `requestedBaseRef` provenance；`requestedBaseRef` 不是 request 字段。

Runtime 计算并发布：

```text
.code-harness/runs/<runId>/analysis/change-set.json
```

Snapshot 至少包含：

```text
requestedBaseRef       # provenance only
resolvedBaseCommit
mergeBase
headCommit
currentBranch
includeWorkingTree
files[].path/status/sources/hunks
gitStateSha256
snapshotSha256
```

`requestedBaseRef` 的字符串形式不构成 Git identity；`main / origin/main / refs/heads/main` 只有在本地实际 resolve 到同一 commit 且 canonical Git state 相同时才等价。`files[]` 由 Runtime 按 Harness Review Scope 过滤并合并 committed/staged/unstaged/untracked，同路径多 source 由 Runtime 合并去重。

Agent/Reviewer/Orchestrator **不得**再调用 `git_diff` 独立生成另一套 Review ChangeSet，不得自行生成或修补 `baseCommit/mergeBase/headCommit/currentBranch/includeWorkingTree/changedFiles.path/changedFiles.sources`。Agent 只能消费 Runtime Snapshot 做 semantic analysis，并把 semantic proposal 写入 `requests/change-analysis-proposal.json`。

`analysis certify` 的 canonical request 固定为：

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

Runtime 必须重新计算 live Snapshot 并验证 `resolvedBaseCommit / mergeBase / headCommit / currentBranch / includeWorkingTree / gitStateSha256 / snapshotSha256`；Snapshot 之后 Review Scope Git bytes/state 发生变化必须 fail closed。Runtime 再从 Snapshot 组装正式 `reviewScope` 与 `changedFiles.path/sources`，验证 semantic evidence、EntryPoint Inventory 与 Coverage 后才发布 Certified ChangeAnalysis。

### Review Report 受控入口

`report review` 只消费 `.code-harness/runs/<runId>/requests/` 下的结构化 JSON transport。请求中的 `runId` 必须与 transport 所属 run 目录一致，目录逃逸、绝对路径、非 JSON 输入均拒绝。

Runtime 只能把正式 Review Artifact 写到：

```text
.code-harness/runs/<runId>/review.md
```

Reviewer / Orchestrator 不得使用 arbitrary write_file 自由生成 `review.md`，也不得把 transport JSON 作为正式 `review.json` Artifact。Markdown 必须由 Controlled Runtime 的 deterministic renderer 固定生成；成功生成 `review.md` 后必须删除已消费的 transport JSON。

### API Documentation Report 受控入口

`report api-doc` 只消费 `.code-harness/runs/<runId>/requests/` 下的结构化 JSON transport。transport 固定包含 `runId / harnessVersion / apiDoc`，其中 `apiDoc` 必须由 Runtime 使用 `.code-harness/contracts/api-doc.schema.json` 做 Draft 2020-12 校验。

Runtime 只能把正式 API Documentation Artifact 写到：

```text
.code-harness/runs/<runId>/api-doc.md
```

API Doc Agent / Orchestrator 不得自由生成最终 Markdown；必须调用 deterministic renderer。成功写入 `api-doc.md` 后删除已消费 transport。API Documentation 全程只读，不调用 DB，不进入 Test/Fix approval。

---

## Git / 只读工具

### `git_diff(baseRef, headRef?, includeWorkingTree?) -> DiffResult`

这是历史/低层只读 Diff helper，不再是正常 Review/Test/API-doc changed 的 Authority。正常流程必须使用 Runtime `analysis snapshot`；Reviewer/Orchestrator 不得用 `git_diff` 构造第二套 deterministic changedFiles。

其低层语义仍是：

```text
mergeBase = merge-base(baseRef, HEAD)
committed = mergeBase → HEAD
+ staged
+ unstaged
+ untracked
```

不得用普通工作区 `git diff` 冒充完整 Review；不得自动 fetch/pull。即使宿主暴露该 helper，也不得用其输出覆盖或修补 Runtime Canonical Snapshot。

### `git_refs() -> GitRefsResult`

仅读本地 refs：`currentBranch`、`localBranches`、`remoteBranches`、`originHead`。不联网。该工具可以用于初始化/展示，但不得覆盖 Runtime Snapshot 中的 Git identity。

### `read_code(paths, lineRanges?) -> CodeBundle`

只读 `scope.sourceIncludes` / `scope.testIncludes` 允许的仓库文本，路径不能逃出仓库。Review 时所有 Canonical Snapshot required changed source/test files 必须读取。

### `find_symbol(symbol, scope?) -> SymbolSearchResult`

确定性定位 Java 类/接口/枚举/方法声明。底层当前为 ast-grep，但 Contract 不暴露 ast-grep pattern。

### `find_references(symbol, scope?) -> ReferenceSearchResult`

确定性定位项目内部直接引用/调用。用于 changed Service 反向寻找 Controller/Service 上游，以及调用链继续展开。

### `find_implementations(symbol, scope?) -> ImplementationSearchResult`

定位接口实现/继承实现。例如 `OrderService -> OrderServiceImpl`。无法定位时必须进入 `reviewCoverage.unresolvedSymbols`，不得猜路径。

### `get_symbol_info(symbol, scope?) -> SymbolInfo`

确定性读取 Java 符号声明信息，支持 `CLASS / INTERFACE / ENUM / METHOD / FIELD`。至少返回 `symbol / kind / declaringType / signature / annotations / path / lineStart / lineEnd`；METHOD/FIELD 可返回 `returnType`。当同一输入存在多个可匹配声明（例如重载方法）时必须返回 `AMBIGUOUS_SYMBOL`，Runtime 不得猜选其中一个。

### `find_by_annotation(annotationName, scope?) -> AnnotationSearchResult`

按 Java Annotation 名称查找声明，主要用于 API discovery。输入只允许 `annotationName + scope`，例如 `RestController / RequestMapping / PostMapping / GetMapping`；返回 `symbol / kind / annotation / path / lineStart / lineEnd`。

禁止 Agent 输入或 Runtime 暴露：

```text
ast-grep raw rule
raw pattern
regex
arbitrary query language
```

### `find_callers(symbol, scope?) -> CallerSearchResult`

定位项目源码中 method symbol 的直接调用位置，返回 `callerSymbol / path / line`。V1.3 仅保证现有 ast-grep 可确定识别的静态源码范围，不承诺运行时多态、反射、复杂泛型语义或 Spring Proxy 解析。

六个 current-project 导航 Contract 的 scope 都必须是仓库内相对路径；第一版只支持 Java。目录逃逸必须拒绝。不得无界扫描整个仓库，调用方应从与 Canonical ChangeSet / API target 直接相关的 module/source scope 开始。所有导航都只能通过 Controlled Runtime 的固定 ast-grep pattern 执行，Agent/Skill 不得直接调用 ast-grep 或传入 rule/pattern/regex/query language。

### `workspace_verify(id) -> WorkspaceVerificationResult`

映射到固定命令：

```text
codea-harness-tools workspace verify --id <id>
```

`id` 必须来自显式 `harness.yaml.workspaceDependencies`。Runtime 只验证配置中的 direct Maven sibling workspace dependency；不得扫描任意 sibling。只有机器结果 `VERIFIED` 才允许后续 workspace navigation；`VERSION_UNRESOLVED / COORDINATE_MISMATCH / VERSION_MISMATCH / SOURCE_NOT_FOUND` 均不得读取 dependency 源码作为 confirmed evidence。

### `workspace_inherited(workspace, from, method) -> WorkspaceNavigationResult`

映射到：

```text
codea-harness-tools nav workspace-inherited --workspace <id> --from <symbol> --method <method>
```

只允许已通过 `workspace_verify` 的 `VERIFIED` workspace。用于 current-project symbol 因继承方法断链时解析 dependency superclass method；返回的 workspace/symbol/path/role/source/from 只能原样作为 `WORKSPACE_INHERITANCE` evidence。

### `workspace_superclass_call(workspace, from, method) -> WorkspaceNavigationResult`

映射到：

```text
codea-harness-tools nav workspace-superclass-call --workspace <id> --from <symbol> --method <method>
```

只允许在 VERIFIED dependency source 内继续解析 superclass/template method 的确定性内部调用。PARTIAL/ambiguity 必须停止 confirmed chain，不得猜测。

### `workspace_template_dispatch(workspace, from, hook, concrete?) -> WorkspaceNavigationResult`

映射到：

```text
codea-harness-tools nav workspace-template-dispatch --workspace <id> --from <symbol> --hook <hook> [--concrete <class>]
```

只允许 VERIFIED dependency template dispatch。唯一 concrete override 可返回 `workspace=current` 并继续 current-project confirmed callChain；多 override 必须 `PARTIAL / AMBIGUOUS_TEMPLATE_DISPATCH`。Workspace dependency 始终只属于 Navigation / Chain Context，不得进入 Change Set、Review Scope、Finding Scope 或 Write Scope。

### `read_test_report(runId) -> TestReportBundle`
仅读取配置 `reportDir` 下本次 Maven 运行产物。

### `read_service_logs(runId, from, to) -> LogBundle`
只读取本次 run 时间窗口内 stdout/stderr 与配置应用日志。

### `list_project_tree(root, maxDepth?, includes?, excludes?) -> TreeResult`
目录结构只读，默认排除 `.git`、`target`、`node_modules`、日志与大型制品。

### `read_project_file(path) -> FileContent`
允许 `pom.xml`、Java/test 源码、非生产 application 配置、Maven Wrapper、根 AGENTS；禁止密钥、`.env`、证书/私钥/生产凭据。`.code-harness/database.yaml` 始终禁止通过普通 Agent read tools 读取，即使 environment=TEST/LOCAL。password/token/secret/accessKey/privateKey 等值返回前脱敏。

---

## Schema / Coverage 工具

### `validate_contract(schemaPath, inputPath) -> ValidationResult`

真实实现为 Runtime `validate` 子命令。路径必须在 `.code-harness` 内，Schema 必须在 `.code-harness/contracts/`；失败返回非零状态，不允许 Agent“肉眼认为通过”。

- JSON Schema 使用成熟的 Draft 2020-12 Validator 实现，不在 Harness 内自维护 Schema 语义子集。
- YAML 输入先由标准 YAML parser 解析，再以 JSON-compatible instance 进入同一 JSON Schema Validator。
- JSON 输入直接执行完整 JSON Contract 校验。
- `change-analysis-proposal.schema.json` 只校验 Agent semantic proposal；它不授权 Agent 写 deterministic Git facts。
- `change-set.schema.json` 校验 Runtime-owned Canonical Snapshot；Schema 通过仍不能替代 certify 时的 live snapshot identity revalidation。
- 当 Schema 为 `change-analysis.schema.json` 时，Schema PASS 后 Runtime 必须再次执行机器 Review Coverage 校验：
  - 所有 `changedFiles[].path` 必须出现在 `reviewCoverage.reviewedFiles[].path`；
  - `unresolvedSymbols` 必须为空；
  - Agent 声明的 `reviewCoverage.status` 必须与机器计算结果一致；
  - 任一不满足时返回非零状态，Orchestrator 不得继续 Review/Test。
- `api-doc.schema.json` 由 `report api-doc` 在渲染前再次执行真实 Draft 2020-12 校验；Schema 失败不得生成 `api-doc.md`。

Upgrade 必须使用**新版升级包**的 `harness-config.schema.json` 校验迁移后的配置。

### Database 本地配置边界

- `.code-harness/database.template.yaml` 是 Framework Managed 的无真实凭据模板，默认 `enabled: false`。
- `.code-harness/database.yaml` 是可选本机 Project State，必须被 `.code-harness/.gitignore` 忽略；Harness 不自动创建带真实凭据的该文件。
- Database 配置只允许 `environment: TEST|LOCAL`、`dialect: mysql`；任何 DB 操作前必须由受控 Runtime 使用标准 YAML parser + Draft 2020-12 Schema 校验。
- `database.yaml` 缺失/disabled 时返回 `DATABASE_EVIDENCE_UNAVAILABLE`；仅表示 Database Evidence capability unavailable，不得导致其他 Harness 能力失败。
- Project Adapter 和普通 Agent read tools 不得读取、打印或复制其中的连接凭据。

---

## Database Evidence 受控工具

这些工具只授予 Runtime Debugger：

### `db_ping(runId) -> DBStatus`
映射到固定 `db ping --run-id <id>`，仅用于确认可选 DB capability 是否可用。

### `db_list_tables(schema, runId) -> TableList`
仅在 `allowSchemaDiscovery=true` 且 schema 位于 `allowedSchemas` 时执行。

### `db_describe_table(schema, table, runId) -> ColumnList`
仅在 `allowSchemaDiscovery=true`、schema allowlist 和 identifier Gate 全部通过后执行。

### `db_query_readonly(sql, params, runId, purpose) -> DatabaseEvidence`

Host/Runtime 必须先把结构化请求写入：

```text
.code-harness/runs/<runId>/requests/<file>.json
```

再调用固定 `db query --input <path>`。Runtime 强制：

```text
request path/fields
→ database-config Schema + TEST/LOCAL
→ dbguard mature AST read-only gate
→ maxQueriesPerDiagnosis budget
→ MySQL readonly runtime
→ timeout + maxRows + sensitive-column redaction
→ database-evidence Contract validation
→ .code-harness/runs/<runId>/evidence/db/<queryId>.json
```

硬规则：

```text
Agent never invokes mysql.exe
Agent never reads .code-harness/database.yaml
Agent never receives/logs DB password
Agent never bypasses dbguard
Agent never sends raw SQL as a CLI argument
DB tools use controlled runtime only
```

`QUERY_BUDGET_EXCEEDED` 必须在数据库连接/执行前停止该自动查询。动态值必须使用 `?` + params；重复结果列名必须由 SQL alias 消除，否则 Runtime 拒绝，避免 Evidence 丢字段。

---

## 进程管理工具

### `run_maven_test(testClass, runId) -> ProcessResult`
仅执行 `harness.yaml.integrationTest.executable` + 配置 args，替换 `${testClass}`；强制超时，不经过 Shell。

### `start_service(runId) -> ServiceHandle`
仅执行配置 service executable/args，采集 stdout/stderr，返回 `rootPid/startedAt/processGroup`。

### `stop_service(runId, serviceHandle) -> StopResult`
只停止本次 run 记录的 `processGroup` 进程树，拒绝未知 handle。

---

## 写入工具

### `write_test(path, content, planId) -> WriteResult`
仅限 approved Test Plan，路径必须在 `allowedTestPaths` 且不在 deniedPaths。

### `apply_approved_patch(fixPlanId, changes) -> PatchResult`
仅限 approved Fix Plan 中列出的生产文件，路径必须在 `allowedProductionPaths` 且不在 deniedPaths。

### `write_harness_file(path, content) -> WriteResult`
Project Adapter 只可写 `.code-harness/harness.yaml` / `project.md`；禁止借此改业务源码、测试、pom/application。

### `update_root_agents_entry(approved, content) -> WriteResult`
只有用户明确同意后，才可创建/更新根 `AGENTS.md` 的 `<!-- CODEA-HARNESS:START -->...END` 标记区块，区块外不得修改。

---

## 升级工具

### `upgrade_harness(sourceDir?, targetDir?) -> UpgradeResult`

唯一升级入口；真实实现为 `codea-dcep-tools.exe upgrade`。

### Preflight

1. 读取/校验 current/target SemVer；同版 `ALREADY_UP_TO_DATE`，降级/非法版本 `MANUAL_ACTION_REQUIRED`，0 修改。
2. 新包必须含：

```text
VERSION AGENTS.md bootstrap.md upgrade.md
harness.template.yaml project.template.md
agents/** skills/** contracts/** tools/**
bin/codea-dcep-tools.exe
bin/ast-grep.exe
```

缺失任一项 → `MANUAL_ACTION_REQUIRED`，0 修改。
3. 在任何文件写入前，计算 registered migration 是否可执行。需要人工判断的信息无法确定 → 0 修改停止。

### Framework Managed

新版必须按完整集合 **replace**，不是 overlay。新版不存在的旧 Framework 文件视为 stale，必须删除并进入 `removedFiles[]`：

```text
AGENTS.md bootstrap.md upgrade.md VERSION .gitignore
harness.template.yaml project.template.md
agents/** skills/** contracts/** tools/**
bin/** tools-runtime/**
```

项目状态保留：`project.md`、`runs/**`；`harness.yaml` 仅 registered Config Migration 可最小修改。业务根 `AGENTS.md` 不触碰。

### `add-review-config-v1`

仅当顶层 `review:` 不存在时执行，baseRef 严格按：

```text
origin/HEAD
origin/master
origin/main
origin/develop
master
main
develop
```

然后追加：

```yaml
review:
  baseRef: <detected>
  includeWorkingTree: true
```

已有 review 完全保留。无法识别 → `MANUAL_ACTION_REQUIRED`，0 修改。

### Staged 原子事务与清理语义

```text
备份完整旧 Harness
→ 从旧 Harness 构建 stage
→ stage 中删除全部 Framework Managed
→ 仅复制新版 Framework Managed 到 stage
→ 执行 registered migration
→ 使用 stage 中的新 Schema validate harness.yaml
→ PASS：把 stage 的 Framework Managed 应用到 target，删除 stale
→ 最后替换 codea-dcep-tools.exe
→ 删除 stage + backup + 已消费的 sourceDir
→ UPGRADED
```

失败语义：

```text
任一 apply 步骤失败
→ 使用 backup 恢复旧 Harness
→ rollbackPerformed=true（恢复成功时）
→ 保留 sourceDir 供重新处理
→ 清理 stage + backup
→ UPGRADE_FAILED
```

Windows 下禁止向运行中的 `.code-harness/bin/codea-dcep-tools.exe` 原地写入/覆盖。Runtime 必须先把新 exe 完整写到 staged 文件，再将运行中的旧 exe rename 到 **`.code-harness` 同级、同卷**的临时路径，最后把 staged 新 exe rename 到规范路径。旧 exe 的停放路径必须在 Harness 树外且不能跨卷；旧进程退出后对该临时文件做 best-effort 清理。

禁止 AI 猜配置、自动 re-init、绕过 Runtime 复制/删除升级文件。
