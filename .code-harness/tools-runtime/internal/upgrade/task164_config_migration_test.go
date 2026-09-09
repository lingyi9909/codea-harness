package upgrade

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"codea-harness-tools/internal/schema"
)

const (
	task164Exact163SchemaNormalizedSHA256 = "0a192ce95adb9ff69eeb5c89cabfc8557e17437faa00e094bba7ba2bd49febf2"
	task164Real163Config = `version: 2
project:
  type: maven
  root: .
  module: "payments"
review:
  baseRef: origin/release/1.6.3
  includeWorkingTree: false
integrationTest:
  executable: ./mvnw
  args:
    - test
  reportDir: target/surefire-reports
  timeoutSeconds: 777
service:
  executable: ./mvnw
  args:
    - spring-boot:run
  startupTimeoutSeconds: 123
  readiness:
    type: log
    pattern: Started
  logFile: null
stopService:
  mode: processTree
initialization:
  status: READY
  unresolved: []
scope:
  sourceIncludes:
    - src/main/java/**
  testIncludes:
    - src/test/java/**
  mapperIncludes:
    - src/main/resources/**/*Mapper.xml
  configIncludes:
    - src/main/resources/**/*.yml
write:
  allowedTestPaths:
    - src/test/**
  allowedProductionPaths:
    - src/main/**
  deniedPaths: []
runs:
  directory: .code-harness/runs
`
)

func make163To164Pair(t *testing.T, config string) (string, string) {
	t.Helper()
	source, target := makePair(t, config)
	write(t, target, "VERSION", "1.6.3\n")
	write(t, source, "VERSION", "1.6.4\n")
	return source, target
}

