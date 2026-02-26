package ax

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/flowkater/ax/internal/core"
	"github.com/spf13/cobra"
)

const (
	defaultApprovalPolicy = "never"
	contextCompactionCode = "E_CONTEXT_COMPACTION"
	approvalRequiredCode  = "E_RUN_APPROVAL_REQUIRED"
)

type runOptions struct {
	tdd         bool
	loop        bool
	tier        string
	depth       string
	approval    string
	resume      bool
	decision    string
	decisionSet bool
	steerText   string
	reviewCount int
	force       bool
	forceReason string
	noWorktree  bool
}

type stepTurnMapping struct {
	Step   string
	TurnID string
	Tier   string
}

type runtimeContext struct {
	mode      core.RuntimeMode
	sessionID string
	clusterID string
	nodeID    string
}

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
			now := time.Now()
			rt, err := resolveRuntimeContext(cmd, now)
			if err != nil {
				return err
			}
			return core.WithStateLock(wd, func() error {
				if err := ensureMVPLayout(wd); err != nil {
					return err
				}
				id, err := createProposal(wd, args[0], rt, now)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "propose: created %s\n", id)
				return nil
			})
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
			now := time.Now()
			rt, err := resolveRuntimeContext(cmd, now)
			if err != nil {
				return err
			}
			return core.WithStateLock(wd, func() error {
				if err := ensureMVPLayout(wd); err != nil {
					return err
				}
				planFile, err := createPlan(wd, from, rt, now)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "plan: created %s\n", filepath.Base(planFile))
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Proposal ID or path")
	_ = cmd.MarkFlagRequired("from")
	return cmd
}

func newRunCmd() *cobra.Command {
	var (
		plan        string
		tdd         bool
		loop        bool
		tier        string
		depth       string
		approval    string
		resume      bool
		decision    string
		steerText   string
		reviewCount int
		force       bool
		forceReason string
		noWorktree  bool
	)
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run tasks from plan",
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			now := time.Now()
			rt, err := resolveRuntimeContext(cmd, now)
			if err != nil {
				return err
			}
			return core.WithStateLock(wd, func() error {
				if err := ensureMVPLayout(wd); err != nil {
					return err
				}

				runFile, taskCount, err := runPlan(wd, plan, runOptions{
					tdd:         tdd,
					loop:        loop,
					tier:        tier,
					depth:       depth,
					approval:    approval,
					resume:      resume,
					decision:    decision,
					decisionSet: cmd.Flags().Changed("decision"),
					steerText:   steerText,
					reviewCount: reviewCount,
					force:       force,
					forceReason: forceReason,
					noWorktree:  noWorktree,
				}, rt, now)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "run: created %s (tasks=%d)\n", filepath.Base(runFile), taskCount)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&plan, "plan", "", "Plan ID or path")
	cmd.Flags().BoolVar(&tdd, "tdd", false, "Enable TDD scaffolding")
	cmd.Flags().BoolVar(&loop, "loop", false, "Enable tier loop execution scaffold for --tdd")
	cmd.Flags().StringVar(&tier, "tier", "", "Target TDD tier (T0|T1|T2); progression gate enforced")
	cmd.Flags().StringVar(&depth, "depth", "", "Depth override (quick|normal|deep|explore)")
	cmd.Flags().StringVar(&approval, "approval-policy", defaultApprovalPolicy, "Approval policy (never|on-failure|unless-allow-listed|always)")
	cmd.Flags().StringVar(&approval, "approval", defaultApprovalPolicy, "Deprecated alias for --approval-policy")
	cmd.Flags().BoolVar(&resume, "resume", false, "Resume from previous failed step scaffolding")
	cmd.Flags().StringVar(&decision, "decision", "accept", "Run decision (accept|reject|steer)")
	cmd.Flags().StringVar(&steerText, "steer", "", "Steer instruction text when --decision=steer")
	cmd.Flags().IntVar(&reviewCount, "review-count", 0, "Current review attempt count for TDD quality gate")
	cmd.Flags().BoolVar(&force, "force", false, "Override quality gate (requires --force-reason)")
	cmd.Flags().StringVar(&forceReason, "force-reason", "", "Reason for --force")
	cmd.Flags().BoolVar(&noWorktree, "no-worktree", false, "Disable worktree scaffolding")
	_ = cmd.MarkFlagRequired("plan")
	return cmd
}

func newVerifyCmd() *cobra.Command {
	var (
		proposal string
		testsRaw string
		buildRaw string
		acRaw    string
	)
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify proposal against outputs",
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			now := time.Now()
			rt, err := resolveRuntimeContext(cmd, now)
			if err != nil {
				return err
			}
			return core.WithStateLock(wd, func() error {
				if err := ensureMVPLayout(wd); err != nil {
					return err
				}

				reportPath, proposalID, err := verifyProposal(wd, proposal, testsRaw, buildRaw, acRaw, rt, now)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "verify: created %s\n", filepath.ToSlash(filepath.Join(proposalID, filepath.Base(reportPath))))
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&proposal, "proposal", "", "Proposal ID or path")
	cmd.Flags().StringVar(&testsRaw, "tests", "", "Test evidence status (pass|fail|unknown)")
	cmd.Flags().StringVar(&buildRaw, "build", "", "Build evidence status (pass|fail|unknown)")
	cmd.Flags().StringVar(&acRaw, "ac", "", "Acceptance criteria status (pass|fail|unknown)")
	_ = cmd.MarkFlagRequired("proposal")
	return cmd
}

func newArchiveCmd() *cobra.Command {
	var (
		proposal               string
		allowUnverifiedArchive bool
	)
	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Archive completed proposal",
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			now := time.Now()
			rt, err := resolveRuntimeContext(cmd, now)
			if err != nil {
				return err
			}
			return core.WithStateLock(wd, func() error {
				if err := ensureMVPLayout(wd); err != nil {
					return err
				}

				proposalID, err := archiveProposal(wd, proposal, allowUnverifiedArchive, rt, now)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "archive: moved %s\n", proposalID)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&proposal, "proposal", "", "Proposal ID or path")
	cmd.Flags().BoolVar(&allowUnverifiedArchive, "allow-unverified-archive", false, "Allow archiving without verify.md")
	_ = cmd.MarkFlagRequired("proposal")
	return cmd
}

func newDiscoverCmd() *cobra.Command {
	var party bool
	cmd := &cobra.Command{
		Use:   "discover <topic>",
		Short: "Discover context for a topic",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			now := time.Now()
			rt, err := resolveRuntimeContext(cmd, now)
			if err != nil {
				return err
			}
			return core.WithStateLock(wd, func() error {
				if err := ensureMVPLayout(wd); err != nil {
					return err
				}

				report, matches, err := discoverTopic(wd, args[0], party, rt, now)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "discover: created %s (matches=%d)\n", filepath.Base(report), matches)
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&party, "party", false, "Include Architect/User/QA persona sections")
	return cmd
}

func newQuickCmd() *cobra.Command {
	var (
		filesChanged int
		crossModule  bool
		coreTouch    bool
	)
	cmd := &cobra.Command{
		Use:   "quick <task>",
		Short: "Run quick task flow",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			now := time.Now()
			rt, err := resolveRuntimeContext(cmd, now)
			if err != nil {
				return err
			}
			return core.WithStateLock(wd, func() error {
				if err := ensureMVPLayout(wd); err != nil {
					return err
				}

				if core.ShouldEscalateQuickStats(filesChanged, crossModule, coreTouch) {
					proposalID, err := createProposal(wd, args[0], rt, now)
					if err != nil {
						return err
					}
					planFile, err := createPlan(wd, proposalID, rt, now)
					if err != nil {
						return err
					}
					fmt.Fprintf(cmd.OutOrStdout(), "quick: escalated proposal=%s plan=%s\n", proposalID, filepath.Base(planFile))
					return nil
				}

				runFile, err := runQuick(wd, args[0], rt, now)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "quick: completed %s\n", filepath.Base(runFile))
				return nil
			})
		},
	}
	cmd.Flags().IntVar(&filesChanged, "files-changed", 0, "Estimated changed files for quick escalation")
	cmd.Flags().BoolVar(&crossModule, "cross-module", false, "Whether change crosses modules")
	cmd.Flags().BoolVar(&coreTouch, "core-touch", false, "Whether core state/codex/run areas are touched")
	return cmd
}

func newStateCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "state",
		Short: "Show current ax state",
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			return core.WithStateLock(wd, func() error {
				if err := ensureMVPLayout(wd); err != nil {
					return err
				}
				return printState(cmd, wd, jsonOut)
			})
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print machine readable state JSON")
	return cmd
}

