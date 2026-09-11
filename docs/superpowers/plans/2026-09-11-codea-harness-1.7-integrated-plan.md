# Codea Harness 1.7 完整项目执行计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. 人工研发可直接按任务执行，不要求安装这些 Agent 技能。

**Goal:** 在内网零新增依赖的默认约束下，交付自动调用上下文、业务知识辅助且证据可追溯的Code Review，并保持现有TeamAI/Git/安装升级边界。

**Architecture:** Runtime从已认证变更/范围按需分析关系；独立Reviewer负责语义审核，Runtime认证并渲染。业务规则从显式本地来源读取，TeamAI只同步其原资源，Harness完整包仍通过原机制安装升级。

**Tech Stack:** 核对点go.mod为Go 1.23.10，沿用Git、正式包ast-grep、现有yaml.v3/JSON Schema及已批准模块；不新增LSP、数据库、服务或模型客户端。

**Spec:** [1.7完整设计](../specs/2026-09-11-codea-harness-1.7-integrated-design.md)。本文已包含所需的代码任务和接口，不要求拼接旧八任务计划。

**Status:** 研发待执行，所有复选框保持未完成。代码位置核对点为 `b864bd19b9313bd882da89612e701d8bb3221e7d`；正式实施基线由T1核实最终验收通过的1.6.4.final。本次仅提交文档。

## Global Constraints

