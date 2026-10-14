# Codea Review 2.0｜基于 OpenCode 2.x 原生 Agent 的轻量 Java Review 方案

> 日期：2026-10-14
> 状态：**第二轮 CONDITIONAL PASS 反馈修订稿，待第三方独立复核**；尚未实施、尚未发布或完成内网验收
> 目标环境：Windows 内网、OpenCode 2.x TUI、公司私有大模型、Java/Spring Boot/Maven
> 说明：保留首轮 P1/P2 修订，增补第二轮 P1 安装/工作区一致性与 Shell 只读边界、P2 性能/真实项目验证；本次**仅更新原设计文档**，不改 Harness 1.8 或产品配置/代码。

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

**V1 明确不做**：全仓扫查、完整 AST 调用图、每次穷举全部受影响 Controller、方法级严格白名单、机器保证工具调用次数、独立 OCR 引擎、多代理并行、自动修复/执行测试、CI 自动评论、SHA256 正式审计证书及强权限沙箱。Review 的一个专用 primary Agent 是 V2 的原生配置，不属于多 Agent 编排，也不新建执行引擎。

普通需求的目标性能：变更文件数 5–20 且 diff 新增+删除行不超过 800 时，端到端 P50 ≤ 3 分钟、P90 ≤ 5 分钟；至少 10 次同模型独立测试后才能据此声称达标，当前尚未验证。

## 3. 核心架构与用户级配置、Skill 实际加载、Reviewer 权限（第二轮 P1-01/P1-02）

### 3.1 产品交付位置：默认 Windows 用户级，不写 Java 仓库

```text
OpenCode 2.x TUI（Java 项目工作目录）
  └─ /codea-review [可选 Controller]
       └─ 用户级 commands/codea-review.md（当前会话选择 reviewer primary）
            └─ 原生 skill 工具加载 codea-review
                 └─ SKILL.md + 两份必读 references
                      └─ Git Diff 快扫 → Java 风险假设 → 按需上下文 → 中文简报
```

**默认安装目标**：OpenCode 官方 V2 全局目录 `~/.config/opencode`。在默认 Windows PowerShell 用户环境中通常为 `$HOME\.config\opencode`，例如 `C:\Users\<用户>\.config\opencode`；不可误用 `%APPDATA%\opencode`、旧版或其他产品目录。若配置了用户主目录/便携模式/系统策略，以目标 OpenCode exact 版本发现的真实路径为准，**不猜目录，也不修改 Java 项目**。

```text
~/.config/opencode/
  agents/
    codea-reviewer.md                 # 一个原生 primary Agent，仅影响此 agent
  commands/
    codea-review.md                   # /codea-review 唯一预期入口
  skills/
    codea-review/
      SKILL.md                        # 两阶段步骤和失败处理
      references/
        java-risk-checklist.md        # 必读：Java 高风险判定
        report-template.md            # 必读：中文简报格式
```

上面是**未来实施布局，本轮不创建文件**。最小使用要求仅有已安装的 OpenCode 2.x、公司私有模型及本机 Git。Review 不将配置复制进 `.opencode`，因而不会因为首次安装把 Java 工作区变 Dirty。后续 T2 测试 fixture 可在**公开测试仓库**中单独建文件，不向真实业务仓库注入文件。不要顺手修改全局 `opencode.json(c)` 的 `default_agent`、Build/Plan 权限或原有 Provider。

**发现和冲突**（官方 V2）：
- 原生 Agent：`~/.config/opencode/agents/<id>.md`；Command：`~/.config/opencode/commands/<id>.md`；Skill：`~/.config/opencode/skills/<id>/SKILL.md`。
- 项目级同名 Command 会覆盖用户级；Skill 以精确**路径 ID** 为键，项目级 `.opencode/skills` 和显式 `skills` 路径可覆盖全局同 ID 的定义。Agent 的后加载配置可能合并/覆盖同名定义。**不默许覆盖**：若发现项目级 `codea-reviewer` / `codea-review` 同名对象，预检输出冲突路径并停止，要求用户在项目侧处理；不能在模型 Prompt 中假装读到了全局版本。
- Skill `name` 仅显示名称，实际调用 ID 是 `codea-review`。Skill 目录可内含 references，但 OpenCode 不会自动加载 supporting files。
- 必须在 Windows 中文、空格用户名路径和 Java 项目子目录中证明实际加载来源和 references 正确。T1 先在无项目配置的干净项目运行，再测试同名冲突 fail-closed。

### 3.2 Command 显式加载 Skill（保留首轮 P1-01，澄清 slash）

Command 是 Prompt 模板，**不等于**自动执行 Skill。未来全局 `commands/codea-review.md` 的设计示例（本轮不创建）：

```markdown
---
description: 只读检查当前分支已提交 Java 需求的中高风险
agent: codea-reviewer
---

执行 Codea Java 快速 Review；可选 Controller 重点：$ARGUMENTS
首个模型工具调用必须使用原生 skill 工具加载精确 ID codea-review。
Skill 加载失败或任何必读 references 读取失败，立即停止并报 SKILL_LOAD_FAILED；
不得回退为普通模型自述的 Java Review。
只用已提交 Git Diff；预检阶段核查 HEAD/基线和工作区变更，
对无关 untracked 只提示不读取，禁止混用工作区源码与提交证据。
只读、禁外联、不运行代码/测试/构建，不使用 Subagent。
输出已覆盖/未覆盖范围、真实中高风险 Finding 和待确认风险。
```

