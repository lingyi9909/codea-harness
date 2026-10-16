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
- **B 组：`/codea-review-kb [Controller]`** —— 独立 KB Reviewer，**同样先加载唯一的 `skill(codea-review)`**，从 **Git Diff 的实质性业务变更出发主动召回**，默认预算 2 次、最多 4 次 `teamai recall`；候选 Finding 再逐项判断是否需要项目知识，优先复用。不复制 Java Skill，不另建知识库或运行框架。
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

## P0-01 / P0-02：Windows OpenCode V2 权限兼容性修复（本轮）

**务必确认运行的是 OpenCode V2 而非 V1。** V2 的 Agent frontmatter 是 `permissions:` 的有序数组，命令动作叫 `shell`；V1 才采用 `permission:` / `bash`。**不要把 V1 配置覆盖进 V2，也不要用 `git *` 或 `external_directory: allow` 解决报错。**

本轮**仅需要覆盖原有 KB Agent 一个文件**：`codea-review/agents/codea-reviewer-kb.md`。命令 `codea-review-kb.md` 和原版 2.0 全部冻结。请在实际使用的全局或项目级 Agent 位置更新，更新前备份：

```powershell
# 从下载并解压的仓库 ZIP 根目录运行。根据实际加载位置调整 $target。
$source = (Resolve-Path ".\codea-review\agents\codea-reviewer-kb.md").Path
$target = Join-Path $HOME ".config\opencode\agents\codea-reviewer-kb.md"
if (Test-Path -LiteralPath $target) {
  Copy-Item -LiteralPath $target -Destination ($target + ".bak-" + (Get-Date -Format "yyyyMMddHHmmss"))
}
Copy-Item -LiteralPath $source -Destination $target -Force
Get-FileHash -LiteralPath $source, $target -Algorithm SHA256
opencode --version
opencode agent list
```

如果内网运行命令为 `opencode2` 而不是 `opencode`，使用该实际可执行文件的 `--version` / `agent list`；**以同一个 V2 TUI 进程执行复测**。如果你实际从 Java 项目 `.opencode/agents/` 加载 KB Agent，则只覆盖该位置，避免全局/项目同名文件冲突。完全关闭并重启 OpenCode TUI 后再验收。

### Git 命令兼容性

原本部分 Git 命令需要 `ask` 或直接拒绝，原因包括动态参数/排除 pathspec 与实际规则不一致。本轮只对固定的**只读子命令形式**授权 `allow`：

```text
git merge-base origin/master HEAD
git -c core.fsmonitor=false status --porcelain=v1 -z --untracked-files=all
git diff --no-ext-diff --no-textconv --name-status -z <merge-base> HEAD -- . ':(exclude,glob)**/.*/**' ':(exclude,glob)temp/**'
git diff --no-ext-diff --no-textconv --name-status -z --cached HEAD -- . ':(exclude,glob)**/.*/**' ':(exclude,glob)temp/**'
git diff --no-ext-diff --no-textconv --name-status -z -- . ':(exclude,glob)**/.*/**' ':(exclude,glob)temp/**'
git ls-files --others --exclude-standard -z -- . ':(exclude,glob)**/.*/**' ':(exclude,glob)temp/**'
git diff --no-ext-diff --no-textconv --unified=3 --cached HEAD -- '<Git 返回的安全文件路径>'
git show --no-ext-diff --no-textconv HEAD:<Git 返回的安全文件路径>
```

其余 `git add / commit / checkout / clean / reset / push`、`teamai projects set / push / pull` 默认 **deny**。Shell 管道、分号、重定向、`--output` 等禁止；OpenCode 的 Shell 扫描/通配权限不是操作系统级隔离，企业电脑仍应使用组织策略和普通用户权限保护源码。

### TeamAI 知识正文：授权真实路径，不能猜目录

