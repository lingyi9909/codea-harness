# Codea Review 2.0｜基于 OpenCode 2.x 原生 Agent 的轻量 Java Review 方案

> 日期：2026-10-14
> 状态：**CONDITIONAL PASS 审核意见修订稿，待独立复核**；尚未实施、尚未发布或完成内网验收
> 目标环境：Windows 内网、OpenCode 2.x TUI、公司私有大模型、Java/Spring Boot/Maven
> 说明：依据独立审核 P1-01～04 / P2-01～06 修订；本次**仅更新原设计文档**，不更改 Harness 1.8、产品 Skill/Command/Agent 或发布物。

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

## 3. 核心架构、Skill 确实加载与 Review 专用只读配置（P1-01/P1-03）

### 3.1 最小主链路及文件边界

```text
OpenCode 2.x TUI
   │ 用户 /codea-review [可选 Controller]
   ▼
Command (.opencode/commands/codea-review.md)
   │ 在当前主会话选择 codea-reviewer primary Agent
   │ 必须调用原生 skill 工具并验证返回
   ▼
codea-review Skill (SKILL.md)
   │ 必须按需读取 references（内容不自动加载）
   ▼
预检 Git / 状态 → Git Diff 快速扫描 → 风险假设验证 / 按需上下文
   ▼
简洁中文报告（无代码修改）
```

拟实施文件（**仅为设计路径，本轮不得创建**）：

```text
.opencode/
  agents/codea-reviewer.md             # OpenCode V2 原生单个 primary Agent 配置，非插件/编排
  commands/codea-review.md             # /codea-review 入口；不使用命令 Shell 插值
  skills/codea-review/
    SKILL.md                            # 两阶段步骤、失败停止语义
    references/
      java-risk-checklist.md            # 高风险规则，必须加载
      report-template.md                # 简报格式，必须加载
tests/fixtures/codea-review/            # 后续 T1/T2 才创建
```

这是三份配置入口（Agent、Command、Skill）而非新增引擎。Review Agent 仅是官方配置的**一个 primary Agent**，不创建或调用 Subagent；旧 Harness 1.8 不参与。`/codea-review` 是独立命令，不覆盖旧 `/harness-review`。

### 3.2 Command 对 Skill 的显式加载协议

OpenCode V2 Command 是提交到会话的 Prompt 模板，**不会天然加载同名 Skill**。V2 的 Skill ID 来自路径，不依赖 frontmatter 名称；`references/` 中的文件也不会自动注入。

T1 实施时，`.opencode/commands/codea-review.md` 的**建议最小模板（设计示例，不是已创建产品文件）**：

```markdown
---
description: 使用 Java 风险规则只读审查当前分支已提交变更
agent: codea-reviewer
---

执行 Codea 快速 Java Review。用户可选 Controller：$ARGUMENTS

首个模型工具动作必须调用原生 skill 工具，skill ID 精确为 codea-review；
只有获得成功的工具返回后，才执行后续 Git 或源码读取。
Skill 不存在、权限拒绝、工具失败，立即停止：报告“SKILL_LOAD_FAILED”，
不得改成通用 Review 冒充 Codea 专项审查。
按 SKILL.md 要求读取 references/java-risk-checklist.md 和 references/report-template.md；
任何必读文件不可读时停止并报具体缺失文件。
必须在审查前检查 Git ref 与工作区是否干净，只审已提交差异。
保持只读，不修改代码、不跑构建/测试、不执行外网调用；
审查输出必须区分已检查/未检查范围和已确认/待确认风险。
不要使用额外 Subagent。最后输出简洁中文结果。
```

**可验证的实际行为**：
- `skill` 工具入参 ID 必须是 `codea-review`；工具返回表示加载了正确 `SKILL.md` 正文。
- `SKILL.md` 正文明确要求先通过 OpenCode 原生 `read` 读取两个 `references/` 文件，按工具返回路径解析，不靠模型凭记忆复述；规则与报告模板为必读，其他可选背景资料才允许按需读。
- 把 `SKILL.md` 中 `slash: false` 写在 frontmatter，避免 V2 自动注册同名 `/codea-review` Skill 入口覆盖命令；`name: codea-review`、`description` 保留作为显示信息。目录 ID 大小写精确匹配。若同名 global Skill 与项目 Skill 冲突，先核对 V2 的 ID 解析结果。
- **可观察性不等于强制机制**：Prompt 要求调用工具，但模型可能遗漏。T1 从真实 OpenCode 会话工具轨迹核对 `skill(codea-review)` **实际成功**、随后两次 `read` 成功；只看到最终文本说“已加载”一律 FAIL。
- T1 负向：删除或禁用 Skill / 拒绝 `skill` 权限 / 缺一份 reference；必须明确停机。若真实模型不能稳定触发加载，T1 FAIL，**不得宣称 Command 已保证加载**；先调整 Command 提示，不自建 Loader/Plugin。
- OpenCode V2 `!` + 反引号的 **Command Shell 插值发生在 Agent 权限检查之外**。模板内严格禁止 `!` Shell 插值，也禁止把 `$ARGUMENTS` 放进任何 Shell 插值或命令字符串；Git 命令必须由受控 Agent 的工具调用执行。
- V2 的 `agent: codea-reviewer` 会将该会话当前 Agent 切换到 reviewer；Review 后用户如要正常开发须手动切回 Build。不要修改 `default_agent`，不改变其他会话的默认 Build 配置；`subtask` 在目标 V2 中不产生隔离效果，不作为保障。

