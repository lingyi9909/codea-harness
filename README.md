# Codea Harness

Codea Harness V1 是面向 Java + Spring Boot + Maven 项目的 **Agent 原生 Harness 规范包**。它不是独立 CLI 产品；用户对工程 Agent 表达 `harness review/test/api-doc/debug-service/fix/verify/upgrade`，Agent 按 `.code-harness/` 内的规范、Agent、Skill、Contract 和受控工具执行。

## 1.3.0 变化

1. **Review Report Persistence**：`harness review` 与 `harness test` 的 Review 阶段把结构化结果交给 Controlled Runtime，确定性生成 `.code-harness/runs/<runId>/review.md`；PASSED / FAILED / MANUAL_ACTION_REQUIRED 都有正式 Artifact。
2. **Frontend API Documentation**：新增 `harness api-doc <Controller|Controller.method|changed>`，基于 Controller、DTO、Enum、Validation 和 Direct Service evidence 生成前端 API 文档，唯一正式产物为 `.code-harness/runs/<runId>/api-doc.md`。
3. **Lightweight Code Navigation**：在既有 `find_symbol / find_references / find_implementations` 上新增 `get_symbol_info / find_by_annotation / find_callers`，继续由受控 Runtime 封装固定 ast-grep 能力。

## 安装

面向公司离线 Windows x64 环境，优先使用 `package-windows-x64` 生成的离线包：

```text
codea-harness-1.3.0-windows-x64.zip
```

解压后将其中 `.code-harness/` 复制到目标项目根目录。

离线包包含受控 Runtime、固定版本 ast-grep、Agent/Skills/Contracts 以及无真实凭据的 `database.template.yaml`。`.code-harness/database.yaml` 是本机 Project State，不进入发布包。

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
harness api-doc OrderController
harness api-doc OrderController.approve
harness api-doc changed
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

`harness review`、`harness test` 与 `harness api-doc changed` 复用同一 Change Set：

```text
merge-base(baseRef, HEAD) → HEAD 的 committed
+ staged
+ unstaged
+ untracked
```

Harness 不自动 `git fetch`。

## Review Coverage 与 Review Report

Reviewer 必须读取所有 changed source/test files，并使用 Code Navigation 沿直接相关内部调用链展开。`reviewCoverage.status != COMPLETE` 时 review/test 停止为 `MANUAL_ACTION_REQUIRED`。

Review 阶段的正式 Artifact 固定为：

```text
.code-harness/runs/<runId>/review.md
```

数据流固定：

```text
structured Review result
→ .code-harness/runs/<runId>/requests/<transport>.json
→ Controlled Runtime
→ deterministic Markdown renderer
→ review.md
→ 删除已消费 transport
```

模型不得自由写最终 `review.md`。

## Frontend API Documentation

支持：

```text
harness api-doc OrderController
harness api-doc OrderController.approve
harness api-doc changed
```

`changed` 模式：

```text
Review Change Set
→ Reviewer.analyze-change
→ Runtime validate change-analysis
→ validated affectedControllers
→ API target selection
→ API Doc Agent
```

这里只复用 ChangeAnalysis 识别目标，不执行 `review-code`、不生成 Findings/review.md，也不进入 Test/Fix。

API 文档分析深度固定：

```text
Controller
→ Request DTO
→ Response DTO/VO
→ Enum
→ Validation
→ Direct Service Method（最多一层）
→ STOP
```

不继续 Repository / Mapper / DAO / DB / MQ / Redis / RPC Server。

Request 参数明确区分：

```text
@RequestBody   → BODY
@RequestParam  → QUERY
@PathVariable  → PATH
@RequestHeader → HEADER
```

Validation 只承载真实校验约束，不用 transport annotation 冒充参数位置。

唯一正式 Artifact：

```text
.code-harness/runs/<runId>/api-doc.md
```

结构化 `apiDoc` 先通过 Draft 2020-12 Schema，再由 Controlled Runtime deterministic renderer 生成 Markdown。

## Lightweight Code Navigation

