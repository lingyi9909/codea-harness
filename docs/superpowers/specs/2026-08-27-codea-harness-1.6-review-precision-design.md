# Codea Harness 1.6 — Review Precision Design

## Goal

Codea Harness 1.6 只做一件事：**把 Code Review 做深、做准、做稳定**。

1.5.3 已经解决了“审什么”的主要可靠性问题：真实 Git Change Set、Changed Controller / EntryPoint Completeness、Certified ChangeAnalysis、Verified Chain、Review Scope Selection 都由 Controlled Runtime 约束。1.6 不重做这些能力，而是把同样的 trust model 继续推进到 Finding 侧：

```text
Agent proposes.
Runtime verifies.
Only Runtime-certified Findings enter the formal report.
```

目标链路：

```text
Change Set
  -> Certified ChangeAnalysis
  -> ReviewUnit
  -> Rule Dispatch
  -> Agent Finding Proposal
  -> Runtime Finding Verify / Anchor / Dedup
  -> Certified Findings
  -> review.md
```

用户可见目标：

1. 同一个 Spring 变更多次 Review，重要 Finding 的结果明显更稳定；
2. Finding 必须能落到真实文件、真实 symbol / resource evidence，声称 LINE 时必须回挂到真实源码行；
3. Review 不再把一堆散文件直接交给 Agent，而是按业务入口和 Verified Chain 组织完整上下文；
4. Spring/MyBatis 高价值风险由 Runtime 确定性分发规则，Agent 不再仅靠通用 Prompt 临场决定“该查什么”；
5. 误报优先控制，风格/命名/泛化重构继续默认不报；
6. 每个版本可以通过固定 Benchmark 证明 Precision、Must-find Recall、Anchor Rate 和稳定性是否变好。

## Baseline

- Exact 1.6 design baseline: `6f4c050783a7ec21f370799c1a8c69c9b51a9e92`（Codea Harness 1.5.3 release merge）。
- 1.5.3 的以下语义全部锁定，不因 1.6 修改：
  - Change Set = merge-base(baseRef, HEAD) 到 HEAD committed + staged + unstaged + untracked；
  - Certified ChangeAnalysis 是 Review/Chain 的唯一 authoritative analysis；
  - Workspace Dependency 只能提供 Navigation / Chain Context，不扩展 Change Set、Review Scope 或 Finding Scope；
  - plain `harness review`: 0 Chains -> AUTO_FULL，1 Chain -> AUTO_SINGLE，2+ -> USER_SELECTION；
  - explicit Controller / Controller.method / downstream target 的 1.5.3 selection 语义保持不变；
  - Test code 默认不产生普通 Code Review Finding，只保留 TEST_VALIDITY；
  - Chain YAML `version: 1`，不做 Project State migration；
  - Windows 10/11 x64、离线运行、Maven/Spring/Java 场景保持不变。

## Non-goals

1.6 明确**不做**：

- 新的 CI Review 产品入口；
- session list / resume；
- doctor；
- Dashboard；
- 多语言扩展；
- 通用 SAST 平台；
- project-wide call graph；
- 新 Java parser / JDT LS；
- bundle 并发；
- Token 计费 UI；
- Test/Fix/Apply 新功能；
- 改写 1.5.3 Chain / Review Selection 行为。

这些全部可以在 Review Precision 稳定后再讨论。

## Current problem

### Problem A — Review context is still Agent-shaped

1.5.3 已经证明 `changedFiles[] / callChains[] / symbolLocations[] / resourceRelations[]`，但进入 Finding Review 后，Agent 仍需要自己把这些事实重新组织成“本次应该一起看的上下文”。

风险：

- Controller、ServiceImpl、Mapper.xml、YML 之间的关联可能在 Prompt 中分散；
- 同一个大 Change Set 中不同入口相互污染上下文；
- Agent 为补上下文产生额外 `read_code` 调用，顺序和选择存在波动。

### Problem B — Rules are mostly prose instructions

当前 `review-code` Skill 已经有高价值检查范围，但“哪个 Review Scope 应检查哪类风险”主要仍通过自然语言 Prompt 传递。

风险：

- 事务代码、Mapper XML、配置变更等规则被模型漏看；
- 不相关规则也可能消耗注意力；
- 无法精确度量某条规则是“未分发、已分发未发现、发现后被验证拒绝”。

### Problem C — Finding is not independently certified

当前 Review transport 已经校验：

