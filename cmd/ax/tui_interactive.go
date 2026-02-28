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
	"ctrl+t": "retry",
	"ctrl+u": "resume",
	"ctrl+x": "interrupt",
	"ctrl+b": "rollback",
	"ctrl+f": "fork",
	"ctrl+s": "steer",
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

type tuiCommandDoneMsg struct {
	line   string
	result tuiCommandResult
	err    error
}

type tuiCommandStreamDeltaMsg struct {
	entry tuiTranscriptEntry
}

type tuiCommandAsyncMsg struct {
	stream <-chan tea.Msg
	inner  tea.Msg
}

type interactiveTUIModel struct {
	base           string
	rt             runtimeContext
	refresh        int
	screen         tuiScreen
	snapshot       tuiSnapshot
	status         string
	pendingAction  string
	inputBuffer    []rune
	inputCursor    int
	commandHistory []string
	historyIndex   int
	transcript     []tuiTranscriptEntry
	maxTranscript  int
	lastLatencyMS  int64
	snapshotFn     func(string, int, time.Time) (tuiSnapshot, error)
	actionFn       func(string, runtimeContext, string, time.Time) error
	commandFn      func(string, runtimeContext, string, time.Time) (tuiCommandResult, error)
	commandEventFn func(string, runtimeContext, string, time.Time, tuiCommandEventCallback) (tuiCommandResult, error)
	nowFn          func() time.Time
}

