from pathlib import Path

p=Path('.code-harness/tools-runtime/internal/reviewcontext/build_170.go')
s=p.read_text()
old='''		for _, relation := range facts.Calls {
			if err := addRelation(relation); err != nil {
				return Context170{}, err
			}
			if relation.Resolution == "EXACT" && relation.Kind == "JAVA_CALL" && item.downDepth < budget.MaxDownstreamDepth {'''
new='''		for _, relation := range facts.Calls {
			if err := addRelation(relation); err != nil {
				return Context170{}, err
			}
			_, accepted := relationByKey[relationKey170(relation)]
			if accepted && relation.Resolution == "EXACT" && relation.Kind == "JAVA_CALL" && item.downDepth < budget.MaxDownstreamDepth {'''
assert old in s, 'downstream relation block not found'
s=s.replace(old,new)
s=s.replace('''			} else if relation.Resolution == "EXACT" && relation.Kind == "JAVA_CALL" && item.downDepth >= budget.MaxDownstreamDepth {''','''			} else if accepted && relation.Resolution == "EXACT" && relation.Kind == "JAVA_CALL" && item.downDepth >= budget.MaxDownstreamDepth {''',1)
old='''		for _, relation := range callerRelations {
			if err := addRelation(relation); err != nil {
				return Context170{}, err
			}
			if relation.Resolution == "EXACT" && item.upDepth < budget.MaxUpstreamDepth && relation.From.Side == "CURRENT" && relation.From.Kind == "METHOD" {'''
new='''		for _, relation := range callerRelations {
			if err := addRelation(relation); err != nil {
				return Context170{}, err
			}
			_, accepted := relationByKey[relationKey170(relation)]
			if accepted && relation.Resolution == "EXACT" && item.upDepth < budget.MaxUpstreamDepth && relation.From.Side == "CURRENT" && relation.From.Kind == "METHOD" {'''
assert old in s, 'upstream relation block not found'
s=s.replace(old,new)
s=s.replace('''			} else if relation.Resolution == "EXACT" && item.upDepth >= budget.MaxUpstreamDepth {''','''			} else if accepted && relation.Resolution == "EXACT" && item.upDepth >= budget.MaxUpstreamDepth {''',1)
p.write_text(s)

sections={
'.code-harness/skills/analyze-change/SKILL.md': '''\n\n## 1.7 按需 Review Context（T5）\n\nReview 的关系上下文只由 Controlled Runtime 构造。Agent 只能为同一 run 请求 `review context`，请求体固定为 `runId + phase`；不得提供 `roots/files/baseRef/snapshot/seeds/budget`。DISCOVERY 使用 Runtime 已有 Canonical ChangeSet，RULES 使用同 run Certified ChangeAnalysis + ReviewUnit + RuleDispatch。返回的 `review-call-context.json` / `review-rule-context.json` 仅是本次 review 的有界上下文，不是新的 Snapshot、索引或跨 run 权威；Agent 不得写入或修补 `analysis/**`。\n''',
'.code-harness/agents/reviewer.md': '''\n\n## 1.7 T5 Runtime Context 边界\n\nReviewer 继续保持独立角色，只消费 Runtime 提供的 same-run context，不自行扫描额外 roots，也不提供 seeds/budget/roots。DISCOVERY/RULES 请求仅包含 `runId` 与 `phase`；未知、歧义、预算耗尽关系必须保留为边界或 BLOCKED，不能猜测为 EXACT。Reviewer 的 semantic proposal 仍走既有 `codea-reviewer-submit`，不得写 `analysis/**`，也不能因有新上下文而接管 Runtime/Host 阶段推进。\n''',
'.code-harness/agents/orchestrator.md': '''\n\n## 1.7 T5 两阶段按需上下文\n\n1.7 Review 的顺序固定为：Runtime Canonical ChangeSet → DISCOVERY `review context` → 独立 Reviewer semantic proposal / Runtime certification → ReviewUnit →（T7 接入前 knowledge 关闭）一次 RuleDispatch → RULES `review context` → Runtime 内部把 `REVIEW_PLANNING` 推进到 `REVIEW_EXECUTION`。1.7 dispatch 本身不得立即推进阶段；RULES 未成功时 FINDINGS 不得开始。历史 run 若没有 1.7 DISCOVERY artifact，继续走既有 1.6 流程。Orchestrator 不得创建第二套 snapshot/context authority，也不得向 Runtime 注入 roots/files/seeds/budget。\n''',
'.code-harness/AGENTS.md': '''\n\n## 1.7 T5 Review Context 内部命令\n\n允许的内部入口为 `codea-dcep-tools.exe review context --input .code-harness/runs/<runId>/requests/<name>.json`。request 只允许 `runId`、`phase`，phase 仅 `DISCOVERY|RULES`。DISCOVERY 输出 `analysis/review-call-context.json`；RULES 输出 `analysis/review-rule-context.json`。两者均为 same-run Runtime Managed artifact，不可跨 run 复用，不扩展 ReviewUnit.Files / ChangedHunks / ReviewScope，也不授权 Agent 写 `analysis/**`。\n''',
'.code-harness/tools/README.md': '''\n\n## Review Context（1.7 T5，内部）\n\n```text\ncodea-dcep-tools.exe review context --input .code-harness/runs/<runId>/requests/<name>.json\n```\n\n唯一请求字段是 `runId` 和 `phase`（`DISCOVERY` 或 `RULES`）。DISCOVERY 从同 run 的 `analysis/change-set.json` 派生，产出 `analysis/review-call-context.json`；RULES 只在 Certified ChangeAnalysis、ReviewUnit、RuleDispatch 已就绪时执行，产出 `analysis/review-rule-context.json`，成功后由 Runtime 内部完成 REVIEW_PLANNING → REVIEW_EXECUTION。固定每阶段预算为 40 files / 200 candidates / 1 MiB source / 15s，向上 3 层、向下 6 层；超限保留 issue/BLOCKED，不扩大原 review selection。该命令只用于 Review，不是通用索引、Snapshot 或跨 run cache。\n'''
}
for name,section in sections.items():
    p=Path(name); text=p.read_text()
    marker=section.strip().splitlines()[0]
    if marker not in text:
        p.write_text(text.rstrip()+section+'\n')
