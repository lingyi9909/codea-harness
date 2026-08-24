# Codea Harness

Codea Harness V1 是面向 Java + Spring Boot + Maven 项目的 Agent 原生 Harness 规范包。源码仓库只保存 Source；正式 Windows 产品由 CI 构建 Runtime、注入固定 ast-grep 后打包发布。

## 1.5.0

1. **Business Chain Project State**：支持一条业务链一个 YAML，提供 `harness chain list/show/discover/refresh/validate`；EntryPoint 只允许生产 Controller Method，Java path/role/resource relation 必须来自机器验证证据。
2. **Lazy Discovery + Exact Canonicalization**：只在当前 Change Set 或显式 target 范围发现 Chain；V1/V2 仅在 verified core facts 完全一致时合并，不使用 fuzzy/name similarity。
3. **Review Consumes Verified Chains**：FULL/TARGETED Review 优先复用当前代码事实仍然 VALID 的 ACCEPTED Chain；缺失时使用当前 Run 的 `DISCOVERED + TEMPORARY` Chain；STALE 必须由用户明确选择临时使用、刷新或停止。
4. **Review provenance**：`review.md` 可展示业务链、Chain ID、来源和状态；临时 Chain 会提示尚未沉淀。Chain 不能改变 1.4 的 Change Set、ReviewScopeSelection、Coverage 或 Finding Gate。
5. **1.4.0 → 1.5.0 Windows Upgrade Gate**：`chains/**` 与其他 Project State 一样受保护；1.4→1.5 没有新的配置 migration，因此 `harness.yaml`、`project.md`、`database.yaml`、`runs/**`、`chains/**` 均保持原内容，其中业务 Chain 必须 byte-for-byte 保持。

**1.5.0 的 Chain 仅接入 Review；不支持 Test/Debug/Fix Chain。** Test、Debug、Fix、Verify 继续沿用既有 1.4 语义，不把 Chain 自动作为它们的新 Scope 真相源。

1.5.0 继续只发布 Windows x64；不包含 Maven Doctor、Linux/macOS、Gradle、JDT LS、JaCoCo、PIT 或 SARIF。

## 1.4.0

1. **Targeted Review**：支持 `harness review <Class>`、`harness review <Class.method>` 和 `harness review list`。完整 Change Set 与本次定向 Scope 分离，Scope/Coverage 必须经过 Runtime 机器验证；定向结论不能冒充整个 Change Set 已完整评审。
2. **Mapper.xml / YML Review**：`*Mapper.xml` 与 `src/main/resources/**/*.yml` 纳入 FULL Review Coverage；TARGETED 只纳入与目标调用链有明确 evidence relation 的资源文件。
3. **Human Report UX Standard**：`review.md` 使用统一中文首屏、机器 role evidence 驱动的调用链标签、标准 Finding 块和明确下一步。Renderer 不根据类名后缀猜角色。
4. **Runtime Apply Safety**：Fix/Test 正式写入必须先 seal exact plan，再经 Runtime 校验 approved diff、base hash、声明文件集和路径策略，原子 apply/rollback 并生成 apply evidence；`.git/**` 与 `.code-harness/**` 为不可配置 hard-deny。
5. **1.3.2 → 1.4.0 升级兼容**：旧 `harness.yaml version=1` 由目标版本 Runtime 的 registered migration 升到 v2，并补充 Mapper/YML scope；用户已有配置保持不变，除登记 migration 修改外不做全量重写。

## 不要使用 GitHub Source Code 安装/升级

> ⚠ 不支持使用 GitHub `Code → Download ZIP` 或 `git clone` 得到的源码目录进行安装或升级。

源码仓库不会提交：

```text
.code-harness/bin/codea-harness-tools.exe
.code-harness/bin/ast-grep.exe
```

这两个二进制只由 `package-windows-x64` Release CI 构建/注入。边界固定：

```text
GitHub repository = Source
CI = build Runtime + inject pinned ast-grep + verify
Release ZIP = 可安装/可升级产品
```

## 首次安装

使用：

```text
codea-harness-1.5.0-windows-x64-install.zip
```

解压后顶层直接得到：

```text
.code-harness/
├── VERSION
├── RELEASE-MANIFEST.json
├── bin/codea-harness-tools.exe
├── bin/ast-grep.exe
├── contracts/chain.schema.json
├── contracts/chain-validation-result.schema.json
├── templates/chain.template.yaml
└── ...
```

