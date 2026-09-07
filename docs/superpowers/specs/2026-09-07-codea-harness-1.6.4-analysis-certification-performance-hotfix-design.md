# Codea Harness 1.6.4 Analysis Certification Performance Hotfix Design

## 1. 背景

Codea Harness 1.6.3 已完成正式验收与发布收口。实际项目继续使用后发现一个新的性能问题：

```text
harness review
→ Runtime canonical ChangeSet
→ Reviewer analyze-change
→ analysis certify
```

当 Change Set 约 20 个文件时，`analysis certify` 仍可能超时。

该问题会直接阻断后续 ReviewOptions，因此当 certify 未成功时，即使实际涉及 2+ 条业务 Chain，也不会进入：

```text
review options
→ USER_SELECTION
→ 展示 Chain 选择
```

本轮 1.6.4 只解决 **Analysis Certification Performance**。

本轮不重新设计 1.6.2/1.6.3 已接受的 Review Authority、Chain Authority、USER_SELECTION、Finding Authority、Upgrade Authority。

本轮建议版本：**1.6.4**。

建议实施顺序：

1. Task 1 — Batch Entrypoint Inventory
2. Task 2 — Analysis Certify Performance Evidence
3. Task 3 — Snapshot Freshness Fast Path（条件 Task）
4. Final Certification

规则：**上一 Task 未正式验收通过，不进入下一 Task。Task 3 只有在 Task 1 + Task 2 的 telemetry 明确证明 Snapshot recompute 已成为主要剩余瓶颈时才进入。**

---

## 2. 正式开发基线

本设计开始前的 `main` exact HEAD：

`ba60e4fa49b28734e3f550e951e187bf199d675d`

该 HEAD 已包含 Codea Harness 1.6.3 正式发布收口。

研发正式开工时仍必须 fresh 执行：

```bash
git fetch origin
git checkout main
git pull --ff-only
git rev-parse HEAD
```

并回传实际 exact HEAD。

不得从任何 1.6.3 Task 分支、release 临时分支或旧 hotfix 分支继续开发。

正式 Task 1 应从本设计合入后的 `main` 创建独立开发分支。

---

# 3. 本轮范围

## 3.1 In Scope

本轮只允许优化：

- `analysis certify` 内部性能；
- Entrypoint Inventory 的 Current/Base Controller Endpoint 扫描方式；
- Base source 批量读取方式；
- certify 运行阶段性能 telemetry；
- 在明确必要时优化 Snapshot freshness 校验实现。

核心目标：

> 在不降低 Runtime Authority、Snapshot freshness、Entrypoint completeness、Evidence、Coverage、USER_SELECTION 前置门禁的前提下，把 `analysis certify` 从 per-file/per-pattern 子进程放大，改造成批量、确定性、可观测的执行模型。

## 3.2 Out of Scope

本轮禁止顺手修改：

- Reviewer analyze-change prompt/semantic 内容；
- ReviewOptions 0/1/2+ Chain 决策语义；
- USER_SELECTION hard stop；
- ReviewScopeSelection 语义；
- Chain discover/refresh/persist Authority；
- Finding Proposal / Finding Certification；
- Review report renderer；
- Upgrade V2；
- OpenCode session lifecycle；
- Java Navigation 的其他非 Entrypoint Inventory consumer；
- 通用全局 Java Symbol Index；
- ast-grep 版本升级。

本轮不通过改变产品语义换性能。

---

# 4. 当前正式 Authority Chain

当前 plain Review 关键链路固定为：

```text
harness review
    ↓
review begin
    ↓
Runtime-owned fresh runId
    ↓
analysis snapshot
    ↓
analysis/change-set.json
    ↓
Reviewer analyze-change
    ↓
requests/change-analysis-proposal.json
    ↓
analysis certify
    ↓
Certified ChangeAnalysis
    ↓
review options
    ↓
AUTO_FULL / AUTO_SINGLE / USER_SELECTION
    ↓
review select
    ↓
review units / dispatch
    ↓
Finding Proposal
    ↓
Finding Certification
    ↓
review.md
```

如果 `analysis certify` 失败或超时：

```text
禁止继续 review options
禁止继续 USER_SELECTION
禁止继续 review units / dispatch
禁止继续 findings / report
```

因此本轮不能通过把 USER_SELECTION 提前到 uncertified analysis 之前来绕过性能问题。

---

# 5. 已确认根因

## 5.1 `analysis certify` 不是轻量 JSON validation

Canonical certify 当前包含：

```text
读取 + schema 验证 change-set.json
↓
Decode canonical snapshot
↓
重新 runtime.Compute(...) 验证 live Snapshot freshness
↓
读取 + schema 验证 proposal
↓
assembleCanonicalAnalysis
↓
ChangeAnalysis schema validation
↓
BuildEntrypointInventory
↓
VerifyEntrypointDispositions
↓
validateEvidenceAtRoot
↓
coverage.VerifyAnalysisJSON
↓
publish certified artifacts
```