Skill `SKILL.md` 应有 `name: codea-review`、`description` 和 **`slash: false`**；经查 V2 官方 Skills 文档确实支持顶层 `slash: false`，并支持 `metadata.opencode/slash`，**两者不要同时写成冲突值**。T1 针对目标 **exact 2.x** 核验 `/codea-review` 实际映射的**是 Command，而非同名 Skill slash**；模型执行后工具轨迹必须有成功的 `skill(codea-review)`，然后成功读取 `references/java-risk-checklist.md` 和 `references/report-template.md`，最后才能 Git Review。静态配置正确不等于已在模型运行时成功；失败一律 T1 FAIL。

**禁止在 Command 模板使用 `!`+反引号 Shell 插值**：V2 官方文档确认其执行在 Agent 工具权限流程之外，会绕过后面的 Reviewer Shell 规则。`$ARGUMENTS` 只能作为待校验的用户目标文本，不能拼成任意 Shell 命令。

命令 `agent: codea-reviewer` 切换**当前会话**的 primary Agent；Review 后用户可手动切回 Build。单独命令不改变其他会话的 Build/Plan，也不写 `default_agent`。不要使用 V2 无实际执行效果的 `subtask` 冒充隔离。

### 3.3 Reviewer 专用 V2 权限：固定 Git 命令允许、动态只读 Git 人工授权

**选用原生 `codea-reviewer` primary Agent，不改全局 Build/Plan 配置**。以下仅是未来设计建议的核心权限形状（不是已经通过 Windows 验收的安全策略）：

```yaml
---
description: Java 已提交差异只读审查
mode: primary
permissions:
  - { action: "*", resource: "*", effect: deny }
  - { action: read, resource: "*", effect: allow }
  - { action: read, resource: "*.env*", effect: deny }
  - { action: glob, resource: "*", effect: allow }
  - { action: grep, resource: "*", effect: allow }
  - { action: skill, resource: codea-review, effect: allow }
  - { action: shell, resource: "git rev-parse --show-toplevel", effect: allow }
  - { action: shell, resource: "git rev-parse HEAD", effect: allow }
  - { action: shell, resource: "git rev-parse --abbrev-ref HEAD", effect: allow }
  - { action: shell, resource: "git rev-parse origin/master", effect: allow }
  - { action: shell, resource: "git rev-parse master", effect: allow }
  - { action: shell, resource: "git status --porcelain=v1 -z --untracked-files=all", effect: allow }
  - { action: shell, resource: "git merge-base *", effect: ask }
  - { action: shell, resource: "git diff --no-ext-diff --no-textconv --name-status -z *", effect: ask }
  - { action: shell, resource: "git diff --no-ext-diff --no-textconv --numstat -z *", effect: ask }
  - { action: shell, resource: "git diff --no-ext-diff --no-textconv --unified=3 *", effect: ask }
  - { action: shell, resource: "git show --no-ext-diff --no-textconv *", effect: ask }
  - { action: shell, resource: "*--output*", effect: deny }
  - { action: shell, resource: "*;*", effect: deny }
  - { action: shell, resource: "*|*", effect: deny }
  - { action: shell, resource: "*&*", effect: deny }
  - { action: shell, resource: "*>*", effect: deny }
  - { action: shell, resource: "*<*", effect: deny }
  - { action: shell, resource: "*\u0060*", effect: deny }
---
只读审查，不编辑、不运行测试/构建；动态 Git 命令需用户核对后授权。
---
```

**特别说明**：宽泛允许 `git diff ... *` 或 `git show ... *` 是不安全的，因为可加 `--output` 或利用 Shell 管道重定向；本版不再将这些模式设为 `allow`。完整命令参数存在变化的 `merge-base/diff/show` 仅允许 `ask`，由开发人员**逐条核对确认**是否确属约定的只读命令；不允许“永远允许”模式放宽授权，不能以手动批准绕过已配置的 `deny`。如审批提示过多影响效率，T1 应实际测量并决定是否缩小 V1 支持范围，而不是直接将通配 `ask` 改为 `allow`。固定形式之外的 `git commit/checkout/clean/push/reset` 以及外部工具均被兜底 `deny`，`edit/write/patch` 共用的 `edit` 操作同样被兜底禁止；未引入任何自定义 Git 执行工具。***注意：以上模式仅是审查方案示例，具体元字符解析由 V2 Shell scanner 决定，T1 实测前不得以“配置存在”宣称可阻断任何绕过***。

- **T1 必过负向行为**：`git diff --output=...`、`git show --output=...`、重定向 `>`/`>>`、PowerShell `;`/`|`/`&`/`$(...)`、Git 全局 `-c`/外部 diff/textconv、`git commit/checkout/clean/push/reset/stash`、脚本/网络命令、`edit/write/patch`、`execute`、`subagent` 和已接入的 MCP/插件。必须在**真实 Windows OpenCode Tool permission** 中被拒绝，不能“只生成文本不执行”充当负测；对实际待测工具注入请求并核对工具拒绝结果、工作区文件内容未变。若某种命令在规则下返回 `ask` 而非 `deny`，按“未被自动阻断”处理并判安全负测 FAIL，不允许测试员手动拒绝就算权限通过。
- Shell scanner 是按命令文本和解析语义检查，仍可能被新参数、别名、外部配置、命令执行链或环境变量规避，不能证明任意 Shell 输入不可产生写入。`git status` 等读取型 Git 命令也可能更新 Git 内部缓存/索引时间戳；我们的承诺是**不改业务源码/提交历史**，不是磁盘零写入。
- **故障降级**：如果 T1 无法同时满足基本差异读取与危险命令阻断，不向生产发布“机器级只读”；先选择更窄的 Git 命令操作集合或显式人工批准作为有限防误操作，再复核 V1 适用性。严禁为达到绝对保障改造 Harness 式权限网关。

