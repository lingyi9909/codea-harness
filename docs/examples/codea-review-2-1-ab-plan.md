# Codea Review 2.1｜A/B 测试安排（P2，尚未执行）

> **计划，不是实测结果。** P0 CLI 与项目知识库已由用户在内网确认；P1 KB Reviewer 真实工具轨迹与知识归属须先验收。后续由真实 Windows / OpenCode / 私有模型执行。

## 实验的唯一主要变量

- A：同一冻结 Codea Review 2.0，输入 `/codea-review`。
- B：新增 KB Reviewer，输入 `/codea-review-kb`；唯一预期增量是主动调用 TeamAI 项目知识。
- 两组使用完全相同的模型 ID/配置、温度（如可控）、项目/HEAD、staged/unstaged/untracked 文件、重要系统提示、Java Skill、报告模板及工具授权流程。每次新开会话，交换 A/B 运行顺序，以减少时序影响。
- 检查 TeamAI 是否已由全局/项目规则或 SessionStart Hook 自动注入知识：若存在，只可声称“B 主动 Recall 相对现有环境的效果”，不可声称 A 完全不含知识。
- 测试期间不要修改业务 Git 变更；业务仓库中已有的 `temp/` 报告不参与 Git 业务审查。

## 四类场景 × 两组各两次 = 16 次

| 场景 | 人工先定真实结论（不向模型提供） | A × 2 | B × 2 | 重点 |
| --- | --- | --- | --- | --- |
| S1 常规 Java 事务/幂等/SQL | 明确已知的一条通用风险 | 待执行 | 待执行 | B 不退步 |
| S2 项目专属业务规则 | 例如订单结算中不允许直接取消，代码移除校验且无替代保护 | 待执行 | 待执行 | B 发现有项目知识依据的额外真实风险 |
| S3 合法实现 | 满足现行规范并有等效保护 | 待执行 | 待执行 | B 不引入误报 |
| S4 项目冲突规则 | 两个已绑定 TeamAI 项目的不同业务约束 | 待执行 | 待执行 | B 不跨项目引用专属规则 |

每轮提前由人工保存**脱离给模型的 Ground Truth**（期望问题、业务规则、排除反证、允许知识作用域、版本时间）；样本代码注释和用户 Prompt 不能提示正确答案。P2 结果必须是私有模型实际输出，而非编写的示意 Finding。

## 每次运行必填记录

| 字段 | 样例/填写约束 |
| --- | --- |
| 场景/组/序号 | S2-B-1 |
| 日期与顺序 | 真实时间与先 A 后 B/先 B 后 A |
| Git 快照 | 基线 ref、HEAD SHA、staged/unstaged/untracked 的内容哈希；两组要相同 |
| 模型与 OpenCode 版本 | exact ID/版本，温度与工具策略 |
| TeamAI 作用域 | 活动项目 ID 列表、项目知识路径、是否已预注入 |
| 查询轨迹 | 原生 Shell 命令、是否经 ask、Recall 次数、来源/命中/采用结果 |
| 结果 | TP/FP/FN（逐个缺陷）、严重误报、代码证据是否成立 |
| 性能 | 纯模型+工具耗时、授权等待、总耗时（秒） |
| 故障 | 知识召回失败、来源模糊、回退是否成功 |
| 报告归档 | 单独的 Markdown 文件路径 + 脱敏 OpenCode TUI 轨迹 |

## 归档防覆盖（测试人员人工操作，非 Reviewer 新功能）

两组 Reviewer 都保留冻结的固定报告路径：`temp/codea-review-report.md`。**每执行一次都应立即复制**到本地受保护的归档位置再运行下一组，例如在 Windows PowerShell：

```powershell
New-Item -ItemType Directory -Force -Path ".\temp\ab" | Out-Null
Copy-Item -LiteralPath ".\temp\codea-review-report.md" -Destination ".\temp\ab\S2-A-1.md"
# 另一次执行 /codea-review-kb 之后：
Copy-Item -LiteralPath ".\temp\codea-review-report.md" -Destination ".\temp\ab\S2-B-1.md"
```

根据实际场景改文件名；保存每轮经过脱敏的工具调用轨迹。归档在项目根目录 `temp/` 下，它被已有 Git 文件筛选排除。**不要把内网知识正文/未脱敏项目源码提交到公开 GitHub。**

## 第一轮判定

- **收益**：B 在至少一个项目专属规则案例中相对 A 增加真实、有效、有源可查的 Finding。
- **质量**：不因 TeamAI 引入严重误报、跨项目知识错用；S1 通用风险召回不得明显下降。
- **可靠性**：TeamAI 失败可回退基础审查，报告完整，权限没有被绕过。
- **性能**：以额外耗时约 ≤30% 为优化目标（非第一轮硬门槛），测量授权次数、各阶段时长。
- **实验强度**：16 次是探索性小样本；偶然一次多发现不能证明稳定改善，结果波动明显时扩充案例。
- **发布**：仅在 P2 实测成立后另行决定是否默认启用；本次 P1 保留两个独立入口、原版行为不变。