其中第一优先级性能热点已经定位在：

```text
BuildEntrypointInventory
→ navigationEntrypointScanner.Current/Base
→ Navigator.FindControllerEndpoints
→ runRaw
```

## 5.2 当前 `runRaw()` 的子进程模型

当前 `runRaw()` 对每一个 AST pattern 单独执行一次：

```text
ast-grep.exe --pattern <pattern-1> <scope>
ast-grep.exe --pattern <pattern-2> <scope>
ast-grep.exe --pattern <pattern-3> <scope>
...
```

也就是说：

```text
pattern 数量
=
ast-grep process 数量
```

这在 Windows 下会产生明显 process startup 开销。

## 5.3 Controller Type pattern 数量

当前 `allTypePatterns()`：

基础 declaration pattern 共 10 条：

```text
class
public class
final class
public final class
abstract class
public abstract class
interface
public interface
enum
public enum
```

`withAnnotationVariants()` 对每个基础 pattern 再增加：

```text
@$_ANN <declaration>
@$_ANN($$$ANNARGS) <declaration>
```

因此：

```text
10 × 3 = 30 type patterns
```

当前扫描一个存在的 production Java 文件时，至少会启动：

```text
30 × ast-grep.exe
```

以确定是否存在 Controller declaration。

## 5.4 Controller Method pattern 数量

当前 `allMethodPatterns()`：

基础 method shape：

```text
$RET $M($$$ARGS) { $$$BODY }
$RET $M($$$ARGS);
```

基础 2 条。

modifier 集合当前共 11 个：

```text
public
protected
private
static
public static
protected static
private static
final
public final
abstract
public abstract
default
```

按当前实现实际组合后：

```text
2 base
+ 11 × 2 modifier variants
= 24 method patterns before annotation variants
```

然后同样经过 `withAnnotationVariants()`：

```text
24 × 3 = 72 method patterns
```

因此，一旦一个文件的 Type phase 发现 Controller，当前同一侧还会追加：

```text
72 × ast-grep.exe
```

最终该 Controller 文件单侧最多约：

```text
30 + 72 = 102 ast-grep processes
```

## 5.5 精确 process amplification 模型

不能把所有 Java 文件都简单按 `204 ast-grep/file` 估算。

当前更精确的模型是：

```text
AstProcessCount
=
30 × Ncurrent
+ 30 × Nbase
+ 72 × Ccurrent
+ 72 × Cbase
```

其中：

- `Ncurrent`：当前侧需要实际 AST 扫描、且文件当前存在的 changed production Java 数；通常为 A/M；
- `Nbase`：Base 侧需要扫描的 production Java 数；通常为 M/D；
- `Ccurrent`：Current Type phase 实际发现 Controller 的文件数；
- `Cbase`：Base Type phase 实际发现 Controller 的文件数。

典型情况示例：

```text
21 changed files
15 production Java
其中 4 个 Controller
15 个 Java 全部 Modified
```

则当前理论 ast-grep process 数约为：

```text
Current Type:   15 × 30 = 450
Base Type:      15 × 30 = 450
Current Method:  4 × 72 = 288
Base Method:     4 × 72 = 288
--------------------------------
Total                    = 1476 ast-grep processes
```

如果 Controller 比例更高，进程数继续显著上升。

因此，21-file Change Set 出现 60s 级别 timeout 并不是意外。

## 5.6 Base scanner 还有 per-file Git / temp 放大

当前每个 Base 文件还会独立执行：

```text
git merge-base <base> HEAD
git show <mergeBase>:<path>
MkdirTemp
写入单文件临时 tree
ast-grep 扫描
RemoveAll temp
```

问题：

1. `snapshot.MergeBase` 已经由 Runtime canonical snapshot 算出并绑定 Authority，Base scanner 再次逐文件 `git merge-base` 没必要；
2. Base source 可以批量读取，不需要 N 次 `git show`；
3. 临时目录可以按整个 inventory batch 创建一次，不应按文件创建 N 次。

## 5.7 `changeset.Compute()` 重算是第二优先级

Canonical certify 还会重新执行：

```text
runtime.Compute(root, snapshot.RequestedBaseRef, snapshot.IncludeWorkingTree)
```

用来防止：

```text
snapshot
→ proposal
→ certify
```

期间 Git state 发生变化。

这个 fail-closed 目的正确，不能删除。

但它会再次执行：

- base resolve；
- HEAD resolve；
- merge-base；
- branch；
- committed diff；
- staged diff；
- unstaged diff；
- untracked enumeration/hash；
- end-of-compute identity recheck。

