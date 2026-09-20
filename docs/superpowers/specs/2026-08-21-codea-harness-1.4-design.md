# Codea Harness 1.4 Design

> Status: APPROVED DESIGN
>
> Target version: **1.4.0**
>
> Baseline: Codea Harness 1.3.2 (`4eb44a2d8bfd7d2f7825815df2d06c49c0c5e48b`)

## 1. 目标

Codea Harness 1.4 不扩展运行平台、不引入 JDT LS、不建设 Maven Doctor，也不重做已经通过验收的 Review/Test/DB/Upgrade 主流程。

本版本聚焦四个真实使用问题：

1. **Targeted Review**：支持 `harness review <Class>` / `harness review <Class.method>`，在完整 Change Set 基础上只评审用户指定的业务调用链，并保持机器 Coverage 可证明。
2. **B 端 Resource Review**：把 MyBatis `*Mapper.xml` 与 `src/main/resources/**/*.yml` 纳入 Review Change Set 与正式 Finding Scope，只检查与本次变更相关的高价值风险。
3. **Human Report UX Standard**：统一所有正式人读 Markdown 的中文、首屏摘要、状态图标、调用链展示、Finding 展示和下一步提示。
4. **Runtime Apply Safety Gate**：把已批准的生产/测试代码写入从“Agent 自律”提升为 Runtime 强制校验，确保批准的 patch 与实际写入一致，并强制路径边界。

最终目标：让 Review 更精准、更适合 B 端、更容易读，同时把最危险的写操作补上机器强制力。

---

## 2. 非目标

1. 不做 `harness doctor`。
2. 不做 Maven offline / Nexus / Artifactory 自动适配。
3. 不增加 Linux/macOS/ARM64 发行包；1.4 继续 Windows x64。
4. 不支持 Gradle/Kotlin。
5. 不引入 JDT LS、JavaParser、Spoon 等新导航引擎。
6. 不引入 JaCoCo、PIT、SpotBugs、PMD、SARIF。
7. 不增加 Flyway/Liquibase SQL、`pom.xml`、任意 XML 的 Review；Resource Review 只包含 `*Mapper.xml` 与 `.yml`。
8. 不做 sampled review；任何 FULL/TARGETED Review 只有其声明 Scope 被完整覆盖后才能 COMPLETE。
9. 不删除 Orchestrator，不改成依赖宿主自行调度。
10. 不改变 Database Evidence、Test Provenance、Upgrade Transaction、Existing Test 保护等 1.2/1.3 已验收语义。

---

## 3. 总体架构

```text
User
  │
  ├─ harness review
  │      └─ FULL Review
  │
  ├─ harness review <Class>
  │      └─ TARGETED Review / class scope
  │
  ├─ harness review <Class.method>
  │      └─ TARGETED Review / method scope
  │
  └─ harness review list
         └─ 仅列出本次 Change Set 已识别调用链

Host Agent
  │
  ▼
Orchestrator
  │
  ├─ Review Scope Resolver
  │    ├─ FULL
  │    └─ TARGETED
  │
  ├─ Reviewer.analyze-change
  │    ├─ Java source/test
  │    ├─ Mapper.xml
  │    └─ yml
  │
  ├─ Runtime Coverage Verification
  │
  ├─ Reviewer.review-code / resource-review
  │
  └─ Runtime Report Renderer

Approved Fix/Test Patch
  │
  ▼
Runtime Apply Gate
  ├─ planId/fixPlanId
  ├─ unifiedDiff
  ├─ diffSha256
  ├─ base file hashes
  ├─ allowed/denied path policy
  ├─ atomic apply
  └─ machine result
```

---

# 4. Targeted Review

## 4.1 用户意图

1. `harness review`
   - 现有 FULL Review，语义保持不变。
2. `harness review list`
   - 只分析当前 Change Set，并列出已确认/可用的业务调用链，不执行 Finding Review。
