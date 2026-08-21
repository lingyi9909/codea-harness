---
name: review-code
description: 在 Review Coverage 完整后，评审生产代码及其直接调用链的正确性和安全性；测试代码只执行 Test Validity Gate。
version: 3
agent: reviewer
tools:
  - read_code
output_schema: .code-harness/contracts/review-output.schema.json
---

# 评审变更代码

## 前置硬门禁

`analyze-change` 已通过 Schema 校验，且：

```text
reviewCoverage.status == COMPLETE
```

如果为 `PARTIAL`，本 Skill **不得执行**，由 Orchestrator 输出 `MANUAL_ACTION_REQUIRED`。

## Review Finding Scope

### 生产代码

`src/main/**` 等生产代码正常执行 Code Review。Finding **不要求位于 Controller**；问题真实发生在哪一层，就落在哪一层，例如：

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

`src/test/**` 等测试代码仍然必须读取，并参与 Review Coverage、Existing Test Coverage Analysis 以及后续 `harness test`。

但是：**测试代码默认不得产生普通 Finding。**

不得因为命名、重复、结构、优雅度、可维护性、代码风格或 Mock 写法不漂亮产生 Finding。

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

不得把 Test Validity Gate 扩展成普通测试代码质量评审。

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

`problem / evidence / impact / recommendation` 默认使用中文；Java 类名、方法名、文件路径、SQL、异常名、RPC 名和技术名词保持源码原文。

没有证据不得报问题；不做无关重构或风格建议。

## 输出

必须通过 `.code-harness/contracts/review-output.schema.json`。