当前判断：它是潜在第二瓶颈，但在几百/上千次 ast-grep process 被消除之前，不应先扩大范围修改 Snapshot contract。

---

# 6. 本轮设计原则

## 6.1 只改变执行模型，不改变 Authority

本轮核心原则：

```text
same facts
same fail-closed rules
same certified artifacts
fewer child processes
less repeated IO
better observability
```

## 6.2 Batch，不并发制造旧模型

正确方向：

```text
per-file × per-pattern process
→
per-side batch scan
```

禁止方向：

```text
把原来 1000+ ast-grep process
用 8/16/32 goroutine 并发跑
```

减少 process count 优先于提高 process concurrency。

## 6.3 保持 pinned ast-grep 0.42.1

本轮不升级 ast-grep。

理由：

- 1.6.3 已对 pinned ast-grep 0.42.1 建立正式 regression；
- 本轮应隔离“执行模型变化”和“解析引擎版本变化”；
- 先证明 pinned binary 可以通过 multi-rule scan 达到目标。

## 6.4 不先引入 Controller 文本 prefilter

Task 1 第一版不使用：

```text
filename == *Controller.java
contains @RestController
contains @Controller
```

作为跳过 AST 的前置条件。

原因：

- annotation 可能在本次变更中被删除；
- Base 侧仍需要识别旧 Controller；
- fully-qualified annotation / formatting / Unicode 等情况不能由简单文本代替；
- batch 后全体 changed production Java 的 Type scan 成本应已经足够低。

未来如果真实 500+ Java Change Set 仍有性能问题，可以另做 conservative candidate prefilter，但不得进入本轮 Task 1。

---

# 7. Task 1 — Batch Entrypoint Inventory

## 7.1 目标

把 Entrypoint Inventory 从：

```text
O(files × patterns × process startup)
```

改成：

```text
O(total candidate source bytes + AST matching)
+
O(1)级 process startup per scan phase
```

## 7.2 推荐正式结构

```text
Canonical Snapshot
        │
        ▼
Classify production Java sides
        │
        ├───────────────┐
        │               │
Current side        Base side
A/M existing        M/D base objects
        │               │
        │        snapshot.MergeBase
        │               │
        │        git cat-file --batch
        │               │
        └───────┬───────┘
                ▼
      ONE batch temp workspace
      current/<relative-path>
      base/<relative-path>
      rules/<deterministic-rules>
                │
                ▼
      Current Type batch scan
                │
      identify Controller files
                │
      Current Method batch scan
      only Controller files
                │
                ▼
        Base Type batch scan
                │
      identify Base Controller files
                │
        Base Method batch scan
      only Base Controller files
                │
                ▼
 path -> []ControllerEndpoint
                │
                ▼
 existing collectModifiedEntrypoints153
                │
                ▼
 Runtime EntrypointInventory
```

## 7.3 为什么采用 Two-phase Batch，而不是“一侧一个 102-rule scan”

理论上可以一侧一次同时扫描全部 102 rules。

但当前真实语义存在一个重要优化：

```text
Type phase 未发现 Controller
→ 不执行 72 条 Method patterns
```

实际 changed Java 中，大部分通常是：

- Service；
- ServiceImpl；
- Mapper/Repository；
- DTO/VO/Entity；
- Config；
- Utility。

如果为了追求“1 process/side”而对所有 Java 都执行 72 method rules，会增加不必要 AST work。

因此本设计推荐：

```text
每侧最多 2 个 ast-grep process：
1. Type batch
2. Controller-only Method batch
```

最终：

```text
Current Type    <= 1
Current Method  <= 1
Base Type       <= 1
Base Method     <= 1
--------------------------------
Total ast-grep  <= 4 per inventory
```

如果某侧没有 Controller：

```text
该侧 Method batch = 0
```

这比“严格 2 process 总量”更符合真实 workload，同时仍然把 process count 从数百/上千降到常数级。

## 7.4 Scanner interface

生产代码不再以：

```go
Current(ctx, path)
Base(ctx, snapshot, path)
```

作为主要 inventory 扫描模型。

建议演进为语义类似：

```go
type EntrypointBatchScanner interface {
    CurrentBatch(
        ctx context.Context,
        paths []string,
    ) (map[string][]ControllerEndpoint, error)

    BaseBatch(
        ctx context.Context,
        snapshot changeset.Snapshot,
        paths []string,
    ) (map[string][]ControllerEndpoint, error)
}
```

具体名字可以按仓库风格调整，但 Authority 边界必须是：

- input paths 来自 canonical snapshot；
- output path 必须能 exact map 回 canonical repo-relative path；
- scanner 不能自己扩大到全 repo；
- scanner 不能修改 Snapshot/ChangeSet。

## 7.5 Current/Base path classification

继续保持现有 status 语义，不在本轮修改 rename/status contract。

Current 侧 candidate：