func newInteractiveTUIModel(base string, rt runtimeContext, refresh int) interactiveTUIModel {
	if refresh <= 0 {
		refresh = 1
	}
	history, err := loadTUIInputHistory(base, rt.sessionID, 200)
	if err != nil {
		history = []string{}
	}
	return interactiveTUIModel{
		base:           base,
		rt:             rt,
		refresh:        refresh,
		screen:         tuiScreenDashboard,
		status:         "loading...",
		historyIndex:   -1,
		commandHistory: history,
		maxTranscript:  200,
		snapshotFn:     buildTUISnapshot,
		actionFn:       executeTUIAction,
		commandFn:      executeTUICommand,
		commandEventFn: executeTUICommandWithEvents,
		nowFn:          time.Now,
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
	case tuiCommandAsyncMsg:
		switch inner := msg.inner.(type) {
		case tuiCommandStreamDeltaMsg:
			m.appendStreamingDelta(inner.entry)
			if strings.TrimSpace(inner.entry.Content) != "" {
				m.status = "streaming response..."
			}
			return m, waitTUICommandAsync(msg.stream)
		case tuiCommandDoneMsg:
			return m.handleCommandDone(inner)
		case nil:
			return m, nil
		default:
			return m, waitTUICommandAsync(msg.stream)
		}
	case tuiCommandStreamDeltaMsg:
		m.appendStreamingDelta(msg.entry)
		if strings.TrimSpace(msg.entry.Content) != "" {
			m.status = "streaming response..."
		}
		return m, nil
	case tuiCommandDoneMsg:
		return m.handleCommandDone(msg)
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

	b.WriteString("\n")
	b.WriteString(renderTUITranscriptPane(m.transcript))
	b.WriteString("\n")
	b.WriteString(renderTUIStatusLine(m.snapshot, m.status, m.lastLatencyMS))
	b.WriteString("\n")
	b.WriteString(renderTUIInputLine(m.inputBuffer, m.inputCursor))
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

	if k == "ctrl+c" {
		return m, tea.Quit
	}

	if key.Type == tea.KeyEnter {
		line := strings.TrimSpace(string(m.inputBuffer))
		if line == "" {
			return m, nil
		}
		m.appendTranscript(tuiTranscriptEntry{
			Time:    m.nowFn().UTC().Format(time.RFC3339),
			Role:    tuiTranscriptRoleUser,
			Content: line,
		})
		m.commandHistory = append(m.commandHistory, line)
		m.historyIndex = -1
		m.inputBuffer = nil
		m.inputCursor = 0
		m.status = "running command..."
		return m, m.commandCmd(line)
	}

	switch key.Type {
	case tea.KeyUp:
		if len(m.commandHistory) == 0 {
			return m, nil
		}
		if m.historyIndex < 0 {
			m.historyIndex = len(m.commandHistory) - 1
		} else if m.historyIndex > 0 {
			m.historyIndex--
		}
		m.setInputBuffer(m.commandHistory[m.historyIndex])
		return m, nil
	case tea.KeyDown:
		if len(m.commandHistory) == 0 || m.historyIndex < 0 {
			return m, nil
		}
		if m.historyIndex >= len(m.commandHistory)-1 {
			m.historyIndex = -1
			m.setInputBuffer("")
			return m, nil
		}
		m.historyIndex++
		m.setInputBuffer(m.commandHistory[m.historyIndex])
		return m, nil
	case tea.KeyBackspace:
		if m.inputCursor <= 0 || len(m.inputBuffer) == 0 {
			return m, nil
		}
		m.inputBuffer = append(m.inputBuffer[:m.inputCursor-1], m.inputBuffer[m.inputCursor:]...)
		m.inputCursor--
		return m, nil
	case tea.KeyDelete:
		if m.inputCursor >= len(m.inputBuffer) || len(m.inputBuffer) == 0 {
			return m, nil
		}
		m.inputBuffer = append(m.inputBuffer[:m.inputCursor], m.inputBuffer[m.inputCursor+1:]...)
		return m, nil
	case tea.KeyLeft:
		if m.inputCursor > 0 {
			m.inputCursor--
		}
		return m, nil
	case tea.KeyRight:
		if m.inputCursor < len(m.inputBuffer) {
			m.inputCursor++
		}
		return m, nil
	case tea.KeyHome:
		m.inputCursor = 0
		return m, nil
	case tea.KeyEnd:
		m.inputCursor = len(m.inputBuffer)
		return m, nil
	}

	if len(m.inputBuffer) == 0 {
		switch k {
		case "tab":
			m.screen = tuiScreen((int(m.screen) + 1) % len(tuiScreenOrder))
			return m, nil
		case "shift+tab", "backtab":
			m.screen = tuiScreen((int(m.screen) - 1 + len(tuiScreenOrder)) % len(tuiScreenOrder))
			return m, nil
		case "ctrl+r":
			m.status = "refresh requested"
			return m, m.refreshCmd()
		}

		if action, ok := tuiActionByKey[k]; ok {
			m.pendingAction = action
			m.status = fmt.Sprintf("confirm %s", action)
			return m, nil
		}
	}

	if key.Type == tea.KeyRunes && len(key.Runes) > 0 {
		m.insertRunes(key.Runes)
		return m, nil
	}

	return m, nil
}

func (m *interactiveTUIModel) insertRunes(rs []rune) {
	if len(rs) == 0 {
		return
	}
	if m.inputCursor < 0 {
		m.inputCursor = 0
	}
	if m.inputCursor > len(m.inputBuffer) {
		m.inputCursor = len(m.inputBuffer)
	}
	left := append([]rune(nil), m.inputBuffer[:m.inputCursor]...)
	right := append([]rune(nil), m.inputBuffer[m.inputCursor:]...)
	left = append(left, rs...)
	m.inputBuffer = append(left, right...)
	m.inputCursor += len(rs)
}

func (m *interactiveTUIModel) setInputBuffer(line string) {
	m.inputBuffer = []rune(line)
	m.inputCursor = len(m.inputBuffer)
}

func (m *interactiveTUIModel) appendTranscript(entries ...tuiTranscriptEntry) {
	if len(entries) == 0 {
		return
	}
	m.transcript = append(m.transcript, entries...)
	limit := m.maxTranscript
	if limit <= 0 {
		limit = 200
	}
	if len(m.transcript) > limit {
		m.transcript = append([]tuiTranscriptEntry(nil), m.transcript[len(m.transcript)-limit:]...)
	}
}

func (m interactiveTUIModel) handleCommandDone(msg tuiCommandDoneMsg) (interactiveTUIModel, tea.Cmd) {
	if msg.err != nil {
		m.status = fmt.Sprintf("command failed: %v", msg.err)
		m.appendTranscript(tuiTranscriptEntry{
			Time:    m.nowFn().UTC().Format(time.RFC3339),
			Role:    tuiTranscriptRoleError,
			Content: msg.err.Error(),
		})
		return m, nil
	}
	m.lastLatencyMS = msg.result.DurationMS
	if strings.TrimSpace(msg.result.Status) != "" {
		m.status = msg.result.Status
	} else {
		m.status = "command completed"
	}
	m.appendTranscript(msg.result.Transcript...)
	return m, m.refreshCmd()
}

func (m *interactiveTUIModel) appendStreamingDelta(entry tuiTranscriptEntry) {
	if strings.TrimSpace(entry.Role) == "" {
		entry.Role = tuiTranscriptRoleAssistant
	}
	if strings.TrimSpace(entry.Time) == "" {
		entry.Time = time.Now().Format(time.RFC3339)
	}
	if len(m.transcript) > 0 {
		lastIdx := len(m.transcript) - 1
		last := &m.transcript[lastIdx]
		if strings.TrimSpace(last.Role) == tuiTranscriptRoleAssistant &&
			strings.TrimSpace(last.ThreadID) == strings.TrimSpace(entry.ThreadID) &&
			strings.TrimSpace(last.TurnID) == strings.TrimSpace(entry.TurnID) {
			last.Content += entry.Content
			return
		}
	}
	m.appendTranscript(entry)
}

func (m interactiveTUIModel) commandCmd(line string) tea.Cmd {
	base := m.base
	rt := m.rt
	nowFn := m.nowFn
	commandFn := m.commandFn
	commandEventFn := m.commandEventFn
	return func() tea.Msg {
		if commandEventFn != nil {
			stream := make(chan tea.Msg, 32)
			go func() {
				defer close(stream)
				result, err := commandEventFn(base, rt, line, nowFn(), func(entry tuiTranscriptEntry) {
					stream <- tuiCommandStreamDeltaMsg{entry: entry}
				})
				stream <- tuiCommandDoneMsg{line: line, result: result, err: err}
			}()
			first, ok := <-stream
			if !ok {
				return tuiCommandDoneMsg{line: line, err: fmt.Errorf("command stream closed unexpectedly")}
			}
			return tuiCommandAsyncMsg{stream: stream, inner: first}
		}
		result, err := commandFn(base, rt, line, nowFn())
		return tuiCommandDoneMsg{line: line, result: result, err: err}
	}
}

func waitTUICommandAsync(stream <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-stream
		if !ok {
			return nil
		}
		return tuiCommandAsyncMsg{stream: stream, inner: msg}
	}
}

