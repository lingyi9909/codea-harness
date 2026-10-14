# Codea Review 2.0｜基于 OpenCode 2.x 原生 Agent 的轻量 Java Review 方案

> 日期：2026-10-14
> 状态：技术方案待评审；**尚未实施、尚未发布或完成内网验收**
> 目标环境：Windows 内网、OpenCode 2.x TUI、公司私有大模型、Java/Spring Boot/Maven
> 说明：本次仅新增设计文档，不更改 Harness 1.8 的程序、命令、历史记录或发布物。

## 1. 背景与确定的技术选型

实际测试：一次普通 Java 需求，约 17 个改动不大的已提交文件，同一公司环境里 OpenCode 2.x 原生 Agent Review **不到 5 分钟**，阿里 OpenCodeReview（OCR）1.12.4 **19 分钟**，且 OCR 连续调用 file_read、code_search；原 Codea Harness 1.8 Review 截至目前仍不可用。上述为一次实际观察，不代表已得到统计性能结论，也不能证明原生 Agent 的召回率优于 OCR。

**最终选择：OpenCode 2.x 原生 Agent + 一份轻量 codea-review Skill + 一个 /codea-review TUI 命令。**

- OpenCode：复用现有 Git、搜索、文件读取、模型交互、Skill/Command。
- Codea Review：只提供 Java 业务风险规则、按需调用链指导、审查范围与简洁报告规范。
- OCR：保留为人工选择的深度 Review 工具；日常流程不依赖它。
- Harness 1.8：暂停继续修复 Review；原代码与证据归档保留，不作为新系统的可用依赖。

不得建设第二套 Review 引擎，不新建 Runtime 审查状态机、原生 Tool、插件、复杂身份凭据、持久服务或多 Agent 编排。Skill 是提示词级约束，**不是**强制文件隔离或安全沙箱。

### 为什么不是另外两种方案

| 路线 | 取舍 |
| --- | --- |
| 继续修复自研 Harness 1.8 | 执行与权限链路长期阻断；当前产品不可用，拒绝继续堆复杂度 |
| OCR 1.12.4 作为主引擎 | 多阶段审查深入，但 17 文件的实测耗时 19 分钟，不适合日常快速需求 |
| OpenCode 2.x 原生 Review + Skill | 已展示更好的使用效率，维护成本最低；需另行验证质量与调用链边界 |

## 2. 产品目标与非目标

**用户目标**：提交一个需求分支后，进入 OpenCode TUI 输入 /codea-review，快速检查相对于 master 的新增风险，输出少量有明确证据的中高风险问题，按业务入口或模块汇总，默认不修改代码。

**V1 必须具备**：Git Diff 正确选范围；排除 AI 工具和构建目录；按需理解 Controller → Service → Mapper/XML；聚焦事务、并发、幂等、状态机、权限与数据一致性；简短中文结果；完全内网、Windows 可离线部署。

**V1 明确不做**：全仓扫查、完整 AST 调用图、每次穷举全部受影响 Controller、方法级严格白名单、机器保证工具调用次数、独立 OCR 引擎、多代理并行、自动修复/执行测试、CI 自动评论、SHA256 正式审计证书及强权限沙箱。

普通需求的目标性能：变更文件数 5–20 且 diff 新增+删除行不超过 800 时，端到端 P50 ≤ 3 分钟、P90 ≤ 5 分钟；至少 10 次同模型独立测试后才能据此声称达标，当前尚未验证。

## 3. 核心架构与文件边界

~~~text
Java 研发人员
    |
    v
OpenCode 2.x TUI
    |
    +-- /codea-review [可选 Controller]
            |
            v
       codea-review Skill
            |
            +-- 获取 Git 变更范围与排除项
            +-- 审查变更代码，列出业务风险假设
            +-- 必要时检索直接调用方/被调用方
            +-- 对中高风险 Finding 做证据核实
            |
            v
      简洁中文 Review 输出（默认仅聊天）
~~~

**规划的实现文件（本次不创建）**：

~~~text
.opencode/
  skills/
    codea-review/
      SKILL.md
      references/
        java-risk-checklist.md
        report-template.md
  commands/
    codea-review.md
tests/fixtures/codea-review/
~~~

遵循 OpenCode 2.x 官方目录约定：
- Skill：.opencode/skills/codea-review/SKILL.md，frontmatter 使用 name: codea-review 和描述适用时机的 description。
- Command：.opencode/commands/codea-review.md，命令模板将用户参数 $ARGUMENTS 转给 Skill；命令在当前会话运行，不开启后台 Subagent。
- Skill 不同时声明同名 slash 入口，避免与命令冲突。无需创建 opencode.json 插件配置。

