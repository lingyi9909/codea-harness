package upgrade

import (
	"strings"
	"testing"
)

func Test164ConfigMigrationRED(t *testing.T) {
	config := strings.Replace(
		validConfig("review:\n  baseRef: origin/develop\n  includeWorkingTree: true\n"),
		"version: 1",
		"version: 2",
		1,
	)
	source, target := makePair(t, config)
	write(t, target, "VERSION", "1.6.3\n")
	write(t, source, "VERSION", "1.6.4\n")

	result := Run(Options{SourceDir: source, TargetDir: target})
	if result.Status != StatusUpgraded {
		t.Fatalf("unexpected pre-fix upgrade result=%+v", result)
	}
	if contains(result.Migrations, "config-1.6.3-to-1.6.4") {
		t.Log("CONFIG_MIGRATION_163_TO_164_REGISTERED PASS")
		return
	}
	t.Fatalf("CONFIG_MIGRATION_163_TO_164_REGISTERED RED: migrations=%v", result.Migrations)
}
