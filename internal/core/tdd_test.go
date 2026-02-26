package core

import "testing"

func TestResolveTDDTierTarget_Gate(t *testing.T) {
	if _, err := ResolveTDDTierTarget(TDDState{}, "T2"); err == nil {
		t.Fatal("expected T2 to be blocked without previous tier completion")
	}
	if _, err := ResolveTDDTierTarget(TDDState{CompletedTiers: []string{"T0"}}, "T1"); err != nil {
		t.Fatalf("expected T1 to be unlocked after T0: %v", err)
	}
	if _, err := ResolveTDDTierTarget(TDDState{CompletedTiers: []string{"T0", "T1"}}, "T2"); err != nil {
		t.Fatalf("expected T2 to be unlocked after T1: %v", err)
	}
}

func TestResolveTDDTierTarget_DefaultsToNextTier(t *testing.T) {
	tier, err := ResolveTDDTierTarget(TDDState{}, "")
	if err != nil {
		t.Fatalf("resolve default tier: %v", err)
	}
	if tier != "T0" {
		t.Fatalf("expected T0, got %s", tier)
	}

	tier, err = ResolveTDDTierTarget(TDDState{CompletedTiers: []string{"T0"}}, "")
	if err != nil {
		t.Fatalf("resolve default tier after T0: %v", err)
	}
	if tier != "T1" {
		t.Fatalf("expected T1, got %s", tier)
	}
}

func TestAdvanceTDDState_PersistsCompletedTiers(t *testing.T) {
	st := AdvanceTDDState(TDDState{}, "T0", "deep", "user", ApprovalPolicyNever, false)
	if st.CurrentTier != "T0" || st.CurrentStep != TDDStepDone {
		t.Fatalf("unexpected state: %+v", st)
	}
	if len(st.CompletedTiers) != 1 || st.CompletedTiers[0] != "T0" {
		t.Fatalf("unexpected completed tiers: %+v", st.CompletedTiers)
	}
	if st.CompletedSteps != 3 || st.TotalSteps != 9 {
		t.Fatalf("unexpected step counts: %d/%d", st.CompletedSteps, st.TotalSteps)
	}

	st = AdvanceTDDState(st, "T1", "deep", "auto", ApprovalPolicyOnFailure, true)
	if len(st.CompletedTiers) != 2 || st.CompletedTiers[1] != "T1" {
		t.Fatalf("unexpected completed tiers after T1: %+v", st.CompletedTiers)
	}
	if st.CompletedSteps != 6 {
		t.Fatalf("expected completed_steps=6, got %d", st.CompletedSteps)
	}
}

func TestTierStatusLabel(t *testing.T) {
	completed := []string{"T0", "T1"}
	if got := TierStatusLabel("T0", completed); got != "completed" {
		t.Fatalf("unexpected status: %s", got)
	}
	if got := TierStatusLabel("T2", completed); got != "pending" {
		t.Fatalf("unexpected status: %s", got)
	}
}

func TestResolveTDDProgression_ResumeAdvances(t *testing.T) {
	prev := TDDState{
		Enabled:        true,
		CompletedTiers: []string{"T0"},
	}
	next, progress, err := ResolveTDDProgression(prev, true, "deep", "user", ApprovalPolicyNever)
	if err != nil {
		t.Fatalf("ResolveTDDProgression resume: %v", err)
	}
	if next.CurrentTier != "T1" {
		t.Fatalf("expected T1, got %s", next.CurrentTier)
	}
	if progress != "advanced:T1" {
		t.Fatalf("unexpected progress: %s", progress)
	}
}

func TestResolveTDDProgression_BlocksWhenAllCompleted(t *testing.T) {
	prev := TDDState{
		Enabled:        true,
		CompletedTiers: []string{"T0", "T1", "T2"},
	}
	if _, _, err := ResolveTDDProgression(prev, true, "deep", "auto", ApprovalPolicyNever); err == nil {
		t.Fatal("expected all-tier-completed gate error")
	}
}