func newRecoverCmd() *cobra.Command {
	var strategy string
	cmd := &cobra.Command{
		Use:   "recover",
		Short: "Recover runtime/session state after interruption",
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			now := time.Now()
			rt, err := resolveRuntimeContext(cmd, now)
			if err != nil {
				return err
			}
			return core.WithStateLock(wd, func() error {
				if err := ensureMVPLayout(wd); err != nil {
					return err
				}
				st, err := core.LoadState(wd)
				if err != nil {
					return err
				}
				applyRuntimeState(st, rt, "recover")
				requestedMode := strings.TrimSpace(strings.ToLower(strategy))
				switch requestedMode {
				case "", "auto":
					requestedMode = "auto"
				case "resume", "rerun":
				default:
					return fmt.Errorf("invalid recover strategy %q (use auto|resume|rerun)", strategy)
				}

				resolvedMode := requestedMode
				if requestedMode == "auto" {
					resolvedMode = "rerun"
					explicitResumeCandidate := strings.TrimSpace(st.Run.LastFailedStep) != "" || strings.TrimSpace(st.Run.LastFailureCause) != ""
					if explicitResumeCandidate {
						resolvedMode = "resume"
					} else {
						lastRunEntry, ok, journalErr := latestRuntimeJournalEntry(wd, st.Runtime.SessionID, "run")
						if journalErr != nil {
							return journalErr
						}
						if ok {
							switch lastRunEntry.Stage {
							case "start", "checkpoint", "failed":
								if st.TDD.Enabled {
									resolvedMode = "resume"
								} else {
									resolvedMode = "rerun"
								}
							case "completed":
								resolvedMode = "rerun"
							}
						}
					}
				}
				if resolvedMode == "resume" {
					if _, err := restoreRuntimeCheckpoint(wd, st); err != nil {
						return err
					}
					if !st.TDD.Enabled {
						return errors.New("recover resume unavailable: no persisted tdd state")
					}
				}
				_ = appendRuntimeJournal(wd, runtimeJournalEntry{
					Time:      now.Format(time.RFC3339),
					SessionID: st.Runtime.SessionID,
					ClusterID: st.Runtime.ClusterID,
					NodeID:    st.Runtime.NodeID,
					Command:   "recover",
					Stage:     "start",
					Mode:      string(st.Runtime.Mode),
					Phase:     string(st.Phase),
				})
				st.Run.LastFailureCause = ""
				if resolvedMode == "rerun" {
					st.Run.LastFailedStep = ""
				}
				st.ForcePhase(core.PhaseImplementation, "recover:"+resolvedMode, now)
				st.SetLastResult("recover", resolvedMode, st.Runtime.SessionJournal, now)
				st.ClearLastError()
				if err := st.Save(wd); err != nil {
					return err
				}
				_ = appendRuntimeJournal(wd, runtimeJournalEntry{
					Time:      time.Now().Format(time.RFC3339),
					SessionID: st.Runtime.SessionID,
					ClusterID: st.Runtime.ClusterID,
					NodeID:    st.Runtime.NodeID,
					Command:   "recover",
					Stage:     "completed",
					Mode:      string(st.Runtime.Mode),
					Phase:     string(st.Phase),
					Artifact:  st.Runtime.SessionJournal,
				})
				fmt.Fprintf(cmd.OutOrStdout(), "recover: strategy=%s session=%s phase=%s\n", resolvedMode, st.Runtime.SessionID, st.Phase)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&strategy, "strategy", "auto", "Recovery strategy (auto|resume|rerun)")
	return cmd
}

func newDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Runtime diagnostics",
	}

	var jsonOut bool
	runtimeCmd := &cobra.Command{
		Use:   "runtime",
		Short: "Inspect runtime lock/state health",
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			return core.WithStateLock(wd, func() error {
				if err := ensureMVPLayout(wd); err != nil {
					return err
				}
				st, err := core.LoadState(wd)
				if err != nil {
					return err
				}
				lockDir := filepath.Join(wd, ".ax", "locks")
				entries, _ := os.ReadDir(lockDir)
				payload := map[string]any{
					"phase":           st.Phase,
					"runtime_mode":    st.Runtime.Mode,
					"session_id":      st.Runtime.SessionID,
					"cluster_id":      st.Runtime.ClusterID,
					"node_id":         st.Runtime.NodeID,
					"journal_path":    st.Runtime.SessionJournal,
					"active_sessions": len(st.Runtime.ActiveSessions),
					"lock_files":      len(entries),
					"state_file":      filepath.ToSlash(filepath.Join(".ax", "state.yaml")),
				}
				if jsonOut {
					body, err := json.MarshalIndent(payload, "", "  ")
					if err != nil {
						return err
					}
					fmt.Fprintln(cmd.OutOrStdout(), string(body))
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "phase: %s\n", payload["phase"])
				fmt.Fprintf(cmd.OutOrStdout(), "runtime mode: %s\n", payload["runtime_mode"])
				fmt.Fprintf(cmd.OutOrStdout(), "session id: %s\n", emptyFallback(fmt.Sprint(payload["session_id"])))
				fmt.Fprintf(cmd.OutOrStdout(), "cluster id: %s\n", emptyFallback(fmt.Sprint(payload["cluster_id"])))
				fmt.Fprintf(cmd.OutOrStdout(), "node id: %s\n", emptyFallback(fmt.Sprint(payload["node_id"])))
				fmt.Fprintf(cmd.OutOrStdout(), "journal path: %s\n", emptyFallback(fmt.Sprint(payload["journal_path"])))
				fmt.Fprintf(cmd.OutOrStdout(), "active sessions: %d\n", payload["active_sessions"])
				fmt.Fprintf(cmd.OutOrStdout(), "lock files: %d\n", payload["lock_files"])
				fmt.Fprintf(cmd.OutOrStdout(), "state file: %s\n", payload["state_file"])
				return nil
			})
		},
	}
	runtimeCmd.Flags().BoolVar(&jsonOut, "json", false, "Print machine readable runtime diagnostics")
	cmd.AddCommand(runtimeCmd)
	return cmd
}