```text
production Java
且当前文件存在
通常 A / M
```

Base 侧 candidate：

```text
production Java
且 merge-base 侧存在
通常 M / D
```

注意：当前代码对 rename 的具体 ChangeSet 表达方式不属于本轮范围。

Batch 实现必须保持现有生产行为 parity，不得借性能 hotfix 顺手重定义 rename semantics。

## 7.6 Batch temporary workspace

为避免：

- Windows 命令行长度；
- 传大量绝对路径；
- scan 意外扩大到无关源码；

推荐整个 inventory 只创建 **一个** temp root：

```text
%TEMP%/codea-harness-entrypoint-batch-<id>/
  current/
    module-a/src/main/java/...
    module-b/src/main/java/...
  base/
    module-a/src/main/java/...
    module-b/src/main/java/...
  rules/
    type-rules...
    method-rules...
```

Current 文件：

- 从工作区读取 exact bytes；
- 写入 `current/<repo-relative-path>`；
- 不做内容 normalize；
- 不改换行；
- 不改编码。

Base 文件：

- 从 `snapshot.MergeBase` 批量读取 exact blob bytes；
- 写入 `base/<repo-relative-path>`。

ast-grep 输出 path 必须去掉：

```text
current/
base/
```

前缀后恢复原 canonical repo-relative path。

任何 path traversal / normalization mismatch 必须 fail closed。

## 7.7 Base source 批量读取

禁止继续 per-file：

```text
git merge-base
git show
```

Base batch 固定使用 canonical snapshot 已有：

```text
snapshot.MergeBase
```

Base source 建议一次：

```text
git cat-file --batch
```

读取所有：

```text
<snapshot.MergeBase>:<path>
```

目标：

```text
Base Git content process <= 1
per-file git merge-base = 0
per-file git show = 0
```

### Base missing behavior

如果当前旧实现对某 path 的 Base object 不存在时返回空结果，则 batch loader 应保持同等语义。

但：

- Git command/system failure；
- batch protocol malformed；
- path identity mismatch；
- 读取到错误 object；

必须 fail closed，不能把 Runtime failure 当“不是 Controller”。

## 7.8 Multi-rule ast-grep execution

本轮仍使用 pinned：

```text
ast-grep 0.42.1
```

Task 1 必须先增加一个真实 CLI capability probe，确认该 pinned binary 的 multi-rule `scan` 输出结构可以稳定提供：

- rule identity；
- file path；
- match text；
- start/end range。

只有 capability probe PASS 后，正式 Batch Scanner 才可依赖该模式。

### Rule semantics 必须 exact preserve

本轮禁止重新设计 Controller AST grammar。

当前：

```text
30 Type patterns
72 Method patterns
```

必须被确定性映射成 batch rule set。

推荐给每条 rule 分配稳定 ID：

```text
entrypoint-type-000 ... entrypoint-type-029
entrypoint-method-000 ... entrypoint-method-071
```

Rule pack 的 pattern 来源应继续复用现有 canonical pattern generator，避免维护两份不同 pattern 列表。

Batch 结果解析后仍继续复用现有语义函数，例如：

- `typeKindAndName()`；
- `hasAnyAnnotation153()`；
- `smallestContaining()`；
- method/controller range containment；
- existing sort/dedupe；
- `collectModifiedEntrypoints153()`。

也就是说：

```text
改变 transport/execution model
不改变 semantic classifier
```

## 7.9 不要直接重构通用 `runRaw()` consumer

Task 1 只为 Entrypoint Inventory 新增 batch path。

不要顺手把以下全部迁移：

- `GetSymbolInfo`；
- `FindCallers`；
- `FindImplementations`；
- Workspace navigation；
- generic annotation lookup。

这些属于不同风险面。

允许新增类似：

```text
internal/nav/entrypoint_batch_164.go
internal/analysis/base_source_batch_164.go
```

现有 `runRaw()` 仍保留给其他正式功能。

## 7.10 Context / timeout

当前 `BuildEntrypointInventory()` 有 60s context timeout。

Task 1 完成后：

- 不应靠提高该 timeout 获得 PASS；
- 可继续保留 60s 作为安全上限；
- performance gate 必须远低于 60s。

本轮不要求降低 timeout，因为安全 margin 与性能 SLA 是两回事。

---

# 8. Task 1 Authority Invariants

Task 1 必须保持以下全部不变：

