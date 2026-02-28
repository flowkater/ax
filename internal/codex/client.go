package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
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

	mu          sync.Mutex
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	decoder     *json.Decoder
	stderr      bytes.Buffer
	initialized bool
}

const (
	rpcMethodInitialize     = "initialize"
	rpcMethodInitialized    = "initialized"
	rpcMethodThreadStart    = "thread/start"
	rpcMethodThreadRead     = "thread/read"
	rpcMethodThreadResume   = "thread/resume"
	rpcMethodThreadFork     = "thread/fork"
	rpcMethodThreadRollback = "thread/rollback"
	rpcMethodTurnStart      = "turn/start"
	rpcMethodTurnInterrupt  = "turn/interrupt"
	rpcMethodReviewStart    = "review/start"
)

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
	err := c.callAndDecode(ctx, rpcMethodThreadStart, map[string]any{
		"approvalPolicy": "never",
	}, func(raw json.RawMessage) error {
		thread, err := decodeThreadResult(raw)
		if err != nil {
			return err
		}
		out = thread
		if strings.TrimSpace(out.Title) == "" {
			out.Title = title
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *StdioClient) RunTurn(ctx context.Context, threadID, prompt string) (*Turn, error) {
	turn, _, err := c.callTurn(ctx, threadID, prompt, false)
	if err != nil {
		return nil, err
	}
	return turn, nil
}

func (c *StdioClient) GetThread(ctx context.Context, threadID string) (*Thread, error) {
	var out Thread
	err := c.callAndDecode(ctx, rpcMethodThreadRead, map[string]any{
		"threadId": threadID,
	}, func(raw json.RawMessage) error {
		thread, err := decodeThreadResult(raw)
		if err != nil {
			return err
		}
		out = thread
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *StdioClient) ResumeSession(ctx context.Context, threadID string) (*Thread, error) {
	var out Thread
	err := c.callAndDecode(ctx, rpcMethodThreadResume, map[string]any{
		"threadId":       threadID,
		"approvalPolicy": "never",
	}, func(raw json.RawMessage) error {
		thread, err := decodeThreadResult(raw)
		if err != nil {
			return err
		}
		out = thread
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *StdioClient) ForkSession(ctx context.Context, threadID string) (*Thread, error) {
	var out Thread
	err := c.callAndDecode(ctx, rpcMethodThreadFork, map[string]any{
		"threadId":       threadID,
		"approvalPolicy": "never",
	}, func(raw json.RawMessage) error {
		thread, err := decodeThreadResult(raw)
		if err != nil {
			return err
		}
		out = thread
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *StdioClient) RollbackTurns(ctx context.Context, threadID, toTurnID string) (*Thread, error) {
	var out Thread
	params := map[string]any{
		"threadId": threadID,
		"numTurns": 1,
	}
	err := c.callAndDecode(ctx, rpcMethodThreadRollback, params, func(raw json.RawMessage) error {
		thread, err := decodeThreadResult(raw)
		if err != nil {
			return err
		}
		out = thread
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *StdioClient) SteerTurn(ctx context.Context, threadID, turnID, instruction string) (*Turn, error) {
	var out Turn
	params := map[string]any{
		"threadId": threadID,
		"target": map[string]any{
			"type":         "custom",
			"instructions": instruction,
		},
	}
	err := c.callAndDecode(ctx, rpcMethodReviewStart, params, func(raw json.RawMessage) error {
		turn, _, _, err := decodeTurnFromResult(raw, threadID)
		if err != nil {
			return err
		}
		out = turn
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *StdioClient) InterruptTurn(ctx context.Context, threadID, turnID string) (*InterruptResult, error) {
	if strings.TrimSpace(turnID) == "" {
		return &InterruptResult{Interrupted: true, ThreadID: threadID, TurnID: turnID}, nil
	}
	var out InterruptResult
	err := c.callAndDecode(ctx, rpcMethodTurnInterrupt, map[string]any{
		"threadId": threadID,
		"turnId":   turnID,
	}, func(raw json.RawMessage) error {
		parsed, err := decodeInterruptResult(raw)
		if err != nil {
			return err
		}
		out = parsed
		if strings.TrimSpace(out.ThreadID) == "" {
			out.ThreadID = threadID
		}
		if strings.TrimSpace(out.TurnID) == "" {
			out.TurnID = turnID
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !out.Interrupted {
		return nil, errors.New("interrupt was not acknowledged by app-server")
	}
	return &out, nil
}

func (c *StdioClient) StreamTurn(ctx context.Context, threadID, prompt string) (<-chan StreamEvent, error) {
	_, events, err := c.callTurn(ctx, threadID, prompt, true)
	if err != nil {
		return nil, err
	}
	out := make(chan StreamEvent, len(events))
	for _, evt := range events {
		out <- evt
	}
	close(out)
	return out, nil
}

func (c *StdioClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closeProcessLocked()
}

func (c *StdioClient) callAndDecode(ctx context.Context, method string, params any, decode func(json.RawMessage) error) error {
	if c.command == "" {
		return errors.New("stdio client command is required")
	}
	return c.doWithRetries(ctx, func(callCtx context.Context) error {
		c.mu.Lock()
		defer c.mu.Unlock()

		if err := c.ensureProcessLocked(); err != nil {
			return err
		}
		if err := c.ensureInitializedLocked(callCtx); err != nil {
			_ = c.closeProcessLocked()
			return err
		}

		id := fmt.Sprintf("%d", atomic.AddUint64(&c.seq, 1))
		if err := c.sendRequestLocked(JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      id,
			Method:  method,
			Params:  params,
		}); err != nil {
			_ = c.closeProcessLocked()
			return err
		}

		resp, err := c.waitForResponseLocked(callCtx, id, nil)
		if err != nil {
			return err
		}
		if resp.errObj != nil {
			return &jsonRPCError{code: resp.errObj.Code, message: resp.errObj.Message}
		}
		return decode(resp.result)
	})
}

func (c *StdioClient) callTurn(ctx context.Context, threadID, prompt string, stream bool) (*Turn, []StreamEvent, error) {
	if c.command == "" {
		return nil, nil, errors.New("stdio client command is required")
	}

	var (
		finalTurn   *Turn
		finalEvents []StreamEvent
	)
	err := c.doWithRetries(ctx, func(callCtx context.Context) error {
		turn, events, err := c.callTurnOnce(callCtx, threadID, prompt, stream)
		if err != nil {
			return err
		}
		finalTurn = turn
		finalEvents = events
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return finalTurn, finalEvents, nil
}

func (c *StdioClient) callTurnOnce(ctx context.Context, threadID, prompt string, stream bool) (*Turn, []StreamEvent, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureProcessLocked(); err != nil {
		return nil, nil, err
	}
	if err := c.ensureInitializedLocked(ctx); err != nil {
		_ = c.closeProcessLocked()
		return nil, nil, err
	}

	id := fmt.Sprintf("%d", atomic.AddUint64(&c.seq, 1))
	if err := c.sendRequestLocked(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  rpcMethodTurnStart,
		Params:  buildTurnStartParams(threadID, prompt, stream),
	}); err != nil {
		_ = c.closeProcessLocked()
		return nil, nil, err
	}

	tracker := newTurnTracker(threadID)
	resp, err := c.waitForResponseLocked(ctx, id, tracker.observe)
	if err != nil {
		return nil, nil, err
	}
	if resp.errObj != nil {
		return nil, nil, &jsonRPCError{code: resp.errObj.Code, message: resp.errObj.Message}
	}

	if events, ok := decodeStreamEventsResult(resp.result); ok {
		events = normalizeStreamEvents(events, threadID)
		turn := turnFromStreamEvents(events, threadID)
		if turn == nil {
			turn = &Turn{ThreadID: threadID}
		}
		return turn, events, nil
	}

	turn, wrapped, status, err := decodeTurnFromResult(resp.result, threadID)
	if err != nil {
		return nil, nil, err
	}
	if turn.ID == "" {
		turn.ID = tracker.lastTurnID()
	}

	needsCompletion := wrapped &&
		strings.TrimSpace(turn.Content) == "" &&
		!strings.EqualFold(strings.TrimSpace(status), "completed")
	if needsCompletion {
		targetTurnID := strings.TrimSpace(turn.ID)
		if targetTurnID == "" {
			return nil, nil, errors.New("turn/start response missing turn id")
		}
		if !tracker.completed(targetTurnID) {
			if err := c.waitForTurnCompletionLocked(ctx, targetTurnID, tracker); err != nil {
				return nil, nil, err
			}
		}
		if completedTurn := tracker.completedTurn(); completedTurn != nil {
			if completedTurn.ID != "" {
				turn.ID = completedTurn.ID
			}
			if completedTurn.ThreadID != "" {
				turn.ThreadID = completedTurn.ThreadID
			}
			if completedTurn.Content != "" {
				turn.Content = completedTurn.Content
			}
		}
	}

	if turn.ThreadID == "" {
		turn.ThreadID = threadID
	}
	if turn.Content == "" {
		turn.Content = tracker.joinedDelta()
	}

	events := tracker.events(turn.ID)
	if stream {
		events = normalizeStreamEvents(events, threadID)
	} else {
		events = nil
	}
	return &turn, events, nil
}

func (c *StdioClient) waitForTurnCompletionLocked(ctx context.Context, targetTurnID string, tracker *turnTracker) error {
	for {
		msg, err := c.readMessageLocked(ctx)
		if err != nil {
			return err
		}

		if msg.hasID {
			if msg.method != "" {
				_ = c.replyMethodNotSupportedLocked(msg.id, msg.method)
			}
			continue
		}
		if msg.method == "" {
			continue
		}
		tracker.observe(msg)
		if tracker.completed(targetTurnID) {
			return nil
		}
	}
}

func (c *StdioClient) doWithRetries(ctx context.Context, fn func(context.Context) error) error {
	attempts := c.retries + 1
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		callCtx := ctx
		cancel := func() {}
		if _, ok := ctx.Deadline(); !ok && c.timeout > 0 {
			callCtx, cancel = context.WithTimeout(ctx, c.timeout)
		}
		err := fn(callCtx)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		if !isRetryable(err) || attempt == attempts-1 {
			break
		}
		c.resetProcess()
		if err := sleepWithContext(ctx, c.backoffForAttempt(attempt)); err != nil {
			return err
		}
	}
	return lastErr
}

func (c *StdioClient) ensureProcessLocked() error {
	if c.cmd != nil {
		return nil
	}

	cmd := exec.Command(c.command, c.args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return err
	}
	c.stderr.Reset()
	cmd.Stderr = &c.stderr
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return err
	}

	c.cmd = cmd
	c.stdin = stdin
	c.decoder = json.NewDecoder(stdout)
	c.initialized = false
	return nil
}

func (c *StdioClient) ensureInitializedLocked(ctx context.Context) error {
	if c.initialized {
		return nil
	}
	const id = "ax-init"
	if err := c.sendRequestLocked(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  rpcMethodInitialize,
		Params: map[string]any{
			"clientInfo": map[string]any{
				"name":    "ax",
				"version": "0.0.0",
			},
		},
	}); err != nil {
		return err
	}
	resp, err := c.waitForResponseLocked(ctx, id, nil)
	if err != nil {
		return err
	}
	if resp.errObj != nil {
		return &jsonRPCError{code: resp.errObj.Code, message: resp.errObj.Message}
	}
	if err := c.sendNotificationLocked(rpcMethodInitialized, nil); err != nil {
		return err
	}
	c.initialized = true
	return nil
}

func (c *StdioClient) sendRequestLocked(req JSONRPCRequest) error {
	if c.stdin == nil {
		return errors.New("app-server stdin is not available")
	}
	if req.JSONRPC == "" {
		req.JSONRPC = "2.0"
	}
	if err := json.NewEncoder(c.stdin).Encode(req); err != nil {
		_ = c.closeProcessLocked()
		return err
	}
	return nil
}

func (c *StdioClient) sendNotificationLocked(method string, params any) error {
	if c.stdin == nil {
		return errors.New("app-server stdin is not available")
	}
	payload := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		payload["params"] = params
	}
	if err := json.NewEncoder(c.stdin).Encode(payload); err != nil {
		_ = c.closeProcessLocked()
		return err
	}
	return nil
}

func (c *StdioClient) replyMethodNotSupportedLocked(id, method string) error {
	if c.stdin == nil {
		return nil
	}
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]any{
			"code":    -32601,
			"message": fmt.Sprintf("method not supported by ax bridge: %s", method),
		},
	}
	return json.NewEncoder(c.stdin).Encode(payload)
}

func (c *StdioClient) waitForResponseLocked(ctx context.Context, targetID string, onNotification func(rpcMessage)) (rpcMessage, error) {
	for {
		msg, err := c.readMessageLocked(ctx)
		if err != nil {
			return rpcMessage{}, err
		}

		if msg.hasID {
			if msg.method != "" && len(msg.result) == 0 && msg.errObj == nil {
				_ = c.replyMethodNotSupportedLocked(msg.id, msg.method)
				continue
			}
			if msg.id == targetID {
				return msg, nil
			}
			continue
		}

		if msg.method != "" && onNotification != nil {
			onNotification(msg)
		}
	}
}

func (c *StdioClient) readMessageLocked(ctx context.Context) (rpcMessage, error) {
	if c.decoder == nil {
		return rpcMessage{}, errors.New("app-server stdout is not available")
	}

	done := make(chan struct{})
	cmd := c.cmd
	if ctx != nil {
		go func() {
			select {
			case <-ctx.Done():
				if cmd != nil && cmd.Process != nil {
					_ = cmd.Process.Kill()
				}
			case <-done:
			}
		}()
	}

	var raw map[string]json.RawMessage
	err := c.decoder.Decode(&raw)
	close(done)
	if err != nil {
		if errors.Is(err, io.EOF) {
			_ = c.closeProcessLocked()
			if ctx != nil && ctx.Err() != nil {
				return rpcMessage{}, ctx.Err()
			}
			return rpcMessage{}, errNoResponse
		}
		_ = c.closeProcessLocked()
		if ctx != nil && ctx.Err() != nil {
			return rpcMessage{}, ctx.Err()
		}
		return rpcMessage{}, err
	}

	msg := rpcMessage{}
	if rawID, ok := raw["id"]; ok && len(rawID) > 0 && string(rawID) != "null" {
		msg.hasID = true
		msg.id = canonicalRequestID(rawID)
	}
	if rawMethod, ok := raw["method"]; ok {
		_ = json.Unmarshal(rawMethod, &msg.method)
	}
	if rawParams, ok := raw["params"]; ok {
		msg.params = rawParams
	}
	if rawResult, ok := raw["result"]; ok {
		msg.result = rawResult
	}
	if rawErr, ok := raw["error"]; ok && len(rawErr) > 0 && string(rawErr) != "null" {
		var rpcErr JSONRPCErrorObj
		if err := json.Unmarshal(rawErr, &rpcErr); err == nil {
			msg.errObj = &rpcErr
		} else {
			msg.errObj = &JSONRPCErrorObj{Code: -32603, Message: string(rawErr)}
		}
	}
	return msg, nil
}

func (c *StdioClient) resetProcess() {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.closeProcessLocked()
}

func (c *StdioClient) closeProcessLocked() error {
	var closeErr error
	if c.stdin != nil {
		if err := c.stdin.Close(); err != nil {
			closeErr = err
		}
	}
	if c.cmd != nil {
		done := make(chan error, 1)
		go func(cmd *exec.Cmd) {
			done <- cmd.Wait()
		}(c.cmd)
		select {
		case err := <-done:
			if err != nil && closeErr == nil {
				closeErr = err
			}
		case <-time.After(2 * time.Second):
			if c.cmd.Process != nil {
				_ = c.cmd.Process.Kill()
			}
			if err := <-done; err != nil && closeErr == nil {
				closeErr = err
			}
		}
	}
	c.cmd = nil
	c.stdin = nil
	c.decoder = nil
	c.initialized = false
	c.stderr.Reset()
	return closeErr
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

type rpcMessage struct {
	id     string
	hasID  bool
	method string
	params json.RawMessage
	result json.RawMessage
	errObj *JSONRPCErrorObj
}

type turnTracker struct {
	threadID       string
	deltas         []string
	lastTurn       string
	completedTurnV *Turn
	hasCompleted   bool
	eventsV        []StreamEvent
}

func newTurnTracker(threadID string) *turnTracker {
	return &turnTracker{threadID: threadID}
}

func (t *turnTracker) observe(msg rpcMessage) {
	switch msg.method {
	case "item/agentMessage/delta":
		var payload struct {
			ThreadID string `json:"threadId"`
			TurnID   string `json:"turnId"`
			Delta    string `json:"delta"`
		}
		if err := json.Unmarshal(msg.params, &payload); err != nil {
			return
		}
		if payload.TurnID == "" {
			return
		}
		t.lastTurn = payload.TurnID
		if payload.Delta != "" {
			t.deltas = append(t.deltas, payload.Delta)
			t.eventsV = append(t.eventsV, StreamEvent{
				Type:     StreamEventDelta,
				ThreadID: emptyFallbackString(payload.ThreadID, t.threadID),
				TurnID:   payload.TurnID,
				Delta:    payload.Delta,
			})
		}
	case "turn/completed":
		var payload struct {
			ThreadID string          `json:"threadId"`
			Turn     json.RawMessage `json:"turn"`
		}
		if err := json.Unmarshal(msg.params, &payload); err != nil {
			return
		}
		turn, _, err := decodeTurnObject(payload.Turn, emptyFallbackString(payload.ThreadID, t.threadID))
		if err != nil {
			return
		}
		t.hasCompleted = true
		if turn.ID != "" {
			t.lastTurn = turn.ID
		}
		if turn.Content == "" {
			turn.Content = strings.Join(t.deltas, "")
		}
		t.completedTurnV = &turn
		t.eventsV = append(t.eventsV, StreamEvent{
			Type:      StreamEventCompleted,
			ThreadID:  emptyFallbackString(turn.ThreadID, t.threadID),
			TurnID:    turn.ID,
			Completed: true,
		})
	}
}

func (t *turnTracker) completed(turnID string) bool {
	if !t.hasCompleted {
		return false
	}
	if strings.TrimSpace(turnID) == "" {
		return true
	}
	if t.completedTurnV == nil {
		return false
	}
	return strings.TrimSpace(t.completedTurnV.ID) == strings.TrimSpace(turnID)
}

func (t *turnTracker) lastTurnID() string {
	if t.completedTurnV != nil && strings.TrimSpace(t.completedTurnV.ID) != "" {
		return strings.TrimSpace(t.completedTurnV.ID)
	}
	return strings.TrimSpace(t.lastTurn)
}

func (t *turnTracker) completedTurn() *Turn {
	if t.completedTurnV == nil {
		return nil
	}
	cp := *t.completedTurnV
	return &cp
}

func (t *turnTracker) joinedDelta() string {
	return strings.Join(t.deltas, "")
}

func (t *turnTracker) events(turnID string) []StreamEvent {
	events := make([]StreamEvent, 0, len(t.eventsV)+2)
	for _, evt := range t.eventsV {
		events = append(events, evt)
	}
	if len(events) == 0 {
		delta := t.joinedDelta()
		if delta == "" {
			delta = "stream:delta"
		}
		events = append(events, StreamEvent{
			Type:     StreamEventDelta,
			ThreadID: t.threadID,
			TurnID:   turnID,
			Delta:    delta,
		})
		events = append(events, StreamEvent{
			Type:      StreamEventCompleted,
			ThreadID:  t.threadID,
			TurnID:    turnID,
			Completed: true,
		})
	}
	return events
}

func buildTurnStartParams(threadID, prompt string, stream bool) map[string]any {
	params := map[string]any{
		"threadId":       threadID,
		"approvalPolicy": "never",
		"input": []map[string]any{
			{
				"type": "text",
				"text": prompt,
			},
		},
	}
	if stream {
		params["stream"] = true
	}
	return params
}

func decodeThreadResult(raw json.RawMessage) (Thread, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return Thread{}, errors.New("thread response missing result")
	}
	var wrapper struct {
		Thread json.RawMessage `json:"thread"`
	}
	if err := json.Unmarshal(raw, &wrapper); err == nil && len(wrapper.Thread) > 0 && string(wrapper.Thread) != "null" {
		return decodeThreadObject(wrapper.Thread)
	}
	return decodeThreadObject(raw)
}

func decodeThreadObject(raw json.RawMessage) (Thread, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return Thread{}, errors.New("thread payload missing")
	}
	var payload struct {
		ID          string          `json:"id"`
		Title       string          `json:"title"`
		CreatedAtV1 string          `json:"created_at"`
		CreatedAtV2 json.RawMessage `json:"createdAt"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return Thread{}, err
	}
	thread := Thread{
		ID:    payload.ID,
		Title: payload.Title,
	}
	if strings.TrimSpace(payload.CreatedAtV1) != "" {
		thread.CreatedAt = payload.CreatedAtV1
	} else {
		thread.CreatedAt = formatFlexibleTime(payload.CreatedAtV2)
	}
	return thread, nil
}

func decodeTurnFromResult(raw json.RawMessage, defaultThreadID string) (Turn, bool, string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return Turn{}, false, "", errors.New("turn response missing result")
	}
	var wrapper struct {
		Turn json.RawMessage `json:"turn"`
	}
	if err := json.Unmarshal(raw, &wrapper); err == nil && len(wrapper.Turn) > 0 && string(wrapper.Turn) != "null" {
		turn, status, err := decodeTurnObject(wrapper.Turn, defaultThreadID)
		return turn, true, status, err
	}
	turn, status, err := decodeTurnObject(raw, defaultThreadID)
	return turn, false, status, err
}

func decodeTurnObject(raw json.RawMessage, defaultThreadID string) (Turn, string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return Turn{}, "", errors.New("turn payload missing")
	}
	var payload struct {
		ID          string          `json:"id"`
		ThreadID    string          `json:"thread_id"`
		ThreadIDV2  string          `json:"threadId"`
		Role        string          `json:"role"`
		Content     string          `json:"content"`
		Text        string          `json:"text"`
		CreatedAtV1 string          `json:"created_at"`
		CreatedAtV2 json.RawMessage `json:"createdAt"`
		Status      string          `json:"status"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return Turn{}, "", err
	}
	if strings.TrimSpace(payload.ID) == "" &&
		strings.TrimSpace(payload.Status) == "" &&
		strings.TrimSpace(payload.Content) == "" &&
		strings.TrimSpace(payload.Text) == "" {
		return Turn{}, "", errors.New("turn response missing id")
	}

	turn := Turn{
		ID:       payload.ID,
		ThreadID: emptyFallbackString(payload.ThreadID, payload.ThreadIDV2, defaultThreadID),
		Role:     payload.Role,
		Content:  emptyFallbackString(payload.Content, payload.Text),
	}
	if strings.TrimSpace(payload.CreatedAtV1) != "" {
		turn.CreatedAt = payload.CreatedAtV1
	} else {
		turn.CreatedAt = formatFlexibleTime(payload.CreatedAtV2)
	}
	return turn, payload.Status, nil
}

func decodeInterruptResult(raw json.RawMessage) (InterruptResult, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" || trimmed == "{}" {
		return InterruptResult{Interrupted: true}, nil
	}
	var out InterruptResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return InterruptResult{}, err
	}
	hasInterrupted := bytes.Contains(raw, []byte(`"interrupted"`))
	if !hasInterrupted {
		out.Interrupted = true
	}
	if hasInterrupted && !out.Interrupted {
		return InterruptResult{}, errors.New("interrupt was not acknowledged by app-server")
	}
	if out.ThreadID == "" {
		var payload struct {
			ThreadID string `json:"threadId"`
		}
		if err := json.Unmarshal(raw, &payload); err == nil {
			out.ThreadID = payload.ThreadID
		}
	}
	if out.TurnID == "" {
		var payload struct {
			TurnID string `json:"turnId"`
		}
		if err := json.Unmarshal(raw, &payload); err == nil {
			out.TurnID = payload.TurnID
		}
	}
	return out, nil
}

func decodeStreamEventsResult(raw json.RawMessage) ([]StreamEvent, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, false
	}
	var events []StreamEvent
	if err := json.Unmarshal(raw, &events); err != nil {
		return nil, false
	}
	return events, true
}

func turnFromStreamEvents(events []StreamEvent, fallbackThreadID string) *Turn {
	if len(events) == 0 {
		return nil
	}
	turn := &Turn{ThreadID: fallbackThreadID}
	for _, evt := range events {
		if evt.TurnID != "" {
			turn.ID = evt.TurnID
		}
		if evt.ThreadID != "" {
			turn.ThreadID = evt.ThreadID
		}
		if evt.Delta != "" {
			turn.Content += evt.Delta
		}
	}
	return turn
}

func canonicalRequestID(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var num json.Number
	if err := decoder.Decode(&num); err == nil {
		return num.String()
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return strings.TrimSpace(string(raw))
}

func formatFlexibleTime(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var num json.Number
	if err := decoder.Decode(&num); err == nil {
		if i64, err := num.Int64(); err == nil {
			return time.Unix(i64, 0).UTC().Format(time.RFC3339)
		}
		if f64, err := num.Float64(); err == nil {
			return time.Unix(int64(f64), 0).UTC().Format(time.RFC3339)
		}
	}
	return strings.TrimSpace(string(raw))
}

func emptyFallbackString(primary string, fallbacks ...string) string {
	if strings.TrimSpace(primary) != "" {
		return strings.TrimSpace(primary)
	}
	for _, candidate := range fallbacks {
		if strings.TrimSpace(candidate) != "" {
			return strings.TrimSpace(candidate)
		}
	}
	return ""
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
