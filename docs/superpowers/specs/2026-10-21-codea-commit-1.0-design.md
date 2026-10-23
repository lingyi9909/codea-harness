# Codea Commit 1.0 设计方案

> 2026-10-23 交互契约修订：用户新方案要求真实暂存 Diff 与完整说明预览后，等待本轮明确确认。以 [当前 Skill](../../../codea-commit/compatibility/v2.0.27/payload/skills/codea-commit/SKILL.md) 和 [实施/验收记录](../plans/2026-10-23-codea-commit-preview-confirmation.md) 为准；历史安装路径亦须先满足 v2.0.27 参数安全门禁。

> **2026-10-22 研发补充：** 已提供 [现有 OpenCode 原生拦截补丁](../../../codea-commit/host-patches/README.md) 与真实源码/HTTP/CLI 回归，不新增 Runtime/插件/Agent。公司维护方应用或适配并验收前，以下暂停安装门槛继续有效。

> **2026-10-21 审核更正：BLOCKED，暂停安装与分发。** 原设计遗漏了 OpenCode 在模型前展开参数并执行 Shell 的边界，提示词拒绝非空参数不足以阻止执行；没有占位符也可能 fallback 追加参数，Skill 也可能自动注册为命令。以下原方案暂不能用于安装/发布。必须先完成 [宿主原生前置校验及 S01—S08 验收](../../examples/codea-commit-command-security.md)，再恢复 Command/Skill 和后续功能验收。不新增 Runtime/MCP/插件/Agent，不改 Reviewer/模型/Review 流程。
日期：2026-10-21
交付性质：供研发评审和实施的设计稿，尚未开发或安装。

## 1. 目标与现有背景
开发者在 OpenCode 输入一次 /codea-commit，由当前公司私有模型读取真实改动，生成清楚的中文提交说明，并完成一次本地 Git 提交。正常路径不要求开发者再粘贴长提示词，也不重复确认已经明确的提交范围。

沿用现有职责：
- OpenCode：模型、工具执行、权限交互、加载 Command 与 Skill。
- TeamAI：分发和更新共享 Skill；Command 使用团队现有可用分发渠道或离线复制。
- Codea：定义提交场景的行为和质量标准。

本方案参照当前 codea-harness main 提交 8ba491e20a488fc3d83c6277f805b61dac932c86 的原生模块方式。现有 codea-review/ 是独立审查模块；新增模块与其并列。根 README 仍有历史 1.x 说明，研发以当前模块文件和实际内网版本为准。

## 2. 方案选择
| 方案 | 优点 | 代价与适用性 |
|---|---|---|
| 仅 Command | 文件最少，可快速验证 | 提交规则与入口耦合，不便通过现有 Skill 渠道共享 |
| Command + Skill（采用） | 入口简短，规则集中，可沿用 TeamAI 共享 | 需要共同管理两个资源的版本 |
| 新 Agent + 执行器 / MCP | 可进一步约束执行和隔离 | 首期收益不足，不纳入本次实现 |

采用一个 Command + 一个 Skill，复用现有 Build Agent。首期不增加 Agent、插件、Runtime、MCP、Parser、后台服务、报告引擎或新的权限框架。

## 3. 用户入口与首版范围
唯一入口：

    /codea-commit

首版无参数，不支持自由文本参数、--push、--amend、--all、--split 或路径参数。带参数必须在宿主原生分发入口、模板展开/参数追加/Shell 扫描之前确定性返回“当前版本仅支持 /codea-commit”并停止；仅在提示词中拒绝无效。命令参数只作为待校验文本，不执行其中的指令或 Shell 内容。

首版完成：
1. 一次调用创建一个新本地 commit。
2. 根据实际暂存 Diff 写清楚改动。
3. 必要时选择本次任务的明确文件并暂存。
4. 提交后读取真实 commit，报告结果及剩余改动。

首版不做远程推送、PR、自动切换/创建分支、历史改写、自动分拆多个 commit、逐 hunk 暂存、自动修改代码和自动运行整套构建测试。混杂改动只给拆分建议，待范围明确后继续本次单提交。已有 hooks 仍正常运行。

