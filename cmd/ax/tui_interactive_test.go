package ax

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestInteractiveTUIModelRendersDashboardFromSnapshot(t *testing.T) {
	m := newInteractiveTUIModel(t.TempDir(), runtimeContext{}, 1)
	m.snapshot = tuiSnapshot{
		Dashboard: tuiDashboard{
			Phase:      "run",
			Progress:   66,
			Proposal:   "proposal-1",
			Plan:       "plan-1.md",
			LastResult: "run:completed",
			LastError:  "none",
		},
		RuntimeJournal: []journalBrief{{Time: "2026-02-27T00:00:00Z", Command: "run", Stage: "execute"}},
	}
	view := m.View()
	for _, want := range []string{"Screen A Dashboard", "phase: run", "progress: 66", "proposal: proposal-1", "recent_logs"} {
		if !strings.Contains(view, want) {
			t.Fatalf("dashboard view missing %q:\n%s", want, view)
		}
	}
	if !strings.Contains(view, "keys:") {
		t.Fatalf("expected key help to be always visible: %s", view)
	}
}

func TestInteractiveTUIModelScreenNavigation(t *testing.T) {
	m := newInteractiveTUIModel(t.TempDir(), runtimeContext{}, 1)
	model, _ := m.Update(runeKey('2'))
	m = model.(interactiveTUIModel)
	if m.screen != tuiScreenRuns {
		t.Fatalf("expected screen runs, got %v", m.screen)
	}

	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = model.(interactiveTUIModel)
	if m.screen != tuiScreenVerify {
		t.Fatalf("expected screen verify after tab, got %v", m.screen)
	}

	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = model.(interactiveTUIModel)
	if m.screen != tuiScreenRuns {
		t.Fatalf("expected screen runs after shift+tab, got %v", m.screen)
	}
}

func TestInteractiveTUIModelDangerousActionConfirmationFlow(t *testing.T) {
	m := newInteractiveTUIModel(t.TempDir(), runtimeContext{}, 1)
	m.snapshotFn = func(string, int, time.Time) (tuiSnapshot, error) { return tuiSnapshot{}, nil }
	var called []string
	m.actionFn = func(_ string, _ runtimeContext, action string, _ time.Time) error {
		called = append(called, action)
		return nil
	}

	model, _ := m.Update(runeKey('t'))
	m = model.(interactiveTUIModel)
	if m.pendingAction != "retry" {
		t.Fatalf("expected pending retry, got %q", m.pendingAction)
	}

	model, _ = m.Update(runeKey('n'))
	m = model.(interactiveTUIModel)
	if m.pendingAction != "" {
		t.Fatalf("expected pending action clear after cancel, got %q", m.pendingAction)
	}
	if len(called) != 0 {
		t.Fatalf("expected no action call on cancel, got %v", called)
	}

	model, _ = m.Update(runeKey('t'))
	m = model.(interactiveTUIModel)
	model, cmd := m.Update(runeKey('y'))
	m = model.(interactiveTUIModel)
	if cmd == nil {
		t.Fatal("expected action command after confirmation")
	}
	result := cmd()
	model, _ = m.Update(result)
	m = model.(interactiveTUIModel)
	if len(called) != 1 || called[0] != "retry" {
		t.Fatalf("expected retry action once, got %v", called)
	}
	if !strings.Contains(m.status, "confirmed") {
		t.Fatalf("expected confirmed status, got %q", m.status)
	}
}

func TestNormalizeTUIActionValidation(t *testing.T) {
	if _, err := normalizeTUIAction("retry"); err != nil {
		t.Fatalf("retry should be valid: %v", err)
	}
	if _, err := normalizeTUIAction("unknown"); err == nil {
		t.Fatal("expected unknown action to fail")
	}
}

func runeKey(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}
