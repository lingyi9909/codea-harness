# Codea Harness V1 设计方案

- 状态：待评审
- 日期：2026-08-03
- 适用范围：Java + Spring Boot + Maven

## 1. 背景与目标

当前系统以 B 端数据提交、审核、发布和状态流转为主。研发通常需要自行编写单元测试、本地启动服务、执行测试、查看日志并定位修复问题。

V1 建设一套运行在研发本地电脑上的 Harness，通过 Skills 与 Subagent 组合，将以下过程形成可复用闭环：

```text
Git Diff 与相关调用链分析
→ Code Review
→ 生成测试计划
→ 人工确认
→ 创建或修改单元测试
→ 启动本地服务
→ 执行本次新增或修改的测试
→ 读取测试输出、启动日志和测试期间服务日志
→ 分析问题
→ 生成最小修复方案
→ 人工确认
→ 修改代码
→ 重跑测试验证
```

V1 同时支持 CLI 和研发 Agent 自然语言入口，两种入口统一调用同一个 Harness 引擎。

## 2. V1 范围

### 包含

- Java + Spring Boot + Maven 单项目；
- 以 Git Diff 为入口，按需读取相关调用链；
- Code Review；
- 单元测试计划生成；
- 人工确认后创建或修改测试；
- 本地服务启动、停止和就绪判断；
- 精确执行本次新增或修改的测试；
- 读取 Maven 输出、服务启动日志和测试期间服务日志；
- 区分测试代码、环境和生产代码问题；
- 生成最小修复方案；
- 人工确认后修改生产代码并重跑测试。

### 暂不包含

- 复杂任务状态机和门禁记录持久化；
- 全模块或全项目回归；
- 完整接口自动化场景库；
- 前端页面自动化；
- 多服务编排和跨服务链路分析；
- 数据库、Redis、配置中心等依赖的独立预检查；
- 自动提交代码、推送分支和创建 PR；
- 测试环境或生产环境执行。

V1 不主动检查外部依赖。Harness 执行项目约定的启动命令；服务达到配置的就绪条件，即可开始测试。启动失败时直接分析启动日志。

## 3. 核心原则

1. **Harness 负责编排，Subagent 负责判断。**
2. **Skill 定义稳定的方法、输入输出和限制。**
3. **Subagent 不直接执行任意 Shell，只能调用受控工具。**
4. **只处理与本次 Git Diff 相关的代码、测试和修复。**
5. **测试代码和生产代码修改均保留必要人工确认。**
6. **优先最小修改，不做无关重构。**

## 4. Subagent 设计

### 4.1 Reviewer

负责：

- 分析 Git Diff；
- 按需读取相关调用链；
- 检查业务逻辑、状态流转、事务、权限和并发风险；
- 输出文件位置、风险、证据和修改建议。

限制：不修改代码，不扫描整个模块，不输出无关重构建议。

Code Review 问题经人工确认后可直接进入修复流程。修复后必须补充或更新测试并执行验证。

### 4.2 Test Agent

负责：

1. 生成测试计划；
2. 人工确认后创建或修改单元测试。

测试计划至少包含：

- 测试场景；
- 前置条件；
- Mock；
- 输入；
- 预期结果。

限制：未确认前不得写测试文件；不得修改生产代码；不得删除已有测试或弱化断言。

### 4.3 Runtime Debugger

负责：

- 启动和停止服务；
- 根据日志关键字或健康检查判断服务就绪；
- 执行本次新增或修改的测试；
- 收集测试输出、启动日志和测试期间服务日志；
- 分析并分类失败原因。

至少区分：编译问题、测试代码问题、Mock 问题、环境或启动问题、生产代码缺陷、无法确定的问题。

限制：只分析，不修改代码。

### 4.4 Fix Agent

输入为经确认的 Review 问题或生产代码缺陷。

流程：

```text
输出根因和最小修复方案
→ 标明修改文件及影响范围
→ 人工确认
→ 应用修复
→ 交回 Runtime Debugger 重跑测试
```

限制：未经确认不得修改生产代码；不得修改无关文件；不得删除测试、弱化断言或吞掉异常。

## 5. Skills 设计

```text
skills/
├── analyze-change/
├── review-code/
├── design-tests/
├── generate-tests/
├── start-service/
├── run-tests/
├── analyze-failure/
└── fix-bug/
```

对应关系：

```text
Reviewer: analyze-change + review-code
Test Agent: design-tests + generate-tests
Runtime Debugger: start-service + run-tests + analyze-failure
Fix Agent: fix-bug
```

## 6. 受控工具

V1 至少提供：

```text
git_diff
read_code
write_test
run_maven_test
start_service
stop_service
read_test_report
read_service_logs
apply_approved_patch
```

约束：

- `write_test` 仅在测试计划确认后使用；
- `apply_approved_patch` 仅在修复方案确认后使用；
- 工具参数必须受 Harness 配置和允许范围约束。

## 7. AGENTS.md 与 harness.yaml

### AGENTS.md

面向 Agent，描述：

- 项目结构和模块职责；
- 业务背景及关键状态流转；
- 构建、启动和测试说明；
- 代码与单元测试规范；
- Review 重点；
- 禁止修改的目录和本地开发注意事项。

Harness 启动任务时先读取项目根目录 `AGENTS.md`。未来如支持子目录级文件，更具体目录的规则优先。

### harness.yaml

面向机器执行，配置：

- 项目标识；
- Maven 构建和测试命令；
- 服务启动和停止命令；
- 日志关键字或健康检查就绪条件；
- 测试报告和服务日志路径；
- 命令及文件访问限制。

说明性、业务性规则放 `AGENTS.md`；可执行参数放 `harness.yaml`，避免重复维护。

## 8. 建议目录

```text
项目根目录/
├── AGENTS.md
└── harness/
    ├── harness.yaml
    ├── agents/
    │   ├── reviewer.md
    │   ├── test-agent.md
    │   ├── runtime-debugger.md
    │   └── fix-agent.md
    ├── skills/
    ├── tools/
    └── runs/<run-id>/
        ├── review.md
        ├── test-plan.md
        ├── test-result.json
        ├── service.log
        ├── diagnosis.md
        └── patch.diff
```

## 9. 使用入口

```text
harness review
harness test
harness fix
harness verify
```

自然语言入口也应转换为相同的 Harness 操作，不复制一套流程实现。

## 10. 人工确认点

V1 只保留两个确认点：

1. 测试计划确认后才能创建或修改测试；
2. 修复方案确认后才能修改生产代码。

## 11. V1 验收标准

1. 能读取 `AGENTS.md` 和 `harness.yaml`；
2. 能从 Git Diff 识别本次变更；
3. 能按需读取相关调用链；
4. Review 结果包含文件位置、证据和建议；
5. 测试计划包含场景、前置条件、Mock、输入和预期结果；
6. 未确认前不写测试文件；
7. 能按配置启动和停止服务；
8. 支持日志关键字或健康检查判断就绪；
9. 只执行本次新增或修改的测试；
10. 能读取测试输出、启动日志和测试期间服务日志；
11. 能区分测试、环境和生产代码问题；
12. 能生成最小修复方案；
13. 未确认前不修改生产代码；
14. 修复后自动重跑相关测试；
15. Subagent 不能执行受控工具之外的任意命令。

## 12. 后续演进

V1 稳定后，再根据实际收益考虑：多项目管理、完整接口场景、数据库与缓存断言、多服务编排、测试数据管理、状态持久化、自动提交和测试环境执行。
