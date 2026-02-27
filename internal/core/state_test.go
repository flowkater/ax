package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStateTransitionMetadataAndValidation(t *testing.T) {
	now := time.Date(2026, 2, 26, 0, 0, 0, 0, time.UTC)
	st := DefaultState(now)

	if err := st.Transition(PhaseProposal, "propose", now.Add(time.Minute)); err != nil {
		t.Fatalf("proposal transition: %v", err)
	}
	if st.Phase != PhaseProposal {
		t.Fatalf("phase mismatch: %s", st.Phase)
	}
	if st.TriggerCommand != "propose" {
		t.Fatalf("trigger mismatch: %s", st.TriggerCommand)
	}
	if st.TransitionedAt == "" {
		t.Fatal("expected transitioned_at")
	}

	err := st.Transition(PhaseArchived, "archive", now.Add(2*time.Minute))
	if err == nil {
		t.Fatal("expected invalid transition error")
	}
	if !strings.Contains(err.Error(), "ERR_INVALID_PHASE_TRANSITION") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStateSaveAndLoadLegacy(t *testing.T) {
	tmp := t.TempDir()
	statePath := StatePath(tmp)
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := "version: v2\nphase: discovery\n"
	if err := os.WriteFile(statePath, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	st, err := LoadState(tmp)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if st.Phase != PhaseDiscovery {
		t.Fatalf("expected discovery, got %s", st.Phase)
	}

	st.AddContext(".ax/proposals/p-1/proposal.md")
	if err := st.Save(tmp); err != nil {
		t.Fatalf("save state: %v", err)
	}

	reloaded, err := LoadState(tmp)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if len(reloaded.ContextChain) != 1 {
		t.Fatalf("expected context chain len 1, got %d", len(reloaded.ContextChain))
	}
}

func TestStateAllowsNewProposalAfterArchive(t *testing.T) {
	now := time.Date(2026, 2, 26, 0, 0, 0, 0, time.UTC)
	st := DefaultState(now)
	sequence := []Phase{
		PhaseProposal,
		PhasePlanning,
		PhaseImplementation,
		PhaseVerification,
		PhaseArchived,
		PhaseProposal,
	}
	for i, next := range sequence {
		if err := st.Transition(next, "cycle", now.Add(time.Duration(i+1)*time.Minute)); err != nil {
			t.Fatalf("transition to %s failed: %v", next, err)
		}
	}
	if st.Phase != PhaseProposal {
		t.Fatalf("expected phase proposal after new cycle, got %s", st.Phase)
	}
}

func TestLoadContextLayers(t *testing.T) {
	tmp := t.TempDir()
	mustWriteFile(t, filepath.Join(tmp, ".ax", "context-policy.md"), "# Context Policy")
	mustWriteFile(t, filepath.Join(tmp, ".ax", "memory", "MEMORY.md"), "# MEMORY")
	mustWriteFile(t, filepath.Join(tmp, ".ax", "memory", "gotchas.md"), "# gotchas")
	mustWriteFile(t, filepath.Join(tmp, ".ax", "plans", "p-1-plan.md"), "# Plan")

	st := DefaultState(time.Now())
	st.AddContext(".ax/plans/p-1-plan.md")

	layers := LoadContextLayers(tmp, st, []string{".ax/plans"})
	if layers.Protocol.Path == "" {
		t.Fatal("expected protocol context")
	}
	if len(layers.TaskScoped) != 1 {
		t.Fatalf("expected 1 task-scoped snippet, got %d", len(layers.TaskScoped))
	}
	if len(layers.SessionMemory) != 2 {
		t.Fatalf("expected 2 session snippets, got %d", len(layers.SessionMemory))
	}

	rendered := FormatContextLayers(layers)
	if !strings.Contains(rendered, "Context Inputs") {
		t.Fatalf("unexpected formatted context: %q", rendered)
	}
}

func TestRunStateTurnHistoryCompatibilityAndTrim(t *testing.T) {
	tmp := t.TempDir()
	st := DefaultState(time.Now())
	st.Run.ThreadID = "th-1"
	st.Run.ActiveTurnID = "tu-1"
	st.Run.EngineMode = "scaffold"
	for i := 0; i < 120; i++ {
		st.AppendTurnRef(TurnRef{
			TurnID:    "tu",
			Step:      "S",
			Status:    "completed",
			StartedAt: time.Now().UTC().Format(time.RFC3339),
			EndedAt:   time.Now().UTC().Format(time.RFC3339),
		}, 100)
	}
	if len(st.Run.TurnHistory) != 100 {
		t.Fatalf("expected history trimmed to 100, got %d", len(st.Run.TurnHistory))
	}
	if err := st.Save(tmp); err != nil {
		t.Fatalf("save state: %v", err)
	}
	reloaded, err := LoadState(tmp)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if reloaded.Run.ThreadID != "th-1" {
		t.Fatalf("expected thread_id persisted, got %q", reloaded.Run.ThreadID)
	}
	if len(reloaded.Run.TurnHistory) != 100 {
		t.Fatalf("expected persisted turn history length 100, got %d", len(reloaded.Run.TurnHistory))
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