3. `harness review OrderController`
   - TARGETED class review。评审该 Class 与当前 Change Set 有关的所有调用链。
4. `harness review OrderController.approve`
   - TARGETED method review。只评审该 method 对应且受当前 Change Set 影响的调用链。
5. `harness review OrderService` / `OrderService.approve`
   - 允许 Service 作为 target；若一个 target 解析到多条上游业务链，必须让用户选择，不得默认 ALL。

Targeted Review 不是历史全量代码审计。它始终围绕当前 Review Change Set，只把指定 target 相关的 changed code + call-chain context 纳入 Scope。

## 4.2 新 Contract

新增 ReviewIntent / ReviewScopeMode 概念：

```json
{
  "mode": "FULL | TARGETED",
  "target": {
    "symbol": "OrderController.approve",
    "kind": "CLASS | METHOD"
  }
}
```

FULL 时 `target = null`。

Target 解析后生成机器可验证的 `resolvedReviewScope`：

```json
{
  "mode": "TARGETED",
  "target": {
    "symbol": "OrderController.approve",
    "kind": "METHOD"
  },
  "selectedCallChains": [
    {
      "entryPoint": "OrderController.approve",
      "chain": [
        "OrderController.approve",
        "OrderService.approve",
        "OrderServiceImpl.approve",
        "OrderMapper.updateStatus"
      ]
    }
  ],
  "scopedFiles": [
    "src/main/java/.../OrderController.java",
    "src/main/java/.../OrderService.java",
    "src/main/java/.../OrderServiceImpl.java",
    "src/main/java/.../OrderMapper.java",
    "src/main/resources/mapper/OrderMapper.xml"
  ]
}
```

`selectedCallChains` / `scopedFiles` 必须来自 ChangeAnalysis + Code Navigation 证据，不得由 Renderer 或 Agent 任意补齐。

## 4.3 Coverage 语义

FULL：

```text
reviewScope.changedFiles ⊆ reviewedFiles
+ 全部相关 call-chain symbols 已解析/读取
```

TARGETED：

```text
resolvedReviewScope.scopedFiles ⊆ reviewedFiles
+ selectedCallChains 全部内部 symbols 已解析/读取
```

因此 Targeted Review 不要求评审与 target 无关的 changed files，但报告必须明确：

```text
评审模式：定向评审
评审目标：OrderController.approve
本次 Change Set 总文件：N
本次定向 Scope 文件：M
未纳入本次定向评审：N-M（不代表已评审）
```

禁止把 TARGETED 结果表述成整个 Change Set 已完整 Review。

## 4.4 多链选择

当 target 唯一解析到 1 条链：自动继续。

当解析到 2+ 条业务链：

- 宿主支持结构化多选时优先多选。
- 否则编号 fallback：`1` / `1,3` / `ALL`。
- 空选择/取消 → STOP。
- 禁止默认 ALL。

选择只是只读 Review Scope 选择，不等于测试/修复审批。

## 4.5 `harness review list`

输出本次 Change Set 已识别调用链：

```text
1. OrderController.approve
   → OrderService.approve
   → OrderServiceImpl.approve
   → OrderMapper.updateStatus

2. RefundController.refund
   → RefundService.refund
   → ...
```

只列已确定性识别链；candidate/unresolved 必须单独标注，不得包装成 confirmed chain。

---

# 5. B 端 Resource Review

## 5.1 Scope

新增默认配置：

```yaml
scope:
  sourceIncludes:
    - src/main/java/**/*.java
  testIncludes:
    - src/test/java/**/*.java
  mapperIncludes:
    - src/main/resources/**/*Mapper.xml
  configIncludes:
    - src/main/resources/**/*.yml
```

1.4 只新增 `mapperIncludes` 与 `configIncludes`。

## 5.2 Mapper.xml Review

Mapper.xml 属于生产代码 Finding Scope，重点检查：

