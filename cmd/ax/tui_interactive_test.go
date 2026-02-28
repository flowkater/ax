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
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
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

	model, _ := m.Update(ctrlKey('t'))
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

	model, _ = m.Update(ctrlKey('t'))
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

func TestInteractiveTUIModelCommandSubmitFlow(t *testing.T) {
	m := newInteractiveTUIModel(t.TempDir(), runtimeContext{}, 1)
	m.snapshotFn = func(string, int, time.Time) (tuiSnapshot, error) { return tuiSnapshot{}, nil }
	m.commandEventFn = nil
	var submitted string
	m.commandFn = func(_ string, _ runtimeContext, line string, _ time.Time) (tuiCommandResult, error) {
		submitted = line
		return tuiCommandResult{
			Status: "run completed",
			Transcript: []tuiTranscriptEntry{
				{Role: tuiTranscriptRoleAssistant, Content: "world"},
			},
		}, nil
	}

	for _, r := range []rune("hello") {
		model, _ := m.Update(runeKey(r))
		m = model.(interactiveTUIModel)
	}
	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(interactiveTUIModel)
	if cmd == nil {
		t.Fatal("expected command cmd on enter")
	}
	msg := cmd()
	model, _ = m.Update(msg)
	m = model.(interactiveTUIModel)

	if submitted != "hello" {
		t.Fatalf("expected submitted input hello, got %q", submitted)
	}
	if len(m.transcript) != 2 {
		t.Fatalf("expected user+assistant transcript entries, got %d", len(m.transcript))
	}
	if !strings.Contains(m.View(), "# Transcript") {
		t.Fatalf("expected transcript pane in view:\n%s", m.View())
	}
}

func TestInteractiveTUIModelStreamsDeltaBeforeCompletion(t *testing.T) {
	m := newInteractiveTUIModel(t.TempDir(), runtimeContext{}, 1)
	m.snapshotFn = func(string, int, time.Time) (tuiSnapshot, error) { return tuiSnapshot{}, nil }
	m.commandEventFn = func(_ string, _ runtimeContext, _ string, _ time.Time, emit tuiCommandEventCallback) (tuiCommandResult, error) {
		emit(tuiTranscriptEntry{Role: tuiTranscriptRoleAssistant, ThreadID: "th-1", TurnID: "tu-1", Content: "hello "})
		emit(tuiTranscriptEntry{Role: tuiTranscriptRoleAssistant, ThreadID: "th-1", TurnID: "tu-1", Content: "world"})
		return tuiCommandResult{
			Status: "run completed",
		}, nil
	}

	for _, r := range []rune("stream") {
		model, _ := m.Update(runeKey(r))
		m = model.(interactiveTUIModel)
	}
	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(interactiveTUIModel)
	if cmd == nil {
		t.Fatal("expected command cmd on enter")
	}

	msg := cmd()
	model, follow := m.Update(msg)
	m = model.(interactiveTUIModel)
	if follow == nil {
		t.Fatal("expected follow-up async command for stream")
	}
	if !strings.Contains(m.status, "streaming") {
		t.Fatalf("expected streaming status, got %q", m.status)
	}

	msg = follow()
	model, _ = m.Update(msg)
	m = model.(interactiveTUIModel)
	if len(m.transcript) != 2 {
		t.Fatalf("expected user+assistant transcript entries, got %d", len(m.transcript))
	}
	if got := m.transcript[len(m.transcript)-1].Content; got != "hello world" {
		t.Fatalf("expected merged streamed delta, got %q", got)
	}
}

func TestInteractiveTUIModelTreatsActionKeysAsInputWhenBufferNotEmpty(t *testing.T) {
	m := newInteractiveTUIModel(t.TempDir(), runtimeContext{}, 1)
	model, _ := m.Update(runeKey('/'))
	m = model.(interactiveTUIModel)
	model, _ = m.Update(runeKey('t'))
	m = model.(interactiveTUIModel)
	if m.pendingAction != "" {
		t.Fatalf("expected no pending action while typing, got %q", m.pendingAction)
	}
	if got := string(m.inputBuffer); got != "/t" {
		t.Fatalf("expected input '/t', got %q", got)
	}
}

func TestInteractiveTUIModelInterruptUsesCtrlX(t *testing.T) {
	m := newInteractiveTUIModel(t.TempDir(), runtimeContext{}, 1)
	model, _ := m.Update(ctrlKey('x'))
	m = model.(interactiveTUIModel)
	if m.pendingAction != "interrupt" {
		t.Fatalf("expected ctrl+x to map interrupt action, got %q", m.pendingAction)
	}
}

func runeKey(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func ctrlKey(r rune) tea.KeyMsg {
	switch r {
	case 't':
		return tea.KeyMsg{Type: tea.KeyCtrlT}
	case 'u':
		return tea.KeyMsg{Type: tea.KeyCtrlU}
	case 'x':
		return tea.KeyMsg{Type: tea.KeyCtrlX}
	case 'b':
		return tea.KeyMsg{Type: tea.KeyCtrlB}
	case 'f':
		return tea.KeyMsg{Type: tea.KeyCtrlF}
	case 's':
		return tea.KeyMsg{Type: tea.KeyCtrlS}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
	}
}
