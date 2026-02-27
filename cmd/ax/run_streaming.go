package ax

import (
	"errors"
	"strings"
	"time"

	"github.com/flowkater/ax/internal/codex"
	"github.com/flowkater/ax/internal/core"
)

type runStreamSummary struct {
	DeltaEvents     int
	CompletedEvents int
	LastTurnID      string
}

func consumeRunStreamEvents(base string, st *core.State, planID, step, threadID string, events <-chan codex.StreamEvent) (runStreamSummary, error) {
	if st == nil {
		return runStreamSummary{}, errors.New("state is required for stream consumption")
	}

	var summary runStreamSummary
	for evt := range events {
		eventType := normalizeRunStreamEventType(evt)
		if eventType == "" {
			continue
		}

		eventTime := time.Now()
		turnID := strings.TrimSpace(evt.TurnID)
		if turnID != "" {
			summary.LastTurnID = turnID
			st.Run.ActiveTurnID = turnID
		}

		switch eventType {
		case codex.StreamEventDelta:
			summary.DeltaEvents++
			if st.TDD.Enabled {
				st.TDD.CurrentStep = step + "/stream"
			}
		case codex.StreamEventCompleted:
			summary.CompletedEvents++
			if st.TDD.Enabled {
				st.TDD.CurrentStep = step
			}
		}

		if err := st.Save(base); err != nil {
			return summary, err
		}
		if err := writeRuntimeCheckpoint(base, st, "in_progress", "run", eventTime); err != nil {
			return summary, err
		}
		if err := appendRuntimeJournal(base, runtimeJournalEntry{
			Time:       eventTime.Format(time.RFC3339),
			SessionID:  st.Runtime.SessionID,
			ClusterID:  st.Runtime.ClusterID,
			NodeID:     st.Runtime.NodeID,
			Command:    "run",
			Stage:      "stream_" + eventType,
			Mode:       string(st.Runtime.Mode),
			Phase:      string(st.Phase),
			PlanID:     planID,
			ThreadID:   threadID,
			TurnID:     turnID,
			Step:       step,
			EngineMode: st.Run.EngineMode,
		}); err != nil {
			return summary, err
		}
	}

	if summary.CompletedEvents == 0 {
		return summary, errors.New("stream completed event missing")
	}
	return summary, nil
}

func normalizeRunStreamEventType(evt codex.StreamEvent) string {
	eventType := strings.ToLower(strings.TrimSpace(evt.Type))
	if eventType != "" {
		switch eventType {
		case codex.StreamEventDelta, codex.StreamEventCompleted:
			return eventType
		default:
			return ""
		}
	}
	if evt.Completed {
		return codex.StreamEventCompleted
	}
	if strings.TrimSpace(evt.Delta) != "" {
		return codex.StreamEventDelta
	}
	return ""
}
