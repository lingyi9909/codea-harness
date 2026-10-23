# OpenCode TUI v2.0.27 测试包与原生修复

**2.0.27 原生参数修复已开发并验证。** 本目录提供 Command + Skill 测试资源、三个现有宿主文件的最小源码补丁、独立样例和安装回退步骤。公司构建、Windows、实际 TUI 交互、私有模型正常提交与 TeamAI 仍 PENDING，PR 保持 Draft，不自动合并。

未修改的公开 2.0.27 仍有参数执行问题。测试资源仅用于**已经应用等价前置拦截且通过探针的现有宿主**，不能直接复制到未经修复的公司安装。包内不含替换二进制、Runtime、MCP、插件或 Agent，也不含旧 1.2.27 补丁。

## 最新复审修正：验收器保留原始换行

本次仅修测试/验收工具，Command、Skill、展示模板、原生补丁和探针代码均未改。checkpoint按bytes读取后严格UTF-8解码，保留CRLF/LF/混合换行；fixture消息明确写UTF-8字节，避免Windows默认文本换行转换。verify改读`git cat-file commit HEAD`原始正文并逐字节比较，既不把Git log的附加展示换行当成正文，也不再用rstrip吞掉真实末尾空行。

新增七个子场景：CRLF、混合换行、无终止换行、多末尾空行的正确提交应通过；CRLF改LF、正文改变、真实末尾空行被删应失败。先观察旧工具四个子场景RED，修后通过；保留消息清理矩阵和strip负对照。独立CRLF样例的输入/checkpoint/真实commit-body Base64见 `CRLF-RESULTS.json`。

本地回归 **65/65 PASS**（fixture21 + Review40 + Review Git4）。这是macOS构造原始字节对照，不冒充Windows执行。资源未变，本轮不重跑API；此前相同产品资源的12/12探针记录保留，不计作新运行。Windows/公司模型/TUI/TeamAI继续PENDING。

## 前次复审修正：独立模板与确认全文保真

展示格式已拆到 `payload/skills/codea-commit/templates/commit-preview.md`。Command仍加载同一个Skill；Skill从原生返回的基目录读取模板，未完整读到就停止，不 add/commit。模板仅组织聊天Markdown，不是新入口/网页/执行器，不需要修改宿主。改变格式只改该模板；流程和安全约束继续放在SKILL.md。

安装仍为两项目标：**一个Command文件 + 一个完整Skill目录**，现在共有三个产品文件：

```text
commands/codea-commit.md
skills/codea-commit/SKILL.md
skills/codea-commit/templates/commit-preview.md
```

必须复制整个 `payload/skills/codea-commit/`，备份/回退也保留整个单个Skill目录，不能漏掉templates。分别记录三个文件摘要；TeamAI仍需内网确认是否完整携带模板，不假定自动同步。

P2消息清理已改为一次性的 `git commit --cleanup=verbatim -F <消息文件>`；默认/strip、自定义注释字符不再静默清理用户确认的正文与空白。不修改任何Git配置，不绕hooks/签名；项目明确不允许原文模式或Git不支持时，在预览确认前停止说明冲突，不回退默认清理。细节与真实Git对照见 `MESSAGE-CLEANUP.md`。

本轮当前资源/模板探针见 `TEMPLATE-CLEANUP-RESULTS.json`；`PREVIEW-RESULTS.json`和`PATCHED-RESULTS.json`均为此前资源历史，不替代本轮交互验收。原生Skill只列出配套文件，不自动把模板正文交给模型，所以必须保留显式读取步骤；实际模型读取仍PENDING。

## 已有交互：预览后明确确认

Command/Skill payload 已按 2026-10-23 用户方案更新，与 candidates 同步：真实 index 预览、全文说明、三项选择“确认提交 / 修改说明 / 取消”。原生 question 缺失/被禁用/失败时聊天提问并结束执行等待，关闭/取消不再询问并终止本轮。新消息要重新确认，目标/HEAD/index 变化要重新完整预览；取消保留原有和新准备的暂存。

