---
name: upgrade-harness
description: 使用项目根目录 .code-harness-upgrade/ 中的新版 Harness 安全升级当前 .code-harness，项目配置与运行记录保持不变。
version: 1
agent: orchestrator
tools:
  - upgrade_harness
output_schema: .code-harness/contracts/upgrade-result.schema.json
---

# 升级 Harness

## 目标

使用项目根目录 `.code-harness-upgrade/` 中的新版 Harness，原子升级当前 `.code-harness/`。项目配置（`harness.yaml`、`project.md`）和运行记录（`runs/**`）保持不变；升级失败自动回滚。

## 适用场景

- 用户把新版 `.code-harness` 目录复制到业务项目根目录并命名为 `.code-harness-upgrade/` 后，说 `harness upgrade`
- 需要把 Harness 从旧版本升级到新版本

## 不适用场景

- 没有 `.code-harness-upgrade/` 目录
- 尚未初始化（不存在 `.code-harness/`）

## 输入

- 升级包目录 `.code-harness-upgrade/`（默认）
- 当前 Harness 目录 `.code-harness/`（默认）

## 允许使用的工具

- `upgrade_harness`——唯一升级工具，负责完整的 Preflight、备份、更新、校验、回滚事务

## 前置条件

- 项目根目录存在 `.code-harness-upgrade/`
- 项目根目录存在 `.code-harness/`

## 执行步骤

1. **检查升级包**：确认项目根目录存在 `.code-harness-upgrade/`。不存在 → 停止并提示用户先复制新版 Harness 目录。
2. **调用 upgrade_harness**：调用 `upgrade_harness(sourceDir, targetDir)`，使用默认参数（`sourceDir = .code-harness-upgrade/`，`targetDir = .code-harness/`）。本 Skill 不自行复制、移动或删除任何文件。
3. **读取 UpgradeResult**：读取返回的结构化结果（`status`、`fromVersion`、`toVersion`、`updatedFiles`、`removedFiles`、`preservedFiles`、`rollbackPerformed`、`errors`）。
4. **输出用户结果**：根据 `status` 呈现：
   - `UPGRADED` → 报告新版本、更新/删除/保留的文件
   - `ALREADY_UP_TO_DATE` → 报告已是当前版本，无变化
   - `MANUAL_ACTION_REQUIRED` → 报告原因（降级 / 升级包不完整 / VERSION 非法）
   - `UPGRADE_FAILED` → 报告 `rollbackPerformed` 与 `errors` 中的不兼容字段

## 输出

必须通过 `.code-harness/contracts/upgrade-result.schema.json` 校验的 `UpgradeResult`，以及面向用户的摘要。

## 停止条件

- `.code-harness-upgrade/` 不存在 → 停止并提示用户
- `status` 为 `ALREADY_UP_TO_DATE` / `MANUAL_ACTION_REQUIRED` / `UPGRADE_FAILED` → 输出结果后停止
- `status` 为 `UPGRADED` → 输出结果后停止

## 禁止行为

- 不得自行复制、移动、删除文件——所有文件操作必须交给 `upgrade_harness`
- 不得猜测版本、不得联网、不得执行 `git fetch` / `git pull`
- 不得做配置迁移（不猜新配置、不自动重新 init、不偷偷加字段）
- 不得触碰业务源码、测试源码、`pom.xml`、根目录 `AGENTS.md`、`harness.yaml`、`project.md`、`runs/**`
- 不得新增其他升级工具或拆分升级步骤
