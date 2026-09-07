# Codea Harness 1.6.4 Entrypoint Inventory Scope Amendment

## 1. Authority

本文件是以下正式设计的补充修订：

`docs/superpowers/specs/2026-09-07-codea-harness-1.6.4-analysis-certification-performance-hotfix-design.md`

如果本文件与原设计中 Task 1 的 Entrypoint Inventory Scope、Base Scan Scope、Performance Gate 描述存在冲突，以本文件为准。

本修订不改变 1.6.4 的版本范围，也不增加新的产品功能。它只把实际环境暴露出的一个更前置性能根因提升为 Task 1 的硬门禁。

---

## 2. 新的实际现象

实际项目执行：

```text
harness review
→ analysis snapshot
```

Runtime canonical snapshot 已正确收敛到约：

```text
4–5 changed files
```

该部分符合预期。

但后续：

```text
analysis certify
→ BuildEntrypointInventory
→ ENTRYPOINT_BASE_SCAN
```

实际执行仍出现 Base 侧对整个项目/整个 Java source tree 的扫描，最终导致 certify timeout。

因此本轮性能问题不能只描述为：

```text
per-file × per-pattern ast-grep process amplification
```

还必须增加一个更优先的根因：

```text
Canonical ChangeSet Scope 已经收敛
但 Entrypoint Inventory 执行层再次扩大 Scan Scope
```

即：

**ENTRYPOINT INVENTORY SCOPE WIDENING**。

---

## 3. 语义判断

Entrypoint Inventory 的职责不是：

```text
建立整个项目所有 Controller / Endpoint 的全局索引
```

它的职责是为当前 Runtime Canonical ChangeSet 建立 completeness authority，证明：

```text
本次变化涉及的 changed production Java 中，
所有需要成为 Controller/Endpoint obligation 的事实都没有遗漏。
```

因此 Entrypoint Inventory 的合法源码边界必须完全从当前 certified candidate snapshot 派生。

对于 FULL Review，不允许因为 intent=FULL 就把 Inventory Scan Scope 扩展为：

```text
.
src/main/java
<module>/src/main/java
repository root
module root
任意 glob
整个项目 Java tree
```

FULL 的含义是最终 Review coverage 覆盖完整 Canonical ChangeSet，**不是 Entrypoint Inventory 可以扫描整个 repository**。

---

## 4. 当前源码意图与实际运行偏差

当前主干实现的设计意图本身是 snapshot-bounded：

```text
BuildEntrypointInventory
→ for each snapshot.Files
→ only production Java
→ scanner.Current(changed.Path)
→ scanner.Base(snapshot, changed.Path)
```

因此如果实际 release/runtime 日志表现为：

```text
ENTRYPOINT_BASE_SCAN
→ full project scan
```

研发必须把它视为一个独立 blocker：

```text
代码层存在 scope widening
或 runner/invocation 层丢失 exact-file scope
或 release binary 的执行路径与当前源码假设不一致
```

不能只通过 batch/multi-rule 优化掩盖。

---

# 5. Task 1 修订后的正式顺序

原 Task 1：

```text
Batch Entrypoint Inventory
```

修订为两个必须按顺序完成的子阶段：

```text
Task 1A — Snapshot-Bounded Entrypoint Scan Scope
Task 1B — Batch Entrypoint AST Execution
```

**Task 1A 未通过，不得进入 Task 1B 验收。**

原因：

如果 scope 本身是整个项目，那么把 100+ pattern 合并成少量 ast-grep process 只是：

```text
更快地扫描错误范围
```

而不是正确的性能设计。

---

# 6. Task 1A — Snapshot-Bounded Entrypoint Scan Scope

## 6.1 Runtime-owned Scan Plan

建议新增一个内部 Runtime 结构：

```text
EntrypointScanPlan
```

逻辑字段至少包括：

```text
runId
snapshotSha256
mergeBase
currentPaths[]
basePaths[]
currentScopeSha256
baseScopeSha256
```

该 Plan 只能由 Runtime 从已经读取并验证的 canonical snapshot 派生。

