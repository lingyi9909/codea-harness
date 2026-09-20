# Codea Harness 1.5 Chain Review Design

> Status: APPROVED DESIGN
>
> Target version: **1.5.0**
>
> Baseline: Codea Harness 1.4.0 (`bedf2cde3784a6ee15d408271a023a95570c46b8`)

## 1. 目标

Codea Harness 1.5 只建设一套**实用、可读、可人工修正、可长期沉淀**的业务调用链（Business Chain）能力，并只接入 `harness review`。

本版本不建设全仓调用图，不把 Chain 扩展到 Test / Debug / Fix / Verify，不引入复杂规则引擎，也不引入新的 Java 导航引擎。

1.5 的核心目标：

1. 以生产 Controller Method 作为候选业务入口，按需发现当前 Change Set 或用户指定 target 相关的业务调用链。
2. 将多个入口与共享核心业务链分离，支持常见 V1 / V2 Controller 共用同一 Service 链路的 B 端场景。
3. 用固定 YAML 格式沉淀用户确认后的 Chain，使开发人员可以直接阅读、修改、提交 Git。
4. 用户修改后的 Chain 必须经过 Runtime + Code Navigation 重新验证，不能把 YAML 中不存在的代码事实当真。
5. 已确认 Chain 作为 Project State 保存到 `.code-harness/chains/**`，Upgrade 永不覆盖。
6. `harness review` 优先复用有效 Chain；不存在或过期时 Lazy Discover，再完成 Review。

一句话：

> Framework 负责发现和验证；用户负责确认和修正；`chains/**` 负责长期沉淀；Review 负责消费。

---

## 2. 非目标

1. 不做 Test / Debug / Fix / Verify 的 Chain 化。
2. 不做全仓永久 Call Graph / Graph Database。
3. 不在 `harness init` 时扫描并生成全部 Controller Chain。
4. 不做复杂相似度算法，不做 80%/90% 模糊合并。
5. 不做通用 Rule Engine、Pattern Rule、自动学习规则。
6. 不做 `chain merge/split/edit/ignore` 交互命令；第一版允许开发直接改 YAML。
7. 不引入 JDT LS / JavaParser / Spoon。
8. 不扩展 Job / MQ / Dubbo / EventListener EntryPoint；1.5 第一版只做生产 Controller Method。
9. 不扩展新的 Resource 类型；继续只消费 Java、`*Mapper.xml`、`src/main/resources/**/*.yml`。
10. 不改变 1.4 的 Review Coverage、Runtime Apply Safety、DB Evidence、Upgrade transaction 等既有语义。
11. 不增加 Linux/macOS/Gradle/Maven Doctor/SARIF。

---

# 3. 核心概念

## 3.1 EntryPoint

Chain 的候选入口是**生产 Controller Method**，不是 Controller Class。

例如：

```text
OrderController.create
OrderController.approve
OrderController.cancel
```

是三个 EntryPoint。

EntryPoint 必须：

1. 来自生产源码，不允许 `src/test/**` / TestSource。
2. 由 Code Navigation 证明是 `@Controller` / `@RestController` 中的 endpoint method。
3. 与当前 Change Set 有影响关系，或用户通过 `harness chain discover <target>` 显式指定。
4. exact symbol/path 必须可确定；ambiguous/unresolved 不得沉淀为 ACCEPTED Chain。

明显的测试、Demo、Mock、Sample Controller 可作为辅助排除信号，但类名不能取代源码 role evidence。

## 3.2 Business Chain

Business Chain 表示开发真正关心的一条 B 端业务执行路径：

```text
Controller.method
→ Service interface / service
→ Service implementation
→ Repository / Mapper
→ Mapper.xml
→ 外部边界（可选）
```

停止边界沿用现有 Review 设计：

```text
Repository / Mapper
RPC Client
MQ Client
Cache Client
第三方 SDK
JDK / Spring Framework
```

禁止无界扫描。

## 3.3 EntryPoint 与 Business Chain 分离

一个 Business Chain 可以有多个 EntryPoint。

典型 V1/V2：