1. Canonical Change Set 只有 Runtime authority。
2. Batch paths 只能来自当前 canonical snapshot。
3. Controller identity 仍由 AST + existing semantic filtering 决定。
4. `@Controller` / `@RestController` 语义不变。
5. Spring mapping annotation 语义不变。
6. Added Controller/endpoint 语义不变。
7. Modified endpoint 语义不变。
8. Pure deleted endpoint 仍允许 `DispositionRemoved`。
9. Controller class-level change 仍能使当前 Controller endpoints 成为 obligation。
10. Base-side class-level deletion semantics 不变。
11. Nested Class ownership 不变。
12. Multi-module exact path identity 不变。
13. Duplicate symbol/path authority 不变。
14. `VerifyEntrypointDispositions` 不降低要求。
15. `ENTRYPOINT_COMPLETENESS_INCOMPLETE` 仍 fail closed。
16. Evidence validation 不变。
17. Coverage validation 不变。
18. 只有 Certified ChangeAnalysis 才能进入 ReviewOptions。
19. USER_SELECTION 仍只能发生在 certify 成功之后。

---

# 9. Task 1 强制测试

## 9.1 RED evidence

必须在 1.6.3 正式 baseline 上证明旧执行模型存在 process amplification。

测试不能只写 wall-clock。

至少提供 instrumented runner 证据：

```text
TYPE_PATTERN_COUNT = 30
METHOD_PATTERN_COUNT = 72
```

构造多个 changed Java / Controller 文件，证明旧模型 process count 随：

```text
files × patterns
```

增长。

## 9.2 Functional parity

新 Batch Scanner 必须与旧 reference semantics 比较。

旧行为可以作为 test-only reference helper 保留，不要求继续留在生产执行路径。

至少覆盖：

- `@RestController`；
- `@Controller`；
- fully-qualified Controller annotation；
- `@RequestMapping`；
- `@GetMapping`；
- `@PostMapping`；
- `@PutMapping`；
- `@PatchMapping`；
- `@DeleteMapping`；
- 多 endpoint；
- 一个文件多个 type；
- nested class；
- Controller annotation 新增；
- Controller annotation 删除；
- endpoint 新增；
- endpoint 修改；
- endpoint 删除；
- whole Controller 删除；
- class-level changed hunk；
- Base-only Controller；
- Current-only Controller；
- multi-module same class name；
- duplicate symbol different path；
- 大型 Controller / 大 JSON record regression。

要求：

```text
ExpectedEntrypoints semantic equality
path equality
symbol equality
disposition equality
stable ordering
```

## 9.3 Process count gate

必须增加 deterministic process counter gate。

新模型至少满足：

```text
CURRENT_TYPE_AST_PROCESSES <= 1
CURRENT_METHOD_AST_PROCESSES <= 1
BASE_TYPE_AST_PROCESSES <= 1
BASE_METHOD_AST_PROCESSES <= 1
TOTAL_ENTRYPOINT_AST_PROCESSES <= 4
```

如果 Current/Base 某侧没有 Controller：

```text
对应 METHOD_AST_PROCESSES == 0
```

Base Git：

```text
BASE_CAT_FILE_PROCESSES <= 1
PER_FILE_MERGE_BASE_PROCESSES == 0
PER_FILE_GIT_SHOW_PROCESSES == 0
```

Process count gate 是本轮最重要的非时间型性能回归门禁。

## 9.4 Real pinned ast-grep Windows gate

必须在 `windows-latest` 使用正式 pinned ast-grep 0.42.1。

不能只使用 fake runner 声称 Task 1 完成。

必须证明：

- real multi-rule scan works；
- output parser works；
- path mapping works；
- large JSON record works；
- functional parity works。

---

# 10. Task 2 — Analysis Certify Performance Evidence

## 10.1 目标

以后不再让用户只看到：

```text
analysis certify timeout
```

而不知道真正耗时阶段。

Task 2 增加 Runtime-owned performance evidence。

## 10.2 Artifact

正式路径建议：

```text
.code-harness/runs/<runId>/analysis/certify-performance.json
```

该 artifact：

- Runtime-owned；
- Agent 不得创建/修改；
- 仅用于性能诊断与验收；
- 不成为 semantic authority；
- 不进入 ChangeAnalysis certificate hash；
- 不允许其内容影响 Review correctness decision。

## 10.3 Schema

建议 schema version 1，至少包含：

```json
{
  "schemaVersion": 1,
  "runId": "review-...",
  "snapshotSha256": "...",
  "status": "COMPLETE",
  "counts": {
    "changedFiles": 21,
    "productionJavaCurrent": 15,
    "productionJavaBase": 15,
    "controllerFilesCurrent": 4,
    "controllerFilesBase": 4
  },
  "timingMs": {
    "snapshotFreshness": 420,
    "snapshotSchemaAndDecode": 12,
    "proposalSchema": 11,
    "analysisAssembly": 4,
    "entrypointInventory": 1900,
    "currentTypeAst": 310,
    "currentMethodAst": 160,
    "baseSourceLoad": 90,
    "baseTypeAst": 300,
    "baseMethodAst": 170,
    "entrypointVerification": 9,
    "evidenceValidation": 40,
    "coverageValidation": 8,
    "publish": 15,
    "total": 2450
  },
  "processes": {
    "currentTypeAst": 1,
    "currentMethodAst": 1,
    "baseTypeAst": 1,
    "baseMethodAst": 1,
    "baseCatFile": 1
  }
}
```

