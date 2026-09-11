package nav

import "testing"

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

func Test170IdentityIncludesPackageSideAndWorkspace(t *testing.T) {
	base := ReviewRef170{Workspace:"current", Path:"src/main/java/a/Pay.java", Side:"CURRENT", Kind:"METHOD", OwnerFQCN:"a.Pay", Name:"pay", ParameterTypes:[]string{}}
	variants := []ReviewRef170{
		{Workspace:"current", Path:"src/main/java/b/Pay.java", Side:"CURRENT", Kind:"METHOD", OwnerFQCN:"b.Pay", Name:"pay", ParameterTypes:[]string{}},
		{Workspace:"current", Path:"src/main/java/a/Pay.java", Side:"BASE", Kind:"METHOD", OwnerFQCN:"a.Pay", Name:"pay", ParameterTypes:[]string{}},
		{Workspace:"dependency:payments", Path:"src/main/java/a/Pay.java", Side:"CURRENT", Kind:"METHOD", OwnerFQCN:"a.Pay", Name:"pay", ParameterTypes:[]string{}},
	}
	want, err := ReviewRefKey170(base); if err != nil { t.Fatal(err) }
	for i, v := range variants {
		got, err := ReviewRefKey170(v); if err != nil { t.Fatalf("variant %d: %v", i, err) }
		if got == want { t.Fatalf("variant %d collides with base identity", i) }
	}
}

func Test170WindowsEquivalentPathHasOneIdentity(t *testing.T) {
	a := ReviewRef170{Workspace:"current", Path:`src\main\java\a\Pay.java`, Side:"CURRENT", Kind:"TYPE", OwnerFQCN:"a.Pay", Name:"Pay", ParameterTypes:[]string{}}
	b := a
	b.Path = "SRC/main/java/a/Pay.java"
	ka, err := ReviewRefKey170(a); if err != nil { t.Fatal(err) }
	kb, err := ReviewRefKey170(b); if err != nil { t.Fatal(err) }
	if ka != kb { t.Fatalf("Windows-equivalent paths differ: %q != %q", ka, kb) }
}

func Test170RejectsUnsafeReviewRef(t *testing.T) {
	ref := ReviewRef170{Workspace:"current", Path:"../private/Pay.java", Side:"CURRENT", Kind:"TYPE", OwnerFQCN:"a.Pay", Name:"Pay", ParameterTypes:[]string{}}
	if _, err := ReviewRefKey170(ref); err == nil { t.Fatal("traversal path accepted") }
}

func Test170ValidateRelationStrictResolution(t *testing.T) {
	from := ReviewRef170{Workspace:"current", Path:"src/main/java/a/Pay.java", Side:"CURRENT", Kind:"METHOD", OwnerFQCN:"a.Pay", Name:"submit", ParameterTypes:[]string{}}
	target := ReviewRef170{Workspace:"current", Path:"src/main/java/a/Risk.java", Side:"CURRENT", Kind:"METHOD", OwnerFQCN:"a.Risk", Name:"check", ParameterTypes:[]string{}}
	evidence := SourceRange170{Ref:from, StartLine:10, EndLine:10, StartColumn:5, EndColumn:17}

	valid := Relation170{ID:"r1", Kind:"JAVA_CALL", Resolution:"EXACT", From:from, Targets:[]ReviewRef170{target}, Evidence:[]SourceRange170{evidence}, Assumptions:[]string{}}
	if err := ValidateRelation170(valid); err != nil { t.Fatalf("valid EXACT rejected: %v", err) }

	missingEvidence := valid; missingEvidence.Evidence = []SourceRange170{}
	if err := ValidateRelation170(missingEvidence); err == nil { t.Fatal("EXACT without evidence accepted") }

	multi := valid; multi.Targets = []ReviewRef170{target, target}
	if err := ValidateRelation170(multi); err == nil { t.Fatal("multi-target EXACT accepted") }

	unknown := valid; unknown.Resolution = "AMBIGUOUS"; unknown.Targets = []ReviewRef170{target, target}; unknown.Reason = ""
	if err := ValidateRelation170(unknown); err == nil { t.Fatal("non-EXACT without reason accepted") }
}

func Test170ValidateRelationRejectsInvalidSourceRange(t *testing.T) {
	from := ReviewRef170{Workspace:"current", Path:"src/main/java/a/Pay.java", Side:"CURRENT", Kind:"METHOD", OwnerFQCN:"a.Pay", Name:"submit", ParameterTypes:[]string{}}
	target := ReviewRef170{Workspace:"current", Path:"src/main/java/a/Risk.java", Side:"CURRENT", Kind:"METHOD", OwnerFQCN:"a.Risk", Name:"check", ParameterTypes:[]string{}}
	relation := Relation170{ID:"r1", Kind:"JAVA_CALL", Resolution:"EXACT", From:from, Targets:[]ReviewRef170{target}, Evidence:[]SourceRange170{{Ref:from, StartLine:10, EndLine:9, StartColumn:5, EndColumn:4}}, Assumptions:[]string{}}
	if err := ValidateRelation170(relation); err == nil { t.Fatal("invalid source range accepted") }
}