Agent、Orchestrator、Reviewer 不得提交或覆盖 `currentPaths/basePaths`。

## 6.2 Current exact-file set

Current side 必须固定为：

```text
snapshot.Files
∩ production Java
∩ current source actually exists
```

正常状态对应：

```text
A
M
```

删除文件 D 不进入 Current scan。

设：

```text
CurrentPaths = { exact changed Java file paths }
```

AST scanner 只能消费该 exact set。

## 6.3 Base exact-file set

Base side 必须固定为：

```text
snapshot.Files
∩ production Java
∩ source exists at snapshot.MergeBase
```

正常状态对应：

```text
M
D
```

新增文件 A 不进入 Base scan。

设：

```text
BasePaths = { exact changed Java file paths }
```

Base scanner 只能读取：

```text
snapshot.MergeBase:<BasePaths[i]>
```

不得为了建立 inventory 枚举 mergeBase 下整个项目 Java tree。

---

## 6.4 禁止自由 scope API

Task 1A 后，Entrypoint Inventory 专用 scanner 不应继续依赖可以传入任意字符串的：

```text
scan(scope string)
```

作为最终 production authority API。

推荐改为：

```text
CurrentBatch(ctx, exactPaths []ExactRepoPath)
BaseBatch(ctx, snapshot, exactPaths []ExactRepoPath)
```

或者 scanner 直接消费 Runtime-owned `EntrypointScanPlan`。

关键要求：

**调用者没有能力把 scope 从 exact files 换成 repo/module/source root。**

---

## 6.5 Runner 层必须再次 fail-closed

即使上层已经给出 exact paths，AST runner 仍必须二次验证。

合法输入：

```text
file1.java
file2.java
file3.java
```

禁止输入：

```text
.
src/main/java
module/src/main/java
repository root
module root
*
**/*.java
任何 directory scope
```

一旦检测到 widening，必须失败：

```text
ENTRYPOINT_SCAN_SCOPE_WIDENED
```

不得 fallback 到 full-project scan。

---

## 6.6 Base temp tree 也必须是 exact-file-only

如果 Task 1B 为 Base batch scan 创建临时 tree，临时目录内容必须只有：

```text
BasePaths
```

例如：

```text
%TEMP%/codea-entrypoint-base-<id>/
  module-a/src/main/java/.../A.java
  module-a/src/main/java/.../B.java
  module-b/src/main/java/.../C.java
```

其中 A/B/C 必须全部来自 `BasePaths`。

禁止使用：

```text
git worktree
完整 checkout
完整 git archive
整个 module source copy
整个 repository copy
```

来建立 Base scan tree。

否则虽然 ast-grep process count 下降，scan bytes 仍会随整个项目增长。

---

## 6.7 Base source 读取

Task 1B 仍按原设计使用：

```text
snapshot.MergeBase
+
git cat-file --batch
```

但 batch request 只能包含 `BasePaths`。

必须验证：

```text
requested object path ∈ BasePaths
```

否则：

```text
ENTRYPOINT_BASE_SOURCE_OUT_OF_SCOPE
```

不得自动读取额外 Java 文件。

---

## 6.8 AST result membership

AST scanner 返回每个 match 后必须验证：

Current：

```text
match.path ∈ CurrentPaths
```

Base：

```text
match.path ∈ BasePaths
```

任何超出 Plan 的结果必须 fail closed：

```text
ENTRYPOINT_SCAN_RESULT_OUT_OF_SCOPE
```

不得静默丢弃后继续生成 COMPLETE inventory。

---

# 7. Task 1B — Batch Entrypoint AST Execution

Task 1B 继续采用原设计的 Two-phase Batch：

每侧：

```text
Phase 1 — Type rules batch scan exact paths
Phase 2 — Method rules batch scan only exact Controller paths
```

当前规则规模保持：

```text
30 Type patterns
78 Method patterns
```

不升级 pinned ast-grep 0.42.1，不改变 matching semantics。

整个 Inventory 正常上限：

