package core

import "testing"

func TestEvaluateApprovalPolicy(t *testing.T) {
	cases := []struct {
		name         string
		policy       string
		phase        Phase
		decision     string
		hadFailure   bool
		wantRequired bool
		wantReason   string
		wantErr      bool
	}{
		{name: "never", policy: ApprovalPolicyNever, phase: PhaseImplementation, decision: "accept", wantRequired: false, wantReason: "never"},
		{name: "always", policy: ApprovalPolicyAlways, phase: PhaseImplementation, decision: "accept", wantRequired: true, wantReason: "always"},
		{name: "on-failure idle", policy: ApprovalPolicyOnFailure, phase: PhaseImplementation, decision: "accept", hadFailure: false, wantRequired: false, wantReason: "on-failure"},
		{name: "on-failure triggered by failure", policy: ApprovalPolicyOnFailure, phase: PhaseImplementation, decision: "accept", hadFailure: true, wantRequired: true, wantReason: "on-failure"},
		{name: "on-failure triggered by reject", policy: ApprovalPolicyOnFailure, phase: PhaseImplementation, decision: "reject", wantRequired: true, wantReason: "on-failure"},
		{name: "unless allow-listed accept", policy: ApprovalPolicyUnlessAllowListed, phase: PhaseImplementation, decision: "accept", wantRequired: false, wantReason: "allow-listed"},
		{name: "unless blocked", policy: ApprovalPolicyUnlessAllowListed, phase: PhaseImplementation, decision: "steer", wantRequired: true, wantReason: "unless-allow-listed"},
		{name: "invalid", policy: "bad", phase: PhaseImplementation, decision: "accept", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision, err := EvaluateApprovalPolicy(tc.policy, tc.phase, tc.decision, tc.hadFailure)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if decision.Required != tc.wantRequired {
				t.Fatalf("required=%t want %t", decision.Required, tc.wantRequired)
			}
			if decision.Reason != tc.wantReason {
				t.Fatalf("reason=%q want %q", decision.Reason, tc.wantReason)
			}
		})
	}
}

func TestIsValidApprovalPolicy(t *testing.T) {
	for _, policy := range []string{ApprovalPolicyNever, ApprovalPolicyOnFailure, ApprovalPolicyUnlessAllowListed, ApprovalPolicyAlways} {
		if !IsValidApprovalPolicy(policy) {
			t.Fatalf("expected valid policy: %s", policy)
		}
	}
	if IsValidApprovalPolicy("unknown") {
		t.Fatal("expected invalid policy")
	}
}
