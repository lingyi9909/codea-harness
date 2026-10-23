# Codea Commit 1.0 开发流程与研发任务书

> 2026-10-23：预览确认交互由 [修订实施记录](2026-10-23-codea-commit-preview-confirmation.md) 与当前 v2.0.27 Skill 替代；本文保留原开发任务背景，不恢复历史 blocked 入口。

> **2026-10-22 研发补充：** 已提供 [现有 OpenCode 原生拦截补丁](../../../codea-commit/host-patches/README.md) 与真实源码/HTTP/CLI 回归，不新增 Runtime/插件/Agent。公司维护方应用或适配并验收前，以下暂停安装门槛继续有效。

> **2026-10-21 审核更正：BLOCKED，暂停安装与分发。** 原设计遗漏了 OpenCode 在模型前展开参数并执行 Shell 的边界，提示词拒绝非空参数不足以阻止执行；没有占位符也可能 fallback 追加参数，Skill 也可能自动注册为命令。以下原方案暂不能用于安装/发布。必须先完成 [宿主原生前置校验及 S01—S08 验收](../../examples/codea-commit-command-security.md)，再恢复 Command/Skill 和后续功能验收。不新增 Runtime/MCP/插件/Agent，不改 Reviewer/模型/Review 流程。

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
> 本次由用户交给研发团队实施；上面的执行技能说明仅适用于采用对应技能的 AI 研发代理，不要求人类研发安装技能。

**Goal:** 开发者输入 /codea-commit 后，在明确范围内完成一次本地提交，并得到准确中文说明和真实结果。

**Architecture:** 一个 Command 加一个 Skill，Command 路由现有 Build 并显式加载 Skill。所有范围选择、说明和核验规则集中在 Skill，Git 仍由 OpenCode 原生工具执行，TeamAI 沿用现有共享渠道。

**Tech Stack:** OpenCode 原生 Markdown Command / Skill、现有私有模型 Provider、Git；Python 标准库仅用于验收 fixture，不成为产品依赖。

**Spec:** [Codea Commit 1.0 设计方案](../specs/2026-10-21-codea-commit-1.0-design.md)。实施时先读设计稿，本文件不取代设计中的范围规则和 C01—C16 验收矩阵。

## Global Constraints
- 唯一入口 /codea-commit，首版无参数，一次调用最多一个新本地 commit。
- 仅两个产品资源：codea-commit/commands/codea-commit.md、codea-commit/skills/codea-commit/SKILL.md。
- 复用现有 Build，不新建 Agent / Parser / Runtime / MCP / 插件 / 后台服务。
- 不改现有 codea-review、Reviewer 权限、默认 Agent、默认模型、公司 Provider 或 TeamAI CLI。
- 已有暂存优先；部分暂存不得被工作区版本扩大；自动 add 只使用明确且属于本次任务的路径。
- 不 push、不 amend、不切换分支，不自动 reset / restore / stash，不执行空提交。
- 保留 hooks 和签名；原生工具询问按组织策略处理，不新增宽泛 Shell/Git allow。
- 中文说明来自完整暂存内容；复杂改动的原因必须有依据，不能编造测试或工单。
- 工具脚本仅用于测试；不能把执行器藏在 fixtures 中成为产品依赖。
- 所有测试在新建临时仓库进行，不在真实公司业务仓库做破坏性验收。

## Review Focus
1. 同文件同时暂存与未暂存：只提交 index 中的部分，说明不描述剩余部分。
2. Command 切到 Build 后可能缺少会话上下文：无法可靠认领文件时询问范围，不猜测。
3. hooks 修改消息/内容，或者失败返回但 HEAD 已变化：如实报告已经产生的 commit，不自动回滚。
4. Windows 中文、空格、引号、美元符号、前导短横线路径：Shell 与 pathspec 保真，不能被解释成指令。
5. TeamAI 只同步 Skill、入口残留旧版或被同名项目配置覆盖：核验实际来源，不能只检查文件存在。

这五项分别由任务 2、任务 1/3、任务 3、任务 2/3、任务 4 的测试和真机检查覆盖。

## Task 1：确认内网兼容性并冻结首版行为
**Files**
- Create: docs/examples/codea-commit-acceptance.md
- Read only: codea-review/commands/codea-review.md、codea-review/README.md
- Read only: 当前公司 OpenCode 和 TeamAI 使用说明

**Interfaces**
- Consumes：设计稿、当前 codea-harness 主线和真实公司环境。
- Produces：验收表中的版本信息、Build Agent ID、Command 路由结果、Skill 安装/加载来源、分发方式和权限现状。

