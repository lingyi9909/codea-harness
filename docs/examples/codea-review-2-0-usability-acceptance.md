# Codea Review 2.0 Usability｜独立验收清单（未执行的项目保持 PENDING）

基线：`a03d8ccdbee4f37ca2198b0154ab3515ef149817`，开发分支：`feature/codea-review-2-0-usability`。

## A. 自动化确定性回归

```bash
python tests/test_codea_review_2_0_hardening.py
```

真实 Git fixture 继续验证 committed/staged/unstaged/untracked、SQL-only、跨层去重、重命名、隐藏目录和 temp 排除、index/worktree 取证。权限测试只验证**V2 规则匹配的离线近似实现**：固定无动态参数的常用查询 `allow`、危险 Git 写动词的常见形式 `deny`、动态 `diff/show/ls-files/blame/grep/log` 与未知 Shell 保持 `ask`；**不能把模拟 PASS 等同于真机 Shell 权限安全性**。报告测试检验五章骨架、卡片、分组、Finding 编号与样稿三条明细的字段完整性，不能代替模型真实审查结果。

## B. Windows 内网 OpenCode 2.x 权限实测（PENDING）

确认当前 Agent 来源、全局/项目/组织权限与 V2 版本；在实际业务工作区验证：

| 项目 | 通过条件 | 状态 |
|---|---|---|
| 固定 Git status/rev-parse 与固定 merge-base | 实际不反复弹授权，查询内容与旧版一致 | PENDING |
| 四层 Git diff/ls-files 与单文件 show | 动态参数正确执行，`ask` 经过确认后正常查询 | PENDING |
| Git log/show/blame/grep 补充查询 | 固定安全形式按新规则运行，动态其他形式 `ask` | PENDING |
| Git 写操作与危险参数 | 直接 `git add/commit/reset/push...` 不被自动批准；组合/变体不得擅自执行 | PENDING |
| 业务源码只读 | 前后 Git 状态/业务文件无 Reviewer 引入的改动（在隔离仓库做负测） | PENDING |
| 正式报告保存 | `temp/codea-review-report.md` 的完整五章与 TUI 完全一致；失败如实提示 | PENDING |

注意：`git diff --no-ext-diff --no-textconv ...` 包含动态参数和 `**` pathspec，不能仅依靠 OpenCode 简单通配规则证明安全；本轮安全优先保留 `ask`。如果“减少授权”效果仍不足，记录真实命令与弹窗次数，**先进行独立风险评估**，而不是自动扩大权限。

## C. 报告展示真实模型验收（全部 PENDING）

在**同一 Git fixture、同一私有模型配置**下分别运行：HIGH-only、MEDIUM-only、HIGH+MEDIUM、同级多个 Finding、零 Finding、SQL-only、PARTIAL/需复核/报告写入失败七类场景。

对每份报告检查：五章顺序、摘要原字段与完整风险概览、卡片数据、组内原编号排序、每个已确认 Finding 单独的原始代码证据和建议、待确认不包装为 Finding、SQL-only 不虚构 Controller、TUI/file 一致。演示排版样稿不充当真实运行证明。

## D. 审查能力非退化（PENDING）

使用已验收 2.0 基线与本分支的**相同 Git 快照、私有模型、Controller 参数、工具轨迹**比对。Java 八类、SQL 风险、SQL-only、四层文件数与来源、Finding 根因/严重程度与完整证据都必须一致（允许自然语言排版差异）。之前三个历史通用 Finding 的原始样本如果可访问，在公司内网重复审查；不上传公网仓库，不用演示标题或人工构造的代码冒充历史结果。

## E. 审核边界

代码审核、自动化测试、OpenCode 真机验证、私有模型 Finding 回归、Markdown 保存核验分别给独立状态。未完成的测试不得记 PASS；禁止在本轮调整审查算法、知识检索、Runtime、报告引擎或 Command 安装方式。PR 保持 Draft 直至独立验收确认。
