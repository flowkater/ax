package ax

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/flowkater/ax/internal/core"
)

func TestProposeCreatesRequiredSectionsAndStateContext(t *testing.T) {
	tmp := setupTempCWD(t)

	out, err := executeAX(t, "propose", "Build MVP")
	if err != nil {
		t.Fatalf("execute propose: %v", err)
	}
	if !strings.Contains(out, "propose: created") {
		t.Fatalf("unexpected output: %q", out)
	}

	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	proposalDir := filepath.Join(tmp, ".ax", "proposals", proposalID)
	mustExist(t, filepath.Join(proposalDir, "proposal.md"))
	mustExist(t, filepath.Join(proposalDir, "design.md"))
	mustExist(t, filepath.Join(proposalDir, "tasks.md"))
	mustExist(t, filepath.Join(proposalDir, "specs"))

	proposalBody := mustRead(t, filepath.Join(proposalDir, "proposal.md"))
	for _, section := range []string{"## Problem / Goal", "## Scope / Out of Scope", "## Acceptance Criteria", "## Risks / Assumptions"} {
		if !strings.Contains(proposalBody, section) {
			t.Fatalf("proposal missing section %q", section)
		}
	}
	tasksBody := mustRead(t, filepath.Join(proposalDir, "tasks.md"))
	if got := strings.Count(tasksBody, "- [ ]"); got < 10 {
		t.Fatalf("expected at least 10 tasks, got %d", got)
	}

	state := readState(t, tmp)
	if state.Phase != core.PhaseProposal {
		t.Fatalf("unexpected phase: %s", state.Phase)
	}
	if state.Current.Proposal != proposalID {
		t.Fatalf("expected current proposal %q, got %q", proposalID, state.Current.Proposal)
	}
	if len(state.ContextChain) == 0 {
		t.Fatal("expected context chain entries")
	}
}

func TestPlanRequiresExistingProposal(t *testing.T) {
	setupTempCWD(t)

	_, err := executeAX(t, "plan", "--from", "missing-proposal")
	if err == nil {
		t.Fatal("expected error for missing proposal")
	}
}

func TestPlanCreatesRequiredSectionsFromProposal(t *testing.T) {
	tmp := setupTempCWD(t)

	if _, err := executeAX(t, "propose", "Planner"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))

	if _, err := executeAX(t, "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	planName := singleEntryName(t, filepath.Join(tmp, ".ax", "plans"))
	planBody := mustRead(t, filepath.Join(tmp, ".ax", "plans", planName))

	for _, section := range []string{"## Phase Plan", "## File / Module Candidates", "## Test Strategy", "## Rollback / Risk Response", "## Task Mapping"} {
		if !strings.Contains(planBody, section) {
			t.Fatalf("plan missing section %q", section)
		}
	}
	if !strings.Contains(planBody, "T-01") {
		t.Fatal("expected task mapping IDs in plan")
	}

	state := readState(t, tmp)
	if state.Phase != core.PhasePlanning {
		t.Fatalf("unexpected phase: %s", state.Phase)
	}
	if state.Current.Plan == "" {
		t.Fatal("expected current plan to be set")
	}
}