- 只新增 Code Review 八项能力及其内网/TeamAI 协同；不扩展 Test、Debug、Fix、Verify 或 API Doc 产品能力。
- TeamAI 原有目录、配置和管理方式保持不变，仅增加 Harness 接入资源；不迁移现有知识。
- Git 管理总仓文件，TeamAI 命令同步其资源，Harness 沿用原正式安装/升级流程；三者不能相互替代。
- 不新增持久代码索引、跨 run 解析缓存、图数据库、图查询、后台监听或 LSP/JDT。
- 不新增快照或独立认证体系；保留既有 ChangeSet、certification、freshness、Reviewer Host 和八阶段 Runtime 权威。
- 不整套集成 OpenCodeReview/Code Review Graph，不引入向量库、Embedding 或新的知识管理平台。
- 每次 Review 自动派生本次调用链；正常 Review 不写 .code-harness/chains/**，不要求人工刷新。
- 内网完全无法访问外网；默认零新增第三方依赖，缺包逐项申请，不自动下载或通过 vendor 绕过审批。
- Review 内不执行 Git/TeamAI 同步或自动升级；上下文不扩大 FULL/TARGETED、代码读写范围或用户授权。
- 日志平台、登录验证码、连接器/MCP、故障诊断、修复增强及经验自动生成不进入本版。
- 不另建发布保障、升级回滚或评测平台；必要的版本适配和验证复用现有机制。
- 正式开发从验收通过的 1.6.4.final 建立独立分支，本次交付只包含文档，不宣称功能已实现。

## 执行顺序、责任与工作量

```text
T1 基线与契约
  ├─ T2 Java/Spring ─→ T4 Dubbo
  ├─ T3 MyBatis
  └─ T7 知识读取（最终命令接线依赖T5）
T2 + T3 + T4 → T5 代码上下文/阶段接线 → T6 自动临时链
T5 + T6 + T7 → T8 证据/Host检查完成声明/报告
T1 + T7 + T8 → T9 TeamAI及原安装升级兼容 → T10真实验收
```

| 任务 | 建议负责角色 | 规划工作量（人日） |
|---|---|---|
| T1 基线与契约 | 核心负责人 | 1–2 |
| T2 Java/Spring | 解析开发 | 5–7 |
| T3 MyBatis | 解析开发 | 3–4 |
| T4 Dubbo | 解析开发 | 2–3 |
| T5 上下文及阶段接线 | Runtime开发 | 4–6 |
| T6 自动临时链 | Runtime开发 | 2–3 |
| T7 知识读取 | Runtime开发 | 4–6 |
| T8 证据/Host/报告 | Runtime与Host开发 | 5–7 |
| T9 协同与兼容 | 集成开发、内网管理员 | 2–4 |
| T10 真实验收 | 测试、业务负责人、研发 | 3–5 |

合计31–47人日，仅用于研发评估排期，不是完成承诺；不含等待1.6.4.final验收、内网依赖审批或环境准备。T1后解析与知识读取可分工并行，共享schema、Host、Orchestrator由一名负责人协调。每任务独立提交，依据真实测试验收后进入依赖任务。

里程碑：M1=T1契约冻结；M2=T2–T6技术Review；M3=T7–T8业务Review；M4=T9–T10完整使用验收。不能将M2标成1.7全部交付。

## 执行环境和文件清单规则

所有Create路径为拟新增、Modify路径按本次代码树核对；最终基线若移动路径，先在T1记录映射，不趁机重构无关代码。命令默认在核心仓 `.code-harness/tools-runtime` 执行；TeamAI操作在实际业务项目根执行。

开发/测试所需Go、Git、ast-grep、OpenCode、TeamAI及PowerShell必须已在执行环境获准准备。Windows需可用的 `pwsh`；系统自带 `powershell.exe` 不等同于它。离线测试设置：

```powershell
$env:GOTOOLCHAIN = "local"
$env:GOPROXY = "off"
$env:GOSUMDB = "off"
$env:CODEA_AST_GREP = "C:\approved-tools\ast-grep.exe"
```

ast-grep路径为操作示例，使用批准包实际位置；该变量只为真实AST测试定位工具，不改变生产安装机制。缺依赖记录名称/版本及申请，不自动联网、不跳过测试。

按任务运行新增失败案例→最小实现→相关回归。每步中的测试名是计划要求的实际测试名，写入测试后检查匹配数量，不以零测试通过代替验证。文档/模板本身使用格式、引用与复制场景检查，不为低影响文本修改新建测试框架。

## 共享接口：T1 定义，后续任务保持一致

这些是拟新增的 Go 接口，不是声称当前已存在。字段的 JSON 名按下述 lowerCamelCase 明确标注；所有 slices 输出为空数组而不是 null。

```go
// internal/nav/review_types_170.go
package nav

type ReviewRef170 struct {
    Workspace string   `json:"workspace"`
    Path      string   `json:"path"`
    Side      string   `json:"side"` // BASE | CURRENT
    Kind      string   `json:"kind"` // METHOD | FIELD | TYPE | STATEMENT | SQL_FRAGMENT
    OwnerFQCN string   `json:"ownerFqcn"`
    Name      string   `json:"name"`
    ParameterTypes []string `json:"parameterTypes"`
}
type SourceRange170 struct {
    Ref ReviewRef170 `json:"ref"`
    StartLine int `json:"startLine"`
    EndLine int `json:"endLine"`
    StartColumn int `json:"startColumn"`
    EndColumn int `json:"endColumn"` // 一基Unicode码点列、右开边界；由AST字节位置统一转换
}
type Relation170 struct {
    ID string `json:"id"`
    Kind string `json:"kind"`
    Resolution string `json:"resolution"`
    From ReviewRef170 `json:"from"`
    Targets []ReviewRef170 `json:"targets"`
    Evidence []SourceRange170 `json:"evidence"`
    Reason string `json:"reason"`
    Assumptions []string `json:"assumptions"`
}
type Issue170 struct {
    Code string `json:"code"`
    At SourceRange170 `json:"at"`
    Detail string `json:"detail"`
}
type MethodFacts170 struct {
    Method ReviewRef170 `json:"method"`
    Calls []Relation170 `json:"calls"`
    Issues []Issue170 `json:"issues"`
}
type Injection170 struct {
    Owner ReviewRef170 `json:"owner"`
    Name string `json:"name"`
    DeclaredType string `json:"declaredType"`
    Kind string `json:"kind"` // FIELD | CONSTRUCTOR | SETTER
    Qualifier string `json:"qualifier"`
    ResourceName string `json:"resourceName"`
    Evidence []SourceRange170 `json:"evidence"`
}

// 下列函数体分别由 T1/T2 实现。
func ReviewRefKey170(ref ReviewRef170) (string, error)
func ValidateRelation170(relation Relation170) error
func (n Navigator) InspectMethod170(ctx context.Context, ref ReviewRef170) (MethodFacts170, error)
func (n Navigator) ResolveInjection170(ctx context.Context, injection Injection170) (Relation170, error)
```

方法声明段用于说明签名，复制到 Go 文件时由任务实现函数体并补标准库 import。不改变已有 Navigator 的三字段配置；跨 workspace 由上层为每个 VERIFIED root 创建对应 Navigator。

```go
// internal/reviewcontext/model_170.go
package reviewcontext

import (
    "context"
    "codea-harness-tools/internal/nav"
)

type Need170 struct {
    ReviewUnitID string `json:"reviewUnitId"`
    RuleID string `json:"ruleId"`
    Seeds []nav.ReviewRef170 `json:"seeds"`
    RequiredKinds []string `json:"requiredKinds"`
}
type Budget170 struct {
    MaxFiles int
    MaxCandidates int
    MaxSourceBytes int
    MaxUpstreamDepth int
    MaxDownstreamDepth int
    MaxMillis int
}
type Usage170 struct {
    Files int `json:"files"`
    Candidates int `json:"candidates"`
    SourceBytes int `json:"sourceBytes"`
    ElapsedMillis int64 `json:"elapsedMillis"`
}
type BuildInput170 struct {
    RunID string
    Phase string
    Seeds []nav.ReviewRef170
    Resources []nav.SourceRange170
    Needs []Need170
    Budget Budget170
}
type Check170 struct {
    ReviewUnitID string `json:"reviewUnitId"`
    RuleID string `json:"ruleId"`
    Status string `json:"status"` // READY | BLOCKED
    RelationIDs []string `json:"relationIds"`
    Reasons []string `json:"reasons"`
}
type Context170 struct {
    RunID string `json:"runId"`
    Phase string `json:"phase"`
    Relations []nav.Relation170 `json:"relations"`
    Checks []Check170 `json:"checks"`
    Issues []nav.Issue170 `json:"issues"`
    Usage Usage170 `json:"usage"`
}
type Resolver170 interface {
    Method(context.Context, nav.ReviewRef170) (nav.MethodFacts170, error)
    Callers(context.Context, nav.ReviewRef170) ([]nav.Relation170, error)
    Mapper(context.Context, nav.SourceRange170) ([]nav.Relation170, []nav.Issue170, error)
    Dubbo(context.Context, nav.ReviewRef170) ([]nav.Relation170, []nav.Issue170, error)
}

// T1 提供默认值；T5 实现编排与复验。
func DefaultBudget170() Budget170
func Build170(ctx context.Context, input BuildInput170, resolver Resolver170) (Context170, error)
func VerifyRelations170(ctx context.Context, claimed Context170, input BuildInput170, resolver Resolver170) error
```

Resolver.Method 的真实适配器负责在 JAVA_CALL 接口目标之后调用 Spring 解析。Resolver.Callers 委托已有候选搜索 + 新签名确认。Mapper/Dubbo 的适配器只能访问 Runtime 传入的已验证 roots；所有 BASE 内容获取复用既有 base 提取流程，不新建快照或源文件封存机制。

结构不依赖 analysis/reviewunit：由命令层加载既有权威产物，构造 BuildInput170；analysis/finding 只在需要验证关系时消费上述低层接口，禁止循环导入。

### 状态及错误契约

- 非 EXACT 关系必须有 Reason；EXACT 必须有实际源码 Evidence。
- callsite 使用起止行列，不能只用行号；增加同一行两次相同调用和Unicode前缀的位置测试，调用点ID必须不同且源码定位准确。
- JAVA_CALL 的 EXACT 目标必须唯一；SPRING_BINDING 的 EXACT 目标必须唯一且注册条件已确定。
- MYBATIS_STATEMENT / SQL_INCLUDE 的 EXACT 只针对具体语句/片段关联，不表示动态 SQL 必然执行。
- DUBBO_CONTRACT 的 EXACT 仅指源码契约匹配，不表示实际路由。
- 明确错误码：JAVA_OVERLOAD_UNRESOLVED、JAVA_RECEIVER_UNRESOLVED、SPRING_BEAN_AMBIGUOUS、SPRING_CONDITION_UNRESOLVED、GENERATED_CONSTRUCTOR_UNSUPPORTED、SQL_INCLUDE_CYCLE、SQL_INCLUDE_UNRESOLVED、DUBBO_CONFIG_UNRESOLVED、PROVIDER_SOURCE_UNAVAILABLE、CONTEXT_BUDGET_EXCEEDED、RECURSION_BOUNDARY、CONTEXT_RELATION_NOT_VERIFIED。
- 用户输入/协议/路径错误返回 error；正常业务未知返回 Issues/非 EXACT，不把全部不支持场景伪装成进程崩溃。
- 同一输入按 workspace/path/owner/signature/callsite 排序；实际耗时不参与关系 ID 或语义去重键。

## 共享知识接口：T1 定义，T7/T8 使用

以下均为拟新增接口。它们属于 `internal/knowledge`，仅依赖现有路径/文件封装与 Go 标准库，不导入 finding/report/reviewrules，防止循环依赖。命令层负责把权威 ReviewUnit 投影成 Unit170。

```go
// internal/knowledge/model_170.go
package knowledge

type Source170 struct {
    ID string `json:"id" yaml:"id"`
    Root string `json:"root" yaml:"root"`
    Path string `json:"path" yaml:"path"`
    Kind string `json:"kind" yaml:"kind"`
    Required bool `json:"required" yaml:"required"`
}
type Binding170 struct {
    Version int `json:"version" yaml:"version"`
    ProjectID string `json:"projectId" yaml:"projectId"`
    TeamRoot string `json:"teamRoot,omitempty" yaml:"teamRoot,omitempty"`
    Sources []Source170 `json:"sources" yaml:"sources"`
}
type Unit170 struct {
    ID string
    Paths []string
    EntryPoints []string // ownerFqcn#method(parameterTypes)
}
type AppliesTo170 struct {
    Paths []string `json:"paths" yaml:"paths"`
    EntryPoints []string `json:"entryPoints" yaml:"entryPoints"`
}
type Rule170 struct {
    RuleID string `json:"ruleId" yaml:"ruleId"`
    ProjectID string `json:"projectId" yaml:"projectId"`
    Status string `json:"status" yaml:"status"`
    Version string `json:"version" yaml:"version"`
    Owner string `json:"owner" yaml:"owner"`
    Source string `json:"source" yaml:"source"`
    ApprovalRef string `json:"approvalRef" yaml:"approvalRef"`
    AppliesTo AppliesTo170 `json:"appliesTo" yaml:"appliesTo"`
    Supersedes []string `json:"supersedes" yaml:"supersedes"`
    Exceptions []string `json:"exceptions" yaml:"exceptions"`
}
type SourceRecord170 struct {
    SourceID string `json:"sourceId"`
    Root string `json:"root"`
    Path string `json:"path"`
    Kind string `json:"kind"`
    Required bool `json:"required"`
    SHA256 string `json:"sha256,omitempty"`
    Rule *Rule170 `json:"rule,omitempty"`
    Status string `json:"status"` // READY | BLOCKED | NOT_APPLICABLE
    Reasons []string `json:"reasons"`
}
type BusinessCheck170 struct {
    ReviewUnitID string `json:"reviewUnitId"`
    RuleKey string `json:"ruleKey"` // BUSINESS:<sourceId>:<ruleId>
    SourceID string `json:"sourceId"`
    RuleID string `json:"ruleId"`
    Status string `json:"status"` // READY | BLOCKED
    Reasons []string `json:"reasons"`
}
type Manifest170 struct {
    RunID string `json:"runId"`
    ProjectID string `json:"projectId"`
    Status string `json:"status"` // NOT_CONFIGURED | READY | PARTIAL | INVALID
    BindingSHA256 string `json:"bindingSha256"`
    Sources []SourceRecord170 `json:"sources"`
    Checks []BusinessCheck170 `json:"checks"`
    Issues []string `json:"issues"`
}
type Document170 struct { SourceID string; Content string }
type LoadResult170 struct {
    Manifest Manifest170
    Documents []Document170 // 只作为当次返回值，不序列化进运行记录
}
type LoadInput170 struct {
    RunID string
    RepoRoot string
    Binding Binding170
    BindingSHA256 string
    Units []Unit170
}
```

实现的函数签名：

```go
func ParseBinding170(data []byte) (Binding170, error)
func ParseRule170(data []byte) (Rule170, string, error)
func Applies170(rule Rule170, unit Unit170) bool
func Load170(ctx context.Context, input LoadInput170) (LoadResult170, error)
func Verify170(ctx context.Context, input LoadInput170, expected Manifest170) error
```

Go 文件按函数需要导入标准库 context；这里的声明展示对接签名，不是可直接编译的空函数。ParseBinding170 校验 YAML/字段与路径词法，Load170 校验文件最终路径、类型与实际身份；不配置时由命令层构造 NOT_CONFIGURED，不能用空 Binding 冒充一个有效配置。绑定摘要为实际 context.yaml 字节的 SHA256；未配置时使用字符串 `ABSENT`，后续发现文件出现同样算绑定变化。

Manifest 按 sourceId、unitId/ruleKey 排序，空数组统一 `[]`，不含时间和机器绝对路径；规范序列化所得 SHA256 作为 knowledgeSha256。BusinessCheck 的 RuleKey 是 dispatch 身份，证据中的 ruleId 是业务文档身份，不能混用。

固定错误/缺口码：KNOWLEDGE_CONFIG_INVALID、KNOWLEDGE_PATH_DENIED、KNOWLEDGE_SOURCE_MISSING、KNOWLEDGE_RULE_INVALID、KNOWLEDGE_PROJECT_MISMATCH、KNOWLEDGE_RULE_DUPLICATE、KNOWLEDGE_RULE_NOT_ACTIVE、KNOWLEDGE_BUDGET_EXCEEDED、KNOWLEDGE_SOURCE_CHANGED、KNOWLEDGE_BINDING_CHANGED、KNOWLEDGE_REFERENCE_INVALID、REVIEW_CHECKS_INCOMPLETE。路径/资料问题在 manifest 标记不可用；非法 run/请求、既有权威前提不成立返回命令错误并沿用硬停止。

## Task 1: 最终基线、共享契约和真实 AST 测试入口

**对应需求：** F1/F5/F7/F8基础，不创建独立基础设施功能。

- [ ] **前置步骤：确认实施基线。** 取得1.6.4.final最终验收记录，核对commit与正式包；从该commit建立独立1.7开发分支。执行并记录 `git rev-parse HEAD`、`git status --short`，将下文已核对路径与最终树对照。无法确认基线时先保留文档准备，不把当前main默认为验收通过。

**Files**

- Create: `.code-harness/tools-runtime/internal/nav/review_types_170.go`
- Create: `.code-harness/tools-runtime/internal/nav/review_types_170_test.go`
- Create: `.code-harness/tools-runtime/internal/nav/review_fixture_170_test.go`
- Create: `.code-harness/tools-runtime/internal/reviewcontext/model_170.go`
- Create: `.code-harness/tools-runtime/internal/reviewcontext/model_170_test.go`
- Create: `.code-harness/contracts/review-context-request.schema.json`
- Create: `.code-harness/contracts/review-context.schema.json`
- Create: `.code-harness/tools-runtime/internal/schema/review_context_170_test.go`

**Interfaces:** 提供共享数据类型、ReviewRefKey170、ValidateRelation170、DefaultBudget170；此时不公开尚未完成的产品调用入口。

- [ ] **Step 1：添加身份碰撞及关系校验测试（先完成本任务前置步骤的最终基线核对）。** 方法同名不同参数类型、同名不同包、BASE/CURRENT、workspace 不同必须产生不同 key；缺 Evidence 的 EXACT、多目标 EXACT、../ 路径必须拒绝。核心测试：

```go
func Test170OverloadIdentity(t *testing.T) {
    a := ReviewRef170{
        Workspace: "current", Path: "src/main/java/a/Pay.java",
        Side: "CURRENT", Kind: "METHOD", OwnerFQCN: "a.Pay",
        Name: "pay", ParameterTypes: []string{"java.lang.String"},
    }
    b := a
    b.ParameterTypes = []string{"java.lang.Long"}
    ka, err := ReviewRefKey170(a)
    if err != nil { t.Fatal(err) }
    kb, err := ReviewRefKey170(b)
    if err != nil { t.Fatal(err) }
    if ka == kb { t.Fatal("overloads share an identity") }
}
```

- [ ] **Step 2：运行失败测试。** `go test ./internal/nav ./internal/schema -run Test170 -count=1`；未实现函数/契约应失败，记录实际输出。
- [ ] **Step 3：实现类型和严格校验。** key 使用标准库 JSON 序列化后的明确字段，不用易碰撞的字符串拼接；沿用既有 projectpath 归一化，防止路径穿越和 Windows 大小写路径重复。XML 引用使用 STATEMENT/SQL_FRAGMENT，不伪装成方法。
- [ ] **Step 4：实现 request schema。** 必填 runId/phase，additionalProperties=false；phase 仅 DISCOVERY/RULES。响应 schema 覆盖 Context170、Relation170、SourceRange170 及枚举。预算不得由 request 覆盖。
- [ ] **Step 5：建立后续真实 AST 测试公共 helper。** 放在 nav 同包测试中，签名固定如下，T2 使用它；缺批准工具必须失败，不能 Skip。

```go
func newReviewFixture170(t *testing.T, files map[string]string) Navigator {
    t.Helper()
    exe := os.Getenv("CODEA_AST_GREP")
    if exe == "" { t.Fatal("CODEA_AST_GREP must point to the approved ast-grep binary") }
    if _, err := os.Stat(exe); err != nil { t.Fatal(err) }
    root := t.TempDir()
    for path, content := range files {
        dst := filepath.Join(root, filepath.FromSlash(path))
        if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil { t.Fatal(err) }
        if err := os.WriteFile(dst, []byte(content), 0644); err != nil { t.Fatal(err) }
    }
    return Navigator{RepoRoot: root, AstGrepPath: exe}
}
```

helper 只接受测试常量 map；生产路径校验不能复用未经验证的 map 写盘逻辑。测试文件补 context/os/path/filepath/testing 等所需标准库 import。

- [ ] **Step 6：实现默认探索预算并验证。** 每阶段 40 files / 200 candidates / 1048576 bytes / upstream 3 / downstream 6 / 15000 ms。
- [ ] **Step 7：运行 `go test ./internal/nav ./internal/reviewcontext ./internal/schema -run Test170 -count=1`，检查无新增 go.mod/go.sum，提交。**

**独立交付：** 契约与身份校验可用，未启用未完成的 Review 行为。建议 commit：`feat(review): define run-local call context contracts`。

### T1 补充：最终基线、知识契约和状态接线

**Files**

- Create: `.code-harness/tools-runtime/internal/knowledge/model_170.go`
- Create: `.code-harness/tools-runtime/internal/knowledge/config_contract_170_test.go`
- Create: `.code-harness/tools-runtime/internal/finding/checks_170.go`
- Create: `.code-harness/contracts/context-config.schema.json`
- Create: `.code-harness/contracts/business-rule.schema.json`
- Create: `.code-harness/contracts/review-knowledge-request.schema.json`
- Create: `.code-harness/contracts/review-knowledge.schema.json`
- Create: `.code-harness/contracts/review-checks.schema.json`

- [ ] **Step 8：冻结新增契约。** 按本计划共享类型和设计第13节建schema：严格字段、必填required、状态、来源及检查完成声明。schema测试以合法示例和未知字段、重复YAML key、缺失字段、数组/envelope错用为正反例。保持finding-proposals顶层数组。
- [ ] **Step 9：确认阶段接入。** DISCOVERY位于CHANGE_ANALYSIS；knowledge和RULES均属于REVIEW_PLANNING。记录当前dispatch自动推进位置，并在T5中将推进放到最终规划完成后；不新增可供Agent传入的阶段控制参数。
- [ ] **Step 10：确认Host v2和旧run兼容策略。** FINDINGS新增checks绑定，CHANGE_ANALYSIS仍沿用原形式；基于可信run版本决定需求。仅把新接口定义合入T1，不提前激活未实现的生产分支。
- [ ] **Step 11：验证契约。** `go test ./internal/schema ./internal/knowledge ./internal/nav -run Test170 -count=1`；检查所有新增schema字段与下游Go JSON标签完全一致，记录无新依赖。


## Task 2: Java / Spring 精确解析

**对应需求：** F1。

**Files**

- Create: `.code-harness/tools-runtime/internal/nav/java_resolution_170.go`
- Create: `.code-harness/tools-runtime/internal/nav/java_resolution_170_test.go`
- Create: `.code-harness/tools-runtime/internal/nav/spring_resolution_170.go`
- Create: `.code-harness/tools-runtime/internal/nav/spring_resolution_170_test.go`
- Modify: `.code-harness/tools-runtime/internal/nav/project_calls_163.go`
- Modify: `.code-harness/tools-runtime/internal/nav/extended.go`
- Modify: `.code-harness/tools-runtime/internal/nav/workspace_ast.go`
- Modify: `.code-harness/tools-runtime/internal/chain/project_discover_163.go`

**Interfaces:** 消费 T1 ReviewRef170/Relation170；实现 Navigator.InspectMethod170 和 ResolveInjection170。旧 FindDirectMethodCalls 通过同一实现提供兼容投影；未知/重载不唯一不得继续返回 Resolved=true。

- [ ] **Step 1：加入真实 Java fixture 与测试。** 核心 super 回归：

```java
package demo;
class Parent { void pay() {} }
class Child extends Parent {
    @Override void pay() {}
    void submit() { super.pay(); }
}
```

放入 newReviewFixture170 的 `src/main/java/demo/Child.java`。InspectMethod170 输入 Child.submit()；断言唯一 JAVA_CALL 目标 OwnerFQCN=demo.Parent、Name=pay，禁止 demo.Child.pay。同一文件两个类型必须按 AST 所属类型区分。

- [ ] **Step 2：增加重载与遮蔽反例。**

```java
package demo;
class First { void check() {} }
class Second { void check() {} }
class Example {
    First service;
    void run(Second service) { service.check(); }
    void send(String value) {}
    void send(Long value) {}
    void uncertain() { send(null); }
}
```

run 解析到 Second.check；uncertain 对 String/Long 保留歧义，不能只按参数个数选中。跨包同名类型分别建 a.Service/b.Service 的 fixture，验证 explicit import 决定声明类型。

- [ ] **Step 3：执行 `go test ./internal/nav -run 'Test170(Java|Super|Overload|Shadow)' -count=1`，确认当前实现不能满足新增断言。**
- [ ] **Step 4：实现 AST 所属类型、作用域和签名抽取。** super 从父声明查找，局部/参数遮蔽字段；简单实参类型才做唯一重载选择。lambda、反射或推断不足返回明确未知。不要引入 JDT/JavaParser。
- [ ] **Step 5：加入 Spring 正反例。** 两个实现 + Qualifier、单 Primary、两个 Primary、Resource(name)、显式构造器、单构造器无注解、setter、@Bean、缺失 Profile、Lombok @RequiredArgsConstructor。每个例子明确预期：

| 用例 | 预期 |
|---|---|
| 完整已知注册集合 + 唯一 Qualifier 匹配 | EXACT |
| 两个可用候选，唯一 Primary 且无限定冲突 | EXACT |
| 两个 Primary | AMBIGUOUS |
| Profile/Conditional 无法确定 | CONDITIONAL 或 UNRESOLVED，禁止 EXACT |
| Lombok 生成构造器且无可独立证明字段注入 | GENERATED_CONSTRUCTOR_UNSUPPORTED |
| 普通同名 @Service 注解而 import 不属于 Spring | 不作为 Spring 注册证据 |

- [ ] **Step 6：实现候选集合与筛选。** 先判断是否具有完整已知注册范围；仅遇到一个源码实现不代表唯一 Bean。记录候选来源和条件；配置未知不能让 LLM 补事实。ResolveInjection170 返回 SPRING_BINDING，JAVA_CALL 和注入选择分开。
- [ ] **Step 7：让 project discovery 调用共享实现。** 保留 legacy workspace/path 标识；不能把复杂签名强行压入裸名 Chain。方法已删除给未知，不自动换成同名重载。
- [ ] **Step 8：运行 `go test ./internal/nav ./internal/chain -count=1` 和既有 workspace 相关回归，检查真实 AST 解析成功，提交。**

**独立交付：** 现有调用导航得到更准确结果，未接入新的 Review 自动流程。建议 commit：`feat(nav): resolve Java calls and Spring injection conservatively`。
## Task 3: MyBatis 关联上下文

**对应需求：** F3。

**Files**

- Create: `.code-harness/tools-runtime/internal/reviewcontext/mybatis_170.go`
- Create: `.code-harness/tools-runtime/internal/reviewcontext/mybatis_170_test.go`
- Modify: `.code-harness/tools-runtime/internal/chain/project_discover_163.go`
- Modify: `.code-harness/review-rules/spring-v1.yaml`

**Interfaces:** 本任务实现 `ResolveMapper170(ctx context.Context, repoRoot string, source nav.SourceRange170) ([]nav.Relation170, []nav.Issue170, error)`；T5 的 Resolver.Mapper 适配此函数。只处理当前提供的 source root；BASE 语句比较由 T5 以既有 base 读取能力供给，不让本函数独立推导 Git 基准。

- [ ] **Step 1：增加 XML 真实文件 fixture。**

```xml
<mapper namespace="demo.OrderMapper">
  <sql id="tenantFilter">tenant_id = #{tenantId}</sql>
  <update id="updateStatus">
    UPDATE orders SET status = #{status}
    WHERE id = #{id} AND <include refid="tenantFilter"/>
  </update>
</mapper>
```

Mapper.updateStatus 使用显式 @Param("tenantId")/@Param("status")/@Param("id")。断言产生 Mapper 方法→statement、statement→sql fragment 两类关系；不能把 XML 节点用不存在的 Java 方法表示。

- [ ] **Step 2：补删除 tenant 条件、改参数名、合法动态 SQL、跨 namespace include、include 循环、databaseId 未知用例。** 预期分别为可供规则比较的关系/前后证据、契约差异、无自动漏洞判断、关联或明确缺失、SQL_INCLUDE_CYCLE、条件未知。
- [ ] **Step 3：运行 `go test ./internal/reviewcontext -run Test170MyBatis -count=1`，确认失败后实现。**
- [ ] **Step 4：用 encoding/xml 解析 namespace、statement、sql/include、参数与结果属性。** 保留原始行范围，禁止外部实体/DTD 联网；XML 异常作为当前资源解析错误，不尝试自动安装 parser。
- [ ] **Step 5：关联现有规则证据。** MYBATIS-SQL/ISOLATION 使用前后语句；BIND 需要可控性依据；CONTRACT 使用显式参数/result mapping。动态分支保留候选，不将模板直接视为已执行 SQL。
- [ ] **Step 6：复用既有 Mapper 查找入口并运行 `go test ./internal/reviewcontext ./internal/chain ./internal/reviewrules -count=1`，提交。**

**独立交付：** Mapper/XML 关联能被 Runtime 获取，规则 catalog 如需增加 RequiredEvidence，必须等 T8 evidence schema 同步后才启用新 kind，避免中间提交打断现有 Review。建议 commit：`feat(review): resolve MyBatis statement context`。
## Task 4: 有限 Dubbo 边界

**对应需求：** F4。

**Files**

- Create: `.code-harness/tools-runtime/internal/reviewcontext/dubbo_170.go`
- Create: `.code-harness/tools-runtime/internal/reviewcontext/dubbo_170_test.go`
- Modify: `.code-harness/tools-runtime/internal/workspace/maven.go`（仅确有可复用 identity 查询需提取时；不放宽原验证）
- Modify: `.code-harness/tools-runtime/internal/chain/project_discover_163.go`

**Interfaces:** 定义 `ProviderRoot170 struct { Workspace string; Root string }` 和 `ResolveDubbo170(ctx context.Context, currentRoot string, consumer nav.ReviewRef170, providers []ProviderRoot170) ([]nav.Relation170, []nav.Issue170, error)`。providers 仅由现有 workspace 验证结果构造；不能由 Agent request 提供。T5 Resolver.Dubbo 使用该函数。

- [ ] **Step 1：创建 Consumer/Provider fixture。**

```java
package demo;
import org.apache.dubbo.config.annotation.DubboReference;
class OrderService {
    @DubboReference(group="risk", version="1.0")
    RiskService risk;
}
```

Provider 用明确 DubboService import、相同 interface/group/version。匹配结果为 DUBBO_CONTRACT，不是“运行时调用已发生”。

- [ ] **Step 2：增加反例。** group 不同、version 不同、属性占位符无法解析、多个 Provider、普通同名注解、Provider 缺失、Provider workspace 未验证；禁止第一个候选获选或扫描未声明 sibling。
- [ ] **Step 3：运行 `go test ./internal/reviewcontext -run Test170Dubbo -count=1`，确认失败。**
- [ ] **Step 4：解析注解身份、服务字段和可验证本地配置。** 没有明确默认语义时保留未知；历史注解按 import 判断。不访问注册中心、不运行 Maven 下载、不发起 RPC。
- [ ] **Step 5：提供缺口原因及停止边界。** 缺 Provider 给 PROVIDER_SOURCE_UNAVAILABLE；方法只存在于其他不匹配版本不能继续链。
- [ ] **Step 6：运行 `go test ./internal/reviewcontext ./internal/workspace -count=1`，确认依赖身份隔离原回归未退化，提交。**

**独立交付：** 本地可验证的 Dubbo 契约关联和明确缺口。建议 commit：`feat(review): identify bounded Dubbo contract context`。
## Task 5: 按需上下文编排、预算及认证接入

**对应需求：** F5，连接 F1/F3/F4。此任务是主接线，不新建快照能力。

**Files**

- Create: `.code-harness/tools-runtime/internal/reviewcontext/build_170.go`
- Create: `.code-harness/tools-runtime/internal/reviewcontext/build_170_test.go`
- Create: `.code-harness/tools-runtime/internal/reviewcontext/verify_170.go`
- Create: `.code-harness/tools-runtime/cmd/codea-dcep-tools/review_context_command_170.go`
- Create: `.code-harness/tools-runtime/cmd/codea-dcep-tools/review_context_command_170_test.go`
- Create: `.code-harness/tools-runtime/internal/analysis/review_relations_170.go`
- Create: `.code-harness/tools-runtime/internal/analysis/review_relations_170_test.go`
- Modify: `.code-harness/tools-runtime/cmd/codea-dcep-tools/review_precision_command.go`
- Modify: `.code-harness/tools-runtime/internal/analysis/evidence.go`
- Modify: `.code-harness/tools-runtime/internal/analysis/certify_canonical_162.go`
- Modify: `.code-harness/tools-runtime/internal/reviewunit/build.go`
- Modify: `.code-harness/tools-runtime/internal/reviewrules/dispatch.go`
- Modify: `.code-harness/skills/analyze-change/SKILL.md`
- Modify: `.code-harness/agents/reviewer.md`
- Modify: `.code-harness/agents/orchestrator.md`
- Modify: `.code-harness/AGENTS.md`
- Modify: `.code-harness/tools/README.md`

**Interfaces:** 实现 Build170/VerifyRelations170 及 `runReviewContext170(args []string) error`；接入 runReview160 的 context 分支。Runtime 构造 BuildInput170，严禁 agent-supplied seeds/budget/roots。

- [ ] **Step 1：写入 request 拒绝及来源派生测试。** 合法请求为 {"runId":"review-case","phase":"DISCOVERY"}。额外 roots、files、baseRef、snapshot 字段全部拒绝；body/path runId 不一致拒绝；RULES 在认证、units、dispatch 未就绪时拒绝。
- [ ] **Step 2：写 Build170 的具体场景断言。**

| 测试名 | 输入和必须断言 |
|---|---|
| Test170ParallelCallsStaySeparate | A.submit 分别调用 Risk.check、Repo.save；不存在 Risk→Repo |
| Test170BudgetReportsBlocked | MaxFiles=1，规则需要两个不同文件；该规则 BLOCKED，其他不依赖第二文件的规则可 READY |
| Test170UntrackedSeedIncluded | 新 Service 在同 run canonical untracked 集中；不能因 git grep 不含它而遗漏 |
| Test170DeletedMethodBeforeContext | 方法从 CURRENT 删除；BASE 可作比较，不能作为 CURRENT target |
| Test170ContextCannotExpandScope | 依赖文件进入 Relations/Evidence，但不进入 Unit.Files、ChangedHunks 或 selection |
| Test170ForgedRelationRejected | 改 relation target 后仍保持真实行号；VerifyRelations170 必须重验后拒绝 |

- [ ] **Step 3：运行 `go test ./internal/reviewcontext ./internal/analysis ./cmd/codea-dcep-tools -run Test170 -count=1`，确认新增场景失败。**
- [ ] **Step 4：实现 Runtime 输入构造与源码适配。** DISCOVERY 取现有 ChangeSet 的 exact paths/hunks；BASE 复用既有本地 base 提取/批量 AST；RULES 取现有已认证 units/dispatch。current 与 verified dependency Navigator 分开映射。没有 base 源码只给限制，不创建新 SnapshotReader 或 artifact。
- [ ] **Step 5：实现固定预算和普通工作队列。** 每个关系按真实 caller 保留；visited 防递归；向上搜索按 scope 文件候选过滤。每阶段固定 40/200/1MiB/15秒，超限停止探索并附 issue。必读 changed files 不受额外上下文上限豁免。实际进程调用走已有 Runner，无 shell 字符串求值。
- [ ] **Step 6：按规则构造 Needs。** 从现有 rule role 和 required evidence 映射所需关系，不假设拿到任意链就所有规则 READY。Spring TX 需要 caller/callee 和可验证代理关系；MyBatis 使用语句；Dubbo 缺源码只阻断实际需要 Provider 代码的规则。
- [ ] **Step 7：接入现有认证。** Agent 可以从运行记录引用关系，但 analysis/finding 不能直接信任磁盘 JSON。VerifyRelations170 对被用到的关系调用同一解析器复验并检查种子/roots/规则归属。既有 snapshot/certification 公共签名和语义不改；把额外关系校验放在 analysis/evidence 扩展点，不能用它代替既有认证。
- [ ] **Step 8：处理旧投影。** 只有可无歧义表示的连续关系进入旧 callChains/ChainRefs；同路径重载等留在新 relation records。未修改现有 ReviewUnit.Files 构造的允许范围。上下文需求在 RULES 中由 unitId 引用，无需把 dependency 文件塞进 Files。
- [ ] **Step 9：同步内部命令白名单、request schema 使用说明、两阶段调用顺序。** 明确本次结果不能跨 run 复用，Agent 不写 analysis/**。新步骤仅 review 启用，其他入口不默认调用。
- [ ] **Step 10：运行 `go test ./internal/nav ./internal/reviewcontext ./internal/analysis ./internal/reviewunit ./internal/reviewrules ./cmd/codea-dcep-tools -count=1`，修正受影响的精确协议断言后提交。**

**独立交付：** 已有 Review 可取得按需上下文；未知关系保留边界。建议 commit：`feat(review): gather bounded context from current review inputs`。

### T5 补充：保持独立 Reviewer 和八阶段权威

**Files**

- Modify: `.code-harness/tools-runtime/internal/reviewprogress/progress.go`
- Modify: `.code-harness/tools-runtime/cmd/codea-dcep-tools/review_progress_command.go`
- Create: `.code-harness/tools-runtime/cmd/codea-dcep-tools/review_context_progress_170_test.go`

- [ ] **Step 11：先加阶段时序反例。** 旧dispatch完成即推进REVIEW_EXECUTION时，knowledge/RULES尚未绑定，必须拒绝FINDINGS；尝试在失败/终态run再次context必须拒绝。
- [ ] **Step 12：实现规划内最终化。** Unit→knowledge→一次发布含knowledge摘要的dispatch→RULES上下文→Runtime内部推进规划。1.7的dispatch不立即推进；RULES成功且权威产物齐备后才推进。READY/BLOCKED存上下文，不反写不可变dispatch，不新增finalize/advance产品命令；旧run保留旧流程。
- [ ] **Step 13：保持Host分工。** Main仅调用Runtime和委派独立Reviewer；所有语义proposal经codea-reviewer-submit，所有状态展示取Runtime事件。Reviewer无法完成即原failure-only信号与硬停止，不能因为新上下文可用而Main接管。
- [ ] **Step 14：运行阶段回归。** `go test ./internal/reviewprogress ./internal/reviewauthority ./cmd/codea-dcep-tools -run 'Test170|Test.*Progress|Test.*Reviewer' -count=1`；确认八个阶段及不可跳阶段/不可主Agent伪造的原断言仍成立。T7尚未接入前，仅完成代码路径接线并保持业务知识关闭，T7接入后复测最终规划绑定。
## Task 6: Review 自动重发现临时 Chain

**对应需求：** F2。

**Files**

- Create: `.code-harness/tools-runtime/internal/reviewscope/chain_auto_context_170_test.go`
- Create: `.code-harness/tools-runtime/cmd/codea-dcep-tools/chain_auto_context_170_test.go`
- Modify: `.code-harness/tools-runtime/internal/reviewscope/chain_context.go`
- Modify: `.code-harness/tools-runtime/cmd/codea-dcep-tools/chain_review_context.go`
- Modify: `.code-harness/skills/review-code/SKILL.md`
- Modify: `.code-harness/agents/orchestrator.md`
- Modify: `.code-harness/AGENTS.md`
- Modify: `README.md`

**Interfaces:** 正常 Review 命令层调用 ResolveChainContexts 时采用 AUTO_TEMPORARY；保留显式 Chain 管理 API 和既有 persist 授权。AllowTemporaryForStale 保留兼容读取，但不能由 Agent 决定正常 Review 是否自动更新。

- [ ] **Step 1：基于已有 chain_context_test 创建 stale fixture。** 保存旧链 A→B，当前已认证链为 A→C，输入正常 Review；断言使用 C、状态 TEMPORARY、无 STALE_REQUIRES_DECISION。
- [ ] **Step 2：加入文件不变测试。** Review 前读取 `.code-harness/chains/order.yaml` bytes，运行后再次读取并 bytes.Equal；名称/备注不丢失，手工旧链不被覆盖。保存文件损坏时，从当前分析恢复临时链并给提示，不信任损坏内容。
- [ ] **Step 3：加入边界回归。** 入口删除只保留删除说明；方法重载不唯一不能切换目标；plain Review/多上游Service仍按现有RuntimeOptions选择；显式Controller/方法保持direct TARGETED并覆盖机器要求分支，不额外弹菜单。选择第一条不能因刷新变成全部。
- [ ] **Step 4：运行 `go test ./internal/reviewscope ./cmd/codea-dcep-tools -run Test170AutoChain -count=1`，记录旧 stale 流程失败。**
- [ ] **Step 5：实现 Review 专用默认策略。** 可调用当前已有临时 discovery/certification 回调；解析事实来自 T5/current certified analysis，不从旧 YAML 推导真值。删除维护确认分支的 Review 使用点，不删除显式 chain persist 的授权校验。
- [ ] **Step 6：更新 Agent/Skill 文案。** 不再指示用户先 refresh；仍禁止写 Project State 和伪造 human selection。确实无法解析的部分说明原因，不请求用户凭猜测指定实现。
- [ ] **Step 7：运行 `go test ./internal/reviewscope ./internal/chain ./cmd/codea-dcep-tools -count=1`，重点检查保存授权、USER_SELECTION 和旧显式管理入口，提交。**

**独立交付：** 普通 Review 的旧链过期无需人工维护，不改变项目 Chain 文件。建议 commit：`feat(review): refresh temporary chain context automatically`。
## Task 7: 明确绑定的业务知识读取

**对应需求：** F7；依赖 T1。与 T2–T4 可并行，正式命令接线依赖 T5。

**Files**

- Create: `.code-harness/tools-runtime/internal/knowledge/config_170.go`
- Create: `.code-harness/tools-runtime/internal/knowledge/load_170.go`
- Create: `.code-harness/tools-runtime/internal/knowledge/verify_170.go`
- Create: `.code-harness/tools-runtime/internal/knowledge/knowledge_170_test.go`
- Create: `.code-harness/tools-runtime/cmd/codea-dcep-tools/review_knowledge_command_170.go`
- Create: `.code-harness/tools-runtime/cmd/codea-dcep-tools/review_knowledge_command_170_test.go`
- Modify: `.code-harness/tools-runtime/cmd/codea-dcep-tools/review_precision_command.go`
- Modify: `.code-harness/tools-runtime/internal/reviewrules/dispatch.go`
- Modify: `.code-harness/tools-runtime/internal/reviewrules/model.go`
- Modify: `.code-harness/contracts/rule-dispatch.schema.json`
- Modify: `.code-harness/tools/README.md`

**Interfaces:** 实现 ParseBinding170、ParseRule170、Applies170、Load170、Verify170；命令层 `runReviewKnowledge170(args []string) error` 只接受同 run 请求，从项目配置及已验证单位构造 LoadInput170。`knowledge` 分支只在 REVIEW_PLANNING 的正确位置可用。

- [ ] **Step 1：增加严格配置测试。** 缺 required、RULES却required=false、未知 root/kind、重复 sourceId、重复 YAML key、多个 YAML 文档、URL/绝对 source path、`../`、ADS、UNC 和空 projectId 均拒绝。核心失败样例：

```go
func Test170KnowledgeRejectsTraversal(t *testing.T) {
    raw := []byte("version: 1\nprojectId: order-service\nsources:\n" +
        "  - id: rule-a\n    root: PROJECT\n    path: ../private.md\n" +
        "    kind: RULES\n    required: true\n")
    if _, err := ParseBinding170(raw); err == nil {
        t.Fatal("traversal source accepted")
    }
}
```

- [ ] **Step 2：增加适用性和状态样例。** 显式目录前缀命中对应 unit、另一个模块不命中、显式 `./` 匹配项目全部 unit；精确入口含参数签名，裸类名不能当相同入口。草稿/退休、重复 ruleId、项目不匹配、空正文及缺批准来源不能成为 ACTIVE 规则。缺失必需文档无法获知适用性时不得静默 NOT_APPLICABLE。
- [ ] **Step 3：增加真实临时目录读取测试。** 用 `t.TempDir/os.WriteFile` 创建业务根、团队资料根和元数据；覆盖项目知识、共享参考、越界 symlink、非普通文件、最多20文档/256KiB/5经验、非UTF-8和读取中变化。只读取 source 声明的文件，不执行 source/approvalRef 中的 URL。
- [ ] **Step 4：运行失败测试。** `go test ./internal/knowledge ./cmd/codea-dcep-tools -run Test170Knowledge -count=1`，检查测试确实被执行，不能用零匹配当通过。
- [ ] **Step 5：按以下次序实现加载。**

```text
命令校验 run/阶段及已认证单位
→ 严格读取 context.yaml 或记录 ABSENT
→ 校验 source 声明、根和最终普通文件路径
→ RULES 优先，随后 required 参考，最后 optional 资料
→ 完整读取及字节摘要，解析元数据与适用性
→ 形成 BusinessCheck170 及必要缺口
→ 输出当次 Documents；磁盘只写 Manifest170 元数据
```

- [ ] **Step 6：完善重复来源及预算。** 相同最终文件可复用本次内存读取，字节/文件数按唯一文件计；不同 sourceId 声明同一正式 ruleId 仍报重复。必需规则超限或无法读取产生明确 BLOCKED；可选参考缺失不阻断不依赖它的技术规则。Load170 不写源文件、不执行同步。
- [ ] **Step 7：接入 RuleDispatch。** 从 BusinessCheck170 构造本次BUSINESS临时规则：Kind=AGENT、RuleVersion=1、默认severity=medium、需要BUSINESS_RULE和源码证据；文档version原样留在知识元数据。有效catalog摘要绑定原内置catalog和knowledgeSha256，VerifyContext使用相同规则集，不将公司规则写入spring-v1.yaml。发布不可变dispatch后由T5的RULES计算就绪状态并完成规划；保留scope校验，知识不形成新的代码Anchor。
- [ ] **Step 8：实现 Verify170 并验证内容变化。** 认证前重新读取绑定/被依赖文档，字段和摘要必须匹配；修改规则正文、改变 teamRoot、删去 source、替换同名文件都不能沿用原结论。新增测试先加载、再写不同内容、再调用 Verify170，必须返回 KNOWLEDGE_SOURCE_CHANGED 或 KNOWLEDGE_BINDING_CHANGED。
- [ ] **Step 9：运行 `go test ./internal/knowledge ./internal/reviewrules ./internal/schema ./cmd/codea-dcep-tools -count=1`。** 未配置知识的既有项目保留技术 Review；LIST/0-change 不增加知识依赖，非法 run 请求仍硬停止。提交。

**独立交付：** 明确本地来源可被读取、适用性和缺口可验证；还不能单独宣称模型已完成业务审核。建议 commit：`feat(review): load explicitly bound business knowledge`。
## Task 8: 关系证据、规则缺口及最终报告

**对应需求：** F6/F7，依赖 T5、T6、T7。

**Files**

- Create: `.code-harness/tools-runtime/internal/finding/context_evidence_170.go`
- Create: `.code-harness/tools-runtime/internal/finding/context_evidence_170_test.go`
- Create: `.code-harness/tools-runtime/internal/report/review_context_170.go`
- Create: `.code-harness/tools-runtime/internal/report/review_context_170_test.go`
- Modify: `.code-harness/tools-runtime/internal/finding/model.go`
- Modify: `.code-harness/tools-runtime/internal/finding/evidence.go`
- Modify: `.code-harness/tools-runtime/internal/finding/verify.go`
- Modify: `.code-harness/tools-runtime/internal/finding/certify.go`
- Modify: `.code-harness/tools-runtime/internal/finding/certificate.go`
- Modify: `.code-harness/tools-runtime/internal/finding/dedup.go`
- Modify: `.code-harness/tools-runtime/internal/report/review.go`
- Modify: `.code-harness/contracts/finding-proposals.schema.json`
- Modify: `.code-harness/contracts/certified-findings.schema.json`
- Modify: `.code-harness/contracts/report-review-request.schema.json`
- Modify: `.code-harness/review-rules/spring-v1.yaml`
- Modify: `.code-harness/agents/reviewer.md`
- Modify: `.code-harness/skills/review-code/SKILL.md`
- Modify: `.code-harness/AGENTS.md`

**Interfaces:** EvidenceRef 增加可选 RelationID/Workspace/SourceSide，对应 relationId/workspace/sourceSide。CONTEXT_RELATION 必填 relationId；BASE 只用于前后比较，不能证明当前调用存在。CertifiedSet 增加可选 ReviewContext 指针，结构固定：

```go
type BlockedCheck170 struct {
    ReviewUnitID string `json:"reviewUnitId"`
    RuleID string `json:"ruleId"`
    Reasons []string `json:"reasons"`
}
type ReviewContextSummary170 struct {
    Status string `json:"status"` // COMPLETE | PARTIAL
    BlockedChecks []BlockedCheck170 `json:"blockedChecks"`
}
// CertifiedSet: ReviewContext *ReviewContextSummary170
// JSON 名为 reviewContext，旧 run 可缺省；1.7 新 run 必须写。
```

此 summary 由 Runtime 从复验过的 RULES context 计算，加入既有 CertifiedSet hash/certificate 覆盖，不新增 certificate 文件。缺字段或跨 run 伪造时，新 run 不能默认 COMPLETE。

- [ ] **Step 1：新增上下文证据攻击及合法样例。** 合法 current Mapper/XML 关系通过；VERIFIED dependency 只读关系可作证据但外部 Anchor 拒绝；伪造 relationId、改 target、改 workspace、BASE 冒充 CURRENT、另一个 unit 的 relation 全部拒绝。
- [ ] **Step 2：增加精确签名去重测试。** 同一位置不同根因不能因裸方法名而合并；同一关系同一问题的不同措辞按现有规则去重。不能直接把 relationId 放入所有旧 finding ID 导致已有去重全面改变。
- [ ] **Step 3：增加报告状态测试。** 一项 BLOCKED + 一个已认证 finding ⇒ MANUAL_ACTION_REQUIRED，保留该已认证问题；BLOCKED + 零 finding ⇒ 未完成；全部就绪且宿主正常完成 + 零 finding ⇒ 未发现问题。宿主超时/工具失败仍走既有 RuntimeErrors，不能因为 READY 就推导检查完成。
- [ ] **Step 4：运行 `go test ./internal/finding ./internal/report -run Test170 -count=1`，确认失败。**
- [ ] **Step 5：实现 relation 复验及 Summary。** 复用 T5 的 VerifyRelations170，执行现有 schema/anchor/unit/dispatch 校验；只新增受控 evidence kind，不解除原有 dependency 路径门禁。保持 proposals 顶层数组，不能擅自变成 envelope 破坏老调用。
- [ ] **Step 6：同步 strict schema 和版本兼容。** 旧 run summary 缺省沿旧分支；1.7 新 run 从 Runtime 实际版本要求 summary，不能让 Agent 传 harnessVersion 绕过。旧 findings/evidence 类型继续可读。
- [ ] **Step 7：修改 WriteCertifiedReport 与 writeCertifiedSetReport160。** 原代码会把有 findings 强制 FAILED、无 findings 强制 PASSED；现在先判断 Runtime summary/宿主失败。已有硬认证失败保持无正式 finding 的安全停止；只有 CertifiedSet 真实加载成功才可在“部分规则未完成”报告中保留 findings。禁止 PARTIAL transport 变成原始 Agent finding 通道。
- [ ] **Step 8：调整高价值规则的证据要求和 Reviewer 反证步骤。** 事务规则不能只依赖符号存在；MyBatis 条件变化必须引用前后语句；高风险候选在宿主同次审查内核对保护逻辑与输入，不新建第二模型引擎。
- [ ] **Step 9：实现用户输出。** 主体只列待处理问题；未完成列具体缺口；结尾一段研发转述。0-change 继续走既有正式报告流程。完整导航与技术字段留 run 记录。
- [ ] **Step 10：运行 `go test ./internal/finding ./internal/report ./internal/schema ./internal/reviewrules ./cmd/codea-dcep-tools -count=1`，保留现有 tamper/scope/dedup/zero-change 回归，提交。**

**独立交付：** 有关系依据的 finding、不会冒充通过的缺口报告。建议 commit：`feat(review): verify context evidence and report incomplete checks`。
### T8 补充：业务证据与检查完成声明

本节与 Task 8 的关系证据工作一起验收，不能作为可省略的后续优化。

**Files**

- Create: `.code-harness/tools-runtime/internal/finding/business_evidence_170_test.go`
- Create: `.code-harness/tools-runtime/internal/reviewauthority/checks_170_test.go`
- Modify: `.code-harness/tools/codea-reviewer-submit.ts`
- Modify: `.code-harness/tools-runtime/internal/reviewauthority/authority.go`
- Modify: `.code-harness/tools-runtime/cmd/codea-dcep-tools/review_precision_command.go`
- Modify: `.code-harness/tools-runtime/cmd/codea-dcep-tools/report_certified_findings_164.go`
- Modify: `.code-harness/contracts/certified-findings-cert.schema.json`
- Modify: `.code-harness/contracts/reviewer-host-contract.md`

**Interfaces:** EvidenceRef 添加 `SourceID/RuleID/SourceSHA256`，JSON 为 sourceId/ruleId/sourceSha256；BUSINESS_RULE 必填三者，其他 kind 不接受无关字段。业务 Finding 使用已有 PRODUCTION_CODE 类别，规则身份为 BUSINESS 命名空间；不给知识文档创建代码 Anchor。

```go
// internal/finding/checks_170.go（T1 声明，T8 实现校验）
type CheckResult170 struct {
    ReviewUnitID string `json:"reviewUnitId"`
    RuleID string `json:"ruleId"`
    Status string `json:"status"` // COMPLETED | INCOMPLETE
    Reason string `json:"reason"`
    SourceIDs []string `json:"sourceIds"`
}
```

- [ ] **Step 11：增加检查完成声明反例。** READY技术/业务项的零条记录、重复、另一个unit、未分发rule、使用未读source均不能产生完整结论；合法 INCOMPLETE 产生未完成。Runtime BLOCKED项不因checks填写COMPLETED而恢复。0-change/LIST仍走原特殊流程。
- [ ] **Step 12：扩展原提交工具。** kind=findings 增加 checks 字符串参数，校验JSON数组，写固定同run `review-checks.json`；finding-proposals仍为数组。最后发布v2 receipt，绑定两份文件的SHA256；kind=change-analysis保留v1语义。不得由Main直接写checks后伪造receipt。
- [ ] **Step 13：扩展Host验证。** 以可信run版本选择v1/v2；校验独立会话、Reviewer消息、completed submit工具调用与两个实际输入的JSON一致性。伪造checks哈希、拿另一次调用的checks、v1降级、缺文件、仅一份写成功必须失败；维持reviewer-unavailable现有硬停止。
- [ ] **Step 14：验证业务证据。** 规则违反必须匹配适用Unit、已分发BUSINESS key、source/rule ID、实际内容摘要和代码证据。语义冲突、未获批准的规则变化写入有效Reviewer的INCOMPLETE reason；不伪造错误代码Finding。
- [ ] **Step 15：扩展既有证书。** CertifiedSet和Certificate的1.7形式都记录 knowledgeSha256/reviewChecksSha256；保持旧run字段缺省和旧规范序列化。重新加载CertifiedSet时比较上下游一致性；首次发布前复验绑定和被引用知识。改配置/删除来源/篡改manifest或checks均不能出正式结论。
- [ ] **Step 16：计算最终汇总。** Runtime综合代码上下文BLOCKED、知识不可用、checks INCOMPLETE得到reviewContext PARTIAL；全部已分发READY项有合法COMPLETED且Host完成才可能COMPLETE。NOT_CONFIGURED仅允许技术范围结论并声明业务未启用。正式报告和用户摘要都只列问题/缺口，仍从CertifiedSet渲染。
- [ ] **Step 17：运行并提交。** `go test ./internal/knowledge ./internal/finding ./internal/reviewauthority ./internal/report ./internal/schema ./cmd/codea-dcep-tools -run Test170 -count=1` 后执行受影响包全量回归。实际Host工具参数解析/会话导出兼容纳入T9/T10，不能用手写receipt替代。

必须核对的新增错误路径：缺少有效检查完成声明返回 REVIEW_CHECKS_INCOMPLETE；业务source引用错配返回 KNOWLEDGE_REFERENCE_INVALID；资料变化返回 KNOWLEDGE_SOURCE_CHANGED/BINDING_CHANGED。Host证据不成立仍按现有REVIEWER_UNAVAILABLE路径处理，不用这些错误码绕过Host硬停止。
## Task 9: TeamAI 接入与原安装升级兼容

**对应需求：** F8；依赖 T1、T7；可先准备文档，实际协议联调依赖 T8。

本任务跨两个仓库，必须分别提交。TeamAI 现有目录不重排；不创建新的知识库、同步工具、安装器或升级框架。

**Files — 核心仓库**

- Modify: `.code-harness/bootstrap.md`
- Modify: `.code-harness/contracts/reviewer-host-contract.md`
- Modify: `.code-harness/tools-runtime/internal/upgrade/inventory.go`
- Modify: `.code-harness/tools-runtime/internal/upgrade/upgrade.go`
- Modify: `.code-harness/tools-runtime/internal/upgrade/reviewer_host_164.go`
- Create: `.code-harness/tools-runtime/internal/upgrade/review_170_compat_test.go`
- Create: `.github/scripts/task170-release-package.ps1`
- Modify: `.github/workflows/package-windows-x64.yml`
- Modify: `.code-harness/VERSION`（只在完整版本集成时更新到 `1.7.0`）
- Modify: `README.md`

新增 task170-release-package.ps1 是现有打包流程的 1.7 版本适配入口，沿用已验收的 builder、完整包结构和检查；不要改写冻结的 1.6.4 release 脚本或解除其版本/源文件校验。已有 Workflow 增加 1.7 路由和相应必要测试，保留原版本分支；不新增发布平台或独立“保障项目”。

**Files — `lingyi9909/teamai-harness`**

- Modify: `skills/codea-harness-review/SKILL.md`
- Modify: `rules/codea-harness-collaboration.md`
- Modify: `docs/codea-harness/copy-and-use.md`
- Modify: `docs/codea-harness/ownership-and-knowledge.md`
- Modify: `templates/codea-harness/business-rule.md`（补齐1.7正式规则元数据格式，保留草稿标识）
- Modify: `templates/codea-harness/project-knowledge.md`（说明实际来源根与相对路径，保持可选模板）
- Modify: `README.md`

这些路径是已存在的接入资源。实际内网仍沿用其原布局；不要把公共示例的 `docs/codea-knowledge`、templates 或某个 packages 目录变为必需结构。完整包放到已有内网制品位置即可。

**Interfaces:** TeamAI 入口只调用项目已安装 bootstrap/Orchestrator；核心安装和 upgrade 入口不变。Git 更新文件、TeamAI pull 同步资源、Harness 升级更新项目安装三者分开；context.yaml 是应保留的可选 Project State。

- [ ] **Step 1：补升级失败样例。** 构造1.6.4已安装Host和1.7目标Host不同的升级场景，断言三个受管Host文件得到对应版本；现有 `crossesReviewerHostBoundary` 只跨入1.6.4，必须先记录它不能覆盖该场景的失败。
- [ ] **Step 2：增加保留和冲突场景。** 升级前后 context.yaml、已有知识、harness.yaml、project.md、database.yaml、runs/chains 字节一致；只替换可证明属于旧正式包的Host文件。用户不同内容、缺少Host清单、错误版本或部分失败按既有流程保护/回滚，不能用删除冲突文件过关。
- [ ] **Step 3：运行 `go test ./internal/upgrade -run Test170 -count=1`，确认上述新增断言失败后实现。** 将新 Framework 文件加入既有清单；context.yaml 放入 Project State 保留集合；在原Host事务中登记1.7更新边界，不另写第二套文件复制器。
- [ ] **Step 4：适配现有正式包。** task170 复用已批准 builder；安装/升级包包含1.7 Runtime、core skills、schema、Reviewer及提交工具，保留安装器原入口。基线OpenCode版本若需变化必须逐项申请并验收，不能暗中更新package依赖。正式包不能携带填写好的公司 context.yaml。
- [ ] **Step 5：更新入口和资料。** 同一业务项目运行1.6.4入口时只用原契约，不能创建1.7知识配置；正式1.7安装后按新合同工作。保留用户指定目标，Reviewer失败不接管，不在Review中执行git/teamai同步或升级。
- [ ] **Step 6：验证 TeamAI 实际行为。** 在公司已有总仓的测试副本中只加对应 skills/rules/接入说明，保留原 teamai.yaml、角色/项目筛选和知识路径。用批准版本运行 `teamai --version`、`teamai init --help`、已有接入命令、`teamai pull`、`teamai status`、`teamai doctor`；确认入口可发现且规则有效。不得连接外网仓库作为源。
- [ ] **Step 7：验证三个更新动作的边界。** Git更新总仓不更改已安装业务项目；TeamAI pull 只同步其资源，不更改三个核心Host文件/Runtime；Harness升级不改团队资料。记录前后指定文件摘要。两种部署触碰同一 `.opencode` 根不代表共同拥有同名文件。
- [ ] **Step 8：验证完整包安装。** 在Windows使用已批准 `pwsh` 运行原 `install.ps1 -ProjectRoot` 入口；不把 `powershell.exe` 当作等价前提。缺组件时一次说明全部缺项，不自动安装依赖。团队总仓不作为被审核业务根。
- [ ] **Step 9：运行相关Go/Host/安装验证并分别提交两个仓库。** 新包验证复用原流程，失败按日志修复；无实际内网条件时保留Step6–8未执行，T10最终接续，不把文档审查当联调通过。

**独立交付：** 原有目录、同步、安装、升级方式保持可用；1.7仅增加必要的版本内容与协作接入。建议 commits：核心 `feat(review): integrate 1.7 resources with existing delivery`；TeamAI `docs(review): align entry and intranet ownership with 1.7`。
## Task 10: 随功能完成的真实 Review 与内网验收

**对应需求：** 八项功能的必要测试，不建设独立平台或发布专项。

**Files**

- Create: `.code-harness/tools-runtime/testdata/review-170/README.md`
- Create: `docs/superpowers/evidence/2026-09-11-codea-harness-1.7-integrated-validation.md`（执行时记录实际日期、版本和输出；没有结果不得写通过）
- Modify: `README.md`
- Modify: `.code-harness/tools/README.md`

**Interfaces:** 不新增模型 API 客户端。真实 Review 使用现有 OpenCode 宿主及获准内网模型；模型与工具调用记录取现有 run 输出。

- [ ] **Step 1：整理至少 12 个独立变更样例。** 6 个缺陷：事务自调用变更、Qualifier 目标错配、Mapper 参数不匹配、租户条件删除、公共 SQL 条件弱化、Dubbo 契约版本不匹配。6 个反例：正确跨 Bean 调用、正确 Qualifier、合法动态 SQL、正确参数映射、匹配 Dubbo、无风险命名变更。设计需要配置才能成立的缺陷时，将该配置作为输入材料提供，不靠文件名判定真值。
- [ ] **Step 2：把 fixture source 与评判答案分开。** 模型能读取测试仓库代码和 diff，不能读取 expected 结论、手写 proposals 或本计划中的答案表。临时仓库只复制源文件，不复制评判说明。原 24-case 不作为模型检出率统计。
- [ ] **Step 3：用相同内网模型/规则预算运行基线与 1.7。** 记录 baseline commit、candidate commit、模型名称和版本、输入 diff、实际问题、漏报、误报、总耗时。人工核对，每个失败给实际输出；小样本不宣称统计精度提升。
- [ ] **Step 4：在公司 Windows 禁公网环境执行正常 Review。** 沿用已有安装方式；验证新链自动更新、无 refresh 确认、项目 YAML 不变、无在线依赖下载。读取允许的内网模型地址不等于放开外网。缺新依赖时先记录申请，不自动补包。
- [ ] **Step 5：执行一次最终代码回归。**

```powershell
go test ./...
go vet ./...
git diff --check
git diff -- go.mod go.sum
```

工作目录仍为 tools-runtime，最后一条默认应无新增依赖差异。沿用仓库现有受影响的宿主协议测试；只执行T9在既有流程中的1.7版本适配，不新建独立发布认证/升级回滚体系。既有打包流水线若自动运行不得禁用。

- [ ] **Step 6：逐条记录验收。** 关系误判、scope 扩张、无依据 finding、未完成被报通过、运行时下载任一出现均需修复后复测；其他质量差异按实际问题解释。未执行的 Windows/真实模型检查保持“未执行”，不能用 mock 输出覆盖。
- [ ] **Step 7：更新用户说明并提交结果。** README 写清正常 Review 自动临时 Chain、支持/不支持模式及离线依赖；不提供 index/graph/snapshot 新命令。建议 commit：`test(review): validate call context in real offline review flows`。

### T10 补充：知识、协同与最终交接

- [ ] **Step 8：运行6个业务知识真实模型案例。** 规则违反、符合规则、必需规则缺失、自然语言冲突、资料变化导致旧引用失效、草稿规则不能豁免。答案不出现在模型工作区；固定来源与规则版本，不把机器缺口当模型检出能力。
- [ ] **Step 9：完成T9的真实Windows/TeamAI/完整包安装升级联调。** 未执行项必须真实补齐。采用已有批准版本，记录TeamAI/OpenCode/PowerShell/Runtime/模型版本及实际Git SHA，确认无公网访问和自动下载。
- [ ] **Step 10：检查相关Workflow直到终态。** 失败时查日志、定位、最小修复、提交、重跑并再查；无法获得内网/模型凭据、环境不可用或批准依赖缺失等外部阻塞要说明具体项目和责任人，不将其写为通过。
- [ ] **Step 11：填写交接记录并移交。** 记录基线SHA、实现SHA、核心/TeamAI提交、正式包标识、A1–A12结果、18个实际模型案例及误报/漏报/耗时。全部必要项完成后才标记1.7可交付；本文中的未勾选任务不自动变为已实现。

每项模型失败须记录实测差异并修复/复测。初始验收要求：明确缺陷有有效证据，正确反例不产生错误正式Finding，缺口场景准确未完成。18个案例不足以声称全业务统计精度。

## 需求覆盖与交付检查

- [ ] F1→T1/T2；F2→T6；F3→T3；F4→T4；F5→T5；F6→T8；F7→T1/T7/T8；F8→T9。T10覆盖全部A1–A12。
- [ ] 不迁移TeamAI原有目录，不引入第二套知识维护、同步或安装升级机制。
- [ ] 不新增索引/图/LSP/快照，不扩展日志平台、登录、诊断或修复功能。
- [ ] 不覆盖保存链、知识及项目状态，不放宽范围/依赖/独立Reviewer/Host和证据门禁。
- [ ] 新接口、strict schema、版本、工具输入、证书摘要与renderer一致；旧run序列化未被新字段破坏。
- [ ] 相关Workflow及真实内网/Windows/模型验收已有实际证据；未完成项未被标为通过。

研发交接记录至少包含：实施基线、两个仓库的提交、已完成任务、失败及未执行项、正式包、验收结果、剩余外部阻塞与负责人。执行计划仍以核心仓库为唯一维护源；内网离线副本与同提交设计一并导入，不独立演变。
