package codex

import (
	"context"
	"errors"
	"fmt"
)

// MapCodexError maps adapter/server failures to AX_ENGINE_* taxonomy.
func MapCodexError(err error) (code string, message string, retryable bool) {
	if err == nil {
		return "", "", false
	}

	var rpcErr *jsonRPCError
	if errors.As(err, &rpcErr) {
		switch rpcErr.code {
		case -32600:
			return "AX_ENGINE_INVALID_REQUEST", rpcErr.message, false
		case -32601:
			return "AX_ENGINE_METHOD_NOT_FOUND", rpcErr.message, false
		case -32602:
			return "AX_ENGINE_INVALID_PARAMS", rpcErr.message, false
		case -32603:
			return "AX_ENGINE_INTERNAL", rpcErr.message, true
		case -32700:
			return "AX_ENGINE_PARSE_ERROR", rpcErr.message, false
		default:
			return "AX_ENGINE_INTERNAL", fmt.Sprintf("json-rpc %d: %s", rpcErr.code, rpcErr.message), isRetryable(err)
		}
	}

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "AX_ENGINE_TIMEOUT", err.Error(), true
	case errors.Is(err, context.Canceled):
		return "AX_ENGINE_CANCELED", err.Error(), false
	default:
		return "AX_ENGINE_INTERNAL", err.Error(), isRetryable(err)
	}
}
