# Codea Harness V1 设计方案

- 状态：待评审
- 日期：2026-08-04
- 技术栈：Java + Spring Boot + Maven

## 1. 目标

面向当前 B 端系统研发流程，建设一套运行在研发本地电脑上的 Harness，形成以下核心闭环：

- 基于 Git Diff 对本次变更涉及的全部代码进行 Code Review；
- 以 Controller 为入口生成集成测试；
- 真实调用 Controller、Service 和 Repository；
- 使用项目现有测试数据库配置执行测试；
- 读取测试结果、应用日志和异常堆栈；
- 分析问题并生成最小修复方案；
- 人工确认后修改代码并重新验证；
- 保留独立的本地服务启动和日志调试能力。

V1 同时支持 CLI 和自然语言入口，两种入口共用同一个 Harness Engine。

## 2. V1 核心边界

### 2.1 Code Review 范围

Code Review 不局限于 Controller。

Reviewer 以 Git Diff 为入口，检查本次变更涉及的全部代码，包括：

- Controller；
- Service；
- Repository / DAO / Mapper；
- DTO、VO、Entity；
- 校验器；
- 权限和身份处理；
- 异常处理器；
- 配置和必要的公共组件；
- 与本次变更直接相关的调用链代码。

V1 不扫描整个仓库，只读取本次 Diff 和与变更直接相关的代码。

### 2.2 测试范围

V1 测试以 Controller 为入口，但验证真实内部调用链：

```text
MockMvc
→ 真实 Controller
→ 真实 Service
→ 真实 Repository
→ 项目现有测试数据库
```

推荐使用：

```java
@SpringBootTest
@AutoConfigureMockMvc
```

V1 不再使用 Mock Service 作为默认方式。

支持验证：

- Controller 请求参数；
- Bean Validation；
- Header、身份、权限、租户和机构信息；
- Controller 到 Service 的真实调用；
- Service 业务规则；
- 状态流转；
- Repository 查询和更新；
- 事务行为；
- HTTP 状态码和响应体；
- 统一异常处理；
- 数据库结果。

### 2.3 数据库、测试数据和外部依赖

数据库使用项目现有测试环境配置。

V1 不建设独立的测试数据准备和管理能力。

测试数据优先通过 Controller 请求进入系统，由真实 Controller、Service 和 Repository 完成数据创建、修改和状态流转。

如果测试场景必须依赖已有数据，则沿用项目现有测试方式。Harness 不统一提供 Fixture、SQL 初始化、数据隔离和清理框架。

Harness 不负责：

- 自动启动数据库；
- 自动创建数据库；
- 自动初始化完整测试环境；
- 自动检查数据库、Redis、MQ 等依赖。

如果项目现有测试配置无法使用，Harness 读取错误并输出问题，不自动修改环境。

对于外部依赖：

- 项目内部 Controller、Service、Repository 使用真实 Bean；
- 第三方接口、外部系统、MQ、远程 RPC 等按项目现有测试方式 Mock 或替代；
- Harness 不建设统一的外部依赖模拟平台。

### 2.4 本地服务调试

本地服务调试作为独立路径保留，不与集成测试强制串联。

Harness 负责：

- 按项目配置启动和停止服务；
- 启动服务时记录本次进程 PID；
- 停止时只结束本次由 Harness 启动的进程；
- 捕获启动进程的 stdout 和 stderr，并保存为本次调试日志；
- 如果配置了应用日志文件，则额外读取该文件；
- 通过日志关键字或健康检查判断服务就绪；
- 记录本次调试时间窗口；
- 收集服务启动日志和接口调用期间日志；
- 分析启动失败和运行时异常。

接口由研发或前端手工触发，Harness 不负责自动调用接口。

### 2.5 V1 不包含

- 任务状态机；
- 流程恢复；
- 多服务编排；
- 自动准备数据库和中间件；
- 前端页面自动化；
- 完整接口自动化平台；
- 自动提交、推送和创建 PR；
- 测试环境或生产环境执行。

## 3. 两条核心执行路径

### 3.1 Controller 入口集成测试

