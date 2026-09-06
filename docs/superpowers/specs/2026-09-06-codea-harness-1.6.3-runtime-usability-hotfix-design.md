# Codea Harness 1.6.3 Runtime Usability Hotfix Design

## 1. 背景

Codea Harness 1.6.2 Review Reliability Hotfix 已完成正式验收。本轮 1.6.3 不重新设计 1.6.2 Review Authority，而是修复实际项目使用中暴露的 4 个 Runtime Usability 问题：

1. 大型 Java Class / 大型项目下 Workspace AST 导航容易 timeout；
2. `harness chain discover` 在当前分支与主分支无差异、working tree clean 时无法发现现有调用链；
3. `harness review` 涉及多个调用链时，实际 Agent 没有强制等待用户选择 Review Scope；
4. 当前 Harness Upgrade 采用大范围 managed-tree replace，用户升级体验和 Windows Runtime 替换成本偏高。

本轮建议版本：**1.6.3**。

建议实施顺序：

1. Task 1 — Workspace AST Performance Hardening
2. Task 2 — Clean Project Chain Discovery
3. Task 3 — Multi-Chain Review Explicit User Selection
4. Task 4 — Upgrade V2 Hash-Aware Delta Apply
5. Final Certification

规则：**上一 Task 未正式验收通过，不进入下一 Task。**

---

## 2. 正式开发基线

本设计提交前的 `main` exact HEAD：

`c81f19cc2fab6925bd278c75b594006ce585f581`

研发正式开工时仍必须 fresh 执行：

```bash
git fetch origin
git checkout main
git pull --ff-only
git rev-parse HEAD
```

并回传实际 exact HEAD。

### 2.1 特别说明：不要使用已有误开发分支作为正式基线

仓库当前存在：

`hotfix/1.6.3-runtime-usability`

该分支包含前期排查过程中误产生的设计、测试和部分实现提交。

这些内容：

- 不属于正式研发成果；
- 不属于 Accepted Baseline；
- 不应直接作为后续开发基线；
- 不应整体 cherry-pick；
- 只能作为排查线索参考。

正式研发应从本设计提交后的 `main` 创建 Task 1 分支。

---

# 3. Task 1 — Workspace AST Performance Hardening

## 3.1 实际现象

实际项目中，大型 Java Class（例如 1000+ 行，甚至更大的 Service / Controller）执行代码解析、Workspace navigation、调用链分析时出现 timeout。

代码中不存在“1000 行即超时”的硬阈值。1000 行只是当前项目规模下容易触发性能问题的经验临界点。

## 3.2 已定位根因一：每个 pattern 重复全目录扫描

当前 Workspace AST 导航存在扫描放大：

```text
pattern 1 -> ast-grep -> src/main/java
pattern 2 -> ast-grep -> src/main/java
pattern 3 -> ast-grep -> src/main/java
...
```

单个 method lookup 又会展开多组 Java modifier / declaration patterns，并可能进一步执行：

- owner class lookup；
- method lookup；
- all class types；
- nested class ownership；
- method call lookup；
- superclass / subclass / template dispatch。

因此一次 Navigation 的实际成本与以下因素共同增长：

- Java 文件数量；
- 单文件大小；
- pattern 数量；
- 同一 Navigation 内重复扫描次数。

### 禁止方案

不能仅通过：

```text
30s -> 60s -> 120s
```

扩大 timeout 作为正式修复。

提高 timeout 最多只能作为 safety margin，不能替代扫描结构优化。

## 3.3 已定位根因二：`bufio.Scanner` 默认约 64KB token 上限

当前代码使用：

```go
bufio.NewScanner(...)
```

读取：

```text
ast-grep --json=stream
```

Go `bufio.Scanner` 默认单 token 最大约 64KB。

ast-grep 对类似：

```text
class BigService { $$$BODY }
```

的匹配记录可能在 JSON `text` 字段中携带整个 Class Body。大型类可能导致单条 JSON record 超过默认 scanner token 上限，从而出现：

- `scanner.Err()`；
- match 丢失；
- PARTIAL / navigation failure；
- 上层重试或最终 timeout。

