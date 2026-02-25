package ax

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProposeCreatesProposalScaffold(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(prev) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"propose", "Build MVP"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute propose: %v", err)
	}

	base := filepath.Join(tmp, ".ax")
	mustExist(t, filepath.Join(base, "state.yaml"))
	mustExist(t, filepath.Join(base, "memory", "MEMORY.md"))

	entries, err := os.ReadDir(filepath.Join(base, "proposals"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 proposal dir, got %d", len(entries))
	}
	proposalDir := filepath.Join(base, "proposals", entries[0].Name())
	mustExist(t, filepath.Join(proposalDir, "proposal.md"))
	mustExist(t, filepath.Join(proposalDir, "design.md"))
	mustExist(t, filepath.Join(proposalDir, "tasks.md"))
	mustExist(t, filepath.Join(proposalDir, "specs"))
}

func TestPlanCreatesPlanFile(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(prev) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"plan", "--from", "p-001"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute plan: %v", err)
	}

	mustExist(t, filepath.Join(tmp, ".ax", "plans", "p-001-plan.md"))
}

func TestStateInitializesLayout(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(prev) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"state"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute state: %v", err)
	}

	mustExist(t, filepath.Join(tmp, ".ax", "state.yaml"))
	mustExist(t, filepath.Join(tmp, ".ax", "proposals"))
	mustExist(t, filepath.Join(tmp, ".ax", "plans"))
	mustExist(t, filepath.Join(tmp, ".ax", "archive"))
	mustExist(t, filepath.Join(tmp, ".ax", "runs"))
	mustExist(t, filepath.Join(tmp, ".ax", "discovery"))
	mustExist(t, filepath.Join(tmp, ".ax", "memory", "MEMORY.md"))
	mustExist(t, filepath.Join(tmp, ".ax", "memory", "gotchas.md"))
	mustExist(t, filepath.Join(tmp, ".ax", "context-policy.md"))
}

func TestVerifyCreatesReportUnderProposal(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(prev) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"propose", "Check Verify"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute propose: %v", err)
	}

	proposalEntries, err := os.ReadDir(filepath.Join(tmp, ".ax", "proposals"))
	if err != nil {
		t.Fatal(err)
	}
	if len(proposalEntries) != 1 {
		t.Fatalf("expected 1 proposal dir, got %d", len(proposalEntries))
	}
	proposalID := proposalEntries[0].Name()

	root = NewRootCmd()
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"verify", "--proposal", proposalID})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute verify: %v", err)
	}

	mustExist(t, filepath.Join(tmp, ".ax", "proposals", proposalID, "verify.md"))
}

func TestArchiveMovesProposalToArchive(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(prev) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"propose", "Archive Me"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute propose: %v", err)
	}

	proposalEntries, err := os.ReadDir(filepath.Join(tmp, ".ax", "proposals"))
	if err != nil {
		t.Fatal(err)
	}
	if len(proposalEntries) != 1 {
		t.Fatalf("expected 1 proposal dir, got %d", len(proposalEntries))
	}
	proposalID := proposalEntries[0].Name()

	root = NewRootCmd()
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"archive", "--proposal", proposalID})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute archive: %v", err)
	}

	mustNotExist(t, filepath.Join(tmp, ".ax", "proposals", proposalID))
	mustExist(t, filepath.Join(tmp, ".ax", "archive", proposalID))
}

func TestRunCreatesRunReport(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(prev) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"plan", "--from", "p-001"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute plan: %v", err)
	}

	root = NewRootCmd()
	out.Reset()
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"run", "--plan", "p-001-plan.md"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute run: %v", err)
	}
	if !strings.Contains(out.String(), "run: created") {
		t.Fatalf("unexpected output: %q", out.String())
	}

	runs, err := os.ReadDir(filepath.Join(tmp, ".ax", "runs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run report, got %d", len(runs))
	}
}

func TestDiscoverCreatesReport(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(prev) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tmp, "topic-notes.md"), []byte("this file includes context-policy details"), 0o644); err != nil {
		t.Fatal(err)
	}

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"discover", "context-policy"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute discover: %v", err)
	}
	if !strings.Contains(out.String(), "discover: created") {
		t.Fatalf("unexpected output: %q", out.String())
	}

	reports, err := os.ReadDir(filepath.Join(tmp, ".ax", "discovery"))
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 {
		t.Fatalf("expected 1 discovery report, got %d", len(reports))
	}
}

func TestQuickCreatesProposalPlanAndRun(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(prev) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"quick", "fix-tests"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute quick: %v", err)
	}
	if !strings.Contains(out.String(), "quick: proposal=") {
		t.Fatalf("unexpected output: %q", out.String())
	}

	entries, _ := os.ReadDir(filepath.Join(tmp, ".ax", "proposals"))
	if len(entries) != 1 {
		t.Fatalf("expected 1 proposal, got %d", len(entries))
	}
	plans, _ := os.ReadDir(filepath.Join(tmp, ".ax", "plans"))
	if len(plans) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(plans))
	}
	runs, _ := os.ReadDir(filepath.Join(tmp, ".ax", "runs"))
	if len(runs) != 1 {
		t.Fatalf("expected 1 run report, got %d", len(runs))
	}
}

func mustExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected path to exist %s: %v", path, err)
	}
}

func mustNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected path to not exist %s", path)
	}
}
