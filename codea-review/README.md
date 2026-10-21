# Codea Review 2.0 · 基础能力完善（Java / Mapper XML / SQL）

本目录使用原生 OpenCode V2 Agent / Command / Skill，报告模板随 Skill 目录一起安装或经现有 TeamAI 原生 push/pull 同步；不增加 Review Runtime、模板引擎、插件或安装服务。

```text
codea-review/
├── agents/codea-reviewer.md
├── commands/codea-review.md
└── skills/codea-review/
    ├── SKILL.md
    └── references/report-template.md
```

> 状态：用户已确认既有 Codea Review 2.0 可在公司 Windows 内网运行；**本 hardening 分支的 Git 权限调整、SQL-only、真实模型 Finding 回归仍待公司实际版本验收**。GitHub Actions 上的 Windows Git/Python 测试不是私有模型实测。

## Windows 用户级离线安装（不修改 Java 项目）

前提：企业允许的 OpenCode 2.x TUI、Git for Windows ≥ 2.36、公司私有大模型 Provider 已配置，且你拥有目标用户目录的写权限。复制整个仓库或离线压缩包至 Windows 后，在仓库根目录运行以下 PowerShell 命令。不要在企业政策不允许的环境中改变 PowerShell 安全设置。

```powershell
$source = (Resolve-Path ".\codea-review").Path
$homeConfig = Join-Path $HOME ".config\opencode"
$entries = @(
    @("agents\codea-reviewer.md", "agents\codea-reviewer.md"),
    @("commands\codea-review.md", "commands\codea-review.md"),
    @("skills\codea-review", "skills\codea-review")
)
# 首次安装为不覆盖模式；升级前先手动备份已有文件。
foreach ($entry in $entries) {
    $target = Join-Path $homeConfig $entry[1]
    if (Test-Path -LiteralPath $target) { throw "目标已存在，请先备份并检查同名配置：$target" }
}
foreach ($entry in $entries) {
    $from = Join-Path $source $entry[0]
    $to = Join-Path $homeConfig $entry[1]
    New-Item -ItemType Directory -Path (Split-Path -Parent $to) -Force | Out-Null
    Copy-Item -LiteralPath $from -Destination $to -Recurse
    if (Test-Path -LiteralPath $to -PathType Container) {
        Get-ChildItem -LiteralPath $to -Recurse -File | Get-FileHash -Algorithm SHA256 | Format-Table
    } else {
        Get-FileHash -LiteralPath $to -Algorithm SHA256 | Format-Table
    }
}
```

**首次启动前**检查当前 Java 项目及父级目录的 `.opencode/commands/codea-review.md`、`.opencode/agents/codea-reviewer.md`、`.opencode/skills/codea-review` 与兼容 Skill 目录，避免覆盖全局同名 ID。冲突时先停止，不要在被覆盖的 `/codea-review` 内才发现问题。不要将这些配置和 Skill 目录复制到 Java 业务 Git 工作区；不更改 `opencode.json` 的默认 Build/Plan/Provider。

## 使用方式

1. 在 **允许未提交修改的 Git 需求分支**目录启动公司已批准的 OpenCode 2.x TUI，确认当前模型选择的是公司私有 Provider，Tool Calling 正常。
2. 输入 `/codea-review`；或 `/codea-review OrderController`。也可使用下面的显式参数。
3. 观察第一个工具调用实际为 `skill(codea-review)`；确认 Skill 返回并完成参数校验后，Reviewer 获取选定本地基线（默认 `origin/master`，不存在才用 `master`）与 HEAD 的 merge-base，还纳入 staged/unstaged/相关 untracked 工作区变更，按 Git Diff 先快扫后按需查上下文。
4. Agent 中精确配置 `allow` 的固定 Git 查询自动执行；未明确允许的动态 Git 查询继续触发 OpenCode V2 `ask`，开发者核对实际 SHA、业务路径和参数后决定是否授权。如果未触发 `ask` 或查询被拒，检查实际 Agent、项目/全局配置和组织策略；不引入 Git 包装服务或额外权限框架。Reviewer 不执行测试且不修改业务代码；完成审查后自动使用原生 write 创建/覆盖项目根目录的 `temp/codea-review-report.md`（没有 temp 时自动建目录）。
5. Review 完成后通过原生 `read` 读取**实际加载的受信任 Skill 目录**下 `references/report-template.md`，按 Codea Review 2.0 独立模板输出完整五章 Markdown；若模板未同步、读取被拒或不完整，报告 `REPORT_TEMPLATE_LOAD_FAILED`，不回退旧版样例。核查 HEAD/INDEX/WORKTREE/UNTRACKED 证据来源。结束后手动切回 Build Agent，以免 Reviewer 的权限限制影响正常开发。