字段可按实现细节微调，但以下必须固定可观测：

- total；
- snapshot freshness；
- entrypoint inventory；
- current/base type scan；
- current/base method scan；
- base source load；
- evidence；
- coverage；
- AST process count；
- base Git process count。

## 10.4 Failure telemetry

如果 certify 失败，应尽可能仍生成 performance artifact：

```json
{
  "status": "FAILED",
  "errorCode": "ENTRYPOINT_CURRENT_SCAN_FAILED",
  "timingMs": { ...已完成阶段... }
}
```

要求：

- telemetry 写失败不能覆盖原正式错误；
- telemetry 不得把 timeout 转成 success；
- 原 fail-closed error code 保持 authority；
- 不记录 source content；
- 不记录 secret/config values；
- 不要求记录用户绝对文件系统路径。

## 10.5 Performance counters 的 Authority

Process counter 必须由 Runtime scanner/runner 在实际进程启动位置记录。

禁止通过：

```text
文件数 × 估算 pattern 数
```

伪造“实际 process count”。

---

# 11. Real Windows Performance Gate

Task 2 必须建立 dedicated 1.6.4 Windows performance workflow。

性能验收只统计：

```text
Runtime analysis certify
```

不包含 LLM `Reviewer analyze-change` 时间。

## 11.1 Fixture A — 21-file Real-world Regression

构造接近本次真实问题的 fixture：

```text
21 changed files
约 15-20 production Java
3-5 Controller
其余 Service / ServiceImpl / Mapper / DTO 等
至少包含 Modified 文件，确保 Current + Base 双侧扫描
```

目标：

```text
analysis certify <= 5s
```

CI hard limit：

```text
<= 15s
```

同时 assert：

```text
TOTAL_ENTRYPOINT_AST_PROCESSES <= 4
BASE_CAT_FILE_PROCESSES <= 1
PER_FILE_MERGE_BASE == 0
PER_FILE_GIT_SHOW == 0
```

## 11.2 Fixture B — 50-file Scaling

构造：

```text
50 changed production Java
约 8-10 Controller
Current + Base
multi-module
```

目标：

```text
analysis certify <= 10s
```

CI hard limit：

```text
<= 20s
```

Process count 仍必须是常数级：

```text
AST <= 4
Base Git content <= 1
```

## 11.3 Fixture C — 100-file Scaling Guard

构造：

```text
100 changed production Java
约 10-20 Controller
```

主要目的不是模拟日常开发，而是防止算法重新退回：

```text
files × process count
```

目标：

```text
analysis certify <= 20s
```

CI hard limit：

```text
<= 30s
```

强制：

```text
TOTAL_ENTRYPOINT_AST_PROCESSES <= 4
```

## 11.4 时间门禁与 process 门禁的关系

两者都必须有。

原因：

- wall-clock 会受 GitHub runner 抖动影响；
- process count 是结构性 regression gate；
- process count PASS 但 wall-clock FAIL，说明还有新的 IO/AST 性能问题；
- wall-clock PASS 但 process count FAIL，说明旧放大可能暂时被快机器掩盖，不能接受。

---

# 12. Task 3 — Snapshot Freshness Fast Path（条件 Task）

## 12.1 为什么是条件 Task

Task 1 完成后，最大已知热点应从：

```text
hundreds/thousands ast-grep processes
```

降为：

```text
<= 4 ast-grep processes
```

这时 Task 2 telemetry 才能真实显示下一个瓶颈。

只有满足类似以下条件时才进入 Task 3：

```text
21/50-file fixture 中
snapshotFreshness >= total 的 40%
或
snapshotFreshness 本身持续 > 2s
```

阈值可在 Task 2 验收时根据 Windows fresh data 最终确认。

如果 Task 1 后：

```text
21 files certify <= 5s
50 files certify <= 10s
```

且 Snapshot freshness 并非主要耗时，则 Task 3 可以明确：

```text
NOT REQUIRED FOR 1.6.4
```

不为了“计划里有 Task 3”强行开发。

## 12.2 当前不能做的错误 fast path

禁止用：

```text
git status 看起来没变
```

代替现有 freshness authority。

禁止只比较：

```text
HEAD
branch
changed file names
```

因为 staged/unstaged/untracked bytes 可能改变而路径不变。

## 12.3 如果进入 Task 3，正确目标

职责拆分：

```text
changeset.Compute()
= 生成完整 canonical Snapshot

changeset.VerifyFreshness(...)
= 证明当前 Git source identity 仍与已封存 Snapshot 对应
```

Fast freshness 必须继续覆盖：

