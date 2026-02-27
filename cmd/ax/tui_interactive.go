package ax

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

type tuiScreen int

const (
	tuiScreenDashboard tuiScreen = iota
	tuiScreenRuns
	tuiScreenVerify
	tuiScreenArchive
	tuiScreenEngine
)

var tuiScreenOrder = []string{
	"Screen A Dashboard",
	"Screen B Runs",
	"Screen C Verify",
	"Screen D Archive",
	"Screen E Engine",
}

var tuiActionByKey = map[string]string{
	"t": "retry",
	"u": "resume",
	"i": "interrupt",
	"b": "rollback",
	"f": "fork",
	"s": "steer",
}

type tuiTickMsg struct{}

type tuiSnapshotLoadedMsg struct {
	snapshot tuiSnapshot
	err      error
}

type tuiActionDoneMsg struct {
	action string
	err    error
}

type interactiveTUIModel struct {
	base          string
	rt            runtimeContext
	refresh       int
	screen        tuiScreen
	snapshot      tuiSnapshot
	status        string
	pendingAction string
	snapshotFn    func(string, int, time.Time) (tuiSnapshot, error)
	actionFn      func(string, runtimeContext, string, time.Time) error
	nowFn         func() time.Time
}

func newInteractiveTUIModel(base string, rt runtimeContext, refresh int) interactiveTUIModel {
	if refresh <= 0 {
		refresh = 1
	}
	return interactiveTUIModel{
		base:       base,
		rt:         rt,
		refresh:    refresh,
		screen:     tuiScreenDashboard,
		status:     "loading...",
		snapshotFn: buildTUISnapshot,
		actionFn:   executeTUIAction,
		nowFn:      time.Now,
	}
}

func runInteractiveTUI(cmd *cobra.Command, base string, rt runtimeContext, refresh int) error {
	m := newInteractiveTUIModel(base, rt, refresh)
	program := tea.NewProgram(m, tea.WithAltScreen(), tea.WithOutput(cmd.OutOrStdout()))
	_, err := program.Run()
	return err
}

func (m interactiveTUIModel) Init() tea.Cmd {
	return tea.Batch(m.refreshCmd(), m.tickCmd())
}

func (m interactiveTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.updateKey(msg)
	case tuiTickMsg:
		return m, tea.Batch(m.tickCmd(), m.refreshCmd())
	case tuiSnapshotLoadedMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("snapshot error: %v", msg.err)
			return m, nil
		}
		m.snapshot = msg.snapshot
		if strings.TrimSpace(m.pendingAction) == "" {
			m.status = "refreshed"
		}
		return m, nil
	case tuiActionDoneMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("action %s failed: %v", msg.action, msg.err)
			return m, nil
		}
		m.status = fmt.Sprintf("action %s confirmed", msg.action)
		return m, m.refreshCmd()
	}
	return m, nil
}

func (m interactiveTUIModel) View() string {
	var b strings.Builder

	b.WriteString("ax tui (read-only default)\n")
	b.WriteString(fmt.Sprintf("refresh_interval=%ds\n", m.refresh))
	b.WriteString(renderTUITabs(m.screen))
	b.WriteString("\n")

	if strings.TrimSpace(m.pendingAction) != "" {
		b.WriteString(fmt.Sprintf("confirm action '%s'? (y/N)\n", m.pendingAction))
	}
	if strings.TrimSpace(m.status) != "" {
		b.WriteString("status: " + m.status + "\n")
	}
	b.WriteString("\n")

	switch m.screen {
	case tuiScreenDashboard:
		b.WriteString(renderTUIScreenDashboard(m.snapshot))
	case tuiScreenRuns:
		b.WriteString(renderTUIScreenRuns(m.snapshot))
	case tuiScreenVerify:
		b.WriteString(renderTUIScreenVerify(m.snapshot))
	case tuiScreenArchive:
		b.WriteString(renderTUIScreenArchive(m.snapshot))
	case tuiScreenEngine:
		b.WriteString(renderTUIScreenEngine(m.snapshot))
	default:
		b.WriteString(renderTUIScreenDashboard(m.snapshot))
	}

	b.WriteString("\n\nkeys: ")
	b.WriteString(strings.Join(tuiKeyBindingHelp, " | "))
	return b.String()
}

func (m interactiveTUIModel) updateKey(key tea.KeyMsg) (interactiveTUIModel, tea.Cmd) {
	k := strings.ToLower(strings.TrimSpace(key.String()))
	if strings.TrimSpace(m.pendingAction) != "" {
		switch k {
		case "y":
			action := m.pendingAction
			m.pendingAction = ""
			m.status = fmt.Sprintf("running action %s...", action)
			return m, m.actionCmd(action)
		case "n", "esc":
			m.status = "action canceled"
			m.pendingAction = ""
			return m, nil
		}
		return m, nil
	}

	switch k {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "tab":
		m.screen = tuiScreen((int(m.screen) + 1) % len(tuiScreenOrder))
		return m, nil
	case "shift+tab", "backtab":
		m.screen = tuiScreen((int(m.screen) - 1 + len(tuiScreenOrder)) % len(tuiScreenOrder))
		return m, nil
	case "1", "2", "3", "4", "5":
		m.screen = tuiScreen(int(k[0] - '1'))
		return m, nil
	case "r":
		m.status = "refresh requested"
		return m, m.refreshCmd()
	}

	if action, ok := tuiActionByKey[k]; ok {
		m.pendingAction = action
		m.status = fmt.Sprintf("confirm %s", action)
		return m, nil
	}

	return m, nil
}

func (m interactiveTUIModel) tickCmd() tea.Cmd {
	d := time.Duration(m.refresh) * time.Second
	if d <= 0 {
		d = time.Second
	}
	return tea.Tick(d, func(time.Time) tea.Msg {
		return tuiTickMsg{}
	})
}