1. UPDATE / DELETE 缺失 WHERE 或条件明显过宽。
2. 本次变更移除/弱化已有租户、机构、用户等数据隔离条件。
3. 动态 SQL 条件变化导致关键过滤条件可能失效。
4. `resultMap/resultType` 与 Java Mapper/DTO/Entity 的本次变更不一致。
5. XML statement id 与 Java Mapper method 的新增/修改不一致。
6. 参数名/参数结构与 Mapper method 本次变更不一致。
7. 明显无边界批量更新/删除风险。

不得因为 XML 格式、缩进、命名风格产生普通 Finding。

V1.4 不要求新建完整 MyBatis 编译器。可使用受控 XML 解析 + 现有 SQL Parser 对可提取 SQL 做确定性候选检查；无法确定动态 SQL 最终语义时允许 LLM 结合 Java Mapper 与 XML 证据判断，但不得伪造确定性结论。

## 5.3 YML Review

`.yml` 属于生产配置 Finding Scope，重点检查本次变更：

1. 数据库连接/连接池关键参数。
2. timeout、线程池、队列等容量与超时配置。
3. Redis/MQ/RPC endpoint/timeout/retry 等配置。
4. 日志级别被异常提升或关闭关键日志。
5. Spring Profile / feature switch 行为变化。
6. 敏感信息直接写入配置。
7. 配置 key 删除/改名与 Java `@Value` / `@ConfigurationProperties` 使用不一致。

不得对未变化的大量配置做泛化审查。

## 5.4 Coverage

Mapper/YML 一旦进入 FULL Review changedFiles，必须被读取并进入机器 Coverage；读取失败 → PARTIAL。

TARGETED Review 只在资源文件与 selected call chain / target 有明确关联时纳入 scopedFiles。无法证明关联时不得强行加入 Targeted Scope；但报告可提示“该资源文件属于 Change Set，未纳入本次定向评审”。

---

# 6. Human Report UX Standard

## 6.1 全局原则

除以下内容外，所有用户可见固定文案默认中文：

- 源码类名、方法名、字段名、路径
- SQL / JSON / YAML / XML 原文
- 第三方技术名词，如 Spring Boot、MySQL、Redis
- 机器 Contract enum（仅 JSON，不直接作为 UI）

## 6.2 首屏标准

所有正式人读 Markdown 第一屏必须优先回答：

1. 结果是什么？
2. 检查了什么？
3. 有几个问题？
4. 用户下一步做什么？

Review 示例：

```markdown
# 🔍 代码评审报告

| 项目 | 内容 |
|---|---|
| 评审结果 | ❌ 未通过 |
| 评审模式 | 🎯 定向评审 |
| 评审目标 | `OrderController.approve` |
| 调用链 | 1 条 |
| 涉及文件 | 7 个 |
| 问题 | 2 个 |

### 问题概览

🔴 严重 0　🟠 高 1　🟡 中 1　🟢 低 0
```

## 6.3 调用链展示

Renderer 根据 symbol role 展示：

```text
🌐 接口入口
OrderController.approve
↓
⚙️ 业务接口
OrderService.approve
↓
🧠 业务实现
OrderServiceImpl.approve
↓
🗄 数据访问
OrderMapper.updateStatus
↓
📄 Mapper XML
OrderMapper.xml#updateStatus
```

role label 属于展示层，不修改机器 symbol。

## 6.4 Finding 展示

固定结构：

```text
### 🟠 F-001｜高

📍 位置
...

❗ 问题
...

🔎 证据
...

💥 影响
...

🛠 修复建议
...

🧪 是否需要测试
是/否
```

CRITICAL/HIGH/MEDIUM/LOW 的机器枚举继续保留；展示分别为 🔴严重 / 🟠高 / 🟡中 / 🟢低。

## 6.5 统一“下一步”区块

FAILED：提示优先处理阻断问题，可使用 `harness fix finding:<id>`。

PASSED：明确“无需处理阻断问题”。

MANUAL_ACTION_REQUIRED：列出缺失证据/未解析项和用户需要完成的具体动作。

