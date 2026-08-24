---
name: review-code
description: 在 FULL 或 Runtime 已验证的 TARGETED Review Scope 完整后执行 Finding Review；Java/Mapper/YML 生产变更按证据评审，测试代码只执行 Test Validity Gate。
version: 4
agent: reviewer
tools:
  - read_code
output_schema: .code-harness/contracts/review-output.schema.json
---

# 评审变更代码

## 前置硬门禁

本 Skill 不得自行决定 Review 是否完整，必须消费 Controlled Runtime 已验证的门禁结果。

### FULL

```text
mode == FULL
change-analysis.schema.json == VALID
reviewCoverage.status == COMPLETE
Runtime FULL Coverage == COMPLETE
```

FULL 任一条件不满足，本 Skill **不得执行**，由 Orchestrator 输出 `MANUAL_ACTION_REQUIRED`。changed Mapper.xml/YML 与 Java/test 一样属于 FULL required set，漏读任何一个都不能执行 Finding Review。

### TARGETED

```text
mode == TARGETED
review-scope.schema.json == VALID
Runtime verified ReviewScopeSelection == VERIFIED
Scoped Coverage == COMPLETE
verified selectedCallChains != empty
verified scopedFiles != empty
```

TARGETED 不得仅凭 Agent 声明的 reviewCoverage.status == COMPLETE 放行。Full Change Set 的 `reviewCoverage.status` 可以因为 scope 外文件未读取而为 PARTIAL；TARGETED 是否可进入 Finding Review，只看 Runtime 对 ReviewScopeSelection 重新计算后的 Scoped Coverage。

Runtime verified ReviewScopeSelection 是本 Skill 唯一允许使用的定向边界；Agent 原始 target/scopedFiles/selectedCallChains 不得覆盖机器验证结果。Mapper.xml/YML 只有经 `resourceRelations` + Runtime 验证后才能进入 Targeted Finding Scope。

## Review Finding Scope

### TARGETED 硬边界

TARGETED 时：

1. 只读取并评审 Runtime verified `selectedCallChains` 与 `scopedFiles`。
2. `Finding.file` 必须属于 Runtime verified scopedFiles。
3. scope 外 changed file 可以出现在完整 Change Set 中，但不得产生本次 Targeted Finding。
4. 与 selected chain 无 evidence relation 的 Mapper.xml/YML 不得顺手纳入。
5. 发现 scope 外潜在问题时不得顺手输出 Finding；用户需要 FULL Review 或新的 Targeted Review 才能正式评审。
6. selectedCallChains/scopedFiles 的任何 Runtime verification 失败 → STOP / `MANUAL_ACTION_REQUIRED`。

### Java 生产代码

FULL 时对 `src/main/**` Java 生产代码正常执行 Code Review；TARGETED 时仅对 verified scopedFiles 中的生产代码执行。Finding **不要求位于 Controller**；问题真实发生在哪一层，就落在哪一层，例如 Controller、Service/ServiceImpl、Repository/Mapper/DAO、Validator、DTO/VO/Entity、Config/ExceptionHandler/Utility。

生产代码 Finding 固定：

```text
category = PRODUCTION_CODE
```

可检查参数校验、业务规则、状态流转、事务边界、权限/租户、幂等性、异常处理、数据一致性、空指针、边界条件、调用链和兼容性问题。

### Mapper.xml

进入 FULL required set 或 TARGETED verified scopedFiles 的 `*Mapper.xml` 使用：

```text
category = PRODUCTION_CODE
```

只允许对本次变更有明确证据的高价值风险产生 Finding：

1. `UPDATE / DELETE` 缺失 `WHERE`，或本次变更使 WHERE 条件明显过宽。
2. 本次变更移除/弱化已有租户、机构、用户等数据隔离条件。
3. `动态 SQL` 条件变化导致关键过滤条件可能失效。
4. XML `statement id` 与 Java Mapper method 的新增/修改不一致。
5. `参数` 名/参数结构与 Mapper method 本次变更不一致。
6. `resultMap/resultType` 与 Java Mapper/DTO/Entity 的本次变更不一致。
7. 明显`无边界批量更新/删除`风险。

**不得因为 XML 格式、缩进、命名风格产生 Finding。**

动态 SQL 无法确定最终 SQL 语义时，只能把可见 XML + Java Mapper 证据作为分析依据；证据不足时不报确定性 Finding。不得把候选风险伪造成已确认问题。

