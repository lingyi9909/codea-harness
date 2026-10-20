# Codea Review 2.0｜独立模板真实验收记录

PR #64，基线 `98d1c9906421d495d63004c1bb5e805f02151dc3`；本轮拆分仅涉及报告模板，**不能通过 GitHub 静态测试声明公司内网 TeamAI/OpenCode 实测通过**。

## 前置读取（Windows 内网、待验证）

1. 在允许的内网环境确认现有 TeamAI **原生 push/pull** 已同步整个 `codea-review` Skill 目录，包括 `SKILL.md` 和 `references/report-template.md`，记录两端真实路径与 SHA256。
2. 在实际 Java 项目 Git 根目录启动公司 OpenCode 2.x + 私有模型，首次工具调用 `skill(codea-review)`，记录该 Skill 的真实加载目录。
3. 在完整审查结果确认后，由 Reviewer **原生 read** 读取加载目录下 `references/report-template.md`，记录真实路径、工具调用、权限结果和模板五章标题。不得读业务项目同名文件，不使用 Shell 绕过 `external_directory`。
4. 若失败，应原样返回 `REPORT_TEMPLATE_LOAD_FAILED` 与读取错误，不回退旧版格式，不修改现有 Agent 权限。

| 验收项 | 结果 | 证据 |
|---|---|---|
| TeamAI push/pull 同步 references 文件及哈希 | PENDING | |
| skill(codea-review) 首次工具加载 | PENDING | |
| 原生 read 命中可信全局 Skill 的模板 | PENDING | |
| external_directory deny 不造成模板读取失败 | PENDING | |
| 同名业务项目模板不会被误读 | PENDING | |
| 失败时明确提示、不回退旧报告 | PENDING | |

## Review 及报告实测（均 PENDING）

同一 Git fixture/模型/Controller 参数下验证四层 Git 文件清单、Java 八类规则、SQL-only、Finding 原编号等级/证据及零 Finding、HIGH-only、MEDIUM-only、混合风险、同级多项、PARTIAL/需复核、保存失败场景。第 0–4 节与第 6 节及 Agent 权限应与基线保持一致。

正式报告必须：五章完整、TUI 正文等于 `temp/codea-review-report.md` 正文（写入成功时）、有风险则逐项展示真实证据；写入失败输出完整 TUI 正文和真实错误。验收需记录实际工具轨迹、脱敏报告、私有模型版本与内网 OpenCode 精确版本；测试未执行不得写 PASS。

静态自动化命令：`python tests/test_codea_review_2_0_hardening.py`。其 PASS 只覆盖文件、布局、权限配置与 Git fixture 的确定性行为，不证明全局 Skill 的原生 read 或真实模型发现质量。
