package reviewcontext

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"codea-harness-tools/internal/nav"
)

type queue170 struct {
	ref       nav.ReviewRef170
	downDepth int
	upDepth   int
}

type budgetState170 struct {
	inputFiles  map[string]bool
	files       map[string]bool
	candidates  int
	sourceBytes int
	blocked     map[string]bool
}

type sourceSizer170 interface {
	SourceBytes(context.Context, nav.ReviewRef170) (int, error)
}

func Build170(ctx context.Context, input BuildInput170, resolver Resolver170) (Context170, error) {
	out := Context170{RunID: strings.TrimSpace(input.RunID), Phase: strings.ToUpper(strings.TrimSpace(input.Phase)), Relations: []nav.Relation170{}, Checks: []Check170{}, Issues: []nav.Issue170{}}
	if out.RunID == "" {
		return Context170{}, fmt.Errorf("REVIEW_CONTEXT_RUN_ID_INVALID")
	}
	if out.Phase != "DISCOVERY" && out.Phase != "RULES" {
		return Context170{}, fmt.Errorf("REVIEW_CONTEXT_PHASE_INVALID: %s", input.Phase)
	}
	if resolver == nil {
		return Context170{}, fmt.Errorf("REVIEW_CONTEXT_RESOLVER_UNAVAILABLE")
	}
	budget := normalizedBudget170(input.Budget)
	started := time.Now()
	if budget.MaxMillis > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(budget.MaxMillis)*time.Millisecond)
		defer cancel()
	}
	state := &budgetState170{inputFiles: map[string]bool{}, files: map[string]bool{}, blocked: map[string]bool{}}
	for _, seed := range input.Seeds {
		state.inputFiles[fileKey170(seed)] = true
	}
	for _, resource := range input.Resources {
		state.inputFiles[fileKey170(resource.Ref)] = true
	}

	relationByKey := map[string]nav.Relation170{}
	addIssue := func(issue nav.Issue170) {
		out.Issues = append(out.Issues, issue)
	}
	addBudgetIssue := func(kind string, at nav.SourceRange170) {
		state.blocked[kind] = true
		addIssue(nav.Issue170{Code: "CONTEXT_BUDGET_EXCEEDED", At: at, Detail: "bounded context exploration stopped before adding " + kind})
	}
	addRelation := func(rel nav.Relation170) error {
		rel = cloneRelation170(rel)
		if err := nav.ValidateRelation170(rel); err != nil {
			return err
		}
		state.candidates++
		if budget.MaxCandidates > 0 && state.candidates > budget.MaxCandidates {
			addBudgetIssue(rel.Kind, firstEvidence170(rel))
			return nil
		}
		newFiles := map[string]nav.ReviewRef170{}
		for _, target := range rel.Targets {
			key := fileKey170(target)
			if key != "" && !state.inputFiles[key] && !state.files[key] {
				newFiles[key] = target
			}
		}
		for _, evidence := range rel.Evidence {
			key := fileKey170(evidence.Ref)
			if key != "" && !state.inputFiles[key] && !state.files[key] {
				newFiles[key] = evidence.Ref
			}
		}
		if budget.MaxFiles > 0 && len(state.files)+len(newFiles) > budget.MaxFiles {
			addBudgetIssue(rel.Kind, firstEvidence170(rel))
			return nil
		}
		additionalBytes := 0
		if sizer, ok := resolver.(sourceSizer170); ok {
			for _, ref := range newFiles {
				size, err := sizer.SourceBytes(ctx, ref)
				if err != nil {
					return fmt.Errorf("REVIEW_CONTEXT_SOURCE_SIZE_FAILED: %w", err)
				}
				if size > 0 {
					additionalBytes += size
				}
			}
		}
		if budget.MaxSourceBytes > 0 && state.sourceBytes+additionalBytes > budget.MaxSourceBytes {
			addBudgetIssue(rel.Kind, firstEvidence170(rel))
			return nil
		}
		for key := range newFiles {
			state.files[key] = true
		}
		state.sourceBytes += additionalBytes
		key := relationKey170(rel)
		if _, exists := relationByKey[key]; !exists {
			relationByKey[key] = rel
		}
		return nil
	}

	for _, resource := range input.Resources {
		if err := ctx.Err(); err != nil {
			addBudgetIssue("RESOURCE", resource)
			break
		}
		relations, issues, err := resolver.Mapper(ctx, resource)
		if err != nil {
			return Context170{}, fmt.Errorf("REVIEW_CONTEXT_MAPPER_FAILED: %w", err)
		}
		out.Issues = append(out.Issues, issues...)
		for _, relation := range relations {
			if err := addRelation(relation); err != nil {
				return Context170{}, err
			}
		}
	}

	queue := make([]queue170, 0, len(input.Seeds))
	for _, seed := range input.Seeds {
		if seed.Side != "CURRENT" || seed.Kind != "METHOD" {
			continue
		}
		queue = append(queue, queue170{ref: seed})
	}
	visited := map[string]bool{}
	for len(queue) > 0 {
		if err := ctx.Err(); err != nil {
			at := nav.SourceRange170{Ref: queue[0].ref, StartLine: 1, EndLine: 1, StartColumn: 1, EndColumn: 2}
			addBudgetIssue("TIME", at)
			break
		}
		item := queue[0]
		queue = queue[1:]
		refKey, err := nav.ReviewRefKey170(item.ref)
		if err != nil {
			return Context170{}, err
		}
		if visited[refKey] {
			continue
		}
		visited[refKey] = true

		facts, err := resolver.Method(ctx, item.ref)
		if err != nil {
			return Context170{}, fmt.Errorf("REVIEW_CONTEXT_METHOD_FAILED: %w", err)
		}
		out.Issues = append(out.Issues, facts.Issues...)
		for _, relation := range facts.Calls {
			if err := addRelation(relation); err != nil {
				return Context170{}, err
			}
			_, accepted := relationByKey[relationKey170(relation)]
			if accepted && relation.Resolution == "EXACT" && relation.Kind == "JAVA_CALL" && item.downDepth < budget.MaxDownstreamDepth {
				for _, target := range relation.Targets {
					if target.Side == "CURRENT" && target.Kind == "METHOD" {
						queue = append(queue, queue170{ref: target, downDepth: item.downDepth + 1, upDepth: item.upDepth})
					}
				}
			} else if accepted && relation.Resolution == "EXACT" && relation.Kind == "JAVA_CALL" && item.downDepth >= budget.MaxDownstreamDepth {
				addIssue(nav.Issue170{Code: "RECURSION_BOUNDARY", At: firstEvidence170(relation), Detail: "downstream review context depth limit reached"})
			}
		}

		dubboRelations, dubboIssues, err := resolver.Dubbo(ctx, item.ref)
		if err != nil {
			return Context170{}, fmt.Errorf("REVIEW_CONTEXT_DUBBO_FAILED: %w", err)
		}
		out.Issues = append(out.Issues, dubboIssues...)
		for _, relation := range dubboRelations {
			if err := addRelation(relation); err != nil {
				return Context170{}, err
			}
		}

		callerRelations, err := resolver.Callers(ctx, item.ref)
		if err != nil {
			return Context170{}, fmt.Errorf("REVIEW_CONTEXT_CALLERS_FAILED: %w", err)
		}
		for _, relation := range callerRelations {
			if err := addRelation(relation); err != nil {
				return Context170{}, err
			}
			_, accepted := relationByKey[relationKey170(relation)]
			if accepted && relation.Resolution == "EXACT" && item.upDepth < budget.MaxUpstreamDepth && relation.From.Side == "CURRENT" && relation.From.Kind == "METHOD" {
				queue = append(queue, queue170{ref: relation.From, downDepth: item.downDepth, upDepth: item.upDepth + 1})
			} else if accepted && relation.Resolution == "EXACT" && item.upDepth >= budget.MaxUpstreamDepth {
				addIssue(nav.Issue170{Code: "RECURSION_BOUNDARY", At: firstEvidence170(relation), Detail: "upstream review context depth limit reached"})
			}
		}
	}

	out.Relations = make([]nav.Relation170, 0, len(relationByKey))
	for _, relation := range relationByKey {
		out.Relations = append(out.Relations, relation)
	}
	sort.Slice(out.Relations, func(i, j int) bool {
		return relationSortKey170(out.Relations[i]) < relationSortKey170(out.Relations[j])
	})
	out.Checks = buildChecks170(input.Needs, out.Relations, state.blocked)
	sort.Slice(out.Issues, func(i, j int) bool {
		left := out.Issues[i].Code + "\x00" + fileKey170(out.Issues[i].At.Ref) + fmt.Sprintf("\x00%09d\x00%09d", out.Issues[i].At.StartLine, out.Issues[i].At.StartColumn)
		right := out.Issues[j].Code + "\x00" + fileKey170(out.Issues[j].At.Ref) + fmt.Sprintf("\x00%09d\x00%09d", out.Issues[j].At.StartLine, out.Issues[j].At.StartColumn)
		return left < right
	})
	out.Usage = Usage170{Files: len(state.files), Candidates: state.candidates, SourceBytes: state.sourceBytes, ElapsedMillis: time.Since(started).Milliseconds()}
	return out, nil
}

