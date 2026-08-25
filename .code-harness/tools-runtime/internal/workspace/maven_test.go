package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyDirectMavenDependencyLiteralVersion(t *testing.T) {
	repo, dep := newMavenWorkspace(t)
	writePOM(t, filepath.Join(repo, "pom.xml"), projectPOM("com.company", "order-service", "1.0.0", `
  <dependencies>
    <dependency><groupId>com.company</groupId><artifactId>company-framework</artifactId><version>2.3.1</version></dependency>
  </dependencies>`))
	writePOM(t, filepath.Join(dep, "pom.xml"), projectPOM("com.company", "company-framework", "2.3.1", ""))

	got := VerifyDirectMavenDependencies(repo, []Dependency{workspaceDep(dep)})
	assertVerification(t, got, StatusVerified, "", "2.3.1", "2.3.1")
}

func TestVerifyDirectMavenDependencyPropertyVersion(t *testing.T) {
	repo, dep := newMavenWorkspace(t)
	writePOM(t, filepath.Join(repo, "pom.xml"), projectPOMWithBody("com.company", "order-service", "1.0.0", `
  <properties><framework.version>2.3.1</framework.version></properties>
  <dependencies>
    <dependency><groupId>com.company</groupId><artifactId>company-framework</artifactId><version>${framework.version}</version></dependency>
  </dependencies>`))
	writePOM(t, filepath.Join(dep, "pom.xml"), projectPOMWithBody("com.company", "company-framework", "${revision}", `
  <properties><revision>2.3.1</revision></properties>`))

	got := VerifyDirectMavenDependencies(repo, []Dependency{workspaceDep(dep)})
	assertVerification(t, got, StatusVerified, "", "2.3.1", "2.3.1")
}

func TestVerifyDirectMavenDependencyLocalParent(t *testing.T) {
	parent := t.TempDir()
	repo := filepath.Join(parent, "order-service")
	dep := filepath.Join(parent, "company-framework")
	mustMkdir(t, repo)
	mustMkdir(t, dep)

	writePOM(t, filepath.Join(parent, "parent-pom.xml"), `<?xml version="1.0"?>
<project><modelVersion>4.0.0</modelVersion><groupId>com.company</groupId><artifactId>company-parent</artifactId><version>9.0.0</version><properties><framework.version>2.3.1</framework.version></properties></project>`)
	writePOM(t, filepath.Join(repo, "pom.xml"), `<?xml version="1.0"?>
<project><modelVersion>4.0.0</modelVersion>
  <parent><groupId>com.company</groupId><artifactId>company-parent</artifactId><version>9.0.0</version><relativePath>../parent-pom.xml</relativePath></parent>
  <artifactId>order-service</artifactId>
  <dependencies><dependency><groupId>com.company</groupId><artifactId>company-framework</artifactId><version>${framework.version}</version></dependency></dependencies>
</project>`)
	writePOM(t, filepath.Join(dep, "pom.xml"), projectPOM("com.company", "company-framework", "2.3.1", ""))

	got := VerifyDirectMavenDependencies(repo, []Dependency{workspaceDep(dep)})
	assertVerification(t, got, StatusVerified, "", "2.3.1", "2.3.1")
}

func TestVerifyDirectMavenDependencyLocalDependencyManagement(t *testing.T) {
	repo, dep := newMavenWorkspace(t)
	writePOM(t, filepath.Join(repo, "pom.xml"), projectPOMWithBody("com.company", "order-service", "1.0.0", `
  <properties><framework.version>2.3.1</framework.version></properties>
  <dependencyManagement><dependencies>
    <dependency><groupId>com.company</groupId><artifactId>company-framework</artifactId><version>${framework.version}</version></dependency>
  </dependencies></dependencyManagement>
  <dependencies>
    <dependency><groupId>com.company</groupId><artifactId>company-framework</artifactId></dependency>
  </dependencies>`))
	writePOM(t, filepath.Join(dep, "pom.xml"), projectPOM("com.company", "company-framework", "2.3.1", ""))

	got := VerifyDirectMavenDependencies(repo, []Dependency{workspaceDep(dep)})
	assertVerification(t, got, StatusVerified, "", "2.3.1", "2.3.1")
}