- Finding.file 是否属于 reviewed/scoped files；
- category / severity / confidence 格式；
- required fields 是否存在。

但 Runtime 还没有独立证明：

- Agent 指定的 line 是否真的对应问题证据；
- Agent 引用的 symbol / statement / config key 是否存在；
- Finding 是否仍绑定当前 Certified ChangeAnalysis；
- 同一事实是否被重复包装为多个 Finding；
- evidence 是否来自当前 ReviewUnit，而不是 Agent 记忆或 scope 外上下文。

### Problem D — No stable quality benchmark

没有固定 Spring 变更基准集时，Prompt/规则/上下文的调整无法用同一组样例证明质量提高，只能靠单次人工观感。

## Architecture decision

### Considered approaches

#### A. Prompt-only deep review

继续扩展 `review-code/SKILL.md`，增加更多 Spring/MyBatis 检查说明。

优点：快。

缺点：结果稳定性、可验证性和可度量性基本不变，不符合 1.5.3 已形成的 Runtime authority 方向。

**Reject.**

#### B. Machine-only Spring static rules

把 Spring/MyBatis 风险全部实现成 Runtime 静态规则，直接输出 Finding。

优点：稳定、低成本。

缺点：大量企业业务风险需要语义和上下文；容易变成一个低质量 SAST，与 Codea Agent-native 定位冲突。

**Reject as primary architecture.** 只允许少量 100% 可判定规则成为 MACHINE Finding。

#### C. Deterministic pipeline + Agent semantic judgment

Runtime 负责 ReviewUnit、Rule Dispatch、Finding anchor/evidence/certification；Agent 只负责需要语义判断的风险发现和解释。

优点：沿用 1.5.3 trust model，同时保留 LLM 深度分析能力。

**Selected.**

## 1. ReviewUnit

### Principle

ReviewUnit 是 Runtime-owned 的**最小完整评审上下文单元**，不是 Agent 自己组织的文件列表。

来源只能是同 run 的：

- Certified ChangeAnalysis；
- Runtime-verified ReviewScopeSelection（TARGETED 时）；
- Verified `callChains[]`；
- Verified `symbolLocations[]`；
- Verified current-project `resourceRelations[]`；
- real Change Set hunks。

### Artifact

```text
.code-harness/runs/<runId>/analysis/review-units.json
```

示例：

```json
{
  "runId": "r123",
  "changeAnalysisSha256": "...",
  "mode": "TARGETED",
  "units": [
    {
      "id": "RU-9d9d...",
      "entryPoint": "OrderController.approve",
      "chain": [
        "OrderController.approve",
        "OrderService.approve",
        "OrderServiceImpl.approve",
        "OrderMapper.updateStatus"
      ],
      "files": [
        {"path":"src/main/java/.../OrderController.java","role":"Controller","changed":true},
        {"path":"src/main/java/.../OrderServiceImpl.java","role":"Service","changed":true},
        {"path":"src/main/java/.../OrderMapper.java","role":"Mapper","changed":false},
        {"path":"src/main/resources/.../OrderMapper.xml","role":"MapperXml","changed":true}
      ],
      "changedHunks": [
        {"path":"src/main/resources/.../OrderMapper.xml","newStart":42,"newLines":8}
      ]
    }
  ]
}
```

### Construction rules

1. 一个 confirmed entrypoint branch 对应一个 canonical ReviewUnit；exact core chain 相同的重复 branch 必须 canonicalize。
2. 同一个 entrypoint 存在多个 verified branch 时，branch 不得被静默合并；每个不同 verified core signature 都有独立 Unit ID。
3. FULL 模式下，所有 Finding-scope required files 必须至少属于一个 Unit，无法绑定到 confirmed chain 的 changed production file 进入 deterministic `RU-FILE-*` standalone Unit，而不是丢弃。
4. TARGETED 模式下只允许使用 verified `scopedFiles` 和 selected callChains；scope 外文件不得进入 Unit。
5. Workspace Dependency symbol 可以作为 `contextSymbols`，但 dependency path 绝不进入 `files[]`，绝不允许产生 Finding。
6. Unit ID 必须由 canonical content digest 生成，不能由 Agent 名称或数组顺序决定。
7. `review-units.json` 必须绑定 `changeAnalysisSha256` 与 mode/scope identity；analysis/scope 变化后旧 Unit 必须 stale。

## 2. Rule Dispatch

