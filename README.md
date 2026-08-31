# Codea Harness

Codea Harness V1 是面向 Java + Spring Boot + Maven 项目的 Agent 原生 Harness 规范包。源码仓库只保存 Source；正式 Windows 产品由 CI 构建 Runtime、注入固定 ast-grep 后打包发布。

## 1.5.0

1. **Chain Management**：新增一链一 YAML 的业务 Chain Project State，以及 `harness chain list/show/discover/refresh/validate`、lazy discovery、exact canonicalization、STALE 检测和用户确认后的安全持久化。
2. **Review Consumes Verified Chains**：FULL/TARGETED Review 优先复用经当前代码事实重新验证的 ACCEPTED Chain；缺失时 lazy discover 当前 Run 的临时 Chain；STALE Chain 必须由用户明确选择临时使用、刷新或停止。
3. **Review provenance**：`review.md` 增加业务链、Chain ID、来源与状态；临时 Chain 明确提示尚未沉淀。Chain 不改变 1.4 的 Change Set、ReviewScopeSelection、Coverage 或 Finding Gate。
4. **1.4.0 → 1.5.0 Windows Upgrade Gate**：安装完整 Chain Framework；`harness.yaml`、`project.md`、`database.yaml`、`runs/**`、`chains/**` 保持原内容，其中 `chains/**` 必须 byte-for-byte 保持。
5. **Release boundary**：**Chain 仅接入 Review；不支持 Test/Debug/Fix Chain。** Test、Debug、Fix、Verify 继续保持既有 1.4 语义。

1.5.0 继续只发布 Windows x64；不包含 Maven Doctor、Linux/macOS、Gradle、JDT LS、JaCoCo、PIT 或 SARIF。

## 1.4.0

1. **Targeted Review**：支持 `harness review <Class>`、`harness review <Class.method>` 和 `harness review list`。完整 Change Set 与本次定向 Scope 分离，Scope/Coverage 必须经过 Runtime 机器验证；定向结论不能冒充整个 Change Set 已完整评审。
2. **Mapper.xml / YML Review**：`*Mapper.xml` 与 `src/main/resources/**/*.yml` 纳入 FULL Review Coverage；TARGETED 只纳入与目标调用链有明确 evidence relation 的资源文件。
3. **Human Report UX Standard**：`review.md` 使用统一中文首屏、机器 role evidence 驱动的调用链标签、标准 Finding 块和明确下一步。Renderer 不根据类名后缀猜角色。
4. **Runtime Apply Safety**：Fix/Test 正式写入必须先 seal exact plan，再经 Runtime 校验 approved diff、base hash、声明文件集和路径策略，原子 apply/rollback 并生成 apply evidence；`.git/**` 与 `.code-harness/**` 为不可配置 hard-deny。
5. **1.3.2 → 1.4.0 升级兼容**：旧 `harness.yaml version=1` 由目标版本 Runtime 的 registered migration 升到 v2，并补充 Mapper/YML scope；用户已有配置保持不变，除登记 migration 修改外不做全量重写。

1.4.0 继续只发布 Windows x64；不包含 Maven Doctor、Linux/macOS、Gradle、JDT LS、JaCoCo、PIT 或 SARIF。

## 1.3.2 Review Report UX Fix

1. `review.md` 固定 UI 全中文，机器 Contract enum 继续保持英文。
2. Review Report transport 使用 `callChains[] {entryPoint, chain[]}`，支持 0/1/多条真实调用链，不再压平。
3. Finding 严重级别固定映射为 `🔴 严重 / 🟠 高 / 🟡 中 / 🟢 低`，并由 Runtime 按 severity → file → line → id 确定性排序。
4. 测试代码仍必须参与 Review Coverage，但默认不做普通代码质量 Review；只有测试失真才允许 `TEST_VALIDITY` Finding。

## 1.3.1 Release Packaging Fix

1. Windows Release 拆分为首次安装包和升级包。
2. 正式包包含 `RELEASE-MANIFEST.json`，记录版本、平台、架构、Runtime/ast-grep 版本与 SHA256。
3. 升级统一通过 `.code-harness-upgrade/upgrade.md` bootstrap；在调用 Runtime 前先验证正式升级包完整性。

