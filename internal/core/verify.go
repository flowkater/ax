package core

import (
	"fmt"
	"strings"
)

// VerifyJudgment is PASS/FAIL/CONDITIONAL PASS output.
type VerifyJudgment string

const (
	VerifyPASS        VerifyJudgment = "PASS"
	VerifyFAIL        VerifyJudgment = "FAIL"
	VerifyConditional VerifyJudgment = "CONDITIONAL PASS"
)

// EvidenceStatus represents evidence result state.
type EvidenceStatus string

const (
	EvidencePass    EvidenceStatus = "PASS"
	EvidenceFail    EvidenceStatus = "FAIL"
	EvidenceUnknown EvidenceStatus = "UNKNOWN"
)

// VerifyEvidence holds required evidence dimensions.
type VerifyEvidence struct {
	Tests      EvidenceStatus
	Build      EvidenceStatus
	Acceptance EvidenceStatus
}

// VerifySnapshot is a parsed verify report snapshot.
type VerifySnapshot struct {
	Status   VerifyJudgment
	Evidence VerifyEvidence
	Unmet    []string
	Next     []string
}

// VerifyDiff captures previous/current verify differences.
type VerifyDiff struct {
	PreviousStatus VerifyJudgment
	CurrentStatus  VerifyJudgment
	Changed        bool

	StatusChanged   bool
	EvidenceChanges []string

	UnmetAdded   []string
	UnmetRemoved []string
	NextAdded    []string
	NextRemoved  []string

	AddedLines   []string
	RemovedLines []string
}

// EvaluateVerification returns deterministic judgment and unmet items.
func EvaluateVerification(e VerifyEvidence) (VerifyJudgment, []string, []string) {
	var unmet []string
	var nextActions []string

	if e.Tests == EvidenceFail {
		unmet = append(unmet, "테스트 실패")
		nextActions = append(nextActions, "실패 테스트를 수정하고 재실행하세요.")
	} else if e.Tests == EvidenceUnknown {
		unmet = append(unmet, "테스트 실행 근거 미기록")
		nextActions = append(nextActions, "go test ./... 실행 로그를 첨부하세요.")
	}

	if e.Build == EvidenceFail {
		unmet = append(unmet, "빌드 실패")
		nextActions = append(nextActions, "빌드 오류를 수정하고 go build ./...를 재실행하세요.")
	} else if e.Build == EvidenceUnknown {
		unmet = append(unmet, "빌드 근거 미기록")
		nextActions = append(nextActions, "go build ./... 결과를 첨부하세요.")
	}

	if e.Acceptance == EvidenceFail {
		unmet = append(unmet, "Acceptance Criteria 미충족")
		nextActions = append(nextActions, "미충족 AC 항목을 보완하세요.")
	} else if e.Acceptance == EvidenceUnknown {
		unmet = append(unmet, "Acceptance Criteria 근거 미기록")
		nextActions = append(nextActions, "AC 체크리스트 근거를 문서화하세요.")
	}

	if e.Tests == EvidencePass && e.Build == EvidencePass && e.Acceptance == EvidencePass {
		return VerifyPASS, nil, nil
	}
	if e.Tests == EvidenceFail || e.Build == EvidenceFail || e.Acceptance == EvidenceFail {
		return VerifyFAIL, unmet, nextActions
	}
	return VerifyConditional, unmet, nextActions
}

// ParseEvidenceStatus parses pass/fail/unknown strings.
func ParseEvidenceStatus(raw string) EvidenceStatus {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case "pass", "ok", "true", "yes":
		return EvidencePass
	case "fail", "false", "no":
		return EvidenceFail
	default:
		return EvidenceUnknown
	}
}

// ParseVerifySnapshot parses markdown verify report into a snapshot.
func ParseVerifySnapshot(report string) (VerifySnapshot, bool) {
	snapshot := VerifySnapshot{Evidence: VerifyEvidence{Tests: EvidenceUnknown, Build: EvidenceUnknown, Acceptance: EvidenceUnknown}}
	if strings.TrimSpace(report) == "" {
		return snapshot, false
	}

	section := ""
	for _, line := range strings.Split(strings.ReplaceAll(report, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			section = trimmed
			continue
		}
		if strings.HasPrefix(trimmed, "- status:") {
			snapshot.Status = VerifyJudgment(strings.TrimSpace(strings.TrimPrefix(trimmed, "- status:")))
			continue
		}

		switch section {
		case "## Evidence":
			switch {
			case strings.HasPrefix(trimmed, "- tests:"):
				snapshot.Evidence.Tests = ParseEvidenceStatus(strings.TrimSpace(strings.TrimPrefix(trimmed, "- tests:")))
			case strings.HasPrefix(trimmed, "- build:"):
				snapshot.Evidence.Build = ParseEvidenceStatus(strings.TrimSpace(strings.TrimPrefix(trimmed, "- build:")))
			case strings.HasPrefix(trimmed, "- acceptance_criteria:"):
				snapshot.Evidence.Acceptance = ParseEvidenceStatus(strings.TrimSpace(strings.TrimPrefix(trimmed, "- acceptance_criteria:")))
			}
		case "## Unmet Items":
			if strings.HasPrefix(trimmed, "- ") {
				item := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
				if item != "" && !strings.EqualFold(item, "none") {
					snapshot.Unmet = append(snapshot.Unmet, item)
				}
			}
		case "## Next Actions":
			if strings.HasPrefix(trimmed, "- ") {
				item := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
				if item != "" && !strings.EqualFold(item, "none") {
					snapshot.Next = append(snapshot.Next, item)
				}
			}
		}
	}

	if snapshot.Status == "" {
		return snapshot, false
	}
	return snapshot, true
}

