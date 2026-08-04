# Codea Harness 项目交接文档

- 项目仓库：`https://github.com/zhway9909/codea-harness`
- 目标分支：`develop`
- 当前阶段：V1 设计已确认，尚未开始规范包落库与实施
- 交接日期：2026-08-04

## 1. 项目目标

为 Java + Spring Boot + Maven 项目建设一套适配 B 端系统研发场景的 Harness 规范。

V1 不是独立软件平台，也暂不开发 Harness Engine。首期交付以规范、Skills、Subagent、YAML 模板和执行说明为主，由 Codex、OpenCode 或其他研发 Agent 按规范执行。

核心能力：

- 基于 Git Diff 对本次变更涉及的全部代码进行 Code Review；
- 以 Controller 为入口生成集成测试；
- 集成测试真实调用 Controller、Service 和 Repository；
- 使用项目现有测试数据库配置；
- 执行测试并读取 Maven 输出、Surefire 报告及测试期间应用日志；
- 分析测试问题、环境问题和生产代码问题；
- 输出最小修复方案；
- 人工确认后修改代码并重新验证；
- 独立支持本地服务启动、日志收集和联调问题分析。

## 2. 已确认的 V1 边界

### 2.1 Code Review

Code Review 不只检查 Controller，而是覆盖本次 Git Diff 涉及的全部代码，包括：

- Controller；
- Service；
- Repository、DAO、Mapper；
- DTO、VO、Entity；
- 校验器；
- 权限、身份、租户和机构信息处理；
- 异常处理器；
- 配置及与本次变更直接相关的公共组件。

以 Git Diff 为入口，按需读取相关调用链，不扫描整个仓库。

### 2.2 Controller 入口集成测试

测试入口：

```text
MockMvc
→ 真实 Controller
→ 真实 Service
→ 真实 Repository
→ 项目现有测试数据库
```

推荐方式：

```java
@SpringBootTest
@AutoConfigureMockMvc
```

默认不 Mock 项目内部 Service 和 Repository。

外部系统、第三方接口、MQ、RPC 等依赖，沿用项目已有测试方式进行 Mock 或替代。

### 2.3 测试数据

V1 不建设独立测试数据平台。

测试数据优先通过 Controller 请求进入系统，由真实 Controller、Service 和 Repository 完成数据创建、修改和状态流转。

如果测试必须依赖已有数据，则沿用项目现有测试方式。

V1 不统一提供：

- Fixture；
- SQL 初始化；
- 数据隔离；
- 数据清理框架；
- 数据库或中间件自动启动。

### 2.4 本地服务调试

本地服务调试与集成测试是两条独立路径，不强制串联。

Harness 负责：

- 按固定配置启动服务；
- 捕获启动进程 stdout 和 stderr；
- 记录本次启动进程 PID；
- 只停止本次由 Harness 启动的进程；
- 通过日志关键字或健康检查判断服务就绪；
- 记录本次联调时间窗口；
- 收集本次启动及接口调用期间日志；
- 分析启动失败和运行时异常。

接口由研发或前端手工触发，V1 不提供自动 HTTP 请求能力。

### 2.5 明确不做

- 不做任务状态机；
- 不做流程恢复；
- 不做多服务编排；
- 不做完整接口自动化平台；
- 不做前端页面自动化；
- 不自动准备数据库、Redis、MQ；
- 不自动提交代码；
- 不自动推送分支；
- 不自动创建 PR；
- 不在测试环境或生产环境执行。

## 3. 两条执行闭环

### 3.1 集成测试闭环

```text
读取 Git Diff
→ 分析全部变更代码
→ 识别受影响 Controller 和真实调用链
→ Code Review
→ 生成集成测试计划
→ 人工确认
→ 创建或修改集成测试
→ 执行 SpringBootTest + MockMvc
→ 真实调用 Service、Repository 和测试数据库
→ 读取 Maven 输出、Surefire 报告和测试期间日志
→ 分析失败
→ 测试代码问题则修改测试并重跑
→ 生产代码问题则生成最小修复方案
→ 人工确认
→ 修改生产代码
→ 重跑对应集成测试
```

### 3.2 本地服务调试闭环

```text
启动本地服务
→ 判断服务就绪
→ 等待研发或前端触发接口
→ 记录联调时间窗口
→ 收集该时间段服务日志
→ 分析问题
→ 生成最小修复方案
→ 人工确认
→ 修改代码
→ 重启服务
→ 再次由研发或前端验证
```

