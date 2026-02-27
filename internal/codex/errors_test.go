package codex

import (
	"context"
	"testing"
)

func TestMapCodexError_JSONRPCMappings(t *testing.T) {
	cases := []struct {
		code int
		want string
	}{
		{-32600, "AX_ENGINE_INVALID_REQUEST"},
		{-32601, "AX_ENGINE_METHOD_NOT_FOUND"},
		{-32602, "AX_ENGINE_INVALID_PARAMS"},
		{-32603, "AX_ENGINE_INTERNAL"},
		{-32700, "AX_ENGINE_PARSE_ERROR"},
	}
	for _, tc := range cases {
		code, msg, _ := MapCodexError(&jsonRPCError{code: tc.code, message: "x"})
		if code != tc.want {
			t.Fatalf("rpc code %d -> %s (want %s)", tc.code, code, tc.want)
		}
		if msg == "" {
			t.Fatalf("expected non-empty message for code %d", tc.code)
		}
	}
}

func TestMapCodexError_ContextMappings(t *testing.T) {
	code, _, retryable := MapCodexError(context.DeadlineExceeded)
	if code != "AX_ENGINE_TIMEOUT" || !retryable {
		t.Fatalf("deadline mapping mismatch: code=%s retryable=%t", code, retryable)
	}
	code, _, retryable = MapCodexError(context.Canceled)
	if code != "AX_ENGINE_CANCELED" || retryable {
		t.Fatalf("cancel mapping mismatch: code=%s retryable=%t", code, retryable)
	}
}
