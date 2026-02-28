package ax

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/flowkater/ax/internal/codex"
	"github.com/flowkater/ax/internal/core"
)

type tuiTranscriptEntry struct {
	Time     string
	Role     string
	Content  string
	ThreadID string
	TurnID   string
}

const (
	tuiTranscriptRoleUser      = "user"
	tuiTranscriptRoleAssistant = "assistant"
	tuiTranscriptRoleSystem    = "system"
	tuiTranscriptRoleError     = "error"
)

type tuiCommandResult struct {
	Command    tuiCommandKind
	Status     string
	ThreadID   string
	TurnID     string
	DurationMS int64
	Transcript []tuiTranscriptEntry
}

type tuiCommandEventCallback func(tuiTranscriptEntry)

type tuiStreamSummary struct {
	DeltaEvents     int
	CompletedEvents int
	LastTurnID      string
	Content         string
}

var (
	tuiSensitiveKeyPattern = regexp.MustCompile(`(?i)\b(api[_-]?key|token|secret|password)\b\s*[:=]\s*([^\s]+)`)
	tuiBearerPattern       = regexp.MustCompile(`(?i)\bbearer\s+([a-z0-9._-]+)`)
)

func executeTUICommand(base string, rt runtimeContext, raw string, now time.Time) (tuiCommandResult, error) {
	return executeTUICommandWithEvents(base, rt, raw, now, nil)
}