把 `.code-harness/` 放到项目根目录，然后对工程 Agent 说：

```text
读取 .code-harness/bootstrap.md，执行 harness init
```

初始化或后续使用可能生成本机 Project State：

```text
.code-harness/harness.yaml
.code-harness/project.md
.code-harness/database.yaml
.code-harness/runs/**
.code-harness/chains/**
```

正式 install ZIP 不包含这些 Project State 实例；特别是不会预置任何业务 `chains/*.yaml`。

## 版本升级

使用：

```text
codea-harness-1.5.0-windows-x64-upgrade.zip
```

解压后顶层直接得到 `.code-harness-upgrade/`。升级入口固定为：

```text
读取 .code-harness-upgrade/upgrade.md，执行升级
```

不要绕过 `upgrade.md` 直接执行 Runtime。

`upgrade.md` 首先做 Agent 层只读 Package Preflight，一次性检查正式升级包 required source。任一缺失：

```text
MANUAL_ACTION_REQUIRED
→ 一次列出全部缺失项
→ 缺 exe 时提示可能误用了 GitHub Source Code
→ 不调用任何 Runtime
→ 不创建 stage/backup
→ 0 文件修改
→ STOP
```

Package Preflight 通过后，调用**目标版本升级包 Runtime**：

```text
.code-harness-upgrade/bin/codea-harness-tools.exe upgrade
```

registered migration 属于目标版本 Runtime，因此不能依赖旧安装 Runtime 预知未来版本 migration。升级事务仍由 Controlled Runtime 完成，并继续保持 staged replace、rollback、Windows executable replacement 和 Project State 保护。

### 1.4.0 → 1.5.0

1.5.0 不新增 `harness.yaml` migration。升级只 replace Framework Managed 内容并安装 Chain Framework；Project State 不参与 managed replace：

```text
harness.yaml          # byte-for-byte 保持
project.md            # byte-for-byte 保持
database.yaml         # byte-for-byte 保持
runs/**               # byte-for-byte 保持
chains/**             # byte-for-byte 保持
```

`chains/**` 永远是 Project State。即使正式包或异常 source tree 中出现同名业务 Chain，Runtime 也不得用它覆盖用户已有 Chain；`removedFiles` 也不得包含 `chains/**`。

### 1.3.2 → 1.4.0 历史 migration

`harness.yaml` 的 registered migration 只做：

```yaml
version: 2
scope:
  # 原 sourceIncludes / testIncludes 保留
  mapperIncludes:
    - src/main/resources/**/*Mapper.xml
  configIncludes:
    - src/main/resources/**/*.yml
```

成功后 `.code-harness-upgrade/`、stage 和 backup 都必须清理。

## RELEASE-MANIFEST.json

正式 install/upgrade 包由 CI 动态生成：

```json
{
  "version": "1.5.0",
  "platform": "windows",
  "arch": "x64",
  "runtime": "codea-harness-tools.exe",
  "runtimeSha256": "...",
  "astGrepVersion": "0.42.1",
  "astGrepSha256": "..."
}
```

## Chain Management

用户意图固定为：

```text
harness chain list
harness chain show <id|target>
harness chain discover [target]
harness chain refresh <id>
harness chain validate [id]
```

不增加 `chain accept/merge/split/edit/ignore` 用户命令。`discover` 只写当前 Run 的 `analysis/discovered-chains/**`；保存/refresh Project State 必须经过显式用户确认、Runtime validate 和安全持久化。

开发者可以直接编辑：

```text
.code-harness/chains/*.yaml
```

编辑后应执行 `harness chain validate <id>`。Runtime 会重新验证 entryPoint/node/resource/boundary/id/path/call relation，不能因为 YAML 中写了事实就直接相信。

## Review

默认：

```text
harness review
```

始终执行 FULL Review。定向 Review：

```text
harness review list
harness review OrderController
harness review OrderController.approve
harness review OrderService.approve
```

Review Chain flow 固定为：

```text
verified ChangeAnalysis / Review Scope
→ Accepted Chain lookup + validate
→ valid accepted chain: reuse
→ missing: lazy DISCOVERED temporary chain
→ stale: user decision gate
→ review.md provenance
```

