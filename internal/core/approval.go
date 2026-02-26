package core

import (
	"fmt"
	"strings"
)

const (
	ApprovalPolicyNever             = "never"
	ApprovalPolicyOnFailure         = "on-failure"
	ApprovalPolicyUnlessAllowListed = "unless-allow-listed"
	ApprovalPolicyAlways            = "always"
)

// ApprovalDecision indicates whether an explicit approval gate is active.
type ApprovalDecision struct {
	Required bool
	Reason   string
}

// EvaluateApprovalPolicy evaluates phase/decision approval requirement.
func EvaluateApprovalPolicy(policy string, phase Phase, decision string, hadFailure bool) (ApprovalDecision, error) {
	normalized := strings.TrimSpace(strings.ToLower(policy))
	if !IsValidApprovalPolicy(normalized) {
		return ApprovalDecision{}, fmt.Errorf("invalid approval policy: %s", policy)
	}
	decision = strings.TrimSpace(strings.ToLower(decision))

	switch normalized {
	case ApprovalPolicyNever:
		return ApprovalDecision{Required: false, Reason: "never"}, nil
	case ApprovalPolicyAlways:
		return ApprovalDecision{Required: true, Reason: "always"}, nil
	case ApprovalPolicyOnFailure:
		if hadFailure || decision == "reject" {
			return ApprovalDecision{Required: true, Reason: "on-failure"}, nil
		}
		return ApprovalDecision{Required: false, Reason: "on-failure"}, nil
	case ApprovalPolicyUnlessAllowListed:
		if isAllowListedDecision(phase, decision) {
			return ApprovalDecision{Required: false, Reason: "allow-listed"}, nil
		}
		return ApprovalDecision{Required: true, Reason: "unless-allow-listed"}, nil
	default:
		return ApprovalDecision{}, fmt.Errorf("invalid approval policy: %s", policy)
	}
}

// IsValidApprovalPolicy validates supported policy values.
func IsValidApprovalPolicy(policy string) bool {
	switch strings.TrimSpace(strings.ToLower(policy)) {
	case ApprovalPolicyNever, ApprovalPolicyOnFailure, ApprovalPolicyUnlessAllowListed, ApprovalPolicyAlways:
		return true
	default:
		return false
	}
}

func isAllowListedDecision(phase Phase, decision string) bool {
	decision = strings.TrimSpace(strings.ToLower(decision))
	if decision == "accept" {
		return true
	}
	if phase == PhaseDiscovery && (decision == "" || decision == "accept") {
		return true
	}
	return false
}