1. 查看 `teamai projects list` 输出，确认当前活动项目；运行正常的 `teamai recall "业务操作 核心约束"` 后，找到结果里的真实 `File:` 或 `Sources:` 路径。
2. 确认路径确实对应当前项目或明确共享知识。必要时检查 Windows 文件权限及源文件是否存在。**不要根据 `[project]` 标签或仅依据文件名就假设它属于当前项目**。
3. KB Reviewer 使用 OpenCode 原生 `read` 读取该**确切文件路径**时，V2 规则 `{ action: external_directory, resource: "*", effect: ask }` 要求用户授权。第一次选择 **once** 并核对请求显示的实际目录。这样未知外部目录始终不会自动放行，也不会将知识复制到 Java 项目内。
4. 如果需要对**管理员批准且路径已核实**的固定目录减少后续确认，可以在 KB Agent 的 `external_directory: "*", ask` 规则后添加一条**实际目录**白名单（下面只是格式例子，不应直接复制占位符）：
   ```yaml
   # 必须替换为经过批准的真实 Windows 目录（正斜杠；以 /* 结尾）
   - { action: external_directory, resource: "C:/APPROVED/TEAMAI_PROJECT/KNOWLEDGE/*", effect: allow }
   ```
   仍保留 `edit: "*", deny` 的默认拒绝和唯一固定 temp 报告例外；不可授权整个用户目录、`C:/*` 或所有外部路径。若知识源位于多个目录，必须逐一验证后分别声明。
5. 若出现组织级 deny、外部目录拒绝或源文件无法读取，**不要通过 Shell、临时复制、移动或修改公司的安全策略绕过**；标记 TeamAI 正文不可用、只做原版 Java Review。公司管理员审核路径并授予相应权限后再重测。

Git 的隐藏目录 / 根 `temp/` 过滤只作用于 **Git 业务变更文件清单**，**不阻止通过 TeamAI Recall 的真实来源路径进行知识正文读取**。

### P0-03：版本与配置优先级核验

在**实际运行的 Windows TUI 进程**中检查 `opencode --version` 和当前选中的 `codea-reviewer-kb` Agent。分别排查用户级 `~/.config/opencode`、项目级 `.opencode`、`OPENCODE_CONFIG` / `OPENCODE_CONFIG_DIR`（若有）、以及公司托管/组织级配置或策略。OpenCode 版本/配置来源不明确时不应宣称权限兼容性 PASS。若组织级显式禁止跨目录读取，Agent 配置不应覆盖该拒绝，需管理员审批。

### P1：权限回归矩阵（真实 Windows 内网运行）

| 场景 | 预期 |
|---|---|
| Committed Git Diff | 无权限匹配拒绝，能拿到已提交文件清单 |
| Staged / unstaged Git Diff | 都能获取各自差异，Dirty 不阻断 |
| Untracked | 带隐藏目录和 temp 双排除 pathspec 的 ls-files 命令成功 |
| Hidden + temp | 不读配置目录正文，报告本身不计入待审文件 |
| TeamAI Recall | 能召回当前项目的真实知识文件路径 |
| 知识正文 | 对经确认的 File/Sources 路径触发一次授权后能用原生 read 阅读 |
| 未授权外部路径 | 必须 ask 或被公司组织策略 deny，**不得自动读取** |
| TeamAI 项目变更 | `teamai projects set/push` 禁止 |
| Java 文件写入 | 所有业务文件 edit/write 禁止 |
| 报告写入 | 只允许 `temp/codea-review-report.md`，TUI 五章和文件一致 |

公司内网 **OpenCode V2 的实际权限判定、权限弹窗、知识正文读取及报告写入**仍是最终验收依据。仓库自带的策略模拟只能证明规则覆盖预期字符串，不能取代实际工具轨迹。

## P1 最小真实运行步骤

