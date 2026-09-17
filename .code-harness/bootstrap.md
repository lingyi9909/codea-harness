# Codea Harness 初始化入口

`bootstrap.md` 只负责首次接入和顶层路由。普通 Review 使用 Codea Harness 1.8 report-first 协议，不再携带 pre-1.8 ordinary Review 执行链。

## Codea Harness 1.8 普通 Review

当用户执行 `/harness-review <target>`：

1. 命令入口固定运行 `review start`，在模型第一条回复前创建并回读本 run 的 INCOMPLETE `review.md`。
2. 主 Agent 从入口获得本次 `runId`，不得查找“最新 run”。
3. 主 Agent 只使用结构化 `codea-review`：先 `prepare`；若有多个实际调用链，展示当前 `optionsHash` 和完整菜单并结束当前 turn；只有下一条真实用户选择才能触发 `select`；最后由主 Agent自主评审并调用 `finish`。
4. `scope.reads` 只定义上下文可读范围。正式 finding 必须由 Runtime 重新验证其 evidence 是否属于所选方法/链路，不能借同文件可读性扩到 sibling method。
5. `CURRENT_IMPLEMENTATION` 不允许标记 `introducedByChange=true`；`CHANGES` 也必须由 Runtime 对真实 Git diff 行重新核验。
6. findings 为空仍调用 `finish`；只有返回 `execution=COMPLETE` 才能称完成。PARTIAL coverage 固定为 `UNDETERMINED`。
7. 失败、取消、源码变化或多链尚未选择时保留 INCOMPLETE 报告，不自动重开 run，不进入代码修复。

历史 ordinary Review 说明已独立放在 `.code-harness/history/ordinary-review-pre-1.8.md`，仅用于理解旧 run，不是 active instruction。

## 首次初始化

当用户要求 `harness init`：

1. 读取 `.code-harness/AGENTS.md` 的通用安全、审批和能力边界。
2. 读取 `.code-harness/agents/orchestrator.md` 的当前意图路由。
3. 由 Orchestrator 调用 Project Adapter 和 `adapt-project` Skill 分析目标项目。
4. 只生成 Harness 需要的 `harness.yaml`、`project.md` 等适配信息。
5. 能从项目代码/配置确定的内容自动填写，只询问无法从项目事实确定的项。
6. 未经用户明确同意，不修改业务代码、测试代码、构建配置或目标项目根目录的其他说明文件。

## 其他命令路由

- `harness test`：Integration Test Agent 规划，Runtime Debugger 执行；测试修改仍要求 exact planId 审批。
- `harness debug-service`：Runtime Debugger。
- `harness fix ...`：Fix Agent 生成最小方案；生产修改仍要求 exact fixPlanId 审批。
- `harness verify ...`：Runtime Debugger。
- `harness api-doc ...`：API Doc Agent，只读，最终产物由受控 Runtime 生成。
- `harness chain ...`：保持现有 Chain discover/validate/refresh/edit/seal/persist 的确定性事实与 exact planId 授权边界。
- `harness upgrade`：保持现有离线升级、事务 apply/rollback、manifest 与未知用户文件保护规则。

Test/Fix/API-doc/Chain/Upgrade 的详细行为继续由各自 active Agent/Skill 定义；1.8 Task 3 不改变这些能力。
