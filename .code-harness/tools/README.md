# 受控工具契约

Subagent 只能使用本文件定义的操作。**禁止任意 Shell。** 1.1.1 新增的 Upgrade / Schema Validate / Code Navigation 有确定性 Go Runtime 实现：

```text
.code-harness/bin/codea-harness-tools.exe
```

它不是新的 Harness CLI 产品；用户仍然只表达 `harness review/test/upgrade`。Agent 只能映射到固定子命令，禁止 `cmd /c`、PowerShell、`bash -c`、管道、重定向、命令链接或用户输入命令拼接。

## Runtime 固定入口

```text
codea-harness-tools.exe upgrade
codea-harness-tools.exe validate --schema <under .code-harness/contracts> --input <under .code-harness> [--format auto|yaml|json]
codea-harness-tools.exe nav find-symbol --symbol <symbol> --scope <repo-relative-scope>
codea-harness-tools.exe nav find-references --symbol <symbol> --scope <repo-relative-scope>
codea-harness-tools.exe nav find-implementations --symbol <symbol> --scope <repo-relative-scope>
```

未知子命令、目录逃逸、非法 symbol 必须拒绝。`nav` 由 Runtime 以固定参数调用 `.code-harness/bin/ast-grep.exe`；Agent/Skill 不得直接调用或生成 ast-grep 命令。

---

## Git / 只读工具

### `git_diff(baseRef, headRef?, includeWorkingTree?) -> DiffResult`

完整 Review Change Set：

```text
mergeBase = merge-base(baseRef, HEAD)
committed = mergeBase → HEAD
+ staged
+ unstaged
+ untracked
```

同一文件多来源必须合并去重，并记录 `sources`。不得用普通工作区 `git diff` 冒充完整 Review；不得自动 fetch/pull。

### `git_refs() -> GitRefsResult`

仅读本地 refs：`currentBranch`、`localBranches`、`remoteBranches`、`originHead`。不联网。

### `read_code(paths, lineRanges?) -> CodeBundle`

只读 `scope.sourceIncludes` / `scope.testIncludes` 允许的仓库文本，路径不能逃出仓库。Review 时所有 changed source/test files 必须读取。

### `find_symbol(symbol, scope?) -> SymbolSearchResult`

确定性定位 Java 类/接口/枚举/方法声明。底层当前为 ast-grep，但 Contract 不暴露 ast-grep pattern。

### `find_references(symbol, scope?) -> ReferenceSearchResult`

确定性定位项目内部直接引用/调用。用于 changed Service 反向寻找 Controller/Service 上游，以及调用链继续展开。

### `find_implementations(symbol, scope?) -> ImplementationSearchResult`

定位接口实现/继承实现。例如 `OrderService -> OrderServiceImpl`。无法定位时必须进入 `reviewCoverage.unresolvedSymbols`，不得猜路径。

三个导航 Contract 的 scope 都必须是仓库内相对路径；第一版只支持 Java。不得无界扫描整个仓库，调用方应从与 Change Set 直接相关的模块/source scope 开始。

### `read_test_report(runId) -> TestReportBundle`
仅读取配置 `reportDir` 下本次 Maven 运行产物。

### `read_service_logs(runId, from, to) -> LogBundle`
只读取本次 run 时间窗口内 stdout/stderr 与配置应用日志。

### `list_project_tree(root, maxDepth?, includes?, excludes?) -> TreeResult`
目录结构只读，默认排除 `.git`、`target`、`node_modules`、日志与大型制品。

### `read_project_file(path) -> FileContent`
允许 `pom.xml`、Java/test 源码、非生产 application 配置、Maven Wrapper、根 AGENTS；禁止密钥、`.env`、证书/私钥/生产凭据。password/token/secret/accessKey/privateKey 等值返回前脱敏。

---

## Schema / Coverage 工具

### `validate_contract(schemaPath, inputPath) -> ValidationResult`

真实实现为 Runtime `validate` 子命令。路径必须在 `.code-harness` 内，Schema 必须在 `.code-harness/contracts/`；失败返回非零状态，不允许 Agent“肉眼认为通过”。

- YAML 输入：执行 Harness Config Schema 校验。
- JSON 输入：执行真实 JSON Contract 校验，支持 object/array/items/required/additionalProperties/enum/min/max/$defs `$ref` 等当前 Harness Contract 使用的约束。
- 当 Schema 为 `change-analysis.schema.json` 时，Schema PASS 后 Runtime 必须再次执行机器 Review Coverage 校验：
  - 所有 `changedFiles[].path` 必须出现在 `reviewCoverage.reviewedFiles[].path`；
  - `unresolvedSymbols` 必须为空；
  - Agent 声明的 `reviewCoverage.status` 必须与机器计算结果一致；
  - 任一不满足时返回非零状态，Orchestrator 不得继续 Review/Test。

Upgrade 必须使用**新版升级包**的 `harness-config.schema.json` 校验迁移后的配置。

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

唯一升级入口；真实实现为 `codea-harness-tools.exe upgrade`。

### Preflight

1. 读取/校验 current/target SemVer；同版 `ALREADY_UP_TO_DATE`，降级/非法版本 `MANUAL_ACTION_REQUIRED`，0 修改。
2. 新包必须含：

```text
VERSION AGENTS.md bootstrap.md upgrade.md
harness.template.yaml project.template.md
agents/** skills/** contracts/** tools/**
bin/codea-harness-tools.exe
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
→ 最后替换 codea-harness-tools.exe
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

Windows 下禁止向运行中的 `.code-harness/bin/codea-harness-tools.exe` 原地写入/覆盖。Runtime 必须先把新 exe 完整写到 staged/temp 文件，再将运行中的旧 exe rename 到 Harness 目录外的 OS temp，最后把 staged 新 exe rename 到规范路径；这样目标 Harness 树在命令完成时已经是新版。旧进程退出后，OS temp 中的旧 exe 允许做 best-effort 清理。

禁止 AI 猜配置、自动 re-init、绕过 Runtime 复制/删除升级文件。