**实际保障级别：Codea Review V1 是 OpenCode 原生权限提供的防误操作能力，不等同于操作系统级只读安全沙箱。** Tool Calling/命令 scanner 与默认全局策略必须按具体 exact 版本复验。

## 4. 已提交 Git 范围、Dirty Worktree 与 HEAD 证据一致性（第二轮 P1-01/P2-02/P2-03）

### 4.1 审查范围和工作区状态决策

- 默认 `/codea-review` 使用本地 `origin/master...HEAD` 的共同祖先作为已提交变更基线；不存在则尝试本地 `master`，都没有就报错，不擅自使用 `main`。`origin/master` 仅是**本地 remote-tracking ref**，可能过期，不自动联网 `fetch`；完整基线 SHA、merge-base SHA、HEAD SHA 和分支名必须在报告中提供。
- `/codea-review OrderController` 是**定向重点审查**，并非完整 Controller 审计或严格文件白名单；首轮仍盘点本次所有有效 Diff，随后重点验证其真实上游入口。
- Review 配置默认安装在用户级位置，**不在 Java 项目增加 `.opencode` 文件**。如果开发人员已有项目级 `.opencode` 新文件，它们可被记录为无关 untracked，并在配置冲突检测通过时继续，但绝不因此使用来源不明的同名 Skill。

V1 Dirty 检查采用**不建快照的保守相关性判断**（`git status --porcelain=v1 -z --untracked-files=all`，必要时查看 `git diff --name-status` / `--cached` 的状态，不以“无提示”代表干净）：

| 工作区情况 | 默认处理 | 如何保证证据 |
| --- | --- | --- |
| HEAD 中已提交的业务代码变更 | 正常审查 | 基线/HEAD SHA 与 Git object 一致 |
| staged/unstaged **已跟踪**文件修改 | **V1 保守停止**；无需费力证明无关。严格于“仅影响范围才停止”，避免漏掉间接依赖 | 不自动 stash/checkout/reset |
| 明确无关的 untracked（如非审查目标的临时个人笔记或非冲突 `.opencode` 配置） | 可继续；列入“未纳入的 untracked” | 不读取、不引用、不作为已提交证据 |
| untracked 与本次修改同路径、同包业务类、相关 Mapper/配置/引用，或可能遮蔽工具同名定义 | 停止 | 不把未跟踪代码当 HEAD 内容 |
| untracked 相关性无法判断、路径解析失败 | **停止**，并提示移动或提交后重试 | fail-closed |
| HEAD/基线在 Review 中改变 | 立即终止 `INCOMPLETE/SOURCE_CHANGED` | 废弃旧的上下文和结果结论 |

**不尝试通用依赖图判断**：明确无关的 untracked 的最低条件是文件路径不处于本次有效 Diff 及其明确关联的 Java/Mapper/配置路径中，未被选中/阅读、不属于模型将检索的相关业务包，并非同名 Agent/Command/Skill。任何疑问一律停止。untracked 下有 `.gitignore` 忽略的条目无法由正常 `git status` 完整发现，报告不宣称已检查全部忽略文件；忽略目录中仍可能包含工具配置或影响语义的文件，T1 验证此边界。

**工作区源码默认不作证据**：查找/grep 命中工作区代码后，必须从固定**起始 HEAD SHA** 的 Git 对象读取、核对同符号原文；Finding 证据只能来自确定提交；`git show <fixed-head>:<repo-relative-path>`（带 `--no-ext-diff --no-textconv`）或同等 Git blob 读取。不能从 `read` 的 Dirty 文件直接引用；若在 HEAD 对象中不存在，标为“非本次已提交证据”并停止相关风险归因。删除文件使用 merge-base 对应内容，重命名分别记录旧/新路径。

### 4.2 Git 差异处理与短上下文首扫

1. 预检：Git 根、分支、`HEAD` SHA、基线 SHA、`merge-base`、`status --porcelain`；使用 `--name-status -z` 列完整改动文件和状态，使用 `--numstat -z` 汇总变更。`-z` 的 NUL 原始字节可能不能从 V2 文本 Shell 工具无损返回；T1 必须在含中文、空格、重命名文件场景下实测。无法保真则转换到经验证的安全路径解析策略，不能按空格拆路径，更不能假设无文件。
2. **首轮使用 `git diff --no-ext-diff --no-textconv --unified=3 <merge-base> <fixed-head> -- <path>`**（可按实际体验选 `--unified=5`）。先查看每个有效文件的低上下文 Diff，再针对真实风险假设读取完整方法或用更大 `--unified`，不默认对每个文件 `--unified=20`。
3. 采用仓库相对路径；重命名记录 old/new，删除和二进制不伪造可读正文。`--` 分隔文件路径；用户输入的 Controller 仅作经过校验的符号筛选，不进入 Git 命令。禁止 arbitrary `-c`、`--output`、外部 diff/textconv 和 Shell 插值；动态命令按第 3.3 节显式授权。
4. **Shell 输出截断必须可发现且会改变结论**：建立状态清单和 numstat 的独立预期，逐文件核验 diff header、`@@` hunk 范围及实际返回内容，检测 OpenCode 工具输出的截断标记/行数上限、退出码、NUL/路径丢失、相互不符的文件数。文件 Diff 很长时拆成可读片段或缩小目标，不用静默截断的单次全量返回。若输出没有可信的完整性证明（例如只有部分 hunk、数据超上限、截断但无法分片恢复），该文件记为 **UNREVIEWED/截断**，执行状态 `PARTIAL`；不能靠模型记忆补齐，更不能说“全部文件已审查”。本规则仅要求识别/披露截断，**不引入截断恢复引擎或快照服务**。
5. 阶段边界及出报告前再次核对 HEAD、基线 SHA、tracked 状态和已允许的 untracked 分类。HEAD/基线移动或出现新增相关 Dirty 修改，停止为 `INCOMPLETE/SOURCE_CHANGED`；新增明确无关 untracked 可记录并继续。检查之间可能仍发生竞态，明确没有不可更改的快照证明。

