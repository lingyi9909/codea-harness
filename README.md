# Codea Harness

Codea Harness V1 是面向 Java + Spring Boot + Maven 项目的 **Agent 原生 Harness 规范包**。它仍然不是独立 CLI 产品；用户对 Codex、OpenCode 等工程 Agent 表达 `harness review/test/upgrade`，Agent 按 `.code-harness/` 内的规范、Agent、Skill、Contract 和受控工具执行。

## 1.2.0 变化

1. **Test Target Selection**：当 Review Change Set 影响多个 Controller 时，`harness test` 先让用户明确选择本次测试目标；0 个目标直接结束，1 个目标自动选择，多目标不会默认全选。
2. **Database Evidence**：Runtime Debugger 可在 TEST/LOCAL MySQL 上通过受控只读工具收集数据库证据；所有 SQL 都经过 AST 只读校验、schema allowlist、查询预算、超时、行数上限和 Evidence 落盘门禁。
3. **Failure Code Navigation**：失败分析从日志分类升级为 evidence-backed diagnosis；项目内 stack/file:line 必须读取实际代码，interface 必须解析实现，证据不足时返回 UNKNOWN，不猜外部 RPC 服务端根因。

## 安装

面向公司离线 Windows x64 环境，优先使用 `package-windows-x64` 生成的离线包：

```text
codea-harness-1.2.0-windows-x64.zip
```

解压后将其中 `.code-harness/` 复制到目标项目根目录。

离线包包含：

```text
.code-harness/bin/codea-harness-tools.exe
.code-harness/bin/ast-grep.exe
.code-harness/database.template.yaml
```

源码仓库不直接存储体积较大的第三方 ast-grep 二进制；发布工作流会下载固定版本、校验 SHA-256 后组装到离线包。

## 初始化

对工程 Agent 说：

```text
读取 .code-harness/bootstrap.md，执行 harness init
```

初始化识别 Maven Wrapper/模块、Spring Boot 启动模块、Controller、测试目录/Profile/报告、现有测试规范和 Review baseline，生成：

```text
.code-harness/harness.yaml
.code-harness/project.md
```

无法确定的项目事实进入 `NEEDS_CONFIRMATION`，不得猜测。初始化不会修改业务代码、测试、pom 或 application 配置。

## 主要意图

```text
harness review
harness test
harness debug-service
harness fix finding:F-001
harness fix diagnosis:run-001
harness verify test:OrderControllerIT
harness verify fix:fix-plan-001
harness verify service:debug-001
harness upgrade
```

这些是 Agent 意图，不是新的命令行产品。

## Review Change Set

`harness review` 与 `harness test` 第一阶段共享完全相同的 Change Set：

```text
merge-base(baseRef, HEAD) → HEAD 的 committed
+ staged
+ unstaged
+ untracked
```

默认 baseRef 来自 `harness.yaml.review.baseRef`；可仅本次覆盖：

```text
harness review base:origin/develop
harness test base:origin/develop
```

Harness 不自动 `git fetch`。

## Review Coverage

Reviewer 必须：

- 读取所有 changed source/test files；
- 从 changed Controller/Service/Repository 等符号开始；
- 用 `find_symbol` / `find_references` / `find_implementations` 确定性定位调用链；
- 解析接口到实现类；
- 支持多层 Service；
- 到 Repository/Mapper 或已确认 RPC/MQ/Cache/第三方边界停止。

用户先看到：

```text
Review Scope
Review Coverage
Review Findings
```

如果内部符号无法解析：

```text
reviewCoverage.status = PARTIAL
→ MANUAL_ACTION_REQUIRED
→ 禁止 Review PASSED
→ harness test 禁止进入 Integration Test Agent
```

## Test Target Selection

`harness test` 在 ChangeAnalysis 完成后生成并机器校验 `TestTargetSelection`。

```text
0 affected Controllers
→ NO_TEST_TARGET → DONE

1 affected Controller
→ AUTO_SINGLE

2+ affected Controllers
→ 用户明确选择一个/多个/全部/仅 DIRECT_CHANGE
```

