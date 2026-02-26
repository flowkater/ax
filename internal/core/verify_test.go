package core

import "testing"

func TestEvaluateVerification_PASS(t *testing.T) {
	j, unmet, next := EvaluateVerification(VerifyEvidence{Tests: EvidencePass, Build: EvidencePass, Acceptance: EvidencePass})
	if j != VerifyPASS {
		t.Fatalf("judgment=%s", j)
	}
	if len(unmet) != 0 || len(next) != 0 {
		t.Fatalf("expected no unmet/next, got %#v %#v", unmet, next)
	}
}

func TestEvaluateVerification_FAIL(t *testing.T) {
	j, unmet, next := EvaluateVerification(VerifyEvidence{Tests: EvidencePass, Build: EvidenceFail, Acceptance: EvidenceUnknown})
	if j != VerifyFAIL {
		t.Fatalf("judgment=%s", j)
	}
	if len(unmet) == 0 || len(next) == 0 {
		t.Fatalf("expected unmet actions")
	}
}

func TestEvaluateVerification_CONDITIONAL(t *testing.T) {
	j, unmet, next := EvaluateVerification(VerifyEvidence{Tests: EvidenceUnknown, Build: EvidenceUnknown, Acceptance: EvidenceUnknown})
	if j != VerifyConditional {
		t.Fatalf("judgment=%s", j)
	}
	if len(unmet) != 3 || len(next) != 3 {
		t.Fatalf("expected placeholders, got %#v %#v", unmet, next)
	}
}

func TestParseVerifySnapshotAndBuildVerifyDiff(t *testing.T) {
	prevReport := `# Verify Report

- proposal: p1
- status: CONDITIONAL PASS

## Evidence
- tests: UNKNOWN
- build: PASS
- acceptance_criteria: UNKNOWN

## Unmet Items
- 테스트 실행 근거 미기록

## Next Actions
- go test ./... 실행 로그를 첨부하세요.
`
	currReport := `# Verify Report

- proposal: p1
- status: PASS

## Evidence
- tests: PASS
- build: PASS
- acceptance_criteria: PASS

## Unmet Items
- none

## Next Actions
- none
`

	snapshot, ok := ParseVerifySnapshot(prevReport)
	if !ok {
		t.Fatal("expected snapshot parse success")
	}
	if snapshot.Status != VerifyConditional {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}

	diff := BuildVerifyDiff(prevReport, currReport)
	if !diff.Changed || !diff.StatusChanged {
		t.Fatalf("expected changed status diff, got %+v", diff)
	}
	if len(diff.EvidenceChanges) < 3 {
		t.Fatalf("expected evidence changes, got %+v", diff.EvidenceChanges)
	}
}

func TestBuildVerifyDiff_NoPrevious(t *testing.T) {
	currReport := `# Verify Report

- proposal: p1
- status: PASS

## Evidence
- tests: PASS
- build: PASS
- acceptance_criteria: PASS
`
	diff := BuildVerifyDiff("", currReport)
	if !diff.Changed || diff.PreviousStatus != "NONE" || diff.CurrentStatus != VerifyPASS {
		t.Fatalf("unexpected diff output: %+v", diff)
	}
}