更新现有 Command/Skill 和Skill内模板；安装位置、两项目标（Command文件/完整Skill目录）的备份及卸载范围不变。下面宿主补丁保持原 SHA，不因为增加确认而解除安全安装门禁。轻量版没有自定义文件树/左右分栏/固定操作面板，不使用 working/turn Diff 冒充 index。

当前资源的加载/参数与模板落盘探针见 `TEMPLATE-CLEANUP-RESULTS.json`；之前的探针结果只保留历史，不能当作新版交互已通过。实际 question 可用/不可用、私有模型确认、Windows/TUI/主题与 TeamAI 均 **PENDING**，具体验收步骤见 `PREVIEW-ACCEPTANCE.md`。

## 版本和实际结果

| 验证 | 结果 |
|---|---|
| 用户确认的 TUI | `v2.0.27`；公司构建来源/二进制摘要与服务端身份仍 PENDING |
| 固定上游源码 | `v2.0.27` / `1f4d07a4a519185955ae8ab2ea3607805509e386` |
| 未修改公开发行包 | `@opencode/cli-darwin-arm64@2.0.27`，npm integrity 校验通过；CLI/服务端均2.0.27，原探针 **6 PASS / 5 FAIL** |
| 修复后原生源码新增安全回归 | **17/17 PASS**（13个公共入口、4个真实Server API模板来源） |
| 修复源码 CLI 启动服务后，同一已安装入口探针 | **11/11 PASS**；非空参数HTTP400、中文错误、无sentinel、无提示持久化 |
| 未修改2.0.27客户端连接修复服务 | 中文错误显示，退出码1；不执行Shell、不写提示 |
| Core/Server 类型检查 | PASS |
| Server 全套 | **86 PASS / 1 SKIP / 0 FAIL** |
| Core 正式测试入口全套 | **6605 PASS / 43 SKIP / 8 FAIL**；8项均在未修改源码失败文件对照中复现，未宣称全绿 |
| 根级 `bun run check` | 未通过；网站Astro尝试写用户`Library/Preferences`被沙箱拒绝，未为此扩大权限 |
| 当前 Codea Git/Review 回归（2026-10-23） | **65/65 PASS**（fixture21 + Review40 + Review Git4） |

本次运行平台为macOS arm64，Bun1.4.2。源码进程测试使用上游原生CLI入口及Solid preload，构建宏version为2.0.27、channel为local；临时启动脚本只用于测试，不交付、不替换公司环境，不声称已完成发行二进制构建/签名。详情见 `SOURCE.json`、`VALIDATION.json`；未修/首次修后原始结果分别见 `RESULTS.json`、`PATCHED-RESULTS.json`；当前模板版资源重新核验见 `TEMPLATE-CLEANUP-RESULTS.json`（12/12，包含模板落盘/非入口检查），不证明模型读取或交互。

Core剩余失败涉及本机旧Bash的进程替换/复合语法，以及Bun文件fetch错误文案断言；逐项名称和基线对照已记录，未改这些无关功能。空/仅空白检查只验证进入既有提示路径；真实模型加载Skill、中文说明质量、正常提交、公司权限、Windows/TUI/TeamAI不能由这些结果替代。

## 为什么需要这个最小改动

