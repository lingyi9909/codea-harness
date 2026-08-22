# Codea Harness

Codea Harness V1 是面向 Java + Spring Boot + Maven 项目的 Agent 原生 Harness 规范包。源码仓库只保存 Source；正式 Windows 产品由 CI 构建 Runtime、注入固定 ast-grep 后打包发布。

## 1.3.2 Review Report UX Fix

1. `review.md` 固定 UI 全中文，机器 Contract enum 继续保持英文。
2. Review Report transport 使用 `callChains[] {entryPoint, chain[]}`，支持 0/1/多条真实调用链，不再压平。
3. Finding 严重级别固定映射为 `🔴 严重 / 🟠 高 / 🟡 中 / 🟢 低`，并由 Runtime 按 severity → file → line → id 确定性排序。
4. 测试代码仍必须参与 Review Coverage，但默认不做普通代码质量 Review；只有测试失真才允许 `TEST_VALIDITY` Finding。
5. 1.3.2 不修改 Review Change Set、Coverage COMPLETE/PARTIAL Gate、Finding 判断原则、Approval、Test Target Selection、DB、Debug、Fix、Upgrade transaction 或 API Doc 主流程。

## 1.3.1 Release Packaging Fix

1. Windows Release 拆分为首次安装包和升级包。
2. 正式包包含 `RELEASE-MANIFEST.json`，记录版本、平台、架构、Runtime/ast-grep 版本与 SHA256。
3. 升级统一通过 `.code-harness-upgrade/upgrade.md` bootstrap；在调用任何 Runtime 前先验证正式升级包完整性。
4. 不修改既有 staged transaction、Project State 保留、rollback 或 Windows running-exe replacement 核心逻辑。

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
codea-harness-1.3.2-windows-x64-install.zip
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
codea-harness-1.3.2-windows-x64-upgrade.zip
```

解压后顶层直接得到 `.code-harness-upgrade/`。升级入口固定为：

```text
读取 .code-harness-upgrade/upgrade.md，执行升级
```

不要绕过 `upgrade.md` 直接执行 Runtime。

`upgrade.md` 会先做 Agent 层只读 Package Preflight，一次性检查正式升级包 required source。任一缺失：

```text
MANUAL_ACTION_REQUIRED
→ 一次列出全部缺失项
→ 缺 exe 时提示可能误用了 GitHub Source Code
→ 不调用任何 Runtime
→ 不创建 stage/backup
→ 0 文件修改
→ STOP
```

Package Preflight 通过后，才调用当前已安装的：

```text
.code-harness/bin/codea-harness-tools.exe upgrade
```

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
  "version": "1.3.2",
  "platform": "windows",
  "arch": "x64",
  "runtime": "codea-harness-tools.exe",
  "runtimeSha256": "...",
  "astGrepVersion": "0.42.1",
  "astGrepSha256": "..."
}
```

## Review Report

`harness review` 与 `harness test` 的 Review 阶段由 Controlled Runtime 确定性生成：

```text
.code-harness/runs/<runId>/review.md
```

1.4 Human Report UX 在 1.3.2 中文报告基础上统一首屏。报告打开后第一屏即可看到：

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

调用链仍只消费已经通过 Runtime 验证的 `ChangeAnalysis.callChains[]`，Renderer 不自行增加或修改 machine symbol。角色标签也不能根据类名后缀猜测，只能消费从已验证 `ChangeAnalysis.symbolLocations[] / resourceRelations[]` 原样传递的 role evidence；没有可靠证据时固定降级为 `🔹 代码节点`：

```text
🌐 接口入口   ← verified role=Controller
⚙️ 业务服务   ← verified role=Service
🧠 业务实现   ← verified role=Service + source=FIND_IMPLEMENTATIONS
🗄 数据访问   ← verified role=Repository/Mapper
📄 Mapper XML ← verified resource role=MapperXml
🔹 代码节点   ← 无可靠 role evidence / Other / 其他角色
```

因此，即使源码名称是 `XxxController`、`XxxService` 或 `XxxServiceImpl`，只要机器证据中的真实 role 不匹配，就不得按名称显示对应角色。

Finding 展示固定为一个完整问题块：

```text
### <severity emoji> <findingId>｜<中文级别>
📍 位置
❗ 问题
🔎 证据
💥 影响
🛠 修复建议
🧪 是否需要测试
```

报告末尾固定提供 `## ➡️ 下一步`：❌ 未通过时优先处理阻断 Finding 并可使用 `harness fix finding:<id>`；✅ 通过时明确无需处理阻断问题；⚠️ 需要人工处理时明确列出需要处理的未解析项、缺失评审文件或运行时契约校验错误。

TARGETED 报告始终保留：

```text
本结论只覆盖本次定向评审范围，不代表整个 Change Set 已完成评审。
```

机器 Contract 继续使用：

```text
PASSED | FAILED | MANUAL_ACTION_REQUIRED
CRITICAL | HIGH | MEDIUM | LOW
COMPLETE | PARTIAL
```

这些 enum 只用于机器 JSON/内部状态；最终用户摘要和 `review.md` 使用中文显示，其中 `TEST_VALIDITY` 用户侧统一显示为“测试有效性问题”。Runtime 继续按 severity → file → line → id 确定性排序，同一输入必须 byte-for-byte 生成相同 Markdown。

### Review Finding Scope

生产代码正常 Review：

```text
category = PRODUCTION_CODE
```

测试代码仍必须读取并参与 Review Coverage / Existing Test Coverage，但默认不得因命名、重复、结构、代码风格、可维护性或 Mock 写法不漂亮产生普通 Finding。

测试代码只有存在明确 false-positive 证据时才允许：

```text
category = TEST_VALIDITY
```

例如删除/禁用有效测试、删除或明显弱化关键断言、吞异常导致无条件通过、Mock 内部业务 Bean 绕过真实调用链、修改测试范围使生产变更没有被验证等。

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

Windows x64 Release Gate 由 `.github/workflows/package-windows-x64.yml` 执行，覆盖 Runtime、Review Report Golden、Navigation、Schema、Selection、Database Safety、Windows build、双 ZIP layout、Manifest、`1.3.1 → 1.3.2` live upgrade、Project State hashes、stale framework、running exe replacement、stage/backup/source cleanup 和 artifact upload。