### Principle

Runtime 确定“这个 ReviewUnit 应该检查哪些规则”；Agent 决定需要语义判断的规则是否真的构成问题。

规则分两类：

```text
MACHINE
AGENT
```

- `MACHINE`：只有当 Runtime 能用确定性事实完整证明问题成立时，才允许直接形成 Machine Finding Proposal；
- `AGENT`：Runtime 只产生 RuleDispatch，Agent 必须结合 ReviewUnit 源码判断。

严禁：`matcher hit == finding true` 作为通用规则。

### Framework-owned rule catalog

```text
.code-harness/review-rules/spring-v1.yaml
```

每条规则至少包含：

```yaml
id: MYBATIS-SQL-001
version: 1
kind: AGENT
severityDefault: high
appliesTo:
  roles: [MapperXml]
triggers:
  changedResourceKinds: [MapperXml]
requiredEvidence:
  - CHANGED_RANGE
  - RESOURCE_RELATION
prompt: >-
  检查本次 UPDATE/DELETE 变更是否删除或显著弱化 WHERE / 数据隔离条件；
  只有源码证据足够时才提出 Finding。
```

Rule matcher 只允许消费 Runtime 已有结构化事实：file role、symbol role、changed hunks、verified annotations/evidence、resource kind、chain membership。1.6 不新增正则 Java parser。

### Artifact

```text
.code-harness/runs/<runId>/analysis/rule-dispatch.json
```

每项必须包含：

- `reviewUnitId`
- `ruleId`
- `ruleVersion`
- `kind`
- `severityDefault`
- `requiredEvidence[]`
- `dispatchReason[]`

RuleDispatch 必须 deterministic：相同 Unit + Catalog -> byte-stable canonical output。

## 3. Spring Rule Pack v1

1.6 只做 10 条高价值规则，不追数量。

### MyBatis / DB

1. `MYBATIS-SQL-001`：UPDATE/DELETE WHERE 缺失或本次变更明显弱化；AGENT，若 parser/evidence 能 100% 证明无 WHERE 可升级 MACHINE。
2. `MYBATIS-ISOLATION-001`：tenant/org/user 数据隔离条件被删除/弱化；AGENT。
3. `MYBATIS-BIND-001`：`${}` 新增/扩大用于 SQL 动态拼接；AGENT，明确新增 `${}` 可产生高置信 signal，但是否可利用仍由 Agent 判断。
4. `MYBATIS-CONTRACT-001`：Mapper method 与 XML statement id/parameter/result contract 的本次变更不一致；AGENT。

### Spring transaction

5. `SPRING-TX-001`：`@Transactional` 同 Bean 自调用导致代理事务不生效候选；AGENT。
6. `SPRING-TX-002`：checked exception rollback 语义与本次异常路径不一致候选；AGENT。
7. `SPRING-TX-003`：readOnly transaction 内出现写路径候选；AGENT。

### API / Auth / Config

8. `SPRING-AUTH-001`：changed/new endpoint 的鉴权/权限约束相比同类已验证模式缺失或弱化；AGENT。不得仅凭“没有某个固定注解名”直接报 Finding。
9. `SPRING-VALIDATION-001`：changed/new request handling 对关键输入约束缺失并可沿 verified chain 到达危险操作；AGENT。
10. `SPRING-CONFIG-001`：changed datasource/pool/timeout/retry/log-level/feature-switch key 产生高风险行为；AGENT。

风格、命名、重复代码、一般可维护性规则全部不进入 v1。

## 4. Finding Proposal

Agent 不再直接拥有 formal Finding。

输出位置：

```text
.code-harness/runs/<runId>/requests/finding-proposals.json
```

每个 proposal：

```json
{
  "proposalId": "P-001",
  "reviewUnitId": "RU-...",
  "ruleId": "SPRING-TX-001",
  "category": "PRODUCTION_CODE",
  "severity": "high",
  "anchor": {
    "kind": "LINE",
    "path": "src/main/java/.../OrderServiceImpl.java",
    "line": 88,
    "symbol": "OrderServiceImpl.approve"
  },
  "evidenceRefs": [
    {"kind":"SYMBOL","value":"OrderServiceImpl.approve"},
    {"kind":"SOURCE_RANGE","path":"src/main/java/.../OrderServiceImpl.java","startLine":82,"endLine":91}
  ],
  "problem": "...",
  "impact": "...",
  "recommendation": "...",
  "needsTest": true,
  "introducedByChange": true,
  "confidence": 0.93
}
```

