package ax

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/flowkater/ax/internal/codex"
	"github.com/flowkater/ax/internal/core"
)

type fakeTUIAdapter struct {
	createThreadID string
	runTurn        *codex.Turn
	streamEvents   []codex.StreamEvent
	streamErr      error
	steerTurn      *codex.Turn

	streamCalls           int
	runCalls              int
	createThreadCalls     int
	lastSteerInstruction  string
	lastSteerThreadID     string
	lastSteerTargetTurnID string
}

func (f *fakeTUIAdapter) CreateThread(ctx context.Context, title string) (*codex.Thread, error) {
	f.createThreadCalls++
	id := strings.TrimSpace(f.createThreadID)
	if id == "" {
		id = "th-fake-1"
	}
	return &codex.Thread{ID: id, Title: title, CreatedAt: time.Now().UTC().Format(time.RFC3339)}, nil
}

func (f *fakeTUIAdapter) RunTurn(ctx context.Context, threadID, prompt string) (*codex.Turn, error) {
	f.runCalls++
	if f.runTurn != nil {
		return f.runTurn, nil
	}
	return &codex.Turn{ID: "tu-run-1", ThreadID: threadID, Role: "assistant", Content: "ok", CreatedAt: time.Now().UTC().Format(time.RFC3339)}, nil
}

func (f *fakeTUIAdapter) GetThread(ctx context.Context, threadID string) (*codex.Thread, error) {
	return &codex.Thread{ID: threadID, Title: "existing", CreatedAt: time.Now().UTC().Format(time.RFC3339)}, nil
}

func (f *fakeTUIAdapter) ResumeSession(ctx context.Context, threadID string) (*codex.Thread, error) {
	return &codex.Thread{ID: threadID, Title: "resumed", CreatedAt: time.Now().UTC().Format(time.RFC3339)}, nil
}

func (f *fakeTUIAdapter) ForkSession(ctx context.Context, threadID string) (*codex.Thread, error) {
	return &codex.Thread{ID: threadID + "-fork", Title: "forked", CreatedAt: time.Now().UTC().Format(time.RFC3339)}, nil
}

func (f *fakeTUIAdapter) RollbackTurns(ctx context.Context, threadID, toTurnID string) (*codex.Thread, error) {
	return &codex.Thread{ID: threadID, Title: "rolled", CreatedAt: time.Now().UTC().Format(time.RFC3339)}, nil
}

func (f *fakeTUIAdapter) SteerTurn(ctx context.Context, threadID, turnID, instruction string) (*codex.Turn, error) {
	f.lastSteerInstruction = instruction
	f.lastSteerThreadID = threadID
	f.lastSteerTargetTurnID = turnID
	if f.steerTurn != nil {
		return f.steerTurn, nil
	}
	return &codex.Turn{ID: "tu-steer-1", ThreadID: threadID, Role: "assistant", Content: "steered", CreatedAt: time.Now().UTC().Format(time.RFC3339)}, nil
}

func (f *fakeTUIAdapter) InterruptTurn(ctx context.Context, threadID, turnID string) (*codex.InterruptResult, error) {
	return &codex.InterruptResult{Interrupted: true, ThreadID: threadID, TurnID: turnID}, nil
}

func (f *fakeTUIAdapter) StreamTurn(ctx context.Context, threadID, prompt string) (<-chan codex.StreamEvent, error) {
	f.streamCalls++
	if f.streamErr != nil {
		return nil, f.streamErr
	}
	out := make(chan codex.StreamEvent, len(f.streamEvents))
	for _, evt := range f.streamEvents {
		out <- evt
	}
	close(out)
	return out, nil
}

func withFakeTUIAdapter(t *testing.T, adapter codex.AppServerAdapter) {
	t.Helper()
	prevResolve := resolveCodexConfigForTUI
	prevNew := newCodexAdapterForTUI
	resolveCodexConfigForTUI = func() codex.ClientConfig {
		return codex.ClientConfig{Mode: "real", Timeout: time.Second}
	}
	newCodexAdapterForTUI = func(codex.ClientConfig) (codex.AppServerAdapter, error) {
		return adapter, nil
	}
	t.Cleanup(func() {
		resolveCodexConfigForTUI = prevResolve
		newCodexAdapterForTUI = prevNew
	})
}