```text
OrderControllerV1.approve ┐
                          ├→ OrderService.approve
OrderControllerV2.approve ┘   → OrderServiceImpl.approve
                              → OrderMapper.updateStatus
```

此时保存一条 Business Chain，`entryPoints` 有两个入口。

如果 V2 进入不同业务核心：

```text
OrderControllerV2.approve
→ OrderV2Service.approve
→ RiskService.check
→ OrderMapper.updateStatus
```

则必须独立成另一条 Chain。

## 3.4 简单 Canonicalization 规则

1. 不根据 Controller 名称、V1/V2 后缀直接合并。
2. 去掉 EntryPoint 自身后，如果两个入口的**verified core path 完全一致**，允许归并为同一个 Business Chain。
3. Core path 至少包含业务 Service / implementation / data access 节点；Controller 层 DTO 转换等入口适配差异不要求完全一致。
4. 只支持确定性 exact-match；不做模糊相似度。
5. 自动归并结果仍可由用户通过编辑 YAML 拆开；编辑后必须 validate。

---

# 4. Chain 生命周期

第一版只定义三种状态：

```text
DISCOVERED
ACCEPTED
STALE
```

## DISCOVERED

当前 Run 自动发现出的临时 Chain，尚未进入项目长期知识。

位置：

```text
.code-harness/runs/<runId>/analysis/discovered-chains/*.yaml
```

## ACCEPTED

开发人员已经确认/修改并通过 Runtime validate 的 Chain。

位置：

```text
.code-harness/chains/*.yaml
```

## STALE

已保存 Chain 中的源码事实与当前代码不再一致，例如：

- symbol 不存在；
- exact path 改变；
- 调用关系失效；
- Mapper resource 不存在；
- 已确认入口不再是合法 production Controller endpoint。

STALE Chain 不得静默作为可信 Review Scope 使用。

Review 可在用户明确选择时使用本次重新发现的临时 Chain 继续，但不能把旧 ACCEPTED 文件偷偷改写。

---

# 5. Chain YAML Contract

每条 Business Chain 一个 YAML 文件。

目录：

```text
.code-harness/chains/order-approve.yaml
```

## 5.1 固定格式

```yaml
version: 1

id: order-approve
name: 订单审批
status: ACCEPTED

entryPoints:
  - symbol: OrderControllerV1.approve
    path: src/main/java/com/example/order/OrderControllerV1.java

  - symbol: OrderControllerV2.approve
    path: src/main/java/com/example/order/OrderControllerV2.java

nodes:
  - symbol: OrderService.approve
    path: src/main/java/com/example/order/OrderService.java
    role: SERVICE

  - symbol: OrderServiceImpl.approve
    path: src/main/java/com/example/order/OrderServiceImpl.java
    role: SERVICE

  - symbol: OrderMapper.updateStatus
    path: src/main/java/com/example/order/OrderMapper.java
    role: MAPPER

resources:
  - path: src/main/resources/mapper/OrderMapper.xml
    symbol: OrderMapper.updateStatus
    role: MAPPER_XML

boundaries:
  - symbol: PaymentRpcClient.notify
    path: src/main/java/com/example/client/PaymentRpcClient.java
    role: EXTERNAL

notes: |
  V1、V2 接口共用同一套订单审批核心逻辑。
```

## 5.2 字段约束

### `version`

Chain 文件格式版本。1.5 固定为：

```text
1
```

与 Harness VERSION 分离。

### `id`

项目内唯一、稳定、人类可读：

```text
^[a-z0-9][a-z0-9-]{0,63}$
```

默认由 Runtime 从 canonical entry/core symbol 生成候选值；用户可以修改，但不得与现有 Chain 冲突。

### `name`

用户可读业务名称，必须非空。自动发现时允许生成保守名称；用户可直接修改。

### `status`

只允许：

```text
DISCOVERED
ACCEPTED
STALE
```

长期 `.code-harness/chains/**` 正常只保存 `ACCEPTED` 或明确标记的 `STALE`；临时发现结果默认 `DISCOVERED`。

### `entryPoints[]`

至少 1 个。每个包含：

```text
symbol
path
```

必须由 Navigation 验证为 production Controller endpoint method。

### `nodes[]`

