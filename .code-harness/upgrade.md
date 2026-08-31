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
│   ├── codea-dcep-tools.exe
│   └── ast-grep.exe
└── ...
```

把 `.code-harness-upgrade/` 复制到业务项目根目录。

> ⚠ **不要使用 GitHub `Code → Download ZIP` 或 `git clone` 得到的源码目录进行安装或升级。**
>
> GitHub Source Code 不包含正式 Windows Release 才会注入的：
>
> - `bin/codea-dcep-tools.exe`
> - `bin/ast-grep.exe`
>
> 安装和升级必须使用 `package-windows-x64` 生成的正式离线包。

## 固定升级 Bootstrap

无论当前 Harness 版本是否已经支持 `harness upgrade`，升级统一从下面这个入口开始：

```text
读取 .code-harness-upgrade/upgrade.md，执行升级
```

不要直接跳过本文件调用 Runtime。正式包检查必须先由 Agent 做只读 bootstrap/preflight；Package Preflight 通过后才允许进入 Runtime 升级事务。

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
.code-harness-upgrade/contracts/chain.schema.json
.code-harness-upgrade/contracts/chain-validation-result.schema.json
.code-harness-upgrade/templates/chain.template.yaml
.code-harness-upgrade/skills/discover-chain/SKILL.md
.code-harness-upgrade/skills/validate-chain/SKILL.md
.code-harness-upgrade/bin/codea-dcep-tools.exe
.code-harness-upgrade/bin/ast-grep.exe
```

所有缺失项必须一次性收集后再输出，不得发现一个报一个。

同时确认 `RELEASE-MANIFEST.json` 至少声明：

```text
version
platform = windows
arch = x64
runtime = codea-dcep-tools.exe
astGrepVersion
```

如果任一 required source 缺失，立即输出并停止：

```text
MANUAL_ACTION_REQUIRED

升级包不完整：
missing: <全部缺失项，一次列完>

如果缺少 bin/codea-dcep-tools.exe 或 bin/ast-grep.exe：
检测到的目录可能来自 GitHub Source Code，而不是正式 Windows Release。
请使用：codea-harness-<version>-windows-x64-upgrade.zip

解压后项目根目录必须存在：
.code-harness-upgrade/VERSION
.code-harness-upgrade/RELEASE-MANIFEST.json
.code-harness-upgrade/bin/codea-dcep-tools.exe
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
3. 调用新版升级包中的 `.code-harness-upgrade/bin/codea-dcep-tools.exe upgrade` 执行受控升级事务。
4. 新版 Runtime 再执行自己的 required-source preflight；因为 Agent 层已经完整检查，正常路径不应再出现 package 缺失。
5. Package 完整后进入既有 Upgrade transaction，并执行新版登记的确定性 Config Migration（如有）。
6. Windows 下如果正在执行的新版 Runtime 位于 `.code-harness-upgrade/bin/`，Runtime 会在成功应用 Framework 后先把自身可执行文件移动到同卷、升级目录外的临时位置，再消费 `.code-harness-upgrade/`；Agent 不参与文件事务。
7. 根据 `UpgradeResult` 输出结果。

必须使用新版升级包 Runtime 的原因是：registered migration 属于目标版本 Runtime。旧版本 Runtime 不会预先拥有未来版本新增的确定性 migration；正式升级必须由目标版本 Runtime 负责 migration + transaction，才能保证 Framework 与配置版本一起升级。

## 1.4.0 → 1.5.0 Project State 契约

1.5.0 不新增 `harness.yaml` 配置 migration。升级只替换 Framework Managed 内容并安装 Chain Framework，因此以下 Project State 在 1.4.0 → 1.5.0 升级中必须 **byte-for-byte** 保持：

```text
harness.yaml
project.md
database.yaml
runs/**
chains/**
```

其中 `chains/**` 是开发者维护的长期业务知识，永远不是 Framework Managed：

- managed replace 不得删除 `chains/**`；
- `removedFiles` 不得出现 `chains/**`；
- 正式 install/upgrade package 不得携带任何业务 `chains/*.yaml` 实例；
- 即使异常 source tree 出现同名业务 Chain，也不得覆盖项目已有 Chain；
- 1.4.0 → 1.5.0 Windows live upgrade 必须在升级前后对 sentinel Chain 做 SHA256 比对。

新版本只允许安装 Framework Chain 能力，例如：

```text
contracts/chain.schema.json
contracts/chain-validation-result.schema.json
templates/chain.template.yaml
skills/discover-chain/SKILL.md
skills/validate-chain/SKILL.md
tools-runtime/internal/chain/**
```

升级完成后，正式验收还必须从已安装的新 Runtime 调用 `chain validate` 的确定性参数错误路径，证明安装的 Runtime 确实包含 1.5 Chain 子命令。

## 约束

- 所有升级文件事务必须由 Tool Runtime 完成。
- Agent 层只允许做上述只读 package bootstrap/preflight；不得自行复制、覆盖、删除 Harness 文件。
- 不修改既有 staged replace、Windows running-exe replacement、rollback 核心语义。
- `.code-harness-upgrade/bin/codea-dcep-tools.exe upgrade` 是 Package Preflight 通过后的唯一升级事务入口。
- `harness.yaml`、`project.md`、`database.yaml`、`runs/**`、`chains/**` 是 Project State，升级时保护。
- `database.yaml` 与 `chains/**` 必须 byte-for-byte 保持；1.4.0 → 1.5.0 因没有新 config migration，`harness.yaml` 也必须 byte-for-byte 保持。
- `harness.yaml` 只有在目标版本登记了确定性 Config Migration 时才允许 Runtime 修改；AI 不得猜配置。
- migration 后必须使用新版 Schema 校验；失败完整回滚。
- 不联网、不 `git pull`、不自动重新 init。