func TestExecuteTUICommandRunUsesStreamAndPersistsState(t *testing.T) {
	tmp := setupTempCWD(t)
	adapter := &fakeTUIAdapter{
		createThreadID: "th-stream-1",
		streamEvents: []codex.StreamEvent{
			{Type: codex.StreamEventDelta, ThreadID: "th-stream-1", TurnID: "tu-stream-1", Delta: "hello "},
			{Type: codex.StreamEventDelta, ThreadID: "th-stream-1", TurnID: "tu-stream-1", Delta: "world"},
			{Type: codex.StreamEventCompleted, ThreadID: "th-stream-1", TurnID: "tu-stream-1", Completed: true},
		},
	}
	withFakeTUIAdapter(t, adapter)

	result, err := executeTUICommand(tmp, runtimeContext{sessionID: "sess-tui", mode: core.RuntimeModeSingle}, "hello stream", time.Now())
	if err != nil {
		t.Fatalf("execute tui command: %v", err)
	}
	if result.Command != tuiCommandKindRun {
		t.Fatalf("expected run command, got %q", result.Command)
	}
	if result.ThreadID != "th-stream-1" {
		t.Fatalf("expected thread id th-stream-1, got %q", result.ThreadID)
	}
	if result.TurnID != "tu-stream-1" {
		t.Fatalf("expected turn id tu-stream-1, got %q", result.TurnID)
	}
	if adapter.streamCalls != 1 {
		t.Fatalf("expected exactly one stream call, got %d", adapter.streamCalls)
	}

	st := readState(t, tmp)
	if st.Run.ThreadID != "th-stream-1" {
		t.Fatalf("state thread id mismatch: %q", st.Run.ThreadID)
	}
	if len(st.Run.TurnHistory) == 0 {
		t.Fatalf("expected turn history after run command")
	}
	last := st.Run.TurnHistory[len(st.Run.TurnHistory)-1]
	if last.Step != "tui:run" {
		t.Fatalf("expected tui:run step, got %+v", last)
	}

	journal := mustRead(t, filepath.Join(tmp, ".ax", "logs", "runtime-journal.jsonl"))
	if !strings.Contains(journal, "\"command\":\"tui_command\"") || !strings.Contains(journal, "\"artifact\":\"run\"") {
		t.Fatalf("expected tui_command run journal entry, got:\n%s", journal)
	}
}

func TestExecuteTUICommandWithEventsEmitsStreamDeltaEntries(t *testing.T) {
	tmp := setupTempCWD(t)
	adapter := &fakeTUIAdapter{
		createThreadID: "th-stream-emit-1",
		streamEvents: []codex.StreamEvent{
			{Type: codex.StreamEventDelta, ThreadID: "th-stream-emit-1", TurnID: "tu-stream-emit-1", Delta: "hello "},
			{Type: codex.StreamEventDelta, ThreadID: "th-stream-emit-1", TurnID: "tu-stream-emit-1", Delta: "world"},
			{Type: codex.StreamEventCompleted, ThreadID: "th-stream-emit-1", TurnID: "tu-stream-emit-1", Completed: true},
		},
	}
	withFakeTUIAdapter(t, adapter)

	var emitted []tuiTranscriptEntry
	result, err := executeTUICommandWithEvents(tmp, runtimeContext{sessionID: "sess-stream-emit", mode: core.RuntimeModeSingle}, "/run hello stream", time.Now(), func(entry tuiTranscriptEntry) {
		emitted = append(emitted, entry)
	})
	if err != nil {
		t.Fatalf("execute tui command with events: %v", err)
	}
	if len(emitted) != 2 {
		t.Fatalf("expected 2 emitted stream delta entries, got %d (%+v)", len(emitted), emitted)
	}
	if emitted[0].Content != "hello " || emitted[1].Content != "world" {
		t.Fatalf("unexpected emitted deltas: %+v", emitted)
	}
	if len(result.Transcript) != 0 {
		t.Fatalf("expected final transcript to be omitted when live stream deltas are emitted, got %+v", result.Transcript)
	}
}

func TestExecuteTUICommandRunFallsBackWhenStreamFails(t *testing.T) {
	tmp := setupTempCWD(t)
	adapter := &fakeTUIAdapter{
		createThreadID: "th-fallback-1",
		streamErr:      errors.New("stream unavailable"),
		runTurn:        &codex.Turn{ID: "tu-fallback-1", ThreadID: "th-fallback-1", Role: "assistant", Content: "fallback", CreatedAt: time.Now().UTC().Format(time.RFC3339)},
	}
	withFakeTUIAdapter(t, adapter)

	result, err := executeTUICommand(tmp, runtimeContext{sessionID: "sess-fallback", mode: core.RuntimeModeSingle}, "fallback run", time.Now())
	if err != nil {
		t.Fatalf("execute tui command with fallback: %v", err)
	}
	if adapter.streamCalls != 1 {
		t.Fatalf("expected one stream attempt, got %d", adapter.streamCalls)
	}
	if adapter.runCalls != 1 {
		t.Fatalf("expected one run fallback call, got %d", adapter.runCalls)
	}
	if result.TurnID != "tu-fallback-1" {
		t.Fatalf("expected fallback turn id, got %q", result.TurnID)
	}
}

