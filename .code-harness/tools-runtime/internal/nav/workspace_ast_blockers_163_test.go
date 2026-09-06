package nav

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func Test163WorkspaceASTUnicodeEscapeAmbiguityUsesFullASTAuthority(t *testing.T) {
	astPath := os.Getenv("CODEA_AST_GREP_TEST_PATH")
	if astPath == "" {
		t.Skip("real ast-grep path not configured; Task 1 Windows gate supplies it")
	}
	abs, err := filepath.Abs(astPath)
	if err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	writeJava(t, root, "src/main/java/com/company/A.java", `package com.company;
class TargetService extends BaseService {
    void target() {}
}
`)
	writeJava(t, root, "src/main/java/com/company/B.java", `package com.company;
class \u0054argetService extends BaseService {
    void target() {}
}
`)

	n := Navigator{RepoRoot: root, AstGrepPath: abs}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	inventoryRaw, err := n.runWorkspaceRaw(ctx, workspaceClassPatterns("$C", true)...)
	if err != nil {
		t.Fatalf("full AST fixture inventory failed: %v", err)
	}
	confirmed := map[string]bool{}
	for _, match := range inventoryRaw {
		name := strings.TrimSpace(match.Meta["C"])
		if name == "TargetService" || name == `\u0054argetService` {
			confirmed[match.Path] = true
		}
	}
	if len(confirmed) != 2 {
		t.Fatalf("fixture must expose both declarations through AST wildcard inventory, confirmed=%v raw=%#v", confirmed, inventoryRaw)
	}

	t.Run("superclass", func(t *testing.T) {
		_, err := n.WorkspaceSuperclass(ctx, "TargetService")
		if !errors.Is(err, ErrAmbiguousSymbol) {
			t.Fatalf("WorkspaceSuperclass must preserve AST ambiguity across prefilter misses, got %v", err)
		}
	})

	t.Run("method", func(t *testing.T) {
		_, err := n.WorkspaceMethod(ctx, "TargetService", "target")
		if !errors.Is(err, ErrAmbiguousSymbol) {
			t.Fatalf("WorkspaceMethod must preserve AST ambiguity across prefilter misses, got %v", err)
		}
	})
}

func Test163WorkspaceASTResolverLargeClassWithinProductionBudget(t *testing.T) {
	astPath := os.Getenv("CODEA_AST_GREP_TEST_PATH")
	if astPath == "" {
		t.Skip("real ast-grep path not configured; Task 1 Windows gate supplies it")
	}

	current, dependency := workspaceInheritanceFixture(t)
	var java strings.Builder
	java.WriteString("package com.company.order;\n")
	java.WriteString("import com.company.framework.AbstractTemplate;\n")
	java.WriteString("public class XxxServiceImpl extends AbstractTemplate {\n")
	java.WriteString("    public void submit() { execute(); }\n")
	for i := 0; i < 1800; i++ {
		fmt.Fprintf(&java, "    public void filler%04d() { int value = %d; }\n", i, i)
	}
	java.WriteString("    @Override protected void doExecute() { mapper.updateStatus(); }\n")
	java.WriteString("}\n")
	if java.Len() <= 64*1024 {
		t.Fatalf("large class fixture unexpectedly small: %d bytes", java.Len())
	}
	writeJava(t, current, "src/main/java/com/company/order/XxxServiceImpl.java", java.String())
	for i := 0; i < 250; i++ {
		writeJava(t, current, fmt.Sprintf("src/main/java/com/company/noise/Noise%03d.java", i), fmt.Sprintf(`package com.company.noise;
public class Noise%03d {
    public void noop() {}
}
`, i))
	}

	resolver := workspaceResolverForTest(t, current, verifiedWorkspace(dependency))
	started := time.Now()
	result := resolver.ResolveInheritedCall("XxxServiceImpl.submit", "execute")
	elapsed := time.Since(started)

	if elapsed >= 30*time.Second {
		t.Fatalf("production 30s Runtime budget exceeded: elapsed=%s result=%#v", elapsed, result)
	}
	assertWorkspaceFact(t, result, "company-framework", "AbstractTemplate.execute", "src/main/java/com/company/framework/AbstractTemplate.java", "XxxServiceImpl.submit")
}
