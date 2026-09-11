package schema

import (
	"os"
	"testing"
)

func Test170ReviewContextRequestSchema(t *testing.T) {
	schemaBytes, err := os.ReadFile("../../../contracts/review-context-request.schema.json")
	if err != nil { t.Fatal(err) }
	for _, valid := range []string{
		`{"runId":"review-case","phase":"DISCOVERY"}`,
		`{"runId":"review-case","phase":"RULES"}`,
	} {
		if err := ValidateJSON(schemaBytes, []byte(valid)); err != nil { t.Fatalf("valid request rejected: %v", err) }
	}
	for _, invalid := range []string{
		`{"runId":"review-case","phase":"OTHER"}`,
		`{"runId":"review-case","phase":"DISCOVERY","budget":{"maxFiles":1}}`,
		`{"phase":"DISCOVERY"}`,
	} {
		if err := ValidateJSON(schemaBytes, []byte(invalid)); err == nil { t.Fatalf("invalid request accepted: %s", invalid) }
	}
}

func Test170ReviewContextResponseSchema(t *testing.T) {
	schemaBytes, err := os.ReadFile("../../../contracts/review-context.schema.json")
	if err != nil { t.Fatal(err) }
	valid := `{
		"runId":"review-case","phase":"DISCOVERY",
		"relations":[{"id":"r1","kind":"JAVA_CALL","resolution":"EXACT",
			"from":{"workspace":"current","path":"src/main/java/a/Pay.java","side":"CURRENT","kind":"METHOD","ownerFqcn":"a.Pay","name":"submit","parameterTypes":[]},
			"targets":[{"workspace":"current","path":"src/main/java/a/Risk.java","side":"CURRENT","kind":"METHOD","ownerFqcn":"a.Risk","name":"check","parameterTypes":[]}],
			"evidence":[{"ref":{"workspace":"current","path":"src/main/java/a/Pay.java","side":"CURRENT","kind":"METHOD","ownerFqcn":"a.Pay","name":"submit","parameterTypes":[]},"startLine":10,"endLine":10,"startColumn":5,"endColumn":17}],
			"reason":"","assumptions":[]}],
		"checks":[],"issues":[],
		"usage":{"files":1,"candidates":1,"sourceBytes":100,"elapsedMillis":2}
	}`
	if err := ValidateJSON(schemaBytes, []byte(valid)); err != nil { t.Fatalf("valid response rejected: %v", err) }
	invalid := `{"runId":"review-case","phase":"DISCOVERY","relations":[],"checks":[],"issues":[],"usage":{"files":0,"candidates":0,"sourceBytes":0,"elapsedMillis":0},"unknown":true}`
	if err := ValidateJSON(schemaBytes, []byte(invalid)); err == nil { t.Fatal("response schema accepted unknown field") }
}
