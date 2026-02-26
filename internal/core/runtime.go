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
	Mode           RuntimeMode                  `json:"mode,omitempty"`
	SessionID      string                       `json:"session_id,omitempty"`
	ClusterID      string                       `json:"cluster_id,omitempty"`
	NodeID         string                       `json:"node_id,omitempty"`
	SessionJournal string                       `json:"session_journal,omitempty"`
	ActiveSessions map[string]string            `json:"active_sessions,omitempty"`
	SessionMeta    map[string]RuntimeSessionRef `json:"session_meta,omitempty"`
}

// RuntimeSessionRef stores per-session metadata for cluster/runtime diagnostics.
type RuntimeSessionRef struct {
	SessionID string      `json:"session_id,omitempty"`
	Command   string      `json:"command,omitempty"`
	Mode      RuntimeMode `json:"mode,omitempty"`
	ClusterID string      `json:"cluster_id,omitempty"`
	NodeID    string      `json:"node_id,omitempty"`
	StartedAt string      `json:"started_at,omitempty"`
	UpdatedAt string      `json:"updated_at,omitempty"`
	Status    string      `json:"status,omitempty"`
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

// ResolveRuntimeClusterID resolves cluster id from input/env with local default.
func ResolveRuntimeClusterID(raw string) string {
	cluster := strings.TrimSpace(raw)
	if cluster == "" {
		cluster = strings.TrimSpace(os.Getenv("AX_CLUSTER_ID"))
	}
	if cluster == "" {
		cluster = strings.TrimSpace(os.Getenv("AX_RUNTIME_CLUSTER"))
	}
	if cluster == "" {
		return "local"
	}
	return cluster
}

// ResolveRuntimeNodeID resolves a stable-enough node id for diagnostics.
func ResolveRuntimeNodeID(raw string) string {
	node := strings.TrimSpace(raw)
	if node == "" {
		node = strings.TrimSpace(os.Getenv("AX_NODE_ID"))
	}
	if node == "" {
		node = strings.TrimSpace(os.Getenv("AX_RUNTIME_NODE"))
	}
	if node != "" {
		return node
	}
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		host = "unknown-host"
	}
	return fmt.Sprintf("%s:%d", host, os.Getpid())
}
