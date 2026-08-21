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

不要直接从旧版 `.code-harness/bin/codea-harness-tools.exe` 开始升级。原因是旧 Runtime 无法可靠判断“新版 Runtime 自己缺失”这一类错误。

## Agent 层 Package Preflight（必须先执行）

在调用任何 Runtime、创建 backup/stage、修改 `.code-harness/` 之前，先只读检查：

```text
.code-harness-upgrade/VERSION
.code-harness-upgrade/RELEASE-MANIFEST.json
.code-harness-upgrade/bin/codea-harness-tools.exe
.code-harness-upgrade/bin/ast-grep.exe
```

同时确认 `RELEASE-MANIFEST.json` 至少声明：

```text
version
platform = windows
arch = x64
runtime = codea-harness-tools.exe
astGrepVersion
```

如果任一基础项缺失，立即输出并停止：

```text
MANUAL_ACTION_REQUIRED

升级包不完整：
missing: <全部缺失项，一次列完>

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
2. 读取新版 `.code-harness-upgrade/agents/orchestrator.md` 的 `harness upgrade`。
3. 调用新版 `.code-harness-upgrade/bin/codea-harness-tools.exe upgrade` 所实现的受控 `upgrade_harness`。
4. Runtime 继续执行完整 required-source preflight；若还有缺失项，一次性列出并返回 `MANUAL_ACTION_REQUIRED`，0 文件修改。
5. Package 完整后才进入既有 Upgrade transaction。
6. 根据 `UpgradeResult` 输出结果。

## 约束

- 所有升级文件事务必须由 Tool Runtime 完成。
- Agent 层只允许做上述只读 package bootstrap/preflight；不得自行复制、覆盖、删除 Harness 文件。
- 不修改既有 staged replace、Windows running-exe replacement、rollback 核心语义。
- `harness.yaml`、`project.md`、`database.yaml`、`runs/**` 是 Project State，升级时保护。
- `database.yaml` 必须 byte-for-byte 保持不变。
- `harness.yaml` 只允许执行新版登记的确定性 Config Migration；AI 不得猜配置。
- migration 后必须使用新版 Schema 校验；失败完整回滚。
- 不联网、不 `git pull`、不自动重新 init。
