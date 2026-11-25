# 查询与Schema探索

完整运行规则在[Skill正文](../SKILL.md)。本文件供安装人员与审核人员阅读，不要求Agent额外读取文件。

## 查询顺序

1. 确认系统授权源、业务对象、过滤、列、时间和输出要求。
2. 先按Skill第0节分类：只知道业务ID/订单号或需业务解释属于BIZ，必须先做TeamAI定位；仅明确授权source、表/列且不涉及业务语义的TECH查询才可跳过TeamAI直接进入Schema。
3. search_objects核对实际表、列、类型、索引及键。关联说明分别标元数据事实、数据核验或知识候选。
4. 生成单条无副作用SELECT；用户提供SQL执行同样校验。明确列、稳定排序、LIMIT，禁止越库及敏感原值读取。
5. 按真实返回给中文结果，列出源、条件、SQL、证据和限制。不能把错误当零行。

## 合成示例

**前提：下列`orders`、`payments`指同一个已授权DBHub source内的两张获准数据表，并非两个DBHub sources；表间JOIN或子查询只可在该前提下使用。** 两张表如果实际分属不同source，则本版只查询当前授权源并报告证据缺口，不能跨源生成SQL。

以下仅为文档示例，表名不代表公司Schema，必须先核实。`orders`中的amount_minor为整数分，currency为币种；样例时间列是UTC DATETIME。主键id唯一。用户7的订单10001、10002均为CNY 100分，10003为USD 250分，10004为CNY NULL；10001状态PAID，其余PENDING。四条创建时间均为2026-01-01 00:00:00 UTC。payments包含订单10001两条SUCCEEDED流水、10002一条SUCCEEDED流水。状态含义是本合成夹具的预先约定，真实部署需业务核实。

“查订单10001的编号、状态和金额”：

```sql
SELECT id, status, amount_minor, currency
FROM orders WHERE id = 10001 LIMIT 1;
```

期望1行PAID、100分CNY；按本夹具单位可展示1.00 CNY。

“查用户7在UTC 2026-01-01内待支付的订单，按编号升序展示前2条”：

```sql
SELECT id, status, amount_minor, currency
FROM orders
WHERE user_id = 7 AND status = 'PENDING'
  AND created_at >= '2026-01-01 00:00:00' AND created_at < '2026-01-02 00:00:00'
ORDER BY id ASC LIMIT 2;
```

期望10002、10003；说明可能有下一页，不能说总共2条。追问“下一页”必须保留过滤和排序，以最后id续查：

```sql
SELECT id, status, amount_minor, currency
FROM orders
WHERE user_id = 7 AND status = 'PENDING'
  AND created_at >= '2026-01-01 00:00:00' AND created_at < '2026-01-02 00:00:00'
  AND id > 10003
ORDER BY id ASC LIMIT 2;
```

期望10004；金额NULL显示未知，不填零。分页间数据可能改变，不声称一致快照。

“查一个不存在的编号”：

```sql
SELECT id, status FROM orders WHERE id = 99999 LIMIT 1;
```

期望空表，回答“本源按此条件未返回记录”。用户SQL引用未知列时先探索并解释；工具仍报错时记录真实错误，最多一次有依据纠错。权限/超时错误直接停止。

## 不同数据源时如何回答

例如DBHub只授权`orders`源，但需要的付款记录实际仅存在于`payments`源：可以查询`orders`源内已获准的订单表，**不得**执行`JOIN payments...`（这里的`payments`是另一个source，不是本源获准的表），也不得改用`payments`源的search_objects/execute_sql。输出示例：“Q1已核实当前源订单10002状态为PENDING；Q2支付成功状态尚缺`payments`源证据，不能确定是否已成功支付。需管理员为该源独立审核账号与工具授权，并由人工分源核对流水及时间口径；V1.0不自动执行跨源调查。”
