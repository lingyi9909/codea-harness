package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"
)

func schemaHashText(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestFixPlanRequiresExactPatchIdentity(t *testing.T) {
	schemaBytes, err := os.ReadFile("../../../contracts/fix-plan.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	diff := "--- a/src/main/java/A.java\n+++ b/src/main/java/A.java\n@@ -1 +1 @@\n-old\n+new\n"
	valid := []byte(fmt.Sprintf(`{
      "fixPlanId":"fix-1","rootCause":"root",
      "changes":[{"file":"src/main/java/A.java","reason":"r","change":"c"}],
      "verification":["verify"],
      "unifiedDiff":%q,
      "diffSha256":"%s",
      "files":[{"path":"src/main/java/A.java","baseSha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}]
    }`, diff, schemaHashText(diff)))
	if err := ValidateJSON(schemaBytes, valid); err != nil {
		t.Fatalf("valid fix plan rejected: %v", err)
	}
	invalid := []byte(`{"fixPlanId":"fix-1","rootCause":"root","changes":[{"file":"src/main/java/A.java","reason":"r","change":"c"}],"verification":["verify"]}`)
	if err := ValidateJSON(schemaBytes, invalid); err == nil {
		t.Fatal("fix plan without exact patch identity must be rejected")
	}
	mismatch := []byte(strings.Replace(string(valid), schemaHashText(diff), strings.Repeat("a", 64), 1))
	if err := ValidateJSON(schemaBytes, mismatch); err == nil {
		t.Fatal("fix plan with diffSha256 not matching unifiedDiff must be rejected before approval")
	}
}

func TestTestPlanRequiresPatchIdentityOnlyWhenWriting(t *testing.T) {
	schemaBytes, err := os.ReadFile("../../../contracts/test-plan.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	reuse := []byte(`{"targets":[{"controller":"OrderController","endpoint":"POST /orders","strategy":"REUSE_EXISTING","existingTests":[{"className":"OrderIT","path":"src/test/java/OrderIT.java"}],"scenarios":[{"name":"ok","coverageStatus":"COVERED","coveredBy":["OrderIT.ok"]}]}]}`)
	if err := ValidateJSON(schemaBytes, reuse); err != nil {
		t.Fatalf("reuse-only plan must remain write-free and valid: %v", err)
	}
	writingWithoutIdentity := []byte(`{"planId":"test-1","targets":[{"controller":"OrderController","endpoint":"POST /orders","strategy":"CREATE_NEW","existingTests":[],"scenarios":[{"name":"ok","coverageStatus":"MISSING","coveredBy":[],"request":{"method":"POST","path":"/orders"},"expected":{"httpStatus":200}}]}]}`)
	if err := ValidateJSON(schemaBytes, writingWithoutIdentity); err == nil {
		t.Fatal("writing test plan without patch identity must be rejected")
	}
	diff := "--- /dev/null\n+++ b/src/test/java/OrderIT.java\n@@ -0,0 +1 @@\n+class OrderIT {}\n"
	writing := []byte(fmt.Sprintf(`{
      "planId":"test-1","unifiedDiff":%q,"diffSha256":"%s",
      "files":[{"path":"src/test/java/OrderIT.java","baseSha256":"%s"}],
      "targets":[{"controller":"OrderController","endpoint":"POST /orders","strategy":"CREATE_NEW","existingTests":[],"scenarios":[{"name":"ok","coverageStatus":"MISSING","coveredBy":[],"request":{"method":"POST","path":"/orders"},"expected":{"httpStatus":200}}]}]
    }`, diff, schemaHashText(diff), schemaHashText("")))
	if err := ValidateJSON(schemaBytes, writing); err != nil {
		t.Fatalf("valid writing test plan rejected: %v", err)
	}
	mismatch := []byte(strings.Replace(string(writing), schemaHashText(diff), strings.Repeat("c", 64), 1))
	if err := ValidateJSON(schemaBytes, mismatch); err == nil {
		t.Fatal("writing test plan with mismatched diffSha256 must be rejected before approval")
	}
}
