package schema

import (
	"os"
	"strings"
	"testing"
)

func validHarnessConfigYAML(version string) string {
	return `
version: ` + version + `
project: {type: maven, root: ., module: ""}
review: {baseRef: origin/develop, includeWorkingTree: true}
integrationTest:
  executable: ./mvnw
  args: [test]
  reportDir: target/surefire-reports
  timeoutSeconds: 600
service:
  executable: ./mvnw
  args: [spring-boot:run]
  startupTimeoutSeconds: 120
  readiness: {type: log, pattern: Started}
  logFile: null
stopService: {mode: processTree}
initialization: {status: READY, unresolved: []}
scope:
  sourceIncludes: [src/main/java/**/*.java]
  testIncludes: [src/test/java/**/*.java]
  mapperIncludes: [src/main/resources/**/*Mapper.xml]
  configIncludes: [src/main/resources/**/*.yml]
write:
  allowedTestPaths: [src/test/java/**]
  allowedProductionPaths: [src/main/java/**]
  deniedPaths: [.git/**]
runs: {directory: .code-harness/runs}
`
}

func TestHarnessConfigV2RequiresMapperAndYamlReviewScopes(t *testing.T) {
	schemaBytes, err := os.ReadFile("../../../contracts/harness-config.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	valid := []byte(validHarnessConfigYAML("2"))
	if err := ValidateYAML(schemaBytes, valid); err != nil {
		t.Fatalf("1.4 resource review config rejected: %v", err)
	}

	missingMapper := []byte(strings.Replace(string(valid), "  mapperIncludes: [src/main/resources/**/*Mapper.xml]\n", "", 1))
	if err := ValidateYAML(schemaBytes, missingMapper); err == nil {
		t.Fatal("config v2 without mapperIncludes must be rejected")
	}
	missingConfig := []byte(strings.Replace(string(valid), "  configIncludes: [src/main/resources/**/*.yml]\n", "", 1))
	if err := ValidateYAML(schemaBytes, missingConfig); err == nil {
		t.Fatal("config v2 without configIncludes must be rejected")
	}
}

func TestHarnessConfigV1RemainsBackwardCompatibleUntilUpgradeMigration(t *testing.T) {
	schemaBytes, err := os.ReadFile("../../../contracts/harness-config.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	legacy := validHarnessConfigYAML("1")
	legacy = strings.Replace(legacy, "  mapperIncludes: [src/main/resources/**/*Mapper.xml]\n", "", 1)
	legacy = strings.Replace(legacy, "  configIncludes: [src/main/resources/**/*.yml]\n", "", 1)
	if err := ValidateYAML(schemaBytes, []byte(legacy)); err != nil {
		t.Fatalf("legacy config v1 must remain valid before 1.4 migration: %v", err)
	}
}

func TestHarnessTemplateContainsOnlyLockedResourcePatterns(t *testing.T) {
	data, err := os.ReadFile("../../../harness.template.yaml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"version: 2",
		"mapperIncludes:",
		"src/main/resources/**/*Mapper.xml",
		"configIncludes:",
		"src/main/resources/**/*.yml",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("harness template missing %q", want)
		}
	}
	for _, forbidden := range []string{"**/*.properties", "pom.xml", "build.gradle", "db/migration", "liquibase"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("Task 2 scope expanded unexpectedly with %q", forbidden)
		}
	}
}