### 4.3 默认路径过滤（保留既有规则）

按**仓库相对路径**过滤根目录和嵌套 Maven 模块的 `.opencode/**`、`.teamai/**`、`.code-harness/**`、`target/**`、`node_modules/**`、`build/**`、`dist/**`、`*.class`、`*.log`；`.git` 内部目录不进入 Git Diff。过滤只用于已提交**审查范围**，而非无条件禁止存在无关 untracked；配置冲突另按 `3.1` 检查。

保留 Java、Mapper.xml、pom.xml、application*.yml/yaml、测试及其他实际影响业务的资源文件。所有路径按仓库相对路径处理，不能仅用扩展名或目录名推断符号是否可达；若关键被排除文件确实影响业务，应披露为范围限制。保留排除文件数与规则说明；不照搬 OCR 的 `rule.json`。

### 4.4 规模、异常、权限失败的状态

有效变更超过 **25 个文件或 800 行新增+删除**，转为重点风险抽样与 `PARTIAL`（并列未审清单）。没有 Git 基线、工具请求被拒绝、输出被截断、引用未找到、已提交内容与工作区不可消歧、HEAD/基线改变，均不可给出完整审查结论。遇权限 `ask` 未批准的动态 Git 命令，应显示范围/证据读取不足，明确停止或 `PARTIAL`；**不要尝试调用其他工具绕开授权**。

## 5. 两阶段 Java Review 与停止条件（保留质量门槛，补充小 Diff 首扫）

### 5.1 第一阶段：全部有效 Diff 快速扫描

- **首先用 `--unified=3` 的小上下文 Diff** 覆盖每一个有效文件，再汇总变更方法、Mapper SQL、配置、数据写入及跨服务边界，按 CRUD / Service-Mapper / 状态流转与资金敏感变更分组；出现截断或无法确认完整性时该文件标 `UNREVIEWED`，总执行状态为 `PARTIAL`。
- **不**在第一阶段为每个文件强制做完整调用链搜索；只标记具体的待验证高风险假设。
- 优先找可能线上故障的事务、并发、幂等、数据一致性、金额精度、权限/租户越权、危险 SQL、关键状态转移。忽略纯样式、命名及缺乏证据的泛化优化建议。
- 所有有效 Diff 都要进入首轮范围清单；超预算/大变更必须列出没有覆盖的部分，不能静默省略。

### 5.2 第二阶段：有明确问题才进一步检索

每次 `read/grep/glob` 之前先描述一个待验证假设，例如“入口层是否已有 requestId 幂等”“事务是否被 Spring 代理”“Mapper UPDATE 是否约束 tenant_id”“此 Service 被哪些业务入口调用”。按最短链路查询直接 Controller/Service/Mapper 或明确的消费者、定时任务与接口实现；寻找到足够判断的证据就停止。

- 两轮上下文扩展只是**软提示**，不是工具级硬上限，不能因轮数耗尽而隐藏尚存的重大风险；转为“未确认/部分审查”并建议人工深度检查。
- 无新证据、重复检索相同关键词、风险假设已解决或收益低于额外扫描成本时停止扩展。
- Java 多实现、AOP/事务代理、反射、动态 SQL、RPC/MQ 调用无法确认时保留歧义；**模型推断的调用链不是可证明的完整调用图**。
- 指定 Controller 只用于优先验证它关联的变更，仍需显式标注未被深入检查的其他有效 Diff。

### 5.3 规则优先级和证据要求

| 优先级 | 检查点 | 证据门槛 |
| --- | --- | --- |
| HIGH | 重复扣款/幂等缺失、资金金额精度、事务回滚失效 | 变更点 + 可达触发条件 + 核心保护缺失或失效的正反例 |
| HIGH | 跨租户越权、状态流转非法、缓存/数据库不一致 | 明确身份/对象/状态路径，排除上游有效校验 |
| HIGH | 竞态、无条件 SQL 更新/删除、重试副作用 | 具体并发/SQL 路径，能说明线上业务影响 |
| MEDIUM | 严重 NPE、边界遗漏、SQL 条件错误、资源泄漏 | 可复现条件，影响业务主路径 |
| 不报告 | 格式、命名、一般性能猜测或风格优化 | 不进入 Finding |

**确认 Finding** 要有：触发条件、文件/行/符号（优先变更行）、最小代码证据、业务影响、建议及验证用例；要能区分“本次引入”与“旧缺陷被触及”。没有足够证据只能列“待确认”，不能算 TP 或已确认问题。

### 5.4 最多五条的准确语义

**默认首屏最多展示五条最重要的已确认中高风险问题，但不能丢弃剩余已确认 Finding。** 输出“已确认问题总数、首屏展示数量、其余问题数量”；其余项继续在“其他已确认问题（折叠明细/附录）”中列出每项的严重度、文件行号及一句话事实，用户可要求展开完整细节。报告只读呈现，不为了追求简洁隐去真实问题；重复根因可合并，须保留受影响位置。`PARTIAL` 不能伪称找齐其他高风险缺陷。

## 6. 中文 Review 输出契约（P2-03）

默认仅在当前 TUI 输出，**不**自动创建文件、运行测试、修改代码或写审计证书。输出模板（示意数据，不是测试结果）：

