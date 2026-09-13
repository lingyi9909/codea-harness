package analysis

import (
	"testing"

	"codea-harness-tools/internal/nav"
)

func Test170LegacyProjectionRejectsDependencyWorkspace(t *testing.T) {
	from := nav.ReviewRef170{Workspace:"current", Path:"src/main/java/demo/A.java", Side:"CURRENT", Kind:"METHOD", OwnerFQCN:"demo.A", Name:"submit", ParameterTypes:[]string{}}
	to := nav.ReviewRef170{Workspace:"dep", Path:"src/main/java/shared/B.java", Side:"CURRENT", Kind:"METHOD", OwnerFQCN:"shared.B", Name:"save", ParameterTypes:[]string{}}
	rel := nav.Relation170{ID:"r1", Kind:"JAVA_CALL", Resolution:"EXACT", From:from, Targets:[]nav.ReviewRef170{to}, Evidence:[]nav.SourceRange170{{Ref:from,StartLine:3,EndLine:3,StartColumn:2,EndColumn:9}}, Reason:"", Assumptions:[]string{}}
	got := ProjectLegacyCallChains170([]nav.Relation170{rel})
	if len(got) != 0 { t.Fatalf("dependency context expanded legacy selection: %+v", got) }
}

func Test170LegacyProjectionKeepsOnlyUnambiguousCurrentMethodEdge(t *testing.T) {
	from := nav.ReviewRef170{Workspace:"current", Path:"src/main/java/demo/A.java", Side:"CURRENT", Kind:"METHOD", OwnerFQCN:"demo.A", Name:"submit", ParameterTypes:[]string{}}
	to := nav.ReviewRef170{Workspace:"current", Path:"src/main/java/demo/B.java", Side:"CURRENT", Kind:"METHOD", OwnerFQCN:"demo.B", Name:"save", ParameterTypes:[]string{}}
	rel := nav.Relation170{ID:"r1", Kind:"JAVA_CALL", Resolution:"EXACT", From:from, Targets:[]nav.ReviewRef170{to}, Evidence:[]nav.SourceRange170{{Ref:from,StartLine:3,EndLine:3,StartColumn:2,EndColumn:9}}, Reason:"", Assumptions:[]string{}}
	got := ProjectLegacyCallChains170([]nav.Relation170{rel})
	if len(got) != 1 || len(got[0].Chain) != 2 || got[0].Chain[0] != "A.submit" || got[0].Chain[1] != "B.save" { t.Fatalf("projection=%+v", got) }
}

func Test170LegacyProjectionRejectsOverloadIdentityLoss(t *testing.T) {
	from := nav.ReviewRef170{Workspace:"current", Path:"src/main/java/demo/A.java", Side:"CURRENT", Kind:"METHOD", OwnerFQCN:"demo.A", Name:"submit", ParameterTypes:[]string{"java.lang.String"}}
	to := nav.ReviewRef170{Workspace:"current", Path:"src/main/java/demo/B.java", Side:"CURRENT", Kind:"METHOD", OwnerFQCN:"demo.B", Name:"save", ParameterTypes:[]string{}}
	rel := nav.Relation170{ID:"r1", Kind:"JAVA_CALL", Resolution:"EXACT", From:from, Targets:[]nav.ReviewRef170{to}, Evidence:[]nav.SourceRange170{{Ref:from,StartLine:3,EndLine:3,StartColumn:2,EndColumn:9}}, Reason:"", Assumptions:[]string{}}
	if got := ProjectLegacyCallChains170([]nav.Relation170{rel}); len(got) != 0 { t.Fatalf("overload projected into lossy legacy identity: %+v", got) }
}
