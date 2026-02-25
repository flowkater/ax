package codex

import "context"

// JSONRPCRequest models a generic JSON-RPC 2.0 request payload.
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      string      `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// JSONRPCResponse models a generic JSON-RPC 2.0 response payload.
type JSONRPCResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      string           `json:"id"`
	Result  interface{}      `json:"result,omitempty"`
	Error   *JSONRPCErrorObj `json:"error,omitempty"`
}

// JSONRPCErrorObj models a JSON-RPC 2.0 error object.
type JSONRPCErrorObj struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Thread represents a durable conversation/work stream in Codex app-server.
type Thread struct {
	ID        string `json:"id"`
	Title     string `json:"title,omitempty"`
	CreatedAt string `json:"created_at"`
}

// Turn represents a single request/response interaction in a Thread.
type Turn struct {
	ID        string `json:"id"`
	ThreadID  string `json:"thread_id"`
	Role      string `json:"role"` // user | assistant | system
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

// AppServerAdapter defines the boundary for Codex app-server integration.
type AppServerAdapter interface {
	CreateThread(ctx context.Context, title string) (*Thread, error)
	RunTurn(ctx context.Context, threadID, prompt string) (*Turn, error)
	GetThread(ctx context.Context, threadID string) (*Thread, error)
}