受控导航 Contract：

```text
find_symbol
find_references
find_implementations
get_symbol_info
find_by_annotation
find_callers
```

Agent 只能提供 symbol / annotation / repo-relative scope，不得传 raw ast-grep rule、pattern、regex 或 arbitrary query language。

V1.3 仍只保证 Java + 当前 ast-grep 可确定识别的静态源码范围；不承诺运行时多态、反射、复杂泛型或 Spring Proxy 解析。

## Test Target Selection

`harness test` 在 ChangeAnalysis 完成并通过 Runtime Coverage Gate 后选择 Controller：

```text
0 → NO_TEST_TARGET
1 → AUTO_SINGLE
2+ → 用户明确选择
```

多 Controller 不默认 ALL。Selection 与 Approval 独立，未选择 target 不得进入测试 coverage、计划、生成或执行。

## Existing Test Reuse

每个 selected target 独立判定：

- `REUSE_EXISTING`：充分覆盖，直接执行，不改旧测试。
- `EXTEND_EXISTING`：只补 MISSING 场景，精确 planId 审批后修改。
- `CREATE_NEW`：无合适测试时，精确 planId 审批后新建。

历史 Existing Test 失败禁止自动修改；本次经 planId 生成/修改的方法最多自动修复 2 轮。

## Database Evidence（可选）

把模板复制为本地 Project State：

```text
.code-harness/database.template.yaml
→ .code-harness/database.yaml
```

`database.yaml`：

- 已被 `.gitignore` 忽略；
- 只允许 TEST/LOCAL；
- V1 仅支持 MySQL；
- 不进入离线发布包；
- Upgrade 原始 bytes 保留，不被 Framework replace；
- 普通 Agent 不读取其中凭据。

数据库访问只允许受控只读路径，并经过 schema allowlist、AST SQL Safety、查询预算、超时、行数上限和脱敏 Evidence Gate。

## Failure Code Navigation

失败分析遵循 evidence-first：

```text
Surefire / service logs
→ stack / file:line / symbol
→ read_code / Code Navigation
→ 必要时 bounded Database Evidence
→ Diagnosis
```

内部生产代码根因只有在实际代码被读取后才能判定。外部 RPC/HTTP 边界不猜远端实现；证据不足必须 `UNKNOWN / STOP_UNKNOWN`。

## Upgrade

把新版离线包中的 `.code-harness/` 放到业务项目根并命名 `.code-harness-upgrade/`，然后执行：

```text
harness upgrade
```

`1.2.0 → 1.3.0` 使用既有 staged 原子升级语义：

```text
版本/包完整性检查
→ 备份
→ Framework Managed replace
→ registered migration
→ 新 Schema 校验
→ staged apply
→ 最后替换 Windows 运行中 exe
→ 成功清理 source/stage/backup
```

Project State 持续保护：

```text
harness.yaml
project.md
database.yaml
runs/**
```

其中 `database.yaml` 必须 byte-for-byte 保持不变；无效的可选 `database.yaml` 不作为升级 blocker。

## 审批与安全

- 测试代码修改前必须精确 `批准 <planId>`。
- 生产代码修改前必须精确 `批准 <fixPlanId>`。
- `harness api-doc` 全程只读，不进入上述审批。
- 禁止任意 Shell 求值、管道/重定向/用户命令拼接。
- Agent 不直接调用 ast-grep。
- 不访问生产数据库，不自动安装依赖，不自动 fetch/pull。
- 服务停止只针对本次 `ServiceHandle.processGroup`。

## 开发与验证

受控 Runtime：

```text
.code-harness/tools-runtime/
```

本地 Gate：

```text
cd .code-harness/tools-runtime
go test -count=1 ./...
go vet ./...
```

Windows x64 Release Gate 由 `.github/workflows/package-windows-x64.yml` 执行，覆盖 Runtime、Navigation、Review/API Doc renderer、Selection、Database Safety、`1.2.0 → 1.3.0` live upgrade、package completeness、`database.yaml` leak check 和离线 ZIP artifact。