- [ ] 记录研发起点 SHA；参考设计的 main 为 8ba491e20a488fc3d83c6277f805b61dac932c86，若主线已更新，重新查看相关文件再开始。
- [ ] **先完成 S01—S08 原生参数前置校验门槛**：核对 actual exact 版参数替换/fallback/Shell/模型的执行顺序，在所有可到达命令入口拒绝非空参数；无可靠能力时保持 BLOCKED，不恢复 Command/Skill 分发。
- [ ] 记录 Windows、OpenCode exact 版本、Git、实际 Shell、私有模型标识及 TeamAI 版本。不能仅记录“OpenCode 2.x”。
- [ ] 核验现有 Build ID 是否为 build，原生 skill 工具是否可用；Command 指定 Build 后，是否保留足够的会话任务上下文。结论只影响无暂存场景的范围判断，不允许靠猜测补上下文。
- [ ] 核对只读 Git、明确路径 add、commit、系统临时文件写入在现有策略下的行为，记录是否有原生权限框。不以本功能为理由全局放开 Shell。
- [ ] 核对 TeamAI 能同步哪些资源，确定 Command 的已有分发渠道；记录用户级与项目级同名资源如何检查。
- [ ] 将 C01—C16 和质量检查逐条列入验收表，初始全部 PENDING；列出真实测试负责人和证据栏。
- [ ] 若 exact 版本不支持所需 Command 或 Skill 能力，提交具体差异和最小适配建议，不开始重造执行框架。

**完成条件：**研发能说明目标环境、两个资源实际如何加载、原生权限行为和上下文限制。

## Task 2：建立能验证提交边界的测试数据
**Files**
- Create: tests/fixtures/codea-commit/setup.py
- Create: tests/fixtures/codea-commit/verify.py
- Create: tests/fixtures/codea-commit/README.md

**Interfaces**
- setup.py：输入场景 ID 与一个新的空临时目录，创建独立仓库及预期改动；输出仓库路径和仓库外的验收清单。
- verify.py：输入同一清单与实际仓库，读取真实 Git 状态，按场景检查 HEAD、父提交、已提交内容、index 和剩余文件；报告具体不一致项。
- 这两份脚本不读取/执行 Skill，不调用模型、不模拟模型，不出现在用户安装包中。

- [ ] 先给 verify.py 定义断言：哪些文件/hunk 应提交、哪些必须保留、预期是否产生 commit。不能只检查命令退出码和文件数量。
- [ ] 构造 C02：同一文件 A hunk 已暂存、B hunk 未暂存，另有无关文件；在未执行提交前运行验证，应因缺少预期 commit 失败。
- [ ] 用人工已知正确的普通 Git 提交建立对照，确认验证器能通过；另做一个故意扩大暂存的错误对照，确认能指出 B hunk 被误提。这样验证测试有能力发现核心错误。
- [ ] 扩展到新增/删除/重命名、首次提交、混杂范围、干净仓库、特殊字符路径、linked worktree、二进制改动等数据。未支持的异常文件名要有清晰停止预期。
- [ ] 为 C09/C10 准备可控测试 hooks：拒绝提交、改写消息、改写暂存内容。仅安装到测试仓库，不改全局 hooksPath、身份或真实用户配置。
- [ ] 为 C11/C12 准备未完成操作与提交前状态变化的复现步骤。不要依靠真实网络或真实业务仓库。
- [ ] 验证 setup.py 拒绝覆盖非空目录；脚本使用 Python subprocess 参数数组，不用 shell=True 执行文件名。
- [ ] 在 Windows 运行一次 fixture 自检，确认换行、路径和 Git 输出处理保真。

**完成条件：**关键正常/错误对照均符合预期，fixture 能证明实际 Git 结果对错；不把 fixture 自检当成产品已通过。

## Task 3：实现唯一入口和完整提交 Skill
**Files**
- Create: codea-commit/commands/codea-commit.md
- Create: codea-commit/skills/codea-commit/SKILL.md
- Modify: docs/examples/codea-commit-acceptance.md

**Interfaces**
- Command 输入：原始参数文本，仅空参数有效。
- Command 输出：无参数时由既有 Build 显式加载精确 ID codea-commit；其他输入返回正确用法并停止。
- Skill 输入：当前 worktree、真实 index、必要的会话与项目规则。
- Skill 输出：已提交 / 无改动 / 等待范围选择 / 未提交失败 / 已产生提交需复核；各结果提供真实分支、SHA（如有）和剩余状态。