func TestPlanAcceptsProposalPathInput(t *testing.T) {
	tmp := setupTempCWD(t)

	if _, err := executeAX(t, "propose", "Planner Path"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	proposalPath := filepath.Join(tmp, ".ax", "proposals", proposalID, "proposal.md")

	if _, err := executeAX(t, "plan", "--from", proposalPath); err != nil {
		t.Fatalf("plan from path: %v", err)
	}

	state := readState(t, tmp)
	if state.Current.Proposal != proposalID {
		t.Fatalf("expected current proposal %q, got %q", proposalID, state.Current.Proposal)
	}
}

func TestDiscoverPartyCreatesRequiredSectionsAndState(t *testing.T) {
	tmp := setupTempCWD(t)
	if err := os.WriteFile(filepath.Join(tmp, "topic-notes.md"), []byte("latency tuning notes"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := executeAX(t, "discover", "latency", "--party"); err != nil {
		t.Fatalf("discover: %v", err)
	}

	reportName := singleEntryName(t, filepath.Join(tmp, ".ax", "discovery"))
	report := mustRead(t, filepath.Join(tmp, ".ax", "discovery", reportName))
	for _, section := range []string{"## 문제 재정의", "## 옵션 A/B/C", "## 트레이드오프", "## 리스크", "## 추천안", "### Architect", "### User", "### QA"} {
		if !strings.Contains(report, section) {
			t.Fatalf("discover report missing section %q", section)
		}
	}

	state := readState(t, tmp)
	if state.Phase != core.PhaseDiscovery {
		t.Fatalf("unexpected phase: %s", state.Phase)
	}
}

func TestDiscoverAllowedAfterPlanning(t *testing.T) {
	tmp := setupTempCWD(t)
	if err := os.WriteFile(filepath.Join(tmp, "notes.md"), []byte("quick"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := executeAX(t, "propose", "Discover After Plan"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	if _, err := executeAX(t, "discover", "quick", "--party"); err != nil {
		t.Fatalf("discover after planning should be allowed: %v", err)
	}

	state := readState(t, tmp)
	if state.Phase != core.PhaseDiscovery {
		t.Fatalf("expected discovery phase, got %s", state.Phase)
	}
}

func TestRunTDDCreatesLogsAndUpdatesState(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "propose", "Run Flow"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	planName := singleEntryName(t, filepath.Join(tmp, ".ax", "plans"))

	if _, err := executeAX(t, "run", "--plan", planName, "--tdd", "--loop", "--depth", "deep", "--approval-policy", "on-failure"); err != nil {
		t.Fatalf("run --tdd: %v", err)
	}

	runName := latestEntryName(t, filepath.Join(tmp, ".ax", "runs"))
	runBody := mustRead(t, filepath.Join(tmp, ".ax", "runs", runName))
	for _, want := range []string{"## TDD Loop", "- mode: tdd", "- loop_mode: enabled", "- depth: deep", "- depth_source: user", "T0", "T1", "T2"} {
		if !strings.Contains(runBody, want) {
			t.Fatalf("run report missing %q", want)
		}
	}
	for _, mapping := range []string{"| T0/Red | turn-001 | T0 |", "| T0/Green | turn-002 | T0 |", "| T0/Refactor | turn-003 | T0 |"} {
		if !strings.Contains(runBody, mapping) {
			t.Fatalf("run report missing step-turn mapping %q", mapping)
		}
	}

	logs, err := os.ReadDir(filepath.Join(tmp, ".ax", "logs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) < 2 {
		t.Fatalf("expected at least start/end logs, got %d", len(logs))
	}
	firstLog := mustRead(t, filepath.Join(tmp, ".ax", "logs", logs[0].Name()))
	for _, field := range []string{"step:", "phase:", "thread_id:", "turn_id:", "error_code:"} {
		if !strings.Contains(firstLog, field) {
			t.Fatalf("log missing field %q", field)
		}
	}

	state := readState(t, tmp)
	if state.Phase != core.PhaseImplementation {
		t.Fatalf("unexpected phase: %s", state.Phase)
	}
	if state.TDD.CurrentTier == "" || state.TDD.TotalSteps == 0 {
		t.Fatalf("expected tdd progress in state: %+v", state.TDD)
	}
}

func TestLayoutCreatesSkillRegistryAndTemplateContracts(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "state"); err != nil {
		t.Fatalf("state init: %v", err)
	}

	registry := mustRead(t, filepath.Join(tmp, ".ax", "skills", "registry.yaml"))
	for _, skill := range []string{"interview", "openspec", "tdd-plan", "tdd-go", "tdd-go-loop", "superpowers"} {
		if !strings.Contains(registry, skill) {
			t.Fatalf("skill registry missing %q", skill)
		}
	}

	if _, err := executeAX(t, "propose", "Template Contract"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	proposalBody := mustRead(t, filepath.Join(tmp, ".ax", "proposals", proposalID, "proposal.md"))
	if !strings.Contains(proposalBody, "## Interview Notes") {
		t.Fatalf("proposal missing interview section:\n%s", proposalBody)
	}

	if _, err := executeAX(t, "discover", "contract", "--party"); err != nil {
		t.Fatalf("discover --party: %v", err)
	}
	discovery := mustRead(t, filepath.Join(tmp, ".ax", "discovery", singleEntryName(t, filepath.Join(tmp, ".ax", "discovery"))))
	for _, section := range []string{"## Party Mode", "### Architect", "### User", "### QA"} {
		if !strings.Contains(discovery, section) {
			t.Fatalf("discover missing party section %q", section)
		}
	}
}

func TestVerifyGeneratesVerdictAndEvidenceScaffold(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "propose", "Verify Me"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))

	if _, err := executeAX(t, "verify", "--proposal", proposalID); err != nil {
		t.Fatalf("verify: %v", err)
	}

	verifyBody := mustRead(t, filepath.Join(tmp, ".ax", "proposals", proposalID, "verify.md"))
	for _, want := range []string{"status: CONDITIONAL PASS", "## Evidence", "- tests:", "- build:", "- acceptance_criteria:", "## Unmet Items", "## Next Actions"} {
		if !strings.Contains(verifyBody, want) {
			t.Fatalf("verify report missing %q", want)
		}
	}

	state := readState(t, tmp)
	if state.Phase != core.PhaseVerification {
		t.Fatalf("unexpected phase: %s", state.Phase)
	}
}

func TestVerifyAllowedAfterDiscover(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "propose", "Verify After Discover"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	if _, err := executeAX(t, "discover", "verify", "--party"); err != nil {
		t.Fatalf("discover: %v", err)
	}
	if _, err := executeAX(t, "verify", "--proposal", proposalID, "--tests", "pass", "--build", "pass", "--ac", "pass"); err != nil {
		t.Fatalf("verify after discover should pass: %v", err)
	}

	verifyBody := mustRead(t, filepath.Join(tmp, ".ax", "proposals", proposalID, "verify.md"))
	if !strings.Contains(verifyBody, "status: PASS") {
		t.Fatalf("expected PASS verify report, got:\n%s", verifyBody)
	}
}

func TestVerifyAcceptsProposalPathInput(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "propose", "Verify Path"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	proposalPath := filepath.Join(tmp, ".ax", "proposals", proposalID, "proposal.md")

	if _, err := executeAX(t, "verify", "--proposal", proposalPath, "--tests", "pass", "--build", "pass", "--ac", "pass"); err != nil {
		t.Fatalf("verify from path: %v", err)
	}

	verifyBody := mustRead(t, filepath.Join(tmp, ".ax", "proposals", proposalID, "verify.md"))
	if !strings.Contains(verifyBody, "status: PASS") {
		t.Fatalf("expected PASS verify report, got:\n%s", verifyBody)
	}
}

func TestVerifyWritesDiffAgainstPreviousResult(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "propose", "Verify Diff"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))

	if _, err := executeAX(t, "verify", "--proposal", proposalID, "--tests", "unknown", "--build", "unknown", "--ac", "unknown"); err != nil {
		t.Fatalf("first verify: %v", err)
	}
	if _, err := executeAX(t, "verify", "--proposal", proposalID, "--tests", "pass", "--build", "pass", "--ac", "pass"); err != nil {
		t.Fatalf("second verify: %v", err)
	}

	diffPath := filepath.Join(tmp, ".ax", "proposals", proposalID, "verify-diff.md")
	mustExist(t, diffPath)
	diffBody := mustRead(t, diffPath)
	for _, field := range []string{"# Verify Diff", "previous_status:", "current_status:", "## Diff vs Previous Verify", "unmet_added:", "unmet_removed:"} {
		if !strings.Contains(diffBody, field) {
			t.Fatalf("verify diff missing %q", field)
		}
	}

	verifyBody := mustRead(t, filepath.Join(tmp, ".ax", "proposals", proposalID, "verify.md"))
	if !strings.Contains(verifyBody, "## Previous Verify Diff") {
		t.Fatalf("verify report missing previous diff section:\n%s", verifyBody)
	}
}

func TestArchiveStopsBeforeMoveOnInvalidTransition(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "propose", "Archive Invalid Transition"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "verify", "--proposal", proposalID, "--tests", "pass", "--build", "pass", "--ac", "pass"); err != nil {
		t.Fatalf("verify: %v", err)
	}

	st := readState(t, tmp)
	st.ForcePhase(core.PhaseIdle, "test:force-idle", time.Now())
	if err := core.SaveState(tmp, st); err != nil {
		t.Fatalf("save state: %v", err)
	}

	if _, err := executeAX(t, "archive", "--proposal", proposalID); err == nil {
		t.Fatal("expected archive to fail on invalid transition")
	}
	mustExist(t, filepath.Join(tmp, ".ax", "proposals", proposalID))
	if _, err := os.Stat(filepath.Join(tmp, ".ax", "archive", proposalID)); !os.IsNotExist(err) {
		t.Fatalf("expected archive dir to not exist yet")
	}
}

func TestArchiveStrictModeAndMetadata(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "propose", "Archive Me"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))

	if _, err := executeAX(t, "archive", "--proposal", proposalID); err == nil {
		t.Fatal("expected archive to fail without verify")
	}

	if _, err := executeAX(t, "verify", "--proposal", proposalID, "--tests", "pass", "--build", "pass", "--ac", "pass"); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if _, err := executeAX(t, "archive", "--proposal", proposalID); err != nil {
		t.Fatalf("archive: %v", err)
	}

	archiveDir := filepath.Join(tmp, ".ax", "archive", proposalID)
	mustExist(t, archiveDir)
	meta := mustRead(t, filepath.Join(archiveDir, "archive-metadata.yaml"))
	for _, field := range []string{"proposal_id:", "archived_at:", "final_status:", "artifact_paths:", "verification_result:"} {
		if !strings.Contains(meta, field) {
			t.Fatalf("metadata missing %q", field)
		}
	}

	state := readState(t, tmp)
	if state.Phase != core.PhaseArchived {
		t.Fatalf("unexpected phase: %s", state.Phase)
	}
}

