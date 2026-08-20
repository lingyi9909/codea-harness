package dbconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testSchema = `{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "type":"object",
  "additionalProperties":false,
  "required":["version","enabled","environment","dialect","connection","safety"],
  "properties":{
    "version":{"const":1},
    "enabled":{"type":"boolean"},
    "environment":{"enum":["TEST","LOCAL"]},
    "dialect":{"const":"mysql"},
    "connection":{"type":"object","additionalProperties":false,"required":["host","port","database","username","password","charset"],"properties":{"host":{"type":"string","minLength":1},"port":{"type":"integer","minimum":1,"maximum":65535},"database":{"type":"string","minLength":1},"username":{"type":"string","minLength":1},"password":{"type":"string"},"charset":{"type":"string","minLength":1}}},
    "safety":{"type":"object","additionalProperties":false,"required":["allowedSchemas","maxRows","timeoutSeconds","maxQueriesPerDiagnosis","allowSchemaDiscovery","allowReadonlySql"],"properties":{"allowedSchemas":{"type":"array","minItems":1,"uniqueItems":true,"items":{"type":"string","minLength":1}},"maxRows":{"type":"integer","minimum":1,"maximum":1000},"timeoutSeconds":{"type":"integer","minimum":1,"maximum":30},"maxQueriesPerDiagnosis":{"type":"integer","minimum":1,"maximum":20},"allowSchemaDiscovery":{"type":"boolean"},"allowReadonlySql":{"type":"boolean"}}}
  }
}`

func writeFixture(t *testing.T, yaml string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "database.yaml")
	schemaPath := filepath.Join(dir, "database-config.schema.json")
	if err := os.WriteFile(schemaPath, []byte(testSchema), 0o600); err != nil {
		t.Fatal(err)
	}
	if yaml != "" {
		if err := os.WriteFile(configPath, []byte(yaml), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return configPath, schemaPath
}

func validYAML(environment string) string {
	return `version: 1
enabled: true
environment: ` + environment + `
dialect: mysql
connection:
  host: 127.0.0.1
  port: 3306
  database: order_test
  username: codea_readonly
  password: super-secret-password
  charset: utf8mb4
safety:
  allowedSchemas:
    - order_test
  maxRows: 100
  timeoutSeconds: 10
  maxQueriesPerDiagnosis: 10
  allowSchemaDiscovery: true
  allowReadonlySql: true
`
}

func TestLoadMissingConfigDisablesCapability(t *testing.T) {
	configPath, schemaPath := writeFixture(t, "")
	cfg, err := Load(configPath, schemaPath)
	if err != nil {
		t.Fatalf("missing optional config must not fail harness: %v", err)
	}
	if cfg.Enabled {
		t.Fatal("missing database.yaml must disable database capability")
	}
}

func TestLoadAcceptsTESTAndLOCAL(t *testing.T) {
	for _, env := range []string{"TEST", "LOCAL"} {
		t.Run(env, func(t *testing.T) {
			configPath, schemaPath := writeFixture(t, validYAML(env))
			cfg, err := Load(configPath, schemaPath)
			if err != nil {
				t.Fatal(err)
			}
			if !cfg.Enabled || cfg.Environment != env || cfg.Dialect != "mysql" {
				t.Fatalf("unexpected config: %+v", cfg)
			}
			if cfg.Connection.Password != "super-secret-password" {
				t.Fatal("typed config must retain password for runtime use")
			}
		})
	}
}

func TestLoadRejectsUnsafeOrUnknownConfiguration(t *testing.T) {
	cases := []struct{ name, old, new string }{
		{"production environment", "environment: TEST", "environment: PRODUCTION"},
		{"non mysql dialect", "dialect: mysql", "dialect: postgres"},
		{"empty allowed schemas", "  allowedSchemas:\n    - order_test", "  allowedSchemas: []"},
		{"max rows", "  maxRows: 100", "  maxRows: 1001"},
		{"timeout", "  timeoutSeconds: 10", "  timeoutSeconds: 31"},
		{"query budget", "  maxQueriesPerDiagnosis: 10", "  maxQueriesPerDiagnosis: 21"},
		{"unknown yaml field", "enabled: true", "enabled: true\nunknownField: true"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			yaml := strings.Replace(validYAML("TEST"), tc.old, tc.new, 1)
			configPath, schemaPath := writeFixture(t, yaml)
			if _, err := Load(configPath, schemaPath); err == nil {
				t.Fatal("expected fail closed validation error")
			}
		})
	}
}

func TestLoadErrorNeverLeaksPassword(t *testing.T) {
	yaml := strings.Replace(validYAML("TEST"), "environment: TEST", "environment: PRODUCTION", 1)
	configPath, schemaPath := writeFixture(t, yaml)
	_, err := Load(configPath, schemaPath)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if strings.Contains(err.Error(), "super-secret-password") {
		t.Fatalf("password leaked in error: %v", err)
	}
}

func TestRepositoryDatabaseTemplateLoadsDisabled(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "..", "database.template.yaml"), filepath.Join("..", "..", "..", "contracts", "database-config.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Enabled {
		t.Fatal("distributed database template must default to disabled")
	}
}
