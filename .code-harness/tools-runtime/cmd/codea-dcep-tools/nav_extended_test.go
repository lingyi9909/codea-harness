package main

import (
	"strings"
	"testing"
)

func TestExtendedNavigationActionsAreRegistered(t *testing.T) {
	cases := [][]string{
		{"nav", "get-symbol-info"},
		{"nav", "find-by-annotation"},
		{"nav", "find-callers"},
	}
	for _, args := range cases {
		err := run(args)
		if err == nil || strings.Contains(err.Error(), "unknown nav action") {
			t.Fatalf("action %v is not registered: %v", args, err)
		}
	}
}

func TestFindByAnnotationRejectsRawPattern(t *testing.T) {
	err := run([]string{"nav", "find-by-annotation", "--annotation", "RestController", "--pattern", "$RAW"})
	if err == nil || !strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("raw pattern should be rejected, err=%v", err)
	}
}