```markdown
# Codea Review（快速）
基线：origin/master（基线完整 SHA：...）；merge-base：...；HEAD：...
有效变更：17 文件 / ... 行；排除：...；未审：...
执行：COMPLETE / PARTIAL / FAILED
结论：已确认 HIGH 2 / MEDIUM 1；待确认 1
首屏：展示 3 / 已确认总数 3；其余 0

## 重点风险（最多 5 条展开）
1. [HIGH] OrderService.java:123 重复扣款（关联 OrderController.create）
   触发条件与证据：...
   实际业务影响：...
   修复建议：...
   验证案例：...

## 其他已确认问题（若超过 5）
- [MEDIUM] SomeService.java:88 — 一句话风险描述
（每一项均可进一步展开；不隐去问题数）

## 业务入口与未解析路径
- 已证实关联：OrderController.create → OrderService.create
- 未解析：动态实现/其他入口尚未穷举

## 覆盖与限制
- Dirty Worktree：预检已通过 / 若失败则 Review 未开始
- 排除：.opencode、.teamai 等（数量及原因）
- 未审/未知：...；搜索停止原因：...；截断文件及原因：...
- 未纳入的无关 untracked：...（不被读取、不作 Finding 证据）
```

执行状态、覆盖范围、风险判断分离。没有确认 Finding 只能写“在实际审查范围内未发现有证据的中高风险问题”，**不能**等同零缺陷、审查完所有调用链或允许上线。`COMPLETE` 仅指计划的有效 Diff 首轮覆盖并完成风险驱动验证，**不是**全系统证明安全。用户明确要求落盘时须提示与默认只读模式冲突，需要另行切换到允许写的开发会话，由用户确认后保存；本轮 Codea Reviewer 永不执行写入。

## 7. Windows 内网、安全边界、用户级分发与回退（第二轮 P1-01/P1-02/P2-06）

### 7.1 环境、权限和网络预检

T1 记录 Windows x64、**OpenCode exact 2.x 安装版本及来源/校验**、Git 版本、公司私有 Provider/Model（不记录 Key）、TUI Tool Calling `skill/read/glob/grep/shell` 真实调用、全局用户目录实际解析路径。不能把 OCR 测通模型等同 OpenCode 工具加载测试。

企业内网预检覆盖启动、Skill/Agent/Command 加载、Review、首次安装和更新期间的 DNS/连接目标；仅允许已批准公司模型端点。检查自动更新、第三方插件、遥测、公开 Web、分享和其他出站行为，要求**真实联网轨迹无违规外联**。如果用户全局配置里存在自动更新，按 exact 2.x 官方配置关闭：不要误把 V1 `autoupdate`、旧版 `update` 字段当成已验证写法；OpenCode V2 配置文档可能随具体构建变化，T1 先读取目标版 schema 与日志，再用企业批准的方式禁用。现有全局 Provider 和 Build 配置只读检查，不擅自替换。

### 7.2 原生权限保障的真实边界

只在 `codea-reviewer` Agent 下追加 `permissions`，默认 `deny` 写工具、非约定 Shell、`execute`、`subagent`、外联及可见 MCP。固定形式 Git 可按 `3.3` 精确 allow，动态 Git 仅 `ask` 且审阅完整命令、严禁持久“always allow”降低防误操作。**不**把 `git diff/show *` 宽泛放开为 `allow`，不信“read only”自然语言即可保证安全；不用 `!` Command Shell 插值。

T1 必须对 `--output`、重定向、命令组合、外部 diff/textconv、Git 变更子命令、编辑工具及外部执行做真实工具层拒绝测试，并核对原始 Java 源文件 hash 与 Git status 未意外改变。V2 Shell scanner/权限是**文本和工具级的防误操作**，不是覆盖所有 Git 参数、环境与 Shell 构造的完整副作用静态证明。若实际 `ask` 可以访问不应该被批准的操作，不能把测试员点击“拒绝”当作自动防护通过，需进一步缩小 V1 支持的 Shell 模式或明确发布限制。

**保障级别准确表述**：**Codea Review V1 提供基于 OpenCode 原生权限的防误操作能力，不等同于操作系统级只读安全沙箱。** 不重新开发 Harness 权限隔离、Git 网关或自研执行引擎。

### 7.3 用户级离线安装、升级与卸载

- **安装**：企业认可的离线 ZIP 只含 `agents/codea-reviewer.md`、`commands/codea-review.md`、`skills/codea-review/SKILL.md` 及两份 references；解压/复制到已确认的 Windows `$HOME\.config\opencode` 下对应目录，**不写 Java 项目的 `.opencode`**，不依赖网络安装包或脚本下载。不默认触碰现有 `opencode.json(c)`。
- **预检**：检查目录发现、同名文件（global/project/兼容 Skills），对冲突显示路径并停止；已存在同名非 Codea 文件不得覆盖。核对原生 `/codea-review` 指向全局 Command、`skill(codea-review)` 指向全局 Skill 和正确 Reviewer primary。首次安装选择自造的**干净 Java 仓库**验收，无需把工具配置提交到业务 Git。
- **升级**：复制前由人工备份当前同名三类配置及 supporting files，并在发布包中附文本版本及 SHA256；先替换配置文件、重启或依 V2 的真实热重载能力验证，再跑 T1 预检，不改项目 Provider/Build/Plan 的设置。这里不设计安装器或自动升级服务。
- **回退**：恢复用户级备份或移除本次专属三个对象及 references，重启/重新加载并切回 Build；不修改 Git 工作区、公司模型、原 Harness 1.8 或其他团队的配置。系统级 `%ProgramData%\opencode` 若存在强制策略，本设计不得覆盖，须在 T1 检查权限冲突。

## 8. T1～T3 任务与不可降级的验收门槛（P1-01～04/P2-01～06）