因此 Task 1 必须同时处理：

1. 重复全树扫描；
2. 大 JSON record 解析上限。

## 3.4 正式设计

目标结构：

```text
Symbol Request
    ↓
Candidate File Narrowing
    ↓
1..N Candidate Java Files
    ↓
ast-grep semantic verification
    ↓
Runtime Navigation Fact
```

例如查找：

```text
OrderService.createOrder
```

应该先缩小到：

```text
OrderService.java
```

或少量候选文件，再对候选执行 AST verification，而不是每个 pattern 都重新扫描整个 `src/main/java`。

### Candidate narrowing 可使用

- 文件名；
- declaration 文本预筛；
- deterministic file inventory；
- 已有 run-level cache/index。

但这些只能用于**候选缩小**。

### 最终语义 Authority 仍必须来自 AST

包括：

- Class identity；
- Method identity；
- inheritance；
- Nested Class ownership；
- template dispatch。

不得把 regex/text prefilter 升级成最终语义 Authority。

## 3.5 Cache / Index 方向

Task 1 至少要完成 candidate-file narrowing。

如果实现成本合理，可增加 run-level Java file/symbol cache，使同一 Review / Discover run 中重复访问同一 Java 文件时不重新进行同等规模扫描。

不强制本 Task 一次性建设完整全局 Java Symbol Index，但架构不得阻止后续演进。

## 3.6 Scanner 要求

大 record 必须显式提高 Scanner buffer，或改成无 64KB 默认 token 限制的流式 JSON line reader。

不能依赖“当前类一般不会超过 64KB”。

## 3.7 强制测试

### Case A — Large Class

构造 1500–3000 行 Java Class，验证：

- Class lookup 正确；
- Method lookup 正确；
- 不 timeout。

### Case B — Large Repository

大 Class + 大量无关 Java files。

一旦 concrete candidate 已知，不得对每个 pattern 重复扫描整个 `src/main/java`。

### Case C — Large ast-grep JSON Record

单条 JSON > 64KB，必须完整解析，不得触发 Scanner token-too-long。

### Case D — Nested Class

优化后不得把：

```java
class Outer {
    class Inner {
        void run() {}
    }
}
```

中的 `Inner.run` 错误归属给 `Outer.run`。

### Case E — Inheritance / Template Method

保留既有：

- Workspace inheritance；
- superclass call；
- template method dispatch；
- ambiguous/missing fail-closed。

## 3.8 Task 1 验收要求

研发需提供：

- RED evidence；
- focused GREEN；
- full `internal/nav` regression；
- Windows exact-head CI；
- 如 CI 能安装 pinned ast-grep，则增加 real ast-grep large-file gate。

---

# 4. Task 2 — Clean Project Chain Discovery

## 4.1 当前问题

现有 public：

```text
harness chain discover
```

实际被绑定到：

```text
current Change Set
-> ChangeAnalysis
-> AffectedControllers
-> CallChains
-> Chain Discover
```

所以当：

```text
HEAD == main
working tree clean
```

通常会得到：

```text
ChangeSet = 0
AffectedControllers = 0
Discover = 0 Chains
```

即使项目实际存在：

```text
Controller
-> Service
-> ServiceImpl
-> Mapper
-> Mapper XML
```

显式 `harness chain discover` 也可能发现不到。

## 4.2 设计判断

ChangeSet-driven discovery 对 **Review affected discovery** 是合理的，因为 Review 只需要知道本次变化影响哪些链。

但显式 public `harness chain discover` 的用户语义应该是：

**发现当前项目现有调用链。**

不能要求用户先制造 Git diff 才能 discover。

## 4.3 正式拆分三种语义

### A. PROJECT DISCOVERY

```text
harness chain discover [target]
```

语义：

```text
Current Source
-> EntryPoint / Controller Discovery
-> Navigation
-> Chain Discovery
-> Runtime-owned Candidate
```

不依赖非空 Git ChangeSet。

### B. AFFECTED DISCOVERY

仅供 Review 内部使用：

