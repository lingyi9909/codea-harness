# Codea Harness 1.7 Review Call Context Implementation Plan

> 2026-09-11：本文已由 [1.7完整项目执行计划](2026-09-11-codea-harness-1.7-integrated-plan.md) 取代。新计划完整覆盖代码Review、业务知识与内网/TeamAI协同，原八任务不能作为全部开发范围。

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在无新增索引、图查询和快照功能的前提下，使 Code Review 自动更新相关调用链、获取业务上下文并输出有依据的问题。

**Architecture:** 增强现有 AST/nav 与 Chain，使用普通运行内关系记录。DISCOVERY 从现有 ChangeSet 派生种子，RULES 从现有 ReviewUnit/RuleDispatch 派生上下文需求；正式结论继续经过既有 Runtime 认证和报告入口。Review 自动重发现临时 Chain，不写长期项目 Chain。

**Tech Stack:** 现有 Go 1.23.10、Git、随包 ast-grep、Go 标准库及当前 go.mod；默认零新增第三方模块。

**Spec:** [1.7 设计](../specs/2026-09-09-codea-harness-1.7-review-call-context-design.md)

**Baseline:** `2467e13c493c3a997d2c719955437bff8b9c3cd4`。本文为待执行计划，所有复选框保持未执行；本次文档提交不包含实现或功能验收。

## Global Constraints

