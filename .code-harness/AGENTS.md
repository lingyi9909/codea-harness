# Codea Harness 项目指令

## 范围

本仓库定义 Codea Harness V1。所有变更应限于规范、契约、Agent 指令、Skill 指令、示例配置和包校验。

## V1 行为约定

- 评审以「当前分支相对基线的 merge-base 完整分支差异 + staged + unstaged + untracked」为入口，只读取直接相关的调用链代码。
- 集成测试以 MockMvc 为入口，使用真实的 Controller、Service、Repository，以及项目现有的测试数据库配置。
- 项目内部的 Service 和 Repository Bean 默认不 Mock。
- 外部系统、第三方接口、MQ、RPC 沿用目标项目已有的测试替代方式。
- 本地服务调试与集成测试执行是两条独立路径。

## 初始化门禁

`harness init` 和 `harness review` 可以在任意状态下执行。

以下意图必须在 `harness.yaml` 中 `initialization.status` 为 `READY` 时才能执行：

- `harness test`
- `harness debug-service`
- `harness fix finding:<id>`
- `harness fix diagnosis:<runId>`
- `harness verify test:<class>`
- `harness verify fix:<fixPlanId>`
- `harness verify service:<runId>`

如果 status 为 `NEEDS_CONFIRMATION`，Orchestrator 必须停止并提示用户先完成初始化确认。

## 审批门禁

- 测试计划审批通过前，不得编写或修改测试代码。审批以 `planId` 精确匹配为准——Agent 不能自行审批。
- 修复方案审批通过前，不得修改生产代码。审批以 `fixPlanId` 精确匹配为准——Agent 不能自行审批。
- 新生成但执行失败的测试，如果问题仅出在测试代码本身，可以直接修复并重跑，无需再次审批。

## 意图路由

Orchestrator（`.code-harness/agents/orchestrator.md`）负责路由所有用户意图：

| 意图 | 调用的 Agent | 需要 READY |
|------|-------------|------------|
| `harness init` | Project Adapter | 否 |
| `harness review` | Reviewer | 否 |
| `harness test` | Reviewer → Integration Test Agent → Runtime Debugger →（如需要）Fix Agent | 是 |
| `harness debug-service` | Runtime Debugger | 是 |
| `harness fix finding:<id>` | Fix Agent → Runtime Debugger | 是 |
| `harness fix diagnosis:<runId>` | Fix Agent → Runtime Debugger | 是 |
| `harness verify test:<class>` | Runtime Debugger | 是 |
| `harness verify fix:<fixPlanId>` | Runtime Debugger | 是 |
| `harness verify service:<runId>` | Runtime Debugger（重新启动服务，建立新的日志采集窗口） | 是 |

## Agent 职责划分

- **Reviewer**：分析变更 + 评审代码。只读，不修改任何文件。
- **Integration Test Agent**：设计测试计划 + 生成/修复测试代码。不负责执行测试，也不负责诊断故障。
- **Runtime Debugger**：执行测试 / 启动服务 + 采集日志 + 诊断故障。拥有故障分类和 nextAction 的唯一决定权。
- **Fix Agent**：设计最小修复方案 + 应用经审批的修改。不负责执行测试。
- **Project Adapter**：分析目标项目结构、构建方式和测试规范，自动生成 `harness.yaml` 和 `project.md`。只在 `harness init` 时调用。
- **Orchestrator**：路由意图、管理 Agent 间交接、执行审批门禁、追踪修复轮次、输出用户摘要。修复轮次追踪是 Orchestrator 的唯一职责，Runtime Debugger 不维护轮次。

## 审批规则

1. 用户必须明确写出精确的 `planId` 或 `fixPlanId` 才算审批通过。例如：`批准 test-plan-20260804-001` 或 `批准 fix-plan-20260804-001`。
2. 模糊的肯定（「好」「继续」「可以」「yes」「ok」）**不**视为审批。
3. 计划内容修改后必须生成新的 ID，原有审批不延续。
4. 如果同时存在多份未审批的计划，必须询问用户要处理哪一份。

## 工具约束

- Subagent 只能使用 `.code-harness/tools/README.md` 中列出的受控工具契约。
- 禁止执行任意 Shell 命令。所有 Maven 和服务命令必须使用 `harness.yaml` 中确切配置的 `executable` 和 `args`。
- 执行 Maven 或服务命令时，必须完整展示最终 executable 和 args。禁止 Shell 求值（`shell=true`、`eval`、`bash -c`、`sh -c`）、管道、重定向和命令链接（`&&`、`;`）。
- `stop_service` 必须停止 `ServiceHandle` 中记录的进程树（使用 `processGroup`），而非单个 PID。
- `write_test` 需要经人工审批的测试计划的 `planId`。
- `apply_approved_patch` 需要经人工审批的修复方案的 `fixPlanId`。

## 测试自动修复限制

- 新生成但失败的测试，最多自动修复 **2 轮**。
- 修复轮次由 Orchestrator 按 `planId` 追踪。
- 2 轮修复失败后，保留真实故障分类，将 `nextAction` 设置为 `MANUAL_TEST_REPAIR_REQUIRED`，停止自动修改，输出失败证据和当前测试文件。
- 修复期间禁止以下行为：
  - 删除测试
  - 添加 `@Disabled`
  - 注释掉断言
  - 弱化断言（如将精确值校验改为仅判断非空）
  - 捕获并忽略异常
  - 将真实内部 Bean 替换为 Mock 以绕过生产问题

## 结果摘要格式

每次意图执行完成或停止后，输出统一摘要：

```
结果：PASSED | FAILED | WAITING_APPROVAL | MANUAL_ACTION_REQUIRED

完成：
- 评审 N 个文件
- 生成 M 个测试类
- 执行 K 个场景

发现：
- X 个生产代码问题
- Y 个测试代码问题

下一步：
- 请批准 <planId> | <fixPlanId>
- 或：所有测试通过，无需进一步操作
```

## 禁止行为

- Subagent 不得构造任意 Shell 命令。
- 不得访问生产数据库。
- 不得自动配置依赖环境。
- 不得自动提交、推送或创建 PR。
- 不得进行无关重构或弱化断言。
