package report

import (
	"strings"
	"testing"
)

func TestProjectAdapterGeneratesConfigV2ResourceScopes(t *testing.T) {
	text := readResourceReviewContract(t, "agents", "project-adapter.md")
	for _, want := range []string{
		"version: 2",
		"scope.mapperIncludes",
		"scope.configIncludes",
		"src/main/resources/**/*Mapper.xml",
		"src/main/resources/**/*.yml",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("project-adapter missing Task 2 init rule %q", want)
		}
	}
}

func TestAdaptProjectGeneratesConfigV2ResourceScopes(t *testing.T) {
	text := readResourceReviewContract(t, "skills", "adapt-project", "SKILL.md")
	for _, want := range []string{
		"version: 2",
		"scope.mapperIncludes",
		"scope.configIncludes",
		"src/main/resources/**/*Mapper.xml",
		"src/main/resources/**/*.yml",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("adapt-project missing Task 2 init rule %q", want)
		}
	}
}
