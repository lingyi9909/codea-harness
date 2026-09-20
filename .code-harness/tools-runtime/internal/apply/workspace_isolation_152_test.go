package apply

import (
	"strings"
	"testing"
)

func Test152WorkspaceDependencyFixPathCannotEscapeCurrentRepository(t *testing.T) {
	policy := Policy{AllowedProduction: []string{"src/main/java/**"}}
	path := "../company-framework/src/main/java/com/company/framework/AbstractTemplate.java"

	err := policy.Allow("FIX", path)
	if err == nil {
		t.Fatal("workspace dependency path must never be writable by FIX")
	}
	if !strings.Contains(err.Error(), "UNSAFE_PATH") {
		t.Fatalf("dependency escape must fail at repository boundary, err=%v", err)
	}
}

func Test152WorkspaceDependencyTestPathCannotEscapeCurrentRepository(t *testing.T) {
	policy := Policy{AllowedTest: []string{"src/test/java/**"}}
	path := "../company-framework/src/test/java/com/company/framework/AbstractTemplateTest.java"

	err := policy.Allow("TEST", path)
	if err == nil {
		t.Fatal("workspace dependency path must never be writable by TEST")
	}
	if !strings.Contains(err.Error(), "UNSAFE_PATH") {
		t.Fatalf("dependency escape must fail at repository boundary, err=%v", err)
	}
}