核心业务节点，保持真实调用顺序。

第一版 role 只需要：

```text
SERVICE
REPOSITORY
MAPPER
OTHER
```

role 必须来自验证 evidence；无法可靠识别用 `OTHER`，禁止通过类名后缀猜事实。

### `resources[]`

第一版只允许：

```text
MAPPER_XML
YAML_CONFIG
```

只有能够证明与 Chain 有明确 relation 的资源才进入。

### `boundaries[]`

可选。用于展示到达外部边界后的停止点。

第一版 role：

```text
EXTERNAL
CACHE
MQ
```

不继续进入外部服务实现。

### `notes`

用户自由补充的业务上下文。

`notes` 不参与 Runtime 调用关系真实性判断，不允许 `notes` 覆盖机器事实。

## 5.3 文件顶部说明

Framework 生成 YAML 时固定加入注释：

```yaml
# Codea Harness Business Chain
#
# 这是项目长期业务 Chain，可直接编辑。
# 修改后请执行：harness chain validate <id>
# 代码结构变化后请执行：harness chain refresh <id>
# symbol/path/call relation 必须真实存在，Runtime 会重新校验。
# 本文件属于 Project State，Harness 升级不会覆盖。
```

---

# 6. Chain Schema 与 Runtime 真实性验证

新增：

```text
.code-harness/contracts/chain.schema.json
```

Schema 负责格式校验；Runtime 负责代码事实校验。

## 6.1 Runtime validate 必须验证

1. YAML schema valid。
2. `id` 与文件名一致，例如 `order-approve.yaml → id=order-approve`。
3. entryPoint symbol 存在且 exact path 唯一。
4. entryPoint 是 production Controller endpoint，不允许 TestSource。
5. nodes 每个 symbol 存在且 exact path 匹配。
6. nodes 的顺序对应真实 confirmed call relation。
7. resources path 存在、role/path 双向一致，并有 verified resource relation。
8. boundary symbol/path 真实存在；外部停止语义合法。
9. 同一个项目内 Chain id 不重复。
10. `ACCEPTED` Chain 任何核心事实失效 → validate 失败并返回 STALE 建议，不得静默修复。

Runtime 输出结构化 `ChainValidationResult`：

```json
{
  "chainId": "order-approve",
  "status": "VALID | STALE | INVALID",
  "errors": [],
  "warnings": []
}
```

机器 enum 保持英文，用户摘要中文。

---

# 7. 目录与所有权边界

## 7.1 Framework Managed

1.5 新增 Framework 文件：

```text
.code-harness/contracts/chain.schema.json
.code-harness/contracts/chain-validation-result.schema.json
.code-harness/templates/chain.template.yaml
.code-harness/skills/discover-chain/SKILL.md
.code-harness/skills/validate-chain/SKILL.md
.code-harness/tools-runtime/internal/chain/**
```

如确有必要可增加：

```text
.code-harness/agents/chain-agent.md
```

但优先复用 Reviewer + Orchestrator，YAGNI：没有必要不要新增 Agent。

这些属于 Framework Managed，Upgrade 可以 replace。

## 7.2 Project State

正式扩展为：

```text
.code-harness/harness.yaml
.code-harness/project.md
.code-harness/database.yaml
.code-harness/chains/**
.code-harness/runs/**
```

其中：

```text
chains/**
```

是团队长期沉淀的业务知识，Upgrade 必须保护。

## 7.3 Runtime State

临时发现结果：

```text
.code-harness/runs/<runId>/analysis/discovered-chains/**
```

不得自动复制进 Project State。

---

# 8. Lazy Chain Discovery

## 8.1 用户命令

第一版只支持：

```text
harness chain list
harness chain show <id|target>
harness chain discover [target]
harness chain refresh <id>
harness chain validate [id]
```

仍然是 Agent intent，不定义独立用户 CLI。

Controlled Runtime 可以增加对应 `chain` 子命令用于确定性验证/存取。

## 8.2 `harness chain discover`

无 target：

1. 计算当前 Review Change Set。
2. 从 changed production symbols 向上找受影响 Controller endpoint。
3. 只构建这些 EntryPoint 的 Chain。
4. 测试代码不得作为 EntryPoint。
5. 输出到 `runs/<runId>/analysis/discovered-chains/`。
6. 不自动写入 `chains/**`。

