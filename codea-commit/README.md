# Codea Commit 1.0（待内网验收）

在公司现有 OpenCode 输入 `/codea-commit`，由现有 Build 读取真实改动，生成中文说明并完成一次本地提交。产品只有两个 Markdown 资源，无 Runtime、MCP、插件、新 Agent 或 Python 运行依赖，不修改 Review、模型或权限配置。当前为待验收实现，**公司 exact 版本、原生权限与 TeamAI 分发均 PENDING**。

## 完整安装清单与来源

| 仓库源文件 | 用户级目标（相对 `~/.config/opencode/`） |
|---|---|
| `codea-commit/commands/codea-commit.md` | `commands/codea-commit.md` |
| `codea-commit/skills/codea-commit/SKILL.md` | `skills/codea-commit/SKILL.md` |

只安装这两份文件；本 README、fixture、验收表不用装进 OpenCode。源码目录不是自动安装位置，不向业务仓库写 `.opencode`。本模块可离线复制 Markdown，与根 README 的历史 Runtime 安装包流程不同。

基线、两个资源各自 SHA256、兼容性与验收状态见 [验收记录](../docs/examples/codea-commit-acceptance.md)。包的源版本和两个 SHA 必须一起记录，不能仅记录 Skill 同步成功。若公司 Build ID 不是 `build`，先核实它是现有有提交权限的 Build，再仅适配 Command 的 `agent`，记录适配后 SHA；不要新增 Agent 或默认落到 Reviewer/Plan。

## 安装前核对实际加载来源

在独立测试用户配置与临时 fixture 仓库先验收，然后再部署到开发用户：

1. 记录 Windows、OpenCode **exact** 版本、Git、实际 Shell、公司模型与 TeamAI exact 版本，不以“2.x”代替。
2. 查用户配置以及当前项目到 worktree 根路径中的 `.opencode/commands/codea-commit.md`、`skills/codea-commit`；旧版可能使用单数 `command/`、`skill/`，也应排查。同样查 `.claude/skills`、`.agents/skills` 等该 exact 版支持的兼容 Skill 来源。
3. 查全局/项目的 `opencode.json`、`opencode.jsonc` 是否定义同名 `command.codea-commit`；检查公司额外配置/托管资源与 `OPENCODE_CONFIG`、`OPENCODE_CONFIG_DIR` 指向的实际来源。不要输出 Provider 凭据。
4. 有同名资源或不清楚优先级时，先停止部署并查明生效路径/版本，不能盲目覆盖或猜项目级一定优先。确认 exact 版刷新方式；重新启动后从工具轨迹核实 Build 和 `skill(codea-commit)` 返回的实际来源路径，并与预期安装文件的 SHA 对照。若版本不显示来源，使用该版本已有配置诊断能力；仍无法识别则保持 PENDING。