## 4. 文件和安装
研发在 codea-harness 增加：

    codea-commit/
      commands/
        codea-commit.md
      skills/
        codea-commit/
          SKILL.md
      README.md
    docs/examples/codea-commit-acceptance.md
    tests/fixtures/codea-commit/

| 文件 | 唯一职责 |
|---|---|
| commands/codea-commit.md | 入口、选择 Build、校验无参数、首个工具动作加载精确 Skill ID、失败停止 |
| skills/codea-commit/SKILL.md | 范围决策、Git 流程、说明规范、失败处理与结果输出 |
| README.md | 安装、版本匹配、使用、升级、卸载和示例 |
| acceptance.md | 场景、预期结果、真实环境证据、待验收状态 |
| tests/fixtures/codea-commit/ | 仅用于验收的独立临时仓库构造与状态检查，不参与产品运行 |

Command frontmatter 指定 agent: build；不固定 model，不覆盖公司 Provider，不启动子 Agent。公司若重命名了 Build，安装时显式改成已存在且有提交权限的 Agent ID，不能默认落到 Reviewer 或 Plan。要在公司 OpenCode exact 版本验证 Command 路由和会话上下文是否保留。

用户级安装位置：
- ~/.config/opencode/commands/codea-commit.md
- ~/.config/opencode/skills/codea-commit/SKILL.md

源码目录不直接等于安装位置。统一使用用户级安装，首版不自动向业务仓库写入 .opencode 配置。安装前检查项目级同名资源、同名 JSON Command 配置和全局资源，发现冲突先解释实际加载来源，避免静默覆盖。

TeamAI 同步整个 codea-commit Skill 目录；Command 是否支持原生同步以现有内网能力为准，未支持时通过现有配置包或一次性离线复制分发，不要求修改 TeamAI CLI。README 必须分别列出 Command 和 Skill 的来源版本或 SHA256，不能以“Skill 同步成功”代替“入口已安装”。

升级时备份两个资源，只替换本模块拥有的资源；回退恢复备份。卸载只删除本模块的 Command 和 Skill，不删整个 OpenCode 配置目录。

## 5. 提交流程
流程为：
加载 Skill → 验证环境 → 确定范围 → 阅读完整暂存改动 → 生成说明与完整 Diff 预览 → 等待明确确认 → 提交前复核 → 本地 commit → 核验并输出。

### 5.1 加载与环境
- Command 第一个工具动作必须是原生 skill(codea-commit)。缺失、拒绝或加载失败即停止，不能降级为普通聊天自行提交。
- 定位当前 Git worktree 根目录，所有后续 Git 操作绑定该根目录。支持 linked worktree，不能把 .git 永远当成目录。
- 确认当前分支；detached HEAD 首版停止，告知用户自行选定分支后重试。不自动创建分支。
- 检查冲突和正在进行的 merge、rebase、cherry-pick、revert 等未完成操作，有则停止，保留原状态。
- 无历史的新仓库允许首次提交。HEAD 不存在时不能把正常的新仓库误判为损坏。
- 读取仓库规则和最近最多 30 条非 merge 提交标题。没有历史时采用本模块默认格式。
- 缺少 Git、写权限、身份或签名能力时停止并说明原因，不代改 git config 或签名设置。
- main/master 是否允许提交由项目已有规则决定，不额外发明必须建分支的流程。

### 5.2 范围选择规则（核心契约）
| 状态 | 行为 | 是否需要范围询问（所有有效预览均需提交确认） |
|---|---|---|
| 已有一个意图明确的暂存集 | 提交全部已暂存内容，不新增文件，不扩大到工作区版本 | 否 |
| 已暂存内容含多个无关需求，或与用户明确指定任务冲突 | 展示文件和拆分建议，保持 index | 是，只问本次提交哪一组 |
| 没有暂存，但本会话有清楚的任务及完整改动范围 | 检查实际 Diff 和新文件，明确列出路径，再仅暂存该组文件 | 否 |
| 没有暂存，且缺少可靠任务上下文或混有其他工作 | 列出候选文件与原因，等待用户选范围 | 是 |
| 没有实际改动 | 输出“没有可提交改动” | 否，结束 |

