package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sync/atomic"
)

type StdioClient struct {
	command string
	args    []string
	seq     uint64
}

func NewStdioClient(command string, args ...string) *StdioClient {
	return &StdioClient{command: command, args: args}
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

func (c *StdioClient) call(ctx context.Context, method string, params any, out any) error {
	if c.command == "" {
		return errors.New("stdio client command is required")
	}
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
	enc := json.NewEncoder(stdin)
	if err := enc.Encode(req); err != nil {
		_ = cmd.Wait()
		return err
	}
	_ = stdin.Close()

	sc := bufio.NewScanner(stdout)
	if !sc.Scan() {
		_ = cmd.Wait()
		return errors.New("no JSON-RPC response from app-server")
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(sc.Bytes(), &resp); err != nil {
		_ = cmd.Wait()
		return err
	}
	if resp.Error != nil {
		_ = cmd.Wait()
		return fmt.Errorf("json-rpc error (%d): %s", resp.Error.Code, resp.Error.Message)
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