func TestExecuteTUICommandSteerUsesUserInstruction(t *testing.T) {
	tmp := setupTempCWD(t)
	st := readState(t, tmp)
	st.Run.ThreadID = "th-steer-1"
	st.Run.TurnHistory = []core.TurnRef{{TurnID: "tu-prev-1", Step: "Task-01", Status: "completed", StartedAt: time.Now().UTC().Format(time.RFC3339), EndedAt: time.Now().UTC().Format(time.RFC3339)}}
	if err := core.SaveState(tmp, st); err != nil {
		t.Fatalf("save state: %v", err)
	}

	adapter := &fakeTUIAdapter{}
	withFakeTUIAdapter(t, adapter)

	instruction := "tighten assertion handling"
	result, err := executeTUICommand(tmp, runtimeContext{sessionID: "sess-steer", mode: core.RuntimeModeSingle}, "/steer "+instruction, time.Now())
	if err != nil {
		t.Fatalf("execute steer command: %v", err)
	}
	if adapter.lastSteerInstruction != instruction {
		t.Fatalf("steer instruction mismatch: got=%q want=%q", adapter.lastSteerInstruction, instruction)
	}
	if adapter.lastSteerThreadID != "th-steer-1" {
		t.Fatalf("expected steer thread th-steer-1, got %q", adapter.lastSteerThreadID)
	}
	if adapter.lastSteerTargetTurnID != "tu-prev-1" {
		t.Fatalf("expected steer target tu-prev-1, got %q", adapter.lastSteerTargetTurnID)
	}
	if result.Command != tuiCommandKindSteer {
		t.Fatalf("expected steer command result, got %q", result.Command)
	}
}

func TestAppendTUIInputHistoryMasksSensitiveValues(t *testing.T) {
	tmp := setupTempCWD(t)
	now := time.Now().UTC()
	if err := appendTUIInputHistory(tmp, "sess-mask", "/run token=abcd bearer xyz password:secret", "completed", now); err != nil {
		t.Fatalf("append history: %v", err)
	}
	body := mustRead(t, tuiHistoryPath(tmp, "sess-mask"))
	if strings.Contains(body, "token=abcd") || strings.Contains(body, "xyz") || strings.Contains(body, "secret") {
		t.Fatalf("expected sensitive values to be masked, got:\n%s", body)
	}
	for _, want := range []string{"token=***", "bearer ***", "password=***"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected masked token %q, got:\n%s", want, body)
		}
	}
}