```text
读取 Git Diff
→ 分析全部变更代码
→ 识别受影响 Controller 和真实调用链
→ Code Review
→ 生成集成测试计划
→ 人工确认
→ 创建或修改集成测试
→ 使用 SpringBootTest + MockMvc 执行
→ 真实调用 Service、Repository 和测试数据库
→ 读取 Maven 输出、Surefire 报告和测试期间应用日志
→ 分析失败
→ 测试代码问题则修改测试并重跑
→ 生产代码问题则生成最小修复方案
→ 人工确认
→ 修改生产代码
→ 重跑对应集成测试
```

### 3.2 本地服务调试

```text
启动本地服务
→ 判断服务就绪
→ 等待研发或前端触发接口
→ 记录调试时间窗口
→ 收集该时间段服务日志
→ 分析问题
→ 生成最小修复方案
→ 人工确认
→ 修改代码
→ 重启服务
→ 再次由研发或前端验证
```

两条路径独立执行，不强制串联。

## 4. 核心角色

### 4.1 Reviewer

负责：

- 分析 Git Diff；
- 读取本次变更涉及的全部代码；
- 按需读取相关调用链；
- 输出文件位置、问题、证据、风险级别和修改建议；
- 标记是否需要补充测试。

V1 重点检查：

- Controller 参数和返回；
- Service 业务逻辑；
- 状态流转；
- Repository 查询和更新条件；
- 事务；
- 权限、身份、租户和机构信息；
- 幂等性；
- 异常处理；
- 数据一致性；
- 本次 Diff 直接引入的问题。

Reviewer 只做分析，不修改代码。

### 4.2 Integration Test Agent

负责：

1. 根据受影响 Controller 和调用链生成集成测试计划；
2. 人工确认后创建或修改测试；
3. 使用 MockMvc 作为请求入口；
4. 使用真实 Controller、Service 和 Repository；
5. 使用项目现有测试数据库配置；
6. 测试代码问题可修改测试并重跑；
7. 疑似生产代码问题交给 Fix Agent。

测试计划至少包含：

- 目标 Controller 和接口；
- 关联 Service 和 Repository；
- 测试场景；
- 请求方法、路径和参数；
- Header、身份和权限信息；
- Controller 请求输入；
- 如依赖已有数据，说明项目现有数据准备方式；
- 预期状态流转；
- 预期数据库结果；
- 预期 HTTP 状态码；
- 预期响应体；
- 预期异常；
- 需要 Mock 的外部依赖。

默认覆盖：

- 正常请求；
- 缺少必要参数；
- 参数格式错误；
- Bean Validation 失败；
- 无权限或身份错误；
- 合法状态流转；
- 非法状态流转；
- 数据不存在；
- 重复操作；
- Repository 更新失败或无影响行；
- 事务回滚；
- 返回体和状态码校验。

限制：

- 不使用 Mock Service 代替真实业务逻辑；
- 不访问生产数据库；
- 不自动创建完整测试环境；
- 不删除已有测试；
- 不弱化断言；
- 不为了让测试通过而修改生产逻辑。

### 4.3 Runtime Debugger

支持两种模式。

集成测试模式：

- 执行指定测试；
- 读取 Maven stdout 和 stderr；
- 读取 Surefire XML 和 TXT；
- 读取测试期间应用日志；
- 分析测试失败。

服务调试模式：

- 启动和停止服务；
- 判断服务就绪；
- 等待用户或前端触发请求；
- 读取本次调试时间段日志；
- 分析启动和运行错误。

失败分类保持简单：

```text
TEST_COMPILE_ERROR
TEST_CODE_ERROR
TEST_CONTEXT_ERROR
TEST_DATA_OR_ENVIRONMENT_ERROR
SERVICE_START_ERROR
PRODUCTION_CODE_ERROR
UNKNOWN
```

Runtime Debugger 不修改生产代码。

### 4.4 Fix Agent

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

验证规则：

- 来自集成测试的问题：重跑对应集成测试；
- 来自本地服务调试的问题：重启服务后由研发或前端再次验证。

## 5. Skills

V1 保持精简：

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