### Git 基线与 Controller 参数

```text
/codea-review
/codea-review OrderController
/codea-review --base origin/develop
/codea-review --controller OrderController
/codea-review --base origin/develop --controller OrderController
/codea-review --controller OrderController --base origin/develop
```

- `--base`：选择**本地已经存在、指向 commit 的分支或 remote-tracking ref**，例如 `master`、`develop`、`origin/master`、`origin/develop`、`feature/test`。只读计算 `merge-base(base, HEAD)`，作为 COMMITTED 的起点；STAGED、UNSTAGED、UNTRACKED 不变。不 checkout/switch/fetch/pull，不要求切换当前业务分支。显式基线不存在或无共同祖先直接报错，不回退 master。本地 remote-tracking ref 可能落后，命令不联网更新。
- `--controller`：与旧位置 Controller 简写等价，重点调查入口及必要调用链，不严格排除其他 Controller/SQL。V1 名称为 ASCII 字母或下划线开头、其后字母/数字/下划线的简单类名。无法定位时如实说明，不编造链路；仍审查其余有效变更。
- 两个显式参数顺序任意，各只能出现一次；旧简写也可与 `--base` 组合。缺值、重复、未知选项、多余位置参数、位置 Controller 与 `--controller` 同用均报错并提示用法。V1 不支持引号、`--base=value`、任意 revision（如 `HEAD~3`、`A..B`、`:path`）或 Shell 表达式。
- 首次工具调用仍加载 Skill；完整参数校验完成后才查询 Git。不合法参数不启动 Review、不生成或覆盖已有报告。基线字符白名单 → 完整本地 ref 校验 → commit 类型 → merge-base；仅按 Skill 的固定安全参数模板执行。新增动态 Git 查询仍 `ask`，Agent 权限完全不变。
- 未提供 `--base` 时（无参数、旧 Controller 简写或仅 `--controller`），保持原有 `git rev-parse origin/master` / `master` 和 `git merge-base origin/master HEAD` / `master HEAD` 固定查询与原有 allow 授权体验，不追加配置、ref 格式或对象类型检查。只有显式 `--base` 才执行新增校验；`origin/develop` 等名称优先查询 `refs/remotes/origin/develop`，同名本地 `refs/heads/origin/develop` 不会遮蔽远程跟踪分支。
- 仅在显式 `--base` 路径，为避免 Git 只读查询隐式补取对象，先通过两条只读配置查询检测 partial clone/promisor。遇到 `extensions.partialclone` 或任何含 `promisor` 的配置键（即使值为 false）均明确返回 `BASE_LOCAL_ONLY_REQUIRED` 并停止，不改配置、不 fetch、不覆盖报告；未指定基线的原有路径不受该新增检查影响。

自动回归：`python tests/test_codea_review_2_0_hardening.py` 和 `python tests/fixtures/codea-review-t1/test_git_scope.py`。参数测试从 Skill 声明式契约读取规则，在真实临时 Git 仓库验证不同基线、共同祖先、完整四层、SQL-only、拒绝输入及工作区不变；测试驱动只存在于测试文件，**不作为产品 Parser 或 Runtime 安装**。这些确定性测试不证明模型会正确执行规则；公司 Windows + OpenCode 2.x + 私有模型的默认命令、旧简写、组合参数实际验收仍为 **PENDING**。

示例 fixture 可在测试机器执行 `tests/fixtures/codea-review-t1/setup.ps1`，会独立生成一个包含 `master` 和 `feature/t1-risk-review` 的新 Git 仓库，不接触公司业务代码。

## 安全和版本边界

