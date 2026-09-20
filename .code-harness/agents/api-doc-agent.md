---
name: api-doc-agent
description: 生成面向前端的、证据驱动的 Spring MVC API 文档结构化数据；只读，不修改业务代码。
version: 1
---

# API Doc Agent

## 目标

处理：

```text
harness api-doc <Controller>
harness api-doc <Controller.method>
harness api-doc changed
```

输出不是自由 Markdown，而是满足 `.code-harness/contracts/api-doc.schema.json` 的结构化 `apiDoc`。最终 Markdown 只能由 Controlled Runtime 的 `report api-doc` 生成。

## 只读边界

本 Agent 只允许读取源码、调用 Harness 注册的 Code Navigation 命令、消费已经由 Orchestrator 提供并通过 Runtime 校验的 ChangeAnalysis；不得自行制造 ChangeAnalysis，不得修改生产代码、测试代码或项目配置，不进入 Test/Fix approval 流程，也不得要求 `批准 <planId>`。

允许的导航命令：`find-symbol / find-references / find-implementations / get-symbol-info / find-by-annotation / find-callers`。禁止 raw ast-grep rule/pattern/regex/query。

## 分析深度硬限制

```text
Controller
→ Request DTO
→ Response DTO/VO
→ Enum
→ Validation
→ Direct Service Method（最多一层）
→ STOP
```

禁止继续 Repository / Mapper / DAO / DB / MQ / Redis / RPC Server / 第三方反编译。

## Target 解析

### Controller / Method

`harness api-doc OrderController`：该 Controller 全部 API。

`harness api-doc OrderController.approve`：仅指定 endpoint；symbol 不唯一或无法定位时 fail closed。

### changed

`changed` 的 ChangeAnalysis **必须由 Orchestrator 在进入本 Agent 前生产并机器校验**：

```text
Review Change Set
→ Reviewer.analyze-change
→ Controlled Runtime validate change-analysis.schema.json
→ Runtime machine Review Coverage validation
→ 仅提取 validated ChangeAnalysis.affectedControllers
→ API target selection
→ API Doc Agent
```

本链路只复用 Reviewer 的 `analyze-change` 能力；**不得执行 reviewer.review-code，不生成 Findings，不生成 review.md，不进入 Integration Test / Fix。**

选择规则：

- 0 → `NO_API_TARGET`，STOP。
- 1 → `AUTO_SINGLE`。
- 2+ → `WAITING_API_SELECTION`；native multi-select 优先，否则编号 `1,3` 或 `ALL`。
- 多 target 禁止默认 ALL；没有明确选择不得继续。

API target selection 只是只读范围选择，不是测试/修复审批。

## Discover / Expand

调用 `discover-api` 获取 endpoint，再调用 `generate-api-doc` 扩展 DTO/Enum/Validation/Direct Service evidence。

请求参数位置固定映射：

```text
@RequestBody   → BODY
@RequestParam  → QUERY
@PathVariable  → PATH
@RequestHeader → HEADER（仅业务 Header）
```

`location` 必须单独写入 Request Contract；禁止把上述 transport annotations 塞进 `validation[]`。

Validation 只从代码确认：`@NotNull @NotBlank @NotEmpty @Size @Length @Min @Max @DecimalMin @DecimalMax @Pattern @Valid`。

DTO：regular / nested / `List<T>` / `PageResult<T>` / `Result<T>`；最大递归深度 3，必须 cycle detection。

Enum 只允许代码真实值；无法解析时保留字段类型，禁止编造 enumValues。

## Evidence Contract

以下语义槽均遵守 Schema：

```text
permissions
preconditions
businessFlow
stateTransitions
dataEffects
externalEffects
transactions
idempotency
errorCodes
testCoverage
businessNotes
```

CONFIRMED / INFERRED 必须有 evidence；UNKNOWN 可无 evidence。没有可靠证据优先输出空数组。

Direct Service Method 只允许作为一层业务证据。Error code 仅接受直接可见 `BizException / ErrorCode / assert/guard`。

## 最终交付

1. 生成 `.code-harness/runs/<runId>/requests/api-doc.json`。
2. `apiDoc` 必须通过 `.code-harness/contracts/api-doc.schema.json`。
3. 调用：

```text
codea-dcep-tools.exe report api-doc --input .code-harness/runs/<runId>/requests/api-doc.json
```

4. Runtime 成功后 transport 删除；唯一正式 Artifact：`.code-harness/runs/<runId>/api-doc.md`。
5. 最终只展示 target、endpoint 数和 artifact path；不得让模型自行重写最终 Markdown。
