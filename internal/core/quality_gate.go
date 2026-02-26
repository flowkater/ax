package core

import (
	"fmt"
	"strings"
)

const (
	// DefaultQualityGateLimit is the default maximum review attempts per tier.
	DefaultQualityGateLimit = 3
	// ForcedQualityGateLimit is the hard cap even when --force is used.
	ForcedQualityGateLimit = 5
)

// ValidateQualityGate enforces review-count limits for run --tdd.
func ValidateQualityGate(reviewCount int, force bool, forceReason string) error {
	if reviewCount < 0 {
		return fmt.Errorf("review count cannot be negative: %d", reviewCount)
	}

	if reviewCount <= DefaultQualityGateLimit {
		return nil
	}

	if !force {
		return fmt.Errorf("quality gate exceeded: review_count=%d > %d (use --force --force-reason)", reviewCount, DefaultQualityGateLimit)
	}

	if strings.TrimSpace(forceReason) == "" {
		return fmt.Errorf("--force requires --force-reason when review_count=%d exceeds %d", reviewCount, DefaultQualityGateLimit)
	}

	if reviewCount > ForcedQualityGateLimit {
		return fmt.Errorf("quality gate hard limit exceeded: review_count=%d > %d", reviewCount, ForcedQualityGateLimit)
	}

	return nil
}

// QualityGateSummary is a short status string for reporting artifacts.
func QualityGateSummary(reviewCount int, force bool) string {
	switch {
	case reviewCount <= DefaultQualityGateLimit:
		return "within-limit"
	case force && reviewCount <= ForcedQualityGateLimit:
		return "forced-override"
	default:
		return "blocked"
	}
}
