package codex

import (
	"strings"
	"testing"

	"github.com/flowkater/ax/internal/core"
)

func TestBuildStepPromptDeterministic(t *testing.T) {
	ctx := core.BuiltContextLayers{
		Protocol:   []string{".ax/context-policy.md"},
		TaskScoped: []string{".ax/plans/p-1.md"},
		SessionMem: []string{".ax/memory/MEMORY.md"},
	}
	one := BuildStepPrompt("T-01", "# Plan\n- item", ctx)
	two := BuildStepPrompt("T-01", "# Plan\n- item", ctx)
	if one != two {
		t.Fatal("expected deterministic prompt output")
	}
	for _, want := range []string{"step: T-01", "context.protocol:", "context.task:", "context.session:"} {
		if !strings.Contains(one, want) {
			t.Fatalf("prompt missing %q: %s", want, one)
		}
	}
}

func TestBuildTDDStepPrompt(t *testing.T) {
	got := BuildTDDStepPrompt("T1", "green", "# Plan\n- item")
	for _, want := range []string{"tier: T1", "phase: green", "# Plan"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
}