```text
harness review
-> Canonical ChangeSet
-> ChangeAnalysis
-> Affected EntryPoints
-> affected chain discovery
```

保持现有逻辑。

### C. EXISTING CHAIN REFRESH

```text
harness chain refresh
```

继续负责已有 Chain 的刷新。

## 4.4 Authority 边界

### 禁止伪造 ChangeSet

PROJECT DISCOVERY 不能为了复用旧流程生成假的：

```text
analysis/change-set.json
```

也不能把“整个项目源码”伪装成 changedFiles。

Project source discovery 应拥有独立清晰的输入语义。

### Discover 不得自动持久化正式 Chain

public discover 只能生成 run-scoped Runtime-owned candidates，例如：

```text
.code-harness/runs/<runId>/analysis/discovered-chains/**
```

不得自动写：

```text
.code-harness/chains/**
```

正式持久化仍必须经过既有用户确认 / Runtime persistence authority。

### 不修改 Review Canonical ChangeSet

Task 2 不得修改：

- Snapshot algorithm；
- ChangeSet Authority；
- ChangeAnalysis Certification；
- Review zero-change lifecycle。

## 4.5 强制测试

Fixture：

```text
main == HEAD
working tree clean
```

代码：

```text
OrderController
  ↓
OrderService
  ↓
OrderServiceImpl
  ↓
OrderMapper
  ↓
OrderMapper.xml
```

执行：

```text
harness chain discover OrderController
```

必须验证：

- 成功发现完整 Chain；
- 不依赖非空 ChangeSet；
- candidate 是 Runtime-owned；
- provenance 可验证；
- 不写 `.code-harness/chains/**`；
- 不生成 `review.md`；
- Review affected discovery regression 全绿。

---

# 5. Task 3 — Multi-Chain Review Explicit User Selection

## 5.1 当前设计本身

当前 ReviewOptions 语义：

```text
0 Chain -> AUTO_FULL
1 Chain -> AUTO_SINGLE
2+ Chains -> USER_SELECTION
```

这个产品设计继续保留。

## 5.2 当前真实缺口

虽然 Runtime 能返回：

```text
USER_SELECTION
```

但后续 `review select` request 本质只包含类似：

```text
runId
mode
selectionIds
optionsHash
```

Runtime 能验证 selection 是否属于本次合法 options，但不能可靠判断该 FULL/LIST 是：

- 用户真实选择；还是
- Agent 自己替用户选择。

因此实际执行中可能出现：

```text
USER_SELECTION
-> Agent 自动构造 FULL
-> review select
-> units
-> dispatch
```

导致用户没有被询问。

## 5.3 正式要求：USER_SELECTION = 当前 Assistant Turn Hard Stop

当：

```text
decision = USER_SELECTION
```

Agent 当前 turn 必须：

1. 展示可选 Chain / Scope；
2. 询问用户；
3. **立即结束本轮执行。**

在下一条用户消息到来前，禁止调用：

- `review select`；
- `review units`；
- `review dispatch`；
- Finding Proposal；
- Finding Certification；
- Report Review。

### Turn 1 示例

用户：

```text
harness review
```

系统发现 3 条 Chain。

Agent 应只返回：

```text
本次变更涉及 3 条调用链：
1. Chain A
2. Chain B
3. Chain C

请选择：
A. 全部 Review
B. 选择指定调用链
```

然后结束本轮。

### Turn 2

用户：

```text
全部
```

才允许：

```text
review select FULL
-> review units
-> review dispatch
-> findings
-> report
```

## 5.4 禁止方案

不能只修改：

- `AGENTS.md`；
- orchestrator prompt；
- reviewer prompt；
- skill wording；

然后就宣称问题修复。

这些 active contracts 必须改，但验收必须依赖真实 Agent 行为。

同时，不要声称 Runtime 可以“证明这个 selection 一定来自真人”。本 Task 的目标是通过 active contract + same-session agent behavior + real E2E 锁定流程。

## 5.5 强制 Real OpenCode Same-session E2E

必须使用真实 OpenCode Agent Host，同一个 Session 两轮。

