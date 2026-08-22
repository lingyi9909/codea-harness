package schema

import (
	"os"
	"testing"
)

func TestFixPlanRequiresExactPatchIdentity(t *testing.T) {
	schemaBytes, err := os.ReadFile("../../../contracts/fix-plan.schema.json")
	if err != nil { t.Fatal(err) }
	valid := []byte(`{
      "fixPlanId":"fix-1","rootCause":"root",
      "changes":[{"file":"src/main/java/A.java","reason":"r","change":"c"}],
      "verification":["verify"],
      "unifiedDiff":"--- a/src/main/java/A.java\n+++ b/src/main/java/A.java\n@@ -1 +1 @@\n-old\n+new\n",
      "diffSha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "files":[{"path":"src/main/java/A.java","baseSha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}]
    }`)
	if err := ValidateJSON(schemaBytes, valid); err != nil { t.Fatalf("valid fix plan rejected: %v",err) }
	invalid := []byte(`{"fixPlanId":"fix-1","rootCause":"root","changes":[{"file":"src/main/java/A.java","reason":"r","change":"c"}],"verification":["verify"]}`)
	if err := ValidateJSON(schemaBytes, invalid); err == nil { t.Fatal("fix plan without exact patch identity must be rejected") }
}

func TestTestPlanRequiresPatchIdentityOnlyWhenWriting(t *testing.T) {
	schemaBytes, err := os.ReadFile("../../../contracts/test-plan.schema.json")
	if err != nil { t.Fatal(err) }
	reuse := []byte(`{"targets":[{"controller":"OrderController","endpoint":"POST /orders","strategy":"REUSE_EXISTING","existingTests":[{"className":"OrderIT","path":"src/test/java/OrderIT.java"}],"scenarios":[{"name":"ok","coverageStatus":"COVERED","coveredBy":["OrderIT.ok"]}]}]}`)
	if err := ValidateJSON(schemaBytes,reuse); err != nil { t.Fatalf("reuse-only plan must remain write-free and valid: %v",err) }
	writingWithoutIdentity := []byte(`{"planId":"test-1","targets":[{"controller":"OrderController","endpoint":"POST /orders","strategy":"CREATE_NEW","existingTests":[],"scenarios":[{"name":"ok","coverageStatus":"MISSING","coveredBy":[],"request":{"method":"POST","path":"/orders"},"expected":{"httpStatus":200}}]}]}`)
	if err := ValidateJSON(schemaBytes,writingWithoutIdentity); err == nil { t.Fatal("writing test plan without patch identity must be rejected") }
}