该版[Command加载器](https://github.com/anomalyco/opencode/blob/1f4d07a4a519185955ae8ab2ea3607805509e386/packages/core/src/config/plugin/command.ts)先展开参数/默认追加，再匹配执行Shell，之后才提交模型提示。提示词拒绝、删除占位符、围栏、Agent工具deny都无法前置拦住。[配置schema](https://github.com/anomalyco/opencode/blob/1f4d07a4a519185955ae8ab2ea3607805509e386/packages/schema/src/config/command.ts)没有无参数或关闭插值字段。未修发行包的三种参数实测写出标记后才报模型不可用。

补丁 `opencode-v2.0.27-codea-commit.patch` 只改现有三处文件：

1. `packages/core/src/command.ts`：公共`Command.execute`先检查精确命令ID及原始`prompt.text.trim()`，非空返回原生`ArgumentsError`，在查找/执行命令定义、模板、Shell、切换Agent和模型提示之前停止。空/仅空白沿原路径，其他命令不变。
2. `packages/core/src/session.ts`：补充原生错误类型声明。
3. `packages/server/src/handlers/session.ts`：映射既有`InvalidRequestError`，HTTP400与中文用法。现有TUI/CLI错误通路继续处理；不改公开API合约、客户端、权限或busy/retry状态。

另外包含两个原生回归测试文件。补丁SHA256：`eb1d9c43a0fd3891321e9ed3ef02da392e5f7aacc1ea7ab2d280d7be06412427`。已在干净固定基线检查正向应用/反向检查，并逐文件核对与实际测试源码一致；锁文件与manifest未改。

由现有OpenCode维护方在**现有宿主源码分支**核对实际构建来源后审核应用，继续公司原构建、签名和发布流程。本包不直接替换公司版本。如果公司构建已有等价前置能力，先测试现有能力即可；同版本号不保证同源码，不强行套用到公司分叉。

```text
# 仅在宿主源码仓库，不在业务仓库；先确认HEAD等于固定基线
# 其他源码基线需要维护方移植/审阅，不能只凭apply无冲突认定兼容
git rev-parse HEAD
git apply --check <补丁绝对路径>
git apply <补丁绝对路径>
```

## Command / Skill 加载约定

Command使用`commands/codea-commit.md`，继续`agent: build`，无模型/权限/子Agent覆盖。首个工具动作以`{"id":"codea-commit"}`加载原生Skill；正常私有模型路径仍需内网验证。

[Skill加载器](https://github.com/anomalyco/opencode/blob/1f4d07a4a519185955ae8ab2ea3607805509e386/packages/core/src/config/plugin/skill.ts)与[解析器](https://github.com/anomalyco/opencode/blob/1f4d07a4a519185955ae8ab2ea3607805509e386/packages/core/src/config/plugin/skill-file.ts)用目录名派生ID。真实API已核对完整正文和来源路径；单独Skill对照在该构建不自动注册为Command。不把V1的同名Skill slash行为或`slash: false`假设套到此版本。

可安装的测试资源为 `payload/commands/codea-commit.md` 与完整 `payload/skills/codea-commit/`（SKILL.md和templates/commit-preview.md）；三个文件均与探针临时安装的对应 `candidates/` 输入字节一致。payload不放在OpenCode实际扫描根下，不自动启用；candidates继续作为原始测试输入。

## 先核验现有宿主

需要Python3.9+及公司已有**原生**`opencode.exe`；不要填写npm`.ps1`/`.cmd`启动shim。先确认CLI与实际服务端版本；V2的TUI可能连接共享后台服务，不能仅看TUI版本号。本探针显式启动独立本机服务并读取`/api/info`。

解压包，在包根目录PowerShell运行：

```powershell
$exe = 'C:\公司现有安装目录\opencode.exe' # 替换为真实已安装路径
& $exe --version
& $exe debug paths config
$report = Join-Path $env:TEMP ('codea-v2027-' + [guid]::NewGuid().ToString('N') + '.json')
python .\tests\fixtures\codea-commit\probe_v2.py --opencode $exe --report $report
$LASTEXITCODE
Get-Content -LiteralPath $report -Raw -Encoding UTF8
```

POSIX：`python3 tests/fixtures/codea-commit/probe_v2.py --opencode /absolute/path/to/existing/opencode --report /tmp/<唯一名称>.json`。

脚本使用V2 `/api/*`，不用V1 `/session`探针。先核对版本和路径隔离，再仅向新临时配置复制Command、Skill及目录内模板候选；无模型凭据，指定不存在模型，工具deny，不操作Git、不连接共享服务、不读取业务配置。XDG/配置/测试home/tmp隔离，用户HOME不改；带认证的loopback服务与数据在退出时清理。代理阻断不是网络沙箱承诺，企业验收继续遵守原网络策略。

退出码1=边界失败、2=不完整/不兼容，均不能安装；0仅为这组API检查通过，仍须实际TUI与完整S01—S08复审。不要将模型口头拒绝、跳过项或普通Git自检当作宿主安全通过。不要上传含凭据的配置诊断。

## 安装、备份和卸载（仅安全核验通过后）

1. 先使用公司获准的测试配置和新测试仓库。关闭对应TUI/独立服务，通过既有管理方式暂停本模块TeamAI同步；不修改共享服务、Reviewer或模型配置。
2. 以`opencode debug paths config`确认真实全局路径。默认通常为`~/.config/opencode`，Windows通常为`$HOME\.config\opencode`，不硬编码`%APPDATA%`。仅安装 `<真实config>\commands\codea-commit.md` 与完整 `<真实config>\skills\codea-commit\`（内含SKILL.md和templates/commit-preview.md）。项目级仅用于测试仓库`.opencode/commands/codea-commit.md`和完整`.opencode/skills/codea-commit/`；两种范围选一种。检查同名JSON、项目、兼容Skill与TeamAI显式来源是否覆盖目标。
3. 在所有扫描目录之外新建唯一备份目录，如`$env:TEMP\codea-commit-backup-<GUID>`。记录原两个目标（Command文件与完整Skill目录）是否存在、绝对路径与目录内每个文件SHA256；复制原Command和**单个完整**`skills/codea-commit/`目录，核对备份摘要后才移出原目标。不存在也记入记录；不改整个commands/skills，不覆盖`opencode.json(c)`。
4. 确认目标不存在后，将`payload/commands/codea-commit.md`复制为Command目标，将`payload/skills/codea-commit/`复制为单个Skill目录。保存三个产品文件的安装摘要。任一步失败立即停止，从备份恢复已改的单个目标，不继续试用。不复制整个测试包到配置目录。
5. 在新独立样例仓库启动现有TUI，核对实际来源/摘要、Build路由、首工具`skill({"id":"codea-commit"})`、完整预览、明确确认/取消/修改说明、权限提示、中文说明和真实提交结果。C01—C03每项三次。TeamAI分发能力仍PENDING：分别核对Skill与Command实际落盘路径/摘要，不猜同步命令。
6. 卸载/回退：关闭测试会话、暂停本模块同步；对照安装摘要，已被更新的先备份。只移走本次Command与单个Skill目录，再恢复原两个目标；原来不存在则保持不存在。重启核对入口消失或恢复原定义。不卸载OpenCode，不删除整个配置目录或其他能力。

## 独立Git样例

`setup.py`、`verify.py`、`test_fixtures.py`仅用Python标准库和Git，写操作只在指定新临时仓库，配置为该仓库local；无Codea Runtime/服务/模型依赖。安全未通过前可自检数据，不能试装提交入口。

```powershell
python .\tests\fixtures\codea-commit\test_fixtures.py
$caseRoot = Join-Path $env:TEMP ('codea-C02-' + [guid]::NewGuid().ToString('N'))
python .\tests\fixtures\codea-commit\setup.py C02 $caseRoot
# 安全验收通过后，在输出repo根的TUI输入 /codea-commit；核对完整预览并明确确认后再检查结果
# manifest位于仓库外，不交给模型，不用普通Git手工提交冒充产品验收
python .\tests\fixtures\codea-commit\verify.py ($caseRoot + '.manifest.json') $caseRoot
```

C02检查同文件A已暂存、B未暂存时只提交A，保留B与无关文件。完整C01—C16、hooks/签名失败、范围询问与提交后真实核验沿用仓库原验收表；验证器通过不证明中文说明质量或Skill工具加载，保留真实TUI轨迹。
