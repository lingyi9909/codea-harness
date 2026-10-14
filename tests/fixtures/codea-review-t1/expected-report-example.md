# 参考报告格式——非实际 OpenCode TUI 运行结果

**状态：EXAMPLE_ONLY / NOT_RUN。严禁用本文件充当真实模型验收证据。**

## Codea Review（示例）

- 范围：`master...HEAD`（真实 SHA 待运行时填写）
- 执行：示意 `COMPLETE`（仅展示排版，并未运行）
- 已确认：示例 HIGH 2 条

1. [HIGH] `src/main/java/demo/orders/OrderService.java`：进程内重复请求保护被移除。同一 requestId 的重试会再次调用扣款操作，可能重复扣款。建议恢复或升级为可靠的持久化幂等约束，验证同请求重复提交。
2. [HIGH] `src/main/resources/mapper/OrderMapper.xml`：账户余额更新取消 tenant_id 条件，若跨租户 order_id 非全局唯一，可能误更新其他租户数据。建议恢复租户约束并验证跨租户相同 orderId。

- 关联业务入口示例：`OrderController.charge → OrderService.charge → OrderMapper.decreaseBalance`。
- 未验证：多实例情况下旧逻辑的幂等能力、真实数据库唯一键与业务 SQL 全量调用入口。
- 以上只是人工准备的“预期观察点”，**不是由 OpenCode/私有模型实际产生的报告**。
