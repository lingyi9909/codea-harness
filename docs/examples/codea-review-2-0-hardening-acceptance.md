# Codea Review 2.0｜基础完善独立验收记录（待内网验收）

基线：`afb427ca74a8ffa2b1f799f55649dc8e6f0ba8bb`。开发分支：`feature/codea-review-2-0-hardening`。

> 本文是**验收计划和取证模板，不是实测结果**。尤其是三个历史通用 Finding 尚未取得原始代码快照、差异和对应报告；不得填写“PASS”或将合成 SQL 用例冒充真实历史问题。不要公开真实业务代码/日志。

## A｜确定性自动化（可在公开 GitHub Actions/本地运行）

```bash
python tests/test_codea_review_2_0_hardening.py
```

检查：四层 SQL-only 与 Java+SQL 清单，`.sql/.SQL`，已提交/staged/unstaged/untracked，跨层同名去重，隐藏目录与根 `temp/` 在**读取前排除**；Git diff/show 层次与工作区实际 SQL 正文；OpenCode V2 Agent Git allow/deny 的**离线规则模拟**；原报告路径和五章标题。请记录执行命令、Python/Git 版本、测试完整 stdout/stderr、测试环境与 HEAD SHA。

**注意**：自动化权限模拟只是对配置文本的近似判定，不等于 OpenCode V2 TUI 权限实际行为；自动 Git fixture 只证明文件范围，不证明 LLM 找到了缺陷。

## B｜公司 Windows OpenCode 2.x 真机（P0）

在公司允许的内网 Java 项目**Git 根目录**启动已批准的 OpenCode V2 和私有大模型。确认三个已安装文件来自同一新提交，记录 TUI/Agent 版本、实际权限弹窗与脱敏工具轨迹。下面各项只有实际复测后才能填写结果：

| 用例 | 条件 | 结果（待填写） | 实际证据 |
| --- | --- | --- | --- |
| W1 | HEAD 与本地基线 committed SQL Diff 成功 | PENDING | |
| W2 | staged SQL/Java Diff 成功 | PENDING | |
| W3 | unstaged SQL/Java Diff 成功，Dirty 不阻断 | PENDING | |
| W4 | untracked .sql/.SQL 工作区正文被读到 | PENDING | |
| W5 | SQL-only 输出完整五章，无虚构 Controller 调用链 | PENDING | |
| W6 | Java+SQL 混合变更均计入有效范围 | PENDING | |
| W7 | 同脚本跨多个 Git 层只计 1 个有效文件、分别标来源 | PENDING | |
| W8 | 隐藏目录与根 temp 在清单阶段排除 | PENDING | |
| W9 | 安全 git diff/show/merge-base/ls-files/status 不发生意外权限拒绝 | PENDING | |
| W10 | git add/commit/checkout/reset、Shell 组合与业务 edit 仍被拒绝 | PENDING | |
| W11 | TUI 与 temp/codea-review-report.md 五章完整一致 | PENDING | |
| W12 | 有高风险 SQL 证据时能发现、合法 SQL 不因关键字误报 | PENDING | |

负向写操作仅在**隔离测试仓库**内验证，不能拿真实业务代码尝试修改。组织级拒绝优先，不能用 Agent 权限绕过。

## C｜三个真实历史 Finding（P0，至少独立 3 次/样本）

须先由内网测试人员准备**不上传公开仓库**的三个原始样本：

| 样本 ID | 原始问题根因与触发条件 | 真实代码/工作区快照 | 原版正确 Finding 证据 | 当前状态 |
| --- | --- | --- | --- | --- |
| H1 | 待内网提供 | 待提供 | 待提供 | **PENDING** |
| H2 | 待内网提供 | 待提供 | 待提供 | **PENDING** |
| H3 | 待内网提供 | 待提供 | 待提供 | **PENDING** |

每个样本在**相同 Git 快照**和私有模型配置下开三个独立会话运行 `/codea-review`。记录每轮的模型 ID、OpenCode 版本、基线、HEAD 和 staged/unstaged/untracked 快照校验、Finding 是否正确命中**相同风险根因**、文件/行证据、触发条件、业务影响、误报、耗时、报告归档位置（共 9 次独立运行）。不要用 Finding 数量或相同措辞判断命中。样本产生后先由人工标注 Ground Truth，**不能在待测代码注释或 Prompt 中泄露答案**。

| 运行 | 风险根因命中 | 代码证据准确 | 严重误报 | 五章完整 | 结果 |
| --- | --- | --- | --- | --- | --- |
| H1-1 / H1-2 / H1-3 | 待填 | 待填 | 待填 | 待填 | PENDING |
| H2-1 / H2-2 / H2-3 | 待填 | 待填 | 待填 | 待填 | PENDING |
| H3-1 / H3-2 / H3-3 | 待填 | 待填 | 待填 | 待填 | PENDING |

结果不可只写 PASS，须给出脱敏 Finding、代码证据和逐轮工具轨迹。若缺原始材料则保持 PENDING；不要用合成 SQL fixture 代替三个历史 Finding。

## D｜SQL 高风险与误报（真实模型）

分别准备有证据的 UPDATE/DELETE 约束缺失、重复迁移和兼容 DDL，以及合法的带 WHERE/租户范围的变更。对照人工 Ground Truth；观察是否引用实际 diff、执行前提、表约束、等效保护、触发影响。合法 SQL 不应因为含 UPDATE、DELETE 或 ALTER 就产生确认的中高风险 Finding。无法验证执行环境时，应列待确认而非臆造。

## E｜交付与发布边界

分开报告：**自动化结果**、**Windows/OpenCode V2 权限与 SQL-only 实测**、**历史 Finding 9 次实测**、**脱敏 Review 报告**、**未完成项**。只有真机验证通过才能提出正式发布。无需新 Runtime、知识检索、MCP 或多 Agent。

Codea Review 2.1 PR #62 **冻结为 Draft，暂不合并**，本任务不得修改 2.1 分支、KB Agent、KB Command 或其检索策略。
