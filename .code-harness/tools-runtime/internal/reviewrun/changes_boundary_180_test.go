package reviewrun

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A changed ambiguous implementation must not disappear beside a complete chain.
func Test180ChangesKeepsAmbiguousEntrypointBesideCompleteChain(t *testing.T) {
	root := copyControllerReviewFixture180(t)
	useRealAstGrep180(t, root)
	java := "src/main/java/com/example/"
	for _, name := range []string{"Controller", "Service", "ServiceImpl", "Mapper"} {
		b, err := os.ReadFile(filepath.Join(root, java+"Order"+name+".java"))
		if err != nil {
			t.Fatal(err)
		}
		writeBoundarySource180(t, root, java+"Payment"+name+".java", strings.ReplaceAll(strings.ReplaceAll(string(b), "Order", "Payment"), "order", "payment"))
	}
	b, err := os.ReadFile(filepath.Join(root, "src/main/resources/mapper/OrderMapper.xml"))
	if err != nil {
		t.Fatal(err)
	}
	writeBoundarySource180(t, root, "src/main/resources/mapper/PaymentMapper.xml", strings.ReplaceAll(strings.ReplaceAll(string(b), "Order", "Payment"), "order", "payment"))
	writeBoundarySource180(t, root, java+"BackupOrderServiceImpl.java", `package com.example;
public class BackupOrderServiceImpl implements java.io.Serializable, OrderService {
    public void create() {}
    public void cancel() {}
}
`)
	// Keep one endpoint per controller so dropping Order would manufacture AUTO_SINGLE.
	for _, prefix := range []string{"Order", "Payment"} {
		path := java + prefix + "Controller.java"
		b, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		text := string(b)
		cut := strings.Index(text, "    @PostMapping(\"/"+strings.ToLower(prefix)+"s/cancel\")")
		if cut < 0 {
			t.Fatal("fixture cancel endpoint not found")
		}
		writeBoundarySource180(t, root, path, text[:cut]+"}\n")
	}
	initControllerReviewGitBaseline180(t, root)
	for _, name := range []string{"OrderServiceImpl.java", "PaymentServiceImpl.java"} {
		b, err := os.ReadFile(filepath.Join(root, java+name))
		if err != nil {
			t.Fatal(err)
		}
		writeBoundarySource180(t, root, java+name, string(b)+"\n// changed implementation\n")
	}
	started, err := Start(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Prepare(context.Background(), root, started.RunID, Intent{Mode: "CHANGES"})
	if err != nil {
		t.Fatal(err)
	}
	if got.DiscoveryComplete || !got.SelectionRequired || len(got.Chains) != 2 {
		t.Fatalf("changed ambiguous entrypoints were lost: %+v", got)
	}
	if !strings.Contains(strings.Join(got.Gaps, "\n"), "OrderService implementations=2") {
		t.Fatalf("lost implementation gap: %+v", got)
	}
	_, state, err := loadRun(root, started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if state.ScopeReady || len(state.SelectedIDs) != 0 {
		t.Fatalf("filtered discovery authorized scope: %+v", state)
	}
}

func Test180ChangesMapperXMLParticipatesInImpact(t *testing.T) {
	for _, tc := range []struct {
		name, target, replacement string
		complete                  bool
		count                     int
	}{
		{"targeted SQL change", "OrderController.create", "VALUES (2)", true, 1},
		{"unfiltered SQL change", "", "VALUES (2)", true, 2},
		{"broken namespace retains gap", "OrderController.create", "namespace", false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := copyControllerReviewFixture180(t)
			useRealAstGrep180(t, root)
			initControllerReviewGitBaseline180(t, root)
			path := "src/main/resources/mapper/OrderMapper.xml"
			b, err := os.ReadFile(filepath.Join(root, path))
			if err != nil {
				t.Fatal(err)
			}
			changed := strings.Replace(string(b), "VALUES (1)", tc.replacement, 1)
			if !tc.complete {
				changed = strings.Replace(string(b), "com.example.OrderMapper", "com.unrelated.OrderMapper", 1)
			}
			writeBoundarySource180(t, root, path, changed)
			started, err := Start(root)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Prepare(context.Background(), root, started.RunID, Intent{Mode: "CHANGES", Target: tc.target})
			if err != nil {
				t.Fatalf("XML-only change was rejected: %v", err)
			}
			if got.DiscoveryComplete != tc.complete || len(got.Chains) != tc.count {
				t.Fatalf("XML impact lost or falsely complete: %+v", got)
			}
			_, state, err := loadRun(root, started.RunID)
			if err != nil {
				t.Fatal(err)
			}
			if state.ScopeReady != (tc.complete && tc.count == 1) {
				t.Fatalf("wrong XML scope authority: %+v", state)
			}
			for _, ch := range got.Chains {
				foundSQL := false
				for _, node := range ch.Nodes {
					if node.Role == "SQL" && node.Path == path {
						foundSQL = true
					}
				}
				if foundSQL != tc.complete {
					t.Fatalf("wrong SQL association: %+v", ch)
				}
				if !tc.complete && len(ch.Unresolved) == 0 {
					t.Fatalf("missing XML gap: %+v", ch)
				}
			}
		})
	}
}

func writeBoundarySource180(t *testing.T, root, path, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