有 target：

```text
harness chain discover OrderController
harness chain discover OrderController.approve
```

只解析 target 相关入口。

## 8.3 Discovery 流程

```text
Change Set / target
↓
Candidate EntryPoint
↓
Code Navigation exact evidence
↓
Service / Impl / Mapper
↓
Mapper.xml / relevant yml
↓
Boundary
↓
Canonicalize exact same core path
↓
DISCOVERED Chain YAML
```

任何 unresolved internal symbol：

```text
CHAIN_PARTIAL
```

不得生成伪 ACCEPTED Chain。

## 8.4 V1/V2

只在 verified core path 完全一致时自动归并。

自动归并后的 `entryPoints[]` 保留全部入口；Review 仍需读取各自 changed entry-specific code，不能因为共享 core 就忽略 Controller 自身差异。

---

# 9. Chain 管理

## 9.1 `harness chain list`

默认列项目长期 Chain：

```text
✅ order-approve  订单审批
✅ order-cancel   订单取消
⚠ refund-apply   退款申请（链路已过期）
```

并可单独显示本次 Run 新发现的 DISCOVERED Chain。

## 9.2 `harness chain show`

固定中文可读输出：

```text
订单审批

状态：✅ 已确认

入口：
① OrderControllerV1.approve
② OrderControllerV2.approve

核心链路：
🌐 OrderController.approve
   ↓
⚙️ OrderService.approve
   ↓
🧠 OrderServiceImpl.approve
   ↓
🗄 OrderMapper.updateStatus
   ↓
📄 OrderMapper.xml#updateStatus

说明：
V1、V2 共用核心审批逻辑。
```

展示角色只能来自 verified role evidence；无可靠 role 显示普通代码节点。

## 9.3 用户修改

第一版直接允许开发编辑：

```text
.code-harness/chains/*.yaml
```

修改后必须：

```text
harness chain validate <id>
```

用户可修改：

- name；
- entryPoints；
- nodes；
- resources；
- boundaries；
- notes。

但 Runtime 不允许用户通过 YAML 虚构：

- 不存在的 symbol/path；
- 不存在的调用关系；
- 错误的资源类型；
- Test Controller 冒充 production EntryPoint。

## 9.4 接受 DISCOVERED Chain

第一版不增加 `chain accept` 用户命令。

Orchestrator 在用户明确确认“保存/沉淀这条 Chain”后：

1. 将 DISCOVERED YAML 复制为 Project State candidate；
2. status 改为 `ACCEPTED`；
3. Runtime validate；
4. validate 成功才写入 `.code-harness/chains/<id>.yaml`；
5. 失败则 0 Project State 修改。

这属于 Project State 写入，不得覆盖同 id 现有文件，除非用户明确是在更新该 Chain。

---

# 10. Refresh 与 STALE

## 10.1 使用前验证

Review 使用 ACCEPTED Chain 前至少做轻量 validate：

```text
entryPoints/nodes/resources exact evidence 仍成立？
```

失败：

```text
CHAIN_STALE
```

不得静默继续用旧 Chain。

## 10.2 `harness chain refresh <id>`

流程：

```text
Existing Accepted Chain
+
current source navigation
↓
new discovered candidate
↓
展示差异
↓
用户确认
↓
validate
↓
replace Project State chain
```

例如：

```text
原链：
OrderServiceImpl.approve → OrderMapper.update

当前代码发现：
OrderServiceImpl.approve → RiskService.check → OrderMapper.update

建议：
+ RiskService.check
```

没有用户确认，禁止自动覆盖。

## 10.3 第一版不做规则学习

用户修正后的**完整 Chain YAML 本身就是沉淀结果**。

1.5 不增加 `chain-rules.yaml`。

只有真实使用证明出现大量重复修正后，再考虑后续规则泛化。

---

# 11. Review 接入

## 11.1 FULL Review

`harness review`：