func newReviewCmd() *cobra.Command {
	var proposal string
	cmd := &cobra.Command{
		Use:   "review",
		Short: "Generate 8-lens review scaffolding",
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			now := time.Now()
			rt, err := resolveRuntimeContext(cmd, now)
			if err != nil {
				return err
			}
			return core.WithStateLock(wd, func() error {
				if err := ensureMVPLayout(wd); err != nil {
					return err
				}
				report, err := createReview(wd, proposal, rt, now)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "review: created %s\n", filepath.Base(report))
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&proposal, "proposal", "", "Proposal ID or path")
	_ = cmd.MarkFlagRequired("proposal")
	return cmd
}

func newCompoundCmd() *cobra.Command {
	var audit bool
	cmd := &cobra.Command{
		Use:   "compound",
		Short: "Generate compound triage/audit scaffolding",
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			now := time.Now()
			rt, err := resolveRuntimeContext(cmd, now)
			if err != nil {
				return err
			}
			return core.WithStateLock(wd, func() error {
				if err := ensureMVPLayout(wd); err != nil {
					return err
				}
				report, err := createCompound(wd, audit, rt, now)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "compound: created %s\n", filepath.Base(report))
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&audit, "audit", false, "Include gotcha decay audit placeholders")
	return cmd
}

func ensureMVPLayout(base string) error {
	dirs := []string{
		filepath.Join(base, ".ax"),
		filepath.Join(base, ".ax", "proposals"),
		filepath.Join(base, ".ax", "plans"),
		filepath.Join(base, ".ax", "archive"),
		filepath.Join(base, ".ax", "memory"),
		filepath.Join(base, ".ax", "runs"),
		filepath.Join(base, ".ax", "discovery"),
		filepath.Join(base, ".ax", "logs"),
		filepath.Join(base, ".ax", "reviews"),
		filepath.Join(base, ".ax", "compound"),
		filepath.Join(base, ".ax", "worktrees"),
		filepath.Join(base, ".ax", "runtime"),
		filepath.Join(base, ".ax", "runtime", "checkpoints"),
		filepath.Join(base, ".ax", "skills", "builtin"),
		filepath.Join(base, ".ax", "skills", "custom"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
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

	registryPath := filepath.Join(base, ".ax", "skills", "registry.yaml")
	if _, err := os.Stat(registryPath); os.IsNotExist(err) {
		if err := os.WriteFile(registryPath, []byte(core.DefaultSkillRegistryYAML()), 0o644); err != nil {
			return err
		}
	}

	st, err := core.LoadState(base)
	if err != nil {
		return err
	}
	return st.Save(base)
}

func createProposal(base, title string, rt runtimeContext, now time.Time) (string, error) {
	id := proposalID(title, now)
	proposalDir := filepath.Join(base, ".ax", "proposals", id)
	if err := os.MkdirAll(filepath.Join(proposalDir, "specs"), 0o755); err != nil {
		return "", err
	}

	proposalBody := strings.Join([]string{
		"# Proposal",
		"",
		fmt.Sprintf("- id: %s", id),
		fmt.Sprintf("- title: %s", title),
		fmt.Sprintf("- created_at: %s", now.Format(time.RFC3339)),
		"",
		"## Problem / Goal",
		fmt.Sprintf("- 문제: %s", title),
		"- 목표: 실행 가능한 구현 계획으로 연결",
		"",
		"## Scope / Out of Scope",
		"- Scope: cmd/ax, internal/codex, internal/core",
		"- Out of Scope: GUI/daemon/ws",
		"",
		"## Acceptance Criteria",
		"- [ ] 요구사항 섹션이 채워진다",
		"- [ ] 테스트/빌드 근거를 생성한다",
		"",
		"## Risks / Assumptions",
		"- 리스크: 요구사항 확장으로 인한 범위 증가",
		"- 가정: 로컬 테스트/빌드 환경이 정상 동작",
		"",
		"## Interview Notes",
		"- placeholder: scope boundary / edge case / priority 확인",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(proposalDir, "proposal.md"), []byte(proposalBody), 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(proposalDir, "design.md"), []byte("# Design\n\n## Architecture\n- TBD\n\n## Data Flow\n- TBD\n"), 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(proposalDir, "tasks.md"), []byte(defaultProposalTasks()), 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(proposalDir, "specs", "acceptance.md"), []byte("# Acceptance\n\n- [ ] Define acceptance checklist\n"), 0o644); err != nil {
		return "", err
	}

	st, err := core.LoadState(base)
	if err != nil {
		return "", err
	}
	applyRuntimeState(st, rt, "propose")
	if err := st.Transition(core.PhaseProposal, "propose", now); err != nil {
		return "", err
	}
	st.Current.Proposal = id
	_ = st.AddContext(filepath.ToSlash(filepath.Join(".ax", "proposals", id, "proposal.md")))
	_ = st.AddContext(filepath.ToSlash(filepath.Join(".ax", "proposals", id, "design.md")))
	_ = st.AddContext(filepath.ToSlash(filepath.Join(".ax", "proposals", id, "tasks.md")))
	st.SetLastResult("propose", "created", filepath.ToSlash(filepath.Join(".ax", "proposals", id)), now)
	st.ClearLastError()
	if err := st.Save(base); err != nil {
		return "", err
	}
	return id, nil
}

func defaultProposalTasks() string {
	lines := []string{"# Tasks", ""}
	for i := 1; i <= 10; i++ {
		lines = append(lines, fmt.Sprintf("- [ ] T-%02d - implementation step %d", i, i))
	}
	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

func createPlan(base, from string, rt runtimeContext, now time.Time) (string, error) {
	proposalDir, proposalID, err := resolveProposal(base, from)
	if err != nil {
		return "", fmt.Errorf("plan --from requires existing proposal: %w", err)
	}

	taskIDs, err := extractTaskIDs(filepath.Join(proposalDir, "tasks.md"))
	if err != nil {
		return "", err
	}
	if len(taskIDs) == 0 {
		taskIDs = []string{"T-01", "T-02", "T-03"}
	}

	planFile := filepath.Join(base, ".ax", "plans", sanitizeToken(proposalID)+"-plan.md")
	var b strings.Builder
	b.WriteString("# Plan\n\n")
	b.WriteString(fmt.Sprintf("- from: %s\n", proposalID))
	b.WriteString(fmt.Sprintf("- created_at: %s\n", now.Format(time.RFC3339)))
	b.WriteString("\n## Phase Plan\n")
	b.WriteString("- Phase 1: 계약/스키마 정렬\n")
	b.WriteString("- Phase 2: 구현 및 테스트\n")
	b.WriteString("- Phase 3: 검증/아카이브\n")
	b.WriteString("\n## File / Module Candidates\n")
	b.WriteString("- cmd/ax/*\n- internal/codex/*\n- internal/core/*\n")
	b.WriteString("\n## Test Strategy\n")
	b.WriteString("- go test ./...\n- go build ./...\n- command smoke tests\n")
	b.WriteString("\n## Rollback / Risk Response\n")
	b.WriteString("- 실패 시 마지막 성공 상태로 롤백\n- run --resume로 재진입\n")
	b.WriteString("\n## Task Mapping\n")
	b.WriteString("| Task ID | Plan Step |\n|---|---|\n")
	for i, id := range taskIDs {
		b.WriteString(fmt.Sprintf("| %s | Step-%02d |\n", id, i+1))
	}

	if err := os.WriteFile(planFile, []byte(b.String()), 0o644); err != nil {
		return "", err
	}

	st, err := core.LoadState(base)
	if err != nil {
		return "", err
	}
	applyRuntimeState(st, rt, "plan")
	if err := st.Transition(core.PhasePlanning, "plan", now); err != nil {
		return "", err
	}
	st.Current.Proposal = proposalID
	st.Current.Plan = filepath.Base(planFile)
	_ = st.AddContext(filepath.ToSlash(filepath.Join(".ax", "plans", filepath.Base(planFile))))
	st.SetLastResult("plan", "created", filepath.ToSlash(filepath.Join(".ax", "plans", filepath.Base(planFile))), now)
	st.ClearLastError()
	if err := st.Save(base); err != nil {
		return "", err
	}
	return planFile, nil
}

func runPlan(base, plan string, opts runOptions, rt runtimeContext, now time.Time) (string, int, error) {
	planPath, planID, err := resolvePlan(base, plan)
	if err != nil {
		return "", 0, err
	}

	if err := core.ValidateQualityGate(opts.reviewCount, opts.force, opts.forceReason); err != nil {
		return "", 0, err
	}
	decision := strings.ToLower(strings.TrimSpace(opts.decision))
	if decision == "" {
		decision = "accept"
	}
	switch decision {
	case "accept", "reject", "steer":
	default:
		return "", 0, fmt.Errorf("invalid decision: %s (use accept|reject|steer)", decision)
	}
	if decision == "steer" && strings.TrimSpace(opts.steerText) == "" {
		return "", 0, errors.New("--steer text is required when --decision=steer")
	}
	approvalPolicy, err := core.EvaluateApprovalPolicy(opts.approval, core.PhaseImplementation, decision, false)
	if err != nil {
		return "", 0, err
	}

	st, err := core.LoadState(base)
	if err != nil {
		return "", 0, err
	}
	applyRuntimeState(st, rt, "run")
	if err := st.Transition(core.PhaseImplementation, "run", now); err != nil {
		return "", 0, err
	}
	if err := appendRuntimeJournal(base, runtimeJournalEntry{
		Time:      now.Format(time.RFC3339),
		SessionID: st.Runtime.SessionID,
		ClusterID: st.Runtime.ClusterID,
		NodeID:    st.Runtime.NodeID,
		Command:   "run",
		Stage:     "start",
		Mode:      string(st.Runtime.Mode),
		Phase:     string(st.Phase),
		PlanID:    planID,
	}); err != nil {
		return "", 0, err
	}
	prevTDD := st.TDD

	runName := fmt.Sprintf("%s-run-%s.md", sanitizeToken(planID), now.Format("20060102-150405"))
	runPath := filepath.Join(base, ".ax", "runs", runName)
	startLog := filepath.Join(base, ".ax", "logs", fmt.Sprintf("%s-%s-start.yaml", sanitizeToken(planID), now.Format("20060102-150405")))
	endLog := filepath.Join(base, ".ax", "logs", fmt.Sprintf("%s-%s-end.yaml", sanitizeToken(planID), now.Format("20060102-150405")))
	failLog := filepath.Join(base, ".ax", "logs", fmt.Sprintf("%s-%s-fail.yaml", sanitizeToken(planID), now.Format("20060102-150405")))
	compactionLog := filepath.Join(base, ".ax", "logs", fmt.Sprintf("%s-%s-compaction.yaml", sanitizeToken(planID), now.Format("20060102-150405")))

	threadID := syntheticThreadID(st.Runtime.SessionID + "-" + planID)
	if err := writeObservabilityLog(startLog, "run_start", string(core.PhaseImplementation), threadID, "", "", "run scaffolding started", now); err != nil {
		return "", 0, err
	}

	// Early checkpoint for crash/kill recovery.
	st.Run.LastFailedStep = "run:in-progress"
	st.Run.LastFailureCause = "interrupted-or-crash"
	if opts.tdd && !opts.resume {
		st.TDD = core.TDDState{
			Enabled:        true,
			CurrentTier:    "T0",
			CurrentStep:    "checkpoint",
			CompletedSteps: 0,
			TotalSteps:     9,
			Depth:          strings.TrimSpace(opts.depth),
			DepthSource:    ternary(strings.TrimSpace(opts.depth) == "", "auto", "user"),
			ApprovalPolicy: opts.approval,
			Resume:         opts.resume,
			CompletedTiers: nil,
		}
	} else if !opts.tdd {
		st.TDD = core.TDDState{}
	}
	st.SetLastResult("run", "started", filepath.ToSlash(filepath.Join(".ax", "logs", filepath.Base(startLog))), now)
	if err := st.Save(base); err != nil {
		return "", 0, err
	}
	if err := writeRuntimeCheckpoint(base, st, "in_progress", "run", now); err != nil {
		return "", 0, err
	}
	if err := appendRuntimeJournal(base, runtimeJournalEntry{
		Time:      time.Now().Format(time.RFC3339),
		SessionID: st.Runtime.SessionID,
		ClusterID: st.Runtime.ClusterID,
		NodeID:    st.Runtime.NodeID,
		Command:   "run",
		Stage:     "checkpoint",
		Mode:      string(st.Runtime.Mode),
		Phase:     string(st.Phase),
		PlanID:    planID,
		Artifact:  filepath.ToSlash(filepath.Join(".ax", "state.yaml")),
	}); err != nil {
		return "", 0, err
	}

	body, err := os.ReadFile(planPath)
	if err != nil {
		return "", 0, err
	}
	if !strings.Contains(string(body), "# Plan") {
		return "", 0, errors.New("invalid plan format: missing # Plan")
	}

	affected := core.AnalyzeAffectedDirectories(string(body), planPath)
	depth, depthSource := core.ResolveDepth(string(body), affected, opts.depth)
	taskCount := countUncheckedTasks(string(body))
	if taskCount == 0 {
		taskCount = 1
	}
	tddState := core.TDDState{}
	tddProgress := "disabled"
	if opts.tdd {
		tddState, tddProgress, err = core.ResolveTDDProgression(prevTDD, opts.resume, string(depth), depthSource, opts.approval)
		if err != nil {
			return "", 0, err
		}
	}
	stepTurn := buildStepTurnMappings(taskCount, opts.tdd, tddState)
	startTurnID, endTurnID := firstLastTurnID(stepTurn)

	if decision == "reject" {
		rejectPolicy, evalErr := core.EvaluateApprovalPolicy(opts.approval, core.PhaseImplementation, decision, true)
		if evalErr != nil {
			return "", 0, evalErr
		}
		errCode := "E_RUN_REJECTED"
		errMsg := "run rejected by decision gate"
		if rejectPolicy.Required {
			errCode = approvalRequiredCode
			errMsg = fmt.Sprintf("run rejected: approval required by policy (%s)", rejectPolicy.Reason)
		}
		if err := writeObservabilityLog(failLog, "run_reject", string(core.PhaseImplementation), threadID, startTurnID, errCode, errMsg, now); err != nil {
			return "", 0, err
		}
		st.Run.LastFailedStep = "decision:reject"
		st.Run.LastFailureCause = "manual reject"
		st.SetLastError(errCode, errMsg, now)
		st.SetLastResult("run", "rejected", filepath.ToSlash(filepath.Join(".ax", "logs", filepath.Base(failLog))), now)
		_ = st.AddContext(filepath.ToSlash(filepath.Join(".ax", "logs", filepath.Base(failLog))))
		if err := st.Save(base); err != nil {
			return "", 0, err
		}
		_ = writeRuntimeCheckpoint(base, st, "failed", "run", time.Now())
		_ = appendRuntimeJournal(base, runtimeJournalEntry{
			Time:      time.Now().Format(time.RFC3339),
			SessionID: st.Runtime.SessionID,
			ClusterID: st.Runtime.ClusterID,
			NodeID:    st.Runtime.NodeID,
			Command:   "run",
			Stage:     "failed",
			Mode:      string(st.Runtime.Mode),
			Phase:     string(st.Phase),
			PlanID:    planID,
			ErrorCode: errCode,
			Error:     errMsg,
		})
		return "", 0, errors.New(errMsg)
	}

	worktreeMode := "disabled"
	worktreeRel := ""
	if !opts.noWorktree {
		proposalRef := proposalRefFromPlan(planID, string(body), st.Current.Proposal)
		worktreeMode = "enabled"
		worktreeDir := filepath.Join(base, ".ax", "worktrees", sanitizeToken(proposalRef))
		if st.Runtime.Mode == core.RuntimeModeWorktree || st.Runtime.Mode == core.RuntimeModeShared {
			worktreeDir = filepath.Join(worktreeDir, sanitizeToken(st.Runtime.SessionID))
		}
		if err := os.MkdirAll(worktreeDir, 0o755); err != nil {
			return "", 0, err
		}
		if rel, relErr := filepath.Rel(base, worktreeDir); relErr == nil {
			worktreeRel = filepath.ToSlash(rel)
		} else {
			worktreeRel = filepath.ToSlash(strings.TrimPrefix(worktreeDir, filepath.Clean(base)+string(os.PathSeparator)))
		}
		worktreeMeta := strings.Join([]string{
			"proposal_id: " + proposalRef,
			"session_id: " + st.Runtime.SessionID,
			"runtime_mode: " + string(st.Runtime.Mode),
			"created_at: " + now.Format(time.RFC3339),
			"status: prepared",
			"",
		}, "\n")
		if err := os.WriteFile(filepath.Join(worktreeDir, "worktree.yaml"), []byte(worktreeMeta), 0o644); err != nil {
			return "", 0, err
		}
	}
	contextLayers := core.BuildContextLayers(base, st.ContextChain, affected)

	pendingContext := []string{
		filepath.ToSlash(filepath.Join(".ax", "runs", runName)),
		filepath.ToSlash(filepath.Join(".ax", "logs", filepath.Base(startLog))),
		filepath.ToSlash(filepath.Join(".ax", "logs", filepath.Base(endLog))),
	}
	if worktreeRel != "" {
		pendingContext = append(pendingContext, filepath.ToSlash(filepath.Join(worktreeRel, "worktree.yaml")))
	}
	likelyCompaction := willContextCompactionTrigger(st.ContextChain, pendingContext)

	var report strings.Builder
	report.WriteString("# Run Report\n\n")
	report.WriteString(fmt.Sprintf("- plan: %s\n", planID))
	report.WriteString(fmt.Sprintf("- executed_at: %s\n", now.Format(time.RFC3339)))
	report.WriteString(fmt.Sprintf("- runtime_mode: %s\n", st.Runtime.Mode))
	report.WriteString(fmt.Sprintf("- session_id: %s\n", st.Runtime.SessionID))
	report.WriteString(fmt.Sprintf("- detected_tasks: %d\n", taskCount))
	report.WriteString(fmt.Sprintf("- approval_policy: %s\n", opts.approval))
	report.WriteString(fmt.Sprintf("- approval_required: %t\n", approvalPolicy.Required))
	report.WriteString(fmt.Sprintf("- approval_reason: %s\n", approvalPolicy.Reason))
	report.WriteString(fmt.Sprintf("- worktree_mode: %s\n", worktreeMode))
	if worktreeRel != "" {
		report.WriteString(fmt.Sprintf("- worktree_path: %s\n", worktreeRel))
	}
	report.WriteString(fmt.Sprintf("- mode: %s\n", ternary(opts.tdd, "tdd", "standard")))
	report.WriteString(fmt.Sprintf("- loop_mode: %s\n", ternary(opts.loop, "enabled", "disabled")))
	report.WriteString(fmt.Sprintf("- depth: %s\n", depth))
	report.WriteString(fmt.Sprintf("- depth_source: %s\n", depthSource))
	report.WriteString(fmt.Sprintf("- decision: %s\n", decision))
	if decision == "steer" {
		report.WriteString(fmt.Sprintf("- steer_instruction: %s\n", strings.TrimSpace(opts.steerText)))
	}
	report.WriteString(fmt.Sprintf("- review_count: %d\n", opts.reviewCount))
	report.WriteString(fmt.Sprintf("- quality_gate: %s\n", core.QualityGateSummary(opts.reviewCount, opts.force)))
	if opts.force {
		report.WriteString(fmt.Sprintf("- force_reason: %s\n", strings.TrimSpace(opts.forceReason)))
	}
	report.WriteString(fmt.Sprintf("- context_compaction_threshold: %d\n", core.MaxContextChainEntries))
	report.WriteString(fmt.Sprintf("- context_compaction_expected: %t\n", likelyCompaction))
	report.WriteString("\n## Context Inputs\n")
	for _, p := range affected {
		report.WriteString(fmt.Sprintf("- %s\n", p))
	}
	report.WriteString("\n## Context Layers\n")
	report.WriteString(fmt.Sprintf("- protocol: %s\n", strings.Join(contextLayers.Protocol, ", ")))
	report.WriteString(fmt.Sprintf("- task_scoped: %s\n", strings.Join(contextLayers.TaskScoped, ", ")))
	report.WriteString(fmt.Sprintf("- session_memory: %s\n", strings.Join(contextLayers.SessionMem, ", ")))

	if opts.tdd {
		report.WriteString("\n## TDD Loop\n")
		report.WriteString("- tiers: T0 -> T1 -> T2\n")
		report.WriteString("- steps: Red -> Green -> Refactor\n")
		if opts.loop {
			report.WriteString("- loop: enabled\n")
		}
		report.WriteString(fmt.Sprintf("- tier_progression: %s\n", tddProgress))
		report.WriteString("\n### Tier Progress\n")
		report.WriteString(fmt.Sprintf("- current_tier: %s\n", tddState.CurrentTier))
		report.WriteString(fmt.Sprintf("- current_step: %s\n", tddState.CurrentStep))
		report.WriteString(fmt.Sprintf("- completed_steps: %d/%d\n", tddState.CompletedSteps, tddState.TotalSteps))
		for _, tier := range []string{"T0", "T1", "T2"} {
			tierStatus := "pending"
			if tier == tddState.CurrentTier {
				tierStatus = "active"
			}
			if tddState.CompletedSteps >= tierCompletedStepFloor(tier) {
				tierStatus = "passed"
			}
			report.WriteString(fmt.Sprintf("- %s: %s\n", tier, tierStatus))
		}
		if opts.resume && st.Run.LastFailedStep != "" {
			report.WriteString(fmt.Sprintf("- resumed_from: %s\n", st.Run.LastFailedStep))
		}
	}

	report.WriteString("\n## Step↔Turn Mapping\n")
	if len(stepTurn) == 0 {
		report.WriteString("- none\n")
	} else {
		report.WriteString("| Step | Turn ID | Tier |\n|---|---|---|\n")
		for _, item := range stepTurn {
			report.WriteString(fmt.Sprintf("| %s | %s | %s |\n", item.Step, item.TurnID, emptyFallback(item.Tier)))
		}
	}
	report.WriteString("\n## Status\n- status: completed\n")

	if err := os.WriteFile(runPath, []byte(report.String()), 0o644); err != nil {
		return "", 0, err
	}

	if err := writeObservabilityLog(endLog, "run_end", string(core.PhaseImplementation), threadID, endTurnID, "", "run scaffolding completed", now); err != nil {
		return "", 0, err
	}

	compactionTriggered := false
	if opts.tdd {
		st.TDD = tddState
	} else {
		st.TDD = core.TDDState{}
	}
	st.Current.Run = runName
	st.Run.LastFailedStep = ""
	st.Run.LastFailureCause = ""
	compactionTriggered = st.AddContext(filepath.ToSlash(filepath.Join(".ax", "runs", runName))) || compactionTriggered
	compactionTriggered = st.AddContext(filepath.ToSlash(filepath.Join(".ax", "logs", filepath.Base(startLog)))) || compactionTriggered
	compactionTriggered = st.AddContext(filepath.ToSlash(filepath.Join(".ax", "logs", filepath.Base(endLog)))) || compactionTriggered
	if worktreeRel != "" {
		compactionTriggered = st.AddContext(filepath.ToSlash(filepath.Join(worktreeRel, "worktree.yaml"))) || compactionTriggered
	}
	if compactionTriggered {
		if err := writeObservabilityLog(compactionLog, "context_compaction", string(core.PhaseImplementation), "", "", contextCompactionCode, fmt.Sprintf("context_chain exceeded max=%d and was compacted", core.MaxContextChainEntries), now); err != nil {
			return "", 0, err
		}
		_ = st.AddContext(filepath.ToSlash(filepath.Join(".ax", "logs", filepath.Base(compactionLog))))
	}
	st.SetLastResult("run", ternary(decision == "steer", "steered", "completed"), filepath.ToSlash(filepath.Join(".ax", "runs", runName)), now)
	st.ClearLastError()
	if err := st.Save(base); err != nil {
		return "", 0, err
	}
	if err := writeRuntimeCheckpoint(base, st, "completed", "run", time.Now()); err != nil {
		return "", 0, err
	}
	_ = appendRuntimeJournal(base, runtimeJournalEntry{
		Time:      time.Now().Format(time.RFC3339),
		SessionID: st.Runtime.SessionID,
		ClusterID: st.Runtime.ClusterID,
		NodeID:    st.Runtime.NodeID,
		Command:   "run",
		Stage:     "completed",
		Mode:      string(st.Runtime.Mode),
		Phase:     string(st.Phase),
		PlanID:    planID,
		Artifact:  filepath.ToSlash(filepath.Join(".ax", "runs", runName)),
	})

	return runPath, taskCount, nil
}

func verifyProposal(base, proposal, testsRaw, buildRaw, acRaw string, rt runtimeContext, now time.Time) (reportPath string, proposalID string, err error) {
	proposalDir, proposalID, err := resolveProposal(base, proposal)
	if err != nil {
		return "", "", err
	}
	reportPath = filepath.Join(proposalDir, "verify.md")
	previousBody, _ := os.ReadFile(reportPath)

	evidence := core.VerifyEvidence{
		Tests:      core.ParseEvidenceStatus(testsRaw),
		Build:      core.ParseEvidenceStatus(buildRaw),
		Acceptance: core.ParseEvidenceStatus(acRaw),
	}
	judgment, unmet, next := core.EvaluateVerification(evidence)

	var b strings.Builder
	b.WriteString("# Verify Report\n\n")
	b.WriteString(fmt.Sprintf("- proposal: %s\n", proposalID))
	b.WriteString(fmt.Sprintf("- verified_at: %s\n", now.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("- status: %s\n", judgment))
	b.WriteString("\n## Evidence\n")
	b.WriteString(fmt.Sprintf("- tests: %s\n", evidence.Tests))
	b.WriteString(fmt.Sprintf("- build: %s\n", evidence.Build))
	b.WriteString(fmt.Sprintf("- acceptance_criteria: %s\n", evidence.Acceptance))
	b.WriteString("\n## Unmet Items\n")
	if len(unmet) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, u := range unmet {
			b.WriteString("- " + u + "\n")
		}
	}
	b.WriteString("\n## Next Actions\n")
	if len(next) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, n := range next {
			b.WriteString("- " + n + "\n")
		}
	}
	diffPath := filepath.Join(proposalDir, "verify-diff.md")
	if len(previousBody) > 0 {
		diff := core.BuildVerifyDiff(string(previousBody), b.String())
		diffReport := renderVerifyDiffReport(proposalID, diff, now)
		if err := os.WriteFile(diffPath, []byte(diffReport), 0o644); err != nil {
			return "", "", err
		}
		b.WriteString("\n## Previous Verify Diff\n")
		b.WriteString(core.FormatVerifyDiff(diff))
		b.WriteString("- artifact: verify-diff.md\n")
	}

	st, err := core.LoadState(base)
	if err != nil {
		return "", "", err
	}
	applyRuntimeState(st, rt, "verify")
	if err := st.Transition(core.PhaseVerification, "verify", now); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(reportPath, []byte(b.String()), 0o644); err != nil {
		return "", "", err
	}
	st.Current.Proposal = proposalID
	st.Current.Verify = filepath.ToSlash(filepath.Join(".ax", "proposals", proposalID, "verify.md"))
	_ = st.AddContext(filepath.ToSlash(filepath.Join(".ax", "proposals", proposalID, "verify.md")))
	if len(previousBody) > 0 {
		_ = st.AddContext(filepath.ToSlash(filepath.Join(".ax", "proposals", proposalID, "verify-diff.md")))
	}
	st.SetLastResult("verify", string(judgment), st.Current.Verify, now)
	st.ClearLastError()
	if err := st.Save(base); err != nil {
		return "", "", err
	}

	return reportPath, proposalID, nil
}

func archiveProposal(base, proposal string, allowUnverified bool, rt runtimeContext, now time.Time) (string, error) {
	if archivedDir, archivedID, ok := resolveArchivedProposal(base, proposal); ok {
		st, err := core.LoadState(base)
		if err == nil {
			applyRuntimeState(st, rt, "archive")
			st.Current.Proposal = archivedID
			_ = st.AddContext(filepath.ToSlash(filepath.Join(".ax", "archive", archivedID, "archive-metadata.yaml")))
			st.SetLastResult("archive", "archived", filepath.ToSlash(filepath.Join(".ax", "archive", archivedID)), now)
			st.ClearLastError()
			st.ForcePhase(core.PhaseArchived, "archive:idempotent", now)
			_ = st.Save(base)
		}
		_ = archivedDir
		return archivedID, nil
	}

	proposalDir, proposalID, err := resolveProposal(base, proposal)
	if err != nil {
		return "", err
	}

	verifyPath := filepath.Join(proposalDir, "verify.md")
	if !allowUnverified {
		if _, err := os.Stat(verifyPath); err != nil {
			return "", errors.New("archive strict mode: verify.md is required (use --allow-unverified-archive to override)")
		}
	}

	st, err := core.LoadState(base)
	if err != nil {
		return "", err
	}
	applyRuntimeState(st, rt, "archive")
	if err := st.Transition(core.PhaseArchived, "archive", now); err != nil {
		return "", err
	}

	archiveDir := filepath.Join(base, ".ax", "archive", proposalID)
	if _, err := os.Stat(archiveDir); err == nil {
		return "", fmt.Errorf("archive target already exists: %s", archiveDir)
	}
	if err := os.Rename(proposalDir, archiveDir); err != nil {
		return "", err
	}

	verifyResult := "UNKNOWN"
	if body, err := os.ReadFile(filepath.Join(archiveDir, "verify.md")); err == nil {
		verifyResult = parseVerifyStatus(string(body))
	}

	worktreeSnapshot, err := finalizeWorktreeForArchive(base, archiveDir, proposalID, now)
	if err != nil {
		return "", err
	}

	metadataLines := []string{
		"proposal_id: " + proposalID,
		"archived_at: " + now.Format(time.RFC3339),
		"final_status: archived",
		"artifact_paths:",
		fmt.Sprintf("  - %s", filepath.ToSlash(filepath.Join(".ax", "archive", proposalID, "proposal.md"))),
		fmt.Sprintf("  - %s", filepath.ToSlash(filepath.Join(".ax", "archive", proposalID, "design.md"))),
		fmt.Sprintf("  - %s", filepath.ToSlash(filepath.Join(".ax", "archive", proposalID, "tasks.md"))),
		fmt.Sprintf("  - %s", filepath.ToSlash(filepath.Join(".ax", "archive", proposalID, "verify.md"))),
	}
	if worktreeSnapshot != "" {
		metadataLines = append(metadataLines, fmt.Sprintf("  - %s", worktreeSnapshot))
	}
	metadataLines = append(metadataLines, "verification_result: "+verifyResult, "")
	metadata := strings.Join(metadataLines, "\n")
	if err := os.WriteFile(filepath.Join(archiveDir, "archive-metadata.yaml"), []byte(metadata), 0o644); err != nil {
		return "", err
	}
	st.Current.Proposal = proposalID
	_ = st.AddContext(filepath.ToSlash(filepath.Join(".ax", "archive", proposalID, "archive-metadata.yaml")))
	if worktreeSnapshot != "" {
		_ = st.AddContext(worktreeSnapshot)
	}
	st.SetLastResult("archive", "archived", filepath.ToSlash(filepath.Join(".ax", "archive", proposalID)), now)
	st.ClearLastError()
	if err := st.Save(base); err != nil {
		return "", err
	}
	return proposalID, nil
}

func discoverTopic(base, topic string, party bool, rt runtimeContext, now time.Time) (string, int, error) {
	needle := strings.ToLower(strings.TrimSpace(topic))
	if needle == "" {
		return "", 0, errors.New("topic is required")
	}

	var matches []string
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(base, path)
		if relErr != nil {
			return relErr
		}
		if strings.HasPrefix(rel, ".git") || strings.HasPrefix(rel, ".ax") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if len(matches) >= 20 {
			return nil
		}

		if strings.Contains(strings.ToLower(rel), needle) {
			matches = append(matches, fmt.Sprintf("- file: %s", rel))
		}
		f, openErr := os.Open(path)
		if openErr != nil {
			return nil
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		lineNo := 0
		for sc.Scan() {
			lineNo++
			line := sc.Text()
			if strings.Contains(strings.ToLower(line), needle) {
				matches = append(matches, fmt.Sprintf("- %s:%d %s", rel, lineNo, truncateLine(line, 120)))
				break
			}
			if lineNo >= 400 {
				break
			}
		}
		return nil
	})
	if err != nil {
		return "", 0, err
	}
	sort.Strings(matches)

	reportName := fmt.Sprintf("%s-%s.md", sanitizeToken(topic), now.Format("20060102-150405"))
	reportPath := filepath.Join(base, ".ax", "discovery", reportName)

	var b strings.Builder
	b.WriteString("# Discovery Report\n\n")
	b.WriteString(fmt.Sprintf("- topic: %s\n", topic))
	b.WriteString(fmt.Sprintf("- generated_at: %s\n", now.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("- matches: %d\n", len(matches)))
	b.WriteString("\n## 문제 재정의\n")
	b.WriteString("- 해결해야 할 핵심 문제를 다시 정의한다.\n")
	b.WriteString("\n## 옵션 A/B/C\n")
	b.WriteString("- Option A: 최소 변경\n- Option B: 균형안\n- Option C: 구조 개선\n")
	b.WriteString("\n## 트레이드오프\n")
	b.WriteString("- 속도 vs 안정성 vs 유지보수성\n")
	b.WriteString("\n## 리스크\n")
	b.WriteString("- 요구사항 변동, 복잡도 증가\n")
	b.WriteString("\n## 추천안\n")
	b.WriteString("- Option B를 기본 권고\n")
	if party {
		b.WriteString("\n## Party Mode\n")
		b.WriteString("### Architect\n- 아키텍처/의존성/확장성 관점\n")
		b.WriteString("### User\n- 사용자 경험/엣지 케이스 관점\n")
		b.WriteString("### QA\n- 테스트 가능성/실패 시나리오 관점\n")
		b.WriteString("\n### Tradeoff Matrix\n| Persona | Focus | Risk |\n|---|---|---|\n| Architect | 구조 | 복잡도 증가 |\n| User | 사용성 | 요구사항 누락 |\n| QA | 검증성 | 커버리지 부족 |\n")
	}
	b.WriteString("\n## Findings\n")
	if len(matches) == 0 {
		b.WriteString("- no matches found\n")
	} else {
		for _, m := range matches {
			b.WriteString(m + "\n")
		}
	}

	st, err := core.LoadState(base)
	if err != nil {
		return "", 0, err
	}
	applyRuntimeState(st, rt, "discover")
	if err := st.Transition(core.PhaseDiscovery, "discover", now); err != nil {
		return "", 0, err
	}
	if err := os.WriteFile(reportPath, []byte(b.String()), 0o644); err != nil {
		return "", 0, err
	}
	st.Current.Discover = filepath.ToSlash(filepath.Join(".ax", "discovery", reportName))
	_ = st.AddContext(st.Current.Discover)
	st.SetLastResult("discover", "created", st.Current.Discover, now)
	st.ClearLastError()
	if err := st.Save(base); err != nil {
		return "", 0, err
	}

	return reportPath, len(matches), nil
}

func runQuick(base, task string, rt runtimeContext, now time.Time) (string, error) {
	st, err := core.LoadState(base)
	if err != nil {
		return "", err
	}
	applyRuntimeState(st, rt, "quick")
	previousPhase := st.Phase
	if err := st.Transition(core.PhaseImplementation, "quick", now); err != nil {
		return "", err
	}

	runName := fmt.Sprintf("quick-%s.md", now.Format("20060102-150405"))
	runPath := filepath.Join(base, ".ax", "runs", runName)
	report := strings.Join([]string{
		"# Quick Run Report",
		"",
		"- mode: quick",
		fmt.Sprintf("- task: %s", task),
		fmt.Sprintf("- executed_at: %s", now.Format(time.RFC3339)),
		"- result: completed",
		"",
		"## Notes",
		"- quick 종료 후 state 복원/정리",
		"- 실패 시 standard flow 승격 안내 필요",
		"",
	}, "\n")
	if err := os.WriteFile(runPath, []byte(report), 0o644); err != nil {
		return "", err
	}

	st.Current.Run = runName
	_ = st.AddContext(filepath.ToSlash(filepath.Join(".ax", "runs", runName)))
	st.SetLastResult("quick", "completed", filepath.ToSlash(filepath.Join(".ax", "runs", runName)), now)
	st.ClearLastError()
	st.ForcePhase(previousPhase, "quick:restore", now)
	if err := st.Save(base); err != nil {
		return "", err
	}
	return runPath, nil
}

func createReview(base, proposal string, rt runtimeContext, now time.Time) (string, error) {
	_, proposalID, err := resolveProposal(base, proposal)
	if err != nil {
		return "", err
	}
	reviewName := fmt.Sprintf("%s-review-%s.md", sanitizeToken(proposalID), now.Format("20060102-150405"))
	reviewPath := filepath.Join(base, ".ax", "reviews", reviewName)

	lenses := []string{
		"Correctness",
		"Reliability / Recovery",
		"Test Adequacy",
		"Observability",
		"Security / Safety",
		"Maintainability",
		"Performance",
		"UX / CLI Clarity",
	}

	var b strings.Builder
	b.WriteString("# Review Report\n\n")
	b.WriteString(fmt.Sprintf("- proposal: %s\n", proposalID))
	b.WriteString(fmt.Sprintf("- reviewed_at: %s\n", now.Format(time.RFC3339)))
	b.WriteString("\n")
	for _, lens := range lenses {
		b.WriteString(fmt.Sprintf("## %s\n", lens))
		b.WriteString("- severity: minor\n")
		b.WriteString("- notes: placeholder\n\n")
	}

	if err := os.WriteFile(reviewPath, []byte(b.String()), 0o644); err != nil {
		return "", err
	}

	st, err := core.LoadState(base)
	if err != nil {
		return "", err
	}
	applyRuntimeState(st, rt, "review")
	st.Current.Review = filepath.ToSlash(filepath.Join(".ax", "reviews", reviewName))
	_ = st.AddContext(st.Current.Review)
	st.SetLastResult("review", "created", st.Current.Review, now)
	st.ClearLastError()
	if err := st.Save(base); err != nil {
		return "", err
	}

	return reviewPath, nil
}

func createCompound(base string, audit bool, rt runtimeContext, now time.Time) (string, error) {
	reportName := fmt.Sprintf("compound-%s.md", now.Format("20060102-150405"))
	reportPath := filepath.Join(base, ".ax", "compound", reportName)
	gotchasPath := filepath.Join(base, ".ax", "memory", "gotchas.md")
	rawGotchas, err := os.ReadFile(gotchasPath)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	items := core.ParseGotchas(string(rawGotchas), now)

	var b strings.Builder
	b.WriteString("# Compound Report\n\n")
	b.WriteString(fmt.Sprintf("- created_at: %s\n", now.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("- gotcha_source: %s\n", filepath.ToSlash(filepath.Join(".ax", "memory", "gotchas.md"))))
	b.WriteString(fmt.Sprintf("- gotcha_count: %d\n", len(items)))
	triageResult := core.BuildCompoundTriage(items, now)
	b.WriteString("\n## Triage\n")
	renderCompoundTriageSection(&b, "FixCandidate", triageResult.FixCandidates)
	renderCompoundTriageSection(&b, "Document", triageResult.DocumentItems)
	renderCompoundTriageSection(&b, "Noise", triageResult.NoiseItems)

	if audit {
		auditResult := core.BuildCompoundAudit(items, now)
		b.WriteString("\n## Audit\n")
		b.WriteString(fmt.Sprintf("- decay_candidates: %d\n", len(auditResult.DecayCandidates)))
		b.WriteString(fmt.Sprintf("- archive_candidates: %d\n", len(auditResult.ArchiveCandidates)))
		b.WriteString("\n### Decay Candidates\n")
		writeAuditItems(&b, auditResult.DecayCandidates)
		b.WriteString("\n### Archive Candidates\n")
		writeAuditItems(&b, auditResult.ArchiveCandidates)
	}

	if err := os.WriteFile(reportPath, []byte(b.String()), 0o644); err != nil {
		return "", err
	}

	st, err := core.LoadState(base)
	if err != nil {
		return "", err
	}
	applyRuntimeState(st, rt, "compound")
	st.Current.Compound = filepath.ToSlash(filepath.Join(".ax", "compound", reportName))
	_ = st.AddContext(st.Current.Compound)
	st.SetLastResult("compound", "created", st.Current.Compound, now)
	st.ClearLastError()
	if err := st.Save(base); err != nil {
		return "", err
	}

	return reportPath, nil
}

func resolveRuntimeContext(cmd *cobra.Command, now time.Time) (runtimeContext, error) {
	flags := cmd.Root().PersistentFlags()
	rawMode, err := flags.GetString("runtime-mode")
	if err != nil {
		return runtimeContext{}, err
	}
	if !flags.Changed("runtime-mode") {
		rawMode = strings.TrimSpace(rawMode)
	}
	mode, err := core.ResolveRuntimeMode(rawMode)
	if err != nil {
		return runtimeContext{}, err
	}
	sessionID, err := flags.GetString("session-id")
	if err != nil {
		return runtimeContext{}, err
	}
	clusterID, err := flags.GetString("cluster-id")
	if err != nil {
		return runtimeContext{}, err
	}
	nodeID, err := flags.GetString("node-id")
	if err != nil {
		return runtimeContext{}, err
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		sessionID = core.NewSessionID(now)
	}
	return runtimeContext{
		mode:      mode,
		sessionID: sessionID,
		clusterID: strings.TrimSpace(clusterID),
		nodeID:    strings.TrimSpace(nodeID),
	}, nil
}

func applyRuntimeState(st *core.State, rt runtimeContext, command string) {
	if st == nil {
		return
	}
	now := time.Now()
	if rt.mode == "" {
		rt.mode = core.RuntimeModeSingle
	}
	st.Runtime.Mode = rt.mode
	st.Runtime.ClusterID = core.ResolveRuntimeClusterID(rt.clusterID)
	st.Runtime.NodeID = core.ResolveRuntimeNodeID(rt.nodeID)
	st.Runtime.SessionJournal = filepath.ToSlash(filepath.Join(".ax", "logs", "runtime-journal.jsonl"))
	if strings.TrimSpace(rt.sessionID) == "" {
		rt.sessionID = core.NewSessionID(now)
	}
	st.Runtime.SessionID = rt.sessionID
	if st.Runtime.ActiveSessions == nil {
		st.Runtime.ActiveSessions = map[string]string{}
	}
	if st.Runtime.SessionMeta == nil {
		st.Runtime.SessionMeta = map[string]core.RuntimeSessionRef{}
	}
	st.Runtime.ActiveSessions[rt.sessionID] = command
	meta := st.Runtime.SessionMeta[rt.sessionID]
	if meta.StartedAt == "" {
		meta.StartedAt = now.Format(time.RFC3339)
	}
	meta.SessionID = rt.sessionID
	meta.Command = command
	meta.Mode = rt.mode
	meta.ClusterID = st.Runtime.ClusterID
	meta.NodeID = st.Runtime.NodeID
	meta.UpdatedAt = now.Format(time.RFC3339)
	meta.Status = "active"
	st.Runtime.SessionMeta[rt.sessionID] = meta
	if len(st.Runtime.ActiveSessions) > 64 {
		// best-effort pruning: keep current session only when map grows too large.
		current := st.Runtime.ActiveSessions[rt.sessionID]
		st.Runtime.ActiveSessions = map[string]string{
			rt.sessionID: current,
		}
		currentMeta := st.Runtime.SessionMeta[rt.sessionID]
		st.Runtime.SessionMeta = map[string]core.RuntimeSessionRef{
			rt.sessionID: currentMeta,
		}
	}
}

func printState(cmd *cobra.Command, base string, jsonOut bool) error {
	st, err := core.LoadState(base)
	if err != nil {
		return err
	}
	if jsonOut {
		body, err := json.MarshalIndent(st, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(body))
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "phase: %s\n", st.Phase)
	fmt.Fprintf(cmd.OutOrStdout(), "runtime mode: %s\n", emptyFallback(string(st.Runtime.Mode)))
	fmt.Fprintf(cmd.OutOrStdout(), "session id: %s\n", emptyFallback(st.Runtime.SessionID))
	fmt.Fprintf(cmd.OutOrStdout(), "current proposal: %s\n", emptyFallback(st.Current.Proposal))
	fmt.Fprintf(cmd.OutOrStdout(), "current plan: %s\n", emptyFallback(st.Current.Plan))
	fmt.Fprintf(cmd.OutOrStdout(), "last result: %s\n", formatLastResult(st.LastResult))
	fmt.Fprintf(cmd.OutOrStdout(), "last error: %s\n", formatLastError(st.LastError))
	fmt.Fprintf(cmd.OutOrStdout(), "context_chain: %d\n", len(st.ContextChain))
	fmt.Fprintf(cmd.OutOrStdout(), "progress: %d%%\n", st.Progress)
	if st.TDD.Enabled {
		fmt.Fprintf(cmd.OutOrStdout(), "tdd: tier=%s step=%s completed=%d/%d depth=%s (%s) approval=%s resume=%t\n",
			emptyFallback(st.TDD.CurrentTier),
			emptyFallback(st.TDD.CurrentStep),
			st.TDD.CompletedSteps,
			st.TDD.TotalSteps,
			emptyFallback(st.TDD.Depth),
			emptyFallback(st.TDD.DepthSource),
			emptyFallback(st.TDD.ApprovalPolicy),
			st.TDD.Resume,
		)
	}
	return nil
}

func tierCompletedStepFloor(tier string) int {
	switch strings.ToUpper(strings.TrimSpace(tier)) {
	case "T0":
		return 3
	case "T1":
		return 6
	case "T2":
		return 9
	default:
		return 0
	}
}

func proposalRefFromPlan(planID, planBody, fallback string) string {
	re := regexp.MustCompile(`(?m)^- from:\s*(.+)$`)
	if m := re.FindStringSubmatch(planBody); len(m) > 1 {
		from := strings.TrimSpace(m[1])
		if from != "" {
			if filepath.IsAbs(from) || strings.Contains(from, string(os.PathSeparator)) {
				from = filepath.Base(filepath.Dir(from))
				if from == "." || from == string(os.PathSeparator) {
					from = filepath.Base(strings.TrimSpace(m[1]))
				}
			}
			return sanitizeToken(from)
		}
	}
	trimmed := strings.TrimSuffix(planID, "-plan")
	if strings.TrimSpace(trimmed) != "" {
		return sanitizeToken(trimmed)
	}
	if strings.TrimSpace(fallback) != "" {
		return sanitizeToken(fallback)
	}
	return sanitizeToken(planID)
}

func syntheticThreadID(planID string) string {
	return "thread-" + sanitizeToken(planID)
}

func buildStepTurnMappings(taskCount int, tdd bool, state core.TDDState) []stepTurnMapping {
	if tdd {
		tier := strings.ToUpper(strings.TrimSpace(state.CurrentTier))
		if tier == "" {
			tier = "T0"
		}
		steps := []string{"Red", "Green", "Refactor"}
		out := make([]stepTurnMapping, 0, len(steps))
		for i, step := range steps {
			out = append(out, stepTurnMapping{
				Step:   fmt.Sprintf("%s/%s", tier, step),
				TurnID: fmt.Sprintf("turn-%03d", i+1),
				Tier:   tier,
			})
		}
		return out
	}

	if taskCount <= 0 {
		taskCount = 1
	}
	out := make([]stepTurnMapping, 0, taskCount)
	for i := 1; i <= taskCount; i++ {
		out = append(out, stepTurnMapping{
			Step:   fmt.Sprintf("Task-%02d", i),
			TurnID: fmt.Sprintf("turn-%03d", i),
		})
	}
	return out
}

func firstLastTurnID(items []stepTurnMapping) (string, string) {
	if len(items) == 0 {
		return "", ""
	}
	return items[0].TurnID, items[len(items)-1].TurnID
}

func finalizeWorktreeForArchive(base, archiveDir, proposalID string, now time.Time) (string, error) {
	worktreeDir := filepath.Join(base, ".ax", "worktrees", sanitizeToken(proposalID))
	if _, err := os.Stat(worktreeDir); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	var discovered []string
	err := filepath.WalkDir(worktreeDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) != "worktree.yaml" {
			return nil
		}
		rel, relErr := filepath.Rel(base, path)
		if relErr != nil {
			return relErr
		}
		discovered = append(discovered, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(discovered) == 0 {
		return "", nil
	}

	snapshot := strings.Join([]string{
		"proposal_id: " + proposalID,
		"archived_at: " + now.Format(time.RFC3339),
		"status: merged_and_cleaned",
		"merge_result: success",
		"cleanup_result: success",
		"discovered_worktrees:",
		func() string {
			var b strings.Builder
			for _, p := range discovered {
				b.WriteString("  - " + p + "\n")
			}
			return strings.TrimRight(b.String(), "\n")
		}(),
		"",
	}, "\n")

	archiveWorktree := filepath.Join(archiveDir, "worktree.yaml")
	if err := os.WriteFile(archiveWorktree, []byte(snapshot), 0o644); err != nil {
		return "", err
	}
	if err := os.RemoveAll(worktreeDir); err != nil {
		return "", err
	}
	return filepath.ToSlash(filepath.Join(".ax", "archive", proposalID, "worktree.yaml")), nil
}

func willContextCompactionTrigger(existing []string, additions []string) bool {
	seen := map[string]struct{}{}
	count := 0
	for _, p := range existing {
		n := strings.TrimSpace(filepath.ToSlash(p))
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		count++
	}
	for _, p := range additions {
		n := strings.TrimSpace(filepath.ToSlash(p))
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		count++
		if count > core.MaxContextChainEntries {
			return true
		}
	}
	return false
}

func renderVerifyDiffReport(proposalID string, diff core.VerifyDiff, now time.Time) string {
	var b strings.Builder
	b.WriteString("# Verify Diff\n\n")
	b.WriteString(fmt.Sprintf("- proposal: %s\n", proposalID))
	b.WriteString(fmt.Sprintf("- compared_at: %s\n", now.Format(time.RFC3339)))
	b.WriteString(core.FormatVerifyDiff(diff))
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
	return b.String()
}

func renderCompoundTriageSection(b *strings.Builder, section string, items []core.GotchaItem) {
	b.WriteString(fmt.Sprintf("\n### %s\n", section))
	if len(items) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, item := range items {
		b.WriteString(fmt.Sprintf("- id: %s\n", item.ID))
		b.WriteString(fmt.Sprintf("  text: %s\n", item.Text))
		b.WriteString(fmt.Sprintf("  added: %s\n", item.Added))
		b.WriteString(fmt.Sprintf("  last_relevant: %s\n", item.LastRelevant))
		b.WriteString(fmt.Sprintf("  decay_after: %s\n", item.DecayAfter))
		b.WriteString(fmt.Sprintf("  fix_candidate: %t\n", item.FixCandidate))
		b.WriteString(fmt.Sprintf("  triage: %s\n", item.Triage))
	}
}

func filterTriage(items []core.GotchaItem, triage core.TriageClass) []core.GotchaItem {
	out := make([]core.GotchaItem, 0, len(items))
	for _, item := range items {
		if item.Triage == triage {
			out = append(out, item)
		}
	}
	return out
}

func writeAuditItems(b *strings.Builder, items []core.GotchaItem) {
	if len(items) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, item := range items {
		b.WriteString(fmt.Sprintf("- %s (%s)\n", item.ID, item.Triage))
	}
}

func writeObservabilityLog(path, step, phase, threadID, turnID, errorCode, message string, at time.Time) error {
	payload := strings.Join([]string{
		"step: " + step,
		"phase: " + phase,
		"thread_id: " + emptyFallback(threadID),
		"turn_id: " + emptyFallback(turnID),
		"error_code: " + emptyFallback(errorCode),
		"time: " + at.Format(time.RFC3339),
		"message: " + message,
		"",
	}, "\n")
	return os.WriteFile(path, []byte(payload), 0o644)
}

type runtimeCheckpoint struct {
	SessionID string        `json:"session_id"`
	Status    string        `json:"status"`
	Command   string        `json:"command,omitempty"`
	Mode      string        `json:"mode,omitempty"`
	Phase     string        `json:"phase,omitempty"`
	UpdatedAt string        `json:"updated_at,omitempty"`
	TDD       core.TDDState `json:"tdd"`
	Run       core.RunState `json:"run,omitempty"`
}

func runtimeCheckpointPath(base, sessionID string) string {
	id := sanitizeToken(sessionID)
	if id == "" {
		id = "session"
	}
	return filepath.Join(base, ".ax", "runtime", "checkpoints", id+".json")
}

func writeRuntimeCheckpoint(base string, st *core.State, status, command string, at time.Time) error {
	if st == nil || strings.TrimSpace(st.Runtime.SessionID) == "" {
		return nil
	}
	path := runtimeCheckpointPath(base, st.Runtime.SessionID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	cp := runtimeCheckpoint{
		SessionID: st.Runtime.SessionID,
		Status:    strings.TrimSpace(status),
		Command:   strings.TrimSpace(command),
		Mode:      string(st.Runtime.Mode),
		UpdatedAt: at.Format(time.RFC3339),
		Phase:     string(st.Phase),
		TDD:       st.TDD,
		Run:       st.Run,
	}
	body, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(body, '\n'), 0o644)
}

func loadRuntimeCheckpoint(base, sessionID string) (runtimeCheckpoint, bool, error) {
	path := runtimeCheckpointPath(base, sessionID)
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return runtimeCheckpoint{}, false, nil
		}
		return runtimeCheckpoint{}, false, err
	}
	var cp runtimeCheckpoint
	if err := json.Unmarshal(body, &cp); err != nil {
		return runtimeCheckpoint{}, false, err
	}
	return cp, true, nil
}

func restoreRuntimeCheckpoint(base string, st *core.State) (runtimeCheckpoint, error) {
	if st == nil {
		return runtimeCheckpoint{}, nil
	}
	cp, ok, err := loadRuntimeCheckpoint(base, st.Runtime.SessionID)
	if err != nil {
		return runtimeCheckpoint{}, err
	}
	if !ok {
		return runtimeCheckpoint{}, nil
	}

	// Restore missing resume state from checkpoint payload.
	if !st.TDD.Enabled && cp.TDD.Enabled {
		st.TDD = cp.TDD
	}
	if strings.TrimSpace(st.Run.LastFailedStep) == "" && strings.TrimSpace(cp.Run.LastFailedStep) != "" {
		st.Run.LastFailedStep = cp.Run.LastFailedStep
	}
	if strings.TrimSpace(st.Run.LastFailureCause) == "" && strings.TrimSpace(cp.Run.LastFailureCause) != "" {
		st.Run.LastFailureCause = cp.Run.LastFailureCause
	}

	return cp, nil
}

type runtimeJournalEntry struct {
	Time      string `json:"time"`
	SessionID string `json:"session_id"`
	ClusterID string `json:"cluster_id,omitempty"`
	NodeID    string `json:"node_id,omitempty"`
	Command   string `json:"command"`
	Stage     string `json:"stage"`
	Mode      string `json:"mode,omitempty"`
	Phase     string `json:"phase,omitempty"`
	PlanID    string `json:"plan_id,omitempty"`
	Artifact  string `json:"artifact,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`
	Error     string `json:"error,omitempty"`
}

func runtimeJournalPath(base string) string {
	return filepath.Join(base, ".ax", "logs", "runtime-journal.jsonl")
}

func appendRuntimeJournal(base string, entry runtimeJournalEntry) error {
	if strings.TrimSpace(entry.Time) == "" {
		entry.Time = time.Now().Format(time.RFC3339)
	}
	path := runtimeJournalPath(base)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(body, '\n')); err != nil {
		return err
	}
	return nil
}

func latestRuntimeJournalEntry(base, sessionID, command string) (runtimeJournalEntry, bool, error) {
	path := runtimeJournalPath(base)
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return runtimeJournalEntry{}, false, nil
		}
		return runtimeJournalEntry{}, false, err
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var entry runtimeJournalEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if strings.TrimSpace(sessionID) != "" && entry.SessionID != sessionID {
			continue
		}
		if strings.TrimSpace(command) != "" && entry.Command != command {
			continue
		}
		return entry, true, nil
	}
	return runtimeJournalEntry{}, false, nil
}

func extractTaskIDs(tasksPath string) ([]string, error) {
	body, err := os.ReadFile(tasksPath)
	if err != nil {
		return nil, err
	}
	re := regexp.MustCompile(`(?m)^- \[ \] (T-\d+)`)
	matches := re.FindAllStringSubmatch(string(body), -1)
	ids := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) > 1 {
			ids = append(ids, m[1])
		}
	}
	return ids, nil
}

