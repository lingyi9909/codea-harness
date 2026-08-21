---
name: review-code
description: 在 FULL 或 Runtime 已验证的 TARGETED Review Scope 完整后执行 Finding Review；生产代码按证据评审，测试代码只执行 Test Validity Gate。
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

FULL 任一条件不满足，本 Skill **不得执行**，由 Orchestrator 输出 `MANUAL_ACTION_REQUIRED`。

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

Runtime verified ReviewScopeSelection 是本 Skill 唯一允许使用的定向边界；Agent 原始 target/scopedFiles/selectedCallChains 不得覆盖机器验证结果。

## Review Finding Scope

### TARGETED 硬边界

TARGETED 时：

1. 只读取并评审 Runtime verified `selectedCallChains` 与 `scopedFiles`。
2. `Finding.file` 必须属于 Runtime verified scopedFiles。
3. scope 外 changed file 可以出现在完整 Change Set 中，但不得产生本次 Targeted Finding。
4. 发现 scope 外潜在问题时不得顺手输出 Finding；用户需要 FULL Review 或新的 Targeted Review 才能正式评审。
5. selectedCallChains/scopedFiles 的任何 Runtime verification 失败 → STOP / `MANUAL_ACTION_REQUIRED`。

### 生产代码

FULL 时对 `src/main/**` 等生产代码正常执行 Code Review；TARGETED 时仅对 verified scopedFiles 中的生产代码执行。Finding **不要求位于 Controller**；问题真实发生在哪一层，就落在哪一层，例如：

- Controller
- Service / ServiceImpl
- Repository / Mapper / DAO
- Validator
- DTO / VO / Entity
- Config / ExceptionHandler / Utility

生产代码 Finding 固定：

```text
category = PRODUCTION_CODE
```

可检查参数校验、业务规则、状态流转、事务边界、权限/租户、幂等性、异常处理、数据一致性、空指针、边界条件、调用链和兼容性问题。

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

`problem / evidence / impact / recommendation` 默认使用中文；Java 类名、方法名、文件路径、SQL、异常名、RPC 名和技术名词保持源码原文。

没有证据不得报问题；不做无关重构或风格建议。

## 输出

必须通过 `.code-harness/contracts/review-output.schema.json`；TARGETED 正式 Review Report 还必须通过 Controlled Runtime 的 scoped Finding 校验。
