package ax

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/flowkater/ax/internal/codex"
	"github.com/flowkater/ax/internal/core"
)

var (
	resolveCodexConfigForTUI = codex.ResolveConfig
	newCodexAdapterForTUI    = codex.NewAdapter
)

const defaultTUISteerInstruction = "tui steer request: apply safe incremental correction"

func performTUIEngineAction(st *core.State, action string, now time.Time) (threadID string, turnID string, err error) {
	return performTUIEngineActionWithOptions(st, action, tuiEngineActionOptions{}, now)
}

type tuiEngineActionOptions struct {
	SteerInstruction string
	RollbackTurnID   string
}

func performTUIEngineActionWithOptions(st *core.State, action string, opts tuiEngineActionOptions, now time.Time) (threadID string, turnID string, err error) {
	if st == nil {
		return "", "", fmt.Errorf("state is required")
	}

	cfg := resolveCodexConfigForTUI()
	engineMode := strings.TrimSpace(cfg.Mode)
	if engineMode == "" {
		engineMode = codex.DefaultMode
	}
	st.Run.EngineMode = engineMode

	engine, err := newCodexAdapterForTUI(cfg)
	if err != nil {
		return "", "", err
	}
	if closer, ok := engine.(interface{ Close() error }); ok {
		defer func() { _ = closer.Close() }()
	}

	callCtx, cancel := codexCallContext(cfg.Timeout)
	defer cancel()

	threadID = strings.TrimSpace(st.Run.ThreadID)
	switch action {
	case "retry":
		if threadID == "" {
			st.Run.Blocked = false
			st.Run.RecoverHint = ""
			return "", "", nil
		}
		turn, err := engine.RunTurn(callCtx, threadID, buildTUIRetryPrompt(st))
		if err != nil {
			return threadID, resolveTUITargetTurnID(st), err
		}
		turnID = strings.TrimSpace(turn.ID)
		if turnID != "" {
			appendTUIActionTurnRef(st, turnID, "retried", now)
		}
		st.Run.ActiveTurnID = ""
		st.Run.Blocked = false
		st.Run.LastFailedStep = ""
		st.Run.LastFailureCause = ""
		st.Run.RetryAttempts = 0
		return threadID, turnID, nil
	case "resume":
		if threadID == "" {
			return "", "", fmt.Errorf("resume unavailable: no persisted thread_id")
		}
		thread, err := engine.ResumeSession(callCtx, threadID)
		if err != nil {
			return threadID, "", err
		}
		if resumed := strings.TrimSpace(thread.ID); resumed != "" {
			threadID = resumed
			st.Run.ThreadID = resumed
		}
		st.Run.Blocked = false
		return threadID, strings.TrimSpace(st.Run.ActiveTurnID), nil
	case "interrupt":
		if threadID == "" {
			return "", "", fmt.Errorf("interrupt unavailable: no persisted thread_id")
		}
		turnID = resolveTUITargetTurnID(st)
		if _, err := engine.InterruptTurn(callCtx, threadID, turnID); err != nil {
			return threadID, turnID, err
		}
		st.Run.ActiveTurnID = ""
		return threadID, turnID, nil
	case "rollback":
		if threadID == "" {
			return "", "", fmt.Errorf("rollback unavailable: no persisted thread_id")
		}
		turnID = strings.TrimSpace(opts.RollbackTurnID)
		if turnID == "" {
			turnID = resolveTUITargetTurnID(st)
		}
		thread, err := engine.RollbackTurns(callCtx, threadID, turnID)
		if err != nil {
			return threadID, turnID, err
		}
		if rolled := strings.TrimSpace(thread.ID); rolled != "" {
			threadID = rolled
			st.Run.ThreadID = rolled
		}
		if turnID != "" {
			trimTurnHistoryToTurnID(st, turnID)
		}
		st.Run.ActiveTurnID = ""
		return threadID, turnID, nil
	case "fork":
		if threadID == "" {
			return "", "", fmt.Errorf("fork unavailable: no persisted thread_id")
		}
		thread, err := engine.ForkSession(callCtx, threadID)
		if err != nil {
			return threadID, "", err
		}
		if forked := strings.TrimSpace(thread.ID); forked != "" {
			threadID = forked
			st.Run.ThreadID = forked
		}
		st.Run.ActiveTurnID = ""
		return threadID, "", nil
	case "steer":
		if threadID == "" {
			return "", "", fmt.Errorf("steer unavailable: no persisted thread_id")
		}
		targetTurnID := resolveTUITargetTurnID(st)
		instruction := strings.TrimSpace(opts.SteerInstruction)
		if instruction == "" {
			instruction = defaultTUISteerInstruction
		}
		turn, err := engine.SteerTurn(callCtx, threadID, targetTurnID, instruction)
		if err != nil {
			return threadID, targetTurnID, err
		}
		turnID = targetTurnID
		if steered := strings.TrimSpace(turn.ID); steered != "" {
			turnID = steered
		}
		if turnID != "" {
			appendTUIActionTurnRef(st, turnID, "steered", now)
			st.Run.ActiveTurnID = ""
		}
		return threadID, turnID, nil
	default:
		return threadID, "", fmt.Errorf("unsupported tui action: %s", action)
	}
}

func codexCallContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout > 0 {
		return context.WithTimeout(context.Background(), timeout)
	}
	return context.WithCancel(context.Background())
}

func resolveTUITargetTurnID(st *core.State) string {
	if st == nil {
		return ""
	}
	if active := strings.TrimSpace(st.Run.ActiveTurnID); active != "" {
		return active
	}
	for i := len(st.Run.TurnHistory) - 1; i >= 0; i-- {
		if turnID := strings.TrimSpace(st.Run.TurnHistory[i].TurnID); turnID != "" {
			return turnID
		}
	}
	return ""
}

func buildTUIRetryPrompt(st *core.State) string {
	if st == nil {
		return "tui retry request"
	}
	step := strings.TrimSpace(st.Run.LastFailedStep)
	if step == "" {
		return "tui retry request"
	}
	return "tui retry request for step: " + step
}

func appendTUIActionTurnRef(st *core.State, turnID, status string, now time.Time) {
	if st == nil || strings.TrimSpace(turnID) == "" {
		return
	}
	startedAt := now.UTC().Format(time.RFC3339)
	st.AppendTurnRef(core.TurnRef{
		TurnID:    turnID,
		Step:      "tui:" + status,
		Status:    status,
		StartedAt: startedAt,
		EndedAt:   now.UTC().Format(time.RFC3339),
	}, 100)
}

func trimTurnHistoryToTurnID(st *core.State, turnID string) {
	if st == nil || strings.TrimSpace(turnID) == "" || len(st.Run.TurnHistory) == 0 {
		return
	}
	for idx := len(st.Run.TurnHistory) - 1; idx >= 0; idx-- {
		if strings.TrimSpace(st.Run.TurnHistory[idx].TurnID) == turnID {
			st.Run.TurnHistory = append([]core.TurnRef(nil), st.Run.TurnHistory[:idx+1]...)
			return
		}
	}
}