func isValidApprovalPolicy(v string) bool {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "never", "on-failure", "unless-allow-listed", "always":
		return true
	default:
		return false
	}
}

func parseVerifyStatus(report string) string {
	re := regexp.MustCompile(`(?m)^- status:\s*(.+)$`)
	m := re.FindStringSubmatch(report)
	if len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return "UNKNOWN"
}

func countUncheckedTasks(plan string) int {
	re := regexp.MustCompile(`(?m)^- \[ \] `)
	return len(re.FindAllString(plan, -1))
}

func truncateLine(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func proposalID(title string, now time.Time) string {
	ts := now.Format("20060102-150405")
	return ts + "-" + sanitizeToken(title)
}

func resolveProposal(base, proposal string) (dir string, id string, err error) {
	proposal = strings.TrimSpace(proposal)
	if proposal == "" {
		return "", "", errors.New("proposal is required")
	}

	if filepath.IsAbs(proposal) || strings.Contains(proposal, string(os.PathSeparator)) {
		candidate := proposal
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(base, candidate)
		}
		info, err := os.Stat(candidate)
		if err != nil {
			return "", "", err
		}
		if !info.IsDir() {
			candidate = filepath.Dir(candidate)
		}
		if _, err := os.Stat(filepath.Join(candidate, "proposal.md")); err != nil {
			return "", "", fmt.Errorf("proposal path must include proposal.md: %w", err)
		}
		return candidate, filepath.Base(candidate), nil
	}

	dir = filepath.Join(base, ".ax", "proposals", proposal)
	if _, err := os.Stat(dir); err != nil {
		return "", "", err
	}
	return dir, proposal, nil
}

