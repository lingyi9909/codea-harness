package nav

import (
	"encoding/json"
	"fmt"
	"strings"

	"codea-harness-tools/internal/projectpath"
)

type ReviewRef170 struct {
	Workspace      string   `json:"workspace"`
	Path           string   `json:"path"`
	Side           string   `json:"side"` // BASE | CURRENT
	Kind           string   `json:"kind"` // METHOD | FIELD | TYPE | STATEMENT | SQL_FRAGMENT
	OwnerFQCN      string   `json:"ownerFqcn"`
	Name           string   `json:"name"`
	ParameterTypes []string `json:"parameterTypes"`
}

type SourceRange170 struct {
	Ref         ReviewRef170 `json:"ref"`
	StartLine   int          `json:"startLine"`
	EndLine     int          `json:"endLine"`
	StartColumn int          `json:"startColumn"`
	EndColumn   int          `json:"endColumn"` // one-based Unicode rune columns, end exclusive
}

type Relation170 struct {
	ID          string           `json:"id"`
	Kind        string           `json:"kind"`
	Resolution  string           `json:"resolution"`
	From        ReviewRef170     `json:"from"`
	Targets     []ReviewRef170   `json:"targets"`
	Evidence    []SourceRange170 `json:"evidence"`
	Reason      string           `json:"reason"`
	Assumptions []string         `json:"assumptions"`
}

type Issue170 struct {
	Code   string         `json:"code"`
	At     SourceRange170 `json:"at"`
	Detail string         `json:"detail"`
}

type MethodFacts170 struct {
	Method ReviewRef170 `json:"method"`
	Calls  []Relation170 `json:"calls"`
	Issues []Issue170    `json:"issues"`
}

type Injection170 struct {
	Owner        ReviewRef170     `json:"owner"`
	Name         string           `json:"name"`
	DeclaredType string           `json:"declaredType"`
	Kind         string           `json:"kind"` // FIELD | CONSTRUCTOR | SETTER
	Qualifier    string           `json:"qualifier"`
	ResourceName string           `json:"resourceName"`
	Evidence     []SourceRange170 `json:"evidence"`
}

// ReviewRefKey170 returns a deterministic, collision-resistant identity using
// explicit JSON fields. Windows-equivalent repository paths deliberately share
// one identity; workspace, side, owner and parameter signature remain distinct.
func ReviewRefKey170(ref ReviewRef170) (string, error) {
	normalized, err := normalizeReviewRef170(ref)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("REVIEW_REF_INVALID: encode identity: %w", err)
	}
	return string(data), nil
}

func normalizeReviewRef170(ref ReviewRef170) (ReviewRef170, error) {
	ref.Workspace = strings.TrimSpace(ref.Workspace)
	if ref.Workspace == "" {
		return ReviewRef170{}, fmt.Errorf("REVIEW_REF_INVALID: workspace is required")
	}

	rawPath := strings.TrimSpace(strings.ReplaceAll(ref.Path, "\\", "/"))
	if strings.Contains(rawPath, ":") {
		return ReviewRef170{}, fmt.Errorf("REVIEW_REF_INVALID: path contains Windows drive/ADS separator")
	}
	p, ok := projectpath.Normalize(rawPath)
	if !ok {
		return ReviewRef170{}, fmt.Errorf("REVIEW_REF_INVALID: unsafe path %q", ref.Path)
	}
	ref.Path = strings.ToLower(p)

	ref.Side = strings.TrimSpace(ref.Side)
	if ref.Side != "BASE" && ref.Side != "CURRENT" {
		return ReviewRef170{}, fmt.Errorf("REVIEW_REF_INVALID: unsupported side %q", ref.Side)
	}
	ref.Kind = strings.TrimSpace(ref.Kind)
	switch ref.Kind {
	case "METHOD", "FIELD", "TYPE", "STATEMENT", "SQL_FRAGMENT":
	default:
		return ReviewRef170{}, fmt.Errorf("REVIEW_REF_INVALID: unsupported kind %q", ref.Kind)
	}
	ref.OwnerFQCN = strings.TrimSpace(ref.OwnerFQCN)
	if (ref.Kind == "METHOD" || ref.Kind == "FIELD" || ref.Kind == "TYPE") && ref.OwnerFQCN == "" {
		return ReviewRef170{}, fmt.Errorf("REVIEW_REF_INVALID: ownerFqcn is required for %s", ref.Kind)
	}
	ref.Name = strings.TrimSpace(ref.Name)
	if ref.Name == "" {
		return ReviewRef170{}, fmt.Errorf("REVIEW_REF_INVALID: name is required")
	}
	params := make([]string, len(ref.ParameterTypes))
	for i, parameterType := range ref.ParameterTypes {
		parameterType = strings.TrimSpace(parameterType)
		if parameterType == "" {
			return ReviewRef170{}, fmt.Errorf("REVIEW_REF_INVALID: parameterTypes[%d] is empty", i)
		}
		params[i] = parameterType
	}
	ref.ParameterTypes = params
	return ref, nil
}

func validateSourceRange170(source SourceRange170) error {
	if _, err := normalizeReviewRef170(source.Ref); err != nil {
		return err
	}
	if source.StartLine < 1 || source.EndLine < 1 || source.StartColumn < 1 || source.EndColumn < 1 {
		return fmt.Errorf("CONTEXT_RELATION_INVALID: source range coordinates must be positive")
	}
	if source.EndLine < source.StartLine {
		return fmt.Errorf("CONTEXT_RELATION_INVALID: source range ends before it starts")
	}
	if source.EndLine == source.StartLine && source.EndColumn <= source.StartColumn {
		return fmt.Errorf("CONTEXT_RELATION_INVALID: same-line source range end must be exclusive")
	}
	return nil
}

func ValidateRelation170(relation Relation170) error {
	if strings.TrimSpace(relation.ID) == "" {
		return fmt.Errorf("CONTEXT_RELATION_INVALID: id is required")
	}
	switch relation.Kind {
	case "JAVA_CALL", "SPRING_BINDING", "MYBATIS_STATEMENT", "SQL_INCLUDE", "DUBBO_CONTRACT":
	default:
		return fmt.Errorf("CONTEXT_RELATION_INVALID: unsupported kind %q", relation.Kind)
	}
	switch relation.Resolution {
	case "EXACT", "CONDITIONAL", "AMBIGUOUS", "UNRESOLVED":
	default:
		return fmt.Errorf("CONTEXT_RELATION_INVALID: unsupported resolution %q", relation.Resolution)
	}
	if _, err := normalizeReviewRef170(relation.From); err != nil {
		return err
	}
	for i, target := range relation.Targets {
		if _, err := normalizeReviewRef170(target); err != nil {
			return fmt.Errorf("CONTEXT_RELATION_INVALID: target[%d]: %w", i, err)
		}
	}
	for i, evidence := range relation.Evidence {
		if err := validateSourceRange170(evidence); err != nil {
			return fmt.Errorf("CONTEXT_RELATION_INVALID: evidence[%d]: %w", i, err)
		}
	}
	if relation.Resolution == "EXACT" {
		if len(relation.Targets) != 1 {
			return fmt.Errorf("CONTEXT_RELATION_INVALID: EXACT requires exactly one target")
		}
		if len(relation.Evidence) == 0 {
			return fmt.Errorf("CONTEXT_RELATION_INVALID: EXACT requires source evidence")
		}
	} else if strings.TrimSpace(relation.Reason) == "" {
		return fmt.Errorf("CONTEXT_RELATION_INVALID: non-EXACT relation requires reason")
	}
	return nil
}
