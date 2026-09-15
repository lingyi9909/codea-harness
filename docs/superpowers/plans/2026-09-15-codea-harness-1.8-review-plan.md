# Codea Harness 1.8 Review Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. 本计划不授权派生研发 Agent；按用户当前约束执行，完成一项后提供独立验收结果。人工研发可直接执行，不要求安装这些技能。

**Goal:** 交付找链、人工选链、主 Agent 评审、一次提交即生成汇报版报告的离线 Windows Review。

**Architecture:** 在现有 Go Runtime 内新增局部 reviewrun 流程，复用导航、请求读取、真实用户选择判断与升级机制。OpenCode 正式命令先创建未完成报告；单一结构化工具组织准备/选择/完成。普通 Review 不再依赖旧认证阶段或 Reviewer 身份。

**Tech Stack:** Go 标准库与既有模块、ast-grep 0.42.1、OpenCode TypeScript 自定义工具、固定 Markdown 模板、现有 Windows PowerShell 打包方式。

**Spec:** [1.8 正式设计](../specs/2026-09-15-codea-harness-1.8-review-design.md)。报告版式：[样稿](../../examples/2026-09-15-codea-harness-1.8-review-report-example.md)。必须从同一文档提交读取。

**Status:** 仅设计与实施计划已交付，以下任务尚未实施。1.7 已暂停；产品位置核对点为 `970573a6d2147733b773c0de0c124d9324914ae8`，不是实际私有模型使用验收证据。

**研发分派入口：** [独立任务书、依赖与统一验收要求](../../tasks/codea-harness-1.8/README.md)。用户已要求拆分交给研发；T1 可分派，T2–T5 等待各自前置验收。本次只整理文档，未执行产品开发。

## Global Constraints

- 正常使用只有四步：开始并建报告 → 展示/选择调用链 → 主 Agent 评审 → 提交并落盘。
- Windows 10/11 x64、OpenCode、完全离线运行。不新增 npm/bun 在线安装、HTTP 模型客户端、容器、WSL 或后台服务。
- 1.7 原 integrated-design / integrated-plan 暂停执行，不进入 T6 或后续任务，不自动合入 1.8。
- 2 条及以上必须等待下一条真实用户回复；同一 Controller 多方法同样适用，不默认 ALL。
- 新 Review 不调用旧 analysis/finding 身份认证、ReviewUnit、RuleDispatch 或八阶段 progress；保留旧 run/非本轮功能兼容，不伪造旧证书。
- Review 永不自动修改生产/测试代码；升级、项目状态、Test/Fix/Apply/DB 既有保护保留。
- 请求读取复用 requestjson；工具输出 UTF-8 无 BOM。schema 复用固定内存 URI，不依赖当前路径。
- 默认导航子进程 30 秒，prepare 累计工具预算 120 秒；失败或截断不伪装 COMPLETE。
- 标准小 fixture（≤10 Java 文件、单链）Windows prepare/导航合计 ≤15 秒、finish 本地处理 ≤5 秒；模型时间单独记录。
- ast-grep 0.42.1 Windows ZIP SHA256：fe34f631bb24c08ad146f92ca2a92971a53d179461b509fd8d32dc863bff9f83。
- CI Go 1.23.12、原生 OpenCode 1.18.25 为现有工具核对点；不得因此要求内网用户在线升级或安装 SDK。
- 每个任务先写可观察失败案例，再做最小实现，执行相关回归，提交 exact HEAD 证据。新增测试匹配 0 个不得算通过。
- 按负责人分派启动 T1；每 Task 验收后再推进下一项，不一次性自动开发全部任务或发布。任务验收与正式发布必须按设计第 9 节分层标注。

## 目录和边界

以下 Create 是拟新增路径，Modify 是核对基线中现存路径。禁止把下面的示例接口当作已经实现。

