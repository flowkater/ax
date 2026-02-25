package ax

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newProposeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "propose <title>",
		Short: "Create a new proposal from title",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			if err := ensureMVPLayout(wd); err != nil {
				return err
			}

			id := proposalID(args[0], time.Now())
			proposalDir := filepath.Join(wd, ".ax", "proposals", id)
			if err := os.MkdirAll(filepath.Join(proposalDir, "specs"), 0o755); err != nil {
				return err
			}

			proposalBody := fmt.Sprintf("# Proposal\n\n- id: %s\n- title: %s\n- created_at: %s\n", id, args[0], time.Now().Format(time.RFC3339))
			if err := os.WriteFile(filepath.Join(proposalDir, "proposal.md"), []byte(proposalBody), 0o644); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(proposalDir, "design.md"), []byte("# Design\n\nTBD\n"), 0o644); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(proposalDir, "tasks.md"), []byte("# Tasks\n\n- [ ] Define tasks\n"), 0o644); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "propose: created %s\n", id)
			return nil
		},
	}
	return cmd
}

func newPlanCmd() *cobra.Command {
	var from string
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Create plan from proposal",
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			if err := ensureMVPLayout(wd); err != nil {
				return err
			}

			planFile := filepath.Join(wd, ".ax", "plans", sanitizeToken(from)+"-plan.md")
			content := fmt.Sprintf("# Plan\n\n- from: %s\n- created_at: %s\n\n## Steps\n1. TBD\n", from, time.Now().Format(time.RFC3339))
			if err := os.WriteFile(planFile, []byte(content), 0o644); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "plan: created %s\n", filepath.Base(planFile))
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Proposal ID or path")
	_ = cmd.MarkFlagRequired("from")
	return cmd
}

func newRunCmd() *cobra.Command {
	var plan string
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run tasks from plan",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "run: plan=%s\n", plan)
			return nil
		},
	}
	cmd.Flags().StringVar(&plan, "plan", "", "Plan ID or path")
	_ = cmd.MarkFlagRequired("plan")
	return cmd
}

func newVerifyCmd() *cobra.Command {
	var proposal string
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify proposal against outputs",
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			if err := ensureMVPLayout(wd); err != nil {
				return err
			}

			proposalDir, proposalID, err := resolveProposal(wd, proposal)
			if err != nil {
				return err
			}
			report := fmt.Sprintf("# Verify Report\n\n- proposal: %s\n- verified_at: %s\n- status: pending-manual-review\n", proposalID, time.Now().Format(time.RFC3339))
			if err := os.WriteFile(filepath.Join(proposalDir, "verify.md"), []byte(report), 0o644); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "verify: created %s/verify.md\n", proposalID)
			return nil
		},
	}
	cmd.Flags().StringVar(&proposal, "proposal", "", "Proposal ID or path")
	_ = cmd.MarkFlagRequired("proposal")
	return cmd
}

func newArchiveCmd() *cobra.Command {
	var proposal string
	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Archive completed proposal",
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			if err := ensureMVPLayout(wd); err != nil {
				return err
			}

			proposalDir, proposalID, err := resolveProposal(wd, proposal)
			if err != nil {
				return err
			}
			archiveDir := filepath.Join(wd, ".ax", "archive", proposalID)
			if _, err := os.Stat(archiveDir); err == nil {
				return fmt.Errorf("archive target already exists: %s", archiveDir)
			}
			if err := os.Rename(proposalDir, archiveDir); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "archive: moved %s\n", proposalID)
			return nil
		},
	}
	cmd.Flags().StringVar(&proposal, "proposal", "", "Proposal ID or path")
	_ = cmd.MarkFlagRequired("proposal")
	return cmd
}

func newDiscoverCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "discover <topic>",
		Short: "Discover context for a topic",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "discover: %s\n", args[0])
			return nil
		},
	}
	return cmd
}

func newQuickCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "quick <task>",
		Short: "Run quick task flow",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "quick: %s\n", args[0])
			return nil
		},
	}
	return cmd
}

func newStateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "state",
		Short: "Show current ax state",
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			if err := ensureMVPLayout(wd); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "state: initialized .ax")
			return nil
		},
	}
	return cmd
}

func ensureMVPLayout(base string) error {
	dirs := []string{
		filepath.Join(base, ".ax"),
		filepath.Join(base, ".ax", "proposals"),
		filepath.Join(base, ".ax", "plans"),
		filepath.Join(base, ".ax", "archive"),
		filepath.Join(base, ".ax", "memory"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}

	statePath := filepath.Join(base, ".ax", "state.yaml")
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		if err := os.WriteFile(statePath, []byte("version: v2\nstatus: initialized\n"), 0o644); err != nil {
			return err
		}
	}

	memoryFiles := map[string]string{
		filepath.Join(base, ".ax", "memory", "MEMORY.md"):  "# MEMORY\n\n",
		filepath.Join(base, ".ax", "memory", "gotchas.md"): "# Gotchas\n\n",
	}
	for path, content := range memoryFiles {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return err
			}
		}
	}

	contextPolicyPath := filepath.Join(base, ".ax", "context-policy.md")
	if _, err := os.Stat(contextPolicyPath); os.IsNotExist(err) {
		contextPolicy := `# Context Policy

## Rule
- Capture only durable facts needed for future task execution.

## Why
- Prevent context bloat while preserving high-value operational knowledge.

## Enforcement
- Non-discoverable notes expire after TTL.
- Duplicate entries are merged by semantic key.
- Session-local scratch notes are excluded from memory index.

## Scope
- Applies to propose/plan/run/verify/archive workflow artifacts.
- Default TTL for non-discoverable context: 7d.
`
		if err := os.WriteFile(contextPolicyPath, []byte(contextPolicy), 0o644); err != nil {
			return err
		}
	}

	return nil
}

func proposalID(title string, now time.Time) string {
	ts := now.Format("20060102-150405")
	return ts + "-" + sanitizeToken(title)
}

func resolveProposal(base, proposal string) (dir string, id string, err error) {
	if proposal == "" {
		return "", "", errors.New("proposal is required")
	}

	if filepath.IsAbs(proposal) || strings.Contains(proposal, string(os.PathSeparator)) {
		if _, err := os.Stat(proposal); err != nil {
			return "", "", err
		}
		return proposal, filepath.Base(proposal), nil
	}

	dir = filepath.Join(base, ".ax", "proposals", proposal)
	if _, err := os.Stat(dir); err != nil {
		return "", "", err
	}
	return dir, proposal, nil
}

var tokenSanitizer = regexp.MustCompile(`[^a-z0-9]+`)

func sanitizeToken(s string) string {
	n := strings.ToLower(strings.TrimSpace(s))
	n = tokenSanitizer.ReplaceAllString(n, "-")
	n = strings.Trim(n, "-")
	if n == "" {
		return "untitled"
	}
	return n
}
