package schema

import (
	"os"
	"testing"
)

func Test152ChainSchemaAcceptsWorkspaceIdentity(t *testing.T) {
	schemaBytes, err := os.ReadFile("../../../contracts/chain.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	chain := []byte(`{
  "version":1,
  "id":"chain-152",
  "name":"XxxController.submit",
  "status":"DISCOVERED",
  "entryPoints":[{"workspace":"current","symbol":"XxxController.submit","path":"src/main/java/XxxController.java"}],
  "nodes":[
    {"workspace":"current","symbol":"XxxServiceImpl.submit","path":"src/main/java/XxxServiceImpl.java","role":"SERVICE"},
    {"workspace":"company-framework","symbol":"AbstractTemplate.execute","path":"src/main/java/AbstractTemplate.java","role":"SERVICE"}
  ]
}`)
	if err := ValidateJSON(schemaBytes, chain); err != nil {
		t.Fatalf("workspace-aware chain must validate: %v", err)
	}
}

func Test152LegacyChainSchemaWithoutWorkspaceRemainsValid(t *testing.T) {
	schemaBytes, err := os.ReadFile("../../../contracts/chain.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	legacy := []byte(`{
  "version":1,
  "id":"legacy-chain",
  "name":"legacy",
  "status":"ACCEPTED",
  "entryPoints":[{"symbol":"XxxController.submit","path":"src/main/java/XxxController.java"}],
  "nodes":[{"symbol":"XxxServiceImpl.submit","path":"src/main/java/XxxServiceImpl.java","role":"SERVICE"}]
}`)
	if err := ValidateJSON(schemaBytes, legacy); err != nil {
		t.Fatalf("1.5.1 chain without workspace must remain schema-valid: %v", err)
	}
}