### Turn 1

用户 exact input：

```text
harness review
```

Fixture 产生 2+ Chains。

Assert：

```text
review options -> USER_SELECTION
```

并且本轮：

- Agent 询问用户；
- no `review select`；
- no `review units`；
- no dispatch；
- no report。

### Turn 2

同 Session 用户：

```text
全部
```

Assert：

```text
review select FULL
```

然后完整 authority chain 成功。

### 建议增加 Case

用户第二轮选择：

```text
1、3
```

验证 LIST/TARGETED selection。

---

# 6. Task 4 — Upgrade V2 Hash-Aware Delta Apply

## 6.1 当前机制

当前 Upgrade 核心是：

```text
Target -> Backup
Target -> Stage
Stage remove all managed paths
Source copy all managed paths into Stage
Validate
Apply staged managed tree
Rollback on failure
```

事务安全性较好，但 Apply 范围偏大。

## 6.2 Runtime 源码和 Runtime 可执行文件必须区分

### `tools-runtime/**`

这是 Go Runtime 源码。

最终用户 install / upgrade package 不需要包含它。

### `bin/codea-dcep-tools.exe`

这是运行时可执行文件。

它不能永远不升级，因为核心 Authority / lifecycle / certification / chain / upgrade 行为大量位于 Runtime 中。

正确原则：

```text
Runtime bytes unchanged -> SKIP
Runtime bytes changed -> UPDATE
```

而不是“Runtime 永远不替换”。

## 6.3 正式设计：Hash-aware Delta Apply

保留：

```text
Backup
Stage
Schema Validation
Apply
Rollback
```

只改变 Apply Scope。

对 source managed files 与 installed managed files 做 byte/hash comparison，分类：

```text
UNCHANGED
ADD
UPDATE
REMOVE
```

### UNCHANGED

完全不触碰目标文件。

### ADD

新增。

### UPDATE

替换。

### REMOVE

删除旧 managed file。

## 6.4 Runtime exe 特别规则

如果：

```text
installed bin/codea-dcep-tools.exe
==
package bin/codea-dcep-tools.exe
```

则必须：

```text
SKIP
```

不得执行：

- rename；
- park；
- replace；
- touch。

只有 Runtime 内容发生变化时，才进入现有 Windows running-executable replacement 逻辑。

## 6.5 用户发行包

最终用户 install / upgrade package 不得包含：

```text
tools-runtime/**
*.go
go.mod
go.sum
```

但仓库中 Runtime source 继续保留用于研发和构建。

## 6.6 Project State 边界必须保持

继续保留：

```text
harness.yaml
project.md
database.yaml
chains/**
runs/**
```

其中既有特殊边界：

```text
runs/README.md
```

属于 Framework-managed documentation，不得 regression。

## 6.7 Upgrade Plan UX

建议 preflight 输出清晰差量：

```text
Codea Harness Upgrade

Current: 1.6.2
Target: 1.6.3

Runtime
  codea-dcep-tools.exe   unchanged/update
  ast-grep.exe           unchanged/update

Framework
  Agents                 N changed
  Skills                 N changed
  Contracts              N changed

Project State
  harness.yaml           preserve
  project.md             preserve
  database.yaml          preserve
  chains/**              preserve
  runs/**                preserve
```

底层 delta apply 是本 Task 必需；完整交互页如果工作量较大可以分阶段，但至少要能让用户看到本次实际变化范围。

## 6.8 强制测试

### Case 1 — Runtime unchanged

确认：

- Runtime 未替换；
- running executable replace 未调用。

### Case 2 — Runtime changed

确认：

- 正常升级；
- Windows running exe 逻辑有效。

### Case 3 — Framework-only change

例如只修改一个 Skill，确认只更新对应变化文件。

### Case 4 — Managed file removed

老版本存在、新版本不存在，确认正确删除。

### Case 5 — Project State preserve

确认所有用户状态保持。

### Case 6 — Rollback

强制 apply 中途失败，确认：

- Framework bytes 恢复；
- Runtime bytes 恢复；
- Project State 不损坏。

### Case 7 — Release Package

