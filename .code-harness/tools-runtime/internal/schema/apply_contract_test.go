package schema

import (
	"os"
	"strings"
	"testing"
)

func TestApplyRequestSchemaContract(t *testing.T) {
	schemaBytes, err := os.ReadFile("../../../contracts/apply-request.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	valid := []byte(`{
      "runId":"run-1",
      "planType":"FIX",
      "planId":"fix-1",
      "diffSha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "files":[{"path":"src/main/java/A.java","baseSha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}],
      "unifiedDiff":"--- a/src/main/java/A.java\n+++ b/src/main/java/A.java\n@@ -1 +1 @@\n-old\n+new\n"
    }`)
	if err := ValidateJSON(schemaBytes, valid); err != nil {
		t.Fatalf("valid apply request rejected: %v", err)
	}
	for name, invalid := range map[string]string{
		"empty diff": strings.Replace(string(valid), `"unifiedDiff":"--- a/src/main/java/A.java\n+++ b/src/main/java/A.java\n@@ -1 +1 @@\n-old\n+new\n"`, `"unifiedDiff":""`, 1),
		"invalid plan type": strings.Replace(string(valid), `"planType":"FIX"`, `"planType":"OTHER"`, 1),
		"unknown field": strings.Replace(string(valid), `"runId":"run-1",`, `"runId":"run-1","unexpected":true,`, 1),
	} {
		if err := ValidateJSON(schemaBytes, []byte(invalid)); err == nil {
			t.Fatalf("%s must be rejected", name)
		}
	}
	duplicate := []byte(`{
      "runId":"run-1","planType":"FIX","planId":"fix-1",
      "diffSha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "files":[
        {"path":"src/main/java/A.java","baseSha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
        {"path":"src/main/java/A.java","baseSha256":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}
      ],
      "unifiedDiff":"--- a/src/main/java/A.java\n+++ b/src/main/java/A.java\n@@ -1 +1 @@\n-old\n+new\n"
    }`)
	if err := ValidateJSON(schemaBytes, duplicate); err == nil {
		t.Fatal("duplicate file paths must be rejected")
	}
}

func TestApplyResultSchemaContract(t *testing.T) {
	schemaBytes, err := os.ReadFile("../../../contracts/apply-result.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	valid := []byte(`{
      "runId":"run-1","planType":"TEST","planId":"test-1",
      "diffSha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "status":"APPLIED",
      "appliedAt":"2026-08-22T09:01:00Z",
      "files":[{"path":"src/test/java/AIT.java","beforeSha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","afterSha256":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}],
      "rollbackPerformed":false
    }`)
	if err := ValidateJSON(schemaBytes, valid); err != nil {
		t.Fatalf("valid apply result rejected: %v", err)
	}
	missingAppliedAt := []byte(strings.Replace(string(valid), `"appliedAt":"2026-08-22T09:01:00Z",`, "", 1))
	if err := ValidateJSON(schemaBytes, missingAppliedAt); err == nil {
		t.Fatal("successful apply result without appliedAt must be rejected")
	}
}
