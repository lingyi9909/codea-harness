# Codea Harness

Codea Harness V1 是一套面向 Java + Spring Boot + Maven 项目的规范包，标准化以下流程：基于 Git Diff 的代码评审、以 Controller 为入口的集成测试、本地服务调试、故障诊断，以及经人工审批的最小化修复。

V1 是 **Agent 原生规范包**——不提供独立的 CLI 或 Harness Engine。由工程 Agent（Codex、OpenCode 等）按本仓库中的契约、Agent 指令、Skill 和项目配置执行。

## 快速开始

1. 将 `AGENTS.md` 复制到目标项目根目录，并根据项目实际情况修改。
2. 将 `harness/harness.example.yaml` 复制为 `harness/harness.yaml`，替换其中的示例值。
3. 对工程 Agent 说出以下自然语言意图：

| 意图 | 执行内容 |
|------|---------|
| `harness review` | 分析 Git Diff，评审所有变更代码，输出评审发现 |
| `harness test` | 评审 → 设计测试计划 → 等待审批 → 生成测试 → 执行 → 诊断故障 |
| `harness debug-service` | 启动服务、采集日志、等待人工触发请求、诊断故障 |
| `harness fix finding:<id>` | 针对评审发现生成修复方案 → 等待审批 → 应用修复 → 验证 |
| `harness fix diagnosis:<runId>` | 针对诊断结果生成修复方案 → 等待审批 → 应用修复 → 验证 |
| `harness verify test:<class>` | 重新运行指定测试类并诊断 |
| `harness verify fix:<fixPlanId>` | 重新运行关联测试，验证修复是否成功 |
| `harness verify service:<runId>` | 重新启动本地服务，建立新的日志采集窗口，等待人工触发请求后分析日志并输出验证结果 |

## 审批协议

整个流程有两个正式门禁，需要用户明确审批：

1. **测试计划**：Agent 输出测试计划并附带 `planId`。回复 `批准 <planId>` 进行审批。
2. **修复方案**：Agent 输出修复方案并附带 `fixPlanId`。回复 `批准 <fixPlanId>` 进行审批。

模糊的肯定（「好」「继续」「可以」「yes」「ok」）**不**视为审批。计划内容修改后必须生成新的 ID，原审批自动失效。

## 架构

```
用户意图
    ↓
Orchestrator（路由意图、管理交接、追踪修复轮次）
    ├── Reviewer（分析变更 → 评审代码）
    ├── Integration Test Agent（设计测试 → 等待审批 → 生成测试）
    ├── Runtime Debugger（执行测试/启动服务 → 分析故障）
    └── Fix Agent（设计修复方案 → 等待审批 → 应用修复）
```

完整的路由和交接规范见 `harness/agents/orchestrator.md`。

## 安全门禁

- 测试计划审批通过前，不得编写测试代码。计划以 `planId` 标识，Agent 不能自行审批。
- 修复方案审批通过前，不得修改生产代码。方案以 `fixPlanId` 标识，Agent 不能自行审批。
- Agent 只能使用 `harness/tools/README.md` 中列出的受控工具契约。
- 测试代码最多自动修复 2 轮，超限后停止。
- V1 不会自动提交、推送、创建 PR，也不会在测试/生产环境中自动执行。