func TestArchiveAllowUnverifiedOverride(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "propose", "Archive Override"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))

	// Maintain state-machine validity while still skipping verify artifact generation.
	st := readState(t, tmp)
	st.ForcePhase(core.PhaseVerification, "test:force-verification", time.Now())
	if err := core.SaveState(tmp, st); err != nil {
		t.Fatalf("save state: %v", err)
	}

	if _, err := executeAX(t, "archive", "--proposal", proposalID, "--allow-unverified-archive"); err != nil {
		t.Fatalf("archive allow-unverified: %v", err)
	}

	mustExist(t, filepath.Join(tmp, ".ax", "archive", proposalID))
	state := readState(t, tmp)
	if state.Phase != core.PhaseArchived {
		t.Fatalf("unexpected phase: %s", state.Phase)
	}
}

func TestArchiveIsIdempotent(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "propose", "Archive Idempotent"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "verify", "--proposal", proposalID, "--tests", "pass", "--build", "pass", "--ac", "pass"); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if _, err := executeAX(t, "archive", "--proposal", proposalID); err != nil {
		t.Fatalf("first archive: %v", err)
	}
	if _, err := executeAX(t, "archive", "--proposal", proposalID); err != nil {
		t.Fatalf("second archive should be idempotent: %v", err)
	}

	if got := countEntries(t, filepath.Join(tmp, ".ax", "archive")); got != 1 {
		t.Fatalf("expected one archived proposal, got %d", got)
	}
}