## 4. Subagent 设计

### Reviewer

职责：

- 分析 Git Diff；
- 读取本次变更涉及的全部代码；
- 按需读取相关调用链；
- 输出文件位置、证据、风险等级、影响和修改建议；
- 标记是否需要补充测试。

只分析，不修改代码。

### Integration Test Agent

职责：

- 根据受影响 Controller 和调用链生成测试计划；
- 人工确认后创建或修改集成测试；
- 使用 MockMvc 作为入口；
- 使用真实 Controller、Service 和 Repository；
- 使用项目现有测试数据库配置；
- 测试代码问题可修改测试并重跑；
- 疑似生产代码问题交给 Fix Agent。

### Runtime Debugger

集成测试模式：

- 执行指定测试；
- 读取 Maven stdout、stderr；
- 读取 Surefire XML、TXT；
- 读取测试期间应用日志；
- 分析失败。

服务调试模式：

- 启动和停止服务；
- 记录 PID；
- 捕获 stdout、stderr；
- 判断服务就绪；
- 等待研发或前端触发请求；
- 读取本次时间窗口日志；
- 分析启动和运行错误。

建议失败分类：

```text
TEST_COMPILE_ERROR
TEST_CODE_ERROR
TEST_CONTEXT_ERROR
TEST_DATA_OR_ENVIRONMENT_ERROR
SERVICE_START_ERROR
PRODUCTION_CODE_ERROR
UNKNOWN
```

### Fix Agent

输入：

- 经人工确认的 Code Review 问题；
- Runtime Debugger 判断为生产代码问题的失败。

流程：

```text
输出根因
→ 输出最小修复方案
→ 标明修改文件和影响范围
→ 人工确认
→ 修改代码
→ 按问题来源重新验证
```

## 5. Skills 目录

建议保持精简：

```text
skills/
├── analyze-change/
├── review-code/
├── design-integration-tests/
├── generate-integration-tests/
├── run-integration-tests/
├── debug-local-service/
├── analyze-failure/
└── fix-bug/
```

对应关系：

```text
Reviewer
- analyze-change
- review-code

Integration Test Agent
- design-integration-tests
- generate-integration-tests

Runtime Debugger
- run-integration-tests
- debug-local-service
- analyze-failure

Fix Agent
- fix-bug
```

## 6. 受控工具契约

V1 需要定义以下工具契约：

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

- Subagent 不执行受控工具之外的命令；
- 测试计划确认前不得写测试文件；
- 修复方案确认前不得修改生产代码；
- 服务命令由 `harness.yaml` 固定配置；
- Agent 不得自行拼接任意 Shell；
- `stop_service` 只能结束本次由 Harness 启动并记录的 PID。

## 7. AGENTS.md 与 harness.yaml

### AGENTS.md

描述：

- 项目结构和模块职责；
- Controller、Service、Repository 开发规范；
- 状态流转规则；
- 事务和数据一致性要求；
- 权限、身份、租户和机构规则；
- 统一响应和异常处理；
- 集成测试规范；
- 测试数据库使用方式；
- 外部依赖 Mock 方式；
- 构建、测试和启动说明；
- 禁止修改目录。

### harness.yaml

保存机器可执行配置，至少包含：

```yaml
project:
  type: maven
  root: .

integrationTest:
  executable: "./mvnw"
  args:
    - "-Dtest=${testClass}"
    - "test"
  reportDir: "target/surefire-reports"
  timeoutSeconds: 600
  profile: "test"

service:
  executable: "./mvnw"
  args:
    - "spring-boot:run"
  startupTimeoutSeconds: 120
  readiness:
    type: "log"
    pattern: "Started Application"
  logFile: "logs/application.log"

scope:
  sourceIncludes:
    - "src/main/java/**/*.java"
  testIncludes:
    - "src/test/java/**/*Test.java"

write:
  allowedTestPaths:
    - "src/test/java/**"
  allowedProductionPaths:
    - "src/main/java/**"
  deniedPaths:
    - ".git/**"
    - ".github/**"
```

服务运行约定：

- stdout 和 stderr 保存到 `harness/runs/<run-id>/service.log`；
- 如果配置了应用日志文件，则额外读取；
- 启动服务时记录 PID；
- 停止时只结束本次 PID。

## 8. 建议目录结构

