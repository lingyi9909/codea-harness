# Codea Harness

Codea Harness V1 是面向 Java + Spring Boot + Maven 项目的 Agent 原生 Harness 规范包。源码仓库只保存 Source；正式 Windows 产品由 CI 构建 Runtime、注入固定 ast-grep 后打包发布。

## 1.3.1 Release Packaging Fix

1. Windows Release 拆分为首次安装包和升级包。
2. 正式包新增 `RELEASE-MANIFEST.json`，记录版本、平台、架构、Runtime/ast-grep 版本与 SHA256。
3. 升级统一通过 `.code-harness-upgrade/upgrade.md` bootstrap；在调用任何 Runtime 前先验证正式升级包完整性。
4. 1.3.1 不修改既有 staged transaction、Project State 保留、rollback 或 Windows running-exe replacement 核心逻辑。

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
codea-harness-1.3.1-windows-x64-install.zip
```

解压后顶层直接得到：

```text
.code-harness/
├── VERSION
├── RELEASE-MANIFEST.json
├── bin/codea-harness-tools.exe
├── bin/ast-grep.exe
└── ...
```

把 `.code-harness/` 放到项目根目录，然后对工程 Agent 说：

```text
读取 .code-harness/bootstrap.md，执行 harness init
```

初始化生成本机 Project State：

```text
.code-harness/harness.yaml
.code-harness/project.md
```

## 版本升级

使用：

```text
codea-harness-1.3.1-windows-x64-upgrade.zip
```

解压后顶层直接得到：

```text
.code-harness-upgrade/
├── VERSION
├── RELEASE-MANIFEST.json
├── bin/codea-harness-tools.exe
├── bin/ast-grep.exe
└── ...
```

把 `.code-harness-upgrade/` 放到项目根目录。**升级入口固定为：**

```text
读取 .code-harness-upgrade/upgrade.md，执行升级
```

不要绕过 `upgrade.md` 直接执行 Runtime。

`upgrade.md` 会先做 Agent 层只读 Package Preflight，一次性检查正式升级包 required source，包括：

```text
VERSION
RELEASE-MANIFEST.json
AGENTS.md
bootstrap.md
upgrade.md
harness.template.yaml
project.template.md
agents/
skills/
contracts/
tools/
contracts/harness-config.schema.json
bin/codea-harness-tools.exe
bin/ast-grep.exe
```

任一缺失：

```text
MANUAL_ACTION_REQUIRED
→ 一次列出全部缺失项
→ 缺 exe 时提示可能误用了 GitHub Source Code
→ 不调用任何 Runtime
→ 不创建 stage/backup
→ 0 文件修改
→ STOP
```

Package Preflight 通过后，才调用**当前已安装**的：

```text
.code-harness/bin/codea-harness-tools.exe upgrade
```

这样继续保留既有 Windows running-exe staged replacement，以及成功后删除 `.code-harness-upgrade/` 的事务语义。

Project State 持续保护：

```text
harness.yaml
project.md
database.yaml
runs/**
```

其中 `database.yaml` 必须 byte-for-byte 保持不变。

## RELEASE-MANIFEST.json

正式 install/upgrade 包由 CI 动态生成：

```json
{
  "version": "1.3.1",
  "platform": "windows",
  "arch": "x64",
  "runtime": "codea-harness-tools.exe",
  "runtimeSha256": "...",
  "astGrepVersion": "0.42.1",
  "astGrepSha256": "..."
}
```

## 1.3.0 功能

### Review Report Persistence

`harness review` 与 `harness test` 的 Review 阶段由 Controlled Runtime 确定性生成：

```text
.code-harness/runs/<runId>/review.md
```

PASSED / FAILED / MANUAL_ACTION_REQUIRED 都有正式 Artifact；模型不得自由写最终 Markdown。

### Frontend API Documentation

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

分析深度固定：

```text
Controller
→ Request DTO
→ Response DTO/VO
→ Enum
→ Validation
→ Direct Service Method（最多一层）
→ STOP
```

Request location：`BODY / QUERY / PATH / HEADER`。不继续 Repository / Mapper / DAO / DB / MQ / Redis / RPC Server。

### Lightweight Code Navigation

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

## 其他主要意图

```text
harness review
harness test
harness api-doc <target>
harness debug-service
harness fix finding:<id>
harness fix diagnosis:<runId>
harness verify test:<class>
harness verify fix:<fixPlanId>
harness verify service:<runId>
```

版本升级不直接从这里调用 Runtime；始终读取 `.code-harness-upgrade/upgrade.md` bootstrap。

## Test / DB / Failure Navigation 既有语义

- `harness test` 先做 ChangeAnalysis + Runtime Coverage Gate，再执行 Test Target Selection；多 Controller 不默认 ALL。
- Existing Test：`REUSE_EXISTING / EXTEND_EXISTING / CREATE_NEW`，历史 Existing Test 永不自动修改。
- Database Evidence 仅支持 TEST/LOCAL MySQL，经过 schema allowlist、AST SQL Safety、预算、超时、行数上限和脱敏 Evidence Gate。
- Failure Navigation evidence-first；内部根因必须读取实际源码，外部依赖不猜服务端实现。

## 审批与安全

- 测试代码修改前必须精确 `批准 <planId>`。
- 生产代码修改前必须精确 `批准 <fixPlanId>`。
- `harness api-doc` 全程只读。
- 禁止任意 Shell 求值、管道/重定向/用户命令拼接。
- Agent 不直接调用 ast-grep。
- 不访问生产数据库，不自动安装依赖，不自动 fetch/pull。

## 开发与验证

```text
cd .code-harness/tools-runtime
go test -count=1 ./...
go vet ./...
```

Windows x64 Release Gate 由 `.github/workflows/package-windows-x64.yml` 执行，覆盖 Runtime、Navigation、Review/API Doc renderer、Selection、Database Safety、双 ZIP layout、Manifest、wrong-source bootstrap contract、`1.2.0/1.3.0 → 1.3.1` live upgrade、Project State hashes、stale framework、running exe replacement、stage/backup/source cleanup 和 artifact upload。
