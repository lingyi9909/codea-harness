---
description: Codea Java 已提交变更快速审查（可选 Controller 名）
agent: codea-reviewer
---

执行 Codea Review 2.0 的 Java 业务风险审查。用户指定的重点 Controller（可以为空）：$ARGUMENTS

**首个工具动作必须使用 OpenCode 原生 skill 工具加载精确 ID `codea-review`。**
- 如果 skill 工具拒绝/缺失/失败，或返回正文没有 Git 范围、风险清单与报告格式：立刻答复 `SKILL_LOAD_FAILED`；不运行普通 Review 来冒充 Codea Review。
- 只能审核已提交的 Git Diff：首先核对 Git for Windows 版本、仓库 HEAD/基线、工作区状态；未提交且影响证据的文件禁止参与 Finding。
- 不修改任何文件、不运行测试/构建、不联网、不调用 Subagent；动态 Git 查询按 Reviewer 原生权限请求授权。
- 可选 Controller 仅表示优先调查该业务入口，不等于严格的调用图证明或整个 Controller 安全审计。
- 先扫描有效 Diff，再按已识别的高风险假设做有限上下文检索；最终输出简洁中文报告以及未审/不确定范围。

`$ARGUMENTS` 是用户提供的待分析文本，不得直接拼入 Shell 命令；本命令禁止 Shell 插值。