- Reviewer 的权限仅用于**可信内网开发机上的交互授权**，不是 OS 只读沙箱。原生 read/glob/grep 允许访问项目业务代码，业务 edit/write/patch 默认拒绝，唯一原生写入例外为 `temp/codea-review-report.md`；**固定安全 Git 查询按 Agent 的 allow 执行，动态 Git 查询与其它 Shell 仍 ask，直接 Git 写动词按现有配置 deny；不能把这些规则当作对全部组合命令的强制只读保证**。Skill 仍明确禁止主动发起 Git 写命令、构建、网络与任意 Shell 执行；用户和公司策略应拒绝此类请求。请从 Git 根目录启动 OpenCode。
- 固定 `git -c core.fsmonitor=false status ...` 是为了降低通过 fsmonitor hook 隐式执行程序的风险。Git 版本不达最低要求即应停止。对已被恶意改造的 Git/本机环境不做绝对安全保证。
- `skill` 不会自动加载 `references/`：`SKILL.md` 保留审查规则；独立 Markdown 排版唯一标准为同 Skill 目录下的 `references/report-template.md`，须在生成报告前经原生 `read` 成功读取。**Skill 能加载不代表模板已加载。**
- 本地 `origin/master` 可能落后，不会联网 `fetch`。Dirty Worktree 不再阻断：同时审查 committed、staged、unstaged 和相关 untracked。所有 Git 文件清单先使用 `-- . ':(exclude,glob)**/.*/**' ':(exclude,glob)temp/**'` 过滤根目录/嵌套隐藏目录及项目根目录 temp，隐藏文件不计有效数量，工作区中途变化则标记需复核。
- Codea Review 2.0 模板五章：评审摘要、调用链与覆盖范围、风险清单、问题明细、后续处理与评审边界。**TUI 完整输出 + 自动保存至项目根目录 `temp/codea-review-report.md`（有目录就用、无目录自动创建，每次覆盖）**，不需要 1.8 Go Runtime。项目 `temp/` 在 Git 清单阶段被排除，避免 Review 报告进入下一次有效变更。
- 不能把没有 Finding 理解成“可上线”。模型可能漏报、误报，复杂动态调用只能标记未知。

## 2.0 Hardening：独立 SQL 与 Git 只读权限

- **一个入口**：仍使用 `/codea-review` + 原本的 `codea-review` Skill，不加载 TeamAI、不增加 KB Agent、Runtime/MCP、报告引擎。
- **SQL**：数据库迁移、数据修复与普通独立 `.sql`（包括 `.SQL`）在 COMMITTED、STAGED、UNSTAGED、UNTRACKED 四层均作为有效业务文件；SQL-only 和 Java+SQL 都允许正常 Review。相同脚本跨层只计一个唯一文件、保留多层取证；untracked 从实际工作区 read。脚本只核查有证据的中高风险 UPDATE/DELETE、租户约束、迁移幂等、DDL 兼容、数据一致性，不能仅凭 SQL 关键字臆断问题。
- **权限**：保持 OpenCode **V2** 的 `permissions:` + `shell` 格式，只保留简单工具级规则：默认 deny、read/glob/grep allow、external_directory deny、仅本 Skill allow、固定报告 edit allow、**现有固定 Git 查询 allow、动态查询 ask、常见 Git 写动词 deny**。不引入新的 Git 权限框架；Skill 规定只读审查行为，Shell 用户授权是额外人工审批，**不能声称强制禁止 Git 写操作或危险 Shell**。不改变 Java/SQL 检查清单和固定五章报告。
- **报告**：Codea Review 2.0 独立 `references/report-template.md` 是唯一版式来源，五章标题、Finding 编号、TUI 输出与固定 temp 文件保留。SQL-only 仅在既有“调用链与覆盖范围”注明无 Controller 链路，不添加第六章。

确定性回归可在测试环境运行 `python tests/test_codea_review_2_0_hardening.py`；**这不代表真实 OpenCode 授权或私有模型 Finding 质量已验收**。实际执行与历史三项通用 Finding 的验收表见 [2.0 hardening 验收说明](../docs/examples/codea-review-2-0-hardening-acceptance.md)。其中三项历史真实案例尚无代码快照与原报告可供本仓库复测，必须在公司内网真实执行，不准用合成样例充当历史回归。

### 升级安装（与首次安装区分）

已有 2.0 的企业电脑：先备份 `agents/codea-reviewer.md`、`commands/codea-review.md` 和整个 `skills/codea-review/` 目录，再将本分支这两份文件及完整 Skill 目录复制到**实际已加载**的用户级 `~/.config/opencode/` 或项目级 `.opencode/` 的对应目录；不可同时保留更高优先级的同名旧版本。重启 OpenCode 2.x TUI 后在 Git 根目录运行 `/codea-review`，核对 Git 四层、SQL-only、权限拒绝及五章报告。回退时恢复原配置文件和整个 Skill 目录备份即可。

