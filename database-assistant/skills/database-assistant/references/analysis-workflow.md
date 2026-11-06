# 统计分析规则与可核对示例

完整运行规则在[Skill正文](../SKILL.md)。先确定粒度、去重键、时间/时区、状态/租户过滤、币种与单位、NULL含义、分子分母；不明确时按需TeamAI，仍缺失就向用户澄清。

以下沿用[合成夹具](query-workflow.md)，不是公司业务定义。金额用整数分，实际DECIMAL列保留精度，不转FLOAT。

## 币种、NULL与聚合

```sql
SELECT currency, COUNT(*) AS row_count, COUNT(amount_minor) AS known_amount_count,
       SUM(amount_minor) AS total_minor, MIN(amount_minor) AS min_minor,
       MAX(amount_minor) AS max_minor, AVG(amount_minor) AS avg_minor
FROM orders
WHERE user_id = 7 AND created_at >= '2026-01-01 00:00:00'
  AND created_at < '2026-01-02 00:00:00'
GROUP BY currency ORDER BY currency LIMIT 100;
```

期望CNY：3条、已知金额2条、合计200分、最小/最大/平均100分；USD：1条、合计/平均250分。明确平均值只包含非NULL；未知金额仍未知。数据库实际返回类型/精度以MySQL为准，不能将这两个币种总金额相加。

```sql
SELECT COUNT(*) AS row_count, SUM(amount_minor) AS total_minor, AVG(amount_minor) AS avg_minor
FROM orders WHERE id = 99999;
```

期望(0,NULL,NULL)；仅在业务明确“无记录总额为零”后转换总额，平均值仍不可计算。

## 一对多避免重复

“至少有一条成功流水的CNY订单金额”：使用EXISTS避免两条流水让订单10001金额重复。

```sql
SELECT COUNT(*) AS order_count, SUM(o.amount_minor) AS total_minor
FROM orders o
WHERE o.currency = 'CNY' AND o.user_id = 7
  AND o.created_at >= '2026-01-01 00:00:00' AND o.created_at < '2026-01-02 00:00:00'
  AND EXISTS (SELECT 1 FROM payments p WHERE p.order_id = o.id AND p.status = 'SUCCEEDED');
```

期望2个订单、200分。直接JOIN可能得3行、300分；SUM(DISTINCT amount_minor)又错误地得100分，因为两笔订单金额相同。

需要同时展示流水数时先按order_id聚合，再JOIN；确认主表主键唯一，不能据知识库猜测一对一。

```sql
SELECT o.id, o.amount_minor, p.payment_count
FROM orders o
JOIN (SELECT order_id, COUNT(*) AS payment_count FROM payments
      WHERE status = 'SUCCEEDED' GROUP BY order_id) p ON p.order_id = o.id
WHERE o.currency = 'CNY' AND o.user_id = 7
  AND o.created_at >= '2026-01-01 00:00:00' AND o.created_at < '2026-01-02 00:00:00'
ORDER BY o.id LIMIT 100;
```

期望(10001,100,2)、(10002,100,1)。仅说明匹配事实，支付多次的业务含义须另查。

## 比例、时间及异常筛选

比例使用DECIMAL及NULLIF分母，分母为0展示不可计算；不能把失败率等同失败数/任意非同口径总数。下面只验证空集合零分母语义，不定义公司指标：

```sql
SELECT CAST(SUM(CASE WHEN status = 'PAID' THEN 1 ELSE 0 END) AS DECIMAL(20,6))
       / NULLIF(COUNT(*), 0) AS paid_ratio
FROM orders WHERE id = 99999;
```

期望NULL。真实MySQL需核验精度/舍入；不要平均各分组百分比代替总体加权比例。

按日统计需先确认业务日对应UTC边界，再使用已验证时区转换分组。夏令时日不固定24小时，DATETIME没有自动携带时区；TIMESTAMP受会话时区影响。转换为NULL应报错。对比保持相同粒度、过滤与时长；缺失日期不自动补零。

异常数据可按已明确规则筛选，例如“存在成功流水但订单仍PENDING”：

```sql
SELECT o.id, o.status FROM orders o
WHERE o.status = 'PENDING' AND o.user_id = 7
  AND o.created_at >= '2026-01-01 00:00:00' AND o.created_at < '2026-01-02 00:00:00'
  AND EXISTS (SELECT 1 FROM payments p WHERE p.order_id = o.id AND p.status = 'SUCCEEDED')
ORDER BY o.id LIMIT 100;
```

期望10002；只证明当前条件不一致，不能证明通知失败。HAVING用于已定义的分组阈值；范围不明确先澄清。任何分组结果达到上限都须声明可能截断，不能把部分分组合计当全量。
