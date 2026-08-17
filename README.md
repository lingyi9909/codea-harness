# Codea Harness

Codea Harness V1 是一套面向 Java + Spring Boot + Maven 项目的 **Agent 原生规范包**。将 `.code-harness/` 复制到目标项目即可使用，无需安装任何工具。

V1 标准化以下流程：项目适配初始化、基于 Git Diff 的代码评审、以 Controller 为入口的集成测试、本地服务调试、故障诊断，以及经人工审批的最小化修复。

**V1 不提供独立的 CLI 或 Harness Engine。** 所有 `harness xxx` 均为对工程 Agent（Codex、OpenCode 等）的自然语言意图。

---

## 安装

将 `.code-harness/` 目录复制到目标 Java + Spring Boot + Maven 项目根目录：

```bash
cp -R .code-harness /path/to/target-project/.code-harness
```

> 目标项目不需要预先存在任何 Codea Harness 相关文件。只需要一个普通的 Maven + Spring Boot 项目。

---

## 初始化

对工程 Agent 说：

```
读取 .code-harness/bootstrap.md，执行 harness init
```

初始化过程会自动：

- 识别 Maven 命令（优先使用 Maven Wrapper）
- 识别单模块或多模块结构
- 识别 Spring Boot 启动模块和 Controller 模块
- 识别测试目录、测试 Profile 和测试报告路径
- 识别现有测试规范（`@SpringBootTest`、`@AutoConfigureMockMvc`、Mock 方式等）
- 生成 `.code-harness/harness.yaml`
- 生成 `.code-harness/project.md`

**不会修改**业务代码、测试代码、`pom.xml` 或 `application` 配置。

无法从代码中确定的信息会标记为「未确定」并询问用户，不会猜测。

初始化完成后，可以选择在项目根目录 `AGENTS.md` 中增加快捷入口。已有 `AGENTS.md` 时只追加标记区块，不覆盖原有内容。

---

## 使用

### 有快捷入口时

初始化时选择了增加根目录 `AGENTS.md` 快捷入口后，直接对工程 Agent 说：

```
harness review
harness test                # 评审 → 测试计划 → 审批 → 生成测试 → 执行 → 诊断
harness debug-service       # 启动本地服务，采集日志，等待人工触发请求
harness fix finding:F-001   # 针对评审发现生成修复方案
harness fix diagnosis:run-001  # 针对诊断结果生成修复方案
harness verify test:OrderControllerIT  # 重新运行指定测试
harness verify fix:fix-plan-001  # 验证修复是否成功
harness verify service:debug-001  # 重新启动服务，采集日志，验证结果
```

### 无快捷入口时

对工程 Agent 说：

```
使用 .code-harness 执行 harness review
使用 .code-harness 执行 harness test
```

### 项目结构变化后

当项目模块、测试结构或构建方式发生重大变化时，重新执行初始化：

```
读取 .code-harness/bootstrap.md，执行 harness init
```

重复初始化不会重复追加根目录 `AGENTS.md` 快捷入口区块。

### 评审范围（Review Scope）

`harness review` 评审当前开发分支相对于项目默认基线分支，从双方 merge-base 开始由当前开发分支引入的全部代码变化，并默认同时包含 staged、unstaged 和 untracked 本地变化。

```text
Review Change Set = merge-base(baseRef, HEAD) → HEAD 的已提交变更 + staged + unstaged + untracked
```

`harness test` 的第一阶段复用完全相同的 Change Set。

默认使用 `harness.yaml` 中配置的基线：

```
harness review
```

也可临时指定基线（仅本次生效，不修改配置）：

```
harness review base:origin/develop
harness test base:origin/develop
```

Codea Harness **不会自动执行 `git fetch`**。评审使用当前本地已有的 Git refs。

---

## 审批协议

整个流程有两个正式门禁：

1. **测试计划**：Agent 输出测试计划并附带 `planId`。回复 `批准 <planId>` 进行审批。
2. **修复方案**：Agent 输出修复方案并附带 `fixPlanId`。回复 `批准 <fixPlanId>` 进行审批。

模糊的肯定（「好」「继续」「可以」）**不**视为审批。

---

## 安全门禁

- 测试计划审批通过前，不得编写测试代码。
- 修复方案审批通过前，不得修改生产代码。
- Agent 只能使用 `.code-harness/tools/README.md` 中列出的受控工具契约。
- 测试代码最多自动修复 2 轮，超限后停止。
- V1 不会自动提交、推送、创建 PR，也不会在测试/生产环境中自动执行。

---

## 提交到仓库

建议将完整的 `.code-harness/` 目录提交到目标项目的 Git 仓库，仅排除 `runs/` 下的运行产物（已通过 `.code-harness/.gitignore` 忽略）。

这样团队其他成员拉取项目后无需重新初始化。