- resolved base commit；
- merge base；
- HEAD；
- current branch；
- committed state；
- index/staged state；
- unstaged working tree bytes；
- untracked path membership；
- untracked bytes。

不能只验证文件名。

## 12.4 推荐 fingerprint 方向

如果实现 Task 3，可考虑在 snapshot 阶段额外生成 Runtime-owned freshness manifest，例如：

```text
base/head/branch identity
staged path + index object id
unstaged path + exact content hash
untracked path + exact content hash
```

certify 时：

```text
resolve identities
+ lightweight path/state enumeration
+ exact relevant content hash
→ recompute freshness fingerprint
→ equal => FRESH
→ different => STALE / fail closed
```

但该设计会涉及新的 snapshot-adjacent contract，因此：

- 必须单独设计 schema/provenance；
- 必须证明与当前 `GitStateSHA256` fail-closed 语义等价；
- 不能直接删除旧 `Compute` 路径后声称性能提升；
- 必须有 mutation matrix。

## 12.5 Task 3 强制 stale tests（如果实施）

至少覆盖 snapshot 之后：

- HEAD move；
- base ref move；
- branch change；
- committed diff change；
- staged file bytes change；
- staged file新增/删除；
- unstaged file bytes change；
- unstaged file新增/恢复；
- untracked file新增；
- untracked file删除；
- untracked same path bytes change；
- same path/same size/different bytes；
- staged + unstaged same path mixed state。

任何状态变化都不得错误返回 FRESH。

---

# 13. 禁止方案

本轮以下方案明确禁止作为正式修复：

## 13.1 只增加 timeout

禁止：

```text
60s → 120s → 300s
```

作为主要修复。

可以保留 60s safety timeout，但性能必须通过结构优化满足 SLA。

## 13.2 USER_SELECTION 提前到 certify 前

禁止：

```text
uncertified proposal
→ review options
→ USER_SELECTION
```

ReviewOptions 必须继续消费 Certified ChangeAnalysis。

## 13.3 并发运行原 per-pattern 模型

禁止：

```text
16 goroutines
→ 16 × ast-grep processes parallel
```

不能用并发掩盖 process amplification。

## 13.4 文件名 / regex 直接成为 Controller Authority

禁止：

```text
*Controller.java => Controller fact
contains @RestController => final Controller fact
```

最终 Controller/endpoint identity 仍必须来自 AST + existing Runtime semantic verification。

## 13.5 改写 30/72 patterns 语义

Task 1 不允许为了 batch 方便顺手“精简” pattern set。

先保证 exact semantic parity。

如果未来要重构 Controller rule grammar，必须独立 Task。

## 13.6 引入全局 persistent AST cache

本轮不建设跨 run / 跨 branch 的 persistent semantic cache。

原因：

- invalidation 复杂；
- 容易污染 Authority；
- 当前 process amplification 可以不依赖 persistent cache 解决。

允许 run-local temp/batch data；run 结束后不作为长期事实。

## 13.7 为性能牺牲 fail-closed

任何：

```text
scan timeout => assume no Controller
base object read failure => assume no endpoint
schema error => skip
freshness unknown => continue
```

都禁止。

---

# 14. 文件级建议范围

Task 1 预计主要涉及：

```text
.code-harness/tools-runtime/internal/analysis/entrypoints.go
.code-harness/tools-runtime/internal/nav/controller_endpoints_153.go
```

建议新增独立文件而不是把现有文件继续膨胀：

```text
.code-harness/tools-runtime/internal/nav/entrypoint_batch_164.go
.code-harness/tools-runtime/internal/analysis/base_source_batch_164.go
```

Task 2 可新增：

```text
.code-harness/tools-runtime/internal/analysis/certify_performance_164.go
```

并对：

```text
certify_canonical_162.go
analysis_command.go
```

做最小接线。

Task 3 如果实际进入，再修改：

```text
internal/changeset/**
internal/analysis/certify_canonical_162.go
contracts/**
```

Task 1 不应提前修改 Snapshot schema。

---

# 15. 开发 Task 拆分

## Task 1 — Batch Entrypoint Inventory

必须交付：

1. pinned ast-grep 0.42.1 multi-rule real capability probe；
2. CurrentBatch；
3. BaseBatch；
4. 30 Type rules batch；
5. Controller-only 72 Method rules batch；
6. single batch temp workspace；
7. `snapshot.MergeBase` reuse；
8. `git cat-file --batch` Base source load；
9. exact semantic parity regression；
10. deterministic process count gate；
11. Windows real ast-grep gate。

Task 1 不包含 performance artifact schema。

## Task 2 — Analysis Certify Performance Evidence

必须交付：

