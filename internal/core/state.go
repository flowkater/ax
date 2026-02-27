package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	stateDirName                      = ".ax"
	stateFileName                     = "state.yaml"
	MaxContextChainEntries            = 100
	contextChainRetainAfterCompaction = MaxContextChainEntries
)

// Phase is the lifecycle state tracked in .ax/state.yaml.
type Phase string

const (
	PhaseIdle           Phase = "idle"
	PhaseDiscovery      Phase = "discovery"
	PhaseProposal       Phase = "proposal"
	PhasePlanning       Phase = "planning"
	PhaseImplementation Phase = "implementation"
	PhaseVerification   Phase = "verification"
	PhaseArchived       Phase = "archived"
)

// State keeps machine-parsable workflow status for ax.
type State struct {
	Version        string       `json:"version"`
	Phase          Phase        `json:"phase"`
	Current        CurrentRefs  `json:"current"`
	Runtime        RuntimeState `json:"runtime,omitempty"`
	ContextChain   []string     `json:"context_chain,omitempty"`
	TransitionedAt string       `json:"transitioned_at,omitempty"`
	TriggerCommand string       `json:"trigger_command,omitempty"`
	LastResult     LastResult   `json:"last_result,omitempty"`
	LastError      *LastError   `json:"last_error,omitempty"`
	Run            RunState     `json:"run,omitempty"`
	TDD            TDDState     `json:"tdd"`
	Progress       int          `json:"progress"`
}

// CurrentRefs points to active artifacts.
type CurrentRefs struct {
	Proposal string `json:"proposal,omitempty"`
	Plan     string `json:"plan,omitempty"`
	Run      string `json:"run,omitempty"`
	Discover string `json:"discover,omitempty"`
	Verify   string `json:"verify,omitempty"`
	Review   string `json:"review,omitempty"`
	Compound string `json:"compound,omitempty"`
}

// LastResult keeps latest successful command output summary.
type LastResult struct {
	Command  string `json:"command,omitempty"`
	Status   string `json:"status,omitempty"`
	Artifact string `json:"artifact,omitempty"`
	At       string `json:"at,omitempty"`
}

// LastError keeps latest command failure summary.
type LastError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	At      string `json:"at"`
}

// TDDState holds lightweight scaffolding for run --tdd flows.
type TDDState struct {
	Enabled        bool     `json:"enabled"`
	CurrentTier    string   `json:"current_tier,omitempty"`
	CurrentStep    string   `json:"current_step,omitempty"`
	CompletedSteps int      `json:"completed_steps,omitempty"`
	TotalSteps     int      `json:"total_steps,omitempty"`
	Depth          string   `json:"depth,omitempty"`
	DepthSource    string   `json:"depth_source,omitempty"`
	ApprovalPolicy string   `json:"approval_policy,omitempty"`
	Resume         bool     `json:"resume,omitempty"`
	CompletedTiers []string `json:"completed_tiers,omitempty"`
}

// RunState tracks resumable run metadata.
type RunState struct {
	LastFailedStep   string    `json:"last_failed_step,omitempty"`
	LastFailureCause string    `json:"last_failure_cause,omitempty"`
	ErrorCode        string    `json:"error_code,omitempty"`
	ErrorSummary     string    `json:"error_summary,omitempty"`
	RecoverHint      string    `json:"recover_hint,omitempty"`
	RetryAttempts    int       `json:"retry_attempts,omitempty"`
	RetryLimit       int       `json:"retry_limit,omitempty"`
	Blocked          bool      `json:"blocked,omitempty"`
	ThreadID         string    `json:"thread_id,omitempty"`
	ActiveTurnID     string    `json:"active_turn_id,omitempty"`
	EngineMode       string    `json:"engine_mode,omitempty"`
	TurnHistory      []TurnRef `json:"turn_history,omitempty"`
}