| 文件 | 单一职责 |
|---|---|
| `internal/reviewrun/types.go` | 1.8 窄接口 DTO 与版本标识 |
| `internal/reviewrun/run.go` | 创建/读取 run、幂等与取消 |
| `internal/reviewrun/prepare.go` | 有界发现、单一 nodes 结构、范围/读取记录 |
| `internal/reviewrun/selection.go` | 选项摘要、真实选择绑定、范围生成 |
| `internal/reviewrun/finish.go` | 输入/源码校验、结构化结果保存、报告提交 |
| `internal/reviewrun/report.go`、`review.md.tmpl` | 固定模板与安全 Markdown 输出 |
| `cmd/codea-dcep-tools/review_run_180.go` | 1.8 命令映射；不把业务逻辑塞进既有大文件 |
| `internal/reviewauthority/selection_turn_180.go` | 提取真实下一用户回合判断，不要求已完成的工具 attestation |
| `.code-harness/tools/codea-review.ts` | 结构化 action、当前 Host 上下文绑定、原生 Runtime 调用 |

`internal/*` 和 `cmd/*` 均相对 `.code-harness/tools-runtime/`。不重排旧模块目录。

## 共享接口（由 T1 定义，T2/T3 实现其行为）

```go
package reviewrun

import "context"

type Intent struct {
    Mode string `json:"mode"` // CHANGES | CURRENT_IMPLEMENTATION
    Target string `json:"target"` // empty only for unfiltered CHANGES
}
type Node struct {
    Path string `json:"path"`
    Symbol string `json:"symbol"`
    Role string `json:"role"`
    Workspace string `json:"workspace"`
}
type Chain struct {
    ID string `json:"id"`
    Name string `json:"name"`
    Nodes []Node `json:"nodes"`
    Unresolved []string `json:"unresolved"`
}
type ReadRef struct {
    Path string `json:"path"`
    SHA256 string `json:"sha256"`
    StartLine int `json:"startLine"`
    EndLine int `json:"endLine"`
}
type Evidence struct {
    Ref ReadRef `json:"ref"`
    Quote string `json:"quote"`
}
type Finding struct {
    ID string `json:"id"`
    Severity string `json:"severity"` // CRITICAL | HIGH | MEDIUM | LOW
    Problem string `json:"problem"`
    Impact string `json:"impact"`
    Recommendation string `json:"recommendation"`
    Verification string `json:"verification"`
    Evidence []Evidence `json:"evidence"`
    IntroducedByChange *bool `json:"introducedByChange"`
}
type FinishRequest struct {
    RunID string `json:"runId"`
    Reads []ReadRef `json:"reads"`
    Findings []Finding `json:"findings"`
    PendingRisks []string `json:"pendingRisks"`
    Gaps []string `json:"gaps"`
}
type Options struct {
    RunID string `json:"runId"`
    Hash string `json:"optionsHash"`
    Chains []Chain `json:"chains"`
    DiscoveryComplete bool `json:"discoveryComplete"`
    SelectionRequired bool `json:"selectionRequired"`
    Gaps []string `json:"gaps"`
    ReportPath string `json:"reportPath"`
}
type SelectionRequest struct {
    RunID string `json:"runId"`
    OptionsHash string `json:"optionsHash"`
    IDs []string `json:"selectionIds"`
}
// 来自 Host 执行上下文，不能暴露为模型可填写的 tool args。
// 该标识本身不表示已获授权；Select 必须读取实际消息并核对。
type HostTurn struct { SessionID, MessageID string }
type Outcome struct {
    RunID string `json:"runId"`
    Execution string `json:"execution"`
    ReviewConclusion string `json:"reviewConclusion"`
    Coverage string `json:"coverage"`
    ReportPath string `json:"reportPath"`
    ReportSHA256 string `json:"reportSha256"`
}

func Start(root string) (Outcome, error)
func Prepare(ctx context.Context, root, runID string, intent Intent) (Options, error)
func Select(ctx context.Context, root string, req SelectionRequest, turn HostTurn) (Outcome, error)
func Finish(ctx context.Context, root string, req FinishRequest) (Outcome, error)
func Status(root, runID string) (Outcome, error)
func Cancel(root, runID, reason string) (Outcome, error)
```

以上是接口签名，不是可直接编译的实现文件。数组统一输出 `[]`；schema 拒绝 unknown fields、非法路径、非 UTF-8 和缺必要字段。内部 run.json 使用 schemaVersion=180；不要将 180 解释成发行包 VERSION。