### 3.3 只读技术取舍：Plan vs 单个 Reviewer primary Agent

**决策：采用轻量 `codea-reviewer` primary Agent 配置。** V2 内置 Plan 默认禁止一般编辑，但 **Shell 不默认全禁**，并允许写 OpenCode Plan 文件，不符合“只读 Review”预期。只为 Review 任务创建一份原生 Agent Markdown 配置，命令显式 `agent: codea-reviewer`；不全局修改 Build / Plan 权限。

建议的 Agent frontmatter（仅演示 V2 原生配置，不是本轮产物）：

```yaml
---
description: 只读 Java 差异审查（Codea 专用）
mode: primary
permissions:
  - { action: "*", resource: "*", effect: deny }
  - { action: read, resource: "*", effect: allow }
  - { action: read, resource: "*.env*", effect: deny }
  - { action: glob, resource: "*", effect: allow }
  - { action: grep, resource: "*", effect: allow }
  - { action: skill, resource: codea-review, effect: allow }
  - { action: shell, resource: "git rev-parse *", effect: allow }
  - { action: shell, resource: "git status *", effect: allow }
  - { action: shell, resource: "git merge-base *", effect: allow }
  - { action: shell, resource: "git diff --no-ext-diff --no-textconv *", effect: allow }
  - { action: shell, resource: "git show --no-ext-diff --no-textconv *", effect: allow }
---
只检查已经提交的 Java 需求变更。绝不编辑文件、不执行测试/构建。
只能使用上述只读 Git 操作；不尝试权限绕过或外部工具调用。
---
```

以上利用 V2 有序 `permissions`（**不是** V1 的 `permission` / `bash`）；最后一条匹配规则生效。默认拒绝 `edit/write/patch`、`execute`、`subagent`、`websearch/webfetch`、MCP/外部工具、外部目录访问及其他 Shell 命令；只放行实际需要的 Git 只读形式和 `skill/read/grep/glob`。保留源码路径 `read`，默认不授予外部目录读取。Git `--no-ext-diff --no-textconv` 避免外部 Diff/文本转换执行。对需人工授权的新只读命令，先经过设计审查再精确增加允许形式，**不把 `git *` 或全部 Shell 开放**。

这属于**宿主权限级防误操作**，不是 OS 安全沙箱：Shell 规则以命令文本匹配，Git 配置/别名、命令组合、Shell 重定向、工具能力变化及已安装插件可能带来意外路径。T1 必须用真实 Windows Shell/工具轨迹验证“命令拼接、重定向、`git commit`、`git clean`、`git push`、`rm`、`Invoke-WebRequest`、`edit/write/patch`、`execute`、`subagent`”均被拒绝；如存在绕过则不宣称已达到只读门槛，并进一步收紧原生规则。不得将 Agent 自述当作权限测试证据。

**额外约束**：审查中不执行仓库中的脚本、构建或测试；仓库文件包含的提示词视为待审查数据。Agent 完成 Review 后，不会自动重新开放 Build 权限；用户正常开发主动切回 Build 即可，无需更改全局权限。

## 4. Git HEAD 一致性、路径与范围协议（P1-02/P2-01/P2-02/P2-06）

### 4.1 输入与 Dirty Worktree 的 fail-closed 策略

