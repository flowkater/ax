package core

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const defaultDecayAfterDays = 90

// TriageClass is a compound triage label.
type TriageClass string

const (
	TriageFixCandidate TriageClass = "FixCandidate"
	TriageDocument     TriageClass = "Document"
	TriageNoise        TriageClass = "Noise"
)

// GotchaItem is the normalized gotcha schema rendered by compound report.
type GotchaItem struct {
	ID           string      `json:"id"`
	Text         string      `json:"text"`
	Added        string      `json:"added"`
	LastRelevant string      `json:"last_relevant"`
	DecayAfter   string      `json:"decay_after"`
	FixCandidate bool        `json:"fix_candidate"`
	Triage       TriageClass `json:"triage"`
}

// CompoundAuditResult contains gotchas requiring decay/archive action.
type CompoundAuditResult struct {
	DecayCandidates   []GotchaItem
	ArchiveCandidates []GotchaItem
}

// CompoundTriageResult groups gotchas by triage class.
type CompoundTriageResult struct {
	FixCandidates []GotchaItem
	DocumentItems []GotchaItem
	NoiseItems    []GotchaItem
}

var codeDiscoverablePattern = regexp.MustCompile(`([A-Za-z0-9._-]+/[A-Za-z0-9._/-]+)|(go\s+(test|build)\s+\./\.\.\.)|(README)|(function|struct|interface|class)`) //nolint:lll

// ParseGotchas parses gotcha markdown/json-lines and applies triage.
func ParseGotchas(raw string, now time.Time) []GotchaItem {
	records := parseGotchaRecords(raw, now)
	items := make([]GotchaItem, 0, len(records))
	for _, r := range records {
		triage := classifyGotchaRecord(r)
		items = append(items, GotchaItem{
			ID:           r.ID,
			Text:         r.Text,
			Added:        r.Added,
			LastRelevant: r.LastRelevant,
			DecayAfter:   fmt.Sprintf("%dd", r.DecayAfterDays),
			FixCandidate: r.FixCandidate,
			Triage:       triage,
		})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}

// BuildCompoundAudit computes decay/archive candidates from normalized gotchas.
func BuildCompoundAudit(items []GotchaItem, now time.Time) CompoundAuditResult {
	result := CompoundAuditResult{}
	for _, item := range items {
		decayAfter := parseDecayAfterDays(item.DecayAfter)
		ageDays := gotchaAgeDays(item.LastRelevant, item.Added, now)
		if ageDays >= decayAfter {
			result.DecayCandidates = append(result.DecayCandidates, item)
		}
		if ageDays >= decayAfter*2 {
			result.ArchiveCandidates = append(result.ArchiveCandidates, item)
		}
	}
	return result
}

// BuildCompoundTriage groups already-normalized gotchas by triage.
func BuildCompoundTriage(items []GotchaItem, _ time.Time) CompoundTriageResult {
	result := CompoundTriageResult{}
	for _, item := range items {
		switch item.Triage {
		case TriageFixCandidate:
			result.FixCandidates = append(result.FixCandidates, item)
		case TriageDocument:
			result.DocumentItems = append(result.DocumentItems, item)
		case TriageNoise:
			result.NoiseItems = append(result.NoiseItems, item)
		}
	}
	return result
}

// IsCodeDiscoverable returns true when text is likely discoverable from code/docs.
func IsCodeDiscoverable(text string) bool {
	return codeDiscoverablePattern.MatchString(strings.TrimSpace(text))
}

type gotchaRecord struct {
	ID             string `json:"id"`
	Text           string `json:"text"`
	Added          string `json:"added"`
	LastRelevant   string `json:"last_relevant"`
	DecayAfterDays int    `json:"decay_after_days"`
	DecayAfter     int    `json:"decay_after"`
	FixCandidate   bool   `json:"fix_candidate"`
}

func parseGotchaRecords(raw string, now time.Time) []gotchaRecord {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	out := make([]gotchaRecord, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		r := gotchaRecord{}
		switch {
		case strings.HasPrefix(trimmed, "{"):
			if err := json.Unmarshal([]byte(trimmed), &r); err != nil {
				continue
			}
		case strings.HasPrefix(trimmed, "- "):
			r = parseGotchaKV(strings.TrimPrefix(trimmed, "- "))
			if strings.TrimSpace(r.Text) == "" {
				r.Text = strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			}
		default:
			r.Text = trimmed
		}

		r = normalizeGotchaRecord(r, len(out)+1, now)
		if strings.TrimSpace(r.Text) == "" {
			continue
		}
		out = append(out, r)
	}
	return out
}

func parseGotchaKV(raw string) gotchaRecord {
	parts := strings.Split(raw, "|")
	r := gotchaRecord{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(kv[0]))
		value := strings.TrimSpace(kv[1])
		switch key {
		case "id":
			r.ID = value
		case "text":
			r.Text = value
		case "added":
			r.Added = value
		case "last_relevant":
			r.LastRelevant = value
		case "decay_after":
			if v, err := strconv.Atoi(strings.TrimSuffix(strings.ToLower(value), "d")); err == nil {
				r.DecayAfterDays = v
			}
		case "fix_candidate":
			r.FixCandidate = strings.EqualFold(value, "true") || strings.EqualFold(value, "yes")
		}
	}
	return r
}

func normalizeGotchaRecord(r gotchaRecord, index int, now time.Time) gotchaRecord {
	if strings.TrimSpace(r.ID) == "" {
		r.ID = fmt.Sprintf("gotcha-%02d", index)
	}
	if strings.TrimSpace(r.Added) == "" {
		r.Added = now.Format("2006-01-02")
	}
	if strings.TrimSpace(r.LastRelevant) == "" {
		r.LastRelevant = r.Added
	}
	if r.DecayAfterDays <= 0 {
		if r.DecayAfter > 0 {
			r.DecayAfterDays = r.DecayAfter
		}
	}
	if r.DecayAfterDays <= 0 {
		r.DecayAfterDays = defaultDecayAfterDays
	}
	return r
}

func classifyGotchaRecord(r gotchaRecord) TriageClass {
	if r.FixCandidate {
		return TriageFixCandidate
	}
	if IsCodeDiscoverable(r.Text) {
		return TriageNoise
	}
	return TriageDocument
}

func parseDecayAfterDays(raw string) int {
	raw = strings.TrimSpace(strings.ToLower(raw))
	raw = strings.TrimSuffix(raw, "d")
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return defaultDecayAfterDays
	}
	return v
}

func gotchaAgeDays(lastRelevant, added string, now time.Time) int {
	anchor := parseDateFlexible(lastRelevant)
	if anchor.IsZero() {
		anchor = parseDateFlexible(added)
	}
	if anchor.IsZero() {
		return 0
	}
	d := now.Sub(anchor)
	if d < 0 {
		return 0
	}
	return int(d.Hours() / 24)
}

func parseDateFlexible(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t
		}
	}
	return time.Time{}
}
