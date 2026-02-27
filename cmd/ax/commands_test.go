package ax

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/flowkater/ax/internal/codex"
	"github.com/flowkater/ax/internal/core"
)

type interruptFailingAdapter struct {
	codex.AppServerAdapter
}

func (a interruptFailingAdapter) InterruptTurn(_ context.Context, _, _ string) (*codex.InterruptResult, error) {
	return nil, errors.New("interrupt downstream failed")
}

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

func TestPlanRejectsProposalPathOutsideBase(t *testing.T) {
	tmp := setupTempCWD(t)

	outside := t.TempDir()
	escapeProposalDir := filepath.Join(outside, "escape-proposal")
	if err := os.MkdirAll(escapeProposalDir, 0o755); err != nil {
		t.Fatalf("mkdir escape proposal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(escapeProposalDir, "proposal.md"), []byte("# Proposal"), 0o644); err != nil {
		t.Fatalf("write escape proposal: %v", err)
	}
	relEscapePath, err := filepath.Rel(tmp, filepath.Join(escapeProposalDir, "proposal.md"))
	if err != nil {
		t.Fatalf("relative escape path: %v", err)
	}

	if _, err := executeAX(t, "plan", "--from", relEscapePath); err == nil {
		t.Fatal("expected plan to reject proposal path outside base")
	} else if !strings.Contains(err.Error(), "AX_INPUT_PATH_OUT_OF_SCOPE") {
		t.Fatalf("expected AX_INPUT_PATH_OUT_OF_SCOPE, got: %v", err)
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

func TestSharedRuntimeModeAllowsProposeAfterPlanning(t *testing.T) {
	tmp := setupTempCWD(t)

	if _, err := executeAX(t, "--runtime-mode", "shared", "propose", "Shared One"); err != nil {
		t.Fatalf("propose #1: %v", err)
	}
	first := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "--runtime-mode", "shared", "plan", "--from", first); err != nil {
		t.Fatalf("plan #1: %v", err)
	}
	if _, err := executeAX(t, "--runtime-mode", "shared", "propose", "Shared Two"); err != nil {
		t.Fatalf("propose #2 in shared mode should succeed from planning phase: %v", err)
	}

	if got := countEntries(t, filepath.Join(tmp, ".ax", "proposals")); got != 2 {
		t.Fatalf("expected 2 proposals, got %d", got)
	}
	st := readState(t, tmp)
	if st.Runtime.Mode != core.RuntimeModeShared {
		t.Fatalf("expected runtime mode shared, got %s", st.Runtime.Mode)
	}
}

func TestRuntimeMetadataPersistsFromFlagsAndDoctorRuntimeJSON(t *testing.T) {
	tmp := setupTempCWD(t)

	if _, err := executeAX(t, "--runtime-mode", "shared", "--session-id", "sess-meta-1", "--cluster-id", "cluster-a", "--node-id", "node-a", "propose", "Runtime Metadata"); err != nil {
		t.Fatalf("propose with runtime metadata: %v", err)
	}

	st := readState(t, tmp)
	if st.Runtime.Mode != core.RuntimeModeShared {
		t.Fatalf("expected runtime mode shared, got %s", st.Runtime.Mode)
	}
	if st.Runtime.SessionID != "sess-meta-1" {
		t.Fatalf("expected session-id sess-meta-1, got %s", st.Runtime.SessionID)
	}
	if st.Runtime.ClusterID != "cluster-a" {
		t.Fatalf("expected cluster-id cluster-a, got %s", st.Runtime.ClusterID)
	}
	if st.Runtime.NodeID != "node-a" {
		t.Fatalf("expected node-id node-a, got %s", st.Runtime.NodeID)
	}

	out, err := executeAX(t, "doctor", "runtime", "--json")
	if err != nil {
		t.Fatalf("doctor runtime json: %v", err)
	}
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("unmarshal doctor runtime json: %v\nout=%s", err, out)
	}
	if got := fmt.Sprint(payload["cluster_id"]); got != "cluster-a" {
		t.Fatalf("expected doctor cluster_id=cluster-a, got %s", got)
	}
	if got := fmt.Sprint(payload["node_id"]); got != "node-a" {
		t.Fatalf("expected doctor node_id=node-a, got %s", got)
	}
	if got := fmt.Sprint(payload["journal_path"]); got == "" || got == "<nil>" {
		t.Fatalf("expected doctor journal_path to be populated, got %q", got)
	}
}

func TestRunSharedRuntimeModeCreatesSessionIsolatedWorktree(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "--runtime-mode", "shared", "--session-id", "sess-shared-a", "propose", "Shared Worktree Runtime"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "--runtime-mode", "shared", "--session-id", "sess-shared-a", "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	planName := singleEntryName(t, filepath.Join(tmp, ".ax", "plans"))

	if _, err := executeAX(t, "--runtime-mode", "shared", "--session-id", "sess-shared-a", "run", "--plan", planName); err != nil {
		t.Fatalf("run shared: %v", err)
	}

	metaPath := filepath.Join(tmp, ".ax", "worktrees", proposalID, "sess-shared-a", "worktree.yaml")
	mustExist(t, metaPath)
	meta := mustRead(t, metaPath)
	for _, field := range []string{"session_id: sess-shared-a", "runtime_mode: shared", "status: prepared"} {
		if !strings.Contains(meta, field) {
			t.Fatalf("shared runtime worktree metadata missing %q:\n%s", field, meta)
		}
	}
}

func TestRecoverResumeRestoresTDDSnapshotFromCheckpoint(t *testing.T) {
	tmp := setupTempCWD(t)
	const sessionID = "sess-recover-checkpoint"

	if _, err := executeAX(t, "--session-id", sessionID, "propose", "Recover Checkpoint"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "--session-id", sessionID, "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	planName := singleEntryName(t, filepath.Join(tmp, ".ax", "plans"))

	if _, err := executeAX(t, "--session-id", sessionID, "run", "--plan", planName, "--tdd"); err != nil {
		t.Fatalf("run --tdd: %v", err)
	}

	checkpointPath := filepath.Join(tmp, ".ax", "runtime", "checkpoints", sessionID+".json")
	mustExist(t, checkpointPath)
	checkpoint := map[string]any{
		"session_id": sessionID,
		"status":     "in_progress",
		"tdd": map[string]any{
			"enabled":         true,
			"current_tier":    "T1",
			"current_step":    "green",
			"completed_steps": 6,
			"total_steps":     9,
			"depth":           "normal",
			"depth_source":    "auto",
			"approval_policy": "never",
		},
	}
	body, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		t.Fatalf("marshal checkpoint: %v", err)
	}
	if err := os.WriteFile(checkpointPath, append(body, '\n'), 0o644); err != nil {
		t.Fatalf("write checkpoint: %v", err)
	}

	st := readState(t, tmp)
	st.TDD = core.TDDState{}
	if err := core.SaveState(tmp, st); err != nil {
		t.Fatalf("save cleared state: %v", err)
	}

	if _, err := executeAX(t, "--session-id", sessionID, "recover", "--strategy", "resume"); err != nil {
		t.Fatalf("recover --strategy resume: %v", err)
	}

	st = readState(t, tmp)
	if !st.TDD.Enabled {
		t.Fatalf("expected tdd to be restored from checkpoint, got %+v", st.TDD)
	}
	if st.TDD.CurrentTier != "T1" || st.TDD.CompletedSteps != 6 {
		t.Fatalf("unexpected restored tdd state: %+v", st.TDD)
	}
}

func TestRuntimeModeUsesAXRuntimeModeWhenFlagUnset(t *testing.T) {
	tmp := setupTempCWD(t)
	t.Setenv("AX_RUNTIME_MODE", "shared")

	if _, err := executeAX(t, "propose", "Env Runtime Mode"); err != nil {
		t.Fatalf("propose: %v", err)
	}

	st := readState(t, tmp)
	if st.Runtime.Mode != core.RuntimeModeShared {
		t.Fatalf("expected runtime mode shared from AX_RUNTIME_MODE, got %s", st.Runtime.Mode)
	}
}

func TestRuntimeSessionMetadataTracksClusterNodeAndCommands(t *testing.T) {
	tmp := setupTempCWD(t)
	t.Setenv("AX_CLUSTER_ID", "cluster-e2e")
	t.Setenv("AX_NODE_ID", "node-e2e")
	const sessionID = "sess-meta-001"

	if _, err := executeAX(t, "--runtime-mode", "shared", "--session-id", sessionID, "propose", "Runtime Meta"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "--runtime-mode", "shared", "--session-id", sessionID, "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}

	st := readState(t, tmp)
	if st.Runtime.ClusterID != "cluster-e2e" {
		t.Fatalf("expected cluster id cluster-e2e, got %q", st.Runtime.ClusterID)
	}
	if st.Runtime.NodeID != "node-e2e" {
		t.Fatalf("expected node id node-e2e, got %q", st.Runtime.NodeID)
	}

	meta, ok := st.Runtime.SessionMeta[sessionID]
	if !ok {
		t.Fatalf("expected runtime session metadata for %s", sessionID)
	}
	if meta.Command != "plan" {
		t.Fatalf("expected command plan, got %q", meta.Command)
	}
	if meta.Mode != core.RuntimeModeShared {
		t.Fatalf("expected mode shared, got %q", meta.Mode)
	}
	if meta.ClusterID != "cluster-e2e" {
		t.Fatalf("expected metadata cluster id cluster-e2e, got %q", meta.ClusterID)
	}
	if meta.NodeID != "node-e2e" {
		t.Fatalf("expected metadata node id node-e2e, got %q", meta.NodeID)
	}
	if meta.Status != "active" {
		t.Fatalf("expected metadata status active, got %q", meta.Status)
	}
	if meta.StartedAt == "" || meta.UpdatedAt == "" {
		t.Fatalf("expected metadata timestamps to be set, got %+v", meta)
	}
}

func TestRunPersistsRuntimeSessionMetadataAndJournal(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "--runtime-mode", "shared", "--session-id", "sess-meta", "propose", "Runtime Metadata"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "--runtime-mode", "shared", "--session-id", "sess-meta", "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	planName := singleEntryName(t, filepath.Join(tmp, ".ax", "plans"))
	if _, err := executeAX(t, "--runtime-mode", "shared", "--session-id", "sess-meta", "run", "--plan", planName, "--tdd"); err != nil {
		t.Fatalf("run: %v", err)
	}

	st := readState(t, tmp)
	if st.Runtime.ClusterID == "" {
		t.Fatal("expected cluster_id in runtime state")
	}
	if st.Runtime.NodeID == "" {
		t.Fatal("expected node_id in runtime state")
	}
	if st.Runtime.SessionJournal != filepath.ToSlash(filepath.Join(".ax", "logs", "runtime-journal.jsonl")) {
		t.Fatalf("unexpected runtime journal path: %s", st.Runtime.SessionJournal)
	}
	meta, ok := st.Runtime.SessionMeta["sess-meta"]
	if !ok {
		t.Fatalf("expected runtime session metadata for sess-meta, got %+v", st.Runtime.SessionMeta)
	}
	if meta.Command != "run" {
		t.Fatalf("expected command=run in session metadata, got %+v", meta)
	}
	if meta.Mode != core.RuntimeModeShared {
		t.Fatalf("expected shared mode in session metadata, got %+v", meta)
	}
	if meta.Status != "active" {
		t.Fatalf("expected active status in session metadata, got %+v", meta)
	}

	journal := mustRead(t, filepath.Join(tmp, ".ax", "logs", "runtime-journal.jsonl"))
	for _, want := range []string{`"command":"run"`, `"session_id":"sess-meta"`, `"stage":"start"`, `"stage":"checkpoint"`, `"stage":"completed"`} {
		if !strings.Contains(journal, want) {
			t.Fatalf("runtime journal missing %q:\n%s", want, journal)
		}
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

func TestRecoverCommandUpdatesState(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "propose", "Recover Flow"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	planName := singleEntryName(t, filepath.Join(tmp, ".ax", "plans"))
	if _, err := executeAX(t, "run", "--plan", planName, "--decision", "reject"); err == nil {
		t.Fatal("expected reject run to fail")
	}
	if _, err := executeAX(t, "recover", "--strategy", "rerun"); err != nil {
		t.Fatalf("recover: %v", err)
	}
	st := readState(t, tmp)
	if st.Phase != core.PhaseImplementation {
		t.Fatalf("expected implementation phase after recover, got %s", st.Phase)
	}
	if st.LastResult.Command != "recover" {
		t.Fatalf("expected last_result command recover, got %+v", st.LastResult)
	}
	if st.LastError != nil {
		t.Fatalf("expected last error cleared, got %+v", st.LastError)
	}
}

func TestRecoverAutoSelectsResumeWhenInterrupted(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "propose", "Recover Auto Resume"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	planName := singleEntryName(t, filepath.Join(tmp, ".ax", "plans"))
	if _, err := executeAX(t, "run", "--plan", planName, "--tdd"); err != nil {
		t.Fatalf("run --tdd: %v", err)
	}

	st := readState(t, tmp)
	st.Run.LastFailedStep = "run:in-progress"
	st.Run.LastFailureCause = "interrupted-or-crash"
	if err := core.SaveState(tmp, st); err != nil {
		t.Fatalf("save state: %v", err)
	}

	out, err := executeAX(t, "recover", "--strategy", "auto")
	if err != nil {
		t.Fatalf("recover auto: %v", err)
	}
	if !strings.Contains(out, "strategy=resume") {
		t.Fatalf("expected auto recover to choose resume, got: %s", out)
	}
}

func TestRecoverAutoFallsBackToRerunWithoutResumeState(t *testing.T) {
	setupTempCWD(t)

	out, err := executeAX(t, "recover", "--strategy", "auto")
	if err != nil {
		t.Fatalf("recover auto: %v", err)
	}
	if !strings.Contains(out, "strategy=rerun") {
		t.Fatalf("expected auto recover to choose rerun, got: %s", out)
	}
}

func TestRecoverAutoResolvesToRerunWhenJournalShowsInterruptedRun(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "--runtime-mode", "shared", "--session-id", "sess-recover", "propose", "Recover Journal"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "--runtime-mode", "shared", "--session-id", "sess-recover", "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	st := readState(t, tmp)
	st.TDD = core.TDDState{}
	st.Run.LastFailedStep = ""
	st.Run.LastFailureCause = ""
	if err := core.SaveState(tmp, st); err != nil {
		t.Fatalf("save state: %v", err)
	}
	journalPath := filepath.Join(tmp, ".ax", "logs", "runtime-journal.jsonl")
	entry := `{"time":"2026-02-26T11:00:00Z","session_id":"sess-recover","command":"run","stage":"checkpoint","mode":"shared","phase":"implementation"}` + "\n"
	if err := os.WriteFile(journalPath, []byte(entry), 0o644); err != nil {
		t.Fatalf("write journal: %v", err)
	}

	out, err := executeAX(t, "--runtime-mode", "shared", "--session-id", "sess-recover", "recover", "--strategy", "auto")
	if err != nil {
		t.Fatalf("recover auto: %v", err)
	}
	if !strings.Contains(out, "strategy=rerun") {
		t.Fatalf("expected auto strategy to resolve to rerun, got: %s", out)
	}
	st = readState(t, tmp)
	if st.LastResult.Command != "recover" || st.LastResult.Status != "rerun" {
		t.Fatalf("expected recover rerun last result, got %+v", st.LastResult)
	}
}

func TestDoctorRuntimeOutputsDiagnostics(t *testing.T) {
	setupTempCWD(t)
	if _, err := executeAX(t, "state"); err != nil {
		t.Fatalf("state: %v", err)
	}
	out, err := executeAX(t, "doctor", "runtime")
	if err != nil {
		t.Fatalf("doctor runtime: %v", err)
	}
	for _, field := range []string{"phase:", "runtime mode:", "session id:", "active sessions:", "lock files:", "state file:"} {
		if !strings.Contains(out, field) {
			t.Fatalf("doctor runtime output missing %q: %s", field, out)
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

	runName := latestEntryName(t, filepath.Join(tmp, ".ax", "runs"))
	runBody := mustRead(t, filepath.Join(tmp, ".ax", "runs", runName))
	if !strings.Contains(runBody, "- resumed_from: ") {
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

func TestRunDecisionRejectPropagatesInterruptFailure(t *testing.T) {
	tmp := setupTempCWD(t)

	if _, err := executeAX(t, "propose", "Reject Interrupt Failure"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	planName := singleEntryName(t, filepath.Join(tmp, ".ax", "plans"))

	adapter := interruptFailingAdapter{AppServerAdapter: codex.NewScaffoldAdapter()}
	if _, err := adapter.InterruptTurn(context.Background(), "th", "tu"); err == nil {
		t.Fatal("test adapter should fail interrupt")
	}
	_, _, err := runPlan(tmp, planName, runOptions{
		decision: "reject",
		approval: "never",
		retry:    -1,
	}, runtimeContext{
		mode:      core.RuntimeModeSingle,
		sessionID: "sess-reject-interrupt-fail",
		clusterID: "cluster-test",
		nodeID:    "node-test",
	}, adapter, codex.ClientConfig{
		Mode:    "real",
		Timeout: 5 * time.Second,
		Retries: 0,
	}, time.Now())
	if err == nil {
		t.Fatal("expected reject interrupt failure to be surfaced")
	}
	if !strings.Contains(err.Error(), "reject interrupt failed") {
		t.Fatalf("expected reject interrupt failure message, got: %v", err)
	}

	state := readState(t, tmp)
	if state.LastError == nil || state.LastError.Code == "" {
		t.Fatalf("expected mapped last error, got %+v", state.LastError)
	}
	if state.Run.LastFailedStep != "decision:reject:interrupt" {
		t.Fatalf("expected interrupt failed step tracking, got %+v", state.Run)
	}

	logEntries, err := os.ReadDir(filepath.Join(tmp, ".ax", "logs"))
	if err != nil {
		t.Fatalf("read logs: %v", err)
	}
	foundObs := false
	for _, entry := range logEntries {
		body := mustRead(t, filepath.Join(tmp, ".ax", "logs", entry.Name()))
		if strings.Contains(body, "step: run_reject_interrupt_failed") {
			foundObs = true
			break
		}
	}
	if !foundObs {
		t.Fatal("expected run_reject_interrupt_failed observability log entry")
	}

	journal := mustRead(t, filepath.Join(tmp, ".ax", "logs", "runtime-journal.jsonl"))
	if !strings.Contains(journal, "\"step\":\"decision:reject:interrupt\"") {
		t.Fatalf("expected runtime journal interrupt failure step, got:\n%s", journal)
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

func TestRunWorktreeRuntimeModeCreatesSessionIsolatedWorktree(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "propose", "Worktree Runtime Mode"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	planName := singleEntryName(t, filepath.Join(tmp, ".ax", "plans"))

	if _, err := executeAX(t, "--runtime-mode", "worktree", "--session-id", "sess-test-a", "run", "--plan", planName); err != nil {
		t.Fatalf("run worktree mode: %v", err)
	}

	metaPath := filepath.Join(tmp, ".ax", "worktrees", proposalID, "sess-test-a", "worktree.yaml")
	mustExist(t, metaPath)
	meta := mustRead(t, metaPath)
	for _, field := range []string{"session_id: sess-test-a", "runtime_mode: worktree", "status: prepared"} {
		if !strings.Contains(meta, field) {
			t.Fatalf("worktree metadata missing %q:\n%s", field, meta)
		}
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

func TestVerifyWritesJSONContract(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "propose", "Verify JSON Contract"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "verify", "--proposal", proposalID, "--tests", "pass", "--build", "pass", "--ac", "pass"); err != nil {
		t.Fatalf("verify: %v", err)
	}

	verifyJSONPath := filepath.Join(tmp, ".ax", "proposals", proposalID, "verify.json")
	mustExist(t, verifyJSONPath)
	body := mustRead(t, verifyJSONPath)
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("unmarshal verify.json: %v\n%s", err, body)
	}
	if got := fmt.Sprint(payload["verdict"]); got != "PASS" {
		t.Fatalf("expected verdict PASS, got %q", got)
	}
	evidence, ok := payload["evidence"].(map[string]any)
	if !ok {
		t.Fatalf("expected evidence map, got %#v", payload["evidence"])
	}
	for _, key := range []string{"tests", "build", "criteria"} {
		if _, exists := evidence[key]; !exists {
			t.Fatalf("verify.json evidence missing %q", key)
		}
	}
	if failed, ok := payload["failed_checks"].([]any); !ok || len(failed) != 0 {
		t.Fatalf("expected empty failed_checks for PASS, got %#v", payload["failed_checks"])
	}
}

func TestStateLocksAndJournalFlags(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "propose", "State Flags"); err != nil {
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

	locksOut, err := executeAX(t, "state", "--locks")
	if err != nil {
		t.Fatalf("state --locks: %v", err)
	}
	if !strings.Contains(locksOut, "locks:") || !strings.Contains(locksOut, "state") {
		t.Fatalf("unexpected state --locks output:\n%s", locksOut)
	}

	locksJSONOut, err := executeAX(t, "state", "--locks", "--json")
	if err != nil {
		t.Fatalf("state --locks --json: %v", err)
	}
	locksPayload := map[string]any{}
	if err := json.Unmarshal([]byte(locksJSONOut), &locksPayload); err != nil {
		t.Fatalf("unmarshal locks json: %v\n%s", err, locksJSONOut)
	}
	if _, ok := locksPayload["count"]; !ok {
		t.Fatalf("expected count in locks payload: %#v", locksPayload)
	}

	journalOut, err := executeAX(t, "state", "--journal")
	if err != nil {
		t.Fatalf("state --journal: %v", err)
	}
	if !strings.Contains(journalOut, "runtime_journal:") {
		t.Fatalf("unexpected state --journal output:\n%s", journalOut)
	}

	journalJSONOut, err := executeAX(t, "state", "--journal", "--json")
	if err != nil {
		t.Fatalf("state --journal --json: %v", err)
	}
	journalPayload := map[string]any{}
	if err := json.Unmarshal([]byte(journalJSONOut), &journalPayload); err != nil {
		t.Fatalf("unmarshal journal json: %v\n%s", err, journalJSONOut)
	}
	if _, ok := journalPayload["entries"]; !ok {
		t.Fatalf("expected entries in journal payload: %#v", journalPayload)
	}
}

func TestRunRetryPolicyBlocksAfterExceededRetries(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "propose", "Retry Policy"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	planName := singleEntryName(t, filepath.Join(tmp, ".ax", "plans"))

	if _, err := executeAX(t, "run", "--plan", planName, "--decision", "reject", "--retry", "0"); err == nil {
		t.Fatal("expected run reject with retry=0 to fail")
	}

	st := readState(t, tmp)
	if !st.Run.Blocked {
		t.Fatalf("expected blocked=true after retry exceed, got %+v", st.Run)
	}
	if st.Run.RetryAttempts == 0 || st.Run.RetryLimit != 0 {
		t.Fatalf("unexpected retry counters: %+v", st.Run)
	}
	if st.Run.ErrorCode == "" || st.Run.ErrorSummary == "" || st.Run.RecoverHint == "" {
		t.Fatalf("expected run error metadata fields, got %+v", st.Run)
	}

	if _, err := executeAX(t, "run", "--plan", planName, "--retry", "0"); err == nil {
		t.Fatal("expected blocked run to fail immediately")
	}
}

func TestRunRetryOverrideValidation(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "propose", "Retry Override Range"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	planName := singleEntryName(t, filepath.Join(tmp, ".ax", "plans"))

	if _, err := executeAX(t, "run", "--plan", planName, "--retry", "6"); err == nil {
		t.Fatal("expected --retry out of range to fail")
	}
}

func TestRunRejectsPlanPathOutsideBase(t *testing.T) {
	tmp := setupTempCWD(t)

	outside := t.TempDir()
	escapePlanPath := filepath.Join(outside, "escape-plan.md")
	if err := os.WriteFile(escapePlanPath, []byte("# Plan\n\n- [ ] escape\n"), 0o644); err != nil {
		t.Fatalf("write escape plan: %v", err)
	}
	relEscapePath, err := filepath.Rel(tmp, escapePlanPath)
	if err != nil {
		t.Fatalf("relative escape path: %v", err)
	}

	if _, err := executeAX(t, "run", "--plan", relEscapePath); err == nil {
		t.Fatal("expected run to reject out-of-scope plan path")
	} else if !strings.Contains(err.Error(), "AX_INPUT_PATH_OUT_OF_SCOPE") {
		t.Fatalf("expected AX_INPUT_PATH_OUT_OF_SCOPE, got: %v", err)
	}
}

func TestRunFailsOnInvalidCodexMode(t *testing.T) {
	tmp := setupTempCWD(t)
	t.Setenv("AX_CODEX_MODE", "invalid-mode")
	if _, err := executeAX(t, "propose", "Invalid Codex Mode"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	planName := singleEntryName(t, filepath.Join(tmp, ".ax", "plans"))

	if _, err := executeAX(t, "run", "--plan", planName); err == nil {
		t.Fatal("expected run to fail on invalid AX_CODEX_MODE")
	} else if !strings.Contains(err.Error(), "AX_ENGINE_CONFIG_INVALID") {
		t.Fatalf("expected AX_ENGINE_CONFIG_INVALID, got: %v", err)
	}
}

func TestTUISnapshotAndGuardedAction(t *testing.T) {
	setupTempCWD(t)
	if _, err := executeAX(t, "state"); err != nil {
		t.Fatalf("state init: %v", err)
	}

	out, err := executeAX(t, "tui", "--snapshot", "--format", "json")
	if err != nil {
		t.Fatalf("tui snapshot json: %v", err)
	}
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("unmarshal tui snapshot json: %v\n%s", err, out)
	}
	for _, key := range []string{"generated_at", "screen_a_dashboard", "screen_b_runs", "screen_c_verify", "screen_d_archive", "screen_e_engine"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("tui snapshot missing key %q", key)
		}
	}
	keyHelp, ok := payload["key_binding_help"].([]any)
	if !ok || len(keyHelp) == 0 {
		t.Fatalf("expected non-empty key_binding_help, got %#v", payload["key_binding_help"])
	}

	if _, err := executeAX(t, "tui", "--action", "retry"); err == nil {
		t.Fatal("expected tui action without --confirm to fail")
	}
	actionOut, err := executeAX(t, "tui", "--action", "retry", "--confirm")
	if err != nil {
		t.Fatalf("tui guarded action: %v", err)
	}
	if !strings.Contains(actionOut, "tui: action=retry confirmed") {
		t.Fatalf("unexpected tui action output: %s", actionOut)
	}
}

func TestRunRealModeStreamingSummaryAndJournal(t *testing.T) {
	tmp := setupTempCWD(t)
	t.Setenv("AX_CODEX_MODE", "real")
	t.Setenv("AX_CODEX_BIN", os.Args[0])
	t.Setenv("AX_CODEX_ARGS", "-test.run=TestHelperProcessCodexServerAX")
	t.Setenv("AX_CODEX_TIMEOUT", "5s")
	t.Setenv("AX_CODEX_RETRIES", "0")
	t.Setenv("GO_WANT_HELPER_PROCESS_AX", "1")

	if _, err := executeAX(t, "propose", "Streaming Summary"); err != nil {
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

	runName := singleEntryName(t, filepath.Join(tmp, ".ax", "runs"))
	runBody := mustRead(t, filepath.Join(tmp, ".ax", "runs", runName))
	for _, want := range []string{
		"- stream_delta_events: 1",
		"- stream_completed_events: 1",
		"## Streaming Summary",
		"- delta_events: 1",
		"- completed_events: 1",
	} {
		if !strings.Contains(runBody, want) {
			t.Fatalf("run report missing %q:\n%s", want, runBody)
		}
	}

	journal := mustRead(t, filepath.Join(tmp, ".ax", "logs", "runtime-journal.jsonl"))
	if !strings.Contains(journal, "\"stage\":\"stream_delta\"") {
		t.Fatalf("expected stream_delta runtime journal entry, got:\n%s", journal)
	}
	if !strings.Contains(journal, "\"stage\":\"stream_completed\"") {
		t.Fatalf("expected stream_completed runtime journal entry, got:\n%s", journal)
	}
}

func TestTUIActionSteerUpdatesStateWithEngineCall(t *testing.T) {
	tmp := setupTempCWD(t)
	t.Setenv("AX_CODEX_MODE", "real")
	t.Setenv("AX_CODEX_BIN", os.Args[0])
	t.Setenv("AX_CODEX_ARGS", "-test.run=TestHelperProcessCodexServerAX")
	t.Setenv("AX_CODEX_TIMEOUT", "5s")
	t.Setenv("AX_CODEX_RETRIES", "0")
	t.Setenv("GO_WANT_HELPER_PROCESS_AX", "1")

	if _, err := executeAX(t, "propose", "TUI Steer Sync"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	planName := singleEntryName(t, filepath.Join(tmp, ".ax", "plans"))
	if _, err := executeAX(t, "run", "--plan", planName, "--tdd"); err != nil {
		t.Fatalf("run: %v", err)
	}

	before := readState(t, tmp)
	beforeLen := len(before.Run.TurnHistory)
	if beforeLen == 0 {
		t.Fatalf("expected existing turn history before steer action")
	}

	if _, err := executeAX(t, "tui", "--action", "steer", "--confirm"); err != nil {
		t.Fatalf("tui steer action: %v", err)
	}

	after := readState(t, tmp)
	if len(after.Run.TurnHistory) != beforeLen+1 {
		t.Fatalf("expected steer action to append history (%d -> %d), got %+v", beforeLen, beforeLen+1, after.Run.TurnHistory)
	}
	last := after.Run.TurnHistory[len(after.Run.TurnHistory)-1]
	if last.Step != "tui:steered" || last.Status != "steered" {
		t.Fatalf("expected steered turn ref, got %+v", last)
	}
	if after.Run.ActiveTurnID != "" {
		t.Fatalf("expected active turn cleared after steer action, got %q", after.Run.ActiveTurnID)
	}

	journal := mustRead(t, filepath.Join(tmp, ".ax", "logs", "runtime-journal.jsonl"))
	if !strings.Contains(journal, "\"command\":\"tui\"") || !strings.Contains(journal, "\"artifact\":\"steer\"") {
		t.Fatalf("expected tui steer journal entry, got:\n%s", journal)
	}
}

func TestTUIActionInterruptFailureSetsMappedError(t *testing.T) {
	tmp := setupTempCWD(t)
	t.Setenv("AX_CODEX_MODE", "real")
	t.Setenv("AX_CODEX_BIN", os.Args[0])
	t.Setenv("AX_CODEX_ARGS", "-test.run=TestHelperProcessCodexServerAX")
	t.Setenv("AX_CODEX_TIMEOUT", "5s")
	t.Setenv("AX_CODEX_RETRIES", "0")
	t.Setenv("AX_TEST_INTERRUPT_FAIL", "1")
	t.Setenv("GO_WANT_HELPER_PROCESS_AX", "1")

	if _, err := executeAX(t, "propose", "TUI Interrupt Failure"); err != nil {
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

	st := readState(t, tmp)
	if len(st.Run.TurnHistory) == 0 {
		t.Fatalf("expected turn history for interrupt action")
	}
	st.Run.ActiveTurnID = st.Run.TurnHistory[len(st.Run.TurnHistory)-1].TurnID
	if err := core.SaveState(tmp, st); err != nil {
		t.Fatalf("save state: %v", err)
	}

	if _, err := executeAX(t, "tui", "--action", "interrupt", "--confirm"); err == nil {
		t.Fatal("expected interrupt action to fail")
	} else if !strings.Contains(err.Error(), "interrupt failed") {
		t.Fatalf("expected interrupt failure message, got %v", err)
	}

	after := readState(t, tmp)
	if after.LastError == nil || after.LastError.Code == "" {
		t.Fatalf("expected mapped last error, got %+v", after.LastError)
	}
	if after.Run.ErrorCode == "" || after.Run.ErrorSummary == "" {
		t.Fatalf("expected run error metadata populated, got %+v", after.Run)
	}

	journal := mustRead(t, filepath.Join(tmp, ".ax", "logs", "runtime-journal.jsonl"))
	if !strings.Contains(journal, "\"stage\":\"action_failed\"") || !strings.Contains(journal, "\"artifact\":\"interrupt\"") {
		t.Fatalf("expected tui interrupt failure journal entry, got:\n%s", journal)
	}
}

func TestTUIActionForkAndRollbackSynchronizeEngineState(t *testing.T) {
	tmp := setupTempCWD(t)
	t.Setenv("AX_CODEX_MODE", "real")
	t.Setenv("AX_CODEX_BIN", os.Args[0])
	t.Setenv("AX_CODEX_ARGS", "-test.run=TestHelperProcessCodexServerAX")
	t.Setenv("AX_CODEX_TIMEOUT", "5s")
	t.Setenv("AX_CODEX_RETRIES", "0")
	t.Setenv("GO_WANT_HELPER_PROCESS_AX", "1")

	if _, err := executeAX(t, "propose", "TUI Fork Rollback"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	planName := singleEntryName(t, filepath.Join(tmp, ".ax", "plans"))
	if _, err := executeAX(t, "run", "--plan", planName, "--tdd"); err != nil {
		t.Fatalf("run: %v", err)
	}

	beforeFork := readState(t, tmp)
	originalThreadID := beforeFork.Run.ThreadID
	if originalThreadID == "" {
		t.Fatalf("expected thread id before fork")
	}
	if _, err := executeAX(t, "tui", "--action", "fork", "--confirm"); err != nil {
		t.Fatalf("tui fork action: %v", err)
	}
	afterFork := readState(t, tmp)
	if afterFork.Run.ThreadID == originalThreadID {
		t.Fatalf("expected fork to update thread id (%q -> %q)", originalThreadID, afterFork.Run.ThreadID)
	}

	if len(afterFork.Run.TurnHistory) < 2 {
		t.Fatalf("expected 2+ turns before rollback trim, got %+v", afterFork.Run.TurnHistory)
	}
	targetTurnID := afterFork.Run.TurnHistory[0].TurnID
	afterFork.Run.ActiveTurnID = targetTurnID
	if err := core.SaveState(tmp, afterFork); err != nil {
		t.Fatalf("save state before rollback: %v", err)
	}

	if _, err := executeAX(t, "tui", "--action", "rollback", "--confirm"); err != nil {
		t.Fatalf("tui rollback action: %v", err)
	}
	afterRollback := readState(t, tmp)
	if len(afterRollback.Run.TurnHistory) != 1 {
		t.Fatalf("expected rollback to trim turn history to 1, got %+v", afterRollback.Run.TurnHistory)
	}
	if afterRollback.Run.TurnHistory[0].TurnID != targetTurnID {
		t.Fatalf("expected rollback to keep target turn %q, got %+v", targetTurnID, afterRollback.Run.TurnHistory)
	}

	journal := mustRead(t, filepath.Join(tmp, ".ax", "logs", "runtime-journal.jsonl"))
	if !strings.Contains(journal, "\"artifact\":\"fork\"") || !strings.Contains(journal, "\"artifact\":\"rollback\"") {
		t.Fatalf("expected fork/rollback journal entries, got:\n%s", journal)
	}
}

func TestRunScaffoldPersistsThreadAndTurnHistory(t *testing.T) {
	tmp := setupTempCWD(t)
	t.Setenv("AX_CODEX_MODE", "scaffold")
	if _, err := executeAX(t, "propose", "Codex Scaffold"); err != nil {
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
	st := readState(t, tmp)
	if !strings.HasPrefix(st.Run.ThreadID, "scf-th-") {
		t.Fatalf("expected scaffold thread id, got %q", st.Run.ThreadID)
	}
	if len(st.Run.TurnHistory) == 0 {
		t.Fatalf("expected turn history to be populated")
	}
	if st.Run.ActiveTurnID != "" {
		t.Fatalf("expected active turn cleared, got %q", st.Run.ActiveTurnID)
	}
	if st.Run.EngineMode != "scaffold" {
		t.Fatalf("expected engine mode scaffold, got %q", st.Run.EngineMode)
	}
}

func TestRecoverResumeRequiresThreadID(t *testing.T) {
	tmp := setupTempCWD(t)
	if _, err := executeAX(t, "state"); err != nil {
		t.Fatalf("state: %v", err)
	}
	st := readState(t, tmp)
	st.Run.LastFailedStep = "run:in-progress"
	st.Run.LastFailureCause = "interrupted-or-crash"
	st.Run.ThreadID = ""
	if err := core.SaveState(tmp, st); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := executeAX(t, "recover", "--strategy", "resume"); err == nil {
		t.Fatal("expected resume without thread_id to fail")
	}
}

func TestDoctorRuntimeJSONIncludesCodexFields(t *testing.T) {
	tmp := setupTempCWD(t)
	t.Setenv("AX_CODEX_MODE", "scaffold")
	if _, err := executeAX(t, "propose", "Doctor Codex Fields"); err != nil {
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

	out, err := executeAX(t, "doctor", "runtime", "--json")
	if err != nil {
		t.Fatalf("doctor runtime json: %v", err)
	}
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("unmarshal doctor json: %v\n%s", err, out)
	}
	for _, key := range []string{"engine_mode", "thread_id", "active_turn_id", "turn_count", "codex_bin", "codex_reachable"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("missing doctor field %q in payload %#v", key, payload)
		}
	}
}

func TestRunRealModeAndRecoverAutoUsesThreadState(t *testing.T) {
	tmp := setupTempCWD(t)
	t.Setenv("AX_CODEX_MODE", "real")
	t.Setenv("AX_CODEX_BIN", os.Args[0])
	t.Setenv("AX_CODEX_ARGS", "-test.run=TestHelperProcessCodexServerAX")
	t.Setenv("AX_CODEX_TIMEOUT", "5s")
	t.Setenv("AX_CODEX_RETRIES", "0")
	t.Setenv("GO_WANT_HELPER_PROCESS_AX", "1")

	if _, err := executeAX(t, "propose", "Codex Real Mode"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	planName := singleEntryName(t, filepath.Join(tmp, ".ax", "plans"))
	if _, err := executeAX(t, "run", "--plan", planName, "--decision", "steer", "--steer", "safer"); err != nil {
		t.Fatalf("run real steer: %v", err)
	}
	st := readState(t, tmp)
	if st.Run.ThreadID != "th-real-1" {
		t.Fatalf("expected real thread id th-real-1, got %q", st.Run.ThreadID)
	}
	if len(st.Run.TurnHistory) == 0 {
		t.Fatal("expected turn history in real mode")
	}
	lastTurn := st.Run.TurnHistory[len(st.Run.TurnHistory)-1]
	if !strings.HasPrefix(lastTurn.TurnID, "tu-steer-") && !strings.HasPrefix(lastTurn.TurnID, "tu-stream-") {
		t.Fatalf("expected steer or streamed turn id, got %q", lastTurn.TurnID)
	}

	st.Run.LastFailedStep = "run:in-progress"
	st.Run.LastFailureCause = "interrupted-or-crash"
	if err := core.SaveState(tmp, st); err != nil {
		t.Fatalf("save state: %v", err)
	}
	out, err := executeAX(t, "recover", "--strategy", "auto")
	if err != nil {
		t.Fatalf("recover auto: %v", err)
	}
	if !strings.Contains(out, "strategy=resume") {
		t.Fatalf("expected auto recover to choose resume, got: %s", out)
	}
}

func TestRunRealModeFailsOnEmptyThreadID(t *testing.T) {
	tmp := setupTempCWD(t)
	t.Setenv("AX_CODEX_MODE", "real")
	t.Setenv("AX_CODEX_BIN", os.Args[0])
	t.Setenv("AX_CODEX_ARGS", "-test.run=TestHelperProcessCodexServerAX")
	t.Setenv("AX_CODEX_TIMEOUT", "5s")
	t.Setenv("AX_CODEX_RETRIES", "0")
	t.Setenv("AX_TEST_EMPTY_THREAD_ID", "1")
	t.Setenv("GO_WANT_HELPER_PROCESS_AX", "1")

	if _, err := executeAX(t, "propose", "Codex Empty Thread ID"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	planName := singleEntryName(t, filepath.Join(tmp, ".ax", "plans"))
	if _, err := executeAX(t, "run", "--plan", planName); err == nil {
		t.Fatal("expected run to fail when real mode returns empty thread id")
	}
	st := readState(t, tmp)
	if st.Run.ErrorCode != "AX_ENGINE_INVALID_RESPONSE" {
		t.Fatalf("expected AX_ENGINE_INVALID_RESPONSE, got %q", st.Run.ErrorCode)
	}
	if !strings.Contains(st.Run.ErrorSummary, "empty thread ID") {
		t.Fatalf("expected empty thread id summary, got %q", st.Run.ErrorSummary)
	}
}

func TestRunRealModeFailsOnEmptyTurnID(t *testing.T) {
	tmp := setupTempCWD(t)
	t.Setenv("AX_CODEX_MODE", "real")
	t.Setenv("AX_CODEX_BIN", os.Args[0])
	t.Setenv("AX_CODEX_ARGS", "-test.run=TestHelperProcessCodexServerAX")
	t.Setenv("AX_CODEX_TIMEOUT", "5s")
	t.Setenv("AX_CODEX_RETRIES", "0")
	t.Setenv("AX_TEST_EMPTY_TURN_ID", "1")
	t.Setenv("GO_WANT_HELPER_PROCESS_AX", "1")

	if _, err := executeAX(t, "propose", "Codex Empty Turn ID"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	planName := singleEntryName(t, filepath.Join(tmp, ".ax", "plans"))
	if _, err := executeAX(t, "run", "--plan", planName); err == nil {
		t.Fatal("expected run to fail when real mode returns empty turn id")
	}
	st := readState(t, tmp)
	if st.Run.ErrorCode != "AX_ENGINE_INVALID_RESPONSE" {
		t.Fatalf("expected AX_ENGINE_INVALID_RESPONSE, got %q", st.Run.ErrorCode)
	}
	if !strings.Contains(st.Run.ErrorSummary, "empty turn ID") {
		t.Fatalf("expected empty turn id summary, got %q", st.Run.ErrorSummary)
	}
	if len(st.Run.TurnHistory) != 0 {
		t.Fatalf("expected no successful turns persisted, got %+v", st.Run.TurnHistory)
	}
}

func TestRunPersistsTurnHistoryAfterEachTurn(t *testing.T) {
	tmp := setupTempCWD(t)
	t.Setenv("AX_CODEX_MODE", "real")
	t.Setenv("AX_CODEX_BIN", os.Args[0])
	t.Setenv("AX_CODEX_ARGS", "-test.run=TestHelperProcessCodexServerAX")
	t.Setenv("AX_CODEX_TIMEOUT", "10s")
	t.Setenv("AX_CODEX_RETRIES", "0")
	t.Setenv("AX_TEST_DELAY_REQ_ID", "3")
	t.Setenv("AX_TEST_DELAY_MILLIS", "3500")
	t.Setenv("GO_WANT_HELPER_PROCESS_AX", "1")

	if _, err := executeAX(t, "propose", "Codex Turn Persistence"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	planName := singleEntryName(t, filepath.Join(tmp, ".ax", "plans"))

	done := make(chan error, 1)
	go func() {
		root := NewRootCmd()
		out := &bytes.Buffer{}
		root.SetOut(out)
		root.SetErr(out)
		root.SetArgs([]string{"run", "--plan", planName})
		done <- root.Execute()
	}()

	persisted := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		st := readState(t, tmp)
		if len(st.Run.TurnHistory) > 0 {
			persisted = true
			break
		}
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("run exited early: %v", err)
			}
			t.Fatal("run completed before delayed step check")
		default:
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !persisted {
		st := readState(t, tmp)
		t.Fatalf("expected turn history to persist mid-run, got %+v", st.Run.TurnHistory)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run completion: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("timeout waiting for run completion")
	}
}

func TestRunRealModeUsesPerRPCTimeoutContext(t *testing.T) {
	tmp := setupTempCWD(t)
	t.Setenv("AX_CODEX_MODE", "real")
	t.Setenv("AX_CODEX_BIN", os.Args[0])
	t.Setenv("AX_CODEX_ARGS", "-test.run=TestHelperProcessCodexServerAX")
	t.Setenv("AX_CODEX_TIMEOUT", "2s")
	t.Setenv("AX_CODEX_RETRIES", "0")
	t.Setenv("AX_TEST_DELAY_ALL_TURN_MILLIS", "900")
	t.Setenv("GO_WANT_HELPER_PROCESS_AX", "1")

	if _, err := executeAX(t, "propose", "Codex Per-RPC Timeout"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalID := singleEntryName(t, filepath.Join(tmp, ".ax", "proposals"))
	if _, err := executeAX(t, "plan", "--from", proposalID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	planName := singleEntryName(t, filepath.Join(tmp, ".ax", "plans"))
	if _, err := executeAX(t, "run", "--plan", planName, "--tdd"); err != nil {
		t.Fatalf("run with per-rpc timeout should succeed: %v", err)
	}
	st := readState(t, tmp)
	if len(st.Run.TurnHistory) != 3 {
		t.Fatalf("expected 3 TDD turns, got %d (%+v)", len(st.Run.TurnHistory), st.Run.TurnHistory)
	}
	if st.Run.ErrorCode != "" {
		t.Fatalf("expected cleared run error after success, got %q", st.Run.ErrorCode)
	}
}

func TestHelperProcessCodexServerAX(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS_AX") != "1" {
		return
	}

	var req codex.JSONRPCRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		_ = json.NewEncoder(os.Stdout).Encode(codex.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      "0",
			Error:   &codex.JSONRPCErrorObj{Code: -32700, Message: "parse error"},
		})
		os.Exit(0)
	}
	write := func(v any) {
		_ = json.NewEncoder(os.Stdout).Encode(v)
	}
	emptyThreadID := os.Getenv("AX_TEST_EMPTY_THREAD_ID") == "1"
	emptyTurnID := os.Getenv("AX_TEST_EMPTY_TURN_ID") == "1"
	emptyTurnStep := strings.TrimSpace(os.Getenv("AX_TEST_EMPTY_TURN_STEP"))
	interruptFail := os.Getenv("AX_TEST_INTERRUPT_FAIL") == "1"
	delayStep := strings.TrimSpace(os.Getenv("AX_TEST_DELAY_STEP"))
	delayReqID := strings.TrimSpace(os.Getenv("AX_TEST_DELAY_REQ_ID"))
	delayMillis := 0
	if raw := strings.TrimSpace(os.Getenv("AX_TEST_DELAY_MILLIS")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			delayMillis = parsed
		}
	}
	delayAllTurnMillis := 0
	if raw := strings.TrimSpace(os.Getenv("AX_TEST_DELAY_ALL_TURN_MILLIS")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			delayAllTurnMillis = parsed
		}
	}
	shouldApplyToPrompt := func(prompt, marker string) bool {
		if marker == "" {
			return false
		}
		return strings.Contains(prompt, "step: "+marker)
	}

	switch req.Method {
	case "thread/start":
		threadID := "th-real-1"
		if emptyThreadID {
			threadID = ""
		}
		write(codex.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: codex.Thread{ID: threadID, Title: "real", CreatedAt: "2026-02-26T00:00:00Z"}})
	case "thread/resume":
		threadID := "th-real-1"
		if emptyThreadID {
			threadID = ""
		}
		write(codex.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: codex.Thread{ID: threadID, Title: "resumed", CreatedAt: "2026-02-26T00:00:01Z"}})
	case "thread/read":
		write(codex.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: codex.Thread{ID: "th-real-1", Title: "health", CreatedAt: "2026-02-26T00:00:02Z"}})
	case "thread/fork":
		write(codex.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: codex.Thread{ID: "th-real-fork-1", Title: "forked", CreatedAt: "2026-02-26T00:00:02Z"}})
	case "thread/rollback":
		params, _ := req.Params.(map[string]any)
		threadID, _ := params["thread_id"].(string)
		if strings.TrimSpace(threadID) == "" {
			threadID = "th-real-1"
		}
		write(codex.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: codex.Thread{ID: threadID, Title: "rolled-back", CreatedAt: "2026-02-26T00:00:02Z"}})
	case "turn/start":
		params, _ := req.Params.(map[string]any)
		prompt, _ := params["prompt"].(string)
		stream, _ := params["stream"].(bool)
		if delayAllTurnMillis > 0 {
			time.Sleep(time.Duration(delayAllTurnMillis) * time.Millisecond)
		}
		if delayMillis > 0 && ((delayReqID != "" && req.ID == delayReqID) || shouldApplyToPrompt(prompt, delayStep)) {
			time.Sleep(time.Duration(delayMillis) * time.Millisecond)
		}
		if stream {
			turnID := "tu-stream-" + req.ID
			if emptyTurnID {
				turnID = ""
			}
			write(codex.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: []codex.StreamEvent{
				{Type: codex.StreamEventDelta, ThreadID: "th-real-1", TurnID: turnID, Delta: "hello"},
				{Type: codex.StreamEventCompleted, ThreadID: "th-real-1", TurnID: turnID, Completed: true},
			}})
			break
		}
		turnID := "tu-run-" + req.ID
		if strings.Contains(prompt, "steer_instruction:") {
			turnID = "tu-steer-bootstrap-" + req.ID
		}
		if emptyTurnID || shouldApplyToPrompt(prompt, emptyTurnStep) {
			turnID = ""
		}
		write(codex.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: codex.Turn{ID: turnID, ThreadID: "th-real-1", Role: "assistant", Content: "ok", CreatedAt: "2026-02-26T00:00:03Z"}})
	case "review/start":
		turnID := "tu-steer-" + req.ID
		if emptyTurnID {
			turnID = ""
		}
		write(codex.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: codex.Turn{ID: turnID, ThreadID: "th-real-1", Role: "assistant", Content: "steered", CreatedAt: "2026-02-26T00:00:04Z"}})
	case "turn/interrupt":
		if interruptFail {
			write(codex.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &codex.JSONRPCErrorObj{Code: -32010, Message: "interrupt failed"}})
			break
		}
		write(codex.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"interrupted": true, "thread_id": "th-real-1"}})
	default:
		write(codex.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &codex.JSONRPCErrorObj{Code: -32601, Message: "method not found"}})
	}
	os.Exit(0)
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