### 关键实现算法

```text
Start:
  分配不冲突的随机 ID；创建 run 目录；写最小 run.json
  原子写“未完成”review.md；回读匹配后返回 INCOMPLETE+路径
Finish:
  读取本 run → 检查范围/选择 → 校验所有输入与源码
  若失败，保留可读报告和原选择，返回具体错误
  若已完成且同内容，返回既有报告；不同内容拒绝覆盖
  原子保存 result.json → 渲染 Markdown → 原子替换 review.md
  回读并核对 result 摘要与终态 → 返回 Outcome
Status:
  读取报告元数据并与 result.json 核对；缺失/不匹配为 INCOMPLETE
```

真实操作以报告文件提交为终点；不另造状态证书/完成事件流。合法输入预检失败不要求新 run；取消后的 finish 拒绝。

## Task 1 — 先让 Runtime 真正写出报告

**Files**

- Create：上述 types/run/finish/report 文件、`internal/reviewrun/run_test.go`、`finish_test.go`、最小 `review.md.tmpl`。
- Create：`cmd/codea-dcep-tools/review_run_180.go` 与 `review_run_180_test.go`。
- Modify：`cmd/codea-dcep-tools/review_precision_command.go` 中 review 子命令路由；新增 start/status/cancel/finish 分支，旧行为不替换。
- Reuse：`internal/requestjson/read.go`、`internal/schema/schema.go`。

**Consumes**：现有配置/路径校验、随机 ID 和文件 I/O 能力；不依赖 Reviewer。
**Produces**：Start/Status/Cancel、严格 Finish 保存机制和最小中文报告；Prepare/Select 此时不得伪装可用。

- [ ] 写 `Test180StartWritesIncompleteReport`、`Test180StartFailsWhenReportUnwritable`、`Test180FinishWithoutScopeKeepsReport`。首个测试的核心断言：

```go
func Test180StartWritesIncompleteReport(t *testing.T) {
    root := t.TempDir()
    got, err := Start(root)
    if err != nil { t.Fatal(err) }
    if got.Execution != "INCOMPLETE" || got.RunID == "" { t.Fatal(got) }
    data, err := os.ReadFile(got.ReportPath)
    if err != nil { t.Fatal(err) }
    if !bytes.Contains(data, []byte("评审未完成")) { t.Fatal(string(data)) }
    if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) { t.Fatal("BOM") }
}
```

- [ ] 运行 `go test ./internal/reviewrun -run '^Test180(Start|FinishWithoutScope)' -count=1 -v`，确认失败原因是缺实现/真实行为，不能忽略编译错误并宣称产品红灯。
- [ ] 实现共享 DTO、最小 Start/Status/Cancel，finish 暂时没有已准备 scope 时明确拒绝。包内测试使用真实临时文件验证已经准备的合法内部状态下的渲染路径；测试夹具不进入生产编译。
- [ ] 增加保存故障测试：result 成功/report 失败、临时文件写入失败、Windows 文件被占用、两并发 run、同 run 两次 finish 并发、同内容 finish 重试、不同内容覆盖拒绝；核对旧报告可读且无假 COMPLETE。单 run 短期互斥只覆盖本地读写，人工等待不占锁。
- [ ] 运行 `go test ./internal/reviewrun ./internal/requestjson ./internal/schema -count=1`，在 Windows 用真实文件句柄完成占用测试。
- [ ] 提交 T1，只报告“Runtime 报告机制通过”，不得称主 Agent 链路完成。

## Task 2 — 有界调用链与真实人工选择

**Files**

- Create：`internal/reviewrun/prepare.go`、`selection.go`、`prepare_test.go`、`selection_test.go`。
- Create：`internal/reviewauthority/selection_turn_180.go`、`selection_turn_180_test.go`。
- Create：`internal/reviewrun/testdata/controller-review/`，含 OrderController 两个 endpoint、OrderServiceImpl、OrderMapper、OrderMapper.xml、明确项目配置与可复现 Git 基线。
- Modify：`review_run_180.go` 的 prepare/select 路由；仅在必要时给 `internal/nav/extended.go`、`project_calls_163.go` 增加窄适配，保留批处理优化。