### YML

进入 FULL required set 或 TARGETED verified scopedFiles 的 `src/main/resources/**/*.yml` 使用：

```text
category = PRODUCTION_CODE
```

只检查本次 changed key 对以下高价值行为的影响：

1. `datasource` / 数据库连接池关键参数。
2. `timeout`、线程池、队列容量与超时。
3. `Redis/MQ/RPC` endpoint / timeout / retry。
4. `日志级别`异常提升或关闭关键日志。
5. Spring profile / `feature switch` 行为变化。
6. hard-coded secret / `敏感信息`直接写入配置。
7. 配置 key 删除/改名与 Java `@Value` / `@ConfigurationProperties` 使用不一致。

**不得对未变化的配置做泛化审查。** 不得因为 YAML 排版、key 顺序、空行、注释风格产生 Finding。

### 测试代码

`src/test/**` 等测试代码只要进入 FULL required set 或 TARGETED verified scopedFiles 就必须读取，并参与 Review Coverage、Existing Test Coverage Analysis 以及后续 `harness test`。

但是：**测试代码默认不得产生普通 Finding。**

不得因为以下内容产生 Finding：

```text
命名不好
重复代码
测试结构不好
测试代码不够优雅
可维护性一般
代码风格
Mock 写法不漂亮
```

测试代码只执行 **Test Validity Gate**。只有存在明确代码证据表明测试失真、产生 false-positive 或真实覆盖被破坏时，才允许：

```text
category = TEST_VALIDITY
```

允许范围固定为：

1. 删除有效测试。
2. 使用 `@Disabled` / Ignore 等方式禁用有效测试。
3. 删除或明显弱化关键断言。
4. catch / 吞异常导致测试无条件通过。
5. Mock 内部业务 Bean，导致真实业务调用链被绕过。
6. 修改测试范围，使本次生产变更实际没有被验证。
7. 其他具有明确代码证据的 false-positive 行为。

不得把 Test Validity Gate 扩展成普通测试代码质量 Review。

## Review Chain Context（1.5 Task 4）

`review-code` 可以消费已由 Controlled Runtime 解析完成的 `chainContext`，但 **chainContext 只提供业务上下文**，不改变本 Skill 的任何前置 Gate。

- `ACCEPTED + VALID` 可以作为当前 Review 的已验证长期业务上下文。
- `DISCOVERED + TEMPORARY` 只能作为本次 run 临时上下文；**临时 DISCOVERED Chain 不授权 Project State 写入**。
- STALE/INVALID/PARTIAL Chain context 不允许进入 Finding Review。
- **Finding.file 仍由原 FULL/TARGETED Scope Gate 决定**；不能因为某文件存在于 Chain 就绕过 verified scopedFiles/完整 Change Set 规则。
- Chain 的 `notes` 不得覆盖实际代码证据；Finding 的 problem/evidence/impact/recommendation 仍必须来自本次变更与已读取源码。
- 使用临时 Chain 不等于接受/保存该 Chain；沉淀必须回到 Orchestrator 的用户明确确认 + Task 3 Runtime persist 流程。

Report transport 的 `chainContext` 只能复制 Runtime 已验证的 `id/name/source/status`，Renderer 只负责展示 provenance，不参与 Chain 判断。

## Finding 规则

每条 Finding 必须引用实际证据，并完整记录：

- `id`
- `category`：`PRODUCTION_CODE | TEST_VALIDITY`
- `severity`
- `file`, `line` —— 真实问题位置，不得为了“接口入口”硬挂 Controller
- `problem`
- `evidence`
- `impact`
- `recommendation`
- `needsTest`
- `introducedByChange`
- `confidence`

TARGETED 在输出前必须再次检查 `Finding.file ∈ verified scopedFiles`；Controlled Runtime Renderer 还会执行同一范围校验，形成双层 Gate。

`problem / evidence / impact / recommendation` 默认使用中文；Java 类名、方法名、文件路径、SQL、YAML/XML 原文、异常名、RPC 名和技术名词保持源码原文。

没有证据不得报问题；不做无关重构或风格建议。

## 输出

必须通过 `.code-harness/contracts/review-output.schema.json`；Resource Finding 不新增 category，Mapper.xml/YML 继续使用 `PRODUCTION_CODE`。TARGETED 正式 Review Report 还必须通过 Controlled Runtime 的 scoped Finding 校验。