## 6. 受控工具

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
- 不允许 Agent 自行拼接启动命令；
- 不自动提交、推送或创建 PR。

## 7. AGENTS.md 与 harness.yaml

### 7.1 AGENTS.md

描述：

- 项目结构和模块职责；
- Controller、Service、Repository 规范；
- 业务状态流转规则；
- 事务和数据一致性要求；
- 身份、权限、租户和机构信息规则；
- 统一响应和异常处理方式；
- 集成测试规范；
- 测试数据库使用说明；
- 外部依赖 Mock 方式；
- 构建、测试和启动说明；
- 禁止修改目录。

### 7.2 harness.yaml

保存机器可执行配置：

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

说明性规则放 `AGENTS.md`，可执行参数放 `harness.yaml`。

服务运行时补充约定：

- Harness 将启动进程的 stdout 和 stderr 写入 `harness/runs/<run-id>/service.log`；
- 如果 `service.logFile` 已配置且文件存在，则同时读取该应用日志；
- Harness 启动服务时记录 PID；
- `stop_service` 只结束本次由 Harness 启动的 PID，不处理研发手工启动的服务。


## 8. 目录结构

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

`runs/` 只保存执行产物，不承担状态管理和恢复能力。

## 9. 命令入口

```text
harness review
harness test
harness debug-service
harness fix
harness verify
```

含义：

- `harness review`：评审本次 Git Diff 涉及的全部代码；
- `harness test`：生成、执行和分析 Controller 入口集成测试；
- `harness debug-service`：启动服务并分析研发或前端联调日志；
- `harness fix`：根据确认问题生成和应用最小修复；
- `harness verify`：按问题来源重跑测试或重新执行服务调试。

自然语言入口映射到相同操作。

## 10. 人工确认点

V1 只保留两个确认点：

1. 集成测试计划确认后，才能创建或修改测试代码；
2. 生产代码修复方案确认后，才能修改生产代码。

修改本次由 Harness 新生成、但执行失败的测试代码，不需要额外确认。

## 11. V1 验收标准

1. 能读取 `AGENTS.md` 和 `harness.yaml`；
2. 能从 Git Diff 识别本次变更涉及的全部代码；
3. 能按需读取相关 Controller、Service、Repository、DTO 和异常处理器；
4. Review 输出包含位置、问题、证据和建议；
5. 能生成 Controller 入口集成测试计划；
6. 测试计划包含请求、调用链、Controller 请求输入、已有数据准备方式和预期数据库结果；
7. 未确认测试计划前不得修改测试文件；
8. 能生成基于 `@SpringBootTest + @AutoConfigureMockMvc` 的测试；
9. Controller、Service 和 Repository 使用真实 Bean；
10. 数据库使用项目现有测试配置；
11. 外部依赖按项目现有方式 Mock；
12. 能精确执行对应测试；
13. 能读取 Maven 输出、Surefire 报告和测试期间应用日志；
14. 能区分测试编译、测试代码、测试上下文、测试环境、服务启动、生产代码和未知问题；
15. 测试代码问题可以修改测试并重跑；
16. 生产代码修改前必须输出修复方案并获得确认；
17. 修复后能重跑对应测试；
18. 能按配置启动和停止本地服务，并只结束本次由 Harness 启动的进程；
19. 能通过日志关键字或健康检查判断服务就绪；
20. 能捕获启动进程 stdout/stderr，并按需读取配置的应用日志文件；
21. 能分析服务启动错误和接口运行时错误；
22. 本地服务调试不负责准备数据库、Redis 和其他外部依赖；
23. V1 不建设 Fixture、SQL 初始化、数据隔离和清理框架；
24. 不修改无关文件；
25. 不自动提交、推送或创建 PR；
26. 不引入任务状态机；
27. Subagent 不执行受控工具之外的任意命令。

## 12. 后续演进

V1 稳定后，再根据实际收益考虑：

- 独立 Service 和 Repository 测试；
- 自动准备测试数据库；
- 数据库和中间件容器化；
- Harness 自动调用接口；
- 完整业务场景测试；
- 多服务编排；
- 自动提交和 PR 流程。