当前上游 [Command 文档](https://opencode.ai/docs/commands/) 支持 `agent`、用户级目录及参数占位；[Skill 文档](https://opencode.ai/docs/skills/) 支持原生 skill 加载。它们不能证明公司定制版兼容，发现差异时记录最小目录/路由适配，不重造执行框架。

## Windows 首次离线复制

完成上述来源检查后，在解压的 codea-harness 根目录运行 PowerShell。仅使用原有组织授权，不修改执行策略或权限。先在专用验收账户执行；以下采用用户级路径，不重定向 `$HOME`。

```powershell
$commitSource = (Resolve-Path '.\codea-commit').Path
$commitConfig = Join-Path $HOME '.config\opencode'
$commitEntries = @('commands\codea-commit.md', 'skills\codea-commit\SKILL.md')
# 先检查全部源文件与整个目标 Skill 目录，避免半安装或残留混用。
foreach ($entry in $commitEntries) {
    if (-not (Test-Path -LiteralPath (Join-Path $commitSource $entry) -PathType Leaf)) { throw "缺少源文件：$entry" }
}
if ((Test-Path -LiteralPath (Join-Path $commitConfig 'commands\codea-commit.md')) -or
    (Test-Path -LiteralPath (Join-Path $commitConfig 'skills\codea-commit'))) { throw '已有同名资源，请先检查来源并备份，按升级流程处理' }
# 部署前人工将以下源文件 SHA256 与验收记录中的两个值分别对照。
foreach ($entry in $commitEntries) { Get-FileHash -LiteralPath (Join-Path $commitSource $entry) -Algorithm SHA256 }
foreach ($entry in $commitEntries) {
    $target = Join-Path $commitConfig $entry
    New-Item -ItemType Directory -Path (Split-Path -Parent $target) -Force | Out-Null
    Copy-Item -LiteralPath (Join-Path $commitSource $entry) -Destination $target
    $sourceHash = (Get-FileHash -LiteralPath (Join-Path $commitSource $entry) -Algorithm SHA256).Hash
    if ((Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash -ne $sourceHash) { throw "复制后 SHA 不一致：$entry" }
}
```

两个资源都复制并核验后，按 exact 版本重启/刷新，在临时 Git 仓库调用命令。确认首个工具是原生 `skill(codea-commit)`，由既有 Build 执行，模型仍为公司 Provider。若路径与预期不符，停止并检查同名覆盖。系统临时文件写入、只读 Git、literal 明确路径 add 和 commit 的原生权限提示需分别记录，不新增 Shell/Git allow。

## TeamAI 分发

现有 Review 文档记录通过 TeamAI push/pull 同步**整个 Skill 目录**。本模块仅复用该渠道：发布 `skills/codea-commit/`，拉取后核对实际用户级路径与 SKILL.md 的 SHA。公司 TeamAI 的具体命令、支持资源与行为需内网填入验收表，当前不臆造 CLI 参数。

Command 是否能被 TeamAI 同步 **PENDING**。若不能，使用上述离线文件复制，将 Command 独立纳入公司原有配置包；该离线方式的公司真机验证也为 PENDING。不要求修改 TeamAI CLI。不要以只拉到 Skill 就宣称 `/codea-commit` 已安装；入口旧版、Skill 新版和项目同名覆盖均须单独验收。

## 使用与范围

唯一入口 `/codea-commit`，首版无参数，任何非空参数都返回“当前版本仅支持 /codea-commit”。

- 已有明确暂存集：仅提交 index；同文件未暂存部分保持，不重新 add。
- 暂存混有多个无关需求/敏感或临时文件：只询问缺少的分组选择，保持 index。
- 无暂存且本会话任务和所有内容归属明确：列出路径，读实际内容后仅暂存这些路径并提交。
- 无可靠上下文或同文件混有其他任务：询问范围；“文件被模型碰过”不足以认领完整文件。
- 干净仓库不提交；新仓库可创建根提交。detached/未完成操作/冲突等停止，保留现场。

范围明确时展示摘要后直接提交，无例行二次确认；公司原生权限提示、hooks 与签名仍正常生效。说明默认 `type(scope): 中文行为说明`，有依据才写原因；不编造测试。返回真实 SHA、分支、完整消息、剩余 staged/unstaged/untracked 路径和 hook 观察结果。

示例（仅格式示意，不是实测结果）：

```text
已提交：<真实完整 SHA>
分支：feature/example
fix(order): 避免重复回调导致订单状态回退
本次提交：3 个文件
剩余改动：未暂存 src/Other.java；未跟踪 notes.txt
测试：本命令未主动运行；hook 结果按实际说明
远程：未执行 push
```

提交失败、工具超时或 hook 改写仍检查实际 HEAD。已产生 commit 但内容/关系异常时返回“已产生提交，需复核”，不自动回滚/amend/重试。Skill 是行为约定，不能消除并发修改 index 与 commit 之间的竞态。无法保真处理异常路径或信息不足时停止。

## 升级、回退与卸载

先关闭/暂停当前 OpenCode 会话，记录所有同名资源的真实来源。将本模块的 `commands/codea-commit.md` 和整个 `skills/codea-commit/` **分别备份到 OpenCode 扫描目录之外**（例如公司获准的备份目录），保留原路径、文件清单、SHA 和源版本；首次不存在的资源记为“不存在”。不要备份到 `commands/` 或 `skills/` 内造成重复发现。

升级只替换上述两个受管资源：核实备份后手动移出旧 Command 与 Skill 目录，再执行首次复制与两份 hash 校验，不能仅更新 Skill。过程中失败则关闭会话、恢复两个旧资源，不在混合版本下调用。回退时只移出本次安装的本模块资源，并按记录恢复原先存在的 Command/Skill，原先不存在的保持不存在；重新启动后复核来源和 SHA。

卸载时核实实际安装路径和资源归属，只移除本模块 Command 和 `skills/codea-commit/`；不删除整个 commands、skills、OpenCode 配置目录或其他模块。若 TeamAI 会自动恢复它，还需通过公司已有同步管理流程停止该资源分发。再次启动确认入口消失，既有 `/codea-review` 仍按原配置加载。

安装→升级→回退→卸载的文件级复制自检与真机运行是不同证据。公司环境完成前保持 PENDING，具体步骤与记录见 [fixture 使用说明](../tests/fixtures/codea-commit/README.md) 和 [内网验收表](../docs/examples/codea-commit-acceptance.md)。