func TestCanStartNewProposalAfterArchive(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "propose", "Cycle One"); err != nil {
		t.Fatalf("propose #1: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "verify", "--proposal", proposalID, "--tests", "pass", "--build", "pass", "--ac", "pass"); err != nil {
		t.Fatalf("verify #1: %v", err)
	}
	if _, err := executeAX(t, "archive", "--proposal", proposalID); err != nil {
		t.Fatalf("archive #1: %v", err)
	}

	if _, err := executeAX(t, "propose", "Cycle Two"); err != nil {
		t.Fatalf("propose #2 after archive should succeed: %v", err)
	}
}

func TestQuickEscalatesWhenThresholdExceeded(t *testing.T) {
	tmp := setupTempCWD(t)

	out, err := executeAX(t, "quick", "large-change", "--files-changed", "6")
	if err != nil {
		t.Fatalf("quick: %v", err)
	}
	if !strings.Contains(out, "quick: escalated") {
		t.Fatalf("expected escalation output, got %q", out)
	}

	if got := countEntries(t, filepath.Join(tmp, ".ax", "runs")); got != 0 {
		t.Fatalf("expected 0 runs for escalated quick, got %d", got)
	}
	if got := countEntries(t, filepath.Join(tmp, ".ax", "plans")); got != 1 {
		t.Fatalf("expected plan to be created during escalation, got %d", got)
	}
}

