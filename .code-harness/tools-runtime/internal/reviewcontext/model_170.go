package reviewcontext

import (
	"context"

	"codea-harness-tools/internal/nav"
)

type Need170 struct {
	ReviewUnitID  string             `json:"reviewUnitId"`
	RuleID        string             `json:"ruleId"`
	Seeds         []nav.ReviewRef170 `json:"seeds"`
	RequiredKinds []string           `json:"requiredKinds"`
}

type Budget170 struct {
	MaxFiles           int
	MaxCandidates      int
	MaxSourceBytes     int
	MaxUpstreamDepth   int
	MaxDownstreamDepth int
	MaxMillis          int
}

type Usage170 struct {
	Files         int   `json:"files"`
	Candidates    int   `json:"candidates"`
	SourceBytes   int   `json:"sourceBytes"`
	ElapsedMillis int64 `json:"elapsedMillis"`
}

type BuildInput170 struct {
	RunID     string
	Phase     string
	Seeds     []nav.ReviewRef170
	Resources []nav.SourceRange170
	Needs     []Need170
	Budget    Budget170
}

type Check170 struct {
	ReviewUnitID string   `json:"reviewUnitId"`
	RuleID       string   `json:"ruleId"`
	Status       string   `json:"status"` // READY | BLOCKED
	RelationIDs  []string `json:"relationIds"`
	Reasons      []string `json:"reasons"`
}

type Context170 struct {
	RunID     string            `json:"runId"`
	Phase     string            `json:"phase"`
	Relations []nav.Relation170 `json:"relations"`
	Checks    []Check170        `json:"checks"`
	Issues    []nav.Issue170    `json:"issues"`
	Usage     Usage170          `json:"usage"`
}

type Resolver170 interface {
	Method(context.Context, nav.ReviewRef170) (nav.MethodFacts170, error)
	Callers(context.Context, nav.ReviewRef170) ([]nav.Relation170, error)
	Mapper(context.Context, nav.SourceRange170) ([]nav.Relation170, []nav.Issue170, error)
	Dubbo(context.Context, nav.ReviewRef170) ([]nav.Relation170, []nav.Issue170, error)
}

// DefaultBudget170 freezes the per-phase exploration limits. T5 owns the
// actual Build170/VerifyRelations170 orchestration and is intentionally not
// exposed by this task.
func DefaultBudget170() Budget170 {
	return Budget170{
		MaxFiles:           40,
		MaxCandidates:      200,
		MaxSourceBytes:     1048576,
		MaxUpstreamDepth:   3,
		MaxDownstreamDepth: 6,
		MaxMillis:          15000,
	}
}