func (m interactiveTUIModel) refreshCmd() tea.Cmd {
	base := m.base
	refresh := m.refresh
	nowFn := m.nowFn
	snapshotFn := m.snapshotFn
	return func() tea.Msg {
		snap, err := snapshotFn(base, refresh, nowFn())
		return tuiSnapshotLoadedMsg{snapshot: snap, err: err}
	}
}

func (m interactiveTUIModel) actionCmd(action string) tea.Cmd {
	base := m.base
	rt := m.rt
	nowFn := m.nowFn
	actionFn := m.actionFn
	return func() tea.Msg {
		err := actionFn(base, rt, action, nowFn())
		return tuiActionDoneMsg{action: action, err: err}
	}
}

func renderTUITabs(current tuiScreen) string {
	parts := make([]string, 0, len(tuiScreenOrder))
	for i, title := range tuiScreenOrder {
		if i == int(current) {
			parts = append(parts, fmt.Sprintf("[*%d %s*]", i+1, title))
			continue
		}
		parts = append(parts, fmt.Sprintf("[%d %s]", i+1, title))
	}
	return strings.Join(parts, " ")
}

func renderTUIScreenDashboard(snap tuiSnapshot) string {
	var b strings.Builder
	b.WriteString("# Screen A Dashboard\n")
	b.WriteString(fmt.Sprintf("phase: %s\n", emptyFallback(snap.Dashboard.Phase)))
	b.WriteString(fmt.Sprintf("progress: %d\n", snap.Dashboard.Progress))
	b.WriteString(fmt.Sprintf("proposal: %s\n", emptyFallback(snap.Dashboard.Proposal)))
	b.WriteString(fmt.Sprintf("plan: %s\n", emptyFallback(snap.Dashboard.Plan)))
	b.WriteString(fmt.Sprintf("last_result: %s\n", emptyFallback(snap.Dashboard.LastResult)))
	b.WriteString(fmt.Sprintf("last_error: %s\n", emptyFallback(snap.Dashboard.LastError)))
	b.WriteString("recent_logs:\n")
	if len(snap.RuntimeJournal) == 0 {
		b.WriteString("- none\n")
	} else {
		for i, item := range snap.RuntimeJournal {
			if i >= 5 {
				break
			}
			b.WriteString(fmt.Sprintf("- %s %s/%s\n", item.Time, item.Command, item.Stage))
		}
	}
	return b.String()
}

func renderTUIScreenRuns(snap tuiSnapshot) string {
	var b strings.Builder
	b.WriteString("# Screen B Runs\n")
	b.WriteString(fmt.Sprintf("count: %d\n", snap.Runs.Count))
	if len(snap.Runs.Items) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, item := range snap.Runs.Items {
			b.WriteString(fmt.Sprintf("- %s (%s) status=%s\n", item.Name, item.UpdatedAt, emptyFallback(item.StatusLine)))
		}
	}
	b.WriteString(fmt.Sprintf("recover_hint: %s\n", emptyFallback(snap.RecoveryHint)))
	b.WriteString("actions: t=retry, u=resume\n")
	return b.String()
}

func renderTUIScreenVerify(snap tuiSnapshot) string {
	var b strings.Builder
	b.WriteString("# Screen C Verify\n")
	b.WriteString(fmt.Sprintf("verdict: %s\n", emptyFallback(snap.Verify.Verdict)))
	b.WriteString(fmt.Sprintf("criteria: %d\n", snap.Verify.Criteria))
	if len(snap.Verify.FailedChecks) == 0 {
		b.WriteString("failed_checks: none\n")
	} else {
		for _, item := range snap.Verify.FailedChecks {
			b.WriteString("failed_check: " + item + "\n")
		}
	}
	return b.String()
}

func renderTUIScreenArchive(snap tuiSnapshot) string {
	var b strings.Builder
	b.WriteString("# Screen D Archive\n")
	b.WriteString(fmt.Sprintf("count: %d\n", snap.Archive.Count))
	if len(snap.Archive.Items) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, item := range snap.Archive.Items {
			b.WriteString("- " + item + "\n")
		}
	}
	return b.String()
}

func renderTUIScreenEngine(snap tuiSnapshot) string {
	var b strings.Builder
	b.WriteString("# Screen E Engine\n")
	b.WriteString(fmt.Sprintf("runtime_mode: %s\n", emptyFallback(snap.Engine.RuntimeMode)))
	b.WriteString(fmt.Sprintf("session_id: %s\n", emptyFallback(snap.Engine.SessionID)))
	b.WriteString(fmt.Sprintf("cluster_id: %s\n", emptyFallback(snap.Engine.ClusterID)))
	b.WriteString(fmt.Sprintf("node_id: %s\n", emptyFallback(snap.Engine.NodeID)))
	b.WriteString(fmt.Sprintf("thread_id: %s\n", emptyFallback(snap.Engine.ThreadID)))
	b.WriteString(fmt.Sprintf("active_turn_id: %s\n", emptyFallback(snap.Engine.ActiveTurnID)))
	b.WriteString(fmt.Sprintf("turn_count: %d\n", snap.Engine.TurnCount))
	b.WriteString(fmt.Sprintf("codex_mode: %s\n", emptyFallback(snap.Engine.CodexMode)))
	if len(snap.Engine.ActiveSessions) == 0 {
		b.WriteString("active_sessions: none\n")
	} else {
		for sessionID, phase := range snap.Engine.ActiveSessions {
			b.WriteString(fmt.Sprintf("- %s: %s\n", sessionID, phase))
		}
	}
	b.WriteString("actions: i=interrupt, b=rollback, f=fork, s=steer\n")
	return b.String()
}