1. 计算完整 Change Set。
2. 查找与当前变更有关的 ACCEPTED Chains。
3. 对命中的 Chain 做 validate。
4. valid Chain 直接提供 verified context。
5. 无 Chain 的受影响入口 Lazy Discover 临时 Chain。
6. FULL Coverage 仍以完整 Change Set 为硬 Gate，不因为 Chain 存在降低 Coverage。
7. Finding 仍只能针对本次变更/受影响行为，不把长期 Chain 当历史全库审计。

## 11.2 TARGETED Review

`harness review OrderController.approve`：

优先：

```text
target
→ accepted chain lookup
→ validate
→ targeted scoped review
```

如果不存在：

```text
target
→ lazy discover
→ run chain
→ targeted review
```

如果已有 Chain STALE：

```text
提示链路过期
→ 使用本次重新发现的临时 Chain继续评审（用户明确选择）
或
→ refresh 项目 Chain
```

不得自动覆盖 Project State。

## 11.3 Chain 不替代 Change Set

必须始终保持：

```text
Change Set = 变化事实边界
Chain = 业务上下文边界
```

TARGETED Review 的 scoped coverage 继续使用 1.4 Runtime Gate。

FULL Review 不能因为 Chain 只覆盖部分 changed files 就宣布 COMPLETE。

## 11.4 Review Report

在现有 1.4 中文 UX 基础上增加：

```text
业务链：订单审批
Chain ID：order-approve
Chain 来源：项目已确认 / 本次临时发现
Chain 状态：已确认 / 临时 / 已过期
```

如果使用临时 DISCOVERED Chain：

```text
⚠️ 本次评审使用临时发现的业务链，尚未沉淀到项目 Chain。
```

评审结束可提示：

```text
本次识别到新的业务链“订单审批”。
是否沉淀到项目 `.code-harness/chains/`？
```

---

# 12. Upgrade / Release

## 12.1 `chains/**` 正式成为 Project State

1.5 起 Upgrade Runtime 必须保护：

```text
harness.yaml
project.md
database.yaml
chains/**
runs/**
```

`chains/**` 不属于 Framework Managed，永远不能进入 stale-framework deletion。

## 12.2 1.4.0 → 1.5.0

1.5 尽量不修改 `harness.yaml`，不提升 harness config version。

升级只做：

```text
Framework Replace
+
安装 chain schema/template/skill/runtime
+
保留全部 Project State
```

如果 `.code-harness/chains/` 不存在，不要求 Upgrade 创建；第一次保存 Chain 时创建即可。

## 12.3 Chain Schema Version

每个 Chain 自带：

```yaml
version: 1
```

未来 Chain schema 升级：

1. 默认兼容旧格式则不修改用户文件。
2. 必须迁移时，只允许 Registered Deterministic Chain Migration。
3. Migration 前 backup，后 validate，失败 rollback。
4. Agent 禁止自行批量改写用户 Chain。

## 12.4 Release Package

正式 install/upgrade ZIP 包含：

```text
contracts/chain.schema.json
contracts/chain-validation-result.schema.json
templates/chain.template.yaml
chain skills/runtime
```

禁止包含任何业务实例：

```text
chains/*.yaml
```

## 12.5 Windows Upgrade Gate

必须真实验证：

1. accepted 1.4.0 baseline → 1.5.0。
2. 升级前手工放置 `.code-harness/chains/order-approve.yaml`。
3. 记录 Chain 文件 SHA256。
4. 升级成功后 SHA256 byte-for-byte 不变。
5. `contracts/chain.schema.json`、template、runtime 已安装。
6. stale Framework 正确删除。
7. `.code-harness-upgrade/` / stage / backup 清理。
8. 新 Runtime 能运行 `chain validate` probe。
9. install/upgrade ZIP 都不得携带业务 `chains/*.yaml`。

---

# 13. 安全与可信边界

1. Agent 不得把自己生成的 Chain 直接声明为机器可信。
2. exact path / symbol / call relation 必须来自 Code Navigation evidence。
3. 人工 YAML 可以修改业务表达，但必须重新 validate。
4. notes 是上下文，不是代码事实。
5. ACCEPTED ≠ 永久有效；源码改变后仍可能 STALE。
6. Upgrade 永不覆盖用户 Chain。
7. Review 使用 Chain 时不得降低 FULL/TARGETED Coverage Gate。
8. TestSource 永远不得成为正式 EntryPoint。
9. ambiguous/unresolved internal symbol 不得沉淀为 ACCEPTED。
10. 第一版宁可 PARTIAL/STALE，也不自动猜复杂业务链。