func Test164ConfigMigrationRED(t *testing.T) {
	original := task164Real163Config + "# user-sentinel: preserve-me\n"
	source, target := make163To164Pair(t, original)

	result := Run(Options{SourceDir: source, TargetDir: target})
	if result.Status != StatusUpgraded {
		t.Fatalf("result=%+v", result)
	}
	if !contains(result.Migrations, configMigration163To164) {
		t.Fatalf("CONFIG_MIGRATION_163_TO_164_REGISTERED RED: migrations=%v", result.Migrations)
	}
	got, err := os.ReadFile(filepath.Join(target, "harness.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte(original)) {
		t.Fatalf("user config changed:\n%s", got)
	}
	t.Log("CONFIG_MIGRATION_163_TO_164_REGISTERED PASS")
	t.Log("CONFIG_USER_VALUES_PRESERVED PASS")
}

func Test164ConfigMigrationRunsBeforeTargetSchemaValidation(t *testing.T) {
	source, target := make163To164Pair(t, task164Real163Config)
	write(t, source, "contracts/harness-config.schema.json", `{"type":"object","required":["mustNotExist"]}`)

	result := Run(Options{SourceDir: source, TargetDir: target})
	if result.Status != StatusUpgradeFailed || !result.RollbackPerformed {
		t.Fatalf("result=%+v", result)
	}
	if !contains(result.Migrations, configMigration163To164) {
		t.Fatalf("migration was not registered before target-schema failure: %+v", result)
	}
	if len(result.Errors) == 0 || !strings.Contains(result.Errors[0], "harness.yaml incompatible with new schema") {
		t.Fatalf("unexpected target-schema failure: %+v", result.Errors)
	}
	version, err := os.ReadFile(filepath.Join(target, "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(version)) != "1.6.3" {
		t.Fatalf("target was partially upgraded: VERSION=%q", version)
	}
	t.Log("CONFIG_MIGRATION_BEFORE_TARGET_SCHEMA_VALIDATION PASS")
}

func Test164ConfigMigrationTargetSchemaValidAndIdempotent(t *testing.T) {
	first, err := migrateConfig163To164([]byte(task164Real163Config))
	if err != nil {
		t.Fatal(err)
	}
	second, err := migrateConfig163To164(first)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) || !bytes.Equal(first, []byte(task164Real163Config)) {
		t.Fatal("1.6.3 -> 1.6.4 compatibility migration is not byte-idempotent")
	}

	schemaBytes := task164CurrentSchema(t)
	if err := schema.ValidateYAML(schemaBytes, first); err != nil {
		t.Fatalf("migrated config rejected by 1.6.4 target schema: %v", err)
	}
	t.Log("CONFIG_MIGRATION_TARGET_SCHEMA_VALID PASS")
	t.Log("CONFIG_MIGRATION_IDEMPOTENT PASS")
}

func Test164ConfigMigrationRetainsLegacyVersion1Path(t *testing.T) {
	legacy := validConfig("review:\n  baseRef: origin/develop\n  includeWorkingTree: true\n")
	source, target := make163To164Pair(t, legacy)

	result := Run(Options{SourceDir: source, TargetDir: target})
	if result.Status != StatusUpgraded {
		t.Fatalf("result=%+v", result)
	}
	if !contains(result.Migrations, configMigration163To164) || !contains(result.Migrations, "upgrade-config-v1-to-v2-resource-scopes") {
		t.Fatalf("legacy 1.6.3 config did not retain migration chain: %v", result.Migrations)
	}
	got, err := os.ReadFile(filepath.Join(target, "harness.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	for _, want := range []string{"version: 2", "mapperIncludes:", "configIncludes:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("legacy v1 migration missing %q:\n%s", want, text)
		}
	}
	t.Log("CONFIG_LEGACY_V1_PATH_PRESERVED PASS")
}

func Test164ConfigSchemaCompatibilityGuard(t *testing.T) {
	schemaBytes := task164CurrentSchema(t)
	normalized := bytes.ReplaceAll(schemaBytes, []byte("\r\n"), []byte("\n"))
	actual := fmt.Sprintf("%x", sha256.Sum256(normalized))
	if actual != task164Exact163SchemaNormalizedSHA256 {
		t.Fatalf("harness-config schema changed from exact packaged 1.6.3: got=%s want=%s; add explicit migration or backward-compatibility proof", actual, task164Exact163SchemaNormalizedSHA256)
	}
	t.Logf("CONFIG_SCHEMA_163_164_BACKWARD_COMPATIBLE PASS normalizedSha256=%s", actual)
}

func Test164ConfigMigrationUnsupportedFailsClosed(t *testing.T) {
	if _, _, err := migrateConfigForReleaseEdge(
		[]byte(task164Real163Config),
		configMigration163Source,
		[3]int{1, 6, 5},
	); err == nil || !strings.Contains(err.Error(), "unsupported config migration edge") {
		t.Fatalf("unsupported direct edge did not fail closed: %v", err)
	}

	unsupported := strings.Replace(task164Real163Config, "version: 2", "version: 3", 1)
	source, target := make163To164Pair(t, unsupported)
	before, err := os.ReadFile(filepath.Join(target, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	result := Run(Options{SourceDir: source, TargetDir: target})
	if result.Status != StatusManualActionRequired || result.RollbackPerformed {
		t.Fatalf("result=%+v", result)
	}
	if len(result.Errors) == 0 || !strings.Contains(result.Errors[0], "version must be integer 1 or 2") {
		t.Fatalf("unexpected migration failure: %+v", result.Errors)
	}
	after, err := os.ReadFile(filepath.Join(target, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("failed migration modified installed framework")
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("failed migration consumed source package: %v", err)
	}
	version, err := os.ReadFile(filepath.Join(target, "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(version)) != "1.6.3" {
		t.Fatalf("failed migration changed VERSION=%q", version)
	}
	t.Log("CONFIG_UNSUPPORTED_MIGRATION_FAIL_CLOSED PASS")
}

func task164CurrentSchema(t *testing.T) []byte {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve test file")
	}
	schemaPath := filepath.Clean(filepath.Join(filepath.Dir(file), "../../../contracts/harness-config.schema.json"))
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatal(err)
	}
	return schemaBytes
}