## 不要使用 GitHub Source Code 安装/升级

> ⚠ 不支持使用 GitHub `Code → Download ZIP` 或 `git clone` 得到的源码目录进行安装或升级。

源码仓库不会提交：

```text
.code-harness/bin/codea-dcep-tools.exe
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
├── bin/codea-dcep-tools.exe
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

初始化或后续使用生成的本机 Project State 包括：

```text
.code-harness/harness.yaml
.code-harness/project.md
.code-harness/database.yaml
.code-harness/runs/**
.code-harness/chains/**
```

正式 install/upgrade ZIP 不包含任何上述 Project State 实例，不会预置业务 `chains/*.yaml`。

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
.code-harness-upgrade/bin/codea-dcep-tools.exe upgrade
```

registered migration 属于目标版本 Runtime，因此不能依赖旧安装 Runtime 预知未来版本 migration。升级事务仍由 Controlled Runtime 完成，并继续保持 staged replace、rollback、Windows executable replacement 和 Project State 保护。

### 1.4.0 → 1.5.0

1.5.0 不新增 `harness.yaml` migration。升级只替换 Framework Managed 内容并安装 Chain Framework，因此以下 Project State 在升级前后保持 **byte-for-byte**：

```text
harness.yaml
project.md
database.yaml
runs/**
chains/**
```

`chains/**` 永远是 Project State，不属于 Framework Managed；即使异常升级 source 中出现业务 Chain，也不得覆盖项目已有 Chain，`removedFiles` 也不得出现 `chains/**`。

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

Project State 持续保护：

```text
harness.yaml          # 仅允许 registered migration 改动
project.md            # 保持原内容
database.yaml         # byte-for-byte 保持
runs/**               # 保持原内容
```

成功后 `.code-harness-upgrade/`、stage 和 backup 都必须清理。

## RELEASE-MANIFEST.json

正式 install/upgrade 包由 CI 动态生成：

```json
{
  "version": "1.5.0",
  "platform": "windows",
  "arch": "x64",
  "runtime": "codea-dcep-tools.exe",
  "runtimeSha256": "...",
  "astGrepVersion": "0.42.1",
  "astGrepSha256": "..."
}
```

## Chain Management

1.5 用户意图固定为：

```text
harness chain list
harness chain show <id|target>
harness chain discover [target]
harness chain refresh <id>
harness chain validate [id]
```

不新增 `chain accept/merge/split/edit/ignore` 用户命令。开发者可直接编辑 `.code-harness/chains/*.yaml`，修改后通过 `harness chain validate <id>` 重新验证代码事实。

## Review

默认：

```text
harness review
```

始终执行 FULL Review，保持 1.3.2 既有语义。

1.4 新增：

```text
harness review list
harness review OrderController
harness review OrderController.approve
harness review OrderService.approve
```

`review list` 只列本次 Change Set 已确认调用链，不生成 Finding。`Class` / `Class.method` 进入 TARGETED Review；Service/下游 target 若关联 2+ 条业务链，必须由用户选择，禁止默认 ALL。

1.5 在上述机器 Scope Gate 之后增加 Review Chain Context：有效 ACCEPTED Chain 优先复用；缺失时当前 Run lazy discover；STALE 必须用户决策。Chain 只补充业务上下文，不改变 FULL/TARGETED Coverage 或 Finding Scope。

TARGETED 报告始终保留：

```text
本结论只覆盖本次定向评审范围，不代表整个 Change Set 已完成评审。
```

### Mapper.xml / YML Review

默认 1.4 scope：

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

Mapper.xml 只关注本次变化引入的高价值风险，例如 UPDATE/DELETE WHERE 弱化、tenant/org/user 隔离条件弱化、statement/method/parameter/result 映射不一致和无边界批量写；不得因 XML 风格、缩进或命名产生 Finding。

YML 只关注 changed key 对 datasource/pool、timeout/thread/queue、Redis/MQ/RPC、日志级别、profile/feature switch、敏感信息和 `@Value/@ConfigurationProperties` 映射的影响；不得泛化审查未变化配置。

## Review Report

`harness review` 与 `harness test` 的 Review 阶段由 Controlled Runtime 确定性生成：

```text
.code-harness/runs/<runId>/review.md
```

1.4 Human Report UX 统一首屏，打开报告即可看到：

```text
评审结果
评审模式
评审目标（TARGETED）
Change Set 文件数
本次 Scope 文件数
已评审文件数
问题数量
下一步
```

最终报告固定展示：

```text
# 🔍 代码评审报告
统一首屏摘要
问题概览
生产/测试代码评审范围
真实多条代码调用链
评审覆盖
按严重级别排序的问题清单
中文评审结论
下一步
```

调用链只消费已经通过 Runtime 验证的 `ChangeAnalysis.callChains[]`。角色标签只能消费已验证 `ChangeAnalysis.symbolLocations[] / resourceRelations[]` 原样传递的 role evidence；不能根据 `XxxController/XxxService/XxxServiceImpl` 等名称后缀猜测。没有可靠证据时固定降级为 `🔹 代码节点`：

```text
🌐 接口入口   ← verified role=Controller
⚙️ 业务服务   ← verified role=Service
🧠 业务实现   ← verified role=Service + source=FIND_IMPLEMENTATIONS
🗄 数据访问   ← verified role=Repository/Mapper
📄 Mapper XML ← verified resource role=MapperXml
🔹 代码节点   ← 无可靠 role evidence / Other / 其他角色
```

1.5 当存在一个明确 Chain context 时，首屏额外展示业务链、Chain ID、来源和状态；临时 Chain 明确提示尚未沉淀。

Finding 展示固定为：

```text
### <severity emoji> <findingId>｜<中文级别>
📍 位置
❗ 问题
🔎 证据
💥 影响
🛠 修复建议
🧪 是否需要测试
```

报告末尾固定提供 `## ➡️ 下一步`。机器 enum 仍只用于 JSON/内部状态；用户侧显示中文，其中 `TEST_VALIDITY` 显示为“测试有效性问题”。

### Review Finding Scope

生产代码正常 Review：

```text
category = PRODUCTION_CODE
```

测试代码仍必须读取并参与 Review Coverage / Existing Test Coverage，但普通命名、结构、风格、重复、可维护性问题不得产生 Finding。只有有明确 false-positive 证据时才允许：

```text
category = TEST_VALIDITY
```

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

Runtime 会独立验证：

```text
planId / planType
unifiedDiff exact bytes / diffSha256
files[].path / files[].baseSha256
实际 touched file set
TEST/FIX allowlist
deniedPaths
.git/** 与 .code-harness/** hard-deny
path traversal / binary / unsafe patch
多文件原子 apply 与 rollback
```

批准 Patch A 后把 request 改成自洽 Patch B 仍会因为 sealed approval identity 不一致而拒绝。direct host write、`write_test`、`apply_approved_patch` 等不能作为正式完成路径。

1.5 **不支持 Test/Debug/Fix Chain**；Chain 不改变上述 Test/Fix 写入或 Debug/Verify 语义。

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

分析深度固定：Controller → Request DTO → Response DTO/VO → Enum → Validation → Direct Service Method（最多一层）→ STOP。

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

- `harness test` 先做 ChangeAnalysis + Runtime Coverage Gate，再执行 Test Target Selection；多 Controller 不默认 ALL。
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

Windows x64 Release Gate 由 `.github/workflows/package-windows-x64.yml` 执行，覆盖 1.5 Chain/Review/Apply/Upgrade suites、全量 Go test/vet、真实 ast-grep Navigation smoke、正式 install/upgrade ZIP layout、Manifest、**真实 accepted 1.4.0 baseline → 1.5.0** live upgrade、`harness.yaml/project.md/database.yaml/runs/**/chains/**` preservation、stale framework removal、Runtime replacement、installed `chain validate` capability probe、source/stage/backup cleanup 和 artifact upload。
