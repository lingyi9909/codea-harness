# Codea Harness

Codea Harness V1 是面向 Java + Spring Boot + Maven 项目的 **Agent 原生 Harness 规范包**。它仍然不是独立 CLI 产品；用户对 Codex、OpenCode 等工程 Agent 表达 `harness review/test/upgrade`，Agent 按 `.code-harness/` 内的规范、Agent、Skill、Contract 和受控工具执行。

## 1.1.1 变化

1. **Upgrade 真实落地**：新增 Windows x64 Go Tool Runtime，允许确定性、版本化 Config Migration；旧配置缺 `review` 时可安全补齐，已有 `review` 不覆盖，Schema 失败完整回滚。
2. **Review Call Chain 真实展开**：新增受控 Code Navigation Contract，底层离线包使用 ast-grep；所有 changed files 强制读取，并追踪 Service/Repository/Mapper 等直接相关调用链。
3. **Review Coverage 硬门禁**：覆盖不完整时 `MANUAL_ACTION_REQUIRED`，不得假装 Review PASSED，也不得进入 `harness test` 的测试设计阶段。

## 安装

面向公司离线 Windows x64 环境，优先使用 `package-windows-x64` 生成的离线包 `codea-harness-1.1.1-windows-x64.zip`。解压后将其中 `.code-harness/` 复制到目标项目根目录。

离线包包含：

```text
.code-harness/bin/codea-harness-tools.exe
.code-harness/bin/ast-grep.exe
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

1.1.1 中 Reviewer 必须：

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

## Existing Test Reuse

测试策略保持不变，每个受影响接口独立判定：

- `REUSE_EXISTING`：充分覆盖，直接执行，不改旧测试。
- `EXTEND_EXISTING`：只补 MISSING 场景，精确 planId 审批后修改。
- `CREATE_NEW`：无合适测试时，精确 planId 审批后新建。

历史 Existing Test 失败禁止自动修改；本次经 planId 生成/修改的测试最多自动修复 2 轮。

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
go test ./...
go vet ./...
```

Windows x64 离线包由 `.github/workflows/package-windows-x64.yml` 组装并验证必需二进制。
