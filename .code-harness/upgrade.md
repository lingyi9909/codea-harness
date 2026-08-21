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

把 `.code-harness-upgrade/` 复制到业务项目根目录，然后执行：

```text
harness upgrade
```

旧 Harness 尚无 `harness upgrade` 意图时，读取：

```text
.code-harness-upgrade/upgrade.md
```

再执行升级。

> ⚠ **不要使用 GitHub `Code → Download ZIP` 或 `git clone` 得到的源码目录进行安装或升级。**
>
> GitHub Source Code 不包含正式 Windows Release 才会注入的：
>
> - `bin/codea-harness-tools.exe`
> - `bin/ast-grep.exe`
>
> 安装和升级必须使用 `package-windows-x64` 生成的正式离线包。

## 执行步骤

1. 读取新版 `.code-harness-upgrade/AGENTS.md`。
2. 读取新版 `.code-harness-upgrade/agents/orchestrator.md` 的 `harness upgrade`。
3. 调用 `.code-harness-upgrade/bin/codea-harness-tools.exe upgrade` 所实现的受控 `upgrade_harness`。
4. Upgrade Preflight 校验正式包完整性；缺失 required files 时一次性列出所有缺失项并返回 `MANUAL_ACTION_REQUIRED`，0 文件修改。
5. 根据 `UpgradeResult` 输出结果。

## 约束

- 所有升级文件事务必须由 Tool Runtime 完成。
- 不修改既有 staged replace、Windows running-exe replacement、rollback 核心语义。
- `harness.yaml`、`project.md`、`database.yaml`、`runs/**` 是 Project State，升级时保护。
- `database.yaml` 必须 byte-for-byte 保持不变。
- `harness.yaml` 只允许执行新版登记的确定性 Config Migration；AI 不得猜配置。
- migration 后必须使用新版 Schema 校验；失败完整回滚。
- 不联网、不 `git pull`、不自动重新 init。
