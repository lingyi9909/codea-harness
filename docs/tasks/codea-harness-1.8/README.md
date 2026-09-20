# Codea Harness 1.8｜研发任务总入口

日期：2026-09-15。状态：任务已拆分，尚未开始产品开发；由用户分派研发。本次提交仅包含任务书及文档索引。

目标：**找链 → 人工选链 → 主 Agent 评审 → 正式报告落盘**。1.7 保持暂停，不自动带入其分支代码。

## 先读哪些文档

1. [正式设计](../../superpowers/specs/2026-09-15-codea-harness-1.8-review-design.md)：产品范围、报告完成语义、历史故障与发布门槛。
2. [完整实施计划](../../superpowers/plans/2026-09-15-codea-harness-1.8-review-plan.md)：共享 DTO/接口、代码位置与测试步骤。
3. [汇报版报告样稿](../../examples/2026-09-15-codea-harness-1.8-review-report-example.md)：老板看摘要、研发看明细的固定版式。
4. 下表对应的独立任务书。

本目录为分派入口，不复制或改变共享接口。任务书、设计、总计划出现冲突时先记录差异由负责人确认，不能按个人偏好另造协议。人工研发可直接执行，不要求安装任何 Agent 技能。

## Task 拆分与分派顺序

| Task | 核心交付 | 建议负责能力 | 前置验收 | 当前状态 |
|---|---|---|---|---|
| [T1 报告 Runtime](task-1-report-runtime.md) | 入口创建报告、可靠保存、错误保留、幂等/并发 | Go Runtime | 无 | 可分派，未开始 |
| [T2 调用链与人工选择](task-2-chain-selection.md) | 有界发现、展示选项、真实下一用户回合选择 | Go 导航 / Runtime | T1 | 等待 T1 |
| [T3 主 Agent 到正式报告](task-3-primary-agent-report.md) | 正式入口、结构化工具、一次 finish、真模型最小闭环 | OpenCode / TypeScript + Go | T2 | 等待 T2 |
| [T4 报告版式与历史回归](task-4-report-and-regression.md) | 五段汇报模板、状态与数量正确、H01–H17 回归 | 模板 / Go + 测试 | T3 | 等待 T3 |
| [T5 Windows 与最终验收](task-5-windows-release-acceptance.md) | 真实升级、正式候选包、真模型矩阵、内网验收 | Windows 发布 / 集成测试 | T4 | 等待 T4 |

实现顺序固定 T1 → T2 → T3 → T4 → T5。可以提前分派负责人、阅读文档、准备测试数据和已批准模型环境；依赖任务未验收前，不提前实现或合入共享接口相关代码。尤其 T3 自主执行未通过，不用继续做模板/发布来掩盖问题。

不增加独立的“基础框架任务”“认证平台任务”或新的 1.7 范围。T1 定义的接口由后续任务复用，有变化先更新同一设计/计划并说明影响。

## 开发基线与分支

- 仓库：`lingyi9909/codea-harness`。
- 产品源码核对点：`970573a6d2147733b773c0de0c124d9324914ae8`（1.6.7）。Windows CI 曾通过，但真实模型使用仍有报告问题，不称已验收的产品体验。
- 1.8 设计提交：`2fbcbe77461406e297e9969f3206460049f75efb`；在此之后的本任务包提交只整理文档。
- T1 从**本任务包所在提交**创建 `feature/1.8-task1-report-runtime`；记录完整 base SHA。不要以不固定的“最新 main”替代。
- 后续建议分支：`feature/1.8-task2-chain-selection`、`feature/1.8-task3-primary-report`、`feature/1.8-task4-report-regression`、`feature/1.8-task5-release`；分别从前一任务 accepted exact HEAD 创建。
- 如需集成分支，使用 `feature/1.8-review`，只推进已验收提交。未经负责人安排，不自动合并 main 或发布版本。
- 完成一个 Task 就提交验收并停止。未通过只修本 Task；后续 Task 不能拿未验收 HEAD 开始开发。
- 1.7 分支保留，禁止默认合并/挑选其实现；需要某个已有能力时必须证明本任务需要并单独说明差异。

## 所有研发共同遵守

- 两条及以上调用链一定等真实人工选择；同 Controller 多方法也适用；用户仅说“继续”不等于选全部。
- 主 Agent 完成语义评审；普通 Review 不依赖独立 Reviewer 身份、旧多阶段认证和多份 authority 文件。
- start 先有可读的“未完成”报告；finish 必须真正写盘并回读，工具成功、聊天总结、占位文件都不能单独等于评审完成。
- 空 findings 不等于通过；未解析、超时、取消或部分覆盖要明确表达。
- 只读评审不自动修改业务代码；现有升级/项目状态/Test/Fix/Apply/DB 保护保持。
- Windows、离线、原有安装升级机制；不新增在线 npm/bun 安装、后台服务、WSL、索引或调用图平台。
- 不无界扫描，不无限重试，不因为一个字段错误重开整个 run。
- 不恢复废弃 Task153 workflow，不按关键词删除历史测试。