// DiffVerifySnapshots compares previous/current verify snapshots.
func DiffVerifySnapshots(previous, current VerifySnapshot) VerifyDiff {
	diff := VerifyDiff{
		PreviousStatus: previous.Status,
		CurrentStatus:  current.Status,
	}
	if previous.Status != current.Status {
		diff.StatusChanged = true
		diff.EvidenceChanges = append(diff.EvidenceChanges, fmt.Sprintf("status: %s -> %s", previous.Status, current.Status))
	}
	if previous.Evidence.Tests != current.Evidence.Tests {
		diff.EvidenceChanges = append(diff.EvidenceChanges, fmt.Sprintf("tests: %s -> %s", previous.Evidence.Tests, current.Evidence.Tests))
	}
	if previous.Evidence.Build != current.Evidence.Build {
		diff.EvidenceChanges = append(diff.EvidenceChanges, fmt.Sprintf("build: %s -> %s", previous.Evidence.Build, current.Evidence.Build))
	}
	if previous.Evidence.Acceptance != current.Evidence.Acceptance {
		diff.EvidenceChanges = append(diff.EvidenceChanges, fmt.Sprintf("acceptance_criteria: %s -> %s", previous.Evidence.Acceptance, current.Evidence.Acceptance))
	}

	diff.UnmetAdded, diff.UnmetRemoved = sliceDiff(previous.Unmet, current.Unmet)
	diff.NextAdded, diff.NextRemoved = sliceDiff(previous.Next, current.Next)

	diff.AddedLines = append(diff.AddedLines, diff.UnmetAdded...)
	diff.AddedLines = append(diff.AddedLines, diff.NextAdded...)
	diff.RemovedLines = append(diff.RemovedLines, diff.UnmetRemoved...)
	diff.RemovedLines = append(diff.RemovedLines, diff.NextRemoved...)

	diff.Changed = diff.StatusChanged || len(diff.EvidenceChanges) > 0 || len(diff.AddedLines) > 0 || len(diff.RemovedLines) > 0
	return diff
}

// FormatVerifyDiff renders a markdown-friendly diff summary.
func FormatVerifyDiff(diff VerifyDiff) string {
	var b strings.Builder
	b.WriteString("## Diff vs Previous Verify\n\n")
	b.WriteString(fmt.Sprintf("- previous_status: %s\n", diff.PreviousStatus))
	b.WriteString(fmt.Sprintf("- current_status: %s\n", diff.CurrentStatus))
	b.WriteString(fmt.Sprintf("- changed: %t\n", diff.Changed))

	if len(diff.EvidenceChanges) > 0 {
		b.WriteString("\n## Evidence Changes\n")
		for _, change := range diff.EvidenceChanges {
			b.WriteString("- " + change + "\n")
		}
	}

	b.WriteString("\n## Added Lines\n")
	if len(diff.AddedLines) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, line := range diff.AddedLines {
			b.WriteString("- " + line + "\n")
		}
	}

	b.WriteString("\n## Removed Lines\n")
	if len(diff.RemovedLines) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, line := range diff.RemovedLines {
			b.WriteString("- " + line + "\n")
		}
	}

	b.WriteString("\n- unmet_added:\n")
	if len(diff.UnmetAdded) == 0 {
		b.WriteString("  - none\n")
	} else {
		for _, line := range diff.UnmetAdded {
			b.WriteString("  - " + line + "\n")
		}
	}
	b.WriteString("- unmet_removed:\n")
	if len(diff.UnmetRemoved) == 0 {
		b.WriteString("  - none\n")
	} else {
		for _, line := range diff.UnmetRemoved {
			b.WriteString("  - " + line + "\n")
		}
	}
	return b.String()
}

// BuildVerifyDiff compares previous/current verify reports.
func BuildVerifyDiff(previousReport, currentReport string) VerifyDiff {
	previous, okPrev := ParseVerifySnapshot(previousReport)
	current, okCurr := ParseVerifySnapshot(currentReport)
	if !okCurr {
		return VerifyDiff{}
	}
	if !okPrev {
		return VerifyDiff{
			PreviousStatus:  "NONE",
			CurrentStatus:   current.Status,
			Changed:         true,
			StatusChanged:   true,
			EvidenceChanges: []string{fmt.Sprintf("status: NONE -> %s", current.Status)},
			AddedLines:      append([]string{}, append([]string{}, current.Unmet...)...),
		}
	}
	return DiffVerifySnapshots(previous, current)
}

func sliceDiff(previous, current []string) (added []string, removed []string) {
	prevSet := map[string]struct{}{}
	currSet := map[string]struct{}{}
	for _, v := range previous {
		prevSet[v] = struct{}{}
	}
	for _, v := range current {
		currSet[v] = struct{}{}
	}
	for _, v := range current {
		if _, ok := prevSet[v]; !ok {
			added = append(added, v)
		}
	}
	for _, v := range previous {
		if _, ok := currSet[v]; !ok {
			removed = append(removed, v)
		}
	}
	return added, removed
}