确认最终 package 不包含 Runtime Go source。

---

# 7. 本轮额外发现问题汇总

## AST

1. 每个 AST pattern 重复扫描整个 `src/main/java`；
2. 单次 method/class lookup 会展开大量 pattern；
3. class/method/allTypes/call/inheritance 之间存在重复扫描；
4. `bufio.Scanner` 默认约 64KB token 上限可能无法承载大型 ast-grep JSON record；
5. 单纯增加 timeout 不是正式修复。

## Chain Discover

6. public `harness chain discover` 当前被绑定到 current Change Set；
7. clean branch 导致无 ChangeSet -> 无 Chain；
8. Project Discovery 与 Review Affected Discovery 需要拆分；
9. 不允许通过伪造 ChangeSet 解决。

## Review Selection

10. Runtime 已有 `USER_SELECTION` 语义；
11. Agent 仍可能自行构造 `review select FULL`；
12. prompt 中“需要问用户”缺少真实 same-session E2E 闭环；
13. 必须增加真实 OpenCode 两轮 E2E；
14. 不声称 Runtime 能证明 selection 一定来自真人。

## Upgrade

15. 当前 managed tree 大范围重新 apply；
16. 相同 Runtime exe 也可能被进入替换路径；
17. `tools-runtime/**` 是源码，不应该进入用户发行包；
18. Runtime exe 内容发生变化时仍必须升级；
19. Project State / backup / stage / schema validation / rollback 都不能因为“简化升级”而删除。

---

# 8. 1.6.2 已验收 Authority 不得修改

除非有明确 regression evidence，本轮禁止重新设计：

- Canonical ChangeSet Authority；
- Snapshot Authority；
- ChangeAnalysis Certification；
- ReviewUnit semantics；
- Rule Dispatch semantics；
- Finding Proposal boundary；
- Certified Findings Authority；
- Report Authority；
- zero-change full Review lifecycle；
- fresh Review invocation lifecycle。

特别要求：

- Task 2 不得修改 Review Snapshot / Canonical ChangeSet 算法；
- Task 3 不得扩大到 Finding / Report Authority 重构；
- Task 4 不得把 Project State 重新纳入 Framework managed tree。

---

# 9. 开发分支和验收节奏

建议：

```text
main
 ↓
hotfix/1.6.3-runtime-usability-task1
 ↓ Accepted Baseline
hotfix/1.6.3-runtime-usability-task2
 ↓ Accepted Baseline
hotfix/1.6.3-runtime-usability-task3
 ↓ Accepted Baseline
hotfix/1.6.3-runtime-usability-task4
 ↓ Accepted Baseline
Final Certification
```

每个 Task：

```text
RED
-> Implementation
-> Focused GREEN
-> Regression GREEN
-> Fresh exact-head CI
-> External Acceptance
```

不要四个 Task 同时开发后一次性交付。

---

# 10. 每个 Task 研发交付格式

研发完成一个 Task 后，回传：

```text
Task X 开发完成

Branch:
<branch>

Base / Previous Accepted Baseline:
<SHA>

Exact HEAD:
<SHA>

Changed Files:
...

RED Evidence:
...

Focused Tests:
<command>
PASS

Regression:
<command>
PASS

Fresh GitHub Actions:
workflow:
run id:
exact head:
conclusion: success

Known limitations:
...
```

在验收人明确返回：

```text
PASS / CLOSED
```

前，不进入下一 Task。

---

# 11. Final Certification

Task 1–4 全部验收后，执行 1.6.3 Final Certification。

至少包括：

```text
Task 1 large Java / AST performance gate
Task 2 clean-project Chain discovery gate
Task 3 real OpenCode same-session USER_SELECTION E2E
Task 4 upgrade delta / rollback / package gate

full Go regression
go vet
Windows Runtime exact-head build
pinned ast-grep regression
retained 1.6.2 Review Reliability real-agent E2E
retained same-session fresh Review E2E
exact HEAD verification
```

只有 Final Certification fresh exact-head PASS 后，才能形成 1.6.3 release accepted candidate。
