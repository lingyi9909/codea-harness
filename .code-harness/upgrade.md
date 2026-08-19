# Codea Harness 升级入口

当旧 Harness 尚无 `harness upgrade` 意图时，将新版 `.code-harness` 复制到业务项目根目录并命名为 `.code-harness-upgrade/`，然后说：

```text
读取 .code-harness-upgrade/upgrade.md，执行升级
```

## 执行步骤

1. 读取新版 `.code-harness-upgrade/AGENTS.md`。
2. 读取新版 `.code-harness-upgrade/agents/orchestrator.md` 的 `harness upgrade`。
3. 调用新版 `.code-harness-upgrade/bin/codea-harness-tools.exe upgrade` 所实现的 `upgrade_harness` 受控工具。
4. 根据 `UpgradeResult` 输出结果。

## 约束

- 所有升级文件事务必须由 Tool Runtime 完成。
- `project.md`、`runs/**` 保持不变。
- `harness.yaml` 只允许执行新版登记的确定性 Config Migration；AI 不得猜配置。
- migration 后必须使用新版 Schema 校验；失败完整回滚。
- 不联网、不 `git pull`、不自动重新 init。