// TurnRef tracks one persisted run step↔turn mapping.
type TurnRef struct {
	TurnID    string `json:"turn_id,omitempty"`
	Step      string `json:"step,omitempty"`
	Status    string `json:"status,omitempty"`
	StartedAt string `json:"started_at,omitempty"`
	EndedAt   string `json:"ended_at,omitempty"`
}

var allowedTransitions = map[Phase]map[Phase]struct{}{
	PhaseIdle: {
		PhaseDiscovery: {},
		PhaseProposal:  {},
	},
	PhaseDiscovery: {
		PhaseProposal:     {},
		PhasePlanning:     {},
		PhaseVerification: {},
	},
	PhaseProposal: {
		PhaseDiscovery:    {},
		PhasePlanning:     {},
		PhaseVerification: {},
	},
	PhasePlanning: {
		PhaseDiscovery:      {},
		PhaseImplementation: {},
		PhaseVerification:   {},
	},
	PhaseImplementation: {
		PhaseDiscovery:    {},
		PhasePlanning:     {},
		PhaseVerification: {},
	},
	PhaseVerification: {
		PhaseDiscovery:      {},
		PhaseArchived:       {},
		PhaseImplementation: {},
	},
	PhaseArchived: {
		PhaseDiscovery: {},
		PhaseProposal:  {},
	},
}

// StatePath returns the canonical state path.
func StatePath(base string) string {
	return filepath.Join(base, stateDirName, stateFileName)
}

// DefaultState returns a new initialized state.
func DefaultState(now time.Time) *State {
	return &State{
		Version: "v2",
		Phase:   PhaseIdle,
		Current: CurrentRefs{},
		Runtime: RuntimeState{
			Mode: RuntimeModeSingle,
		},
		TDD:            TDDState{},
		Progress:       ProgressForPhase(PhaseIdle),
		TransitionedAt: now.Format(time.RFC3339),
		TriggerCommand: "state:init",
	}
}

// LoadState loads state.yaml from disk. Legacy tiny YAML inputs are accepted.
func LoadState(base string) (*State, error) {
	path := StatePath(base)
	body, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return DefaultState(time.Now()), nil
	}
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return DefaultState(time.Now()), nil
	}

	var st State
	if jsonErr := json.Unmarshal(body, &st); jsonErr != nil {
		st = legacyToState(trimmed)
	}
	st.normalize()
	return &st, nil
}

func legacyToState(raw string) State {
	st := *DefaultState(time.Now())
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "version:") {
			st.Version = strings.TrimSpace(strings.TrimPrefix(line, "version:"))
		}
		if strings.HasPrefix(line, "phase:") {
			st.Phase = Phase(strings.TrimSpace(strings.TrimPrefix(line, "phase:")))
		}
	}
	return st
}

func (s *State) normalize() {
	if s.Version == "" {
		s.Version = "v2"
	}
	if s.Phase == "" {
		s.Phase = PhaseIdle
	}
	if s.Current == (CurrentRefs{}) {
		s.Current = CurrentRefs{}
	}
	if s.Runtime.Mode == "" {
		s.Runtime.Mode = RuntimeModeSingle
	}
	if s.Runtime.ClusterID == "" {
		s.Runtime.ClusterID = ResolveRuntimeClusterID("")
	}
	if s.Runtime.NodeID == "" {
		s.Runtime.NodeID = ResolveRuntimeNodeID("")
	}
	if s.Runtime.SessionJournal == "" {
		s.Runtime.SessionJournal = filepath.ToSlash(filepath.Join(".ax", "logs", "runtime-journal.jsonl"))
	}
	if s.Runtime.ActiveSessions == nil {
		s.Runtime.ActiveSessions = map[string]string{}
	}
	if s.Runtime.SessionMeta == nil {
		s.Runtime.SessionMeta = map[string]RuntimeSessionRef{}
	}
	if len(s.Runtime.ActiveSessions) > 0 {
		now := time.Now().Format(time.RFC3339)
		for sessionID, command := range s.Runtime.ActiveSessions {
			if _, exists := s.Runtime.SessionMeta[sessionID]; exists {
				continue
			}
			s.Runtime.SessionMeta[sessionID] = RuntimeSessionRef{
				SessionID: sessionID,
				Command:   command,
				Mode:      s.Runtime.Mode,
				ClusterID: s.Runtime.ClusterID,
				NodeID:    s.Runtime.NodeID,
				StartedAt: now,
				UpdatedAt: now,
				Status:    "active",
			}
		}
	}
	if s.ContextChain == nil {
		s.ContextChain = []string{}
	}
	if s.Run.TurnHistory == nil {
		s.Run.TurnHistory = []TurnRef{}
	}
	if s.TransitionedAt == "" {
		s.TransitionedAt = time.Now().Format(time.RFC3339)
	}
	if s.TriggerCommand == "" {
		s.TriggerCommand = "state:normalize"
	}
	s.Progress = ProgressForPhase(s.Phase)
}

