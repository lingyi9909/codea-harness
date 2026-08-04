---
name: adapt-project
description: 自动分析 Java、Spring Boot、Maven 项目并生成 Harness 项目适配配置。
version: 1
agent: project-adapter
tools:
  - list_project_tree
  - read_project_file
  - write_harness_file
---

# 适配目标项目

## 目标

自动扫描目标项目的目录结构、构建配置、模块划分和测试规范，识别关键信息并生成 `harness.yaml` 和 `project.md`。

## 适用场景

- 用户首次执行 `harness init`
- 项目结构发生重大变化，需要重新适配

## 不适用场景

- 项目已经是标准 Maven + Spring Boot 且 `harness.yaml` 可用——不需要重新初始化
- 目标项目不是 Java Maven 项目

## 输入

- 目标项目根目录路径
- 可读取的项目文件

## 允许使用的工具

- `list_project_tree`——读取目录结构（不读文件内容）
- `read_project_file`——读取 `pom.xml`、Java 源码、测试源码、`application` 配置、Maven Wrapper 配置、已有 `AGENTS.md`
- `write_harness_file`——写入 `.code-harness/harness.yaml` 和 `.code-harness/project.md`

## 前置条件

- 目标项目根目录存在 `pom.xml`
- 项目使用 Spring Boot
- 存在至少一个 Java 源文件目录

## 执行步骤

### 1. 扫描目录结构

调用 `list_project_tree(root=., maxDepth=2)` 获取项目顶层目录概览。默认排除 `.git`、`target`、`node_modules`、日志目录。

如需查看更深层结构（如测试目录位置），在特定路径下再次调用并适当增加深度。

### 2. 识别 Maven 命令

按优先级检查：
1. `./mvnw`（Maven Wrapper Unix）
2. `mvnw.cmd`（Maven Wrapper Windows）
3. `mvn`（系统 Maven）

存在 Maven Wrapper 时必须优先使用 Wrapper。

### 3. 识别模块结构

读取根 `pom.xml`：
- 检查 `<modules>` 判断是否多模块
- 检查 `<packaging>pom</packaging>`

对每个子模块读取其 `pom.xml`：
- 检查 `spring-boot-maven-plugin`
- 检查依赖关系

搜索 `@SpringBootApplication` 主类确定启动模块。

搜索 Controller 所在目录确定 Web 模块。

多模块项目生成 Maven 参数时必须包含 `-pl <module> -am`。

### 4. 识别测试规范

最多读取 2-3 个有代表性的已有测试文件（优先集成测试，其次单元测试）。

识别：
- `@SpringBootTest` 用法
- `@AutoConfigureMockMvc` 用法
- `@ActiveProfiles` 中的 profile 值
- 测试基类（如有）
- 测试类命名规则（如 `*Test`、`*IT`、`*IntegrationTest`）
- 测试方法命名规则
- `@MockBean`、`@MockitoBean`、WireMock、Fake 实现等外部依赖替代方式
- 用户、租户、机构上下文的构造方式
- 测试数据准备方式（SQL 初始化、`@BeforeEach`、Builder、Repository 直接操作）
- 是否使用 `@Transactional` 回滚

### 5. 识别测试 Profile

按优先级检查：
1. 已有测试中 `@ActiveProfiles` 明确指定 → 使用该值
2. 存在 `application-test.yml/yaml/properties` → 使用 `test`
3. `pom.xml` 中有 Maven Profile → 使用 `-P<profile>`
4. 多种证据冲突 → 标记为未确定

Profile 直接写入 `integrationTest.args`，不得使用单独不生效的 `profile:` 字段。

### 6. 识别测试报告路径

- 单模块项目：`target/surefire-reports`
- 多模块项目：`<test-module>/target/surefire-reports`

根据实际测试模块位置生成。

### 7. 识别服务启动方式

- 查找 `@SpringBootApplication` 主类所在模块
- 检查该模块的 `pom.xml` 是否包含 `spring-boot-maven-plugin`
- 识别本地开发 Profile（查找 `application-local.*` 或 `application-dev.*`）
- 提取启动类名称用于 readiness 匹配模式
- 检查日志配置确定日志文件位置

