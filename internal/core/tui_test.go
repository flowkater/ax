package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTUIViewModelContractsMarshalExpectedFields(t *testing.T) {
	payload := map[string]any{
		"screen_a": DashboardViewModel{
			Phase:          "implementation",
			Progress:       65,
			Proposal:       "p-1",
			Plan:           "plan-1.md",
			LastResult:     "run:completed",
			LastError:      "none",
			ContextEntries: 3,
		},
		"screen_b": RunsViewModel{
			Count: 1,
			Items: []RunRowView{{Name: "run-1.md", UpdatedAt: "2026-02-27T00:00:00Z", StatusLine: "completed"}},
		},
		"screen_c": VerifyViewModel{
			Verdict:      "PASS",
			Criteria:     100,
			FailedChecks: []string{},
		},
		"screen_d": ArchiveViewModel{
			Count: 1,
			Items: []string{"proposal-1"},
		},
		"screen_e": EngineViewModel{
			RuntimeMode:    "shared",
			SessionID:      "sess-1",
			ClusterID:      "cluster-a",
			NodeID:         "node-a",
			ActiveSessions: map[string]string{"sess-1": "run"},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(body)
	for _, want := range []string{
		`"phase"`, `"progress"`, `"proposal"`, `"plan"`,
		`"count"`, `"items"`, `"status_line"`,
		`"verdict"`, `"criteria"`, `"failed_checks"`,
		`"runtime_mode"`, `"session_id"`, `"cluster_id"`, `"node_id"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing field %s in json: %s", want, got)
		}
	}
}