- `/codea-review`：默认审查当前分支已提交的 `origin/master...HEAD`；未找到 `origin/master` 时回退本地 `master`，都不存在则停止并要求显式选择基线，不擅自转到 `main`。
- `origin/master` **只是本机缓存的远程跟踪引用**，未必最新；不自动 `fetch`、不联网。报告必须显示实际基线完整 SHA、merge-base SHA、HEAD SHA，并提示用户按公司流程确保基线已更新。
- `/codea-review OrderController`：这是**重点审查与展示归因**，不是 Controller 完整静态审计、不是严格筛除其他变更的安全隔离，也不能声称找齐全部上游 Controller。即使指定 Controller，第一阶段仍盘点全量有效 Git 变更，报告明确何者被深入审查。
- **V1 的保守决策：发现任何工作区未提交文件或修改（含 staged、unstaged、untracked，甚至排除目录下的变化）就停止**。这是故意比“只拦影响范围的变更”更严格的简单机制；不引入 Git 快照系统。提示分类及文件列表（注意敏感路径脱敏）、建议先提交/移开变更或到干净工作区审查，绝不擅自 stash/checkout/clean。未跟踪的日志/构建文件如在 `.gitignore` 中被忽略，默认 `git status` 不显示，不能对其作已核查声明。
- 预检使用 `git status --porcelain=v1 -z --untracked-files=all`，不依赖人类可读的 `git status --short`。T1 要验证其 NUL 分隔输出在 Windows OpenCode 工具链中未被截断；若未能可靠解析，必须停止并用人工可解释的保守检查替代，不能默认为干净。
- 用于证据的 canonical 内容优先从 `git show --no-ext-diff --no-textconv HEAD:<repo-relative-path>` 读取，绝不直接把工作区当前内容当作已提交 HEAD 证据。关联源文件也优先由 HEAD 读取，必要的原生 `read` 只作辅助上下文，并在报告证据前与 HEAD 一致性核实。删除文件从 merge-base 读取，重命名前后分别从对应提交读取。

### 4.2 确定性范围与双向版本校验

1. 记录项目根、当前分支、`HEAD` 提交 SHA、本地基线 SHA；未提交变更预检必须通过才开始。所有 Shell 由受控 Agent 调用，`$ARGUMENTS` 只能作待校验目标文本，不能插入 Shell 字符串。
2. `git merge-base <baseline> HEAD` 得到共同祖先；`git diff --no-ext-diff --no-textconv --name-status -z <merge-base> HEAD` 获取状态与路径，`git diff --no-ext-diff --no-textconv --numstat -z <merge-base> HEAD` 统计。与 `git diff <baseline>...HEAD` 的语义相同。尽量使用 NUL 路径分隔避免中文、空格、Tab、换行、引号产生误判。
3. `-z` 的二进制 NUL 输出在 OpenCode Shell 的文本返回中**可能不保真**：T1 用含空格、中文、重命名的 fixture 明确验证；若不能保真则失败并在实施时采用经过测试的非 NUL 安全转义解析策略，不能拆字符串靠空格匹配。
4. 以确定的相对路径读取 `git diff --no-ext-diff --no-textconv --unified=20 <merge-base> HEAD -- <path>`；新增/删除/重命名的 old/new path 与状态要分别记录；含 `..`、绝对路径、仓库外符号链接、注入参数前缀的用户输入必须拒绝或要求人工确认。不能仅依赖扩展名猜文件安全性。
5. **执行期间的有效性**：每一阶段开始及报告生成前重新查询 HEAD/基线 SHA 与 `git status --porcelain=v1 -z --untracked-files=all`。任何变更（包括用户中途 checkout、amend、新增未跟踪文件）立即终止为 `INCOMPLETE/SOURCE_CHANGED`，保留已发现但不宣称最终完整；不可继续消费旧 Git diff。检查之间仍有短暂竞态，因此不宣称“不可更改的快照审计”。
6. 解析失败、路径二义性、已提交对象不可读、仓库尚未提交、空 diff、binary diff 需分别给出可操作的状态；不能把失败或没有 diff 写成“没有缺陷”。

### 4.3 默认排除和保留

按**仓库相对路径**过滤根目录及任意嵌套模块的 `.opencode/**`、`.teamai/**`、`.code-harness/**`、`target/**`、`node_modules/**`、`build/**`、`dist/**`、`*.class`、`*.log`；`.git` 是内部目录，不进入 Git diff。注意配置加载目录（`.opencode/`）虽不审查其变更，但如存在未提交变更，仍须通过上述 fail-closed 预检。

必须保留 `src/**/*.java`、`**/*Mapper.xml`、`pom.xml`、`application*.yml` / `application*.yaml`、相关测试及确有业务影响的配置文件。对根目录与嵌套 Maven 子模块分别测路径模式；排除只作用于候选变更，必要上下文可只读查询，但不能把排除项作为新增 Finding 位置。报告同时列有效变更、排除项和未审项的数量。不要把 OCR 的 `rule.json` 当作 OpenCode 配置。