func TestStateDetailedOutputAndReviewCompoundBasics(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "propose", "Reviewable"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	gotchas := strings.Join([]string{
		"# Gotchas",
		`- {"id":"g-fix","text":"Fix flaky login test","added":"2025-01-01T00:00:00Z","last_relevant":"2025-01-01T00:00:00Z","decay_after":"30d","fix_candidate":true}`,
		`- {"id":"g-noise","text":"See cmd/ax/commands.go for defaults","added":"2025-01-01T00:00:00Z","last_relevant":"2025-01-01T00:00:00Z","decay_after":"30d","fix_candidate":false}`,
		"- Update onboarding docs for archive flow",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(tmp, ".ax", "memory", "gotchas.md"), []byte(gotchas), 0o644); err != nil {
		t.Fatalf("write gotchas: %v", err)
	}

	if _, err := executeAX(t, "review", "--proposal", proposalID); err != nil {
		t.Fatalf("review: %v", err)
	}
	if _, err := executeAX(t, "compound", "--audit"); err != nil {
		t.Fatalf("compound: %v", err)
	}

	stateOut, err := executeAX(t, "state")
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	for _, field := range []string{"phase:", "current proposal:", "current plan:", "last result:", "last error:", "context_chain:"} {
		if !strings.Contains(stateOut, field) {
			t.Fatalf("state output missing %q", field)
		}
	}

	if got := countEntries(t, filepath.Join(tmp, ".ax", "reviews")); got != 1 {
		t.Fatalf("expected 1 review artifact, got %d", got)
	}
	review := mustRead(t, filepath.Join(tmp, ".ax", "reviews", singleEntryName(t, filepath.Join(tmp, ".ax", "reviews"))))
	for _, lens := range []string{"Correctness", "Reliability / Recovery", "Test Adequacy", "Observability", "Security / Safety", "Maintainability", "Performance", "UX / CLI Clarity"} {
		if !strings.Contains(review, lens) {
			t.Fatalf("review missing lens %q", lens)
		}
	}

	if got := countEntries(t, filepath.Join(tmp, ".ax", "compound")); got != 1 {
		t.Fatalf("expected 1 compound artifact, got %d", got)
	}
	compound := mustRead(t, filepath.Join(tmp, ".ax", "compound", singleEntryName(t, filepath.Join(tmp, ".ax", "compound"))))
	for _, field := range []string{
		"### FixCandidate", "### Document", "### Noise",
		"id:", "text:", "added:", "last_relevant:", "decay_after:", "fix_candidate:",
		"## Audit", "### Decay Candidates", "### Archive Candidates",
	} {
		if !strings.Contains(compound, field) {
			t.Fatalf("compound report missing %q", field)
		}
	}
}

func TestRunResumeScaffolding(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "propose", "Resume Flow"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	planName := singleEntryName(t, filepath.Join(tmp, ".ax", "plans"))

	if _, err := executeAX(t, "run", "--plan", planName, "--tdd"); err != nil {
		t.Fatalf("initial run --tdd: %v", err)
	}

	state := readState(t, tmp)
	state.Run.LastFailedStep = "T1/green"
	if err := core.SaveState(tmp, state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	if _, err := executeAX(t, "run", "--plan", planName, "--tdd", "--resume"); err != nil {
		t.Fatalf("run resume: %v", err)
	}

	runName := singleEntryName(t, filepath.Join(tmp, ".ax", "runs"))
	runBody := mustRead(t, filepath.Join(tmp, ".ax", "runs", runName))
	if !strings.Contains(runBody, "- resumed_from: T1/green") {
		t.Fatalf("expected resumed_from scaffold, got:\n%s", runBody)
	}
	state = readState(t, tmp)
	if state.TDD.CurrentTier != "T1" || state.TDD.CompletedSteps != 6 {
		t.Fatalf("expected TDD to progress to T1 with 6 completed steps, got %+v", state.TDD)
	}
}

func TestRunTDDTierProgressionGate(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "propose", "Tier Gate Flow"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	planName := singleEntryName(t, filepath.Join(tmp, ".ax", "plans"))

	if _, err := executeAX(t, "run", "--plan", planName, "--tdd"); err != nil {
		t.Fatalf("run t0: %v", err)
	}
	st := readState(t, tmp)
	if st.TDD.CurrentTier != "T0" || st.TDD.CompletedSteps != 3 {
		t.Fatalf("expected T0/3 after first run, got %+v", st.TDD)
	}

	if _, err := executeAX(t, "run", "--plan", planName, "--tdd", "--resume"); err != nil {
		t.Fatalf("run t1: %v", err)
	}
	st = readState(t, tmp)
	if st.TDD.CurrentTier != "T1" || st.TDD.CompletedSteps != 6 {
		t.Fatalf("expected T1/6 after second run, got %+v", st.TDD)
	}

	if _, err := executeAX(t, "run", "--plan", planName, "--tdd", "--resume"); err != nil {
		t.Fatalf("run t2: %v", err)
	}
	st = readState(t, tmp)
	if st.TDD.CurrentTier != "T2" || st.TDD.CompletedSteps != 9 {
		t.Fatalf("expected T2/9 after third run, got %+v", st.TDD)
	}
}

func TestRunResumeWithoutPersistedTDDFails(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "propose", "Resume Missing State"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	planName := singleEntryName(t, filepath.Join(tmp, ".ax", "plans"))
	if _, err := executeAX(t, "run", "--plan", planName, "--tdd", "--resume"); err == nil {
		t.Fatal("expected resume without persisted TDD state to fail")
	}
}

func TestRunDecisionRejectStoresFailure(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "propose", "Reject Flow"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	planName := singleEntryName(t, filepath.Join(tmp, ".ax", "plans"))

	if _, err := executeAX(t, "run", "--plan", planName, "--decision", "reject"); err == nil {
		t.Fatal("expected run reject decision to fail")
	}

	state := readState(t, tmp)
	if state.LastError == nil || state.LastError.Code != "E_RUN_REJECTED" {
		t.Fatalf("expected E_RUN_REJECTED in state, got %+v", state.LastError)
	}
	if got := countEntries(t, filepath.Join(tmp, ".ax", "worktrees")); got != 0 {
		t.Fatalf("expected no worktree side effects on rejected run, got %d entries", got)
	}
}

func TestRunNoWorktreeDisablesCreation(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "propose", "No Worktree Flow"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	planName := singleEntryName(t, filepath.Join(tmp, ".ax", "plans"))

	if _, err := executeAX(t, "run", "--plan", planName, "--no-worktree"); err != nil {
		t.Fatalf("run --no-worktree: %v", err)
	}

	if got := countEntries(t, filepath.Join(tmp, ".ax", "worktrees")); got != 0 {
		t.Fatalf("expected no worktree entries, got %d", got)
	}
	runBody := mustRead(t, filepath.Join(tmp, ".ax", "runs", singleEntryName(t, filepath.Join(tmp, ".ax", "runs"))))
	if !strings.Contains(runBody, "- worktree_mode: disabled") {
		t.Fatalf("expected disabled worktree mode in run report, got:\n%s", runBody)
	}
}

func TestRunCreatesWorktreeMetadataByDefault(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "propose", "Worktree Flow"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	planName := singleEntryName(t, filepath.Join(tmp, ".ax", "plans"))

	if _, err := executeAX(t, "run", "--plan", planName); err != nil {
		t.Fatalf("run: %v", err)
	}

	metaPath := filepath.Join(tmp, ".ax", "worktrees", proposalID, "worktree.yaml")
	mustExist(t, metaPath)
	meta := mustRead(t, metaPath)
	if !strings.Contains(meta, "status: prepared") {
		t.Fatalf("unexpected worktree metadata:\n%s", meta)
	}
}

func TestArchiveFinalizesAndCleansWorktree(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "propose", "Archive Worktree Lifecycle"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	planName := singleEntryName(t, filepath.Join(tmp, ".ax", "plans"))
	if _, err := executeAX(t, "run", "--plan", planName); err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, err := executeAX(t, "verify", "--proposal", proposalID, "--tests", "pass", "--build", "pass", "--ac", "pass"); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if _, err := executeAX(t, "archive", "--proposal", proposalID); err != nil {
		t.Fatalf("archive: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmp, ".ax", "worktrees", proposalID)); !os.IsNotExist(err) {
		t.Fatalf("expected worktree dir to be cleaned, err=%v", err)
	}
	worktreeSnapshot := mustRead(t, filepath.Join(tmp, ".ax", "archive", proposalID, "worktree.yaml"))
	if !strings.Contains(worktreeSnapshot, "status: merged_and_cleaned") {
		t.Fatalf("unexpected archived worktree snapshot:\n%s", worktreeSnapshot)
	}
	meta := mustRead(t, filepath.Join(tmp, ".ax", "archive", proposalID, "archive-metadata.yaml"))
	if !strings.Contains(meta, fmt.Sprintf(".ax/archive/%s/worktree.yaml", proposalID)) {
		t.Fatalf("archive metadata missing worktree artifact path:\n%s", meta)
	}
}

func TestRunWorktreePathDeterministicFromPlanFromField(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "propose", "Custom Plan Source"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	// Build deterministic custom plan path independent of filename.
	customPlanPath := filepath.Join(tmp, ".ax", "plans", "custom-run-plan.md")
	customPlan := strings.Join([]string{
		"# Plan",
		"",
		"- from: proposal-abc-123",
		"",
		"## Task Mapping",
		"- [ ] T-01",
		"",
	}, "\n")
	if err := os.WriteFile(customPlanPath, []byte(customPlan), 0o644); err != nil {
		t.Fatalf("write custom plan: %v", err)
	}

	// Ensure stale current proposal does not influence worktree path.
	st := readState(t, tmp)
	st.Current.Proposal = "different-proposal"
	st.ForcePhase(core.PhasePlanning, "test:planning", time.Now())
	if err := core.SaveState(tmp, st); err != nil {
		t.Fatalf("save state: %v", err)
	}

	if _, err := executeAX(t, "run", "--plan", customPlanPath); err != nil {
		t.Fatalf("run custom plan: %v", err)
	}

	mustExist(t, filepath.Join(tmp, ".ax", "worktrees", "proposal-abc-123", "worktree.yaml"))
	if _, err := os.Stat(filepath.Join(tmp, ".ax", "worktrees", "custom-run")); err == nil {
		t.Fatal("did not expect worktree path to derive from plan filename")
	}
}

func TestRunApprovalPolicyEngine(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "propose", "Approval Policy"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	planName := singleEntryName(t, filepath.Join(tmp, ".ax", "plans"))

	if _, err := executeAX(t, "run", "--plan", planName, "--approval-policy", "always"); err != nil {
		t.Fatalf("run with always policy: %v", err)
	}
	runName := singleEntryName(t, filepath.Join(tmp, ".ax", "runs"))
	runBody := mustRead(t, filepath.Join(tmp, ".ax", "runs", runName))
	if !strings.Contains(runBody, "- approval_required: true") || !strings.Contains(runBody, "- approval_reason: always") {
		t.Fatalf("approval policy details missing from run report:\n%s", runBody)
	}

	if _, err := executeAX(t, "run", "--plan", planName, "--approval-policy", "on-failure", "--decision", "reject"); err == nil {
		t.Fatal("expected reject under on-failure to fail")
	}
	st := readState(t, tmp)
	if st.LastError == nil || st.LastError.Code != "E_RUN_APPROVAL_REQUIRED" {
		t.Fatalf("expected approval-required error code, got %+v", st.LastError)
	}
}

func TestRunContextCompactionWritesLog(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "propose", "Compaction Flow"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	planName := singleEntryName(t, filepath.Join(tmp, ".ax", "plans"))

	st := readState(t, tmp)
	st.ContextChain = nil
	for i := 0; i < core.MaxContextChainEntries; i++ {
		st.ContextChain = append(st.ContextChain, filepath.ToSlash(filepath.Join(".ax", "ctx", fmt.Sprintf("item-%03d.md", i))))
	}
	if err := core.SaveState(tmp, st); err != nil {
		t.Fatalf("save prefilled state: %v", err)
	}

	if _, err := executeAX(t, "run", "--plan", planName); err != nil {
		t.Fatalf("run with context compaction: %v", err)
	}
	st = readState(t, tmp)
	if len(st.ContextChain) > core.MaxContextChainEntries {
		t.Fatalf("expected context chain to stay within max=%d, got %d", core.MaxContextChainEntries, len(st.ContextChain))
	}
	if len(st.ContextChain) == 0 {
		t.Fatal("expected context chain to retain compacted entries")
	}

	logEntries, err := os.ReadDir(filepath.Join(tmp, ".ax", "logs"))
	if err != nil {
		t.Fatalf("read logs: %v", err)
	}
	found := false
	for _, entry := range logEntries {
		body := mustRead(t, filepath.Join(tmp, ".ax", "logs", entry.Name()))
		if strings.Contains(body, "step: context_compaction") && strings.Contains(body, "error_code: E_CONTEXT_COMPACTION") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected context compaction observability log")
	}
}

func TestRunQualityGateBlocksWithoutForceAndAllowsWithReason(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "propose", "Quality Gate"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	planName := singleEntryName(t, filepath.Join(tmp, ".ax", "plans"))

	if _, err := executeAX(t, "run", "--plan", planName, "--tdd", "--review-count", "4"); err == nil {
		t.Fatal("expected quality gate failure without --force")
	}

	out, err := executeAX(t, "run", "--plan", planName, "--tdd", "--review-count", "4", "--force", "--force-reason", "manual override")
	if err != nil {
		t.Fatalf("forced run should pass: %v", err)
	}
	if !strings.Contains(out, "run: created") {
		t.Fatalf("unexpected output: %q", out)
	}

	runName := singleEntryName(t, filepath.Join(tmp, ".ax", "runs"))
	runBody := mustRead(t, filepath.Join(tmp, ".ax", "runs", runName))
	if !strings.Contains(runBody, "- review_count: 4") {
		t.Fatalf("expected review_count in run report, got:\n%s", runBody)
	}
	if !strings.Contains(runBody, "- quality_gate: forced-override") {
		t.Fatalf("expected forced-override quality gate in report, got:\n%s", runBody)
	}
}

func TestRunQualityGateHardLimit(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "propose", "Quality Gate Hard Limit"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	planName := singleEntryName(t, filepath.Join(tmp, ".ax", "plans"))

	if _, err := executeAX(t, "run", "--plan", planName, "--tdd", "--review-count", "6", "--force", "--force-reason", "manual override"); err == nil {
		t.Fatal("expected quality gate hard-limit error")
	}
}

func executeAX(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func setupTempCWD(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(prev) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	return tmp
}

func readState(t *testing.T, base string) *core.State {
	t.Helper()
	st, err := core.LoadState(base)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	return st
}

func countEntries(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatal(err)
	}
	return len(entries)
}

func singleEntryName(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one entry in %s, got %d", dir, len(entries))
	}
	return entries[0].Name()
}

func latestEntryName(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatalf("expected entries in %s", dir)
	}
	latest := entries[0]
	latestInfo, err := latest.Info()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries[1:] {
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		if info.ModTime().After(latestInfo.ModTime()) || (info.ModTime().Equal(latestInfo.ModTime()) && entry.Name() > latest.Name()) {
			latest = entry
			latestInfo = info
		}
	}
	return latest.Name()
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func mustExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected path to exist %s: %v", path, err)
	}
}
