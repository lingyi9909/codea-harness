---
name: fix-bug
description: 针对已确认生产缺陷设计 exact unified diff，经审批前 Runtime seal 与精确 planId 批准后原子应用。
version: 2
agent: fix-agent
tools:
  - read_code
output_schema: .code-harness/contracts/fix-plan.schema.json
---

# 修复已确认的缺陷

## 目标

根据用户选定的 Finding 或 `PRODUCTION_CODE_ERROR` Diagnosis 设计最小修复。人工批准必须绑定审批前由 Controlled Runtime 封存的 exact patch identity；正式写入只允许 Runtime Apply Safety Gate。

## 前置条件

- 有明确 Finding/Diagnosis 和当前源码 Evidence。
- 未批准前只读业务文件。

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

6. **审批前**把上述 exact identity 原样写入 `.code-harness/runs/<runId>/requests/apply.json`，并通过 `apply-request.schema.json`：

```text
planType=FIX
planId=<fixPlanId>
unifiedDiff=<exact diff>
diffSha256=<exact hash>
files[].path/baseSha256=<exact file identity>
```

7. 在提示用户批准之前调用：

```text
.code-harness/bin/codea-dcep-tools.exe seal-apply --input .code-harness/runs/<runId>/requests/apply.json
```

只有生成：

```text
.code-harness/runs/<runId>/sealed-plans/<fixPlanId>.json
```

才允许请求人工审批。sealed snapshot 是 Runtime-owned approval baseline，固定 `planId / planType / unifiedDiff exact bytes / diffSha256 / files[].path / files[].baseSha256`。
8. 请求用户精确回复 `批准 <fixPlanId>`。模糊肯定不算批准。
9. 批准后**不得重新生成 request**。只能使用审批前 sealed 的同一份 `.code-harness/runs/<runId>/requests/apply.json` 调用：

```text
.code-harness/bin/codea-dcep-tools.exe apply --input .code-harness/runs/<runId>/requests/apply.json
```

10. Runtime 必须先逐字段比对 sealed snapshot。即使 Patch B 的 `diffSha256` 与 Patch B 自己完全自洽，只要它不同于用户批准前 sealed 的 Patch A，就必须 `APPROVAL_IDENTITY_MISMATCH` / STOP / 0 写入。
11. 只有 Runtime 返回 `APPLIED` 且生成：

```text
.code-harness/runs/<runId>/evidence/apply/<fixPlanId>.json
```

并通过 `apply-result.schema.json`，才报告正式应用完成。
12. `SEALED_PLAN_NOT_FOUND`、`APPROVAL_IDENTITY_MISMATCH`、`BASE_CHANGED`、`PLAN_ALREADY_APPLIED`、diff/hash/path/patch validation failure 或 rollback → STOP，不得使用 direct host write 兜底。

## Runtime 不变量

- FIX 只能写 `harness.yaml.write.allowedProductionPaths`。
- `.git/**` 与 `.code-harness/**` 是 Runtime hard-deny，不能被 `harness.yaml` 的 `allowedProductionPaths=["**"]` 或空 `deniedPaths` 覆盖。
- 用户 `deniedPaths` 仍优先于普通 allowlist。
- `apply --input` 必须先确认路径是 `.code-harness/runs/<runId>/requests/*.json`，再读取 JSON；body `runId` 必须等于 path runId。
- Runtime 独立重算 `diffSha256` 与原文件 hash。
- Runtime apply 必须与审批前 sealed snapshot 完整比对 approval identity。
- patch touched set 必须与声明 `files[]` 完全一致。
- 多文件操作失败必须 rollback；部分应用不算成功。
- success evidence 是正式 apply 的唯一完成证明。

## 输出

- Fix Plan（带 `unifiedDiff / diffSha256 / files[].baseSha256`）
- 审批前 `.code-harness/runs/<runId>/sealed-plans/<fixPlanId>.json`
- Runtime Apply evidence path

## 禁止行为

- 不得在审批前修改业务文件。
- 不得在审批后重新生成不同 patch/request 并沿用旧 planId。
- 不得调用旧式 `apply_approved_patch(fixPlanId, changes)` 作为正式应用。
- 不得使用 arbitrary write_file/direct host write 绕过 Runtime。
- 不得修改测试、执行测试、Shell、commit/push/PR。