**Consumes**：T1 Start/Outcome、nodes[] DTO、现有真实导航与下一用户回合判断。
**Produces**：Prepare/Select、绑定内容与菜单的范围，以及源码读取标识。

- [ ] 建立两条链共享一个 Service 的真实 Java/XML fixture，验证不能合并选项绕过用户选择。
- [ ] 写失败测试：`Test180PrepareTwoEndpointsRequiresSelection`、`Test180UnknownImplementationPreventsAutoSingle`、`Test180NodesRejectLegacyParallelRefs`、`Test180SelectNeedsNextActualUser`、`Test180SelectRejectsStaleMenu`、`Test180SelectedSubsetIsExact`。
- [ ] 用表驱动消息夹具验证下表；fixture 只证明 verifier 判断，最终真实 Host 门禁在 T3/T5 验证。

| Host 消息序列 | 结果 |
|---|---|
| 当前菜单 → user「选择 C1」 | 仅 C1 |
| 当前菜单 → assistant「选择 C1」 | 拒绝 |
| user「选择 C1」→ 当前菜单 | 拒绝 |
| 当前菜单 → user「继续」 | 拒绝 |
| 旧菜单 → user「全部」→ 新菜单 | 拒绝 |
| 本 run 菜单 → 其他 run/项目的 user | 拒绝 |

- [ ] 实现预算内导航与读取复用。缺 ast-grep、超时、坏 JSON、缺 range、范围外路径必须返回缺口/错误，不能产一条“完整”选项。使用执行计数器验证同文件多 symbol 只扫描一次，使用真实 ast 验证结果一致。
- [ ] 提取 Host 消息判断：运行中的 select 工具只验证已存在菜单及下一条 user，不依赖该工具先执行完成。运行 export 使用原生进程参数数组和 timeout，保留 opaque ID 与项目绑定。
- [ ] 完成 Prepare/Select 后把 T1 的合法 scope 保存场景改为调用真实 Prepare/Select；保留必要隔离故障夹具。
- [ ] 运行 `go test ./internal/reviewrun ./internal/reviewauthority ./internal/nav -count=1`；Windows 真 ast 执行小项目计时，超预算定位原因。
- [ ] 提交 T2，交付真实选项与所选范围证据。

## Task 3 — 主 Agent 一次提交完成真实报告

**Files**

- Create：`.code-harness/tools/codea-review.ts`、`.github/scripts/review180-host-smoke.py`。
- Modify：`.code-harness/commands/harness-review.md`、`.code-harness/AGENTS.md`、`.code-harness/bootstrap.md`、`.code-harness/agents/orchestrator.md`、`.code-harness/skills/review-code/SKILL.md`。
- Modify：`internal/reviewrun/finish.go`、`finish_test.go`，补齐源码/范围/证据检查；必要时 `review_run_180.go` 增加结构化读取接口供工具内部使用。
- Create：`cmd/codea-dcep-tools/review_entry_180_test.go`。

**Consumes**：T1/T2 完整 Runtime 接口和真实 Host 执行 context。
**Produces**：正式主会话入口与 action 工具；自主模型最小闭环证据。

- [ ] RED：正式命令应在模型第一条回答前已有 review.md；旧入口仅 begin 无文件应失败。命令中的 shell 部分只允许固定 `review start`，用户目标保持普通文本。
- [ ] 工具 args 暴露 action/runId/intent/selection/result；HostTurn 从 context 读取，模型不能填写；工具自行构造 UTF-8 请求、参数数组执行本地 Runtime。对 SDK 使用随正式包已获准的方式，不能新增内网在线安装步骤。
- [ ] 修改当前普通 Review 指令为唯一流程，删除其中互相矛盾的旧 Reviewer/认证步骤，历史说明明确隔离；不得只在一篇长旧指令顶部叠一段“新优先级”。
- [ ] finish 同一次工具调用内部完成校验、写盘和回读；返回固定的 execution/conclusion/path/hash。空 findings 同样调用 finish，部分覆盖不能显示 NO_ISSUES_FOUND。
- [ ] 对 tool 输入写 BOM、缺字段、旧 chainRefs、跨范围、源码变化、opaque Host ID、Windows 路径和取消测试。预检失败修正原 run，禁止自动重跑整轮。
- [ ] 原生 Host 确定性 fixture 测试验证执行传输与 next-user 选择；再接可用真实模型执行单链问题样本。驱动只发 `/harness-review OrderController` 等正常用户输入，监测实际工具调用。
- [ ] 自主执行驱动允许的动作与禁止项：

