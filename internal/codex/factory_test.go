package codex

import (
	"strings"
	"testing"
	"time"
)

func TestResolveConfigDefaults(t *testing.T) {
	t.Setenv("AX_CODEX_MODE", "")
	t.Setenv("AX_CODEX_BIN", "")
	t.Setenv("AX_CODEX_ARGS", "")
	t.Setenv("AX_CODEX_TIMEOUT", "")
	t.Setenv("AX_CODEX_RETRIES", "")

	cfg := ResolveConfig()
	if cfg.Mode != DefaultMode {
		t.Fatalf("expected default mode %q, got %q", DefaultMode, cfg.Mode)
	}
	if cfg.BinPath != DefaultBinPath {
		t.Fatalf("expected default bin %q, got %q", DefaultBinPath, cfg.BinPath)
	}
	if cfg.Timeout != DefaultTimeout {
		t.Fatalf("expected default timeout %s, got %s", DefaultTimeout, cfg.Timeout)
	}
	if cfg.Retries != DefaultRetries {
		t.Fatalf("expected default retries %d, got %d", DefaultRetries, cfg.Retries)
	}
}

func TestResolveConfigEnvOverrides(t *testing.T) {
	t.Setenv("AX_CODEX_MODE", "real")
	t.Setenv("AX_CODEX_BIN", "codex-custom")
	t.Setenv("AX_CODEX_ARGS", "--model gpt-5 --stream")
	t.Setenv("AX_CODEX_TIMEOUT", "45s")
	t.Setenv("AX_CODEX_RETRIES", "4")

	cfg := ResolveConfig()
	if cfg.Mode != "real" {
		t.Fatalf("expected mode real, got %q", cfg.Mode)
	}
	if cfg.BinPath != "codex-custom" {
		t.Fatalf("unexpected bin path: %q", cfg.BinPath)
	}
	if len(cfg.Args) != 3 {
		t.Fatalf("expected 3 args, got %d (%v)", len(cfg.Args), cfg.Args)
	}
	if cfg.Timeout != 45*time.Second {
		t.Fatalf("expected timeout 45s, got %s", cfg.Timeout)
	}
	if cfg.Retries != 4 {
		t.Fatalf("expected retries=4, got %d", cfg.Retries)
	}
}

func TestResolveConfigPreservesInvalidModeForValidation(t *testing.T) {
	t.Setenv("AX_CODEX_MODE", "unknown-mode")

	cfg := ResolveConfig()
	if cfg.Mode != "unknown-mode" {
		t.Fatalf("expected invalid mode to be preserved, got %q", cfg.Mode)
	}
}

func TestNewAdapterModes(t *testing.T) {
	scaffold, err := NewAdapter(ClientConfig{Mode: "scaffold"})
	if err != nil {
		t.Fatalf("new scaffold adapter: %v", err)
	}
	if _, ok := scaffold.(*ScaffoldAdapter); !ok {
		t.Fatalf("expected ScaffoldAdapter, got %T", scaffold)
	}

	real, err := NewAdapter(ClientConfig{
		Mode:    "real",
		BinPath: "codex",
		Timeout: 5 * time.Second,
		Retries: 1,
	})
	if err != nil {
		t.Fatalf("new real adapter: %v", err)
	}
	client, ok := real.(*StdioClient)
	if !ok {
		t.Fatalf("expected StdioClient, got %T", real)
	}
	if client.command != "codex" {
		t.Fatalf("unexpected client command: %s", client.command)
	}
}

func TestNewAdapterRejectsInvalidMode(t *testing.T) {
	_, err := NewAdapter(ClientConfig{Mode: "invalid"})
	if err == nil {
		t.Fatal("expected invalid mode to fail")
	}
	if got := err.Error(); got == "" || !strings.Contains(got, "AX_ENGINE_CONFIG_INVALID") {
		t.Fatalf("expected AX_ENGINE_CONFIG_INVALID error, got %q", got)
	}
}