### 4.4 规模与终止

超过 **25 个有效变更文件或 800 行新增+删除**：仅作高风险优先扫描，标明 `PARTIAL`，鼓励缩小需求范围或手工触发深度 Review；不得不提示就遗漏文件。HEAD/基线发生变化、Dirty 工作区或关键路径/差异读取失败时直接停止；模型推理无法确认的动态边界列为“未解析”，不伪造确定性覆盖。

## 5. 两阶段 Java Review 与停止条件（P2-03/P2-04/P2-06）

### 5.1 第一阶段：全部有效 Diff 快速扫描

- 一次性汇总全部有效文件、变更方法、Mapper SQL、配置、数据写入及跨服务边界，按 CRUD / Service-Mapper / 状态流转与资金敏感变更分组。
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
- 未审/未知：...；搜索停止原因：...
```

执行状态、覆盖范围、风险判断分离。没有确认 Finding 只能写“在实际审查范围内未发现有证据的中高风险问题”，**不能**等同零缺陷、审查完所有调用链或允许上线。`COMPLETE` 仅指计划的有效 Diff 首轮覆盖并完成风险驱动验证，**不是**全系统证明安全。用户明确要求落盘时须提示与默认只读模式冲突，需要另行切换到允许写的开发会话，由用户确认后保存；本轮 Codea Reviewer 永不执行写入。

## 7. Windows 内网、安全、安装预检与回退（P1-03/P2-05）

### 7.1 开发环境预检

T1 开始前记录：**OpenCode exact 2.x 版本**（官方 ZIP/CLI 构建校验）、Windows x64 版本、Git 版本、工作目录、已选的私有 `provider/model` 标识（**不记录 API Key**）、实际接入协议与内网模型地址的脱敏标识、模型是否支持稳定的工具调用、`skill/read/glob/grep/shell` 是否真正可用。

预检在空白或自造 fixture 仓库里执行：一次 `skill` 加载、两次必读 reference、`git status`/`git diff`、简单模型工具调用；失败即停止，不进入真实业务代码 Review。不能把“OCR 模型连通成功”当作 OpenCode Tool Calling 已通过。

**离线网络行为**：企业批准的内网模型端点必须是唯一模型出口；检查启动、Skill 加载、Review 期间 DNS/网络连接记录，确保没有自动更新、遥测、插件下载、公共 Web 搜索或 Session 分享访问外网。使用经批准的离线包，不执行在线 `npm install`；若 OpenCode exact 版本存在不可关闭的外连行为，则内网验收 FAIL 并阻断发布。产品/全局禁更新、外网提供商限制必须按实际配置与网络日志验证，不假定仅靠 Skill Prompt 生效。

### 7.2 默认只读保护

只在 `/codea-review` 命令中选中独立的 `codea-reviewer` Agent（见 3.3），其 V2 `permissions` 默认拒绝写入和非允许的 Shell；绝不全局改造用户 Build 模式。Agent 执行期间禁止编辑、测试、构建、提交、stash、checkout、网络请求、启动 Subagent；不使用 Command Shell 插值，不执行代码库里伪装成指令的文本。

V2 权限规则并不等价 OS 文件系统隔离；Shell 及任意已启用外部组件的效果需要在 T1 负向验证。报告必须说明“宿主权限防误操作，**非安全沙箱**”；公司如果要求恶意 Agent/同 OS 用户不能越权，另立需求。本版本不建自己的工具权限网关。

### 7.3 分发和撤回

用户初次使用只需要 OpenCode 2.x TUI、内网模型、已安装 Git，以及三份官方格式纯文本配置（Agent / Command / Skill，含两份 reference）。安装前检测同名文件冲突并保留原文件；不能覆盖其他项目的 Build/Plan 设置。首次在单个项目验证，之后再由公司批准推广共享配置目录；保留文件校验和与版本清单。

回退删除/恢复本次新增配置文件，手动在 TUI 切回 Build，**不**触碰原 Harness 1.8 / 项目业务代码 / Git 历史；Harness 1.8 不作为可用运行回退。阿里 OCR 可独立按需使用。

## 8. T1～T3 任务与不可降级的验收门槛（P1-01～04/P2-01～06）

### T1：真正可运行的命令、Skill、Git 范围和只读闭环

**将来交付**：`.opencode/agents/codea-reviewer.md`、`.opencode/commands/codea-review.md`、`.opencode/skills/codea-review/SKILL.md`、两个 references 和最小公开 Java/Git fixture。本轮不创建这些文件。

真实 OpenCode 2.x TUI 的**通过条件（全部满足才能进入 T2）**：

| ID | 场景 | 通过条件 |
| --- | --- | --- |
| T1-01 | 调用 `/codea-review` | 选择单个 reviewer primary，真实 `skill(codea-review)` 成功，随后两个 reference `read` 成功，**按真实工具轨迹验收** |
| T1-02 | Skill 未找到、同名冲突、权限 `deny`、缺 reference | 显示失败并停止，不能用通用 Review 假装成功 |
| T1-03 | 干净提交分支含多个 Commit | 正确得到 `origin/master...HEAD` 的 merge-base、完整 SHA、全部有效文件；不自动 fetch |
| T1-04 | staged / unstaged / untracked 各自单独存在，以及三者混合 | 每种都阻断 Review；没有证据读取，不 stash/clean；忽略文件行为如实说明 |
| T1-05 | 工作区干净、没有未提交修改（正例） | 可读取由 `HEAD` 定义的真实 Java 变更与关联文件并输出报告 |
| T1-06 | 开始后 HEAD/基线更新、或新增未提交修改 | `INCOMPLETE/SOURCE_CHANGED` 并中断，不沿用旧 diff |
| T1-07 | 重命名、删除、中文、空格路径、嵌套 Maven `target`/`.opencode` | 路径状态和排除一致；`-z` 不保真时必须 fail-closed |
| T1-08 | `git commit/clean/checkout/push`、Shell 组合或重定向、`edit/write/patch`、`execute`、外网请求、Subagent | 真正被 reviewer 权限拒绝，Git 工作目录 hash/status 未改变；不能仅信模型说“不会执行” |
| T1-09 | `$ARGUMENTS` 含空格、特殊字符和恶意 Shell 文本，含 Command Shell 插值检查 | 参数仅作 Controller 文本目标；命令模板无 Shell 插值、无命令注入 |
| T1-10 | 用户正常 Build 工作流 | Review 配置不改变 Build 的正常工具权限；Review 完成后切回 Build 仍能正常开发 |
| T1-11 | 固定 OpenCode exact 版本 / 私有 Provider / Tool Calling / 离线网络 | 版本、模型、工具记录齐全且实际工具请求成功；无未经批准的外连 |

T1 不能以“Prompt 看起来正确”或静态文件存在代替真实工具轨迹测试。真实轨迹缺失、只读负例失败或离线网络行为不合规，均判 T1 FAIL，不进入 T2。

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

### T3：实际公司 Windows 内网验收与性能

在正式 Windows 内网机器使用**精确 OpenCode 2.x 版本、公司真实私有 Provider/Model 和 TUI**，至少十次独立 Review；覆盖三个分层场景：

| 场景 | 最低覆盖 | 重点 |
| --- | --- | --- |
| 简单 CRUD | 已提交、少量 Java 文件及测试 | 少误报、少无意义跨文件读取 |
| Service/Mapper | 至少一次 Service + Mapper/XML 修改 | 按证据搜索必要 Controller 和 SQL |
| 状态流转/幂等/资金敏感 | 至少一例有真实高风险业务路径 | 不为缩短耗时遗漏高风险论证 |

额外复测历史 **17 个小改动文件**规模；记录端到端耗时、P50/P90、实际有效文件首轮覆盖、`read/grep/glob/shell` 调用次数、重复无效检索、已确认 Finding/误报/漏报/未解析路径、Skill 实际加载与权限拒绝轨迹、Git 前后状态以及外网连接检测结果。

**性能目标仍为 P50 ≤ 3 分钟、P90 ≤ 5 分钟（在同一模型、同一机器、同一并发条件下测量）**；未达标必须真实写 FAIL 或列改进项，不能暗中删除高风险审查步骤凑时长。T3 的通过前提包括：T1/T2 全部通过、实际内网没有未经批准的外联或源码写入、真实模型没有“假成功”。缺失线上内网证据不能用公开模拟模型或 CI 绿色结果替代。

## 9. 风险、边界与后续

主要风险：原生 Agent 更快但严重缺陷漏报、路径排除失准、上下文搜索发散、内网模型工具调用不稳定、Java 多态/反射链不完整。采用 fixture 正反例、范围清单、软搜索预算及“部分完成”状态治理；真正需要硬约束时另立需求，优先利用 OpenCode 官方权限/工具机制，不复建大系统。

**本次交付是待评审的设计文档，不包含可运行 Skill，也不代表新技术方案已通过研发验收。** 用户确认设计后，再按照 Superpowers writing-plans 规范输出到 docs/superpowers/plans/ 的逐任务实现计划，之后才进入产品开发。

## 10. 本轮审核问题修订映射及官方依据

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
