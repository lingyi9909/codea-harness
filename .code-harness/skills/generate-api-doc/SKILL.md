---
name: generate-api-doc
description: 从 Controller/DTO/Enum/Validation/Direct Service evidence 生成 api-doc.schema.json 结构化数据。
version: 1
---

# Generate API Doc

## 输入

来自 `discover-api` 的已选 API targets。

## 分析顺序

对每个 endpoint 固定执行：

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

支持：

- `@RequestBody`
- `@RequestParam`
- `@PathVariable`
- `@RequestHeader`：仅业务 Header

`required`、字段名、类型、validation 必须来自代码。

Validation 支持：

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

硬限制：

- 最大递归深度：3。
- 使用 visited-type set 做 cycle detection。
- 达到深度或出现环时停止继续展开，但保留已确认字段 type。
- 不因无法展开而编造字段。

## Enum

只允许真实源码 enum value。无法解析 enum class 时：

```text
type = <原类型>
enumValues = []
```

禁止模型生成“常见值”。

## Business / Evidence

所有 semantic statement：

```json
{
  "statement": "...",
  "status": "CONFIRMED | INFERRED | UNKNOWN",
  "evidence": ["path/File.java:line"]
}
```

规则：

- `CONFIRMED`：直接源码证据，evidence 至少 1 条。
- `INFERRED`：仅在允许推断且有 supporting evidence 时；evidence 至少 1 条。
- `UNKNOWN`：源码范围确实无法确定且需要显式呈现时；evidence 可空。
- 无内容时输出 `[]`，不要机械填 UNKNOWN。

### permissions
仅代码可见 annotation/guard/direct service check。

### preconditions
仅显式参数/状态/业务前置判断。

### businessFlow
只描述前端需要理解的 direct service flow，不暴露后端实现细节。

### stateTransitions / dataEffects / externalEffects / transactions / idempotency / testCoverage
只有允许分析深度内存在明确 evidence 才填；Task 3 不建立额外深层分析引擎。

### errorCodes
只允许 Controller 或 Direct Service Method 内显式：

```text
BizException
ErrorCode
assert/guard 显式错误码
```

禁止从数据库、日志、经验猜 error code。

### businessNotes
只写前端调用/交互需要知道的信息；禁止记录 Repository/SQL/线程/缓存等内部实现细节。

## Example

允许生成合理 example value，但以下内容必须来自代码：

```text
field name
type
enum
required
response shape
error code
```

## 输出与 Runtime Gate

输出必须构造成：

```json
{
  "runId": "...",
  "harnessVersion": "...",
  "apiDoc": {
    "controllers": []
  }
}
```

`apiDoc` 必须通过 `.code-harness/contracts/api-doc.schema.json`，然后写到：

```text
.code-harness/runs/<runId>/requests/api-doc.json
```

只调用 Controlled Runtime：

```text
codea-harness-tools report api-doc --input .code-harness/runs/<runId>/requests/api-doc.json
```

最终 Artifact 固定：`.code-harness/runs/<runId>/api-doc.md`。