func normalizedBudget170(b Budget170) Budget170 {
	d := DefaultBudget170()
	if b.MaxFiles <= 0 {
		b.MaxFiles = d.MaxFiles
	}
	if b.MaxCandidates <= 0 {
		b.MaxCandidates = d.MaxCandidates
	}
	if b.MaxSourceBytes <= 0 {
		b.MaxSourceBytes = d.MaxSourceBytes
	}
	if b.MaxUpstreamDepth <= 0 {
		b.MaxUpstreamDepth = d.MaxUpstreamDepth
	}
	if b.MaxDownstreamDepth <= 0 {
		b.MaxDownstreamDepth = d.MaxDownstreamDepth
	}
	if b.MaxMillis <= 0 {
		b.MaxMillis = d.MaxMillis
	}
	return b
}

func buildChecks170(needs []Need170, relations []nav.Relation170, budgetBlocked map[string]bool) []Check170 {
	checks := make([]Check170, 0, len(needs))
	for _, need := range needs {
		check := Check170{ReviewUnitID: need.ReviewUnitID, RuleID: need.RuleID, Status: "READY", RelationIDs: []string{}, Reasons: []string{}}
		for _, kind := range need.RequiredKinds {
			matched := []string{}
			for _, relation := range relations {
				if relation.Kind != kind || relation.Resolution != "EXACT" || !relationTouchesAnySeed170(relation, need.Seeds) {
					continue
				}
				matched = append(matched, relation.ID)
			}
			if len(matched) == 0 {
				check.Status = "BLOCKED"
				if budgetBlocked[kind] {
					check.Reasons = append(check.Reasons, "CONTEXT_BUDGET_EXCEEDED:"+kind)
				} else {
					check.Reasons = append(check.Reasons, "RELATION_REQUIRED:"+kind)
				}
				continue
			}
			check.RelationIDs = append(check.RelationIDs, matched...)
		}
		check.RelationIDs = uniqueSorted170(check.RelationIDs)
		check.Reasons = uniqueSorted170(check.Reasons)
		checks = append(checks, check)
	}
	sort.Slice(checks, func(i, j int) bool {
		if checks[i].ReviewUnitID != checks[j].ReviewUnitID {
			return checks[i].ReviewUnitID < checks[j].ReviewUnitID
		}
		return checks[i].RuleID < checks[j].RuleID
	})
	return checks
}

