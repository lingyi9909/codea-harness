# Codea Harness 1.6.3 Runtime Usability Hotfix Design

## 1. 背景

1.6.2 Review Reliability 已完成并进入 `main`。真实项目使用中暴露四个独立但都影响可用性的缺口：

1. 大 Java 类 / 大源码树下 Workspace AST navigation 会在 30 秒预算内超时；
2. public `harness chain discover [target]` 仍绑定 current Change Set，clean branch / HEAD 与 base 相同会发现 0 条 Chain；
3. plain `harness review` 在 2+ Chain 时 Runtime 会产出 `USER_SELECTION`，但 Agent 当前可以在同一用户轮次直接提交 `FULL`，实际没有停下来询问用户；
4. upgrade 仍按完整 managed tree 替换，用户包携带 `tools-runtime/**` 源码，并可能无意义触碰未变化的 Runtime binary。

本 Hotfix 只修复以上四项，不重新设计 1.6.2 已验收的 Canonical ChangeSet / ChangeAnalysis Certification / Certified Findings / Report Authority。

正式开发基线：

`c81f19cc2fab6925bd278c75b594006ce585f581`

开发分支：

`hotfix/1.6.3-runtime-usability`

目标版本：`1.6.3`

---

## 2. 全局约束

- Windows-first；离线运行，不新增联网依赖。
- 继续使用 bundled `ast-grep.exe`，不引入新的 Java parser 依赖。
- Review 的 Canonical ChangeSet Authority 仍只属于 Runtime `analysis snapshot`。
- public Chain project discovery 不得伪装成 Review ChangeSet，也不得改变 Review lazy/affected discovery 的 ChangeSet 语义。
- `.code-harness/chains/**` 仍只能经过 Runtime candidate/certification/write-plan/persist 流程写入；discover 不自动保存。
- `USER_SELECTION` 不得默认 `FULL`，不得由 Agent 在同一用户轮次替用户决定。
- Upgrade 必须继续具备 backup / validate / rollback；不得为了差量更新弱化原子性。
- `harness.yaml`、`project.md`、`database.yaml`、`chains/**`、`runs/**` 继续视为 Project State 并保留。
- `tools-runtime/**` 是开发源码，不作为用户安装/升级所需 Runtime payload。

---

## 3. Task 1 — Workspace AST Navigation Performance

### 3.1 根因

当前 `runWorkspaceRaw()` 对每一个 AST pattern 都独立启动一次 ast-grep，并把 `src/main/java` 整棵树作为扫描目标；`WorkspaceMethod()` / `WorkspaceMethodCalls()` / template dispatch 又会重复 class/method/all-types/call 扫描。

文件行数不是 Authority threshold，`1000+` 只是当前真实项目中开始明显触发 30 秒超时的表现。

### 3.2 设计

保留 ast-grep 作为语法事实来源，但增加 deterministic candidate-file narrowing：

1. Runtime 先在 `src/main/java/**/*.java` 上做廉价文本候选定位，只用于缩小文件集合，不直接产生语义 Authority；
2. 最终 class / method / call / inheritance 判断仍由 ast-grep 结果决定；
3. 已知 owner class 后，method scan 只对 owner candidate file 执行；
4. 已知 `fromSymbol` 后，call scan 只对 `from.Path` 执行；
5. subclasses scan 只对含目标 superclass token 的 candidate Java files 执行；
6. nested type ownership 只需扫描 owner 所在文件，不再全树重扫；
7. 找不到可靠 candidate 时允许 fail-safe 回退 full source scan，不能为了性能制造 false negative。

本 Hotfix 不依赖 ast-grep 未验证的多规则 CLI 特性；优先通过 scope narrowing 降低 process × tree-scan 成本。

### 3.3 验收

- 1500–3000 行 Java 类 fixture 可以完成 WorkspaceMethod/Call navigation；
- 已定位 owner 后，Runner 调用参数不得再次把整个 `src/main/java` 作为 method/call scan target；
- nested class / inheritance 既有测试不退化；
- Full Go regression + vet PASS。

---

## 4. Task 2 — Public Project Chain Discovery

### 4.1 语义拆分

三类 Chain 行为明确分离：

- `harness chain discover [target]` = **PROJECT DISCOVERY**：面向当前源码，即使 Git ChangeSet 为空也应发现现有业务 Chain；
- `harness review` 内的 lazy discovery = **AFFECTED DISCOVERY**：继续绑定同 run Certified ChangeAnalysis / ChangeSet，只发现当前评审需要的 Chain；
- `harness chain refresh <id>` = **EXISTING CHAIN REFRESH**：继续比较当前 source facts 与已保存 Chain。

### 4.2 Authority

public project discovery 不能把“全项目源码”伪造成 `analysis/change-set.json`。新增独立 project-discovery Runtime path：

