package reviewrun

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func moveReviewFixtureFile180(t *testing.T, root, from, to string) {
	t.Helper()
	src := filepath.Join(root, filepath.FromSlash(from))
	dest := filepath.Join(root, filepath.FromSlash(to))
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(src, dest); err != nil {
		t.Fatal(err)
	}
}

// Controller and Service interface live in the API module, the concrete
// ServiceImpl and Mapper in the implementation module, and SQL XML in a
// third Maven module. A module-local-only scan cannot resolve this chain.
func Test180CrossModuleServiceAndMapperChain(t *testing.T) {
	root := copyControllerReviewFixture180(t)
	useRealAstGrep180(t, root)
	if err := os.MkdirAll(filepath.Join(root, "module-api"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, "src"), filepath.Join(root, "module-api", "src")); err != nil {
		t.Fatal(err)
	}
	moveReviewFixtureFile180(t, root,
		"module-api/src/main/java/com/example/OrderServiceImpl.java",
		"module-impl/src/main/java/com/example/OrderServiceImpl.java")
	moveReviewFixtureFile180(t, root,
		"module-api/src/main/java/com/example/OrderMapper.java",
		"module-impl/src/main/java/com/example/OrderMapper.java")
	moveReviewFixtureFile180(t, root,
		"module-api/src/main/resources/mapper/OrderMapper.xml",
		"module-data/src/main/resources/mapper/OrderMapper.xml")

	start, err := Start(root)
	if err != nil {
		t.Fatal(err)
	}
	intent := Intent{Mode: "CURRENT_IMPLEMENTATION", Target: "OrderController.create"}
	opts, err := Prepare(context.Background(), root, start.RunID, intent)
	if err != nil || !opts.DiscoveryComplete || len(opts.Chains) != 1 {
		t.Fatalf("cross-module review cannot follow service: %+v %v", opts, err)
	}
	chain := opts.Chains[0]
	wantPaths := []string{
		"module-api/src/main/java/com/example/OrderController.java",
		"module-impl/src/main/java/com/example/OrderServiceImpl.java",
		"module-impl/src/main/java/com/example/OrderMapper.java",
		"module-data/src/main/resources/mapper/OrderMapper.xml",
	}
	if len(chain.Nodes) != len(wantPaths) {
		t.Fatalf("cross-module chain lost nodes: %+v", chain)
	}
	for i, want := range wantPaths {
		if chain.Nodes[i].Path != want {
			t.Fatalf("chain node %d has path=%q want=%q", i, chain.Nodes[i].Path, want)
		}
	}
	if len(chain.Unresolved) != 0 {
		t.Fatalf("valid cross-module chain has gaps: %+v", chain)
	}
}

// A mapper XML-only change must affect its API Controller even when the
// Mapper interface and the Controller belong to different source roots.
func Test180CrossModuleMapperXMLOnlyChangeImpact(t *testing.T) {
	root := copyControllerReviewFixture180(t)
	useRealAstGrep180(t, root)
	if err := os.MkdirAll(filepath.Join(root, "module-api"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, "src"), filepath.Join(root, "module-api", "src")); err != nil {
		t.Fatal(err)
	}
	moveReviewFixtureFile180(t, root,
		"module-api/src/main/java/com/example/OrderServiceImpl.java",
		"module-impl/src/main/java/com/example/OrderServiceImpl.java")
	moveReviewFixtureFile180(t, root,
		"module-api/src/main/java/com/example/OrderMapper.java",
		"module-impl/src/main/java/com/example/OrderMapper.java")
	moveReviewFixtureFile180(t, root,
		"module-api/src/main/resources/mapper/OrderMapper.xml",
		"module-data/src/main/resources/mapper/OrderMapper.xml")
	initControllerReviewGitBaseline180(t, root)
	xml := "module-data/src/main/resources/mapper/OrderMapper.xml"
	original, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(xml)))
	if err != nil {
		t.Fatal(err)
	}
	modified := strings.Replace(string(original), "VALUES (1)", "VALUES (12)", 1)
	if modified == string(original) {
		t.Fatal("fixture XML mutation did not change a statement")
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(xml)), []byte(modified), 0o600); err != nil {
		t.Fatal(err)
	}
	start, err := Start(root)
	if err != nil {
		t.Fatal(err)
	}
	opts, err := Prepare(context.Background(), root, start.RunID, Intent{Mode: "CHANGES", Target: "OrderController.create"})
	if err != nil || !opts.DiscoveryComplete || len(opts.Chains) != 1 {
		t.Fatalf("cross-module XML-only change was lost: %+v %v", opts, err)
	}
	found := false
	for _, node := range opts.Chains[0].Nodes {
		if node.Role == "SQL" && node.Path == xml {
			found = true
		}
	}
	if !found {
		t.Fatalf("cross-module SQL node missing from affected chain: %+v", opts.Chains[0])
	}
}