func executeTUICommandWithEvents(base string, rt runtimeContext, raw string, now time.Time, emit tuiCommandEventCallback) (tuiCommandResult, error) {
	startedAt := now
	if startedAt.IsZero() {
		startedAt = time.Now()
	}

	parsed, parseErr := parseTUICommand(raw)
	result := tuiCommandResult{}

	err := core.WithStateLock(base, func() error {
		if err := ensureMVPLayout(base); err != nil {
			return err
		}
		st, err := core.LoadState(base)
		if err != nil {
			return err
		}
		applyRuntimeState(st, rt, "tui")

		status := "completed"
		threadID := strings.TrimSpace(st.Run.ThreadID)
		turnID := strings.TrimSpace(st.Run.ActiveTurnID)
		commandName := "invalid"
		if parseErr == nil {
			commandName = string(parsed.Kind)
			result.Command = parsed.Kind
		}

		if parseErr != nil {
			status = "failed"
			errCode := "AX_TUI_INVALID_COMMAND"
			errMsg := parseErr.Error()
			st.Run.ErrorCode = errCode
			st.Run.ErrorSummary = errMsg
			st.Run.RecoverHint = "run /help to list supported commands"
			st.SetLastError(errCode, errMsg, startedAt)
			if saveErr := st.Save(base); saveErr != nil {
				return saveErr
			}
			_ = appendRuntimeJournal(base, runtimeJournalEntry{
				Time:       startedAt.Format(time.RFC3339),
				SessionID:  st.Runtime.SessionID,
				ClusterID:  st.Runtime.ClusterID,
				NodeID:     st.Runtime.NodeID,
				Command:    "tui_command",
				Stage:      status,
				Mode:       string(st.Runtime.Mode),
				Phase:      string(st.Phase),
				Artifact:   commandName,
				ErrorCode:  errCode,
				Error:      errMsg,
				ThreadID:   emptyFallback(threadID),
				TurnID:     emptyFallback(turnID),
				DurationMS: time.Since(startedAt).Milliseconds(),
				EngineMode: st.Run.EngineMode,
			})
			_ = appendTUIInputHistory(base, st.Runtime.SessionID, raw, status, startedAt)
			return parseErr
		}

		var execErr error
		switch parsed.Kind {
		case tuiCommandKindHelp:
			result.Status = "help displayed"
			result.Transcript = append(result.Transcript, tuiTranscriptEntry{
				Time:    startedAt.Format(time.RFC3339),
				Role:    tuiTranscriptRoleSystem,
				Content: renderTUICommandHelp(),
			})
		case tuiCommandKindRun:
			var streamSummary tuiStreamSummary
			var runErr error
			threadID, turnID, streamSummary, runErr = executeTUIRunTurn(st, parsed.Prompt, startedAt, emit)
			if runErr != nil {
				execErr = runErr
				break
			}
			result.Status = "run completed"
			if streamSummary.CompletedEvents > 0 {
				result.Status = fmt.Sprintf("run completed (stream delta=%d completed=%d)", streamSummary.DeltaEvents, streamSummary.CompletedEvents)
			}
			if emit == nil || streamSummary.DeltaEvents == 0 {
				assistant := strings.TrimSpace(streamSummary.Content)
				if assistant == "" {
					assistant = "turn completed"
				}
				result.Transcript = append(result.Transcript, tuiTranscriptEntry{
					Time:     time.Now().Format(time.RFC3339),
					Role:     tuiTranscriptRoleAssistant,
					Content:  assistant,
					ThreadID: strings.TrimSpace(threadID),
					TurnID:   strings.TrimSpace(turnID),
				})
			}
		case tuiCommandKindNewThread:
			threadID, execErr = executeTUINewThread(st, parsed.Title)
			if execErr == nil {
				turnID = ""
				result.Status = "thread created"
				result.Transcript = append(result.Transcript, tuiTranscriptEntry{
					Time:     time.Now().Format(time.RFC3339),
					Role:     tuiTranscriptRoleSystem,
					Content:  fmt.Sprintf("new thread selected: %s", emptyFallback(threadID)),
					ThreadID: strings.TrimSpace(threadID),
				})
			}
		case tuiCommandKindUseThread:
			threadID, execErr = executeTUIUseThread(st, parsed.ThreadID)
			if execErr == nil {
				turnID = ""
				result.Status = "thread switched"
				result.Transcript = append(result.Transcript, tuiTranscriptEntry{
					Time:     time.Now().Format(time.RFC3339),
					Role:     tuiTranscriptRoleSystem,
					Content:  fmt.Sprintf("active thread: %s", emptyFallback(threadID)),
					ThreadID: strings.TrimSpace(threadID),
				})
			}
		case tuiCommandKindSteer:
			threadID, turnID, execErr = performTUIEngineActionWithOptions(st, "steer", tuiEngineActionOptions{SteerInstruction: parsed.Instruction}, startedAt)
			if execErr == nil {
				result.Status = "steer completed"
				result.Transcript = append(result.Transcript, tuiTranscriptEntry{
					Time:     time.Now().Format(time.RFC3339),
					Role:     tuiTranscriptRoleSystem,
					Content:  fmt.Sprintf("steer applied (turn=%s)", emptyFallback(turnID)),
					ThreadID: strings.TrimSpace(threadID),
					TurnID:   strings.TrimSpace(turnID),
				})
			}
		case tuiCommandKindRollback:
			threadID, turnID, execErr = performTUIEngineActionWithOptions(st, "rollback", tuiEngineActionOptions{RollbackTurnID: parsed.TurnID}, startedAt)
			if execErr == nil {
				result.Status = "rollback completed"
				result.Transcript = append(result.Transcript, tuiTranscriptEntry{
					Time:     time.Now().Format(time.RFC3339),
					Role:     tuiTranscriptRoleSystem,
					Content:  fmt.Sprintf("rollback completed (turn=%s)", emptyFallback(turnID)),
					ThreadID: strings.TrimSpace(threadID),
					TurnID:   strings.TrimSpace(turnID),
				})
			}
		case tuiCommandKindInterrupt:
			threadID, turnID, execErr = performTUIEngineAction(st, "interrupt", startedAt)
			if execErr == nil {
				result.Status = "interrupt completed"
				result.Transcript = append(result.Transcript, tuiTranscriptEntry{
					Time:     time.Now().Format(time.RFC3339),
					Role:     tuiTranscriptRoleSystem,
					Content:  fmt.Sprintf("interrupt requested (turn=%s)", emptyFallback(turnID)),
					ThreadID: strings.TrimSpace(threadID),
					TurnID:   strings.TrimSpace(turnID),
				})
			}
		case tuiCommandKindResume:
			threadID, turnID, execErr = performTUIEngineAction(st, "resume", startedAt)
			if execErr == nil {
				result.Status = "resume completed"
				result.Transcript = append(result.Transcript, tuiTranscriptEntry{
					Time:     time.Now().Format(time.RFC3339),
					Role:     tuiTranscriptRoleSystem,
					Content:  fmt.Sprintf("resume completed (thread=%s)", emptyFallback(threadID)),
					ThreadID: strings.TrimSpace(threadID),
					TurnID:   strings.TrimSpace(turnID),
				})
			}
		case tuiCommandKindFork:
			threadID, turnID, execErr = performTUIEngineAction(st, "fork", startedAt)
			if execErr == nil {
				result.Status = "fork completed"
				result.Transcript = append(result.Transcript, tuiTranscriptEntry{
					Time:     time.Now().Format(time.RFC3339),
					Role:     tuiTranscriptRoleSystem,
					Content:  fmt.Sprintf("fork completed (thread=%s)", emptyFallback(threadID)),
					ThreadID: strings.TrimSpace(threadID),
				})
			}
		case tuiCommandKindRetry:
			threadID, turnID, execErr = performTUIEngineAction(st, "retry", startedAt)
			if execErr == nil {
				result.Status = "retry completed"
				result.Transcript = append(result.Transcript, tuiTranscriptEntry{
					Time:     time.Now().Format(time.RFC3339),
					Role:     tuiTranscriptRoleSystem,
					Content:  fmt.Sprintf("retry completed (turn=%s)", emptyFallback(turnID)),
					ThreadID: strings.TrimSpace(threadID),
					TurnID:   strings.TrimSpace(turnID),
				})
			}
		default:
			execErr = fmt.Errorf("unsupported command: %s", parsed.Kind)
		}

		if execErr != nil {
			status = "failed"
			errCode, errMsg, _ := codex.MapCodexError(execErr)
			if errCode == "" {
				errCode = "AX_ENGINE_INTERNAL"
			}
			if errMsg == "" {
				errMsg = execErr.Error()
			}
			st.Run.ErrorCode = errCode
			st.Run.ErrorSummary = errMsg
			st.Run.RecoverHint = "check command and engine state, then retry (or /help)"
			st.SetLastError(errCode, errMsg, startedAt)
			if saveErr := st.Save(base); saveErr != nil {
				return saveErr
			}
			_ = appendRuntimeJournal(base, runtimeJournalEntry{
				Time:       time.Now().Format(time.RFC3339),
				SessionID:  st.Runtime.SessionID,
				ClusterID:  st.Runtime.ClusterID,
				NodeID:     st.Runtime.NodeID,
				Command:    "tui_command",
				Stage:      status,
				Mode:       string(st.Runtime.Mode),
				Phase:      string(st.Phase),
				Artifact:   commandName,
				ErrorCode:  errCode,
				Error:      errMsg,
				ThreadID:   emptyFallback(threadID),
				TurnID:     emptyFallback(turnID),
				DurationMS: time.Since(startedAt).Milliseconds(),
				EngineMode: st.Run.EngineMode,
			})
			_ = appendTUIInputHistory(base, st.Runtime.SessionID, raw, status, startedAt)
			return errors.New(errMsg)
		}

		result.ThreadID = strings.TrimSpace(threadID)
		result.TurnID = strings.TrimSpace(turnID)
		result.DurationMS = time.Since(startedAt).Milliseconds()
		if strings.TrimSpace(result.Status) == "" {
			result.Status = "completed"
		}

		st.Run.ErrorCode = ""
		st.Run.ErrorSummary = ""
		st.Run.RecoverHint = ""
		st.SetLastResult("tui", "command:"+commandName, filepath.ToSlash(filepath.Join(".ax", "state.yaml")), time.Now())
		st.ClearLastError()
		if err := st.Save(base); err != nil {
			return err
		}

		_ = appendRuntimeJournal(base, runtimeJournalEntry{
			Time:       time.Now().Format(time.RFC3339),
			SessionID:  st.Runtime.SessionID,
			ClusterID:  st.Runtime.ClusterID,
			NodeID:     st.Runtime.NodeID,
			Command:    "tui_command",
			Stage:      status,
			Mode:       string(st.Runtime.Mode),
			Phase:      string(st.Phase),
			Artifact:   commandName,
			ThreadID:   emptyFallback(result.ThreadID),
			TurnID:     emptyFallback(result.TurnID),
			DurationMS: result.DurationMS,
			EngineMode: st.Run.EngineMode,
		})
		_ = appendTUIInputHistory(base, st.Runtime.SessionID, raw, status, startedAt)
		return nil
	})
	if err != nil {
		return result, err
	}
	return result, nil
}

