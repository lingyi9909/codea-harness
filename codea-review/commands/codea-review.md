---
description: Codea Java 当前工作区变更快速审查（可选 Controller 名）
agent: codea-reviewer
---

执行 Codea Review 2.0 的 Java、Mapper XML 和独立 SQL 业务风险审查。用户指定的重点 Controller（可以为空）：$ARGUMENTS

**首个工具动作必须使用 OpenCode 原生 skill 工具加载精确 ID `codea-review`。**
- 如果 skill 工具拒绝/缺失/失败，或返回正文没有 Git 范围、风险清单与报告格式：立刻答复 `SKILL_LOAD_FAILED`；不运行普通 Review 来冒充 Codea Review。
- 默认同时审查基线→HEAD 已提交、HEAD→index staged、index→工作区 unstaged、以及相关 untracked 业务文件（包括独立 `.sql` / `.SQL`，SQL-only 也必须审查）。**工作区 Dirty 也必须继续**，以工作区实际代码为准；逐条标注 HEAD/INDEX/WORKTREE/UNTRACKED 来源，不得把未提交内容冒充 HEAD。
- 业务代码保持只读；仅在最终业务 Git 状态核对**之后**，允许通过原生 write 创建/覆盖项目根目录 `temp/codea-review-report.md`，不修改其他文件、不运行测试/构建、不联网、不调用 Subagent；Git 只读查询遵循 Reviewer 原生 V2 限定权限，不能因为某条命令被拒而执行宽泛 Git 或其它 Shell 命令。
- 可选 Controller 仅表示优先调查该业务入口，不等于严格的调用图证明或整个 Controller 安全审计。
- 每层 Git 变更清单先通过 `-- . ':(exclude,glob)**/.*/**' ':(exclude,glob)temp/**'` 排除任意深度隐藏目录和根 `temp/`，过滤**在读取正文前完成**；仅对筛选后的文件审查、去重计数。按需分析调用链；SQL-only 在现有“调用链与覆盖范围”中写独立脚本位置，不构造 Controller 链路。在 TUI 输出 **Harness 1.8 五章完整中文 Markdown 报告**（评审摘要、调用链与覆盖范围、风险清单、问题明细、后续处理与评审边界），同时使用原生 write 将完整相同报告保存为 `temp/codea-review-report.md`，有 temp 直接写入、没有则由 write 创建。写入失败仍在 TUI 展示报告并告知原因，不得声称保存成功。

`$ARGUMENTS` 是用户提供的待分析文本，不得直接拼入 Shell 命令；本命令禁止 Shell 插值。