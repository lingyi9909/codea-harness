package upgrade

import "testing"

func TestTask160ReviewRulesAreFrameworkManaged(t *testing.T) {
	if !isManaged("review-rules/spring-v1.yaml") {
		t.Fatal("1.6 review-rules/** must be Framework Managed so live upgrade installs/replaces the rule catalog")
	}
	if isProjectState("review-rules/spring-v1.yaml") {
		t.Fatal("review-rules/** must never be Project State")
	}
}