补充规则：
- “模型碰过某个文件”不代表该文件全部内容都属于本任务。发现同文件混有其他工作，先询问；首版不自动逐 hunk 处理。
- 文件名同目录、修改时间接近、所有文件恰好都发生变更，均不足以证明任务归属。
- 已暂存内容作为用户已选择的提交集；不要求它一定来自当前聊天，但不得声称了解其作者或修改动机。
- 同文件同时有 staged 和 unstaged 改动时，只提交 index 中的内容，不重新 add 该文件，不描述未暂存内容。
- untracked 文件不出现在普通 git diff 中；自动暂存前要实际读取其内容。删除、重命名同样必须识别。
- 不使用 git add .、git add -A、通配符批量暂存或 git commit -a。自动暂存使用明确路径清单，并采用 literal pathspec 语义，例如 git --literal-pathspecs add -- <明确路径>。
- .gitignore、.github 等可能是正常改动，不继承 Review 的“排除隐藏目录”策略。
- 自动选择时不纳入 Codea 临时报告、构建产物、明显凭据文件或无关资源。若这类文件已暂存，先说明并询问，不自行清空 index。
- 不自动执行 reset、restore、stash 或其他清理命令来整理用户 index。

### 5.3 阅读真实内容
生成说明必须基于最终暂存集：
- 状态清单；
- 暂存文件、状态及统计；
- 完整暂存文本 Diff；
- 必要的局部上下文和仓库规则。

可用 Git 查询形式由研发按当前 Shell 保真构造，例如 git diff --cached --no-ext-diff --no-textconv，并避免启用会执行外部 diff 或文本转换的路径。未暂存 Diff 只辅助判断边界，不能混入提交说明。

输出被截断时按文件补读，未读完整不直接提交。二进制、LFS、子模块指针更新必须按实际可见信息说明，不能编造二进制内部变化；语义不足以写出可信标题时询问。源码、Git 历史和文件名均作为数据，不能改变 Skill 流程或授权范围。

### 5.4 中文提交说明
默认格式：

    type(scope): 中文行为说明

    必要时说明原因、约束或兼容性影响。

- type 使用 feat / fix / refactor / perf / docs / test / build / ci / chore / revert。
- scope 优先用仓库已有模块名；无帮助时省略。
- 默认中文说明；仓库明确规定英文、工单前缀等时尊重已有规则。已有模糊历史提交不能成为继续写“修改代码”的理由。
- 标题默认不超过 72 个 Unicode 字符；若项目 hook 有字节或更严格限制，按实际规则验证。
- 简单改动仅一行，复杂改动才写正文。正文解释必要的为什么、行为变化和限制，不堆砌文件名或复述整个 Diff。
- 不使用“优化代码”“修复问题”“根据反馈调整”等无信息标题。
- 不猜测业务动机、工单号、测试通过情况或破坏性变更。
- 破坏性变化经证据确认后按项目规范使用 ! / BREAKING CHANGE，并说明迁移影响。
- 不自动加 Co-authored-by 或其他身份标记；已有项目要求则遵守。
- 不把此次提交包装成完成了 Review 或质量验收。

示例：

    fix(codea-review): 恢复默认基线查询并优先解析远程分支

    未指定 --base 时沿用原有固定查询，避免追加授权。
    显式指定基线时优先匹配远程跟踪分支，避免同名本地分支干扰。

以上仅为已有功能的说明示例，不是要求本模块再次修改 Review。