```text
Current Type AST process <= 1
Current Method AST process <= 1
Base Type AST process <= 1
Base Method AST process <= 1
```

即：

```text
astGrepProcessCount <= 4
```

且该 process count 不随 repository Java 文件总量增加。

---

# 8. 新的复杂度目标

优化前，如果执行层把 Base scan 扩成全项目，实际复杂度可能接近：

```text
O(repositoryJavaFiles × patterns × process startup)
```

这会导致：

```text
ChangeSet 只有 4–5 files
但 certify 时间仍与整个 repository size 相关
```

Task 1A + Task 1B 后，目标复杂度必须变成：

```text
O(changedProductionJavaBytes + AST matching on exact changed paths)
```

核心 invariant：

> **Entrypoint Inventory runtime cost 必须主要由 Canonical ChangeSet 中的 production Java 数量决定，而不是由整个 repository 的 Java 文件数量决定。**

---

# 9. Task 2 Telemetry 修订

原 `certify-performance.json` 增加 Scope Evidence。

至少记录：

```json
{
  "entrypointScan": {
    "scopeMode": "EXACT_FILES",
    "snapshotChangedFiles": 5,
    "productionJavaFiles": 4,
    "currentRequestedFiles": 3,
    "baseRequestedFiles": 4,
    "currentScannedFiles": 3,
    "baseScannedFiles": 4,
    "unexpectedResultFiles": 0,
    "astGrepProcesses": 4,
    "baseGitBatchProcesses": 1
  }
}
```

建议同时保存：

```text
currentScopeSha256
baseScopeSha256
```

避免 telemetry 只报 count 而无法证明 exact set identity。

该 telemetry 仍为 Runtime-owned performance evidence，不进入正式 certification authority hash。

---

# 10. 新增强制 Regression

## Case S1 — 5 changed files + large unrelated repository

构造 repository：

```text
5000+ unrelated production Java files
```

Canonical ChangeSet：

```text
5 changed files
其中 4 个 production Java
```

执行：

```text
analysis snapshot
analysis certify
```

必须证明：

```text
snapshot.changedFiles == 5
currentRequestedFiles <= 4
baseRequestedFiles <= 4
currentScannedFiles == currentRequestedFiles
baseScannedFiles == baseRequestedFiles
```

并且 AST runner 不得收到：

```text
repo root
module root
src/main/java
```

任何 scope。

## Case S2 — Repository Size Invariance

保持 exact same 5-file ChangeSet。

Fixture A：

```text
100 unrelated Java files
```

Fixture B：

```text
5000 unrelated Java files
```

要求：

```text
Entrypoint requested/scanned file count 完全相同
astGrepProcessCount 完全相同
Base git batch process count 完全相同
```

Wall time 不要求 byte-for-byte 相同，但 Fixture B 不得因为无关项目文件数量增长而出现数量级增长或 timeout。

## Case S3 — Base Scope Exactness

ChangeSet：

```text
A.java M
B.java M
C.java D
D.java A
```

要求：

```text
CurrentPaths = A.java, B.java, D.java
BasePaths    = A.java, B.java, C.java
```

不得扫描其他 Java 文件。

## Case S4 — Controller Annotation Removed

Base：

```java
@RestController
class AController { ... }
```

Current：

```java
class AController { ... }
```

AController 属于 changed file。

要求 Base exact-file scan 仍正确识别旧 Controller obligation。

不得通过 Current 文本 prefilter 把 Base scan 跳过。

## Case S5 — Scope Widen Negative Control

测试 runner 人为收到：

```text
src/main/java
```

必须：

```text
ENTRYPOINT_SCAN_SCOPE_WIDENED
```

0 certified analysis writes。

## Case S6 — Out-of-scope Result Negative Control

模拟 AST runner 对请求：

```text
A.java
B.java
```

返回：

```text
UnrelatedController.java
```

必须：

```text
ENTRYPOINT_SCAN_RESULT_OUT_OF_SCOPE
```

不得发布 COMPLETE inventory。

---

# 11. Windows Performance Gate 修订

