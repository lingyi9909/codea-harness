# Codea Harness 升级入口

本文件是「旧版本首次升级」的入口。当当前 `.code-harness/` 还没有 `harness upgrade` 意图时（例如 1.0.0 → 1.1.0），用它完成升级。

## 使用方式

先把新版 `.code-harness` 目录复制到业务项目根目录，命名为 `.code-harness-upgrade/`，然后说：

```text
读取 .code-harness-upgrade/upgrade.md，执行升级
```

## 执行步骤

1. 读取 `.code-harness-upgrade/AGENTS.md`，了解 Harness 通用规则与安全约束。
2. 读取 `.code-harness-upgrade/agents/orchestrator.md`，找到 `harness upgrade` 意图。
3. 调用 `.code-harness-upgrade/skills/upgrade-harness/SKILL.md`，其内部调用 `upgrade_harness` 工具完成原子升级。
4. 读取 `UpgradeResult`，输出结果。

## 约束

- 所有文件操作必须交给 `upgrade_harness` 工具，不得自行复制、移动、删除文件。
- 项目配置（`harness.yaml`、`project.md`）和运行记录（`runs/**`）保持不变。
- 升级失败自动回滚。
- 不联网、不 `git pull`、不做配置迁移、不重新执行 `harness init`。