func TestVerifyDirectMavenDependencyRejectsWrongOrUnresolvedSource(t *testing.T) {
	cases := []struct {
		name       string
		currentPOM string
		sourcePOM  string
		status     VerificationStatus
		code       string
	}{
		{
			name: "coordinate mismatch",
			currentPOM: projectPOM("com.company", "order-service", "1.0.0", `
  <dependencies><dependency><groupId>com.company</groupId><artifactId>company-framework</artifactId><version>2.3.1</version></dependency></dependencies>`),
			sourcePOM: projectPOM("com.company", "other-framework", "2.3.1", ""),
			status: StatusCoordinateMismatch,
			code: "WORKSPACE_DEPENDENCY_COORDINATE_MISMATCH",
		},
		{
			name: "version mismatch",
			currentPOM: projectPOM("com.company", "order-service", "1.0.0", `
  <dependencies><dependency><groupId>com.company</groupId><artifactId>company-framework</artifactId><version>2.3.1</version></dependency></dependencies>`),
			sourcePOM: projectPOM("com.company", "company-framework", "2.5.0-SNAPSHOT", ""),
			status: StatusVersionMismatch,
			code: "WORKSPACE_DEPENDENCY_VERSION_MISMATCH",
		},
		{
			name: "version unresolved",
			currentPOM: projectPOM("com.company", "order-service", "1.0.0", `
  <dependencies><dependency><groupId>com.company</groupId><artifactId>company-framework</artifactId><version>${unknown.version}</version></dependency></dependencies>`),
			sourcePOM: projectPOM("com.company", "company-framework", "2.3.1", ""),
			status: StatusVersionUnresolved,
			code: "WORKSPACE_DEPENDENCY_VERSION_UNRESOLVED",
		},
		{
			name: "not direct dependency",
			currentPOM: projectPOM("com.company", "order-service", "1.0.0", ""),
			sourcePOM: projectPOM("com.company", "company-framework", "2.3.1", ""),
			status: StatusCoordinateMismatch,
			code: "WORKSPACE_DEPENDENCY_COORDINATE_MISMATCH",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, dep := newMavenWorkspace(t)
			writePOM(t, filepath.Join(repo, "pom.xml"), tc.currentPOM)
			writePOM(t, filepath.Join(dep, "pom.xml"), tc.sourcePOM)
			got := VerifyDirectMavenDependencies(repo, []Dependency{workspaceDep(dep)})
			assertVerification(t, got, tc.status, tc.code, "", "")
			if got[0].ConfirmedRoot != "" {
				t.Fatalf("non-VERIFIED source must not expose confirmed root: %#v", got[0])
			}
		})
	}
}

func TestVerifyDirectMavenDependencySourceNotFound(t *testing.T) {
	repo, dep := newMavenWorkspace(t)
	writePOM(t, filepath.Join(repo, "pom.xml"), projectPOM("com.company", "order-service", "1.0.0", `
  <dependencies><dependency><groupId>com.company</groupId><artifactId>company-framework</artifactId><version>2.3.1</version></dependency></dependencies>`))
	if err := os.RemoveAll(dep); err != nil {
		t.Fatal(err)
	}
	got := VerifyDirectMavenDependencies(repo, []Dependency{workspaceDep(dep)})
	assertVerification(t, got, StatusSourceNotFound, "WORKSPACE_DEPENDENCY_SOURCE_NOT_FOUND", "", "")
}

func newMavenWorkspace(t *testing.T) (string, string) {
	t.Helper()
	parent := t.TempDir()
	repo := filepath.Join(parent, "order-service")
	dep := filepath.Join(parent, "company-framework")
	mustMkdir(t, repo)
	mustMkdir(t, dep)
	return repo, dep
}

func workspaceDep(root string) Dependency {
	return Dependency{
		ID: "company-framework",
		Root: "../company-framework",
		ResolvedRoot: root,
		Maven: MavenCoordinate{GroupID: "com.company", ArtifactID: "company-framework"},
		Mode: ModeReadOnly,
	}
}

func writePOM(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func projectPOM(groupID, artifactID, version, body string) string {
	return projectPOMWithBody(groupID, artifactID, version, body)
}

func projectPOMWithBody(groupID, artifactID, version, body string) string {
	return `<?xml version="1.0"?>
<project><modelVersion>4.0.0</modelVersion><groupId>` + groupID + `</groupId><artifactId>` + artifactID + `</artifactId><version>` + version + `</version>` + body + `</project>`
}

func assertVerification(t *testing.T, got []VerificationResult, status VerificationStatus, code, currentVersion, sourceVersion string) {
	t.Helper()
	if len(got) != 1 {
		t.Fatalf("expected one result, got %#v", got)
	}
	if got[0].Status != status || got[0].Code != code {
		t.Fatalf("unexpected verification result: %#v", got[0])
	}
	if currentVersion != "" && got[0].CurrentVersion != currentVersion {
		t.Fatalf("current version=%q want %q", got[0].CurrentVersion, currentVersion)
	}
	if sourceVersion != "" && got[0].SourceVersion != sourceVersion {
		t.Fatalf("source version=%q want %q", got[0].SourceVersion, sourceVersion)
	}
	if status == StatusVerified && got[0].ConfirmedRoot == "" {
		t.Fatalf("VERIFIED source must expose confirmed root: %#v", got[0])
	}
}
