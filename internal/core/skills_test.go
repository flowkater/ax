package core

import (
	"strings"
	"testing"
)

func TestDefaultSkillRegistryYAMLContainsRequiredSkills(t *testing.T) {
	registry := DefaultSkillRegistryYAML()
	for _, needle := range []string{
		"interview",
		"openspec",
		"tdd-plan",
		"tdd-go",
		"tdd-go-loop",
		"superpowers",
	} {
		if !strings.Contains(registry, needle) {
			t.Fatalf("expected registry to include %q, got:\n%s", needle, registry)
		}
	}
}