Agent 只能引用当前 `reviewUnitId` 已包含/允许的 evidence，不能创造 scope 外 path/symbol。

## 5. Finding Verify / Anchor

### Anchor kinds

正式支持：

```text
LINE
SYMBOL
FILE
CHANGESET
```

规则：

- Agent 声称 `LINE`：Runtime 必须证明 path 属于当前 Unit/Scope、line 存在、line 落在对应 symbol/source range；否则 proposal rejected，不能自动改成另一个行号。
- `SYMBOL`：必须能由 Certified ChangeAnalysis / pinned navigation evidence 回挂 exact symbol + current project path。
- `FILE`：只允许用于确实不存在单一 symbol/line 的文件级事实；path 必须在 Finding Scope。
- `CHANGESET`：只允许跨文件一致性问题，必须列出至少两个 verified evidenceRefs；不能作为无法定位时的兜底。

不能要求所有合法 Finding 都有 line。缺失型问题（例如 new endpoint 的跨层约束缺失）可以是 SYMBOL/FILE，只要证据满足对应 kind。

### Required verification

Runtime 对每条 proposal 验证：

1. proposal schema；
2. same run / same Certified ChangeAnalysis；
3. `reviewUnitId` 存在且未 stale；
4. `ruleId` 在该 Unit 的 RuleDispatch 中；
5. Finding.file/anchor path 属于正式 FULL/TARGETED Finding Scope；
6. dependency workspace path hard reject；
7. symbol/resource evidence 真实；
8. LINE/SOURCE_RANGE 在当前 bytes 中真实存在；
9. `introducedByChange=true` 时 evidence 必须与 Change Set hunk 或由该 hunk 引起的 verified cross-file contract change 有证明关系；
10. TEST_VALIDITY 继续服从原有限制；
11. evidenceRefs 不得引用 Unit 外事实；
12. duplicate semantic key 合并/拒绝。

### Duplicate key

Canonical duplicate identity：

```text
ruleId + anchor.kind + normalizedPath + canonicalSymbol/resource + canonical evidence digest
```

同一问题即使 Agent 生成不同 `proposalId` 或不同中文措辞，也只能进入一个 Certified Finding。

## 6. Certified Findings

Runtime-owned artifacts：

```text
.code-harness/runs/<runId>/analysis/certified-findings.json
.code-harness/runs/<runId>/analysis/certified-findings.cert.json
```

certificate 至少绑定：

- runId
- harnessVersion
- changeSetSha256
- changeAnalysisSha256
- reviewUnitsSha256
- ruleDispatchSha256
- findingProposalsSha256
- certifiedFindingsSha256
- mode / target / scope identity

任何上游 authoritative bytes 变化，旧 Certified Findings 必须 fail closed。

Formal `review.md` 只能由 Runtime loader 读取 Certified Findings；不能再把 Agent `findings[]` 直接作为 report authority。

## 7. Report behavior

用户看到的报告仍保持当前简洁结构，不暴露大量内部 pipeline。

每条正式 Finding 继续展示：

- severity
- category
- file + line/symbol/file anchor
- problem
- evidence
- impact
- recommendation
- needsTest
- confidence

新增可追溯信息可以进入结构化 JSON/内部 report transport：

- `ruleId`
- `anchorKind`
- `reviewUnitId`
- `findingId`

`review.md` 不需要展示证书 SHA 等内部字段。

如果 Agent proposals 全部被 Runtime 拒绝：

- 若 Review Coverage/Runtime pipeline 本身完整，允许正式结果为“未发现已验证问题”；
- 不得把 verification rejection 自动变成 MANUAL_ACTION_REQUIRED，除非 rejection 原因说明 Runtime 自身无法验证 required evidence 或 authoritative artifact stale/corrupt；
- rejection 原因写入 machine/debug artifact，不向用户制造伪 Finding。

## 8. Benchmark

1.6 必须和实现同步建立最小 Benchmark，不允许等 release 最后才补。

目录：

```text
.code-harness/tools-runtime/testdata/review-benchmark/
```

首版固定 24 cases：

- 12 positive must-find；
- 8 negative must-not-find；
- 4 anchor/dedup/stability contract cases。

Positive 至少覆盖：