```python
# 驱动伪代码；只暴露用户会做的交互，不补 Runtime 步骤。
session = start_native_opencode(project, configured_model)
send_user(session, "/harness-review OrderController")
events = observe_until_idle(session)
# 双链场景此处验证尚未 finish，再发送真实明确选择。
# 单链场景不追加“请调用 finish”的人工救场消息。
assert observed_tool(events, "codea-review", action="finish")
assert_report_matches_tool_return(project, events)
```

- [ ] 上述驱动的 helper 在 `review180-host-smoke.py` 定义，复用已验证原生 OpenCode 启动/观察代码；**不得复用 release167-review-e2e.py 中脚本主动 rt(certify/report) 的业务执行方式**。
- [ ] 若真实模型没有自主 finish，记录失败并修入口/工具交互，不通过注入脚本调用绕过。未通过前不进入 T4 扩充。
- [ ] 运行相关 Runtime、Host 与入口测试，提交 T3；明确所用模型与未覆盖的私有模型环境。

## Task 4 — 汇报版式和历史故障永久回归

**Files**

- Modify：`internal/reviewrun/report.go`、`review.md.tmpl`、`finish.go`。
- Create：`internal/reviewrun/report_test.go`、`regression_180_test.go`、`testdata/reports/` 下六类 golden。
- Modify：仅真实新流程不再适用的 active Review contract 测试；每处记录旧断言与新断言理由，不删除历史兼容测试。

**Consumes**：T3 真实结果、scope/读取记录；已批准五段报告样稿。
**Produces**：固定汇报模板、H01–H17 追踪表与全部相关回归。

- [ ] 六个 golden：阻断问题、非阻断问题、无问题、覆盖未完成、等待选择、取消；再覆盖零选项无变更和当前实现模式。
- [ ] 断言首屏有独立执行/风险/覆盖信息；数字从 findings 派生；待确认风险不计正式问题；未选链不能出现在已检查列表。结论矩阵：PARTIAL/未完成/取消→UNDETERMINED，完整且含严重/高→BLOCKING，仅中/低或待确认→ACTION_REQUIRED，完整且两类列表都为空→NO_ISSUES_FOUND。
- [ ] 断言合法证据包含源码中的竖线、反引号、HTML、Windows/中文路径时排版仍正确且无主动内容；长链有完整明细。
- [ ] 对照设计 H01–H17 给每项填写真实 testName/HostScenario；未覆盖不得以文字“已考虑”通过。
- [ ] Markdown 渲染人工查看首屏、链表、问题卡片和长内容折行；只验证实际生成的报告，不拿手写样稿替代。
- [ ] 运行 `go test ./... -count=1` 与 `go vet ./...`；针对修改的新流程契约列出差异，不修改无关 Task153 历史功能。
- [ ] 提交 T4，交付 golden 与真实生成文件截图/预览证据。

## Task 5 — 离线升级、真实模型矩阵与最终验收

**Files**

- Create：`.github/workflows/release-1.8.0-windows-x64.yml`、`.github/scripts/release180-package.ps1`、`release180-upgrade-regression.ps1`、`review180-real-model-e2e.py`。
- Modify：`.code-harness/tools-runtime/internal/upgrade/` 中 managed Host inventory 和版本迁移的窄适配；增加 `release_180_test.go`。
- Modify：`.code-harness/VERSION`→1.8.0、README、CHANGELOG、`.code-harness/upgrade.md`、相关现行 release contract。
- Reuse：1.6.7 真实发布包下载/hash/文件清单/升级事务验证代码；旧 installer/builder 不重写。

**Consumes**：T1–T4 已验收产品、新 Host 文件清单。
**Produces**：与 exact HEAD 绑定的候选包、真实模型/Windows/内网分层证据；所有门槛满足后才是正式交付。