- 只建设 Code Review 的六项功能，不扩展 Test、Debug、Fix、API Doc 产品能力。
- 不新增本地持久索引、跨运行解析缓存、图数据库、图查询、建图命令或后台监听。
- 不新增快照、SnapshotReader、业务上下文快照、双图 overlay 或新的快照认证体系；现有 1.6.4 ChangeSet / certification / freshness 机制保持原状。
- 不整套集成 OpenCodeReview、Code Review Graph、JDT LS、JDT Core、JavaParser、Embedding、向量数据库或知识库。
- 调用关系在本次 Review 自动分析和更新；人工只可选补充业务说明及缺失材料。
- 正常 Review 不写 .code-harness/chains/**；自动更新的是本次运行的链路结果。
- 公司内网不能访问外网；默认不增加第三方依赖，不自动安装包、下载模型资源或 git fetch/pull。
- 只读上下文不得自动扩大 Finding Scope、Write Scope 或用户已经选定的 FULL / TARGETED 范围。
- 不另建评测平台、发布系统或升级回滚专项；必要测试随功能实施，复用现有交付流程。

---

## 执行安排

任务依赖：

```text
T1 运行内关系契约
  ├─ T2 Java/Spring
  ├─ T3 MyBatis
  └─ T4 Dubbo（需 T2 的类型身份；不依赖 MyBatis）
T2 + T3 + T4 → T5 上下文编排与认证接入
T5 → T6 Review 自动临时 Chain
T5 + T6 → T7 Finding/报告
T7 → T8 实际 Review 与内网验收
```

T2/T3 可在 T1 后分开实施；共享契约改动必须先回到 T1 更新，不能各自发明字段。每个任务独立提交和审核。仅针对当前任务运行测试，最终执行一次受影响包与全量 Go 回归。没有公司模型或 Windows 时，T8 对应项保留未完成，禁止伪造结果。

每项计划中的 Create 路径均为拟新增；Modify 路径已在基线树核对。不要因为新文件尚不存在就改用相似旧文件承担所有责任。

命令工作目录统一为仓库 `.code-harness/tools-runtime`。离线构建/测试使用：

```powershell
$env:GOTOOLCHAIN = "local"
$env:GOPROXY = "off"
$env:GOSUMDB = "off"
$env:CODEA_AST_GREP = "C:\approved-tools\ast-grep.exe"
```

上述 ast-grep 路径是示例，必须替换为已批准现有安装路径。缺模块时报告具体缺项，不临时联网；不通过跳过新测试来取得绿灯。生产 Runtime 不依赖 CODEA_AST_GREP 环境变量，该变量只供新增真实 AST 测试定位获准工具。

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
- JAVA_CALL 的 EXACT 目标必须唯一；SPRING_BINDING 的 EXACT 目标必须唯一且注册条件已确定。
- MYBATIS_STATEMENT / SQL_INCLUDE 的 EXACT 只针对具体语句/片段关联，不表示动态 SQL 必然执行。
- DUBBO_CONTRACT 的 EXACT 仅指源码契约匹配，不表示实际路由。
- 明确错误码：JAVA_OVERLOAD_UNRESOLVED、JAVA_RECEIVER_UNRESOLVED、SPRING_BEAN_AMBIGUOUS、SPRING_CONDITION_UNRESOLVED、GENERATED_CONSTRUCTOR_UNSUPPORTED、SQL_INCLUDE_CYCLE、SQL_INCLUDE_UNRESOLVED、DUBBO_CONFIG_UNRESOLVED、PROVIDER_SOURCE_UNAVAILABLE、CONTEXT_BUDGET_EXCEEDED、RECURSION_BOUNDARY、CONTEXT_RELATION_NOT_VERIFIED。
- 用户输入/协议/路径错误返回 error；正常业务未知返回 Issues/非 EXACT，不把全部不支持场景伪装成进程崩溃。
- 同一输入按 workspace/path/owner/signature/callsite 排序；实际耗时不参与关系 ID 或语义去重键。

## Task 1: 运行内关系契约和真实 AST 测试入口

**对应需求：** F1/F5 基础，不创建独立基础设施功能。

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

- [ ] **Step 1：添加身份碰撞及关系校验测试。** 方法同名不同参数类型、同名不同包、BASE/CURRENT、workspace 不同必须产生不同 key；缺 Evidence 的 EXACT、多目标 EXACT、../ 路径必须拒绝。核心测试：

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

**独立交付：** Mapper/XML 关联能被 Runtime 获取，规则 catalog 如需增加 RequiredEvidence，必须等 T7 evidence schema 同步后才启用新 kind，避免中间提交打断现有 Review。建议 commit：`feat(review): resolve MyBatis statement context`。

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
- [ ] **Step 3：加入边界回归。** 入口删除只保留删除说明；方法重载不唯一不能切换目标；2+ 业务链仍要求现有用户 scope selection；选择第一条不能因刷新变成全部。
- [ ] **Step 4：运行 `go test ./internal/reviewscope ./cmd/codea-dcep-tools -run Test170AutoChain -count=1`，记录旧 stale 流程失败。**
- [ ] **Step 5：实现 Review 专用默认策略。** 可调用当前已有临时 discovery/certification 回调；解析事实来自 T5/current certified analysis，不从旧 YAML 推导真值。删除维护确认分支的 Review 使用点，不删除显式 chain persist 的授权校验。
- [ ] **Step 6：更新 Agent/Skill 文案。** 不再指示用户先 refresh；仍禁止写 Project State 和伪造 human selection。确实无法解析的部分说明原因，不请求用户凭猜测指定实现。
- [ ] **Step 7：运行 `go test ./internal/reviewscope ./internal/chain ./cmd/codea-dcep-tools -count=1`，重点检查保存授权、USER_SELECTION 和旧显式管理入口，提交。**

**独立交付：** 普通 Review 的旧链过期无需人工维护，不改变项目 Chain 文件。建议 commit：`feat(review): refresh temporary chain context automatically`。

## Task 7: 关系证据、规则缺口及最终报告

**对应需求：** F6。

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

## Task 8: 随功能完成的真实 Review 与内网验收

**对应需求：** 六项功能的必要测试，不建设独立平台或发布专项。

**Files**

- Create: `.code-harness/tools-runtime/testdata/review-170/README.md`
- Create: `docs/superpowers/evidence/2026-09-09-codea-harness-1.7-review-context-validation.md`（执行时记录实际日期、版本和输出；没有结果不得写通过）
- Modify: `.code-harness/tools-runtime/internal/finding/benchmark_test.go`（仅澄清现有固定 proposal 测试名称/日志含义，保留断言）
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

工作目录仍为 tools-runtime，最后一条默认应无新增依赖差异。沿用仓库现有受影响的宿主协议测试；不新建 1.7 全量发布认证/升级回滚脚本。既有打包流水线若自动运行不得禁用。

- [ ] **Step 6：逐条记录验收。** 关系误判、scope 扩张、无依据 finding、未完成被报通过、运行时下载任一出现均需修复后复测；其他质量差异按实际问题解释。未执行的 Windows/真实模型检查保持“未执行”，不能用 mock 输出覆盖。
- [ ] **Step 7：更新用户说明并提交结果。** README 写清正常 Review 自动临时 Chain、支持/不支持模式及离线依赖；不提供 index/graph/snapshot 新命令。建议 commit：`test(review): validate call context in real offline review flows`。

## 计划完成检查

- [ ] F1 对应 T1/T2，F2 对应 T6，F3 对应 T3，F4 对应 T4，F5 对应 T5，F6 对应 T7。
- [ ] T8 只包含必要验证，未变成新增产品或发布工程。
- [ ] 没有修改/新增快照、持久索引、图查询或后台服务。
- [ ] 没有把 Review 自动链更新变成无授权写 chains/**。
- [ ] 没有放宽 FULL/TARGETED、USER_SELECTION、dependency 或认证边界。
- [ ] 所有新增 strict schema 同步 runtime/model/consumer；错误和未知状态有明确路径。
- [ ] 每项 task 都有真实失败场景、运行命令及可独立审核的提交。