## 历史故障责任表

H 编号与正式设计保持一致。“主修任务”负责最早实现/证明，“最终验证”不免除前置任务自测。

| 故障 | 主修任务 | 最终验证重点 |
|---|---|---|
| H01 BOM | T1、T3 | T4 输入/输出回归，T5 Windows 包 |
| H02 Reviewer identity/authority 依赖 | T3 | T5 无旧 authority 文件完成新路径 |
| H03 opaque session ID | T2、T3 | T5 原生 Host 与安全参数 |
| H04 阶段错位 | T1、T3 | T4 新旧 run 隔离与错误动作 |
| H05 缺 runId | T1、T3 | T5 正式入口与并发会话 |
| H06 仅聊天总结不落盘 | T3 | T5 真模型自主 finish、回读文件 |
| H07 chainRefs 不匹配 | T2、T3 | T4 nodes[] 与旧格式拒绝 |
| H08 导航超时/耗时长 | T2 | T5 真工具计时、过程计数 |
| H09 缺 ast-grep.exe | T2、T5 | T5 正式包 SHA 与 --version |
| H10 Windows 路径/schema/文件占用 | T1、T3 | T5 原生 Windows 路径/故障矩阵 |
| H11 多链自动 ALL/选错子集 | T2、T3 | T5 真实下一用户消息、精确子集 |
| H12 升级版本/Host 文件遗漏 | T5 | 实际 1.6.7→1.8.0 与回滚 |
| H13 stale Task153 workflow contract | T4 | T5 全量回归，不恢复旧 workflow |
| H14 测试脚本替模型完成 | T3 | T5 真实轨迹与禁止代执行检查 |
| H15 局部/零问题冒充通过 | T3、T4 | T5 问题召回、干净样本、覆盖边界 |
| H16 中断/并发/重复覆盖 | T1、T3 | T5 故障注入与真实退出场景 |
| H17 误删有用历史逻辑 | 每个 Task | T4/T5 完整 Go/vet、升级与非本轮保护 |

## 统一交付与验收记录

每个 Task 提交一份 `docs/verification/1.8/task-N-acceptance.md`，其中 N 为任务号。只在有真实结果后填写；没有运行的项目标“未运行及原因”，不能填 PASS。

必须包含：

1. Task、分支、base SHA、candidate exact HEAD（完整 40 位）、变更文件和具体行为。
2. 前置 Task 的 accepted exact HEAD 与验收来源；T1 记录任务包 base。
3. 新增失败案例的实际失败原因、修复后执行命令和结果；不要把 0 tests 当 PASS。
4. 本 Task 覆盖的 H 编号、testName/HostScenario、日志或制品位置。
5. Windows/原生工具的版本与结果；源码所在 HEAD 与测试的 HEAD 必须一致。
6. 涉及报告时交付真实生成的 review.md、对应 result/run 信息、reportSha256；业务数据脱敏。
7. 涉及 Host/模型时标明测试类型：组件夹具、原生 Host 固定响应、真实模型自主执行、内网私有模型。列出模型/Host 版本、是否由模型调用 finish、是否由脚本代执行业务步骤。
8. 涉及发布时给出 Run ID、Job ID、包名、SHA256、升级 baseline 与用户文件保护结果。
9. 已知限制、未验证项、是否满足本任务门槛。研发自测结论与负责人验收结论分列。

证据文档的提交可能在产品测试 HEAD 之后；写清 `testedProductHead` 和 `evidenceCommit`。只有确认后续提交纯文档时才能引用前一个产品 HEAD 的结果；存在任何代码、模板、配置或打包变化必须重测，不能拿旧绿灯顶替。

验收人检查通过后记录 accepted exact HEAD，下一 Task 才可启动。实际业务源码、Token、模型密钥和敏感会话不提交公网；必要证据使用脱敏副本并保留内部核验渠道。

## 最终完成定义

T1 通过只证明报告机制；T3 通过才证明最小真模型自主闭环；T5 还必须完成真实 Windows 包、完整测试、三类真模型各三次 fresh run 和内网验证。

正常成功必须同时满足：模型自主调用 finish、工具返回 COMPLETE、review.md 真实存在且不是初始占位、风险/范围/内容正确、所选调用链未被扩大。双链回复前不能开始正式评审，种入问题必须被发现。磁盘不可写时正确结果是明确失败，不承诺不存在的报告。

内网无法访问、模型配置缺失等要明确标未验证；可交付候选包供验证，但不能宣布正式可用。版本发布与合并由负责人安排。

## 先发给谁

现在只需先将 [Task 1 任务书](task-1-report-runtime.md) 交给 Go Runtime 开发。每份任务书末尾都有可直接转发的研发指令；等 T1 通过验收，再把 accepted exact HEAD 和 Task 2 一起交接。