原 21/50/100-file Gate 保留，并新增一个更直接的实际问题 Gate。

## Gate P0 — Small ChangeSet / Large Repository

Windows Server 2025 / pinned ast-grep 0.42.1。

Repository：

```text
>= 5000 Java files
```

Canonical ChangeSet：

```text
4–5 changed files
```

其中包含至少：

```text
1 Controller modified
1 Service modified
1 deleted/renamed Java case
```

要求：

```text
analysis certify <= 10s target
CI hard limit <= 15s
```

且：

```text
scanned file set == snapshot-derived exact file set
astGrepProcessCount <= 4
per-file git merge-base == 0
per-file git show == 0
full-project Entrypoint Base Scan == 0
```

这个 Gate 的优先级高于原来的 21/50/100 changed-file scalability Gate，因为它直接复现当前真实问题：

```text
ChangeSet 很小
repository 很大
certify 不应超时
```

---

# 12. Task 3 条件仍保持不变

Snapshot Freshness Fast Path 仍是条件 Task。

只有 Task 1A + Task 1B + Task 2 完成后 telemetry 显示：

```text
Entrypoint Inventory 已明显降到目标范围
但 changeset.Compute freshness recompute 成为主要剩余耗时
```

才进入 Task 3。

不得因为新增 Scope 问题就顺手修改 Snapshot Authority。

---

# 13. 明确禁止方案

以下方案不得作为本问题正式修复：

1. 把 inventory timeout 从 60s 提高到 120s/300s；
2. 保留 full-project Base scan，只合并 ast-grep patterns；
3. 保留 full-project scan，只增加 goroutine 并发；
4. FULL intent 默认扫描整个 repository；
5. 用 `*Controller.java` 文件名替代 AST authority；
6. 用 `@RestController` 文本检索直接决定最终 Controller fact；
7. 为 Base scan checkout/copy 整个 repository；
8. 发现 exact file scan 失败后 fallback 到 `src/main/java`；
9. 为了提前展示 Chain Selection，把 USER_SELECTION 提到 analysis certify 之前。

---

# 14. Task 1 正式验收条件

Task 1 只有同时满足以下两部分才允许通过：

## Scope Correctness

```text
ENTRYPOINT_SCAN_SCOPE_EXACT PASS
ENTRYPOINT_BASE_SCOPE_SNAPSHOT_BOUNDED PASS
ENTRYPOINT_NO_PROJECT_ROOT_SCAN PASS
ENTRYPOINT_NO_MODULE_ROOT_SCAN PASS
ENTRYPOINT_NO_SOURCE_ROOT_SCAN PASS
ENTRYPOINT_OUT_OF_SCOPE_RESULT_REJECT PASS
REPOSITORY_SIZE_INVARIANCE PASS
```

## Batch Performance

```text
ENTRYPOINT_TYPE_BATCH PASS
ENTRYPOINT_METHOD_BATCH PASS
ENTRYPOINT_AST_PROCESS_COUNT <= 4
ENTRYPOINT_BASE_GIT_BATCH_COUNT <= 1
ENTRYPOINT_PER_FILE_MERGE_BASE == 0
ENTRYPOINT_PER_FILE_GIT_SHOW == 0
```

以及：

```text
full internal/analysis regression PASS
full internal/nav regression PASS
full go test ./... PASS
go vet ./... PASS
Windows exact-head performance workflow PASS
```

---

# 15. 最终结论

1.6.4 Task 1 的核心目标现在正式修订为：

```text
先把 Entrypoint Inventory Scan Scope
严格绑定到 Canonical Snapshot exact changed-file set

再把 exact-file scan
从 per-file/per-pattern process model
优化为 Current/Base two-phase batch model
```

也就是说，本轮最终优化对象不是单纯：

```text
减少 ast-grep process
```

而是：

```text
Scope 不扩大
+
Process 不放大
```

只有两个条件同时成立，才能解决实际出现的：

```text
analysis snapshot = 4–5 changed files
但 analysis certify / ENTRYPOINT_BASE_SCAN 仍因全项目扫描而 timeout
```

的问题。