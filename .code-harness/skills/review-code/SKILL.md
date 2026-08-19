---
name: review-code
description: 在 Review Coverage 完整后，评审变更及其直接调用链的正确性和安全性，发现落在问题真实发生的文件/行。
version: 2
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

## 评审范围

评审完整 Change Set 以及已被 `reviewCoverage.reviewedFiles` 证明读取的直接调用链文件。Finding **不要求位于 Controller**；问题真实发生在哪一层，就落在哪一层，例如：

- Controller
- Service / ServiceImpl
- Repository / Mapper / DAO
- Validator
- DTO / VO / Entity
- Config / ExceptionHandler / Utility

## 检查项

1. 参数校验：null、空值、范围、DTO/VO 约束。
2. 业务规则：变更逻辑、边界场景、直接调用链中的业务约束。
3. 状态流转：合法前置状态、非法流转、并发修改。
4. 事务边界：`@Transactional` 范围、回滚条件、多 Repository 写入一致性。
5. 权限/租户：身份、角色、租户/机构隔离和上下文传递。
6. 幂等性：重复请求、唯一约束、幂等键。
7. 异常处理：异常层级、统一响应、敏感信息、资源释放。
8. 数据一致性：查询过滤、UPDATE 条件、缓存失效时机。

## Finding 规则

每条 Finding 必须引用实际证据，并记录：

- `id`, `severity`
- `file`, `line` —— **真实问题位置，不得为了“接口入口”硬挂 Controller**
- `evidence`, `impact`, `recommendation`
- `needsTest`, `introducedByChange`, `confidence`

没有证据不得报问题；不做无关重构/风格建议。

## 输出

必须通过 `.code-harness/contracts/review-output.schema.json`。
