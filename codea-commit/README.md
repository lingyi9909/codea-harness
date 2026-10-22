# Codea Commit 1.0（原生修复补丁已开发，待公司适配）

审核发现原 Command 的参数可能在模型拒绝之前进入 OpenCode 的 Shell 展开。这是原设计遗漏和实现共同的阻塞项。**不能通过补充提示词、删除参数占位符或修改工具权限修复。已新增 [现有 OpenCode 原生补丁及回归](host-patches/README.md)，不再仅提供停用措施。公司适配/验收前仍不可安装、合并发布，也不要先在业务仓库尝试带参数命令。**

## 当前交付状态

可自动发现的 Command 和 Skill 已同时撤下，保留两份不可加载的评审草稿：

| 评审草稿（不安装） | 未来目标（原生前置校验通过后才恢复） |
|---|---|
| `blocked/codea-commit.md.txt` | `~/.config/opencode/commands/codea-commit.md` |
| `blocked/SKILL.md.txt` | `~/.config/opencode/skills/codea-commit/SKILL.md` |

**不要重命名、复制到 OpenCode 扫描目录，或经 TeamAI 发布这两份草稿。** 某些原生版本会把 Skill 自动注册为同名命令，仅删除 Command、继续同步 Skill 仍会留下入口。`.txt` 隔离是暂时停用措施，不是功能修复。

原离线验收包已经撤回。新版只提供 BLOCKED 评审资料包，不提供可安装包。源码、fixture 与文档不新增 Runtime、MCP、插件、新 Agent 或权限配置；Reviewer、模型与 Review 流程保持原样。

## 已核实的根因

在独立临时目录运行真实上游 OpenCode `1.2.27`（macOS arm64），禁用模型凭据、设置不存在的模型和 `permission: deny`。输入仅写临时标记的 Shell 参数，原入口在返回模型不存在之前已写出标记。移除占位符、代码围栏与 Skill 自动命令的四个负对照也复现。

上游 [Command 文档](https://dev.opencode.ai/v2/docs/commands/) 说明参数先展开、随后执行 Shell；无占位符时仍会追加参数。[固定版本源码](https://github.com/anomalyco/opencode/blob/v1.2.27/packages/opencode/src/session/prompt.ts#L1755-L1803) 显示执行早于模型和 Agent 工具权限；[Skill 命令注册](https://github.com/anomalyco/opencode/blob/v1.2.27/packages/opencode/src/command/index.ts#L126-L139) 解释只撤 Command 的遗漏。

这是公开上游版本的真实安全复现，**不是公司 exact 版本、Windows、TeamAI 或产品正常提交路径验收**。原 55 项本地 Git/fixture 回归不覆盖 Command 前处理，不能证明此边界安全。

## 解除阻塞的最小要求

公司现有 OpenCode 必须在原生 Command 分发入口、模板替换/参数追加/Shell 扫描**之前**，对 `codea-commit` 的原始参数做确定性校验：空或仅空白才继续；任何非空参数立即返回“当前版本仅支持 /codea-commit”，不进入 Shell、模型、Skill、hooks 或 Git。

先核对公司 exact 版本是否已有这个能力；没有则由宿主维护方做最小原生修正。不引入本模块 Runtime、MCP、插件或新 Agent，不修改 Reviewer 权限、默认模型或其他命令的语义。后置模型校验与 `command.execute.before` 不能代替真正的前置校验；已核查的版本中后者也晚于 Shell 执行。

参考上游 `1.2.27` 的原生修正已开发并运行真实源码服务/CLI 回归；公司源码未提供，不能声称已修复公司版本。公司 exact 版本修正及证据通过前，不恢复可发现文件或安装指令。恢复后仍需验证空参数正常加载现有 Build 的 `skill(codea-commit)`，再按原计划做全部提交边界与说明质量验收。详细门槛、证据和复现命令见 [安全阻塞说明](../docs/examples/codea-commit-command-security.md) 与 [验收记录](../docs/examples/codea-commit-acceptance.md)。

## 若已安装旧版

停止调用 `/codea-commit`；关闭/暂停 OpenCode，核对实际生效来源。只将本模块的 Command 和整个 `skills/codea-commit/` 移到扫描目录之外的备份位置；检查用户级、项目级、兼容 Skill 目录和同名 JSON 配置，避免另一来源重新暴露入口。通过公司原有管理流程暂停本模块 TeamAI 同步。不删除整个 commands、skills 或配置目录，不修改 Review/模型/权限。

重新启动/按 exact 版刷新，核查命令列表中没有任何来源的 `codea-commit`。不要直接在旧入口上试注入参数来确认停用；先用命令发现/配置诊断确认来源，再在隔离测试用户和临时项目进行安全探针。回退到旧版资源不能修复本问题。

## 保留的功能设计（尚未解除阻塞）

原设计仍是单一无参数入口、现有 Build、一次本地 commit：优先提交 index，保留同文件未暂存 hunk；无暂存且任务归属明确才仅 add 明确路径，否则询问；中文说明仅来自完整暂存 Diff；保留 hooks/签名，提交后读取真实 SHA、父提交、完整消息、内容和剩余状态；不 push、amend 或改历史。

这些草稿规则不承担宿主 Shell 前处理的隔离职责。正常功能目前不可用，C01—C16、关键路径三次重复、十组真实脱敏说明评审以及公司安装/回退/分发仍 PENDING，须先解除参数安全阻塞。