func executeTUIRunTurn(st *core.State, prompt string, startedAt time.Time, emit tuiCommandEventCallback) (threadID, turnID string, stream tuiStreamSummary, err error) {
	if st == nil {
		return "", "", tuiStreamSummary{}, fmt.Errorf("state is required")
	}

	cfg := resolveCodexConfigForTUI()
	engineMode := strings.TrimSpace(cfg.Mode)
	if engineMode == "" {
		engineMode = codex.DefaultMode
	}
	st.Run.EngineMode = engineMode

	engine, err := newCodexAdapterForTUI(cfg)
	if err != nil {
		return "", "", tuiStreamSummary{}, err
	}

	threadID = strings.TrimSpace(st.Run.ThreadID)
	if threadID == "" {
		callCtx, cancel := codexCallContext(cfg.Timeout)
		thread, createErr := engine.CreateThread(callCtx, defaultTUIThreadTitle(startedAt))
		cancel()
		if createErr != nil {
			return "", "", tuiStreamSummary{}, createErr
		}
		threadID = strings.TrimSpace(thread.ID)
		if threadID == "" {
			return "", "", tuiStreamSummary{}, fmt.Errorf("create thread returned empty thread id")
		}
		st.Run.ThreadID = threadID
		st.Run.ActiveTurnID = ""
		st.Run.TurnHistory = nil
	}

	var streamErr error
	callCtx, cancel := codexCallContext(cfg.Timeout)
	events, err := engine.StreamTurn(callCtx, threadID, prompt)
	cancel()
	if err == nil {
		stream, streamErr = consumeTUIStreamEvents(events, func(evt codex.StreamEvent) {
			if emit == nil {
				return
			}
			delta := evt.Delta
			if delta == "" {
				return
			}
			emit(tuiTranscriptEntry{
				Time:     time.Now().Format(time.RFC3339),
				Role:     tuiTranscriptRoleAssistant,
				Content:  delta,
				ThreadID: strings.TrimSpace(evt.ThreadID),
				TurnID:   strings.TrimSpace(evt.TurnID),
			})
		})
	}
	if err != nil || streamErr != nil {
		if err != nil {
			streamErr = err
		}
		callCtx, cancel := codexCallContext(cfg.Timeout)
		turn, runErr := engine.RunTurn(callCtx, threadID, prompt)
		cancel()
		if runErr != nil {
			if streamErr != nil {
				return threadID, "", tuiStreamSummary{}, fmt.Errorf("stream failed: %v; run fallback failed: %w", streamErr, runErr)
			}
			return threadID, "", tuiStreamSummary{}, runErr
		}
		turnID = strings.TrimSpace(turn.ID)
		if turnID == "" && strings.EqualFold(engineMode, "real") {
			return threadID, "", tuiStreamSummary{}, fmt.Errorf("run turn returned empty turn id")
		}
		if turnID != "" {
			appendTUIActionTurnRef(st, turnID, "run", time.Now())
		}
		st.Run.ActiveTurnID = ""
		stream.Content = strings.TrimSpace(turn.Content)
		return threadID, turnID, stream, nil
	}

	turnID = strings.TrimSpace(stream.LastTurnID)
	if turnID == "" && strings.EqualFold(engineMode, "real") {
		return threadID, "", tuiStreamSummary{}, fmt.Errorf("stream completed without turn id")
	}
	if turnID != "" {
		appendTUIActionTurnRef(st, turnID, "run", time.Now())
	}
	st.Run.ActiveTurnID = ""
	return threadID, turnID, stream, nil
}

