---
name: fix-bug
description: 针对已确认生产缺陷设计 exact unified diff，经精确 planId 审批后交给 Controlled Runtime 原子应用。
version: 2
agent: fix-agent
tools:
  - read_code
output_schema: .code-harness/contracts/fix-plan.schema.json
---

# 修复已确认的缺陷

## 目标

根据用户选定的 Finding 或 `PRODUCTION_CODE_ERROR` Diagnosis 设计最小修复。审批绑定完整 patch identity；正式写入只允许 Controlled Runtime Apply Safety Gate。

## 前置条件

- 有明确 Finding/Diagnosis 和当前源码 Evidence。
- 未批准前只读。

## 执行步骤

1. 使用 `read_code` 读取受影响文件完整内容，确认根因。
2. 只设计解决根因的最小改动，不修改测试、不弱化校验、不重构无关代码。
3. 对每个将修改的当前文件 exact bytes 计算 `baseSha256`。
4. 生成完整 UTF-8 `unifiedDiff`，其 touched file set 必须与计划 `files[]` 一致；计算 exact bytes 的 `diffSha256`。
5. 输出通过 `.code-harness/contracts/fix-plan.schema.json` 的 Fix Plan：

```json
{
  "fixPlanId":"fix-plan-001",
  "rootCause":"...",
  "changes":[{"file":"src/main/java/A.java","reason":"...","change":"..."}],
  "verification":["..."],
  "unifiedDiff":"--- a/src/main/java/A.java\n+++ b/src/main/java/A.java\n...",
  "diffSha256":"<sha256>",
  "files":[{"path":"src/main/java/A.java","baseSha256":"<sha256>"}]
}
```

6. 请求用户精确回复 `批准 <fixPlanId>`。模糊肯定不算批准。
7. plan 自审批后任何变更（包括空白导致 unifiedDiff bytes 变化）都会改变 `diffSha256`，原审批失效。
8. 批准后生成 `.code-harness/runs/<runId>/requests/apply.json`，其结构通过 `apply-request.schema.json`：

```text
planType=FIX
planId=<approved fixPlanId>
unifiedDiff=<approved exact diff>
diffSha256=<approved hash>
files[].baseSha256=<approved base hashes>
```

9. 调用：

```text
.code-harness/bin/codea-harness-tools apply --input .code-harness/runs/<runId>/requests/apply.json
```

10. 只有 Runtime 返回 `APPLIED` 且生成：

```text
.code-harness/runs/<runId>/evidence/apply/<fixPlanId>.json
```

并通过 `apply-result.schema.json`，才报告正式应用完成。
11. `BASE_CHANGED`、`PLAN_ALREADY_APPLIED`、diff/hash/path/patch validation failure 或 rollback → STOP，不得使用 direct host write 兜底。

## Runtime 不变量

- FIX 只能写 `harness.yaml.write.allowedProductionPaths`。
- `deniedPaths` 与 Framework Managed `.code-harness/**` 优先拒绝。
- Runtime 独立重算 `diffSha256` 与原文件 hash。
- patch touched set 必须与声明 `files[]` 完全一致。
- 多文件操作失败必须 rollback；部分应用不算成功。
- success evidence 是正式 apply 的唯一完成证明。

## 输出

- Fix Plan（带 `unifiedDiff / diffSha256 / files[].baseSha256`）
- Runtime Apply evidence path

## 禁止行为

- 不得在审批前修改文件。
- 不得调用旧式 `apply_approved_patch(fixPlanId, changes)` 作为正式应用。
- 不得使用 arbitrary write_file/direct host write 绕过 Runtime。
- 不得修改测试、执行测试、Shell、commit/push/PR。
