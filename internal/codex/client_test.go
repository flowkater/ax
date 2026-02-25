package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
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

func TestStdioClient_PropagatesJSONRPCError(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	client := NewStdioClient(os.Args[0], "-test.run=TestHelperProcessCodexServer")
	_, err := client.RunTurn(context.Background(), "th-1", "force-error")
	if err == nil {
		t.Fatal("expected error")
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
	case "CreateThread":
		write(JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: Thread{ID: "th-1", Title: "hello", CreatedAt: "2026-02-26T00:00:00Z"}})
	case "RunTurn":
		params, _ := req.Params.(map[string]any)
		prompt, _ := params["prompt"].(string)
		if prompt == "force-error" {
			write(JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &JSONRPCErrorObj{Code: 500, Message: "boom"}})
			os.Exit(0)
		}
		write(JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: Turn{ID: "tu-1", ThreadID: "th-1", Role: "assistant", Content: "ok:" + prompt, CreatedAt: "2026-02-26T00:00:01Z"}})
	case "GetThread":
		write(JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: Thread{ID: "th-1", Title: "hello", CreatedAt: "2026-02-26T00:00:00Z"}})
	default:
		write(JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &JSONRPCErrorObj{Code: -32601, Message: "method not found"}})
	}
	os.Exit(0)
}
