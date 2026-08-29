package reviewrules

import (
	"strings"
	"testing"

	"codea-harness-tools/internal/reviewunit"
)

func TestChangedCurrentRolesUsesPathAuthorityForTests(t *testing.T) {
	unit := reviewunit.Unit{Files: []reviewunit.FileRef{{Path: "src/test/java/com/acme/OrderTest.java", Role: "Test", Changed: true, Workspace: "current"}}}
	roles, err := changedCurrentRoles160(unit)
	if err != nil { t.Fatal(err) }
	if !roles["Test"] { t.Fatal("src/test/** must machine-resolve to Test") }
}

func TestChangedCurrentRolesRejectsAgentRoleMismatch(t *testing.T) {
	cases := []reviewunit.FileRef{
		{Path: "src/test/java/com/acme/OrderTest.java", Role: "Service", Changed: true, Workspace: "current"},
		{Path: "src/main/java/com/acme/OrderService.java", Role: "Test", Changed: true, Workspace: "current"},
	}
	for _, file := range cases {
		_, err := changedCurrentRoles160(reviewunit.Unit{Files: []reviewunit.FileRef{file}})
		if err == nil || !strings.Contains(err.Error(), "RULE_DISPATCH_PATH_ROLE_INVALID") { t.Fatalf("file=%+v err=%v", file, err) }
	}
}
