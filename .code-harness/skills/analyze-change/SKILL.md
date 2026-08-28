---
name: analyze-change
description: 计算完整 Review Change Set，并按 FULL/TARGETED 意图通过受控 Code Navigation 与资源关系证据建立可机器验证的 Review Scope。
version: 6
agent: reviewer
tools: [git_diff, read_code, find_symbol, find_references, find_implementations, workspace_verify, workspace_inherited, workspace_superclass_call, workspace_template_dispatch, validate_contract]
output_schema: .code-harness/contracts/change-analysis.schema.json
---

# 分析代码变更

## 目标
所有 Review 都先计算同一个完整 Change Set：`merge-base(baseRef, HEAD) → HEAD` 的 committed，加上 staged、unstaged、untracked。

```text
sourceIncludes -> src/main/java/**/*.java
testIncludes   -> src/test/java/**/*.java
mapperIncludes -> src/main/resources/**/*Mapper.xml
configIncludes -> src/main/resources/**/*.yml
```

FULL 完整读取整个 required scope；TARGETED 只读取机器验证的 selectedCallChains + evidence resources + scopedFiles；LIST 只列调用链。

## 关键原则
1. Change Set 是唯一入口；TARGETED 不改变 Change Set 计算语义。
2. FULL 必须覆盖所有 source/test/mapper/config changed files。
3. `selectedCallChains` 必须来自 Certified ChangeAnalysis。
4. Agent 声明 COMPLETE 不是 authority；必须经过 Controlled Runtime。
5. Agent 只写 `requests/change-analysis-draft.json`，不得直接写 authoritative analysis artifacts。
6. dependency workspace 只允许做 Navigation / Chain Context，不得进入 changedFiles、Review Scope 或 Write Scope。

## A. 建立完整 Change Set
1. 读取 review baseRef/includeWorkingTree 与 scope includes。
2. baseRef 不存在时停止，不得猜 main/develop。
3. 使用 git_diff 获取 committed/staged/unstaged/untracked，同路径合并 sources。
4. changed file role 必须按路径/已验证语义填写；以下 path-role 是固定 contract：

```text
src/test/**                         -> Test
*Mapper.xml                         -> MapperXml
src/main/resources/**/*.yml        -> YamlConfig
```

`src/test/**` 不得由 Agent 自由选择其他 role；Runtime ReviewUnit authority 会再次 machine-enforce `src/test/** ↔ Test` 双向 invariant。MapperXml/YamlConfig 同样不得降级为 Other。
5. 生成完整 changedFiles[]；TARGETED 也不得丢 Change Set 元数据。

## B. confirmed callChains 与 Navigation Evidence
6. 从 Diff/源码识别 changed symbols。Controller 向下导航，Service/Repository 可反向找上游并继续向下到 Repository/Mapper/DAO 或外部边界。
7. 接口必须 find_implementations；不确定只记录 unresolved/limitation。
8. 每个确定性 Java Navigation 结果写入 symbolLocations：symbol、exact repository path、role、source；不得靠类名后缀猜 role。
9. confirmed chain 写 callChains；candidate 不得包装为 confirmed。
10. workspace fallback 只允许显式 harness.yaml.workspaceDependencies，先 workspace verify，只有 VERIFIED 才可调用 workspace-inherited/workspace-superclass-call/workspace-template-dispatch。
11. workspace != current 的路径绝不进入 changedFiles/resourceRelations/reviewCoverage/scopedFiles。

## C. Resource Relation Evidence
12. changed Mapper.xml 只有 statement id/namespace/Java Mapper method 能确定对应时才写 MAPPER_STATEMENT relation。
13. changed YML 只有 @Value/@ConfigurationProperties 等直接 consumer evidence 时才写 CONFIG_REFERENCE relation。
14. resourceRelations.path 必须是 Change Set exact repository-relative path，evidence 不得为空。

## D. FULL Review
15. 读取全部 source/test/mapper/config changed files；changed test 文件以 role=Test 写入 changedFiles/reviewCoverage。
16. current-project call-chain 文件完整读取；dependency workspace 仅作 context。
17. reviewCoverage.status=COMPLETE 仅当 required changed files 已读、current chain 已解析、unresolvedSymbols 为空。
18. draft 最终必须交给 Runtime `analysis certify` 重新计算 Change Set/Coverage；Agent 自报 COMPLETE 不放行。

## E. TARGETED Review
19. target 只从 confirmed ChangeAnalysis.callChains 解析；Controller/Service role 使用 verified symbolLocations.role。
20. Controller CLASS/METHOD 自动包含对应全部 confirmed branches；Service/下游 2+ 上游链才选择。
21. Java scopedFiles 使用 symbolLocations exact path；资源 scopedFiles 仅接受 changed resource + verified resourceRelation + selected chain 命中。
22. 所有 scopedFiles 必须完整读取并通过 reviewscope.Verify；不得 sampled review。

## F. LIST
23. LIST 只建立 Change Set + confirmed/candidate/unresolved chain 信息，不执行 Finding Review。
