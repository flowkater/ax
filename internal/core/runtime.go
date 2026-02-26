package core

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// RuntimeMode controls concurrency and transition behavior.
type RuntimeMode string

const (
	RuntimeModeSingle   RuntimeMode = "single"
	RuntimeModeShared   RuntimeMode = "shared"
	RuntimeModeWorktree RuntimeMode = "worktree"
	RuntimeModeAuto     RuntimeMode = "auto"
)

// RuntimeState stores lightweight runtime metadata in state.
type RuntimeState struct {
	Mode           RuntimeMode       `json:"mode,omitempty"`
	SessionID      string            `json:"session_id,omitempty"`
	ActiveSessions map[string]string `json:"active_sessions,omitempty"`
}

// ParseRuntimeMode validates and normalizes runtime mode inputs.
func ParseRuntimeMode(raw string) (RuntimeMode, error) {
	mode := RuntimeMode(strings.TrimSpace(strings.ToLower(raw)))
	if mode == "" {
		return RuntimeModeSingle, nil
	}
	switch mode {
	case RuntimeModeSingle, RuntimeModeShared, RuntimeModeWorktree, RuntimeModeAuto:
		return mode, nil
	default:
		return "", fmt.Errorf("invalid runtime mode %q (use single|shared|worktree|auto)", raw)
	}
}

// ResolveRuntimeMode resolves explicit mode and optional env override.
func ResolveRuntimeMode(raw string) (RuntimeMode, error) {
	if strings.TrimSpace(raw) == "" {
		if env := os.Getenv("AX_RUNTIME_MODE"); strings.TrimSpace(env) != "" {
			raw = env
		}
	}
	mode, err := ParseRuntimeMode(raw)
	if err != nil {
		return "", err
	}
	if mode == RuntimeModeAuto {
		return RuntimeModeShared, nil
	}
	return mode, nil
}

// NewSessionID returns a deterministic-enough command session id.
func NewSessionID(now time.Time) string {
	return fmt.Sprintf("sess-%d-%d", now.Unix(), os.Getpid())
}