- UPDATE/DELETE WHERE 弱化；
- tenant/org/user 条件被移除；
- `${}` 动态 SQL 风险；
- Mapper method/XML contract mismatch；
- transaction self-invocation；
- checked exception rollback；
- readOnly write path；
- auth weakening；
- request validation omission；
- dangerous config change；
- TEST_VALIDITY false-positive test；
- cross-file contract inconsistency。

Negative 至少覆盖：

- 纯格式化；
- 命名；
- 测试命名；
- 合法动态 SQL；
- 合法 transaction cross-bean call；
- 合法 readOnly read path；
- 未变化 config；
- workspace dependency context 代码。

### Release quality metrics

固定计算：

```text
Precision = certified true findings / all certified findings
MustFindRecall = must-find cases found / total must-find cases
AnchorRate = findings with verified anchor / all certified findings
DuplicateRate = duplicate certified findings / all certified findings
Stability = repeated deterministic pipeline inputs producing same Unit/Dispatch/Certification decisions
```

Release Gate：

- Precision >= 0.90；
- MustFindRecall >= 0.85；
- AnchorRate = 1.00；
- DuplicateRate = 0；
- deterministic Runtime artifact stability = 1.00。

这些阈值只针对仓库内固定 Benchmark，不宣称代表所有生产代码。

## 9. Runtime commands

1.6 只增加受控内部 Runtime 子命令，不新增一个独立 CLI 产品：

```text
codea-harness-tools review units --run-id <runId>
codea-harness-tools review dispatch --run-id <runId>
codea-harness-tools review certify-findings --input <request.json>
```

它们是现有 Orchestrator 的确定性执行 primitive，不拥有第二套 Review orchestration。

禁止新增：

```text
codea-harness-tools review --mode full
```

这种绕过 Agent-native Orchestrator 的第二套完整 Review pipeline。

## 10. Artifact ownership

Agent-owned proposal：

```text
runs/<runId>/requests/finding-proposals.json
```

Runtime-owned：

```text
runs/<runId>/analysis/review-units.json
runs/<runId>/analysis/rule-dispatch.json
runs/<runId>/analysis/certified-findings.json
runs/<runId>/analysis/certified-findings.cert.json
runs/<runId>/review.md
```

Framework-owned packaged rules：

```text
.code-harness/review-rules/spring-v1.yaml
```

Agent generic write/edit 对 Runtime-owned paths 的既有 deny/certification 原则继续适用。

## 11. Failure semantics

固定 machine codes：

```text
REVIEW_UNIT_STALE
REVIEW_UNIT_SCOPE_VIOLATION
RULE_DISPATCH_STALE
RULE_NOT_DISPATCHED
FINDING_PROPOSAL_INVALID
FINDING_ANCHOR_NOT_VERIFIED
FINDING_EVIDENCE_NOT_VERIFIED
FINDING_SCOPE_VIOLATION
FINDING_DEPENDENCY_SCOPE_FORBIDDEN
FINDING_INTRODUCED_BY_CHANGE_NOT_VERIFIED
FINDING_DUPLICATE
CERTIFIED_FINDINGS_STALE
CERTIFIED_FINDINGS_HASH_MISMATCH
```

语义：

- Agent 单条 proposal 证据不足：reject that proposal；
- authoritative artifact stale/corrupt、scope authority 无法证明：fail closed，formal report 不得继续；
- Runtime 不得“帮 Agent 猜一个正确 line/symbol”后静默放行。

## 12. Acceptance

1.6 只有同时满足以下条件才可 release：

1. ReviewUnit deterministic 且绑定 Certified Analysis/Scope；
2. Rule Dispatch deterministic；
3. Agent 只能输出 proposal，不能直接进入 formal report；
4. LINE/SYMBOL/FILE/CHANGESET anchor 全部按各自证据规则机器验证；
5. scope 外、dependency workspace、未分发 rule、伪造 symbol/line 全部拒绝；
6. Certified Findings 对上游 artifact tamper/stale fail closed；
7. formal `review.md` 只消费 Certified Findings；
8. 10 条 Spring Rule Pack v1 行为有正反测试；
9. 24-case benchmark 达到固定阈值；
10. 1.5.3 的 Review Selection、Chain、Workspace Dependency、Test Validity regressions 全绿；
11. Windows x64 formal install/upgrade package 通过真实 1.5.3 -> 1.6 live upgrade，Project State byte-preserved；
12. 版本更新为 1.6.0 并完成 release checklist。
