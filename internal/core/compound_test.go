package core

import (
	"strings"
	"testing"
	"time"
)

func TestParseGotchas_AssignsSchemaAndTriage(t *testing.T) {
	now := time.Date(2026, 2, 26, 0, 0, 0, 0, time.UTC)
	raw := strings.Join([]string{
		"# Gotchas",
		`{"id":"g-fix","text":"panic on nil pointer","added":"2026-01-01","last_relevant":"2026-02-01","decay_after_days":30,"fix_candidate":true}`,
		"- id: g-noise | text: see internal/core/state.go for phase enum | added: 2025-01-01 | last_relevant: 2025-01-10 | decay_after: 90 | fix_candidate: false",
		"- durable product decision note",
	}, "\n")

	items := ParseGotchas(raw, now)
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	byID := map[string]GotchaItem{}
	for _, item := range items {
		byID[item.ID] = item
	}
	if byID["g-fix"].Triage != TriageFixCandidate {
		t.Fatalf("expected g-fix triage FixCandidate, got %s", byID["g-fix"].Triage)
	}
	if byID["g-noise"].Triage != TriageNoise {
		t.Fatalf("expected g-noise triage Noise, got %s", byID["g-noise"].Triage)
	}
	foundDocument := false
	for _, item := range items {
		if item.Triage == TriageDocument {
			foundDocument = true
			if item.ID == "" || item.DecayAfter == "" {
				t.Fatalf("expected schema defaults for document item: %+v", item)
			}
		}
	}
	if !foundDocument {
		t.Fatal("expected at least one Document triage item")
	}
}

func TestBuildCompoundAudit(t *testing.T) {
	now := time.Date(2026, 2, 26, 0, 0, 0, 0, time.UTC)
	items := []GotchaItem{
		{ID: "a", Text: "x", Added: "2025-01-01", LastRelevant: "2025-01-10", DecayAfter: "90d", Triage: TriageDocument},
		{ID: "b", Text: "y", Added: "2026-01-01", LastRelevant: "2026-02-20", DecayAfter: "90d", Triage: TriageFixCandidate},
	}
	audit := BuildCompoundAudit(items, now)
	if len(audit.DecayCandidates) != 1 || audit.DecayCandidates[0].ID != "a" {
		t.Fatalf("expected only item a as decay candidate, got %+v", audit.DecayCandidates)
	}
	if len(audit.ArchiveCandidates) != 1 || audit.ArchiveCandidates[0].ID != "a" {
		t.Fatalf("expected only item a as archive candidate, got %+v", audit.ArchiveCandidates)
	}
}

func TestIsCodeDiscoverable(t *testing.T) {
	if !IsCodeDiscoverable("see cmd/ax/commands.go") {
		t.Fatal("expected code-discoverable pattern to match")
	}
	if IsCodeDiscoverable("this is a durable decision memo") {
		t.Fatal("expected durable memo to not be treated as code-discoverable")
	}
}
