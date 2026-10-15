# Codea Review 2.1｜TeamAI 知识增强（P1）

> 状态：**P1 配置交付，等待公司 Windows OpenCode + 私有模型实测**。用户已确认 TeamAI CLI/项目知识库/基础 Review 2.0 均正常，**不重复 P0**。本目录中基础 2.0 的三个运行文件必须保持原样。

## 模块与冻结边界

```text
codea-review/
├─ agents/codea-reviewer.md          # 2.0 冻结：不修改
├─ commands/codea-review.md          # 2.0 冻结：不修改
├─ skills/codea-review/SKILL.md      # 2.0 冻结：所有 Java 风险、Git 四层、五章模板和保存逻辑
├─ agents/codea-reviewer-kb.md       # 2.1 P1 新增，独立 primary Agent
└─ commands/codea-review-kb.md       # 2.1 P1 新增，独立入口
```

- **A 组：`/codea-review`** —— 完全采用冻结的 Reviewer、Command 和 Skill，不主动调用 TeamAI。
- **B 组：`/codea-review-kb [Controller]`** —— 独立 KB Reviewer，**同样先加载唯一的 `skill(codea-review)`**，在识别业务风险假设后最多主动检索两次 `teamai recall`。不复制 Java Skill，不另建知识库或运行框架。
- B 组在 `teamai projects list` 无法确认作用域、检索失败/无命中、来源不明或规则证据不充分时，继续普通代码 Review。只有可信知识来源 + 明确 Java 变更证据 + 排除等效保护才生成知识驱动 Finding。
- 两组都使用原有 `temp/codea-review-report.md` 文件，**后执行会覆盖前一份**；A/B 评测必须手动归档每次报告和脱敏工具轨迹。

## Windows 内网离线安装（只新增两份配置）

在一台能够访问 GitHub 的机器下载 **`feature/codea-review-2-1-kb`** 分支 ZIP，按公司许可的离线传输方式带入 Windows。仅复制 **新增两份**配置；不要覆盖原本已经验证可用的 2.0 三个文件。

在解压目录根部运行 PowerShell：

```powershell
$src = (Resolve-Path ".\codea-review").Path
$cfg = Join-Path $HOME ".config\opencode"
$entries = @(
  @("agents\codea-reviewer-kb.md", "agents\codea-reviewer-kb.md"),
  @("commands\codea-review-kb.md", "commands\codea-review-kb.md")
)
foreach ($item in $entries) {
  $target = Join-Path $cfg $item[1]
  if (Test-Path -LiteralPath $target) {
    throw "已有同名 KB 配置，请先审查/备份，不自动覆盖：$target"
  }
}
foreach ($item in $entries) {
  $from = Join-Path $src $item[0]
  $to = Join-Path $cfg $item[1]
  New-Item -ItemType Directory -Force -Path (Split-Path -Parent $to) | Out-Null
  Copy-Item -LiteralPath $from -Destination $to
  Get-FileHash -LiteralPath $to -Algorithm SHA256
}
```

如果现有 2.0 配置位于 Java 项目的 `.opencode` 而不是用户级目录，先确认同名 Agent/Command/Skill 的**实际加载优先级**，避免 KB 入口引用了错误版本。不要靠修改原 /codea-review 来调试 2.1。重启 TUI 确认配置生效。

## P1 最小真实运行步骤

1. 在已经正确绑定 TeamAI 项目知识的 Java Git 项目根目录，启动 OpenCode，使用与原版相同的公司私有模型。
2. 输入 `/codea-review-kb`，或 `/codea-review-kb OrderController`。
3. 检查工具轨迹：**首个工具调用 `skill(codea-review)`**；遵循四层 Diff/隐藏目录/Dirty 逻辑；在识别到相关业务变更后出现固定 `teamai projects list` 及至多两次 `teamai recall "<关键词>"`。Recall 的动态命令需要用户逐次 `ask`，请核对确切查询文本，不要永久授权。
4. 核查实际 TeamAI 返回的 `File`、`Sources`、`[project]/[user]` 与项目激活情况，检查新报告在**评审摘要**含 KB 状态/项目/次数/采用数；有知识支持的 Finding 同时展示**原始文件来源**与**代码变更证据**。
5. 知识未命中时仍获得完整 Java Review 报告。最终五章继续展示在 TUI，并写入 Java 项目根目录 `temp/codea-review-report.md`。
6. 测试知识不可用时的降级，可在**独立测试会话/测试环境**中制造 CLI 无法执行或选择拒绝 Recall 权限；不改变公司生产知识配置。期望继续 Review，并在报告标 `TEAMAI_FAILED` 或相应状态。

**权限检查重点**：KB Reviewer 仅额外允许 `teamai projects list`（固定 allow）及 `teamai recall "<关键词>"`（每次 ask）；`teamai push`、`teamai projects set`、`teamai recall maintenance`、任意 Shell 组合等必须 DENY；业务文件继续不能编辑。若实际 exact OpenCode V2 对双引号动态命令的资源匹配不成立，保留工具轨迹并单独反馈，**不能靠开放 `teamai *` 修复**。

## 失败、项目隔离与可审计性

- CLI `[project]` 标记不是当前项目 ID，多项目同时激活时若来源无法归属，不采用专属知识；`[user]` 只有明确团队共享且业务适用才采用。
- `Snippet` 和标题是候选资料，不构成独立 Finding。知识原文无法核实或与业务现状冲突时只能待确认/跳过，不能编造文件和规范。
- 外部目录读取仍被原生 `external_directory: deny` 限制；不会为了检索越界文件而修改配置。TeamAI Recall 可能自己写入本地检索元数据，但不修改 TeamAI 项目设置/知识正文。
- A/B 中可能已存在 TeamAI 自动规则/Hook；必须记录是否存在，并将结果表述为“主动 Recall 的增量收益”，而非不实的“无知识 vs 有知识”。

## 验收与后续

**P1**：由用户在内网提供两个真实 `/codea-review-kb` 结果——一个有效项目知识命中，另一个无命中或检索失败正常回退，以及脱敏工具轨迹。只有真正引用了有效项目知识、识别了 Java 证据，才能认定增强能力有效。

**P2**：通过 P1 后，再按 [A/B 验证清单](../../docs/examples/codea-review-2-1-ab-plan.md) 完成 S1～S4 × A/B 各两次的 **16 次真实 Review**，不在本次提交伪造任何模型日志或 A/B 结果。

静态仓库检查：`python tests/test_codea_review_2_1_config.py`（只检查配置与冻结基线；不模拟大模型效果）。
