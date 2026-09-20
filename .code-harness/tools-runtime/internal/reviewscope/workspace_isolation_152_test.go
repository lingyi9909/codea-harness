package reviewscope

import "testing"

func Test152ReviewIsolationRetainsDependencyChainButNotScopedFile(t *testing.T) {
	analysis := workspaceReviewAnalysis152(t)
	selection := []byte(`{
		"mode":"TARGETED",
		"target":{"symbol":"XxxController.submit","kind":"METHOD"},
		"selectedCallChains":[{"entryPoint":"XxxController.submit","chain":["XxxController.submit","XxxServiceImpl.submit","AbstractTemplate.execute","XxxServiceImpl.doExecute","XxxMapper.updateStatus"]}],
		"scopedFiles":["src/main/java/com/company/XxxController.java","src/main/java/com/company/XxxServiceImpl.java","src/main/java/com/company/XxxMapper.java"]
	}`)

	verified, err := Verify(selection, analysis)
	if err != nil {
		t.Fatal(err)
	}
	if len(verified.SelectedCallChains) != 1 {
		t.Fatalf("dependency-aware business chain must remain selected: %+v", verified.SelectedCallChains)
	}
	for _, file := range verified.ScopedFiles {
		if file == "src/main/java/com/company/framework/AbstractTemplate.java" {
			t.Fatalf("dependency workspace leaked into scopedFiles: %+v", verified.ScopedFiles)
		}
	}
}