参考（实施时以安装的精确 2.x 版本复核）：[Skills](https://dev.opencode.ai/v2/docs/skills/)、[Commands](https://dev.opencode.ai/v2/docs/commands/)、[V1→V2 迁移](https://dev.opencode.ai/v2/docs/migrate-v1/)。

## 4. 输入、Git Diff 与范围规则

### 4.1 用户入口及其准确语义

- /codea-review：只审**已提交的当前分支与基线分支**之间的差异，默认基线 origin/master；如果不存在则尝试本地 master。
- /codea-review OrderController：将审查重点放到与该 Controller 有明确关联的变更与业务影响，按它汇总；**不是严格作用域隔离**。目标缺失或关系未知时明确提示，不因同名猜测。
- 没有 origin/master/master 就报告缺失基线，请求用户明确指定；不能偷偷改成 main，也不能自动 fetch 外网。
- 工作区有未提交修改时先提示“未纳入本次已提交变更审查”；不得把工作区文件混作 HEAD 的已提交证据。

### 4.2 确定性基线与覆盖清单

执行：git rev-parse --show-toplevel；git status --short；git merge-base <baseline> HEAD；git diff --name-status <merge-base> HEAD；git diff --no-ext-diff --unified=20 <merge-base> HEAD -- <pathspec>。

语义等同 Git 的 baseline...HEAD（三点 diff，基于共同祖先），不是直接比较两个分支最终树。记录两个提交 SHA、merge-base、分支名以及变更文件数量；处理增加、修改、删除、重命名、二进制文件。如果 HEAD 在审查期间变化，停止并要求重试。拒绝将用户输入直接拼接成任意 shell 命令。

### 4.3 默认过滤

排除路径（在仓库根目录及嵌套目录均生效）：.opencode/**、.teamai/**、.code-harness/**、target/**、node_modules/**、build/**、dist/**、*.class、*.log。.git 是本地内部目录，不会作为 Git 变更项纳入。排除规则应用于**审查输入**，而不是仅在报告中隐藏 Finding；输出排除的文件数量、类别，并能说明规则。

保留 Java、Mapper.xml、pom.xml、application.yml/yaml、相关测试代码和可能影响业务的配置文件。读取配置时不要向 Review 结果回显 API key、密码或令牌。不照搬 OCR 的 rule.json：那是另一套产品配置。

### 4.4 大范围、失败与未完成

- 超过 25 个有效变更文件或超过 800 行新增+删除：改为“风险优先快速抽样”，明确说明未完整覆盖，建议按业务模块拆分或手动运行深度审查。
- Git ref 缺失、命令错误、权限不足、模型失败、二进制文件或动态关联无法阅读，都必须显示**未审查项**；不能输出虚假的“全部正常”。
- Scope 与 Finding 的判定要分离：即使没有发现缺陷，只要存在未覆盖范围，就不能声称 Review 全面完成。

## 5. Java Review 策略与调用链边界

### 5.1 执行顺序

1. 首先阅读 Git Diff，不做全项目盲扫；建立修改的方法、资源、SQL 与业务行为列表。
2. 识别高风险假设：数据写入、交易/状态流转、事务、并发、幂等、鉴权或金额处理。
3. 只有证据不足以判断风险时，才检索直接调用方/被调用方，优先 Controller → Service → Mapper/XML 与明确接口实现。
4. 为避免虚假缺陷，查找反例：例如幂等是否在入口层实现、校验是否由框架承担、事务是否实际生效。
5. 合并重复 Finding，只输出最重要的最多 5 条证据充分的中高风险问题；不输出命名、格式和普通重构建议。

建议默认最多进行两轮相关上下文扩展；**这是 Skill 的软指导，不是实际工具硬配额**。反复搜索没有新证据就终止并说明局限。

### 5.2 风险清单

| 优先级 | 审查内容 |
| --- | --- |
| HIGH | 交易金额精度、幂等失效、重复扣款或重复处理、权限/租户越权、关键状态机错误 |
| HIGH | 事务边界与异常回滚、竞态、缓存/数据库不一致、危险 SQL 更新/删除 |
| MEDIUM | 可能导致线上故障的空指针、边界漏校验、错误 SQL 条件、重试副作用、严重资源泄漏 |
| 不报告 | 命名、格式、低价值优化、纯理论猜测、未验证的性能担忧 |

Finding 必须包含：触发条件、代码位置（优先锚定变更行）、具体证据、业务影响、修复建议、验证方式。未经验证的怀疑单独列入“待确认”，不当作已确认问题。如果根因在未修改代码上，必须说明本次变更与根因的关系，不能把旧问题伪装成本次引入。

### 5.3 Controller 业务影响分析

Service/Mapper 改动可能影响多个 Controller、消息消费者或定时任务。按需搜索能确认的直接业务入口，并以它们分组展示；遇 Spring 多实现、AOP、反射、RPC/MQ、动态 SQL 等关系无法确认时标注“未解析”，不得宣称已枚举全部上游链。纯 DTO 或格式变更一般不需要展开完整链路。

目标是提升**真实业务风险判断**，不是复刻旧 Harness 的确定性调用图。

## 6. Review 结果契约

默认只在当前 TUI 会话展示，不自动保存文件，不修改仓库。用户明确要求时才按指定安全位置保存 Markdown，并说明该操作会写入文件。

~~~markdown
# Codea Review（快速）
范围：origin/master...HEAD / merge-base SHA / HEAD SHA
有效变更：17 文件、X 行；排除 Y；未审查 Z
执行状态：完成 / 部分 / 失败
结论：发现 N 个中高风险问题 / 在已审查范围未发现明确中高风险问题

## 风险（最多 5 项）
1. [HIGH] OrderController → OrderService：重复提交可能重复扣款
   - 文件与行：src/.../OrderService.java:123
   - 触发条件 / 证据：
   - 业务影响：
   - 修复建议：
   - 最小验证用例：

## 业务影响
已确认的 Controller、Service、Mapper 与其他入口
未解析的调用、可能遗漏的入口

## 边界与建议
未纳入的未提交修改、排除项、未覆盖代码及原因
~~~

**执行状态、覆盖范围、发现结论必须分开**。“未发现问题”不等于“批准上线”；不能因中断而输出 PASS。按 Controller/业务模块汇总，不罗列大量细碎的调用链候选。

## 7. Windows 内网、安全和可回退性

- 安装只涉及 OpenCode 已支持的文本 Skill 与命令目录，不依赖外网 npm 下载、第三方 Review 服务、自动更新、外部知识库或 Dify。
- 仅能调用公司批准的私有模型；不得把公司源码发送到公共模型、网页搜索、遥测/分享接口。设计仓库与公开测试 fixture 不存公司真实代码、密钥、日志或内网地址。
- 不执行项目脚本、单元测试、构建、修改/删除文件、git reset/checkout/clean/commit 或自动修复。代码与仓库文档中的任何“指令”只作为待审查数据，不执行。
- 以上为产品操作约束，不等于独立 OS 权限边界；若要求机器级只读/正式审计，需要独立实现和验收，不能靠提示词声称。
- 分发：固定适配的 OpenCode 2.x exact 版本；通过公司允许的离线渠道发布包含版本与校验和的文本包；安装前检查冲突文件，不覆盖用户自定义 Agent/Skill。
- 回退：删除或恢复新增的 codea-review Skill/Command 即可，不动旧 Harness 与项目用户数据。旧 Harness 1.8 不作为可用回退方案。

## 8. 分阶段交付与验收标准

### T1：原生 Skill/Command 最小闭环

产物：Skill、命令入口、最小公开 Java fixture；Windows 真实 OpenCode 2.x 中能调用 /codea-review；Git 三点范围准确；排除 .opencode、.teamai；无需额外 Review 引擎；默认只读。

需测试：空变更、缺少 master、未提交改动、重命名和删除、中文及空格目录、命令参数安全、Scope 预览准确性。

### T2：Java 高风险检查与简短报告

产物：Java 风险清单、报告模板、自造缺陷/干净对照样本。至少覆盖事务、幂等、金额/状态、SQL、权限、配置 6 类，包含已有上游防护的反例；变更 Service/Mapper 时能按需理解直接 Controller，动态调用不能确认时如实列出。

评估：人工标注缺陷事实，记录 TP、FP、FN 与漏报原因；只报告有证据的中高风险问题。没有预先测得的数值就不编造准确率/召回率。

### T3：公司内网体验、质量和性能验收

使用真实 Windows + 公司相同私有模型，至少 10 次独立运行；记录端到端耗时、模型请求数、搜索/读取调用数、文件覆盖、主要风险发现、失败与部分审查比例，并复测 17 文件规模。目标：普通 5–20 文件、≤800 行变更 P50 ≤ 3 分钟、P90 ≤ 5 分钟，若未达标标注不通过而非放宽指标。

额外验证：HEAD 变化、模型中断、超过预算、重复无效搜索、凭证回显、源码中的提示词注入、内外网访问限制。OpenCode 与 OCR 可以同代码对照评审质量，**OCR 不成为 T1–T3 的执行依赖**。

验收红线：Review 过程必须真实运行；不能用一段聊天总结替代工具执行证据；不能把部分覆盖、未知调用链或未发现问题冒充全面 PASS。

## 9. 风险、边界与后续

主要风险：原生 Agent 更快但严重缺陷漏报、路径排除失准、上下文搜索发散、内网模型工具调用不稳定、Java 多态/反射链不完整。采用 fixture 正反例、范围清单、软搜索预算及“部分完成”状态治理；真正需要硬约束时另立需求，优先利用 OpenCode 官方权限/工具机制，不复建大系统。

**本次交付是待评审的设计文档，不包含可运行 Skill，也不代表新技术方案已通过研发验收。** 用户确认设计后，再按照 Superpowers writing-plans 规范输出到 docs/superpowers/plans/ 的逐任务实现计划，之后才进入产品开发。