- [ ] 先在临时安装位置运行缺失 Command / 缺失 Skill 的场景，记录当前无法完成预期的结果；不修改真实业务项目。
- [ ] 编写 Command：description、agent: build、无参数校验、首个工具动作 skill(codea-commit)、加载失败停止。不嵌入自动执行 Shell 的预填语法，不固定 model。
- [ ] 编写 Skill 的环境与范围章节，完整落实设计 5.1/5.2；重点处理 staged 优先、无可靠上下文询问、同文件部分暂存和 untracked 正文。
- [ ] 用 C01—C07、C13 运行真实 OpenCode；每次用验证器检查真实 Git 结果。发现错误只修对应提示或配置，不复制出新的运行时。
- [ ] 编写说明章节：type/scope/中文标题/必要正文、项目规范优先、不得描述未暂存变化或虚构测试。
- [ ] 编写暂存和提交章节：明确 literal 路径、完整读取 Diff、提交前再次核对、UTF-8 临时消息文件、git commit -F、不带路径或 -a。
- [ ] 编写失败和事后核验章节：保留 hooks/签名、识别 HEAD 实际变化、核验真实消息与内容、不自动回滚或修代码。
- [ ] 执行 C08—C12、C14—C16；逐项保存工具轨迹和实际 Git 结果。C12 在读取 Diff 与准备提交之间由测试人员修改状态，确认不会拿旧内容提交。
- [ ] 在 Build 上核对正常场景先展示完整预览并等待一次明确提交确认；范围不明时先询问缺少的信息，不能把选择范围当作提交确认。
- [ ] 对两个产品资源做精简检查：Command 不复制 Skill 全文，Skill 不增加不必要参数、报告模板或额外依赖。
- [ ] 用人工选择的真实脱敏 Diff 检查标题和正文质量；即使 Git 边界通过，说明不准确仍须修订再验。

**完成条件：**C01—C16 在目标 OpenCode 中有实际结果，核心错误均修复；原生权限框与业务确认分开记录。

## Task 4：文档、分发与回退
**Files**
- Create: codea-commit/README.md
- Modify: docs/examples/codea-commit-acceptance.md
- 若项目有统一模块目录索引，仅增加 codea-commit 链接；不重写历史 README 或 Review 文档。

**Interfaces**
- 输入：任务 1 确认的实际安装和分发能力。
- 输出：可重复的用户级安装、升级、来源核对、卸载与回退步骤。

- [ ] README 写清目录映射、安装前同名冲突检查、完整文件清单、唯一入口、范围规则、输出示例和限制。
- [ ] 写出两个资源如何分别分发和核验 SHA256。Command 没被 TeamAI 同步时明确采用已核实的离线配置分发方法。
- [ ] 在独立验收用户配置目录完成“首次安装 → 重启/按 exact 版刷新 → 调用命令 → 核对 Skill 来源”。
- [ ] 构造旧版 Command 与新版 Skill、项目级同名资源等 C13/Review Focus 5 场景，确认冲突被识别，不能误以为文件同步成功就是运行新版。
- [ ] 实测备份后升级、回退及卸载，确认其他 OpenCode 资源、业务文件和原 Review 模块未被删除或改写。
- [ ] 用已有 Codea Review fixture 做一次基本回归，确认 /codea-review 的加载入口与权限未受影响。
- [ ] 将每个未执行的内网环节保留为 PENDING，不以静态文件检查替代同步和真机记录。

**完成条件：**研发之外的同事可以只按 README 安装使用；回退可恢复原有行为。

## Task 5：质量验收与 PR 交付
**Files**
- Modify: docs/examples/codea-commit-acceptance.md
- PR 正文列出最终范围、验证和限制，不附真实公司代码或凭据。

**Interfaces**
- 输入：真实执行记录、fixture 结果、至少 10 组脱敏真实改动的说明评审。
- 输出：一个可审核的研发 PR 和明确的已验收/待验收清单。

- [ ] 至少 10 组真实脱敏改动覆盖 Java、SQL、配置、文档和测试，评审说明的准确性与可读性；不以关键词匹配代替人工判断。
- [ ] C01—C03 各重复至少 3 次，记录是否多问、是否扩大范围、是否误报成功；原生权限询问单列。
- [ ] 对照设计验收矩阵，逐项附真实环境、结果、工具轨迹/必要截图、提交 SHA 和残留工作区状态。
- [ ] 复核四个禁止出现的问题：无关代码被提交、部分暂存扩大、编造功能或验证、失败后擅自改写历史。
- [ ] 检查差异：除本模块、fixture、文档及必要索引外，无既有 Review、Provider、权限或 TeamAI CLI 改动。
- [ ] 按设计第 8 节准备 PR 描述，明确产品只有 Command + Skill，测试脚本不随用户运行。
- [ ] 完成代码/提示词评审和用户试用后，按团队现有流程合并发布。未完成关键真机验收时保持 Draft，并如实列出 PENDING。

**完成条件：**研发可以用真实证据证明命令完成了正确提交，说明准确，既有流程未受影响。

## 排期建议
按“兼容性确认 → fixture → Command/Skill → 内网联调 → 评审交付”顺序推进。任务 1 完成后由研发估时；实际工作量主要取决于公司 OpenCode 版本、Shell/中文处理和原生权限行为，不把两个 Markdown 文件的编写量当成全部工期。

## 实施边界
本文件是研发交接计划，仅描述待实施工作；功能尚未开发或安装。实施由研发团队按上述任务及设计稿推进。

