# Codea Review 2.0 · 基础能力完善（Java / Mapper XML / SQL）

本目录只有三个 **OpenCode V2 运行文件**；没有 Review Runtime、插件、TeamAI 同步服务或网络安装依赖。

```text
codea-review/
├── agents/codea-reviewer.md
├── commands/codea-review.md
└── skills/codea-review/SKILL.md
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
    @("skills\codea-review\SKILL.md", "skills\codea-review\SKILL.md")
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
    Copy-Item -LiteralPath $from -Destination $to
    Get-FileHash -LiteralPath $to -Algorithm SHA256 | Format-Table
}
```

**首次启动前**检查当前 Java 项目及父级目录的 `.opencode/commands/codea-review.md`、`.opencode/agents/codea-reviewer.md`、`.opencode/skills/codea-review` 与兼容 Skill 目录，避免覆盖全局同名 ID。冲突时先停止，不要在被覆盖的 `/codea-review` 内才发现问题。不要将这三个文件复制到 Java 业务 Git 工作区；不更改 `opencode.json` 的默认 Build/Plan/Provider。

## 使用方式

1. 在 **允许未提交修改的 Git 需求分支**目录启动公司已批准的 OpenCode 2.x TUI，确认当前模型选择的是公司私有 Provider，Tool Calling 正常。
2. 输入 `/codea-review`；或 `/codea-review OrderController`。
3. 观察第一个工具调用实际为 `skill(codea-review)`；确认 Skill 返回后，Reviewer 获取本地 `origin/master`（或 `master`）与已提交 HEAD，还纳入 staged/unstaged/相关 untracked 工作区变更，按 Git Diff 先快扫后按需查上下文。
4. 受控 Git Diff/Show/merge-base/ls-files 使用 OpenCode V2 只读 allow；如果仍然弹出授权/拒绝，检查**实际命令、当前 Agent 来源、全局/项目/组织规则**，不能开放 `git *` 或自行绕过公司政策。Reviewer 不执行测试且不修改业务代码；完成审查后自动使用原生 write 创建/覆盖项目根目录的 `temp/codea-review-report.md`（没有 temp 时自动建目录）。
5. 在 TUI 读取与 Harness 1.8 版式一致的完整 Markdown 报告，并核查 HEAD/INDEX/WORKTREE/UNTRACKED 证据来源。结束后手动切回 Build Agent，以免 Reviewer 的权限限制影响正常开发。

示例 fixture 可在测试机器执行 `tests/fixtures/codea-review-t1/setup.ps1`，会独立生成一个包含 `master` 和 `feature/t1-risk-review` 的新 Git 仓库，不接触公司业务代码。

## 安全和版本边界

- Reviewer 的权限只做**可信内网开发机上的防误操作**，不是 OS 只读沙箱。仅允许原生 write 写入固定的 `temp/codea-review-report.md`，其他编辑、网络、构建与 Git 写命令继续禁止；只对白名单形式的必要只读 Git 查询使用 `allow`，对危险参数及 Shell 组合命令仍执行 `deny`。请从 Java 项目 Git 根目录启动 OpenCode 以匹配相对路径。
- 固定 `git -c core.fsmonitor=false status ...` 是为了降低通过 fsmonitor hook 隐式执行程序的风险。Git 版本不达最低要求即应停止。对已被恶意改造的 Git/本机环境不做绝对安全保证。
- `skill` 不会自动加载其他文件：本模块把 Java 风险清单和报告格式都写进唯一 `SKILL.md`。
- 本地 `origin/master` 可能落后，不会联网 `fetch`。Dirty Worktree 不再阻断：同时审查 committed、staged、unstaged 和相关 untracked。所有 Git 文件清单先使用 `-- . ':(exclude,glob)**/.*/**' ':(exclude,glob)temp/**'` 过滤根目录/嵌套隐藏目录及项目根目录 temp，隐藏文件不计有效数量，工作区中途变化则标记需复核。
- 1.8 报告五章：评审摘要、调用链与覆盖范围、风险清单、问题明细、后续处理与评审边界。**TUI 完整输出 + 自动保存至项目根目录 `temp/codea-review-report.md`（有目录就用、无目录自动创建，每次覆盖）**，不需要 1.8 Go Runtime。项目 `temp/` 在 Git 清单阶段被排除，避免 Review 报告进入下一次有效变更。
- 不能把没有 Finding 理解成“可上线”。模型可能漏报、误报，复杂动态调用只能标记未知。

