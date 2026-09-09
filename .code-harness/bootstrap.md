# Codea Harness 初始化入口

当用户要求执行 `harness init` 时：

1. 读取 `.code-harness/AGENTS.md`，了解 Harness 通用规则和安全约束。
2. 读取 `.code-harness/agents/orchestrator.md`，了解完整意图路由。
3. 由 Orchestrator 调用 Project Adapter（`.code-harness/agents/project-adapter.md`）。
4. Project Adapter 使用 `adapt-project` Skill（`.code-harness/skills/adapt-project/SKILL.md`）分析目标项目。
5. 根据识别结果自动生成：
   - `harness.yaml`——项目可执行配置
   - `project.md`——项目适配信息
6. 输出初始化结果摘要。如果存在无法从项目文件中确定的信息，列出未确定项。
7. **只询问无法从项目文件中确定的信息**。能从代码中自动识别的内容必须自动填写，不得询问用户。
8. 不得修改业务代码、测试代码、`pom.xml` 或 `application` 配置文件。
9. 未经用户明确同意，不得修改目标项目根目录的 `AGENTS.md`。

## OpenCode Reviewer Host Gate（1.6.4）

首次接入后，所有 `harness review` 语义阶段还必须读取：

```text
.code-harness/contracts/reviewer-host-contract.md
```

正式 release 必须把 canonical Reviewer 注册为项目级 OpenCode subagent：

```text
.opencode/agents/reviewer.md
mode: subagent
```

`harness review` 的 semantic analysis 与 finding proposal 必须由该独立 Reviewer child session 执行；Main Agent / Orchestrator 只负责路由、Runtime 调用和门禁，不得代替 Reviewer 做 semantic review。

如果 Reviewer 无法 resolve/start/invoke/complete，或者没有产出可供 Runtime 校验的有效 proposal，固定输出并立即停止：

```text
REVIEWER_UNAVAILABLE
MANUAL_ACTION_REQUIRED
HARD STOP
```

此状态之后不得继续 analysis certification、review planning/dispatch、finding certification 或 report publication，也不得由 Main Agent / Orchestrator 生成 semantic proposal 作为 fallback。

---

`bootstrap.md` 是用户第一次接入 Codea Harness 时唯一需要主动指定读取的文件。后续所有操作（`harness review`、`harness test` 等）由 Orchestrator 按 `.code-harness/agents/orchestrator.md` 中的路由自动执行。