func renderTUIInputLine(buf []rune, cursor int) string {
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(buf) {
		cursor = len(buf)
	}
	var b strings.Builder
	b.WriteString("command> ")
	b.WriteString(string(buf[:cursor]))
	b.WriteRune('|')
	b.WriteString(string(buf[cursor:]))
	return b.String()
}

func renderTUITranscriptPane(entries []tuiTranscriptEntry) string {
	var b strings.Builder
	b.WriteString("# Transcript\n")
	if len(entries) == 0 {
		b.WriteString("- none\n")
		return b.String()
	}
	start := 0
	if len(entries) > 12 {
		start = len(entries) - 12
	}
	for _, entry := range entries[start:] {
		content := strings.ReplaceAll(strings.TrimSpace(entry.Content), "\n", " ")
		if content == "" {
			content = "(empty)"
		}
		line := fmt.Sprintf("- [%s] %s", strings.ToUpper(strings.TrimSpace(entry.Role)), content)
		if threadID := strings.TrimSpace(entry.ThreadID); threadID != "" {
			line += " thread=" + threadID
		}
		if turnID := strings.TrimSpace(entry.TurnID); turnID != "" {
			line += " turn=" + turnID
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

func renderTUIStatusLine(snap tuiSnapshot, status string, latencyMS int64) string {
	return fmt.Sprintf(
		"statusline: session=%s thread=%s turn=%s mode=%s latency_ms=%d status=%s",
		emptyFallback(strings.TrimSpace(snap.Engine.SessionID)),
		emptyFallback(strings.TrimSpace(snap.Engine.ThreadID)),
		emptyFallback(strings.TrimSpace(snap.Engine.ActiveTurnID)),
		emptyFallback(strings.TrimSpace(snap.Engine.CodexMode)),
		latencyMS,
		emptyFallback(strings.TrimSpace(status)),
	)
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
	b.WriteString("actions: ctrl+t=retry, ctrl+u=resume\n")
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
	b.WriteString("actions: ctrl+x=interrupt, ctrl+b=rollback, ctrl+f=fork, ctrl+s=steer\n")
	return b.String()
}