## 2.0 Hardening：独立 SQL 与 Git 只读权限

- **一个入口**：仍使用 `/codea-review` + 原本的 `codea-review` Skill，不加载 TeamAI、不增加 KB Agent、Runtime/MCP、报告引擎。
- **SQL**：数据库迁移、数据修复与普通独立 `.sql`（包括 `.SQL`）在 COMMITTED、STAGED、UNSTAGED、UNTRACKED 四层均作为有效业务文件；SQL-only 和 Java+SQL 都允许正常 Review。相同脚本跨层只计一个唯一文件、保留多层取证；untracked 从实际工作区 read。脚本只核查有证据的中高风险 UPDATE/DELETE、租户约束、迁移幂等、DDL 兼容、数据一致性，不能仅凭 SQL 关键字臆断问题。
- **权限**：保持 OpenCode **V2** 的 `permissions:` + `shell` 格式；固定 Git 查询和限定参数形式的动态 Git 查询 allow；所有 Git 写操作、`--output`、Shell 管道/分号/重定向等继续 deny。禁止 `git *` 或其它工具绕过拒绝。Java/SQL 都只读，唯一可编辑文件仍为 `temp/codea-review-report.md`。
- **报告**：Harness 1.8 的五章及标题、Finding 编号、TUI 输出与固定 temp 文件保留。SQL-only 仅在既有“调用链与覆盖范围”注明无 Controller 链路，不添加第六章。

确定性回归可在测试环境运行 `python tests/test_codea_review_2_0_hardening.py`；**这不代表真实 OpenCode 授权或私有模型 Finding 质量已验收**。实际执行与历史三项通用 Finding 的验收表见 [2.0 hardening 验收说明](../docs/examples/codea-review-2-0-hardening-acceptance.md)。其中三项历史真实案例尚无代码快照与原报告可供本仓库复测，必须在公司内网真实执行，不准用合成样例充当历史回归。

### 升级安装（与首次安装区分）

已有 2.0 的企业电脑：先备份 `agents/codea-reviewer.md`、`commands/codea-review.md`、`skills/codea-review/SKILL.md` 三份配置，再将本分支这三个文件复制到**实际已加载**的用户级 `~/.config/opencode/` 或项目级 `.opencode/` 的对应目录；不可同时保留更高优先级的同名旧版本。重启 OpenCode 2.x TUI 后在 Git 根目录运行 `/codea-review`，核对 Git 四层、SQL-only、权限拒绝及五章报告。回退时恢复三份 SHA256 备份即可。

## 离线升级、卸载和回退

在 TUI 外备份三个目标文件及其 SHA256，确认新版本与企业 OpenCode exact 版兼容后手动替换；不要覆盖冲突的其他团队同名配置。重新启动或按目标版本的热重载规则验证 `/codea-review` 和 `skill`。回退恢复三个备份文件，或仅删除由本模块安装的三个文件；不要删除整个 `~/.config/opencode`、Build/Plan 或 Provider 配置。

## T1 验收与后续

读取 `docs/examples/codea-review-t1-acceptance.md`：包含真机记录表、最小冒烟、危险命令负测和待填写的真实报告证据。只有真实 Windows + OpenCode V2 + 私有模型完成闭环后才能把 T1 状态标记为 PASS；T2 的 F01～F08 准确率与 T3 的 17 文件 A/B 性能仍是后续独立阶段。

相关设计：`docs/superpowers/specs/2026-10-14-codea-review-2-native-skill-design.md`。