1. Runtime-owned `certify-performance.json`；
2. success/failure timing；
3. actual process counters；
4. 21-file Windows regression；
5. 50-file Windows scaling；
6. 100-file scaling guard；
7. exact-head CI；
8. performance evidence 可用于判断 Task 3 是否需要。

## Task 3 — Snapshot Freshness Fast Path

只有 Task 2 evidence 证明必要时启动。

若不需要：

```text
Task 3 = NOT REQUIRED
```

并在 Final Certification 中记录决策证据。

如果需要，则必须先补充/冻结 Task 3 详细 contract 后再开发，不能临时直接改 `changeset.Compute()`。

---

# 16. Final Certification

1.6.4 Final Certification 至少必须验证：

## 16.1 Functional

- full Go regression；
- `go vet ./...`；
- Windows x64 Runtime build；
- Task 1 exact semantic parity；
- Task 2 telemetry schema；
- 1.6.3 Task 1 Workspace AST regression；
- 1.6.3 Task 2 Clean Project Chain Discovery；
- 1.6.3 Task 3 real OpenCode same-session USER_SELECTION E2E；
- 1.6.3 Task 4 Upgrade V2 regression；
- retained 1.6.2 Review Reliability gates。

## 16.2 Performance

最终必须有 fresh Windows exact-head evidence：

```text
21-file certify <= 15s hard limit
50-file certify <= 20s hard limit
100-file certify <= 30s hard limit
```

同时：

```text
TOTAL_ENTRYPOINT_AST_PROCESSES <= 4
BASE_CAT_FILE_PROCESSES <= 1
PER_FILE_MERGE_BASE == 0
PER_FILE_GIT_SHOW == 0
```

性能目标值：

| Change Set | Target | CI Hard Limit |
|---|---:|---:|
| 20–30 files | ≤5s | 15s |
| 50 files | ≤10s | 20s |
| 100 files | ≤20s | 30s |

## 16.3 Authority

必须 fresh 证明：

```text
analysis certify failure
→ no review options
→ no USER_SELECTION
→ no review units
→ no report
```

以及：

```text
analysis certify success
→ review options 正常继续
→ 2+ Chains 仍进入 USER_SELECTION
```

性能优化不能改变 gate 顺序。

---

# 17. 验收输出要求

每个正式 Task 都必须回传：

- branch；
- base exact SHA；
- current exact HEAD；
- commit list；
- changed files；
- RED evidence；
- focused GREEN；
- full relevant regression；
- `go vet`；
- Windows exact-head CI run ID；
- exact-head marker；
- performance/process evidence。

Task 1 验收尤其关注：

```text
是否真的降低 process count
```

而不是只看“测试是否快了一点”。

Task 2 验收尤其关注：

```text
telemetry 是否来自真实 Runtime execution
```

不得由测试脚本手写数字。

---

# 18. 研发开工要求

研发接手后，当前只允许开发：

```text
Task 1 — Batch Entrypoint Inventory
```

不要进入 Task 2。

Task 1 正式开工前：

1. fresh 读取本设计；
2. 锁定本设计合入后的 `main` exact HEAD；
3. 创建独立 Task 1 分支；
4. 先做旧模型 process amplification RED；
5. 再做 pinned ast-grep multi-rule real capability probe；
6. capability probe 不成立时必须停止并重新评审实现方案，不能偷偷升级 ast-grep；
7. capability probe 成立后再进入 Batch Scanner TDD。

禁止直接在 `main` 开发。

---

# 19. 最终决策摘要

本轮性能问题的核心不是“21 个文件太多”，而是 Entrypoint Inventory 当前把 AST pattern 数量放大成 Windows 子进程数量。

当前精确结构约为：

```text
30 × Current Java files
+ 30 × Base Java files
+ 72 × Current Controller files
+ 72 × Base Controller files
```

因此一个普通 20-file 级 Change Set 就可能产生上千次 ast-grep process。

1.6.4 的第一修复原则固定为：

```text
per-file/per-pattern process
→
batched Current/Base AST scan
```

推荐最终 inventory process shape：

```text
Current Type   <= 1 ast-grep
Current Method <= 1 ast-grep
Base Type      <= 1 ast-grep
Base Method    <= 1 ast-grep
Base source    <= 1 git cat-file process
```

同时继续保持：

```text
Canonical Snapshot Authority
Snapshot freshness fail-closed
Entrypoint completeness
Evidence verification
Coverage verification
Certified ChangeAnalysis
ReviewOptions after certify
USER_SELECTION after ReviewOptions
```

Task 3 的 Snapshot Fast Path 不预设必须开发；是否进入由 Task 1 + Task 2 的真实 Windows telemetry 决定。

本设计的目标不是把 timeout 从 60 秒调到 120 秒，而是把 `analysis certify` 的执行复杂度从“文件数 × pattern 数 × process startup”降到常数级 process startup，并用可重复的 Windows performance gate 防止回归。
