---
name: generate-api-doc
description: 从 Controller/DTO/Enum/Validation/Direct Service evidence 生成 api-doc.schema.json 结构化数据。
version: 1
---

# Generate API Doc

## 输入

来自 `discover-api` 的已选 API targets。

## 分析顺序

```text
Controller method
→ Request parameter / Request DTO
→ Response DTO/VO
→ nested DTO / wrapper
→ Enum
→ Validation
→ Direct Service Method（最多一层）
→ STOP
```

不得继续到 Repository / Mapper / DAO / DB / MQ / Redis / RPC Server。

## Request

支持且必须显式映射参数位置：

```text
@RequestBody   → location=BODY
@RequestParam  → location=QUERY
@PathVariable  → location=PATH
@RequestHeader → location=HEADER（仅明确业务 Header）
```

每个 request field 必须输出：

```text
name
type
location = BODY | QUERY | PATH | HEADER
required
description
validation[]
enumValues[]
```

`location` 与 `validation` 是不同语义。禁止把 `@PathVariable`、`@RequestParam`、`@RequestHeader`、`@RequestBody` 写进 `validation[]`。

Validation 只允许代码中的校验约束：

```text
@NotNull
@NotBlank
@NotEmpty
@Size
@Length
@Min
@Max
@DecimalMin
@DecimalMax
@Pattern
@Valid
```

## DTO / Response 展开

支持 regular DTO、nested DTO、`List<T>`、`PageResult<T>`、`Result<T>`。

- 最大递归深度 3。
- 使用 visited-type set 做 cycle detection。
- 达到深度或出现环时停止继续展开，但保留已确认字段 type。
- 不因无法展开而编造字段。

## Enum

只允许真实源码 enum value。无法解析 enum class 时：

```text
type = <原类型>
enumValues = []
```

## Business / Evidence

所有 semantic statement：

```json
{
  "statement": "...",
  "status": "CONFIRMED | INFERRED | UNKNOWN",
  "evidence": ["path/File.java:line"]
}
```

- CONFIRMED：直接源码证据，evidence >= 1。
- INFERRED：仅允许有 supporting evidence 的受限推断，evidence >= 1。
- UNKNOWN：源码范围内确实无法确定且需要显式呈现时使用。
- 无内容优先 `[]`。

`permissions / preconditions / businessFlow / stateTransitions / dataEffects / externalEffects / transactions / idempotency / testCoverage / businessNotes` 只在允许分析深度内有证据时填写。

`errorCodes` 只允许 Controller 或 Direct Service Method 内显式 `BizException / ErrorCode / assert/guard` 证据。

## Example

example value 可以合理生成，但 field name/type/location/enum/required/response shape/error code 必须来自代码。

## 输出与 Runtime Gate

输出：

```json
{
  "runId": "...",
  "harnessVersion": "...",
  "apiDoc": {"controllers": []}
}
```

`apiDoc` 必须通过 `.code-harness/contracts/api-doc.schema.json`，transport 写入：

```text
.code-harness/runs/<runId>/requests/api-doc.json
```

随后只调用：

```text
codea-dcep-tools.exe report api-doc --input .code-harness/runs/<runId>/requests/api-doc.json
```

最终 Artifact 固定：`.code-harness/runs/<runId>/api-doc.md`。