TARGETED：必须提示“本结论只覆盖本次定向 Scope，不代表整个 Change Set 已完成评审”。

## 6.6 适用范围

1.4 至少统一：

- `review.md`
- 诊断/验证类正式 Markdown（若当前已有）
- Upgrade/Apply 的正式用户摘要

API Doc 不属于 1.4 范围。

---

# 7. Runtime Apply Safety Gate

## 7.1 原则

Agent 可以分析、生成 patch、生成 Fix/Test Plan；正式写入生产/测试代码必须通过 Controlled Runtime。

新增：

```text
codea-harness-tools apply --input .code-harness/runs/<runId>/requests/apply.json
```

禁止把完整 patch 作为 CLI 参数；只允许结构化 request file。

## 7.2 Apply Contract

最小结构：

```json
{
  "runId": "...",
  "planType": "TEST | FIX",
  "planId": "fix-001",
  "diffSha256": "...",
  "files": [
    {
      "path": "src/main/java/.../OrderServiceImpl.java",
      "baseSha256": "..."
    }
  ],
  "unifiedDiff": "..."
}
```

Runtime 必须：

1. 校验 request 路径在 `.code-harness/runs/<runId>/requests/`。
2. 校验 Schema，拒绝 unknown fields。
3. 重新计算 `unifiedDiff` SHA256 == `diffSha256`。
4. 读取当前文件，重新计算 `baseSha256`；不一致 → `BASE_CHANGED`，拒绝写入。
5. 根据 `harness.yaml.write.allowedTestPaths / allowedProductionPaths / deniedPaths` 强制判断文件路径。
6. `TEST` 计划只能写 allowedTestPaths；`FIX` 计划只能写 allowedProductionPaths。
7. deniedPaths 永远优先拒绝。
8. unified diff 内实际 touched files 必须与 files[] 一致。
9. Patch 必须完整、无路径穿越、无 `.git/**`、无 Harness Framework Managed 文件写入。
10. 使用临时文件/备份执行原子或可回滚写入；任意文件失败不得留下半应用状态。
11. 成功输出实际 changed files、before/after SHA256、diffSha256。

## 7.3 Approval 边界

1.4 不声称 Runtime 可以证明“是谁点击了宿主 UI 的批准”。

1.4 的强保证是：

```text
批准时展示/确认的 diffSha256
=
Runtime 收到的 diffSha256
=
Runtime 实际应用的 unifiedDiff
```

如果审批后计划内容改变，必须生成新的 diffSha256；旧批准不得复用。

Orchestrator/Skill 继续要求精确 `批准 <planId>`；Runtime 负责保证“批的是 A，写的也是 A”，而不是把宿主身份认证问题伪装成已经解决。

## 7.4 审计结果

每次 apply 在 run 下生成机器可信结果：

```text
.code-harness/runs/<runId>/evidence/apply/<planId>.json
```

至少包含：

- runId
- planType
- planId
- diffSha256
- status
- appliedAt
- files[path,beforeSha256,afterSha256]
- rollbackPerformed
- errorCode(optional)

同一 `planId` 成功应用后不得重复应用。

---

# 8. Contract 变化

预计新增/修改：

1. `contracts/change-analysis.schema.json`
   - 支持资源文件 role/type。
   - 保持 callChains 结构。
2. 新增 `contracts/review-scope.schema.json`
   - FULL / TARGETED / target / selectedCallChains / scopedFiles。
3. 新增 `contracts/apply-request.schema.json`。
4. 新增 `contracts/apply-result.schema.json`。
5. `harness-config.schema.json`
   - 新增 `scope.mapperIncludes` / `scope.configIncludes`。
6. Review report transport
   - 增加 mode/target/totalChangeSetFiles/scopedFiles/selectedCallChains 信息。

机器 Contract 字段使用稳定英文 enum；Renderer 负责中文展示。

---

# 9. Task 拆分

## Task 1 — Targeted Review