Selection 与 Approval 是两件事：选择测试哪个 Controller 不代表批准写测试代码。未选择 Controller 不得进入 coverage、Test Plan、测试生成或 Runtime execution。

## Existing Test Reuse

测试策略保持不变，每个受影响接口独立判定：

- `REUSE_EXISTING`：充分覆盖，直接执行，不改旧测试。
- `EXTEND_EXISTING`：只补 MISSING 场景，精确 planId 审批后修改。
- `CREATE_NEW`：无合适测试时，精确 planId 审批后新建。

历史 Existing Test 失败禁止自动修改；本次经 planId 生成/修改的方法最多自动修复 2 轮。

## Database Evidence（可选）

需要数据库诊断时，把模板复制为本地 Project State：

```text
.code-harness/database.template.yaml
→ .code-harness/database.yaml
```

`database.yaml`：

- 已被 `.gitignore` 忽略；
- 只允许本地 TEST/LOCAL 环境；
- V1.2 只支持 MySQL；
- 不进入离线发布包；
- 升级时原始 bytes 保留，不被 Framework replace；
- 普通 Agent 不读取其中的密码。

DB Evidence 是**可选诊断能力**。没有 `database.yaml` 时，Harness 其他 review/test/debug 流程仍可继续，只是数据库证据能力不可用。

Runtime Debugger 的数据库访问只允许受控只读路径：

```text
db_ping
db_list_tables
db_describe_table
db_query_readonly
```

禁止 Agent 调用 `mysql.exe` 或任意 Shell 绕过 SQL Safety Gate。

## Failure Code Navigation

失败分析遵循 evidence-first：

```text
Surefire / service logs
→ stack / file:line / symbol
→ read_code / find_implementations / find_references
→ 必要时 bounded Database Evidence
→ Diagnosis
```

项目内生产代码根因只有在实际代码被读取后才能判定 `PRODUCTION_CODE_ERROR`。外部 RPC/HTTP 边界只记录为 external dependency，不猜远端实现；证据或预算不足时必须 `UNKNOWN / STOP_UNKNOWN`。

## Upgrade

把新版离线包中的 `.code-harness/` 放到业务项目根并命名 `.code-harness-upgrade/`。

旧版首次升级：

```text
读取 .code-harness-upgrade/upgrade.md，执行升级
```

1.1.1 及后续：

```text
harness upgrade
```

Upgrade Tool Runtime 实际执行：版本检查 → 包完整性 → migration preflight → 备份 → Framework 更新 → registered migration → 新 Schema 校验 → 成功提交或失败完整回滚。

1.1.1 → 1.2.0 时：

- `.code-harness/database.yaml` 作为 Project State 原样保留；
- `.code-harness/database.template.yaml` 和新 contracts/skills/runtime 文件作为 Framework Managed 正常升级；
- 无效的可选 `database.yaml` 不阻塞 Harness 升级。

`add-review-config-v1` 仅在 `review:` 不存在时按本地 refs 优先级检测 baseRef；已有 `review` 原样保留；无法检测时 0 修改并要求人工配置。

## 审批与安全

- 测试计划写代码前：`批准 <planId>`。
- 生产修复前：`批准 <fixPlanId>`。
- 模糊肯定不算审批。
- 禁止任意 Shell、Shell 求值、管道/重定向/用户命令拼接。
- Agent 不直接调用 ast-grep；只能通过 Code Navigation Contract。
- 不访问生产数据库，不自动安装依赖，不自动 commit/push/PR。
- 服务停止只针对本次 `ServiceHandle.processGroup`。

## 开发与验证

受控 Runtime 源码：

```text
.code-harness/tools-runtime/
```

本地验证：

```text
cd .code-harness/tools-runtime
go test -count=1 ./...
go vet ./...
```

Windows x64 离线包由 `.github/workflows/package-windows-x64.yml` 组装，并验证 Runtime、Code Navigation、Selection、Database Safety、真实 Windows Upgrade 和发布包内容。
