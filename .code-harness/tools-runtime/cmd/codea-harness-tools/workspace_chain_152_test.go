package main

import "testing"

func Test152RealDualProjectWorkspaceBusinessRegression(t *testing.T) {
	result := runWorkspaceBusinessRegression152(t, workspaceBusinessScenario152{})
	if result.Status != "COMPLETE" {
		t.Fatalf("expected COMPLETE, got %+v", result)
	}
}
