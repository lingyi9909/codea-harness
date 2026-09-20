package reviewscope

import (
	"encoding/json"
	"testing"
)

func Test152TargetedReviewUsesWorkspaceChainContextWithoutRequiringDependencyFile(t *testing.T) {
	analysis := workspaceReviewAnalysis152(t)
	selection := []byte(`{
  "mode":"TARGETED",
  "target":{"symbol":"XxxController.submit","kind":"METHOD"},
  "selectedCallChains":[{
    "entryPoint":"XxxController.submit",
    "chain":["XxxController.submit","XxxServiceImpl.submit","AbstractTemplate.execute","XxxServiceImpl.doExecute","XxxMapper.updateStatus"]
  }],
  "scopedFiles":[
    "src/main/java/com/company/XxxController.java",
    "src/main/java/com/company/XxxServiceImpl.java",
    "src/main/java/com/company/XxxMapper.java"
  ]
}`)

	verified, err := Verify(selection, analysis)
	if err != nil {
		t.Fatalf("dependency workspace node is navigation context, not required review file: %v", err)
	}
	for _, path := range verified.ScopedFiles {
		if path == "src/main/java/com/company/framework/AbstractTemplate.java" {
			t.Fatalf("dependency workspace path must never enter scopedFiles: %#v", verified.ScopedFiles)
		}
	}
}

func Test152TargetedReviewRejectsDependencyWorkspacePathInScopedFiles(t *testing.T) {
	analysis := workspaceReviewAnalysis152(t)
	selection := []byte(`{
  "mode":"TARGETED",
  "target":{"symbol":"XxxController.submit","kind":"METHOD"},
  "selectedCallChains":[{
    "entryPoint":"XxxController.submit",
    "chain":["XxxController.submit","XxxServiceImpl.submit","AbstractTemplate.execute","XxxServiceImpl.doExecute","XxxMapper.updateStatus"]
  }],
  "scopedFiles":[
    "src/main/java/com/company/XxxController.java",
    "src/main/java/com/company/XxxServiceImpl.java",
    "src/main/java/com/company/XxxMapper.java",
    "src/main/java/com/company/framework/AbstractTemplate.java"
  ]
}`)

	if _, err := Verify(selection, analysis); err == nil {
		t.Fatal("dependency workspace path must be rejected from TARGETED scopedFiles")
	}
}

func workspaceReviewAnalysis152(t *testing.T) []byte {
	t.Helper()
	value := map[string]any{
		"changedFiles": []any{
			map[string]any{"path": "src/main/java/com/company/XxxController.java", "role": "Controller"},
			map[string]any{"path": "src/main/java/com/company/XxxServiceImpl.java", "role": "Service"},
			map[string]any{"path": "src/main/java/com/company/XxxMapper.java", "role": "Mapper"},
		},
		"callChains": []any{map[string]any{
			"entryPoint": "XxxController.submit",
			"chain": []string{"XxxController.submit", "XxxServiceImpl.submit", "AbstractTemplate.execute", "XxxServiceImpl.doExecute", "XxxMapper.updateStatus"},
		}},
		"symbolLocations": []any{
			map[string]any{"workspace": "current", "symbol": "XxxController.submit", "path": "src/main/java/com/company/XxxController.java", "role": "Controller", "source": "FIND_SYMBOL"},
			map[string]any{"workspace": "current", "symbol": "XxxServiceImpl.submit", "path": "src/main/java/com/company/XxxServiceImpl.java", "role": "Service", "source": "FIND_SYMBOL"},
			map[string]any{"workspace": "company-framework", "symbol": "AbstractTemplate.execute", "path": "src/main/java/com/company/framework/AbstractTemplate.java", "role": "Service", "source": "WORKSPACE_INHERITANCE", "from": "XxxServiceImpl.submit"},
			map[string]any{"workspace": "current", "symbol": "XxxServiceImpl.doExecute", "path": "src/main/java/com/company/XxxServiceImpl.java", "role": "Service", "source": "WORKSPACE_INHERITANCE", "from": "AbstractTemplate.execute"},
			map[string]any{"workspace": "current", "symbol": "XxxMapper.updateStatus", "path": "src/main/java/com/company/XxxMapper.java", "role": "Mapper", "source": "FIND_SYMBOL", "from": "XxxServiceImpl.doExecute"},
		},
		"resourceRelations": []any{},
		"reviewCoverage": map[string]any{
			"reviewedFiles": []any{
				map[string]any{"path": "src/main/java/com/company/XxxController.java", "role": "Controller"},
				map[string]any{"path": "src/main/java/com/company/XxxServiceImpl.java", "role": "Service"},
				map[string]any{"path": "src/main/java/com/company/XxxMapper.java", "role": "Mapper"},
			},
			"unresolvedSymbols": []any{},
		},
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