1. Runtime 枚举 production Controller entrypoints（可按 target 过滤）；
2. 基于现有 deterministic Code Navigation/AST 能力解析 Controller → downstream symbols / implementation / resource relation；
3. 生成 run-scoped project discovery evidence；
4. Runtime 生成 `DISCOVERED` candidate + provenance certificate；
5. 仍只写 `runs/<runId>/analysis/discovered-chains/**`，不写 `.code-harness/chains/**`。

Review 内现有 `chain discover`（Certified ChangeAnalysis consumer）继续保留，避免影响 1.6.2 Review Authority。

### 4.3 兼容目标

`harness chain discover NewController` 在有 ChangeSet 时仍可正常工作；在 clean branch 时也必须工作。

### 4.4 验收

新增真实 Git fixture：

- base/main 与 current HEAD 完全相同；
- working tree clean；
- 项目包含 Controller → Service → Impl → Mapper + Mapper XML；
- `harness chain discover <Controller>` 产生至少一条 Runtime-certified DISCOVERED Chain；
- 不产生 `review.md`；
- 不写 `.code-harness/chains/**`。

---

## 5. Task 3 — Multi-Chain Review User Selection Hard Stop

### 5.1 保留现有 Runtime 决策

继续：

- 0 Chain → `AUTO_FULL`
- 1 Chain → `AUTO_SINGLE`
- 2+ Chain → `USER_SELECTION`

不改变 `review-options.json` 的 Authority 计算。

### 5.2 新的 Agent/Invocation Contract

当 plain `harness review` 得到 `USER_SELECTION`：

1. Agent 必须展示 Chain options；
2. 必须在当前用户轮次结束响应；
3. 当前轮次禁止调用 `review select`、`review units`、`review dispatch`、Finding Certification、Report Review；
4. 下一条用户消息明确选择 `全部` / selection ids 后才允许继续；
5. 不把历史偏好、默认 FULL、Agent 推断当成用户选择。

由于通用 Agent Host 无法让 Runtime 密码学证明一段选择文本来自人类，真正的闭环由 active contract + real OpenCode same-session two-turn E2E 锁定；Runtime 仍负责对选择的合法性和 optionsHash/runId freshness 做强校验。

### 5.3 验收 E2E

同一真实 OpenCode session：

Turn 1：用户只输入 `harness review`

- Runtime options = USER_SELECTION；
- Agent 返回可读菜单；
- transcript / tool trace 中没有 `review select/units/dispatch`。

Turn 2：用户输入 `全部`

- 复用同一 fresh Review run 的 options；
- `review select FULL`；
- 继续 units → dispatch → Certified Findings → report。

---

## 6. Task 4 — Upgrade V2 Delta Apply

### 6.1 Payload 边界

用户安装/升级 payload：

- `bin/codea-dcep-tools.exe`
- `bin/ast-grep.exe`
- `agents/**`
- `skills/**`
- `contracts/**`
- `tools/**`
- framework docs/templates/VERSION

`tools-runtime/**` 不再属于用户 upgrade payload，也不再由 upgrade transaction 管理。

### 6.2 Delta semantics

保留现有完整 backup + stage + schema validation + rollback，但 apply 阶段改为 byte/hash-aware：

- staged file 与 installed file 完全相同时：SKIP，不改 mtime/bytes；
- 新增或内容变化：UPDATE；
- 旧 Framework Managed file 在新 payload 中消失：REMOVE；
- Runtime exe 只有 bytes 变化才进入 Windows rename/parking 替换逻辑；
- ast-grep 同理。

这样无需先引入复杂独立 updater，也能获得用户最关心的“只升级真正变化内容”。后续如需要可再演进 side-by-side Runtime，本 Hotfix 不做。

### 6.3 Manifest

本版本可增加 deterministic `UPGRADE-MANIFEST.json` 作为 package audit/evidence，但 transaction 正确性不能只相信 manifest；apply 前仍以实际 staged bytes/hash 对 installed bytes 做校验。

### 6.4 验收

- source package 不含 `tools-runtime/**` 仍可 upgrade；
- Runtime binary bytes 相同时不触碰 installed runtime；
- Runtime binary bytes 不同时正常替换；
- framework-only upgrade 只更新变化的 managed files；
- Project State 保留；
- schema migration / rollback regression 继续 PASS；
- Windows upgrade tests PASS。

---

## 7. Final Certification

1. Task 1 focused performance/regression tests；
2. Task 2 clean-project discovery tests；
3. Task 3 active contract + real OpenCode two-turn E2E；
4. Task 4 delta upgrade tests；
5. `go test -count=1 ./...`；
6. `go vet ./...`；
7. Windows exact-head Runtime build；
8. retained 1.6.2 real Review lifecycle E2Es；
9. exact `${{ github.sha }}` assertion。

只有 fresh exact-head Gate 全绿后，1.6.3 才进入外部验收。