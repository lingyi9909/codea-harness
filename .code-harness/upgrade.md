# Codea Harness 升级入口

## 必须使用正式 Windows Release

下载正式升级包：

```text
codea-harness-<version>-windows-x64-upgrade.zip
```

解压后应直接得到：

```text
.code-harness-upgrade/
├── VERSION
├── RELEASE-MANIFEST.json
├── bin/
│   ├── codea-harness-tools.exe
│   └── ast-grep.exe
└── ...
```

把 `.code-harness-upgrade/` 复制到业务项目根目录。

> ⚠ **不要使用 GitHub `Code → Download ZIP` 或 `git clone` 得到的源码目录进行安装或升级。**
>
> GitHub Source Code 不包含正式 Windows Release 才会注入的：
>
> - `bin/codea-harness-tools.exe`
> - `bin/ast-grep.exe`
>
> 安装和升级必须使用 `package-windows-x64` 生成的正式离线包。

## 固定升级 Bootstrap

无论当前 Harness 版本是否已经支持 `harness upgrade`，升级统一从下面这个入口开始：

```text
读取 .code-harness-upgrade/upgrade.md，执行升级
```

不要直接跳过本文件调用 Runtime。旧 Runtime 无法可靠判断“新版 Runtime 自己缺失”这一类错误，因此正式包检查必须先由 Agent 做只读 bootstrap/preflight。

## Agent 层 Package Preflight（必须先执行）

在调用任何 Runtime、创建 backup/stage、修改 `.code-harness/` 之前，先只读检查以下 required source：

```text
.code-harness-upgrade/VERSION
.code-harness-upgrade/RELEASE-MANIFEST.json
.code-harness-upgrade/AGENTS.md
.code-harness-upgrade/bootstrap.md
.code-harness-upgrade/upgrade.md
.code-harness-upgrade/harness.template.yaml
.code-harness-upgrade/project.template.md
.code-harness-upgrade/agents/
.code-harness-upgrade/skills/
.code-harness-upgrade/contracts/
.code-harness-upgrade/tools/
.code-harness-upgrade/contracts/harness-config.schema.json
.code-harness-upgrade/bin/codea-harness-tools.exe
.code-harness-upgrade/bin/ast-grep.exe
```

所有缺失项必须一次性收集后再输出，不得发现一个报一个。

同时确认 `RELEASE-MANIFEST.json` 至少声明：

```text
version
platform = windows
arch = x64
runtime = codea-harness-tools.exe
astGrepVersion
```

如果任一 required source 缺失，立即输出并停止：

```text
MANUAL_ACTION_REQUIRED

升级包不完整：
missing: <全部缺失项，一次列完>

如果缺少 bin/codea-harness-tools.exe 或 bin/ast-grep.exe：
检测到的目录可能来自 GitHub Source Code，而不是正式 Windows Release。
请使用：codea-harness-<version>-windows-x64-upgrade.zip

解压后项目根目录必须存在：
.code-harness-upgrade/VERSION
.code-harness-upgrade/RELEASE-MANIFEST.json
.code-harness-upgrade/bin/codea-harness-tools.exe
.code-harness-upgrade/bin/ast-grep.exe
```

此分支硬规则：

```text
不调用任何 Runtime
不创建 stage/backup
不修改旧 .code-harness/
STOP
```

## Package Preflight 通过后的执行步骤

1. 读取新版 `.code-harness-upgrade/AGENTS.md`。
2. 读取新版 `.code-harness-upgrade/agents/orchestrator.md` 的 `harness upgrade` 约束。
3. 调用**当前已安装**的 `.code-harness/bin/codea-harness-tools.exe upgrade` 执行既有受控升级事务。
4. Runtime 再执行自己的 required-source preflight；因为 Agent 层已经完整检查，正常路径不应再出现 package 缺失。
5. Package 完整后进入既有 Upgrade transaction。
6. 根据 `UpgradeResult` 输出结果。

这里故意继续使用当前已安装 Runtime 执行事务，而不是执行 `.code-harness-upgrade/bin/codea-harness-tools.exe`：这样保持既有 Windows running-exe staged replacement 与成功后删除 `.code-harness-upgrade/` 的语义不变。

## 约束

- 所有升级文件事务必须由 Tool Runtime 完成。
- Agent 层只允许做上述只读 package bootstrap/preflight；不得自行复制、覆盖、删除 Harness 文件。
- 不修改既有 staged replace、Windows running-exe replacement、rollback 核心语义。
- `harness.yaml`、`project.md`、`database.yaml`、`runs/**` 是 Project State，升级时保护。
- `database.yaml` 必须 byte-for-byte 保持不变。
- `harness.yaml` 只允许执行新版登记的确定性 Config Migration；AI 不得猜配置。
- migration 后必须使用新版 Schema 校验；失败完整回滚。
- 不联网、不 `git pull`、不自动重新 init。
