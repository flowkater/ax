package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestStdioClient_CreateThreadRunTurnGetThread(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	client := NewStdioClient(os.Args[0], "-test.run=TestHelperProcessCodexServer")
	ctx := context.Background()

	thread, err := client.CreateThread(ctx, "hello")
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	if thread.ID != "th-1" {
		t.Fatalf("unexpected thread id: %s", thread.ID)
	}

	turn, err := client.RunTurn(ctx, "th-1", "say hi")
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if turn.Content != "ok:say hi" {
		t.Fatalf("unexpected turn content: %s", turn.Content)
	}

	got, err := client.GetThread(ctx, "th-1")
	if err != nil {
		t.Fatalf("GetThread: %v", err)
	}
	if got.ID != "th-1" {
		t.Fatalf("unexpected thread id from get: %s", got.ID)
	}
}

func TestStdioClient_LifecycleMethods(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	client := NewStdioClient(os.Args[0], "-test.run=TestHelperProcessCodexServer")
	ctx := context.Background()

	if _, err := client.ResumeSession(ctx, "sess-1"); err != nil {
		t.Fatalf("ResumeSession: %v", err)
	}
	if _, err := client.ForkSession(ctx, "th-1"); err != nil {
		t.Fatalf("ForkSession: %v", err)
	}
	if _, err := client.RollbackTurns(ctx, "th-1", "tu-1"); err != nil {
		t.Fatalf("RollbackTurns: %v", err)
	}
	if _, err := client.SteerTurn(ctx, "th-1", "tu-1", "go safer"); err != nil {
		t.Fatalf("SteerTurn: %v", err)
	}
	if _, err := client.InterruptTurn(ctx, "th-1", "tu-1"); err != nil {
		t.Fatalf("InterruptTurn: %v", err)
	}
}

func TestStdioClient_StreamTurn(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	client := NewStdioClient(os.Args[0], "-test.run=TestHelperProcessCodexServer")
	events, err := client.StreamTurn(context.Background(), "th-1", "stream")
	if err != nil {
		t.Fatalf("StreamTurn: %v", err)
	}
	var got []StreamEvent
	for evt := range events {
		got = append(got, evt)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 events, got %d", len(got))
	}
	if got[0].Type != "delta" || got[1].Type != "completed" {
		t.Fatalf("unexpected events: %#v", got)
	}
}

func TestStdioClient_StreamTurnAutoAppendsCompleted(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	client := NewStdioClient(os.Args[0], "-test.run=TestHelperProcessCodexServer")
	events, err := client.StreamTurn(context.Background(), "th-1", "stream-no-complete")
	if err != nil {
		t.Fatalf("StreamTurn: %v", err)
	}
	var got []StreamEvent
	for evt := range events {
		got = append(got, evt)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 events (delta + synthetic completed), got %d", len(got))
	}
	if got[1].Type != StreamEventCompleted || !got[1].Completed {
		t.Fatalf("expected synthetic completed event, got %#v", got[1])
	}
}

func TestStdioClient_PropagatesJSONRPCError(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	client := NewStdioClient(os.Args[0], "-test.run=TestHelperProcessCodexServer")
	_, err := client.RunTurn(context.Background(), "th-1", "force-error")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestStdioClient_RunTurnHandlesLargeJSONPayload(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	client := NewStdioClient(os.Args[0], "-test.run=TestHelperProcessCodexServer")

	turn, err := client.RunTurn(context.Background(), "th-1", "large-payload")
	if err != nil {
		t.Fatalf("RunTurn large payload: %v", err)
	}
	if len(turn.Content) < 70000 {
		t.Fatalf("expected large content, got len=%d", len(turn.Content))
	}
}

func TestSleepWithContext_Cancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sleepWithContext(ctx, 100*time.Millisecond); err == nil {
		t.Fatal("expected cancellation error")
	}
}

func TestHelperProcessCodexServer(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	var req JSONRPCRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		fmt.Fprintf(os.Stdout, `{"jsonrpc":"2.0","id":"0","error":{"code":-32700,"message":"parse error"}}\n`)
		os.Exit(0)
	}

	write := func(v any) {
		_ = json.NewEncoder(os.Stdout).Encode(v)
	}

	switch req.Method {
	case "thread/start":
		write(JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: Thread{ID: "th-1", Title: "hello", CreatedAt: "2026-02-26T00:00:00Z"}})
	case "turn/start":
		params, _ := req.Params.(map[string]any)
		if stream, _ := params["stream"].(bool); stream {
			prompt, _ := params["prompt"].(string)
			if prompt == "stream-no-complete" {
				events := []StreamEvent{
					{Type: "delta", ThreadID: "th-1", TurnID: "tu-3", Delta: "partial"},
				}
				write(JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: events})
				os.Exit(0)
			}
			events := []StreamEvent{
				{Type: "delta", ThreadID: "th-1", TurnID: "tu-3", Delta: "Hello"},
				{Type: "completed", ThreadID: "th-1", TurnID: "tu-3", Completed: true},
			}
			write(JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: events})
			os.Exit(0)
		}
		prompt, _ := params["prompt"].(string)
		if prompt == "force-error" {
			write(JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &JSONRPCErrorObj{Code: 500, Message: "boom"}})
			os.Exit(0)
		}
		if prompt == "large-payload" {
			write(JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: Turn{ID: "tu-large", ThreadID: "th-1", Role: "assistant", Content: strings.Repeat("L", 70000), CreatedAt: "2026-02-26T00:00:05Z"}})
			os.Exit(0)
		}
		write(JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: Turn{ID: "tu-1", ThreadID: "th-1", Role: "assistant", Content: "ok:" + prompt, CreatedAt: "2026-02-26T00:00:01Z"}})
	case "thread/read":
		write(JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: Thread{ID: "th-1", Title: "hello", CreatedAt: "2026-02-26T00:00:00Z"}})
	case "thread/resume":
		write(JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: Thread{ID: "th-1", Title: "resumed", CreatedAt: "2026-02-26T00:00:00Z"}})
	case "thread/fork":
		write(JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: Thread{ID: "th-2", Title: "fork", CreatedAt: "2026-02-26T00:00:02Z"}})
	case "thread/rollback":
		write(JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: Thread{ID: "th-1", Title: "rolled back", CreatedAt: "2026-02-26T00:00:03Z"}})
	case "review/start":
		write(JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: Turn{ID: "tu-2", ThreadID: "th-1", Role: "assistant", Content: "steered", CreatedAt: "2026-02-26T00:00:04Z"}})
	case "turn/interrupt":
		write(JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"interrupted": true}})
	default:
		write(JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &JSONRPCErrorObj{Code: -32601, Message: "method not found"}})
	}
	os.Exit(0)
}
