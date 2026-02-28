package codex

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultMode    = "scaffold"
	DefaultBinPath = "codex"
	DefaultTimeout = 30 * time.Second
	DefaultRetries = 2
)

// ClientConfig configures adapter creation.
type ClientConfig struct {
	Mode    string
	BinPath string
	Args    []string
	Timeout time.Duration
	Retries int
}

// ResolveConfig reads Codex adapter settings from environment.
func ResolveConfig() ClientConfig {
	cfg := ClientConfig{
		Mode:    DefaultMode,
		BinPath: DefaultBinPath,
		Args:    nil,
		Timeout: DefaultTimeout,
		Retries: DefaultRetries,
	}

	if v := strings.TrimSpace(os.Getenv("AX_CODEX_MODE")); v != "" {
		cfg.Mode = strings.ToLower(v)
	}
	if v := strings.TrimSpace(os.Getenv("AX_CODEX_BIN")); v != "" {
		cfg.BinPath = v
	}
	if v := strings.TrimSpace(os.Getenv("AX_CODEX_ARGS")); v != "" {
		cfg.Args = strings.Fields(v)
	}
	if v := strings.TrimSpace(os.Getenv("AX_CODEX_TIMEOUT")); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.Timeout = d
		}
	}
	if v := strings.TrimSpace(os.Getenv("AX_CODEX_RETRIES")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			cfg.Retries = n
		}
	}
	return cfg
}

// NewAdapter creates an AppServerAdapter from config.
func NewAdapter(cfg ClientConfig) (AppServerAdapter, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" {
		mode = DefaultMode
	}
	switch mode {
	case "scaffold":
		return NewScaffoldAdapter(), nil
	case "real":
		// continue
	default:
		return nil, fmt.Errorf("AX_ENGINE_CONFIG_INVALID: AX_CODEX_MODE must be real|scaffold (got %q)", mode)
	}

	bin := strings.TrimSpace(cfg.BinPath)
	if bin == "" {
		bin = DefaultBinPath
	}
	args := cfg.Args
	if len(args) == 0 {
		args = []string{"app-server"}
	}
	client := NewStdioClient(bin, args...)
	client.timeout = cfg.Timeout
	client.retries = cfg.Retries
	return client, nil
}
