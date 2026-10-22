# Codea Commit 1.0（目标 TUI v2.0.27，参数安全核验未通过）

目标为公司现有 **OpenCode TUI v2.0.27**。公开同版本真实宿主核验 **6 PASS / 5 FAIL**：Command/Skill 可以加载，但非空参数仍能在模型之前执行 Shell。当前不恢复可发现的提交入口，也不合并发布。公司构建、Windows、实际 TUI、私有模型与 TeamAI 验收仍 PENDING。

请先读 [2.0.27 专项核对、独立探针及安装回退说明](compatibility/v2.0.27/README.md)，固定版本、发行包摘要与真实响应分别见该目录的 `SOURCE.json`、`SOURCE-FILES.json`、`RESULTS.json`。

## 本轮交付

- `compatibility/v2.0.27/candidates/codea-commit.md.txt`：V2 候选 Command，继续路由现有 Build，明确以 `{"id":"codea-commit"}` 加载原生 Skill。
- `compatibility/v2.0.27/candidates/SKILL.md.txt`：既有提交规则草稿，正文保持不变。
- `tests/fixtures/codea-commit/probe_v2.py`：显式指定现有二进制，在新临时配置启动独立真实 V2 服务；不替换宿主、不操作业务 Git、不使用模型凭据。
- 独立 Git fixture、真实测试结果、备份/卸载说明和现有宿主最小改动建议。

候选保持 `.txt`，没有安装到扫描目录。**公司目标构建通过 S01—S08 并复审后，才按专项说明恢复两个目标文件**，随后继续 C01—C16、关键路径三次重复和中文说明评审。本轮并未完成 V2 宿主修复，也没有提供已验收的可安装包。

## 所需宿主边界

`codea-commit` 的非空原始参数必须在原生命令公共入口调用命令定义前拒绝，空/仅空白才沿原路径继续；实际 TUI/CLI/API 要展示中文用法错误并结束等待。提示词拒绝、删除参数占位符、代码围栏和 Agent 工具 deny 均不足以阻止这个版本的前处理执行。

[专项说明](compatibility/v2.0.27/README.md) 给出固定源码位置和最小改动范围。若公司版已有等价前置能力，先验证现有能力；否则由现有宿主维护流程适配，不要求直接替换公司版本。本模块不新增 Runtime、MCP、插件、Agent，也不改 Reviewer、Build 权限、模型、Provider 或 Review 流程。

## 保留的功能设计

单一无参数入口、现有 Build、一次本地 commit：优先提交 index，保留同文件未暂存 hunk；无暂存且任务归属明确才仅 add 明确路径，否则询问；中文说明仅来自完整暂存 Diff；保留 hooks/签名，提交后读取真实 SHA、父提交、完整消息、内容和剩余状态；不 push、amend 或改历史。

这些提示规则不承担宿主 Shell 前处理的隔离职责。现有 Git/Review 回归 55/55 通过不等于真实模型正常提交或参数安全已验收。完整规则与矩阵见 [验收记录](../docs/examples/codea-commit-acceptance.md)。

## 旧资源和历史补丁

`blocked/` 保留原评审草稿；`host-patches/` 保留早期上游 **1.2.27** 研究补丁和测试，不适用于 **2.x**，不作为本目标修复。旧可安装离线包已撤回。

若已安装旧资源，先暂停本模块调用与同步，将本模块单个 Command 和 Skill 目录移至扫描目录外的备份，检查实际同名来源并按专项说明回退。不删除整个 commands/skills 或 OpenCode 配置，不改其他能力，也不要在业务仓库试注入参数。
