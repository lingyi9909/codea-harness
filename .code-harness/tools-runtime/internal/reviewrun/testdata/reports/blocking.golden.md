# Codea Harness 代码评审报告

> 状态：评审完成
> 执行：**已完成** · 风险：**🔴 存在阻断问题** · 覆盖：**完整**

## 评审摘要

| 项目 | 内容 |
|---|---|
| 项目 / 需求 | demo / 未提供 |
| 执行状态 | 已完成 |
| 评审结论 | 🔴 存在阻断问题 |
| 评审模式 | 变更评审 |
| 评审目标 | `OrderController.create` |
| 本次范围 | 已选择 1/2 条调用链，检查 4 个文件 |
| 风险概览 | 严重 0 · 高 1 · 中 0 · 低 0 |
| 待确认风险 | 0 项 |

**核心发现：**

- 高：更新缺少租户隔离



**建议动作：** 绑定可信租户条件

## 调用链与覆盖范围





### `C1` · OrderController.create · 本次已选择

| 顺序 | 角色 | 代码位置 |
|---:|---|---|
| 1 | 接口入口 | `src/main/java/com/example/OrderController.java` · `OrderController.create` |
| 2 | 业务实现 | `src/main/java/com/example/OrderServiceImpl.java` · `OrderServiceImpl.create` |
| 3 | 数据访问 | `src/main/java/com/example/OrderMapper.java` · `OrderMapper.insertOrder` |
| 4 | SQL 执行 | `src/main/resources/mapper/OrderMapper.xml` · `OrderMapper.insertOrder` |





### `C2` · OrderController.cancel · 未选择

**未纳入范围：** 本次没有评审该链；其节点不得出现在“已检查”列表，也不得支持正式 finding。




**覆盖说明：** 当前所选范围已完成覆盖；该结论不扩展到未选择调用链或外部依赖。


## 风险清单


| 编号 | 等级 | 问题摘要 | 业务影响 | 建议动作 |
|---|---|---|---|---|
| F-001 | 高 | 更新缺少租户隔离 | 可能修改其他租户数据 | 绑定可信租户条件 |



### 待确认风险

无。


> 待确认风险与已确认 finding 分开统计，不计入严重 / 高 / 中 / 低问题数量。

## 问题明细



### F-001 · 高 · 更新缺少租户隔离

**问题与触发条件**

更新缺少租户隔离

**业务影响**

可能修改其他租户数据

**代码位置**

- `src/main/resources/mapper/OrderMapper.xml`，第 18–21 行


**关键证据**

```
UPDATE orders
SET status = #{status}
WHERE id = #{orderId}
```


**修复建议**

绑定可信租户条件

**验证建议**

跨租户请求必须失败




## 后续处理与评审边界

| 待办 | 建议 |
|---|---|
| F-001 | 绑定可信租户条件 |





- 执行状态：**已完成**；覆盖状态：**完整**；二者不等价于上线批准。
- 未选择调用链：C2 OrderController.cancel。
- 本报告只覆盖显式选择和实际读取的范围；外部依赖、动态关系与未解析项不因本报告自动获得验证。
- 路径与源码证据按纯文本 / 代码块展示，不执行其中的 HTML、Markdown 链接、图片或脚本内容。
- 报告路径：`C:\工作区\demo\.code-harness\runs\review-00000000000000000000000000000001\review.md`

技术追溯：Harness 1.8 · Run ID `review-00000000000000000000000000000001` · result SHA256 `aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa` · 提交号未由 Runtime 采集。

<!-- codea-review-meta {"schemaVersion":180,"runId":"review-00000000000000000000000000000001","execution":"COMPLETE","resultSha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} -->