func TestExecuteTUICommandSlashLifecycleHelperProcessIntegration(t *testing.T) {
	tmp := setupTempCWD(t)
	configureTUICommandDispatcherHelperEnv(t)

	rt := runtimeContext{sessionID: "sess-tui-helper", mode: core.RuntimeModeSingle}
	runResult, err := executeTUICommand(tmp, rt, "/run helper-process slash integration", time.Now())
	if err != nil {
		t.Fatalf("/run command failed: %v", err)
	}
	if runResult.Command != tuiCommandKindRun {
		t.Fatalf("expected run command kind, got %q", runResult.Command)
	}
	if strings.TrimSpace(runResult.ThreadID) == "" || strings.TrimSpace(runResult.TurnID) == "" {
		t.Fatalf("expected non-empty thread/turn ids after /run, got thread=%q turn=%q", runResult.ThreadID, runResult.TurnID)
	}

	steerResult, err := executeTUICommand(tmp, rt, "/steer tighten assertions", time.Now())
	if err != nil {
		t.Fatalf("/steer failed: %v", err)
	}
	if steerResult.Command != tuiCommandKindSteer {
		t.Fatalf("expected steer command kind, got %q", steerResult.Command)
	}
	if strings.TrimSpace(steerResult.TurnID) == "" {
		t.Fatalf("expected steer turn id to be persisted")
	}

	interruptResult, err := executeTUICommand(tmp, rt, "/interrupt", time.Now())
	if err != nil {
		t.Fatalf("/interrupt failed: %v", err)
	}
	if interruptResult.Command != tuiCommandKindInterrupt {
		t.Fatalf("expected interrupt command kind, got %q", interruptResult.Command)
	}

	forkResult, err := executeTUICommand(tmp, rt, "/fork", time.Now())
	if err != nil {
		t.Fatalf("/fork failed: %v", err)
	}
	if forkResult.Command != tuiCommandKindFork {
		t.Fatalf("expected fork command kind, got %q", forkResult.Command)
	}
	if forkResult.ThreadID != "th-real-fork-1" {
		t.Fatalf("expected helper process forked thread id, got %q", forkResult.ThreadID)
	}

	st := readState(t, tmp)
	if len(st.Run.TurnHistory) < 2 {
		t.Fatalf("expected 2+ turns before rollback, got %+v", st.Run.TurnHistory)
	}
	targetTurnID := st.Run.TurnHistory[0].TurnID
	rollbackResult, err := executeTUICommand(tmp, rt, "/rollback "+targetTurnID, time.Now())
	if err != nil {
		t.Fatalf("/rollback failed: %v", err)
	}
	if rollbackResult.Command != tuiCommandKindRollback {
		t.Fatalf("expected rollback command kind, got %q", rollbackResult.Command)
	}
	if rollbackResult.TurnID != targetTurnID {
		t.Fatalf("expected rollback to target turn %q, got %q", targetTurnID, rollbackResult.TurnID)
	}

	st = readState(t, tmp)
	if st.Run.ThreadID != "th-real-fork-1" {
		t.Fatalf("expected persisted forked thread id, got %q", st.Run.ThreadID)
	}
	if len(st.Run.TurnHistory) != 1 || st.Run.TurnHistory[0].TurnID != targetTurnID {
		t.Fatalf("expected rollback to trim history to first turn, got %+v", st.Run.TurnHistory)
	}

	journal := mustRead(t, filepath.Join(tmp, ".ax", "logs", "runtime-journal.jsonl"))
	for _, artifact := range []string{
		"\"artifact\":\"run\"",
		"\"artifact\":\"steer\"",
		"\"artifact\":\"interrupt\"",
		"\"artifact\":\"fork\"",
		"\"artifact\":\"rollback\"",
	} {
		if !strings.Contains(journal, artifact) {
			t.Fatalf("expected journal to include %s, got:\n%s", artifact, journal)
		}
	}
}

func TestExecuteTUICommandInterruptHelperProcessFailure(t *testing.T) {
	tmp := setupTempCWD(t)
	configureTUICommandDispatcherHelperEnv(t)
	t.Setenv("AX_TEST_INTERRUPT_FAIL", "1")

	rt := runtimeContext{sessionID: "sess-tui-helper-interrupt-fail", mode: core.RuntimeModeSingle}
	if _, err := executeTUICommand(tmp, rt, "/run seed for interrupt", time.Now()); err != nil {
		t.Fatalf("seed /run failed: %v", err)
	}

	if _, err := executeTUICommand(tmp, rt, "/interrupt", time.Now()); err == nil {
		t.Fatal("expected /interrupt to fail when helper process is configured to fail interrupt")
	} else if !strings.Contains(err.Error(), "interrupt failed") {
		t.Fatalf("expected interrupt failure message, got %v", err)
	}

	st := readState(t, tmp)
	if !strings.Contains(strings.ToLower(st.Run.ErrorSummary), "interrupt failed") {
		t.Fatalf("expected interrupt failure summary in state, got %q", st.Run.ErrorSummary)
	}

	journal := mustRead(t, filepath.Join(tmp, ".ax", "logs", "runtime-journal.jsonl"))
	if !strings.Contains(journal, "\"artifact\":\"interrupt\"") || !strings.Contains(journal, "\"stage\":\"failed\"") {
		t.Fatalf("expected failed interrupt journal entry, got:\n%s", journal)
	}
}

func configureTUICommandDispatcherHelperEnv(t *testing.T) {
	t.Helper()
	t.Setenv("AX_CODEX_MODE", "real")
	t.Setenv("AX_CODEX_BIN", os.Args[0])
	t.Setenv("AX_CODEX_ARGS", "-test.run=TestHelperProcessCodexServerAX")
	t.Setenv("AX_CODEX_TIMEOUT", "5s")
	t.Setenv("AX_CODEX_RETRIES", "0")
	t.Setenv("GO_WANT_HELPER_PROCESS_AX", "1")
}