Chain 是业务上下文，不是新的 Review Scope 真相源：FULL 仍覆盖完整 required Change Set；TARGETED 仍使用 Runtime verified `selectedCallChains/scopedFiles`；scope-out Finding 仍被拒绝。

TARGETED 报告始终保留：

```text
本结论只覆盖本次定向评审范围，不代表整个 Change Set 已完成评审。
```

### Mapper.xml / YML Review

默认 scope：

```yaml
scope:
  sourceIncludes:
    - src/main/java/**/*.java
  testIncludes:
    - src/test/java/**/*.java
  mapperIncludes:
    - src/main/resources/**/*Mapper.xml
  configIncludes:
    - src/main/resources/**/*.yml
```

FULL Review 中 changed Mapper/YML 不能静默跳过；未读取会使 Coverage 进入 PARTIAL。TARGETED 只有存在经过验证的 resource relation 时才把资源文件加入本次 Scope。

## Review Report

`harness review` 与 `harness test` 的 Review 阶段由 Controlled Runtime 确定性生成：

```text
.code-harness/runs/<runId>/review.md
```

存在明确 Chain context 时首屏额外显示：

```text
业务链
Chain ID
Chain 来源：项目已确认 / 本次临时发现
Chain 状态
```

临时 Chain 会显示尚未沉淀提示。调用链和角色仍只能消费机器验证过的 evidence，不根据类名后缀猜角色。

## Runtime Apply Safety

任何会实际写入生产代码或测试代码的批准计划，正式路径固定为：

```text
生成 exact ApplyRequest
→ Runtime seal-apply（审批前 immutable baseline）
→ 用户精确批准 <planId>/<fixPlanId>
→ 同一 apply.json
→ Runtime apply
→ evidence/apply/<planId>.json
```

1.5 没有把 Chain 接入上述 Test/Fix 写入或 Debug/Verify 流程；**不支持 Test/Debug/Fix Chain**。

## Frontend API Documentation

支持：

```text
harness api-doc OrderController
harness api-doc OrderController.approve
harness api-doc changed
```

唯一正式 Artifact：

```text
.code-harness/runs/<runId>/api-doc.md
```

## Lightweight Code Navigation

受控 Contract：

```text
find_symbol
find_references
find_implementations
get_symbol_info
find_by_annotation
find_callers
```

Agent 不得传 raw ast-grep rule/pattern/regex/arbitrary query。

## Test / DB / Failure Navigation 既有语义

- `harness test` 继续执行 ChangeAnalysis + Runtime Coverage Gate、Test Target Selection 与既有 Existing Test 语义；1.5 Chain 不自动改变测试 target。
- Existing Test：`REUSE_EXISTING / EXTEND_EXISTING / CREATE_NEW`，历史 Existing Test 永不自动修改。
- Database Evidence 仅支持 TEST/LOCAL MySQL，经过 schema allowlist、AST SQL Safety、预算、超时、行数上限和脱敏 Evidence Gate。
- Failure Navigation evidence-first；内部根因必须读取实际源码，外部依赖不猜服务端实现。

## 审批与安全

- 测试代码修改前必须精确 `批准 <planId>`。
- 生产代码修改前必须精确 `批准 <fixPlanId>`。
- 正式写入还必须通过 Runtime sealed approval + Apply Safety Gate。
- `harness api-doc` 全程只读。
- 禁止任意 Shell 求值、管道/重定向/用户命令拼接。
- Agent 不直接调用 ast-grep。
- 不访问生产数据库，不自动安装依赖，不自动 fetch/pull。

## 开发与验证

```text
cd .code-harness/tools-runtime
go test -count=1 ./internal/chain ./internal/reviewscope ./internal/coverage ./internal/report
go test -count=1 ./internal/apply ./internal/schema ./internal/upgrade
go test -count=1 ./...
go vet ./...
```

Windows x64 Release Gate 由 `.github/workflows/package-windows-x64.yml` 执行，覆盖 1.5 Chain/Review/Apply/Upgrade suites、全量 Go test/vet、真实 ast-grep Navigation smoke、正式 install/upgrade ZIP layout、Manifest、**真实 accepted 1.4.0 baseline → 1.5.0** live upgrade、Project State 与 `chains/**` byte-for-byte preservation、stale framework removal、Runtime replacement、installed `chain validate` capability probe、source/stage/backup cleanup 和 artifact upload。