## 2.0 Usability｜Git 授权与 Markdown 报告（本分支）

权限与报告展示优化已在前序提交完成；本轮仅将 Skill 第 5 章的报告版式移到 `references/report-template.md`，并修改 Command 的报告引用和已过期授权说明。**Skill 第 0–4、6 节完全保留，Agent 权限完全不变，Command 的 Git/Review 工作流程不变。**

**安全与体验取舍**：`shell "*": ask` 为默认；仅 `git --version`、固定 status、rev-parse、`merge-base origin/master HEAD` / `master HEAD` 以及若干无动态参数的 log/show/diff 统计形式自动 `allow`。包含 SHA、文件名、排除 pathspec（尤其 `**`）的四层 Git diff、ls-files、单文件 show 与动态 blame/grep/log **继续 ask**。理由是 V2 原生权限采用全值通配符匹配，不能在不引入复杂执行器的前提下充分验证动态参数只读性；不为减少弹窗而使用宽泛 `git diff *` / `git show *` / `git *` 自动授权。常见 `git reset/restore/clean/checkout/add/commit/push/stash/rebase/merge/pull` 直接形式 deny；Shell 组合、其他命令会按默认 ask 处理，**无法宣称能强制拒绝所有变体或绕过写法**。

报告只做视觉分组：首屏文件数/HIGH/MEDIUM Markdown 卡片、已有摘要字段完整保留、调用链与文件范围表格、按真实风险等级分组的清单、按原编号独立的 Finding 详情、原后续处理表。不新增判断或 Finding。真实严重/低风险结果仍计入风险概览，空等级不输出；SQL-only 无 Controller 不补造链路。TUI 与固定 `temp/codea-review-report.md` 仍必须全文一致。

排版对照：[新旧 Markdown 报告示例](../docs/examples/codea-review-2-0-usability-report-comparison.md)；**示例为虚构的占位排版，并不是实际缺陷或已验收的证据**。自动回归命令仍为 `python tests/test_codea_review_2_0_hardening.py`，包括 Git 四层确定性测试及展示结构检查。公司 Windows 私有模型实际权限匹配、Finding 非退化与 7 类报告场景继续待实测。


## 报告模板独立发布与 TeamAI 同步

- 模板与 `SKILL.md` 位于同一 Skill 目录；经现有 TeamAI push/pull **同步整个 `codea-review/` Skill 目录**，不得仅发布 `SKILL.md`。TeamAI 是用户既有的文件同步方式，**不是 Codea Review 的知识检索/评审依赖**，无需改变 TeamAI CLI。
- 业务项目中如果存在 `references/report-template.md` 同名文件，不得作为可信模板。应通过 `skill(codea-review)` 返回的真实 Skill 来源定位，再经原生 `read` 读取同目录的 references 文件。不能使用 Shell 绕过外部目录权限。
- 内网验收须分别确认 push/pull 后的两文件 SHA256、`skill` 加载、原生 `read` 实际路径与文本、五章 Markdown 的 TUI 与报告文件一致；若 `external_directory` 拒绝，记录 `REPORT_TEMPLATE_LOAD_FAILED`，**不能私自把 `external_directory: "*"` 改为 allow**。
- 这些 Windows/OpenCode/TeamAI 真机验收在本次 GitHub 代码改造环境中无法验证，标为 **PENDING**；不要把目录结构静态测试冒充真实读取成功。

## 离线升级、卸载和回退

在 TUI 外备份 Agent、Command 和整个 Skill 目录及其 SHA256，确认新版本与企业 OpenCode exact 版兼容后手动替换；不要覆盖冲突的其他团队同名配置。重新启动或按目标版本的热重载规则验证 `/codea-review` 和 `skill`。回退恢复各原配置及 Skill 目录备份，或仅删除由本模块安装的两个配置文件和整份 Skill 目录；不要删除整个 `~/.config/opencode`、Build/Plan 或 Provider 配置。

## T1 验收与后续

读取 `docs/examples/codea-review-t1-acceptance.md`：包含真机记录表、最小冒烟、危险命令负测和待填写的真实报告证据。只有真实 Windows + OpenCode V2 + 私有模型完成闭环后才能把 T1 状态标记为 PASS；T2 的 F01～F08 准确率与 T3 的 17 文件 A/B 性能仍是后续独立阶段。

相关设计：`docs/superpowers/specs/2026-10-14-codea-review-2-native-skill-design.md`。