### T1：用户级配置、真实 Skill、Git 范围和防误操作闭环

**未来产物**：用户级 `agents/codea-reviewer.md`、`commands/codea-review.md`、`skills/codea-review/SKILL.md` + 两份 references，离线文本 ZIP 与测试 Git fixture；**本轮不创建**。

| ID | 验收输入 | 可证实的通过条件 |
| --- | --- | --- |
| T1-01 | 全新 Windows Java 项目首次从离线包安装 | 配置加载自用户目录、不创建业务项目文件；`/codea-review` 解析为全局 Command，Reviewer primary 正确 |
| T1-02 | Skill 正常/缺失/禁止加载、缺 references | 真实工具轨迹含 `skill(codea-review)` 成功且两个 reference `read` 成功；负例 `SKILL_LOAD_FAILED` 并中止 |
| T1-03 | 项目级与用户级 Agent/Command/Skill 同名，及 `slash` 配置冲突 | 精确识别最终来源、预检阻断冲突，不以不同目录中的错误定义继续审查 |
| T1-04 | staged、unstaged 已跟踪业务文件（分别及混合） | 严格停止；不 stash/clean，不引用工作区证据 |
| T1-05 | 与业务变更无关的 untracked（含项目级非冲突配置）、影响审查的 untracked、无法分类的 untracked | 明确无关的继续并列“未纳入”；相关/未知的停止，Git HEAD 证据始终从固定 commit 读取 |
| T1-06 | HEAD/基线审查中改变、dirty 新增或 Git ref 更新 | 中止并显示 `INCOMPLETE/SOURCE_CHANGED`，不继续使用旧 Finding |
| T1-07 | 真实 Git 变更：增删/重命名、中文/空格、嵌套模块、`-z` NUL | 路径/状态与预期完全一致，不完整或 NUL 不保真则 fail-closed |
| T1-08 | 两阶段小上下文 `--unified=3`、多文件长输出/截断 | 所有有效文件列出首扫结果；超工具限制时显式 `PARTIAL/UNREVIEWED`，不可称 COMPLETE |
| T1-09 | Shell 重定向、`git diff --output`、`show --output`、`-c`、外部 diff/textconv、命令组合、`git commit/checkout/clean/push/reset`、外部执行/MCP/`edit/write/patch` | 必须有**权限拒绝**工具证据且业务源码 hash、状态不变；仅模型说不执行或用户手动点拒绝不合格 |
| T1-10 | 动态 Git 命令及 `$ARGUMENTS` 注入、`!` Shell 插值 | 仅受控 Git 工具操作可申请授权；不执行参数注入，禁止 Command Shell 插值；拒绝被拒绝命令的替代绕过 |
| T1-11 | 精确 OpenCode 版本、公司私有模型 Tool Calling、网络与 Build/Plan 回归 | Windows TUI 真运行，无未授权外联；Review 结束切回 Build 正常开发，既有 Plan 不变 |

T1 任何真实加载失败、危险工具负测未被权限拒绝、无法可靠读取差异或违反离线边界，均判 **FAIL**，不进入 T2。注意 Shell 动态 Git 人工授权耗时须记录，避免为了“安全配置好看”导致业务使用体验不可接受。



### T2：Java 必过缺陷样本与不误报反例

**将来交付**：`java-risk-checklist.md`、`report-template.md`、人工事实标签和自造/开源许可合规的 Java Spring Boot + Mapper/XML fixture。评审提示**仅包含任务及 Git 范围，不泄露预期 Finding**；真实 OpenCode 2.x Agent 自行检索、定位、判断。

每个 fixture 必须附：**最小业务代码/变更、预期 Finding 的确切变更文件与代码证据、触发/验证路径、干净反例、人工判定标签**。至少覆盖下表：

| Fixture | 故障植入（正例）及必须发现的结果 | 配套反例（必须不误报） |
| --- | --- | --- |
| F01 幂等/重复扣款 | 同一 `requestId` 的重试重复扣账且无有效幂等保护；必须报告 HIGH，指出重复扣账路径和触发条件 | 在入口/持久层有事务性幂等键、重复请求不会重复扣账 |
| F02 事务回滚失效 | `@Transactional` 方法吞掉运行时异常并返回成功，先前写入部分提交；必须定位该控制流和数据不一致 | 传播异常或显式标记 rollback-only，能回滚 |
| F03 金额精度 | 新代码用 `double` 做资金求和/转化，产生可重现精度损失；必须定位运算点和具体错误输入 | `BigDecimal` 以字符串构造且统一舍入规则 |
| F04 权限/租户越权 | 新增对象 ID 操作未校验当前主体/tenant，可修改其他租户对象；必须定位更新路径及越权条件 | 上游/DAO 有可证明的租户限定与权限检查 |
| F05 危险 SQL | 新变更 Mapper `UPDATE/DELETE` 遗漏关键 `WHERE`，可能批量误写；必须定位对应 XML/SQL 与风险 | SQL 使用正确 `WHERE id AND tenant_id` 等业务约束 |
| F06 多 Controller 业务影响 | 共享 Service/Mapper 新增风险，至少两个静态可证实 Controller 入口；必须找到已证实的两个入口及具体依赖证据，不得编造“已穷举所有入口” | 一个动态代理或多实现无法确定时明确 `UNRESOLVED`，不猜唯一目标 |
| F07 历史缺陷归因 | 提交只改日志/格式，原有旧风险未由变更引入；不得报告成新增缺陷 | 与该旧风险直接有关的业务变更可指出变化带来的新影响，但需证据 |
| F08 干净业务修改 | 有效事务/幂等/权限校验与正常 SQL 的小型 CRUD 变更；不应臆造 HIGH/MEDIUM | 用配对植入版本确保审查能识别实际缺陷，不靠总是输出零问题通关 |

