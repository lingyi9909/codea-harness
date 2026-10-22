# OpenCode TUI v2.0.27 适配核验包

目标已由用户确认为 **TUI v2.0.27**。当前状态：**发现原生参数前处理阻塞；尚不能恢复可用的自动提交入口**。本目录交付候选 Command + Skill、真实 2.x 核验样例和安装回退说明，不是验收通过的安装包。候选文件保持 `.txt`，不会直接被 OpenCode 扫描。

## 已实际核对的版本

| 项目 | 结果 |
|---|---|
| 用户报告的内网 TUI | `v2.0.27`；来源、二进制摘要和服务端版本未提供 |
| 本次实际启动的公开发行包 | `@opencode/cli-darwin-arm64@2.0.27` |
| 真实版本输出 | `opencode v2.0.27` |
| 真实 `/api/info` 服务端版本 | `2.0.27` |
| 平台 | macOS arm64；Windows、公司构建、实际 TUI、TeamAI 均 PENDING |
| 固定源码 | `anomalyco/opencode`，tag `v2.0.27`，commit `1f4d07a4a519185955ae8ab2ea3607805509e386` |
| 发行包完整性 | npm SHA512 integrity 校验通过；下载与二进制 SHA256 见 `SOURCE.json` |
| 真实 API 检查 | **6 PASS / 5 FAIL**；整体无参数约定 FAIL，详细响应见 `RESULTS.json` |
| 公司模型正常提交与中文说明质量 | PENDING；本探针无模型凭据，故意使用不存在的 Provider/Model |

这些记录证明公开同版本存在阻塞，不能证明公司构建与公开包相同，也不能证明 Windows/TUI 已验收。不要再用先前 1.2.27 的测试结果或补丁作为 2.x 适配依据。本核验包不含 1.2.27 补丁、二进制、依赖或宿主替换程序。

## 加载方式与真实参数行为

