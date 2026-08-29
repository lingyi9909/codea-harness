# Task 6 Test role mapping addendum

`analyze-change` 在生成 ChangeAnalysis draft 时，对测试范围使用固定 path-role 映射：

```text
src/test/java/**/*.java -> Test
```

该映射不是 Agent 自由判断结果。`src/test/**` 必须声明为 `Test`；非 `src/test/**` 不得声明为 `Test`。Controlled Runtime 会在 RuleDispatch 前再次 machine-enforce `src/test/** <-> Test` invariant，Agent 自报 role 不能创建或抑制 `TEST-VALIDITY-001` authority。