1. 在已经正确绑定 TeamAI 项目知识的 Java Git 项目根目录，启动 OpenCode，使用与原版相同的公司私有模型。
2. 输入 `/codea-review-kb`，或 `/codea-review-kb OrderController`。
3. 检查工具轨迹：**首个工具调用 `skill(codea-review)`**；遵循四层 Diff/隐藏目录/Dirty 逻辑；在识别到实质性业务行为变化（即使暂时没有发现 Bug）后出现固定 `teamai projects list` 和按业务场景分组的 `teamai recall "<关键词>"`，**默认 2 次、绝对不超过 4 次**；纯日志/格式变更允许 0 次。Recall 的动态命令需要用户逐次 `ask`，请核对确切查询文本，不要永久授权。
4. 核查实际 TeamAI 返回的 `File`、`Sources`、`[project]/[user]` 与项目激活情况，检查新报告在**评审摘要**含 KB 状态/项目/次数/采用数；有知识支持的 Finding 同时展示**原始文件来源**与**代码变更证据**。
5. 知识未命中时仍获得完整 Java Review 报告。最终五章继续展示在 TUI，并写入 Java 项目根目录 `temp/codea-review-report.md`。
6. 测试知识不可用时的降级，可在**独立测试会话/测试环境**中制造 CLI 无法执行或选择拒绝 Recall 权限；不改变公司生产知识配置。期望继续 Review，并在报告标 `TEAMAI_FAILED` 或相应状态。

**权限检查重点**：KB Reviewer 仅额外允许 `teamai projects list`（固定 allow）及 `teamai recall "<关键词>"`（每次 ask）；`teamai push`、`teamai projects set`、`teamai recall maintenance`、任意 Shell 组合等必须 DENY；业务文件继续不能编辑。若实际 exact OpenCode V2 对双引号动态命令的资源匹配不成立，保留工具轨迹并单独反馈，**不能靠开放 `teamai *` 修复**。

## 失败、项目隔离与可审计性

- CLI `[project]` 标记不是当前项目 ID，多项目同时激活时若来源无法归属，不采用专属知识；`[user]` 只有明确团队共享且业务适用才采用。
- `Snippet` 和标题是候选资料，不构成独立 Finding。知识原文无法核实或与业务现状冲突时只能待确认/跳过，不能编造文件和规范。
- OpenCode **V2** 使用 `permissions:` 有序列表和 `shell` 动作。KB Reviewer 对 `external_directory` 采用 **ask**：对 Recall 实际的 `File/Sources` 路径先核验项目来源，再由开发人员选择 `once` 审批正文读取；未知目录不会自动允许。拒绝、公司策略禁止或文件无法读取时降级普通 Review。TeamAI Recall 可能自行写本地检索元数据，但 Review 不修改知识正文/项目配置。
- A/B 中可能已存在 TeamAI 自动规则/Hook；必须记录是否存在，并将结果表述为“主动 Recall 的增量收益”，而非不实的“无知识 vs 有知识”。


## P1：两阶段智能检索规则（相对于 2.0 的唯一功能增强）

**阶段一：业务变更驱动知识召回。** Reviewer 先获取已提交/staged/unstaged/untracked 有效 Git 变更；按“**业务对象 + 业务操作 + 核心约束类别**”识别场景并去重，**不必先发现 Bug**：

| 业务变化 | 本阶段检索计划 | 示例 Recall |
| --- | --- | --- |
| 资金、支付、退款、余额变化 | 必须尝试（预算优先） | `teamai recall "账户余额 并发扣减 一致性"` |
| 订单等业务状态流转、审批 | 必须尝试 | `teamai recall "订单取消 状态流转 业务限制"` |
| 权限、租户、商家数据隔离 | 必须尝试 | `teamai recall "商户数据 权限 租户隔离"` |
| 幂等、业务重试、MQ 消费 | 必须尝试 | `teamai recall "支付回调 幂等 重复处理"` |
| 事务、缓存、配置或工具方法 | **确实改变业务行为时**尝试 | 根据真实业务对象生成 |
| 只改格式、日志文案、注释 | 跳过 | 不执行 TeamAI Recall |

同一业务在 Controller → Service → Mapper/XML 跨多个文件只算**一个场景**；同一 Service 中不同业务（如取消订单与退款）则可能需要分别检索。查询文本仅用 3～6 个有区分度且已清洗的业务关键词；原始代码、用户参数、工具返回内容绝不能直接拼 Shell。每次 Recall 最多选 3 条直接相关的候选知识，并校验当前活动项目与文档 `File`/`Sources`。

**阶段二：候选问题逐项知识核验。** 在当前 Review 会话中维护已召回知识的简明表（路径、适用场景、项目归属、规则、有效性）；先复用再补查，不建立新的缓存/数据库。