func executeTUINewThread(st *core.State, title string) (string, error) {
	if st == nil {
		return "", fmt.Errorf("state is required")
	}
	cfg := resolveCodexConfigForTUI()
	engineMode := strings.TrimSpace(cfg.Mode)
	if engineMode == "" {
		engineMode = codex.DefaultMode
	}
	st.Run.EngineMode = engineMode
	engine, err := newCodexAdapterForTUI(cfg)
	if err != nil {
		return "", err
	}
	callCtx, cancel := codexCallContext(cfg.Timeout)
	thread, err := engine.CreateThread(callCtx, strings.TrimSpace(title))
	cancel()
	if err != nil {
		return "", err
	}
	threadID := strings.TrimSpace(thread.ID)
	if threadID == "" {
		return "", fmt.Errorf("create thread returned empty thread id")
	}
	st.Run.ThreadID = threadID
	st.Run.ActiveTurnID = ""
	st.Run.TurnHistory = nil
	return threadID, nil
}

func executeTUIUseThread(st *core.State, threadID string) (string, error) {
	if st == nil {
		return "", fmt.Errorf("state is required")
	}
	cfg := resolveCodexConfigForTUI()
	engineMode := strings.TrimSpace(cfg.Mode)
	if engineMode == "" {
		engineMode = codex.DefaultMode
	}
	st.Run.EngineMode = engineMode
	engine, err := newCodexAdapterForTUI(cfg)
	if err != nil {
		return "", err
	}
	target := strings.TrimSpace(threadID)
	if target == "" {
		return "", fmt.Errorf("thread id is required")
	}
	callCtx, cancel := codexCallContext(cfg.Timeout)
	thread, err := engine.GetThread(callCtx, target)
	cancel()
	if err != nil {
		return "", err
	}
	resolved := strings.TrimSpace(thread.ID)
	if resolved == "" {
		resolved = target
	}
	st.Run.ThreadID = resolved
	st.Run.ActiveTurnID = ""
	st.Run.TurnHistory = nil
	return resolved, nil
}