```text
项目根目录/
├── AGENTS.md
└── harness/
    ├── harness.yaml
    ├── agents/
    │   ├── reviewer.md
    │   ├── integration-test-agent.md
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

`runs/` 只保存执行产物，不承担状态管理或恢复能力。

## 9. 命令入口

```text
harness review
harness test
harness debug-service
harness fix
harness verify
```

自然语言入口映射到相同操作。

## 10. 人工确认点

V1 只保留两个确认点：

1. 集成测试计划确认后，才能创建或修改测试代码；
2. 生产代码修复方案确认后，才能修改生产代码。

本次由 Harness 新生成、但执行失败的测试代码，可直接修复并重跑，无需再次确认。

## 11. 下一位执行人的首批交付

应直接在 `develop` 分支创建：

```text
README.md
AGENTS.md
docs/
├── design/
│   └── codea-harness-v1-design.md
├── plans/
│   └── codea-harness-v1-implementation-plan.md
└── contracts/
    ├── review-output.schema.json
    ├── test-plan.schema.json
    ├── diagnosis.schema.json
    └── fix-plan.schema.json

harness/
├── harness.example.yaml
├── agents/
│   ├── reviewer.md
│   ├── integration-test-agent.md
│   ├── runtime-debugger.md
│   └── fix-agent.md
├── skills/
│   ├── analyze-change/SKILL.md
│   ├── review-code/SKILL.md
│   ├── design-integration-tests/SKILL.md
│   ├── generate-integration-tests/SKILL.md
│   ├── run-integration-tests/SKILL.md
│   ├── debug-local-service/SKILL.md
│   ├── analyze-failure/SKILL.md
│   └── fix-bug/SKILL.md
└── tools/
    └── README.md
```

## 12. 推荐实施顺序

### Task 1：规范骨架

产出：

- README；
- AGENTS 模板；
- harness.example.yaml；
- 目录结构；
- 使用边界说明。

### Task 2：Reviewer 规范

产出：

- analyze-change Skill；
- review-code Skill；
- reviewer Subagent；
- Review 输出 Schema；
- 示例输入输出。

### Task 3：集成测试规范

产出：

- design-integration-tests Skill；
- generate-integration-tests Skill；
- run-integration-tests Skill；
- integration-test-agent；
- Test Plan Schema；
- SpringBootTest + MockMvc 示例。

### Task 4：本地服务调试规范

产出：

- debug-local-service Skill；
- runtime-debugger；
- PID、stdout/stderr 和日志时间窗口约定；
- Failure Diagnosis Schema。

### Task 5：修复闭环

产出：

- analyze-failure Skill；
- fix-bug Skill；
- fix-agent；
- Fix Plan Schema；
- 两个人工确认点说明；
- 最大测试修复和代码修复次数建议均为 2。

### Task 6：验收样例

准备一个最小 Spring Boot 示例项目或示例代码片段，覆盖：

- 正常 Controller 调用链；
- 测试代码错误；
- 测试上下文错误；
- 测试数据或环境错误；
- 生产代码错误；
- 服务启动失败；
- 未知问题；
- 未确认测试计划不得写测试；
- 未确认修复方案不得改生产代码；
- 只停止 Harness 启动的服务进程。

## 13. 验收标准摘要

- 能读取 `AGENTS.md` 和 `harness.yaml`；
- 能评审本次 Diff 涉及的全部代码；
- 能识别受影响 Controller、Service、Repository；
- 能生成 Controller 入口集成测试计划；
- 能生成 `@SpringBootTest + @AutoConfigureMockMvc` 测试；
- 能使用项目现有测试数据库配置；
- 能执行测试并读取 Surefire 报告；
- 能分析测试和生产问题；
- 能启动服务、记录 PID、捕获 stdout/stderr；
- 能读取联调时间窗口内日志；
- 未确认前不写测试或生产代码；
- 不修改无关文件；
- 不引入状态机；
- 不自动提交、推送或创建 PR。

## 14. 当前阻塞

当前对话环境无法向 GitHub 写入：

- GitHub App 写入接口返回 `403 Resource not accessible by integration`；
- 本地 Git 使用 PAT 时无法解析 `github.com`。

因此仓库 `develop` 分支尚未完成正式落库。

下一位执行人需要：

1. 使用具备网络和写权限的环境；
2. 将本交接文档和最终设计文档写入 `develop`；
3. 按 Task 1 至 Task 6 顺序执行；
4. 每个 Task 独立提交并评审。

## 15. 安全提醒

此前多个 GitHub PAT 已明文出现在聊天中，应全部撤销并重新生成。新执行人不要继续使用已暴露的 Token。