readiness 默认基于启动类名生成：
```yaml
readiness:
  type: log
  pattern: Started <ApplicationClassName>
```

如果无法确定具体类名，使用通用模式 `Started`。

如果没有明确的日志文件配置，设置 `logFile: null`。

### 8. 生成 harness.yaml

根据所有识别结果生成 `.code-harness/harness.yaml`。参考模板为 `harness.template.yaml`，但必须根据项目实际情况调整：

- Maven 命令、Profile、模块参数必须来自识别结果
- 测试报告路径必须根据实际模块结构调整
- 日志文件路径能识别则填写，不能则填 `null`
- 所有 `scope` 和 `write` 路径必须根据实际项目结构生成
- denied paths 必须包含 `.code-harness/agents/**`、`.code-harness/skills/**`、`.code-harness/contracts/**`、`.code-harness/tools/**`

### 9. 生成 project.md

根据 `project.template.md` 模板填充所有识别到的信息。

**能从代码中识别的必须自动填写。**

**不能识别的写「未确定」，不得虚构。**

### 10. 输出初始化摘要

```
结果：INITIALIZED | NEEDS_CONFIRMATION | FAILED

已识别：
- Maven 执行方式：./mvnw
- 项目模块：单模块
- Spring Boot 启动模块：order-web
- Controller 模块：order-web
- 测试目录：src/test/java
- 测试 Profile：test
- 测试报告目录：target/surefire-reports
- 服务启动方式：./mvnw spring-boot:run -Dspring-boot.run.profiles=local
- 现有测试规范：@SpringBootTest + @AutoConfigureMockMvc，测试类命名 *IT

未确定：
- 测试数据库是否允许写入
- ExternalRpcClient 应使用哪种替代方式
```

## 输出

- `.code-harness/harness.yaml`——使用 `write_harness_file` 写入
- `.code-harness/project.md`——使用 `write_harness_file` 写入
- 初始化摘要（已识别 / 未确定项列表）

## 停止条件

- 目标项目没有 `pom.xml` → 报告并停止
- 找不到 Spring Boot 主类 → 报告并停止
- 所有信息已识别且无未确定项 → INITIALIZED
- 存在未确定项 → NEEDS_CONFIRMATION

## 禁止行为

- 不得修改 `src/main/**` 下的任何文件
- 不得修改 `src/test/**` 下的任何文件
- 不得修改 `pom.xml`
- 不得修改 `application*.yml`、`application*.yaml`、`application*.properties`
- 不得连接数据库
- 不得安装依赖
- 不得虚构不能从代码中确认的信息
- 不得在未经用户同意的情况下修改根目录 `AGENTS.md`
- 不得猜测有冲突的信息——标记为未确定

## 示例

### 单模块项目

```
输入：简单 Spring Boot 项目，根目录有 pom.xml 和 mvnw

识别结果：
- Maven：./mvnw
- 模块：单模块
- 启动类：com.example.DemoApplication
- Controller：src/main/java/com/example/controller/
- 测试：src/test/java/，使用 *Test 命名
- Profile：test（application-test.yml 存在）
- 报告：target/surefire-reports
- 日志：null（无明确日志文件配置）

生成的 harness.yaml：
version: 1
project:
  type: maven
  root: .
  module: ""
integrationTest:
  executable: ./mvnw
  args:
    - -Dspring.profiles.active=test
    - -Dtest=${testClass}
    - test
  reportDir: target/surefire-reports
  timeoutSeconds: 600
service:
  executable: ./mvnw
  args:
    - spring-boot:run
    - -Dspring-boot.run.profiles=local
  startupTimeoutSeconds: 120
  readiness:
    type: log
    pattern: Started DemoApplication
  logFile: null
...
```

### 多模块项目

```
输入：多模块 Maven 项目，order-web 是 Web 模块

生成的 harness.yaml 中：
integrationTest:
  executable: ./mvnw
  args:
    - -pl
    - order-web
    - -am
    - -Dspring.profiles.active=test
    - -Dtest=${testClass}
    - test
  reportDir: order-web/target/surefire-reports
```
