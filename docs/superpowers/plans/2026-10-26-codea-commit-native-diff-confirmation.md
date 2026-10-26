# Codea Commit：原生 Diff 与简洁确认

用户最终选择（2026-10-26）：沿用 OpenCode 原生 Diff，撤销附件 HTML/定制 TUI 面板，换另一份简洁模板。此前未提交的面板设计/计划移至工作区外留档，未实施对应宿主源码。

## 实现与验证清单

1. 在现有 Command + Skill 中明确：先只读完整展示三类候选；未展示前不 add/write-tree/commit；范围不明在展示后询问。使用现有 Build/工具与权限。
2. 复用原生显示能力，不增加 UI 源码/协议/组件。原生 working/branch/committed/turn 没有 index 模式，最终证据来自捕获的真实index tree与HEAD/空树的完整Diff，并在确认前及提交前核对当前基线；初始cached输出不能混用变化后的tree；不得声称 Skill 能打开一个 index 面板。
3. 模板1.1仅组织 Diff 之后的本轮标识、文件范围、完整说明、保留改动与三项确认。内存中的完整基线绑定/失效规则不因简化版式减少。
4. 保留部分暂存保护、修改说明重新确认、取消保留暂存、一次提交、verbatim与hooks/签名、真实结果核验。既有参数安全补丁不变。
5. payload/candidates字节同步、摘要更新；先旧规则只读RED，再新规则GREEN审阅。重跑Git/Review回归与隔离v2.0.27资源API探针；不把这些记录当实际TUI/模型演示。
6. 更新安装/回退、包和Draft PR供复审；P01—P15全部保留PENDING，不合并。

## 提示基线

旧规则“读实际 Diff…列出明确路径，只暂存该组”允许模型读到后就add，未明确要求用户先看到；新规则把完整可见作为暂存前置。旧模板冗长，但并未要求实际不存在的增强面板，不能把这一点误记成旧缺陷。

## 交付边界

产品仍是三个文件、两个安装目标：commands/codea-commit.md与完整skills/codea-commit/（SKILL.md及templates/commit-preview.md）。仅此次展示调整无需新宿主改动；原参数前处理安全安装门禁仍须由公司维护流程验证。实际公司TUI、Windows、私有模型与TeamAI均PENDING。

只读GREEN审阅补充：修复最终Diff与tree可混用不同快照的缺口，所有分段/统计改读同一对不可变对象；初始Git查询使用--no-optional-locks，避免status刷新写index。上述为提示规则，仍不是原子执行器。
