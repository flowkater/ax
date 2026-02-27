package ax

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/flowkater/ax/internal/codex"
	"github.com/flowkater/ax/internal/core"
)

func TestConsumeRunStreamEventsPersistsCountsAndJournal(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "state"); err != nil {
		t.Fatalf("state: %v", err)
	}
	st := readState(t, tmp)
	st.Phase = core.PhaseImplementation
	st.Runtime.SessionID = "sess-stream"
	st.Runtime.ClusterID = "cluster-stream"
	st.Runtime.NodeID = "node-stream"
	st.Run.EngineMode = "real"
	if err := core.SaveState(tmp, st); err != nil {
		t.Fatalf("save state: %v", err)
	}

	events := make(chan codex.StreamEvent, 2)
	events <- codex.StreamEvent{Type: codex.StreamEventDelta, ThreadID: "th-1", TurnID: "tu-1", Delta: "hello"}
	events <- codex.StreamEvent{Type: codex.StreamEventCompleted, ThreadID: "th-1", TurnID: "tu-1", Completed: true}
	close(events)

	summary, err := consumeRunStreamEvents(tmp, st, "plan-stream", "Task-01", "th-1", events)
	if err != nil {
		t.Fatalf("consume stream: %v", err)
	}
	if summary.DeltaEvents != 1 || summary.CompletedEvents != 1 {
		t.Fatalf("unexpected stream summary: %+v", summary)
	}
	if summary.LastTurnID != "tu-1" {
		t.Fatalf("unexpected last turn id: %q", summary.LastTurnID)
	}
	if st.Run.ActiveTurnID != "tu-1" {
		t.Fatalf("expected active turn to track stream turn id, got %q", st.Run.ActiveTurnID)
	}

	journal := mustRead(t, filepath.Join(tmp, ".ax", "logs", "runtime-journal.jsonl"))
	if !strings.Contains(journal, "\"stage\":\"stream_delta\"") || !strings.Contains(journal, "\"stage\":\"stream_completed\"") {
		t.Fatalf("expected stream stages in journal, got:\n%s", journal)
	}
}

func TestConsumeRunStreamEventsFailsWithoutCompletedEvent(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "state"); err != nil {
		t.Fatalf("state: %v", err)
	}
	st := readState(t, tmp)
	st.Phase = core.PhaseImplementation
	st.Runtime.SessionID = "sess-stream-missing-complete"
	st.Runtime.ClusterID = "cluster-stream"
	st.Runtime.NodeID = "node-stream"
	st.Run.EngineMode = "real"
	if err := core.SaveState(tmp, st); err != nil {
		t.Fatalf("save state: %v", err)
	}

	events := make(chan codex.StreamEvent, 1)
	events <- codex.StreamEvent{Type: codex.StreamEventDelta, ThreadID: "th-1", TurnID: "tu-1", Delta: "partial"}
	close(events)

	_, err := consumeRunStreamEvents(tmp, st, "plan-stream", "Task-01", "th-1", events)
	if err == nil {
		t.Fatal("expected missing completed stream to fail")
	}
	if !strings.Contains(err.Error(), "completed event missing") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeRunStreamEventType(t *testing.T) {
	cases := []struct {
		name string
		in   codex.StreamEvent
		want string
	}{
		{
			name: "delta type",
			in:   codex.StreamEvent{Type: codex.StreamEventDelta, Delta: "x"},
			want: codex.StreamEventDelta,
		},
		{
			name: "completed type",
			in:   codex.StreamEvent{Type: codex.StreamEventCompleted, Completed: true},
			want: codex.StreamEventCompleted,
		},
		{
			name: "completed inferred",
			in:   codex.StreamEvent{Completed: true},
			want: codex.StreamEventCompleted,
		},
		{
			name: "delta inferred",
			in:   codex.StreamEvent{Delta: "x"},
			want: codex.StreamEventDelta,
		},
		{
			name: "invalid type ignored",
			in:   codex.StreamEvent{Type: "progress"},
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeRunStreamEventType(tc.in)
			if got != tc.want {
				t.Fatalf("normalize event type mismatch: got=%q want=%q", got, tc.want)
			}
		})
	}
}

func TestConsumeRunStreamEventsUpdatesTDDCurrentStep(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "state"); err != nil {
		t.Fatalf("state: %v", err)
	}
	st := readState(t, tmp)
	st.Phase = core.PhaseImplementation
	st.Runtime.SessionID = "sess-stream-tdd"
	st.Runtime.ClusterID = "cluster-stream"
	st.Runtime.NodeID = "node-stream"
	st.Run.EngineMode = "real"
	st.TDD = core.TDDState{
		Enabled:     true,
		CurrentTier: "T0",
		CurrentStep: "T0/Red",
		TotalSteps:  9,
	}
	if err := core.SaveState(tmp, st); err != nil {
		t.Fatalf("save state: %v", err)
	}

	events := make(chan codex.StreamEvent, 2)
	events <- codex.StreamEvent{Type: codex.StreamEventDelta, ThreadID: "th-1", TurnID: "tu-1", Delta: "partial"}
	events <- codex.StreamEvent{Type: codex.StreamEventCompleted, ThreadID: "th-1", TurnID: "tu-1", Completed: true}
	close(events)

	if _, err := consumeRunStreamEvents(tmp, st, "plan-stream", "T0/Refactor", "th-1", events); err != nil {
		t.Fatalf("consume stream: %v", err)
	}
	if st.TDD.CurrentStep != "T0/Refactor" {
		t.Fatalf("expected TDD current step updated to final step, got %q", st.TDD.CurrentStep)
	}
}

func TestConsumeRunStreamEventsWritesCheckpoint(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "state"); err != nil {
		t.Fatalf("state: %v", err)
	}
	st := readState(t, tmp)
	st.Phase = core.PhaseImplementation
	st.Runtime.SessionID = "sess-stream-checkpoint"
	st.Runtime.ClusterID = "cluster-stream"
	st.Runtime.NodeID = "node-stream"
	st.Run.EngineMode = "real"
	if err := core.SaveState(tmp, st); err != nil {
		t.Fatalf("save state: %v", err)
	}

	events := make(chan codex.StreamEvent, 2)
	events <- codex.StreamEvent{Type: codex.StreamEventDelta, ThreadID: "th-1", TurnID: "tu-1", Delta: "partial"}
	events <- codex.StreamEvent{Type: codex.StreamEventCompleted, ThreadID: "th-1", TurnID: "tu-1", Completed: true}
	close(events)

	if _, err := consumeRunStreamEvents(tmp, st, "plan-stream", "Task-01", "th-1", events); err != nil {
		t.Fatalf("consume stream: %v", err)
	}

	checkpointPath := filepath.Join(tmp, ".ax", "runtime", "checkpoints", "sess-stream-checkpoint.json")
	body := mustRead(t, checkpointPath)
	if !strings.Contains(body, "\"status\": \"in_progress\"") {
		t.Fatalf("expected in_progress checkpoint update, got:\n%s", body)
	}
}