对 F01–F05 **任一真实高风险样本漏报 = T2 FAIL**；F01/F02/F04/F05 的有效保护反例出现对应误报 = T2 FAIL；F06 错报完整链路或漏掉可证实的两个入口 = FAIL；F07 把旧问题归为新增 = FAIL。必须保留失败轨迹，分析是模型遗漏、Skill 提示、Git 输入、工具限制还是业务证据不足，并修正后完整复测，不因“已记录 FN”而过关。

记录 TP/FP/FN、路径证据、工具调用、样本版本及模型配置；必要时对关键样本重复独立会话评估稳定性。**不虚构整体召回率指标，也不承诺能够机械穷举所有 Java 动态调用图**。只报告有证据的中高风险 Finding，不为了速度牺牲必过高风险样本。

**T2 增加一项非替代性的真实开源项目验收**：采用公开 [macrozheng/mall-tiny](https://github.com/macrozheng/mall-tiny) 的指定 **exact commit SHA**、许可证归档和离线源码副本（进入公司内网前通过批准流程引入），作为真实 Java/Spring/MyBatis-Plus 项目复杂度样本，不依赖 T2 时的外网访问。使用其已有 Controller → Service/ServiceImpl → Mapper/XML、`ServiceImpl/BaseMapper` 继承方法、动态 SQL 等真实结构。创建**仅用于本地测试、可回退的单需求 Git 提交**或选定历史 PR/commit，事先由审核人员人工标注预期关联、真实风险/反例及变更位置；模型 Prompt 不透露答案。至少验证一次 Service/Mapper 修改到真实 Controller 的可证实路径、一次继承/动态关系 `UNRESOLVED` 的诚实报告、一次误报判定及一次读/搜次数统计。真实项目不要求全量静态调用图，实际已存在缺陷未经确认不得编造成“已发现”。F01～F08 的必过门槛**不因加入真实项目降低**。

### T3：实际公司 Windows 内网验收与性能

在正式 Windows 内网机器使用**精确 OpenCode 2.x 版本、公司真实私有 Provider/Model 和 TUI**，至少十次独立 Review；覆盖三个分层场景：

| 场景 | 最低覆盖 | 重点 |
| --- | --- | --- |
| 简单 CRUD | 已提交、少量 Java 文件及测试 | 少误报、少无意义跨文件读取 |
| Service/Mapper | 至少一次 Service + Mapper/XML 修改 | 按证据搜索必要 Controller 和 SQL |
| 状态流转/幂等/资金敏感 | 至少一例有真实高风险业务路径 | 不为缩短耗时遗漏高风险论证 |

额外复测历史 **17 个小改动文件**规模；记录端到端耗时、P50/P90、实际有效文件首轮覆盖、`read/grep/glob/shell` 调用次数、重复无效检索、已确认 Finding/误报/漏报/未解析路径、Skill 实际加载与权限拒绝轨迹、Git 前后状态以及外网连接检测结果。

**增加 A/B 基准：同一 Windows 机器、同一 Git Diff、同一私有 Provider/Model/模型参数、尽量相近的并发负载，分别运行 A＝OpenCode 原生快速 Review 与 B＝Codea Review 2.0；每组场景分别记录时长、工具调用、重复 `file_read/code_search`、HIGH Finding 的 TP/FP/FN、未解析路径及覆盖清单。优先比较本次 17 文件样本，按 CRUD／Service-Mapper／状态/金额三个场景拆分结果。B 相对 A 明显变慢或漏掉高风险缺陷时不能仅凭总量指标宣称改进；需给出证据和修订建议。无需安装 PR-Agent/reviewdog/Semgrep 或 OCR 来跑 T3；它们只提供设计参照。**

**性能目标仍为 P50 ≤ 3 分钟、P90 ≤ 5 分钟（在同一模型、同一机器、同一并发条件下测量）**；未达标必须真实写 FAIL 或列改进项，不能暗中删除高风险审查步骤凑时长。T3 的通过前提包括：T1/T2 全部通过、实际内网没有未经批准的外联或源码写入、真实模型没有“假成功”。缺失线上内网证据不能用公开模拟模型或 CI 绿色结果替代。

## 9. 风险、边界与后续

主要风险：原生 Agent 更快但严重缺陷漏报、路径排除失准、上下文搜索发散、内网模型工具调用不稳定、Java 多态/反射链不完整。采用 fixture 正反例、范围清单、软搜索预算及“部分完成”状态治理；真正需要硬约束时另立需求，优先利用 OpenCode 官方权限/工具机制，不复建大系统。

**本次交付是待评审的设计文档，不包含可运行 Skill，也不代表新技术方案已通过研发验收。** 用户确认设计后，再按照 Superpowers writing-plans 规范输出到 docs/superpowers/plans/ 的逐任务实现计划，之后才进入产品开发。

## 10. 首轮审核问题修订映射及官方依据（保留历史记录）

| 审核项 | 修订章节 | 实施前验证门槛 |
| --- | --- | --- |
| P1-01 | 3.2、8/T1-01/02 | skill 工具返回 + reference read 轨迹；失败中止 |
| P1-02 | 4.1/4.2、8/T1-04/05/06 | staged/unstaged/untracked 阻断、HEAD 一致、正反例 |
| P1-03 | 3.3、7.2、8/T1-08/10 | 单 reviewer primary 权限、Shell 负测、不影响 Build |
| P1-04 | 8/T2 F01–F08 | 必过高风险样本和不误报反例，漏报直接 FAIL |
| P2-01/02 | 4.1–4.3、8/T1-07 | NUL/重命名/中文与空格、base SHA 与无联网 |
| P2-03 | 5.4、6 | 五条仅是首屏，完整已确认问题不得遗失 |
| P2-04 | 5.1/5.2、8/T3 | CRUD/Service-Mapper/资金风险分层并测 P50/P90 |
| P2-05 | 7.1、8/T1-11 | exact 版本、私有模型、Tool Calling、真实离线网络 |
| P2-06 | 4.1、5.2 | Controller 只是重点归因，不能宣称严格隔离或全量审计 |

**V2 官方依据（以被部署的 exact 版本为最终运行验收基线）**：
- [V2 Commands](https://opencode.ai/v2/docs/commands)：`agent` 会切换会话当前 Agent；`subtask` 无隔离作用；`!` Shell 插值在 Agent 工具权限之外执行。
- [V2 Skills](https://opencode.ai/v2/docs/skills)：Skill 按精确 ID 调用 `skill`；`references` 不会自动加载；`slash: false` 阻止自动同名 slash。
- [V2 Agents](https://opencode.ai/v2/docs/agents)：一个 primary Agent、独立 `permissions`、Plan Agent 与默认 Build 边界。
- [V2 Permissions](https://opencode.ai/v2/docs/permissions)：`permissions` 有序、`shell` / `edit` / `skill` / `subagent` / `execute`；Shell 不是安全沙箱。
- [V2 migration](https://opencode.ai/v2/docs/migrate-v1)：不得误用 V1 的 `permission` / `bash`。

**修订边界**：仅设计文件修改。下一步提交给独立审核，**审核通过后才另行编制 T1 实施计划**；本次不触发代码实现或发布。


## 11. 第二轮审核问题修订映射、依据与遗留限制

| 项目 | 设计落点 | T1～T3 核查点 |
| --- | --- | --- |
| P1-01 用户级配置与 Dirty 冲突 | 3.1、4.1、7.3 | T1-01/03/04/05；默认不写 Java Git 工作区，允许可证实无关 untracked |
| P1-02 Git Shell 可写副作用 | 3.3、7.2 | T1-09/10；不再将动态 Diff/Show wildcard 直接 allow，真实拒绝轨迹 |
| P2-01 Skill `slash` 验证 | 3.2 | T1-02/03；核对 exact V2 顶层 `slash: false` |
| P2-02 缩小 Diff context | 4.2、5.1 | T1-08，`--unified=3` 先扫描、必要时深入 |
| P2-03 工具输出截断 | 4.2、4.4 | T1-08，`PARTIAL/UNREVIEWED` 与统计一致 |
| P2-04 真实 Java 项目 | 8/T2 | mall-tiny 指定 SHA、受控离线镜像、人工预期证据 |
| P2-05 分层质量 + 性能 | 8/T3 | CRUD、Service/Mapper、资金/状态，P50/P90 和 A/B |
| P2-06 离线检查 | 7.1、8/T1-11 | 模型调用、更新、遥测、插件、外网连接真实记录 |

### 已核对的官方资料

- [OpenCode V2 Config](https://opencode.ai/v2/docs/config)：全局目录、用户级/项目级发现、更新及 Output 限额；具体版本的配置字段可能变化。
- [OpenCode V2 Agents](https://opencode.ai/v2/docs/agents)：`~/.config/opencode/agents`，单 primary 模式，原生 `permissions` 与 Build 区分。
- [OpenCode V2 Commands](https://opencode.ai/v2/docs/commands)：`~/.config/opencode/commands`；项目命令覆盖全局；`!` Shell 插值在 Agent 工具权限之外。
- [OpenCode V2 Skills](https://opencode.ai/v2/docs/skills)：`~/.config/opencode/skills`；顶层 `slash: false` 合法；项目同名 ID/显式资源可覆盖，全局 Skill references 不自动读取。
- [OpenCode V2 Permissions](https://opencode.ai/v2/docs/permissions)：`permissions`、有序最后匹配、`shell` 的完整原始命令文本、宿主 Shell 不等于文件系统沙箱。
- [Git diff options](https://git-scm.com/docs/diff-options)：`--no-ext-diff` / `--no-textconv`；`--output` 等 Git 参数会影响读写边界，应负向测试。
- [mall-tiny 项目](https://github.com/macrozheng/mall-tiny)：公开 SpringBoot/MyBatis-Plus Controller、Service、Mapper/XML 示例；只用于独立模型质量验收。

### 设计层仍无法保证、不得伪装已解决

1. **不能保证只读安全沙箱**：V2 Shell scanner 是命令文本层面的防误操作，规则并非形式化 Git 参数解析器；Git/PowerShell 新绕过形式不能被此设计穷尽。任何关键负向测试不通过都不能宣称 T1 已验收。
2. **不能保证 Prompt 100% 遵守**：模型可能漏加载 Skill、遗漏某个风险或重复检索；以工具轨迹和必过 fixture 验证，失败就标 FAIL。
3. **不能确保所有 Dirty 相关性可判定**：无关 untracked 可以继续，但遇潜在语义影响或冲突时必须停止；不引入快照/依赖服务。
4. **不能保证长输出全部可见**：截断、路径 NUL 丢失等必须 PARTIAL；必要时用户缩小范围。
5. **不能承诺 V2 任意版本同样运行**：用户级配置、frontmatter 和 Shell 权限以目标 exact 构建验证；当前只是文档依据，尚未取得该企业安装包运行证据。

**本轮只修改本设计文档；不创建 Agent/Command/Skill、安装器、测试 fixture 或新 Review Runtime，不进入 T1 开发。修订后提交原设计分支，等待独立审核通过，再独立编写实施计划。**