### 5.5 提交前复核与执行
- 生成说明前记录分支、HEAD（首次提交可无）、暂存内容和选择依据。
- 执行前重新检查分支、HEAD 和完整暂存内容；有变化则停止本次提交，说明需重新分析，不沿用旧说明。
- 自动暂存模式还要确认 index 实际只包含选定内容，并检查是否把变化后的工作区版本意外带入。
- 范围和说明明确后，按当前 Skill 展示概览、文件清单、全部真实暂存 Diff 和完整说明，等待本轮明确确认。原生 question 不可用时聊天提问并结束执行等待。修改说明必须重新展示全文和确认；状态改变需完整重新预览。调用本身不授权 commit，原生权限询问另按原策略生效。取消保留原有和新准备的暂存。
- 提交命令只提交当前 index；不用 -a，不带文件路径，不用 --only、--include 或 --amend。
- 推荐将完整消息写入本次唯一的系统临时 UTF-8 无 BOM 文件，再执行 git commit -F <消息文件>。不把正文拼成可执行 Shell，不向业务目录写消息文件。
- 文件路径按实际 Shell 正确引用；中文、空格、引号、美元符号、反引号和换行不能改变命令语义。无法保真处理的路径先停止，不能靠加宽权限解决。
- 保留 hooks 与签名，不用 --no-verify、--no-gpg-sign、--allow-empty 或改写配置。
- 本 Skill 不主动构建、跑测试或修复 hook 失败。既有 hooks 可能执行测试或格式化，应如实区分。
- index.lock、权限、身份、签名、hook 失败时保留现场，报告原因和当前状态；不删锁、不自动修代码或反复重试。
- 只清理本次创建的消息临时文件；不得清理用户文件。

### 5.6 提交后核验与输出
不能只根据 Shell 返回成功文本宣称完成。必须重新读取：
1. 新 HEAD、父提交和实际 commit message；
2. 新提交文件及内容，与提交前阅读的暂存集比较；
3. 当前分支、index 和剩余工作区状态。

正常提交应为本次创建的一个新 commit，父提交是原 HEAD；首次提交则没有父提交。最终显示实际 commit message，即使 commit-msg hook 改写了它。

若 hook 改写了提交内容、分支或提交关系，或者执行返回失败但 HEAD 已改变，明确报告“已产生提交，需复核”及实际 SHA 和差异；不能误报“未提交”，不能自动 reset 或 amend。hook 只改工作区而未进入 commit 的内容，作为剩余改动列出。

正常输出示意：

    已提交：a1b2c3d
    分支：feature/example

    fix(order): 避免重复回调导致订单状态回退

    本次提交：3 个文件
    剩余改动：2 个未暂存文件（列出路径）
    测试：本命令未主动运行；hook 结果按实际说明
    远程：未执行 push

失败输出明确处于“未创建提交”“等待范围选择”或“已产生提交但需复核”，并给最短下一步，不生成业务目录报告。

## 6. 权限和能力边界
- Build 沿用公司已有身份、Provider 和权限，不安装新的 Git 权限包装器，不修改 Reviewer、默认 Agent、默认模型或组织策略。
- Skill 中允许的操作不等于系统已经授权；被组织策略拒绝时停止并说明。
- 不新增 git * 自动 allow；不为减少弹窗全局放开 Shell。首期是否弹原生权限框以实际内网版本为准，不能承诺完全零弹窗。
- Skill 是模型行为约定，不是强制执行器。两次检查可降低错提风险，无法对其他进程并发修改 index 与 commit 之间的竞争提供原子保证。
- 本模块不主动联网；已有 hooks 的外部行为由仓库和公司环境管理，不能声称本 Command 可强制禁止 hook 联网。
- 既有 hook、filter、签名程序由用户信任的开发环境承担。未来若需要强制隔离，应另立执行环境项目。