固定源码的 [Command 加载器](https://github.com/anomalyco/opencode/blob/1f4d07a4a519185955ae8ab2ea3607805509e386/packages/core/src/config/plugin/command.ts) 扫描 `commands/**/*.md` 与兼容目录 `command/`，Markdown 正文成为模板；JSON 使用 `commands`（复数）。候选保持 `agent: build`，不设置模型、子会话或权限。模型正常路径首个工具应以 `{"id":"codea-commit"}` 加载 Skill；此调用仍须在公司真实模型验证。

[Skill 加载器](https://github.com/anomalyco/opencode/blob/1f4d07a4a519185955ae8ab2ea3607805509e386/packages/core/src/config/plugin/skill.ts) 扫描 `skills`/兼容来源的 Markdown；[解析器](https://github.com/anomalyco/opencode/blob/1f4d07a4a519185955ae8ab2ea3607805509e386/packages/core/src/config/plugin/skill-file.ts) 用目录名派生 ID。真实 `/api/skill` 已核对候选正文完整、路径指向隔离配置。额外只安装 Skill 的对照未被自动注册为 Command；不把 V1 的同名 Skill slash 行为套到这个 V2 构建，也不把 `slash: false` 当作已验证开关。

该版本的 `evaluateTemplate`（Command 加载器 188—250 行）先替换位置参数和 `$ARGUMENTS`，没有占位符时默认追加，然后在生成的整段文本上匹配 Shell 插值并运行。围栏和伪造边界都不阻止它。[Command 配置 schema](https://github.com/anomalyco/opencode/blob/1f4d07a4a519185955ae8ab2ea3607805509e386/packages/schema/src/config/command.ts) 没有无参数约束或关闭插值字段。

真实结果：`--all`、`""` 均返回 HTTP 204 并写入用户提示；三种 Shell 参数均先写出临时 sentinel，随后才记录模型不可用。测试会话和隔离配置的工具权限全部 deny。因此，提示词拒绝、删除占位符、加围栏、拒绝工具权限均不能作为这个版本的前置保障。当前不恢复 Command/Skill 扫描入口。

## 如公司版行为一致，所需最小宿主改动

这是**改动建议，尚未实施/验证**，不是要求替换公司版本，也不提供可直接套用的 V2 补丁：

1. 在既有 [公共 `Command.execute`](https://github.com/anomalyco/opencode/blob/1f4d07a4a519185955ae8ab2ea3607805509e386/packages/core/src/command.ts#L81) 调用命令定义之前，对精确 `input.name === "codea-commit"` 检查 `input.invocation.prompt.text.trim()`。非空即拒绝；空/仅空白继续原路径。这里尚未执行模板、Shell、切换 Agent 或写入模型提示，且覆盖 Markdown/同名 JSON 所注册的入口。
2. 沿用现有原生错误通路返回“当前版本仅支持 /codea-commit”，映射为用法错误 HTTP 400，并确认实际 TUI/CLI 能展示和结束等待。现有通用 `Command.ExecutionError` 在该版 API 映射为 500，不能声称只加一个条件就已完成所有反馈适配。不得重置 busy/retry 会话。
3. 对同名覆盖、普通/引号/多行/注入参数、空白、其他命令、TUI、CLI/API、busy/retry、事件不可用进行真实回归。不得靠模型口头拒绝证明前置拒绝。

无需改权限、Build/Reviewer、Provider、模型、Git hooks 或签名，无需新增 Runtime、MCP、插件、Agent。若公司构建已有等价前置能力，先测现有能力即可，不修改宿主。本包不会自动应用任何宿主改动。

## 先运行独立安全核验

要求 Python 3.9+ 和公司已有的**原生** `opencode.exe`；不要填写 npm 的 `.ps1`/`.cmd` 启动 shim。先核对实际路径、`--version` 与 `debug paths config`；不运行升级/安装命令。V2 客户端可能使用共享后台服务，因此正常 TUI 显示的版本不等于服务端版本；本探针显式启动独立本机服务并读取 `/api/info`。

解压本核验包，PowerShell 在包根目录运行：

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

样例使用 V2 的 `/api/*` 合约，绝不复用 V1 `/session` 探针；要求 CLI 与服务端均为 2.0.27，其他版本输出/路径隔离不符即停止。Command/Skill 只在自动清理的新临时配置中短暂成为可发现文件。工具权限 deny，无模型凭据，无 Git 提交，标记只写测试新目录，不读取业务配置；显式覆盖 XDG/配置/测试 home，不改用户 HOME，不连接共享服务。临时 loopback 服务退出即清理。本地代理设为不可用用于阻断普通代理出站，但不是网络沙箱承诺；企业验收仍依照现有网络策略。

退出码 **1 是发现阻塞，不是通过**；2 是探针不完整/不兼容；0 仅表示这组 API 检查通过，实际 TUI 与完整 S01—S08 仍须验收。无参数/空白检查只验证进入既有提示路径，不是中文提交功能通过。不要上传含真实 Provider 凭据的 `debug config` 输出。

## 安装位置、备份与卸载

**以下恢复步骤仅在公司目标构建通过 S01—S08 并复审后执行；目前未满足。安全核验阶段只运行上述隔离探针，不手工安装候选。**

1. 使用现有获准的测试用户配置/独立测试仓库。关闭对应 TUI/独立服务；通过既有管理方式暂停本模块 TeamAI 同步。不要更改共享服务配置、模型或 Reviewer。
2. 获取 `opencode debug paths config` 的真实目录。默认全局布局通常为 `~/.config/opencode`，Windows 通常为 `$HOME\.config\opencode`；不硬编码 `%APPDATA%`。全局目标只有 `<真实config>\commands\codea-commit.md` 与 `<真实config>\skills\codea-commit\SKILL.md`。项目级仅允许测试仓库的 `.opencode/commands/codea-commit.md` 与 `.opencode/skills/codea-commit/SKILL.md`；不要同时安装两种范围。检查同名 JSON、项目、兼容 Skill 来源及 TeamAI 显式源，确认不会覆盖目标。
3. 在**所有扫描目录之外**新建带唯一名称的备份目录（例如 `$env:TEMP\codea-commit-backup-<GUID>`）。记录原目标是否存在、原绝对路径和 SHA256。若原 Command 存在，复制该文件到备份；若原 Skill 目录存在，完整复制该单个 Skill 目录到备份。逐文件核对摘要后，才将这两个原目标移出扫描目录。不存在也要记录“不存在”。不改整个 commands/skills，不覆盖 `opencode.json(c)`。
4. 从 `candidates/codea-commit.md.txt` 复制为上述 Command 目标；将 `candidates/SKILL.md.txt` 复制为上述 Skill 目标。先创建所需父目录，并逐项检查目标不存在再复制。保存新安装两文件的摘要；任一步失败即停止，从备份恢复已变更的单个目标。不要把本 README、核验脚本或整个测试包同步到全局配置。
5. 在新的独立样例仓库启动现有 TUI，确认来源/摘要、路由现有 Build、首工具 `skill({"id":"codea-commit"})`、原生权限提示及真实提交结果。记录 TUI 和实际服务端版本；关键路径 C01—C03 每项三次。TeamAI 本版能否分发 Command 仍 PENDING；先分别核对 Skill 与 Command 的最终落盘路径/摘要，不猜同步命令。
6. **卸载/回退：**关闭对应测试会话并暂停本模块同步。对照安装记录核对当前两个文件的摘要；若已被他人更新，先备份新内容再处理。只移走本次安装的 `codea-commit.md` 与单个 `skills/codea-commit/` 目录，恢复备份中原有两个目标；原来不存在则保持不存在。重启后核对本次入口已消失或恢复为原定义。不要卸载 OpenCode，不删整个配置/技能目录，不改其他能力。

## 独立提交样例

包内 `tests/fixtures/codea-commit/setup.py`、`verify.py`、`test_fixtures.py` 是自包含标准库样例，不依赖 Codea Runtime、外部服务或模型。Git 写操作只发生在新指定的测试仓库；配置为该仓库 local。先读同目录 README 的场景提示及边界。安全阻塞解除前只运行 fixture 自检，不安装提交入口。

```powershell
python .\tests\fixtures\codea-commit\test_fixtures.py
$caseRoot = Join-Path $env:TEMP ('codea-C02-' + [guid]::NewGuid().ToString('N'))
python .\tests\fixtures\codea-commit\setup.py C02 $caseRoot
# 安全验收通过后，才在输出 repo 的 TUI 输入 /codea-commit。
# manifest 位于仓库外，不提供给模型；禁止用普通 Git 手工提交冒充产品验收。
python .\tests\fixtures\codea-commit\verify.py ($caseRoot + '.manifest.json') $caseRoot
```

C02 专门检查同文件 A 已暂存、B 未暂存时只提交 A，保留 B 和无关文件。完整 C01—C16、hooks/签名失败、范围不明、提交后真实结果等规则沿用仓库原验收表。验证器通过不证明中文说明质量或模型确实加载了 Skill，需保留真实 TUI 工具轨迹。