func consumeTUIStreamEvents(events <-chan codex.StreamEvent, onDelta func(codex.StreamEvent)) (tuiStreamSummary, error) {
	var out tuiStreamSummary
	var transcript strings.Builder
	for evt := range events {
		eventType := normalizeRunStreamEventType(evt)
		if eventType == "" {
			continue
		}
		if turnID := strings.TrimSpace(evt.TurnID); turnID != "" {
			out.LastTurnID = turnID
		}
		switch eventType {
		case codex.StreamEventDelta:
			out.DeltaEvents++
			transcript.WriteString(evt.Delta)
			if onDelta != nil {
				onDelta(evt)
			}
		case codex.StreamEventCompleted:
			out.CompletedEvents++
		}
	}
	out.Content = strings.TrimSpace(transcript.String())
	if out.CompletedEvents == 0 {
		return out, fmt.Errorf("stream completed event missing")
	}
	return out, nil
}

type tuiHistoryEntry struct {
	Time      string `json:"time"`
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
	Input     string `json:"input"`
}

func appendTUIInputHistory(base, sessionID, rawInput, status string, now time.Time) error {
	if strings.TrimSpace(base) == "" {
		return nil
	}
	if strings.TrimSpace(sessionID) == "" {
		sessionID = "global"
	}
	entry := tuiHistoryEntry{
		Time:      now.Format(time.RFC3339),
		SessionID: sessionID,
		Status:    strings.TrimSpace(status),
		Input:     maskSensitiveTUIInput(rawInput),
	}
	path := tuiHistoryPath(base, sessionID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(body, '\n')); err != nil {
		return err
	}
	return nil
}

func loadTUIInputHistory(base, sessionID string, limit int) ([]string, error) {
	if strings.TrimSpace(base) == "" {
		return []string{}, nil
	}
	if strings.TrimSpace(sessionID) == "" {
		sessionID = "global"
	}
	body, err := os.ReadFile(tuiHistoryPath(base, sessionID))
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	history := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry tuiHistoryEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		history = append(history, strings.TrimSpace(entry.Input))
	}
	if limit > 0 && len(history) > limit {
		history = history[len(history)-limit:]
	}
	return history, nil
}

func tuiHistoryPath(base, sessionID string) string {
	if strings.TrimSpace(sessionID) == "" {
		sessionID = "global"
	}
	return filepath.Join(base, ".ax", "runtime", "tui-history-"+sanitizeToken(sessionID)+".jsonl")
}

func defaultTUIThreadTitle(now time.Time) string {
	if now.IsZero() {
		now = time.Now()
	}
	return "tui-" + now.UTC().Format("20060102-150405")
}

func maskSensitiveTUIInput(input string) string {
	masked := tuiSensitiveKeyPattern.ReplaceAllString(input, "$1=***")
	masked = tuiBearerPattern.ReplaceAllString(masked, "bearer ***")
	return masked
}
