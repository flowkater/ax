package ax

import (
	"bytes"
	"os"
	"path/filepath"
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
	mustExist(t, filepath.Join(tmp, ".ax", "memory", "MEMORY.md"))
	mustExist(t, filepath.Join(tmp, ".ax", "memory", "gotchas.md"))
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