// AppendTurnRef appends a turn reference while keeping a bounded history.
func (s *State) AppendTurnRef(ref TurnRef, max int) {
	if s == nil {
		return
	}
	if max <= 0 {
		max = 100
	}
	s.Run.TurnHistory = append(s.Run.TurnHistory, ref)
	if len(s.Run.TurnHistory) > max {
		s.Run.TurnHistory = append([]TurnRef(nil), s.Run.TurnHistory[len(s.Run.TurnHistory)-max:]...)
	}
}

// Transition is a package-level convenience wrapper.
func Transition(st *State, next Phase, trigger string, at time.Time) error {
	if st == nil {
		return fmt.Errorf("ERR_INVALID_PHASE_TRANSITION: nil state")
	}
	return st.Transition(next, trigger, at)
}

// Save persists state in JSON format to state.yaml.
func (s *State) Save(base string) error {
	s.normalize()
	compactionLogPath, err := s.compactContextChain(base, time.Now())
	if err != nil {
		return err
	}
	if compactionLogPath != "" {
		s.addContextPath(compactionLogPath)
		if len(s.ContextChain) > MaxContextChainEntries {
			s.ContextChain = append([]string(nil), s.ContextChain[len(s.ContextChain)-MaxContextChainEntries:]...)
		}
	}
	path := StatePath(base)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return atomicWriteFile(path, body, 0o644)
}

// SaveState is a package-level convenience helper.
func SaveState(base string, st *State) error {
	if st == nil {
		return fmt.Errorf("nil state")
	}
	return st.Save(base)
}

// Transition enforces state-machine phase changes.
func (s *State) Transition(next Phase, trigger string, at time.Time) error {
	s.normalize()
	if next == "" {
		return fmt.Errorf("ERR_INVALID_PHASE_TRANSITION: empty target")
	}
	if s.Phase == next {
		s.TransitionedAt = at.Format(time.RFC3339)
		s.TriggerCommand = trigger
		s.Progress = ProgressForPhase(s.Phase)
		return nil
	}
	// In shared/worktree runtime, global phase is advisory and must not block
	// independent proposal lifecycles.
	if s.Runtime.Mode == RuntimeModeShared || s.Runtime.Mode == RuntimeModeWorktree || s.Runtime.Mode == RuntimeModeAuto {
		s.Phase = next
		s.TransitionedAt = at.Format(time.RFC3339)
		s.TriggerCommand = trigger
		s.Progress = ProgressForPhase(s.Phase)
		return nil
	}
	allowed := allowedTransitions[s.Phase]
	if _, ok := allowed[next]; !ok {
		return fmt.Errorf("ERR_INVALID_PHASE_TRANSITION: %s -> %s", s.Phase, next)
	}
	s.Phase = next
	s.TransitionedAt = at.Format(time.RFC3339)
	s.TriggerCommand = trigger
	s.Progress = ProgressForPhase(s.Phase)
	return nil
}

