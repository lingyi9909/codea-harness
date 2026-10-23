# Codea Commit 提交预览与确认实施记录

> 实施方式：在现有分支直接落实用户提供的完整方案；沿用 Command + Skill，不新增执行组件。本文记录变更与验证，不另行启动设计审批。

目标：真实暂存 Diff 与完整说明先展示，本轮明确确认后才创建一次本地提交。

依据：用户提供《Codea Commit：提交预览与确认研发方案》HTML（2026-10-23），SHA256 `4d7ec3eba6b866ceac876e61edee7f097342c5ffcd0d855602e0eefd77eb2ce7`。该方案替代旧版“摘要后直接提交”的交互契约，完整执行规范见 [当前 Skill](../../../codea-commit/compatibility/v2.0.27/payload/skills/codea-commit/SKILL.md)。

约束：OpenCode TUI v2.0.27，原生 Build/question，沿用模型和权限；只交付现有 Command/Skill 的更新。原参数前置拦截和补丁不变，确认交互不能替代参数修复；不实现增强面板。

- [x] 读取方案和公开 v2.0.27 question Schema/取消行为；确认当前 PR 基线 c3210a76603f89ae1231e20b69545ec2301e7946。
- [x] 旧 Skill 压力模拟：明确范围/无 question/保存说明会直提（RED）；旧预复核可拦状态改变但无重新确认闭环。
- [x] 更新 payload 两份资源与 candidates 两份输入，保持逐字节一致；预览绑定 worktree/branch/HEAD/index tree/全文说明。
- [x] 扩展测试专用只读 checkpoint/verify：等待和取消后 HEAD/index/worktree 保持；确认后核对完整消息、父提交、index 树。先观察缺失 checkpoint 的失败，再实现验证器。
- [x] 同场景 GREEN 压力模拟及独立代码审阅；其结果仅提示词模拟，不是内网验收。
- [x] 运行完整 Codea Git/Review 回归及真实修复后 v2.0.27 API 加载/参数探针；不重跑未改变的宿主完整套件，不冒充交互验收。
- [x] 同步安装、验收文档与带摘要的测试包生成流程；在现有 Draft PR 交付，不合并发布。

重点风险：工具缺失/取消不得直提；新消息不得复用旧确认；分支/HEAD/index 改变必须重新预览；大 Diff 必须逐块展示；新准备的暂存取消后保留。真实私有模型/TUI、Windows 深浅色主题、原生权限和 TeamAI 轨迹均 PENDING，附验收步骤交用户内网执行。

本地复测：fixture 17/17、Review 40/40、Review Git 4/4；更新资源实际源码服务 API 11/11；独立只读压力模拟 13 场景符合规范。旧原生完整套件结果保留历史边界（Core 8 FAIL 已基线复现，root check 未通过），本次未修改宿主补丁，也未重复运行该大套件。

最终审阅：测试 checkpoint 的子目录路径问题已用真实 worktree root 修复；新负/正对照先 RED 后 GREEN，fixture 最终 17/17，独立复核关闭 Important。所有模型/TUI 确认验收继续 PENDING。