## 7. 验收标准
### 功能场景
| 编号 | 场景 | 必须观察到的结果 |
|---|---|---|
| C01 | 单一需求已有暂存 | 完整预览并获本轮明确确认后一个新 commit；说明准确 |
| C02 | 同文件部分暂存，其余未暂存 | commit 只含 staged hunk；未暂存内容保持 |
| C03 | 无暂存，本会话任务明确，包含新增/删除文件 | 仅明确任务路径被暂存和提交 |
| C04 | 无暂存，无可靠上下文或多个任务 | 提问范围；提交前 HEAD/index 不变 |
| C05 | staged 混有无关需求 | 给拆分建议，不自行取消暂存或自动连做多提交 |
| C06 | 工作区干净 | 不创建空提交，不报故障 |
| C07 | 新仓库首次提交 | 已选暂存内容能创建根提交，无需虚构 HEAD |
| C08 | 中文/空格/引号/美元符号/前导短横线路径和消息 | 内容保真，不发生 Shell 或 pathspec 意外执行；无法支持的异常路径明确停止 |
| C09 | hook 拒绝、身份缺失、签名失败、index 锁 | 如实失败，不跳过保护，不擅改配置或删除锁 |
| C10 | hook 改写 index、message 或执行后 HEAD 变化 | 读取实际结果；有提交时返回真实 SHA，内容不符提示需复核 |
| C11 | detached、未完成 merge/rebase/cherry-pick/revert、冲突 | 停止，保留现场，不自动整理历史 |
| C12 | 提交前 HEAD/分支/index 被其他进程修改 | 不使用旧 Diff 和消息继续提交 |
| C13 | 非 Git 目录、Skill 缺失、工具拒绝、非空参数 | 明确停止，不进入自行提交的降级流程 |
| C14 | .gitignore/.github 正常改动，Codea 临时报告混入 | 正常配置不被隐藏过滤；临时报告不被自动捎带 |
| C15 | Diff 截断、二进制/LFS、linked worktree | 补读或说明限制，不编造语义；正确识别 worktree Git 路径 |
| C16 | 重复调用 | 上次内容不重复提交；只处理当前实际待提交状态 |

### 提交说明质量
使用至少 10 组团队真实脱敏改动，覆盖 Java、SQL、配置、文档和测试。由研发逐条评判：
- 类型及模块是否正确；
- 是否说清实际行为变化；
- 复杂改动是否有必要且准确的原因说明；
- 是否只描述本次 commit；
- 是否出现虚构工单、业务原因或测试结果。

不得用仅检验文件存在、关键字齐全或模拟模型的测试宣称语义质量已通过。合成 fixture 验证 Git 边界，真实模型验证是否遵守行为和说明是否有用。

### 发布门槛
- C01—C16 全部有实际结果，不能用“应该可用”代替。
- C01—C03 正常路径不出现额外的业务确认；原生权限框单独记录。
- 不得出现无关代码混入、部分暂存被扩大、说明编造未暂存功能或失败后改写历史。
- 公司 Windows + 实际 OpenCode exact 版 + 私有模型 + 真实 TeamAI 分发链路完成验收。未做的项标记 PENDING。
- 关键正常路径至少重复 3 次；明显越界或误报成功必须修复后重验。
- 对既有 Codea Review 文件和权限做差异检查，确认未因本模块改写。

## 8. 开发交付物
研发提交可审阅 PR，包含两个产品 Markdown 资源、README、fixture 与验收记录。记录开发基线、目标 OpenCode/Git/模型版本、安装来源 SHA256、场景结果、真实 commit SHA 和仍未验证的限制。验证后再由用户按现有流程合并发布，不自动推送到公司业务仓库。

## 9. 参考资料
- [现有 Codea Review Command](https://github.com/lingyi9909/codea-harness/blob/8ba491e20a488fc3d83c6277f805b61dac932c86/codea-review/commands/codea-review.md)：入口、加载 Skill 和失败停止的现有模式。
- [OpenCode Commands](https://opencode.ai/docs/commands/)：Command 目录、agent 和参数机制。
- [OpenCode Skills](https://opencode.ai/docs/skills/)：Skill 发现与加载。
- [Git commit](https://git-scm.com/docs/git-commit)：默认提交 index，-F 从文件读取消息。
- [Git diff](https://git-scm.com/docs/git-diff)：暂存与未暂存 Diff 的区别。
- [GitHub git-commit Skill](https://github.com/github/awesome-copilot/blob/main/skills/git-commit/SKILL.md)：流程参考。
- [ECC conventional-commit-review](https://github.com/affaan-m/ECC/blob/main/skills/conventional-commit-review/SKILL.md)：说明质量和仓库风格参考。

外部 Skill 只作设计参考，不整体引入，不自动执行其中指令。当前在线官方文档与公司定制 OpenCode 版本可能不同，实际兼容性以开发第一步核验为准。