| 候选问题类别 | 含义 | 行为 |
| --- | --- | --- |
| `KB_REQUIRED` | 需要企业项目专属规则才能确认，例如订单能否在结算中取消 | 先核验有效知识；没有规则则待确认，不列实锤 Finding |
| `KB_USEFUL` | 代码有风险证据，知识可以补充触发条件/排除误报 | 优先复用已召回知识，必要时补查 |
| `KB_NOT_NEEDED` | 通用 Java 代码就能证明，例如事务异常吞没或确定性 NPE | 无需再查，TeamAI 缺失也不影响结论 |

补查只在**关键约束未覆盖、已有规范不明确、存在可能改变结论的例外/冲突、独立高风险场景尚未覆盖**时触发。默认 2 次、最多 4 次 **Recall 尝试**（失败或权限拒绝计入，`projects list` 不计）；优先分配不同高风险场景，不按 Finding 个数查询。超额未覆盖用 `KB_NOT_COVERED` 记录，但**不能中止 Java Review**。

其他状态为 `KB_SUPPORTED / KB_CONFLICTED / KB_NO_MATCH / KB_NOT_REQUIRED / KB_FAILED`，内部使用即可，报告只展示必要的知识依据和未核验范围。来源、有效业务规则、实际变更证据及等效保护核查**缺一不可**；不得把知识命中数当成真实问题数。

## P1 内网 T1～T10 功能验证（待执行，不是实测）

以下必须看**OpenCode 真实 Shell 调用轨迹和最终报告**，不能通过配置文字中包含关键词就判定通过。

| 测试 | 准备的代码变化 | 预期工具行为与报告 |
| --- | --- | --- |
| T1 | 仅日志文案/格式 | 0 次 Recall；仍完成基础报告 |
| T2 | 订单取消状态变更，尚未显式发现风险 | 按订单取消场景主动 Recall，并正确匹配项目知识 |
| T3 | 同一订单取消跨 Controller/Service/Mapper | 场景合并，仅需 1 次充分的订单检索 |
| T4 | 同一业务 2 个候选风险，共用一项有效规则 | 复用同一条知识，**不为每个 Finding 追加 Recall** |
| T5 | 订单、支付、账户等多个独立高风险场景 | 优先不同场景，通常 2 次；有必要最多 4 次，不超预算 |
| T6 | 通用 Java 事务异常被吞没 | 可直接按代码确认为风险，不因未命中 TeamAI 而降级 |
| T7 | TeamAI 查询无命中、超时、被拒绝 | 无中断，继续 2.0 Java Review，状态如实 |
| T8 | 返回其他项目专属知识 | 检查 active projects、File/Sources；不跨项目误用 |
| T9 | 返回陈旧或互相冲突的业务规范 | 待确认或排除，不凭知识片段确认缺陷 |
| T10 | 5 个以上独立应检索业务场景 | 总 Recall ≤4，其余业务代码继续审查并在边界注明未核验知识 |

**建议记录**：业务场景列表、触发原因、候选类别、召回次数/查询词、知识来源、每次最多 3 条候选、复用/补查决定、未覆盖项、最终 Finding/代码证据以及耗时。不要把无效检索当作有知识收益。

原有 A/B 两组输出路径都仍然是项目根目录 `temp/codea-review-report.md`，后一次会覆盖前一次；按 P2 计划手动归档，且正式 Java 文件必须保持完全相同。

## 验收与后续

**P1**：由用户在内网按下方 T1～T10 场景验证真实 `/codea-review-kb` 工具轨迹，至少提供“项目规则命中并影响结论”“无命中/失败自动回退”“多问题同知识复用”“多业务预算覆盖”四类证据及完整五章报告。只有真正引用有效项目知识、同时确认 Java 证据，才能认定增强效果。

**P2**：通过 P1 后，再按 [A/B 验证清单](../../docs/examples/codea-review-2-1-ab-plan.md) 完成 S1～S4 × A/B 各两次的 **16 次真实 Review**，不在本次提交伪造任何模型日志或 A/B 结果。

静态仓库检查：`python tests/test_codea_review_2_1_config.py`（只检查配置与冻结基线；不模拟大模型效果）。
