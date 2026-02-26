package core

import (
	"fmt"
	"sort"
	"strings"
)

var tddTierOrder = []string{"T0", "T1", "T2"}

const (
	TDDStepDone = "done"
)

// ResolveTDDProgression adapts persisted TDD state into next tier progress.
func ResolveTDDProgression(prev TDDState, resume bool, depth, depthSource, approvalPolicy string) (TDDState, string, error) {
	if !resume {
		next := AdvanceTDDState(TDDState{}, "T0", depth, depthSource, approvalPolicy, false)
		return next, "started:T0", nil
	}

	if !prev.Enabled {
		return TDDState{}, "", fmt.Errorf("resume requested but no previous tdd state")
	}

	completed := NormalizeCompletedTiers(prev.CompletedTiers)
	if len(completed) >= len(tddTierOrder) {
		return TDDState{}, "", fmt.Errorf("tier progression gate blocked: all tiers already completed")
	}

	target, err := ResolveTDDTierTarget(prev, "")
	if err != nil {
		return TDDState{}, "", err
	}
	next := AdvanceTDDState(prev, target, depth, depthSource, approvalPolicy, true)
	return next, fmt.Sprintf("advanced:%s", target), nil
}

// ResolveTDDTierTarget chooses the requested tier and enforces progression gate.
func ResolveTDDTierTarget(prev TDDState, requestedTier string) (string, error) {
	completed := NormalizeCompletedTiers(prev.CompletedTiers)
	requested := normalizeTier(requestedTier)
	if requested == "" {
		for _, tier := range tddTierOrder {
			if !containsString(completed, tier) {
				requested = tier
				break
			}
		}
		if requested == "" {
			requested = "T2"
		}
	}

	if !isValidTier(requested) {
		return "", fmt.Errorf("invalid tdd tier: %s (use T0|T1|T2)", requestedTier)
	}
	if !isTierUnlocked(requested, completed) {
		return "", fmt.Errorf("tier progression gate blocked: %s requires %s", requested, requiredPreviousTier(requested))
	}
	return requested, nil
}

// AdvanceTDDState records completed tier progress into persisted state.
func AdvanceTDDState(prev TDDState, completedTier string, depth, depthSource, approvalPolicy string, resume bool) TDDState {
	tier := normalizeTier(completedTier)
	completed := NormalizeCompletedTiers(prev.CompletedTiers)
	if isValidTier(tier) && !containsString(completed, tier) {
		completed = append(completed, tier)
		completed = NormalizeCompletedTiers(completed)
	}

	state := TDDState{
		Enabled:        true,
		CurrentTier:    tier,
		CurrentStep:    TDDStepDone,
		CompletedSteps: len(completed) * 3,
		TotalSteps:     len(tddTierOrder) * 3,
		Depth:          depth,
		DepthSource:    depthSource,
		ApprovalPolicy: approvalPolicy,
		Resume:         resume,
		CompletedTiers: completed,
	}
	if state.CompletedSteps > state.TotalSteps {
		state.CompletedSteps = state.TotalSteps
	}
	return state
}

// NormalizeCompletedTiers canonicalizes persisted completed tier list.
func NormalizeCompletedTiers(tiers []string) []string {
	seen := map[string]struct{}{}
	for _, tier := range tiers {
		n := normalizeTier(tier)
		if !isValidTier(n) {
			continue
		}
		seen[n] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for _, tier := range tddTierOrder {
		if _, ok := seen[tier]; ok {
			out = append(out, tier)
		}
	}
	return out
}

// TierStatusLabel returns completed/pending label for report rendering.
func TierStatusLabel(tier string, completedTiers []string) string {
	if containsString(NormalizeCompletedTiers(completedTiers), normalizeTier(tier)) {
		return "completed"
	}
	return "pending"
}

func normalizeTier(tier string) string {
	t := strings.ToUpper(strings.TrimSpace(tier))
	switch t {
	case "T0", "T1", "T2":
		return t
	default:
		return ""
	}
}

func isValidTier(tier string) bool {
	return normalizeTier(tier) != ""
}

func isTierUnlocked(target string, completed []string) bool {
	target = normalizeTier(target)
	completed = NormalizeCompletedTiers(completed)
	switch target {
	case "T0":
		return true
	case "T1":
		return containsString(completed, "T0")
	case "T2":
		return containsString(completed, "T1")
	default:
		return false
	}
}

func requiredPreviousTier(target string) string {
	switch normalizeTier(target) {
	case "T1":
		return "T0 completion"
	case "T2":
		return "T1 completion"
	default:
		return "previous tier completion"
	}
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

// SortTiers returns a stable sorted copy for testing/diagnostics.
func SortTiers(tiers []string) []string {
	out := append([]string(nil), NormalizeCompletedTiers(tiers)...)
	sort.SliceStable(out, func(i, j int) bool {
		return tierIndex(out[i]) < tierIndex(out[j])
	})
	return out
}

func tierIndex(tier string) int {
	for i, candidate := range tddTierOrder {
		if candidate == tier {
			return i
		}
	}
	return len(tddTierOrder)
}