func relationTouchesAnySeed170(relation nav.Relation170, seeds []nav.ReviewRef170) bool {
	fromKey, _ := nav.ReviewRefKey170(relation.From)
	for _, seed := range seeds {
		seedKey, err := nav.ReviewRefKey170(seed)
		if err != nil {
			continue
		}
		if seedKey == fromKey {
			return true
		}
		for _, target := range relation.Targets {
			targetKey, err := nav.ReviewRefKey170(target)
			if err == nil && targetKey == seedKey {
				return true
			}
		}
	}
	return false
}

func relationKey170(relation nav.Relation170) string {
	return relation.ID + "\x00" + relationSortKey170(relation)
}

func relationSortKey170(relation nav.Relation170) string {
	from, _ := nav.ReviewRefKey170(relation.From)
	targets := make([]string, 0, len(relation.Targets))
	for _, target := range relation.Targets {
		key, _ := nav.ReviewRefKey170(target)
		targets = append(targets, key)
	}
	sort.Strings(targets)
	line, col := 0, 0
	if len(relation.Evidence) > 0 {
		line, col = relation.Evidence[0].StartLine, relation.Evidence[0].StartColumn
	}
	return from + "\x00" + relation.Kind + "\x00" + strings.Join(targets, "\x01") + fmt.Sprintf("\x00%09d\x00%09d\x00", line, col) + relation.ID
}

func fileKey170(ref nav.ReviewRef170) string {
	if strings.TrimSpace(ref.Workspace) == "" || strings.TrimSpace(ref.Path) == "" {
		return ""
	}
	return strings.TrimSpace(ref.Workspace) + "\x00" + strings.ToLower(strings.ReplaceAll(strings.TrimSpace(ref.Path), "\\", "/"))
}

func firstEvidence170(relation nav.Relation170) nav.SourceRange170 {
	if len(relation.Evidence) > 0 {
		return relation.Evidence[0]
	}
	return nav.SourceRange170{Ref: relation.From, StartLine: 1, EndLine: 1, StartColumn: 1, EndColumn: 2}
}

func uniqueSorted170(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
