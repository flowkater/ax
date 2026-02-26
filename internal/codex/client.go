package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync/atomic"
	"time"
)

var errNoResponse = errors.New("no JSON-RPC response from app-server")

type jsonRPCError struct {
	code    int
	message string
}

func (e *jsonRPCError) Error() string {
	return fmt.Sprintf("json-rpc error (%d): %s", e.code, e.message)
}

type StdioClient struct {
	command string
	args    []string
	seq     uint64

	timeout time.Duration
	retries int
	backoff []time.Duration
}

func NewStdioClient(command string, args ...string) *StdioClient {
	return &StdioClient{
		command: command,
		args:    args,
		timeout: 30 * time.Second,
		retries: 2,
		backoff: []time.Duration{time.Second, 2 * time.Second},
	}
}

func (c *StdioClient) CreateThread(ctx context.Context, title string) (*Thread, error) {
	var out Thread
	if err := c.call(ctx, "CreateThread", map[string]any{"title": title}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *StdioClient) RunTurn(ctx context.Context, threadID, prompt string) (*Turn, error) {
	var out Turn
	if err := c.call(ctx, "RunTurn", map[string]any{"thread_id": threadID, "prompt": prompt}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *StdioClient) GetThread(ctx context.Context, threadID string) (*Thread, error) {
	var out Thread
	if err := c.call(ctx, "GetThread", map[string]any{"thread_id": threadID}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *StdioClient) ResumeSession(ctx context.Context, threadID string) (*Thread, error) {
	var out Thread
	if err := c.call(ctx, "ResumeSession", map[string]any{"thread_id": threadID}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *StdioClient) ForkSession(ctx context.Context, threadID string) (*Thread, error) {
	var out Thread
	if err := c.call(ctx, "ForkSession", map[string]any{"thread_id": threadID}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *StdioClient) RollbackTurns(ctx context.Context, threadID, toTurnID string) (*Thread, error) {
	var out Thread
	if err := c.call(ctx, "RollbackTurns", map[string]any{"thread_id": threadID, "to_turn_id": toTurnID}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *StdioClient) SteerTurn(ctx context.Context, threadID, turnID, instruction string) (*Turn, error) {
	var out Turn
	params := map[string]any{
		"thread_id":   threadID,
		"turn_id":     turnID,
		"instruction": instruction,
	}
	if err := c.call(ctx, "SteerTurn", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *StdioClient) InterruptTurn(ctx context.Context, threadID, turnID string) (*InterruptResult, error) {
	var out InterruptResult
	if err := c.call(ctx, "InterruptTurn", map[string]any{"thread_id": threadID, "turn_id": turnID}, &out); err != nil {
		return nil, err
	}
	if !out.Interrupted {
		return nil, errors.New("interrupt was not acknowledged by app-server")
	}
	return &out, nil
}

func (c *StdioClient) StreamTurn(ctx context.Context, threadID, prompt string) (<-chan StreamEvent, error) {
	var events []StreamEvent
	if err := c.call(ctx, "RunTurnStream", map[string]any{"thread_id": threadID, "prompt": prompt}, &events); err != nil {
		return nil, err
	}
	events = normalizeStreamEvents(events, threadID)
	out := make(chan StreamEvent, len(events))
	for _, evt := range events {
		out <- evt
	}
	close(out)
	return out, nil
}

func (c *StdioClient) call(ctx context.Context, method string, params any, out any) error {
	if c.command == "" {
		return errors.New("stdio client command is required")
	}

	attempts := c.retries + 1
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		callCtx := ctx
		cancel := func() {}
		if _, ok := ctx.Deadline(); !ok && c.timeout > 0 {
			callCtx, cancel = context.WithTimeout(ctx, c.timeout)
		}
		err := c.callOnce(callCtx, method, params, out)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		if !isRetryable(err) || attempt == attempts-1 {
			break
		}
		if err := sleepWithContext(ctx, c.backoffForAttempt(attempt)); err != nil {
			return err
		}
	}
	return lastErr
}

func (c *StdioClient) callOnce(ctx context.Context, method string, params any, out any) error {
	cmd := exec.CommandContext(ctx, c.command, c.args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      fmt.Sprintf("%d", atomic.AddUint64(&c.seq, 1)),
		Method:  method,
		Params:  params,
	}
	if err := json.NewEncoder(stdin).Encode(req); err != nil {
		_ = stdin.Close()
		_ = cmd.Wait()
		return err
	}
	_ = stdin.Close()

	var resp JSONRPCResponse
	if err := json.NewDecoder(stdout).Decode(&resp); err != nil {
		_ = cmd.Wait()
		if errors.Is(err, io.EOF) {
			return errNoResponse
		}
		return err
	}
	if resp.Error != nil {
		_ = cmd.Wait()
		return &jsonRPCError{code: resp.Error.Code, message: resp.Error.Message}
	}

	payload, err := json.Marshal(resp.Result)
	if err != nil {
		_ = cmd.Wait()
		return err
	}
	if err := json.Unmarshal(payload, out); err != nil {
		_ = cmd.Wait()
		return err
	}

	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

func isRetryable(err error) bool {
	var rpcErr *jsonRPCError
	if errors.As(err, &rpcErr) {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, errNoResponse) {
		return true
	}
	return false
}

func (c *StdioClient) backoffForAttempt(attempt int) time.Duration {
	if attempt < 0 {
		return 0
	}
	if attempt >= len(c.backoff) {
		if len(c.backoff) == 0 {
			return 0
		}
		return c.backoff[len(c.backoff)-1]
	}
	return c.backoff[attempt]
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func normalizeStreamEvents(events []StreamEvent, threadID string) []StreamEvent {
	if len(events) == 0 {
		return []StreamEvent{{Type: StreamEventCompleted, ThreadID: threadID, Completed: true}}
	}

	normalized := make([]StreamEvent, 0, len(events)+1)
	completedSeen := false
	lastTurnID := ""
	for _, evt := range events {
		if evt.ThreadID == "" {
			evt.ThreadID = threadID
		}
		if evt.Type == "" {
			if evt.Completed {
				evt.Type = StreamEventCompleted
			} else {
				evt.Type = StreamEventDelta
			}
		}
		if evt.TurnID != "" {
			lastTurnID = evt.TurnID
		}
		if evt.Type == StreamEventCompleted || evt.Completed {
			evt.Type = StreamEventCompleted
			evt.Completed = true
			completedSeen = true
		}
		normalized = append(normalized, evt)
	}

	if !completedSeen {
		normalized = append(normalized, StreamEvent{
			Type:      StreamEventCompleted,
			ThreadID:  threadID,
			TurnID:    lastTurnID,
			Completed: true,
		})
	}

	return normalized
}