func resolveArchivedProposal(base, proposal string) (dir string, id string, ok bool) {
	proposal = strings.TrimSpace(proposal)
	if proposal == "" {
		return "", "", false
	}

	if filepath.IsAbs(proposal) || strings.Contains(proposal, string(os.PathSeparator)) {
		candidate := proposal
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(base, candidate)
		}
		info, err := os.Stat(candidate)
		if err != nil {
			return "", "", false
		}
		if !info.IsDir() {
			candidate = filepath.Dir(candidate)
		}
		if _, err := os.Stat(filepath.Join(candidate, "archive-metadata.yaml")); err != nil {
			return "", "", false
		}
		return candidate, filepath.Base(candidate), true
	}

	candidate := filepath.Join(base, ".ax", "archive", proposal)
	if _, err := os.Stat(filepath.Join(candidate, "archive-metadata.yaml")); err != nil {
		return "", "", false
	}
	return candidate, proposal, true
}

func resolvePlan(base, plan string) (path string, id string, err error) {
	if strings.TrimSpace(plan) == "" {
		return "", "", errors.New("plan is required")
	}
	if filepath.IsAbs(plan) || strings.Contains(plan, string(os.PathSeparator)) {
		if _, err := os.Stat(plan); err != nil {
			return "", "", err
		}
		return plan, strings.TrimSuffix(filepath.Base(plan), filepath.Ext(plan)), nil
	}
	path = filepath.Join(base, ".ax", "plans", plan)
	if _, err := os.Stat(path); err != nil {
		return "", "", err
	}
	return path, strings.TrimSuffix(plan, filepath.Ext(plan)), nil
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

func ternary[T any](cond bool, whenTrue, whenFalse T) T {
	if cond {
		return whenTrue
	}
	return whenFalse
}

func emptyFallback(s string) string {
	if strings.TrimSpace(s) == "" {
		return "none"
	}
	return s
}

func formatLastResult(result core.LastResult) string {
	if strings.TrimSpace(result.Command) == "" {
		return "none"
	}
	parts := []string{result.Command}
	if result.Status != "" {
		parts = append(parts, result.Status)
	}
	if result.Artifact != "" {
		parts = append(parts, result.Artifact)
	}
	if result.At != "" {
		parts = append(parts, result.At)
	}
	return strings.Join(parts, " | ")
}

func formatLastError(last *core.LastError) string {
	if last == nil {
		return "none"
	}
	parts := []string{last.Code, last.Message, last.At}
	filtered := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			filtered = append(filtered, p)
		}
	}
	if len(filtered) == 0 {
		return "none"
	}
	return strings.Join(filtered, " | ")
}

// parsePositiveInt parses positive integer fallback for markdown metrics.
func parsePositiveInt(raw string, fallback int) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}
