---
name: upgrade-harness
description: 使用确定性 Tool Runtime 原子升级当前 Harness，并执行已登记的版本化 Config Migration 与新版 Schema 校验。
version: 2
agent: orchestrator
tools:
  - upgrade_harness
output_schema: .code-harness/contracts/upgrade-result.schema.json
---

# 升级 Harness

## 目标

使用 `.code-harness-upgrade/` 中的新版 Harness 原子升级 `.code-harness/`。`project.md` 与 `runs/**` 原样保留；`harness.yaml` 默认保留，但允许由 **Tool Runtime 中已登记、确定性、版本化的 Config Migration** 做最小兼容迁移。AI 不得猜配置。

## 执行

1. 检查 `.code-harness-upgrade/` 与 `.code-harness/` 存在。
2. 调用唯一升级工具 `upgrade_harness(sourceDir, targetDir)`；Skill 不自行复制、删除或编辑文件。
3. Tool Runtime 按以下事务执行：

```text
读取旧 VERSION
→ 读取新 VERSION
→ 校验升级包完整性（含两个 Windows Runtime binary）
→ 计算所需 registered migrations
→ 所有需要人工判断的 migration 先 preflight；无法确定则 0 修改 MANUAL_ACTION_REQUIRED
→ 完整备份
→ 更新 Framework Managed 文件
→ 执行 registered migrations
→ 使用新版 harness-config.schema.json 校验 harness.yaml
→ PASS: 最后写 VERSION，删除备份，UPGRADED
→ FAIL: 完整回滚，UPGRADE_FAILED + rollbackPerformed=true
```

## 1.1.1 已登记 Migration

### `add-review-config-v1`

仅当旧 `harness.yaml` **完全没有顶层 `review:`** 时执行：

```yaml
review:
  baseRef: <detected>
  includeWorkingTree: true
```

`baseRef` 严格按本地 refs：

1. `origin/HEAD` 指向
2. `origin/master`
3. `origin/main`
4. `origin/develop`
5. `master`
6. `main`
7. `develop`

均不存在 → `MANUAL_ACTION_REQUIRED`，当前 Harness 0 修改，并提示用户显式配置。**已有 `review` 时整个 block 字节级保持，不重新识别、不覆盖。**

## 禁止行为

- 禁止 AI 猜 module/profile/path/baseRef。
- 禁止自动重新 `harness init`。
- 禁止未登记 migration 修改 `harness.yaml`。
- 禁止联网、`git fetch` / `git pull`。
- 禁止 Skill 绕过 Tool Runtime 自行做文件事务。