// ForcePhase directly sets a phase and metadata for controlled restore flows.
func (s *State) ForcePhase(next Phase, trigger string, at time.Time) {
	s.Phase = next
	s.TransitionedAt = at.Format(time.RFC3339)
	s.TriggerCommand = trigger
	s.Progress = ProgressForPhase(next)
}

// AddContext appends a deterministic artifact path to context_chain.
// It returns true when compaction was triggered due to max-chain overflow.
func (s *State) AddContext(path string) bool {
	path = strings.TrimSpace(filepath.ToSlash(path))
	if path == "" {
		return false
	}
	return s.addContextPath(path)
}

func (s *State) addContextPath(path string) bool {
	for _, existing := range s.ContextChain {
		if existing == path {
			return false
		}
	}
	s.ContextChain = append(s.ContextChain, path)
	return len(s.ContextChain) > MaxContextChainEntries
}

func (s *State) compactContextChain(base string, now time.Time) (string, error) {
	if len(s.ContextChain) <= MaxContextChainEntries {
		return "", nil
	}

	before := len(s.ContextChain)
	dropCount := before - contextChainRetainAfterCompaction
	if dropCount <= 0 {
		dropCount = before - MaxContextChainEntries
	}
	if dropCount <= 0 {
		return "", nil
	}
	dropped := append([]string(nil), s.ContextChain[:dropCount]...)
	s.ContextChain = append([]string(nil), s.ContextChain[dropCount:]...)

	logName := fmt.Sprintf("context-compaction-%s.yaml", now.Format("20060102-150405"))
	logRel := filepath.ToSlash(filepath.Join(".ax", "logs", logName))
	logPath := filepath.Join(base, filepath.FromSlash(logRel))
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return "", err
	}

	payload := []string{
		"step: context_compaction",
		"phase: " + string(s.Phase),
		"thread_id: none",
		"turn_id: none",
		"error_code: none",
		"time: " + now.Format(time.RFC3339),
		fmt.Sprintf("context_chain_threshold: %d", MaxContextChainEntries),
		fmt.Sprintf("context_chain_before: %d", before),
		fmt.Sprintf("context_chain_after: %d", len(s.ContextChain)),
		fmt.Sprintf("dropped_count: %d", len(dropped)),
		"dropped_paths:",
	}
	for _, p := range dropped {
		payload = append(payload, "  - "+p)
	}
	payload = append(payload, "")

	if err := os.WriteFile(logPath, []byte(strings.Join(payload, "\n")), 0o644); err != nil {
		return "", err
	}
	return logRel, nil
}

// AddContextRef appends a deterministic context path (metadata currently tracked by state transition fields).
func AddContextRef(s *State, path string, _ Phase, _ time.Time) {
	if s == nil {
		return
	}
	_ = s.AddContext(path)
}

// SetLastResult stores successful command summary.
func (s *State) SetLastResult(command, status, artifact string, at time.Time) {
	s.LastResult = LastResult{
		Command:  command,
		Status:   status,
		Artifact: artifact,
		At:       at.Format(time.RFC3339),
	}
}

// SetLastError stores standardized failure metadata.
func (s *State) SetLastError(code, message string, at time.Time) {
	s.LastError = &LastError{
		Code:    code,
		Message: message,
		At:      at.Format(time.RFC3339),
	}
}

// ClearLastError removes stale failure metadata.
func (s *State) ClearLastError() {
	s.LastError = nil
}

// ProgressForPhase returns a simple deterministic progress percentage.
func ProgressForPhase(phase Phase) int {
	switch phase {
	case PhaseIdle:
		return 0
	case PhaseDiscovery:
		return 15
	case PhaseProposal:
		return 30
	case PhasePlanning:
		return 45
	case PhaseImplementation:
		return 65
	case PhaseVerification:
		return 85
	case PhaseArchived:
		return 100
	default:
		return 0
	}
}

func atomicWriteFile(path string, body []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}