- [ ] 固定 1.6.7 baseline：HEAD `970573a6d2147733b773c0de0c124d9324914ae8`、Release Run `34950443217`、install artifact `10389138981`、checklist `10389298093`。下载时验证存续状态与官方 digest；过期时从该 exact HEAD 正式重建并标注来源，不能冒充原 artifact。
- [ ] 将新 `codea-review.ts` 和更新入口纳入 installer/upgrade manifest；真实 1.6.7→1.8.0 更新工具、命令和指令。验证用户 Host 文件冲突零覆盖、未知文件保留、apply 后故障全回滚、同版本 noop。
- [ ] 包内运行 ast-grep --version，校验 pinned 下载 SHA；临时目录含空格、中文、`&`、`#`、`%`。升级后重开原生 Host 会话再调用正式入口。
- [ ] Windows fresh 全量 `go test -count=1 ./...`、`go vet ./...`，所有相关集成测试使用真实工具；无包时 fail，不能 skip。
- [ ] 真模型矩阵：单链有问题、单链无问题、双链选 C1 各 3 个 fresh run；当前实现无 diff、目标无相关变更、模型提前停止、工具失败/超时作为额外场景。
- [ ] 种入问题至少包括明确的租户隔离缺口和无条件 SQL 更新；干净对照包含真实保护条件。真实模型必须找到预期高风险证据，不通过空结果绕开验证。
- [ ] 驱动不注入预制 findings 或工具调用指令，不替模型执行 prepare/select/finish/report。双链测试在回复前观测无 finding review/finish、仅有未完成报告；回复后选中范围必须精确。
- [ ] CI 对无真实模型配置的运行输出 `REAL_MODEL_NOT_VERIFIED` 并阻断正式发布标记；fixture transport PASS 不能顶替。凭据只来自已批准环境，不提交或输出 secret。
- [ ] 产出候选包供内网同模型同 Windows 实测；无法直连内网时该层保持未验证，请用户回传脱敏 run/report/实际工具输出，不上传业务源码。
- [ ] 每个场景证据结构至少为：

```json
{
  "schemaVersion": 1,
  "head": "actual-40-character-head",
  "scenario": "two-chains-select-c1",
  "testKind": "REAL_MODEL_AUTONOMOUS",
  "model": "actual-configured-model",
  "hostVersion": "actual-host-version",
  "runId": "actual-runtime-id",
  "userSelectionObserved": true,
  "modelInvokedFinish": true,
  "driverInvokedRuntimeBusinessSteps": false,
  "execution": "COMPLETE",
  "reportPath": "actual-report-path",
  "reportSha256": "actual-sha256",
  "expectedFindingsFound": true,
  "timings": {"modelMs": 0, "navigationMs": 0, "finishMs": 0, "totalMs": 0}
}
```

以上是字段示意，实际证据的 head/hash/时间必须采集，不能照抄示意值。fixture 测试标 `HOST_TRANSPORT_FIXTURE`；内网标 `PRIVATE_MODEL_ACCEPTANCE`。

- [ ] 提交最终产品 HEAD 后获取 fresh Run ID/Job ID 和制品 SHA；确认报告在模型会话中显示路径且磁盘可打开，才向用户提交验收。

## 设计自查与交付声明

- H01/H03/H10 复用已有修复并回归，避免重复重写。
- H02/H04/H07 通过删除普通 Review 的不必要协议解决，不移除人工选择与路径安全。
- H05/H06/H14 通过入口物理文件、单次 finish、真实模型不代执行验收解决。
- H08/H09 用真实导航/打包/预算证据；模型总时长不以 fixture 代替。
- H11 在 T2 机械测试、T3 原生 Host、T5 真实模型/用户选择三层覆盖。
- H12/H13/H17 保留项目状态及历史兼容，不恢复废弃 workflow。
- H15/H16 报告 golden、故障注入、幂等/并发测试覆盖。

设计完成不等于开发完成；Runtime 组件 PASS 不等于 Host PASS；Host fixture PASS 不等于真实模型自主完成；候选包可下载不等于内网验收通过。最终对外报告必须列明真实完成层次。
