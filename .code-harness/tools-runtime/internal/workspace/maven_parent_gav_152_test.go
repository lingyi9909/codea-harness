package workspace

import (
	"path/filepath"
	"testing"
)

func Test152LocalParentGAVMustMatchDeclaredParentBeforeInheritance(t *testing.T) {
	parent := t.TempDir()
	repo := filepath.Join(parent, "order-service")
	dep := filepath.Join(parent, "company-framework")
	mustMkdir(t, repo)
	mustMkdir(t, dep)

	// The child declares real-parent:9.0.0, but relativePath points at a
	// different local POM that happens to provide the exact dependency version.
	// Those facts must never be inherited from the wrong parent.
	writePOM(t, filepath.Join(parent, "parent-pom.xml"), `<?xml version="1.0"?>
<project><modelVersion>4.0.0</modelVersion>
  <groupId>com.company</groupId>
  <artifactId>wrong-parent</artifactId>
  <version>1.0.0</version>
  <properties><framework.version>2.3.1</framework.version></properties>
</project>`)
	writePOM(t, filepath.Join(repo, "pom.xml"), `<?xml version="1.0"?>
<project><modelVersion>4.0.0</modelVersion>
  <parent>
    <groupId>com.company</groupId>
    <artifactId>real-parent</artifactId>
    <version>9.0.0</version>
    <relativePath>../parent-pom.xml</relativePath>
  </parent>
  <artifactId>order-service</artifactId>
  <dependencies>
    <dependency>
      <groupId>com.company</groupId>
      <artifactId>company-framework</artifactId>
      <version>${framework.version}</version>
    </dependency>
  </dependencies>
</project>`)
	writePOM(t, filepath.Join(dep, "pom.xml"), projectPOM("com.company", "company-framework", "2.3.1", ""))

	got := VerifyDirectMavenDependencies(repo, []Dependency{workspaceDep(dep)})
	if len(got) != 1 {
		t.Fatalf("expected one result, got %#v", got)
	}
	if got[0].Status == StatusVerified || got[0].ConfirmedRoot != "" {
		t.Fatalf("mismatched local parent GAV must never produce VERIFIED source: %#v", got[0])
	}
	if got[0].Status != StatusVersionUnresolved || got[0].Code != CodeVersionUnresolved {
		t.Fatalf("mismatched local parent facts must be unusable and leave version unresolved: %#v", got[0])
	}
}
