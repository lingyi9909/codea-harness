# Codea Commit 1.0（目标 TUI v2.0.27，原生修复已开发）

目标为公司现有 **OpenCode TUI v2.0.27**。已完成同版本原生修复：新增安全回归 **17/17 PASS**，真实源码CLI服务探针 **11/11 PASS**，未修改2.0.27客户端可显示中文拒绝错误并非零退出。未修公开发行包的 **6 PASS / 5 FAIL** 保留为对照。测试资源已恢复在隔离的 `payload/`，仅供前置拦截通过的现有宿主安装；不自动启用或合并发布。公司构建、Windows、实际 TUI、私有模型与 TeamAI 验收仍 PENDING。

请先读 [2.0.27 专项核对、独立探针及安装回退说明](compatibility/v2.0.27/README.md)，固定版本、补丁、摘要与修前/修后结果见该目录的 `SOURCE.json`、`VALIDATION.json`、`RESULTS.json`、`PATCHED-RESULTS.json`、`PREVIEW-RESULTS.json`。

## 2026-10-23：提交前预览与确认

输入 `/codea-commit` 后，按“概览 → 文件清单 → 分文件真实暂存 Diff → 完整说明”展示，再等待“确认提交 / 修改说明 / 取消”。原生 question 不可用时聊天提问并结束执行；修改说明要重新展示全文和确认。分支、HEAD 或 index 改变使旧确认失效。取消保留原有和本次准备的暂存，不撤销现场。只显示聊天中的 Markdown Diff，不提供示意图中的增强面板。

这些交互规则已更新于两份资源；真实公司模型/TUI 的确认轨迹仍 PENDING，不能用 API 加载或提示词模拟代替。

## 本轮交付

- `compatibility/v2.0.27/candidates/codea-commit.md.txt`：V2 候选 Command，继续路由现有 Build，明确以 `{"id":"codea-commit"}` 加载原生 Skill。
- `compatibility/v2.0.27/candidates/SKILL.md.txt`：提交预览、显式确认、状态失效与提交后核验规则，与 payload 字节一致。
- `tests/fixtures/codea-commit/probe_v2.py`：显式指定现有二进制，在新临时配置启动独立真实 V2 服务；不替换宿主、不操作业务 Git、不使用模型凭据。
- `payload/`：可条件安装的两份测试资源，内容与真实探针加载的候选一致。
- `opencode-v2.0.27-codea-commit.patch`：已实施并验证的现有宿主最小补丁，仅三个生产文件及两个原生测试文件。
- 独立 Git fixture、真实测试结果、备份/卸载说明。

候选 `.txt` 保留用于探针，`payload/` 提供正式扩展名测试资源，均不放在实际扫描根下。**公司目标构建通过 S01—S08 并复审后，才按专项说明安装两个目标文件**，随后继续 C01—C16、关键路径三次重复和中文说明评审。V2 参考源码修复已完成，公司环境仍未验收。

## 所需宿主边界

`codea-commit` 的非空原始参数必须在原生命令公共入口调用命令定义前拒绝，空/仅空白才沿原路径继续；实际 TUI/CLI/API 要展示中文用法错误并结束等待。提示词拒绝、删除参数占位符、代码围栏和 Agent 工具 deny 均不足以阻止这个版本的前处理执行。

[专项说明](compatibility/v2.0.27/README.md) 给出固定源码位置、实际补丁和验证结果。若公司版已有等价前置能力，先验证现有能力；否则由现有宿主维护流程适配，不要求直接替换公司版本。本模块不新增 Runtime、MCP、插件、Agent，也不改 Reviewer、Build 权限、模型、Provider 或 Review 流程。

## 保留的功能设计

单一无参数入口、现有 Build、一次本地 commit：先预览 index 并等待明确确认，保留同文件未暂存 hunk；无暂存且任务归属明确才仅 add 明确路径，否则询问；中文说明仅来自完整暂存 Diff；保留 hooks/签名，提交后读取真实 SHA、父提交、完整消息、内容和剩余状态；不 push、amend 或改历史。

这些提示规则不承担宿主 Shell 前处理的隔离职责。当前 Git/Review 回归 61/61 通过不等于真实模型正常提交或参数安全已验收。完整规则与矩阵见 [验收记录](../docs/examples/codea-commit-acceptance.md)。

## 旧资源和历史补丁

`blocked/` 保留原评审草稿；`host-patches/` 保留早期上游 **1.2.27** 研究补丁和测试，不适用于 **2.x**，不作为本目标修复。旧可安装离线包已撤回。

若已安装旧资源，先暂停本模块调用与同步，将本模块单个 Command 和 Skill 目录移至扫描目录外的备份，检查实际同名来源并按专项说明回退。不删除整个 commands/skills 或 OpenCode 配置，不改其他能力，也不要在业务仓库试注入参数。
