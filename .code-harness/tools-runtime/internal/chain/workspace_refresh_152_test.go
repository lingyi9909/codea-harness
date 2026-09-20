package chain

import "testing"

func Test152RefreshDetectsWorkspaceOnlyNodeChange(t *testing.T) {
	existing := Chain{
		Version: 1,
		ID:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Name:    "XxxController.submit",
		Status:  StatusAccepted,
		Nodes: []Node{{
			Workspace: CurrentWorkspace,
			Symbol:    "Foo.execute",
			Path:      "src/main/java/Foo.java",
			Role:      "SERVICE",
		}},
	}
	discovered := existing
	discovered.Status = StatusDiscovered
	discovered.Nodes = []Node{{
		Workspace: "company-framework",
		Symbol:    "Foo.execute",
		Path:      "src/main/java/Foo.java",
		Role:      "SERVICE",
	}}

	oldFacts := chainFactSet(existing)
	newFacts := chainFactSet(discovered)
	if len(oldFacts) != 1 || len(newFacts) != 1 {
		t.Fatalf("unexpected fact sets: old=%#v new=%#v", oldFacts, newFacts)
	}
	for fact := range oldFacts {
		if newFacts[fact] {
			t.Fatalf("workspace-only node change must alter refresh identity: %q", fact)
		}
	}
}
