package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeAffectedDirectories(t *testing.T) {
	tmp := t.TempDir()
	ref := filepath.Join(tmp, "design.md")
	if err := os.WriteFile(ref, []byte("touch internal/codex/client.go and cmd/ax/commands.go"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := AnalyzeAffectedDirectories("update internal/core/state.go", ref)
	if len(out) == 0 {
		t.Fatal("expected affected directories")
	}
}

func TestBuildContextLayers(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, filepath.Join(tmp, ".ax", "memory", "MEMORY.md"), "x")
	mustWrite(t, filepath.Join(tmp, ".ax", "memory", "gotchas.md"), "x")
	layers := BuildContextLayers(tmp, []string{".ax/proposals/p/proposal.md", ".ax/plans/p-plan.md"}, []string{".ax/proposals"})
	if len(layers.Protocol) != 1 || len(layers.SessionMem) != 2 {
		t.Fatalf("unexpected layers: %+v", layers)
	}
	if len(layers.TaskScoped) != 1 {
		t.Fatalf("expected filtered task scoped context: %+v", layers.TaskScoped)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