交付：FULL/TARGETED Review、`review list`、Class/Method target resolution、多链选择、Scoped Coverage。

验收关键点：

- 原 `harness review` 无 regression。
- `harness review OrderController` 可只评审该 Controller 相关链。
- `harness review OrderController.approve` 可只评审指定 method 链。
- Service target 多链时必须选择，不默认 ALL。
- TARGETED 报告明确未覆盖整个 Change Set。
- Runtime 机器 Coverage 验证 scopedFiles，而不是相信 Agent COMPLETE。

## Task 2 — Mapper.xml / YML Review

交付：默认 Scope、读取/分类/coverage、资源文件 Finding 规则、Mapper/XML 与 YML 定向关联。

验收关键点：

- Mapper.xml/YML changed file 不再静默逃过 FULL Review。
- 测试 XML/YML 风格问题不得产生 Finding。
- 关键 WHERE/tenant/config 变化可产出 evidence-backed Finding。
- TARGETED 只包含与 target 有证据关系的资源文件。

## Task 3 — Human Report UX Standard

交付：统一中文、首屏摘要、调用链 role UI、Finding UI、下一步区块。

验收关键点：

- 同一输入 deterministic。
- FULL/TARGETED/PARTIAL/PASSED/FAILED 均有 golden。
- 用户可见固定文案不残留英文 UI 标签。
- Targeted 报告首屏明确 target/scope。

## Task 4 — Runtime Apply Safety

交付：Apply Contract、Runtime apply、diff/base hash/path gate、事务写入、evidence。

验收关键点：

- hash mismatch / base changed / denied path / scope violation 全部在 0 写入前拒绝。
- 多文件 patch 中任一失败必须完整回滚。
- TEST/FIX 路径边界不能互串。
- 同 planId 不可重复成功 apply。
- Agent 无法绕过 Runtime 通过既有 Skill 完成正式写入。

## Task 5 — Release / Upgrade / Windows Gate

交付：1.4.0 version、升级兼容、正式 install/upgrade ZIP、Windows exact-head CI。

验收关键点：

- 1.3.2 → 1.4.0 Project State 字节级保护不退化。
- 新 contracts/skills/runtime 文件进入 Framework Managed replace。
- stale managed files 正常删除。
- 正式包无 Project State。
- 全量 Go test/vet、Targeted Review golden、Resource Review golden、Apply golden、Navigation smoke、live upgrade 全部通过。

---

# 10. 兼容性与迁移

1. `harness review` 默认继续 FULL，旧用户无行为变化。
2. 旧 `harness.yaml` 升级时 registered migration 增加默认：

```yaml
scope:
  mapperIncludes:
    - src/main/resources/**/*Mapper.xml
  configIncludes:
    - src/main/resources/**/*.yml
```

3. migration 只能 Runtime 执行；不得由 Agent 猜配置。
4. 用户已有自定义 scope 时必须保留已有值，并按 migration 规则补新字段；不能覆盖用户配置。
5. Runtime Apply 上线后，相关 Agent/Skill 文档必须移除“宿主直接 write_file 即可完成正式写入”的语义，正式写入必须走 apply gate。

---

# 11. 1.4 完成定义

只有同时满足以下条件才可发布 1.4.0：

1. FULL Review 1.3.2 语义无 regression。
2. TARGETED Review 有机器可证明 scoped coverage。
3. Mapper.xml/YML 纳入 FULL Review，并支持 target 相关资源的 scoped review。
4. 人读报告遵守统一 UX 标准。
5. 生产/测试正式写入由 Runtime Apply 强制路径/hash/patch 一致性。
6. 1.3.2 → 1.4.0 Windows upgrade gate 通过。
7. exact-head install/upgrade artifacts 生成成功。

本版本完成后，下一阶段再根据真实试点数据决定是否推进：affectedEntryPoints（Job/MQ/Dubbo）、大 Diff 分片、review.json/SARIF、JDT LS、Maven Doctor 等能力。
