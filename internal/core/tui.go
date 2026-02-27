package core

// DashboardViewModel is Screen A snapshot contract.
type DashboardViewModel struct {
	Phase          string `json:"phase"`
	Progress       int    `json:"progress"`
	Proposal       string `json:"proposal"`
	Plan           string `json:"plan"`
	LastResult     string `json:"last_result"`
	LastError      string `json:"last_error"`
	ContextEntries int    `json:"context_entries"`
}

// RunsViewModel is Screen B snapshot contract.
type RunsViewModel struct {
	Count int          `json:"count"`
	Items []RunRowView `json:"items"`
}

// RunRowView is one run list row.
type RunRowView struct {
	Name       string `json:"name"`
	UpdatedAt  string `json:"updated_at"`
	StatusLine string `json:"status_line"`
}

// VerifyViewModel is Screen C snapshot contract.
type VerifyViewModel struct {
	Verdict      string   `json:"verdict"`
	Criteria     int      `json:"criteria"`
	FailedChecks []string `json:"failed_checks"`
}

// ArchiveViewModel is Screen D snapshot contract.
type ArchiveViewModel struct {
	Count int      `json:"count"`
	Items []string `json:"items"`
}

// EngineViewModel is Screen E snapshot contract.
type EngineViewModel struct {
	RuntimeMode    string            `json:"runtime_mode"`
	SessionID      string            `json:"session_id"`
	ClusterID      string            `json:"cluster_id"`
	NodeID         string            `json:"node_id"`
	ThreadID       string            `json:"thread_id,omitempty"`
	ActiveTurnID   string            `json:"active_turn_id,omitempty"`
	TurnCount      int               `json:"turn_count"`
	CodexMode      string            `json:"codex_mode,omitempty"`
	ActiveSessions map[string]string `json:"active_sessions"`
}