---

# 14. 用户体验示例

## 首次使用

```text
用户：harness review OrderController.approve

Harness：
当前项目尚未沉淀该业务 Chain，正在按当前代码解析。

发现：
订单审批（临时）
OrderController.approve
→ OrderService.approve
→ OrderServiceImpl.approve
→ OrderMapper.updateStatus
→ OrderMapper.xml#updateStatus

随后执行定向 Review。

Review 完成后：
是否将“订单审批”沉淀到项目 Chain？
```

用户确认后：

```text
.code-harness/chains/order-approve.yaml
```

## 后续使用

```text
harness review OrderController.approve
```

Harness：

```text
已找到项目 Chain：订单审批
状态：✅ 已确认
验证：✅ 与当前源码一致
```

直接复用。

## 用户修正

开发打开：

```text
.code-harness/chains/order-approve.yaml
```

补充漏掉节点后：

```text
harness chain validate order-approve
```

验证成功后，后续 Review 使用修正后的 Chain。

## 代码变化

```text
harness review OrderController.approve
```

发现旧 Chain 失效：

```text
⚠️ 项目 Chain“订单审批”已过期。
OrderServiceImpl.approve 已不存在。

当前代码重新发现：
OrderController.approve
→ OrderApprovalService.approve
→ OrderMapper.updateStatus

请选择：
1. 使用临时链继续本次评审
2. 刷新项目 Chain
3. 停止
```

---

# 15. Task 拆分

1. **Task 1 — Chain Contract & Project State**
   - Chain YAML schema/template；
   - validation result contract；
   - `chains/**` Project State；
   - Upgrade/managed boundary 单元测试。

2. **Task 2 — Lazy Discovery & Canonicalization**
   - production Controller Method candidate；
   - current Change Set / explicit target discovery；
   - test entry exclusion；
   - exact navigation；
   - V1/V2 exact core-path canonicalization；
   - DISCOVERED YAML 输出到 runs。

3. **Task 3 — Chain Management**
   - list/show/discover/refresh/validate；
   - accept/save flow；
   - STALE detection；
   - 用户编辑后 Runtime validate；
   - 不做复杂 rule engine。

4. **Task 4 — Review Consumes Chain**
   - FULL/TARGETED lookup；
   - accepted chain validation/reuse；
   - missing/stale lazy fallback；
   - Report Chain 来源/状态；
   - Coverage 不退化。

5. **Task 5 — 1.5 Release / Upgrade / Windows Gate**
   - VERSION/README/CHANGELOG；
   - exact 1.4→1.5 live upgrade；
   - `chains/**` byte-for-byte preservation；
   - dual ZIP；
   - exact-head Windows Gate / Artifact。

---

# 16. 1.5 最终验收标准

```text
1. Chain 最小入口是 production Controller Method。
2. TestSource/Test Controller 不产生 Business Chain。
3. Discovery 默认为 Lazy，不做全仓预生成。
4. V1/V2 只有 verified core path 完全一致才自动归并。
5. 一条 Chain 一个 YAML，开发可直接阅读/修改。
6. 人工修改必须 Runtime validate；不存在的 symbol/path/call relation 不得通过。
7. DISCOVERED Chain 只进 runs/**；用户确认后才进 chains/**。
8. chains/** 属于 Project State，Upgrade byte-for-byte 保留。
9. ACCEPTED Chain 与源码不一致时变 STALE，不得静默继续。
10. Review 优先复用 valid Accepted Chain；没有则 Lazy Discover。
11. Chain 不替代 Change Set，不降低 FULL/TARGETED Coverage Gate。
12. Review 报告明确 Chain 名称、来源、状态。
13. 1.4→1.5 Windows live upgrade 真实通过。
14. Install/Upgrade 包不携带业务 Chain 实例。
15. 不实现本设计明确列出的非目标。
```
