package core

import "testing"

func TestValidateQualityGate_WithinDefaultLimit(t *testing.T) {
	if err := ValidateQualityGate(3, false, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateQualityGate_ExceedsDefaultWithoutForce(t *testing.T) {
	if err := ValidateQualityGate(4, false, ""); err == nil {
		t.Fatal("expected quality gate error")
	}
}

func TestValidateQualityGate_ForceRequiresReason(t *testing.T) {
	if err := ValidateQualityGate(4, true, " "); err == nil {
		t.Fatal("expected missing force reason error")
	}
}

func TestValidateQualityGate_ForceWithinHardLimit(t *testing.T) {
	if err := ValidateQualityGate(5, true, "manual override"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateQualityGate_ForceExceedsHardLimit(t *testing.T) {
	if err := ValidateQualityGate(6, true, "manual override"); err == nil {
		t.Fatal("expected hard limit error")
	}
}

func TestQualityGateSummary(t *testing.T) {
	if got := QualityGateSummary(2, false); got != "within-limit" {
		t.Fatalf("unexpected summary: %s", got)
	}
	if got := QualityGateSummary(4, true); got != "forced-override" {
		t.Fatalf("unexpected summary: %s", got)
	}
	if got := QualityGateSummary(6, false); got != "blocked" {
		t.Fatalf("unexpected summary: %s", got)
	}
}
