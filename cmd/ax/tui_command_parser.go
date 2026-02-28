package ax

import (
	"fmt"
	"strings"
)

type tuiCommandKind string

const (
	tuiCommandKindRun       tuiCommandKind = "run"
	tuiCommandKindSteer     tuiCommandKind = "steer"
	tuiCommandKindInterrupt tuiCommandKind = "interrupt"
	tuiCommandKindResume    tuiCommandKind = "resume"
	tuiCommandKindFork      tuiCommandKind = "fork"
	tuiCommandKindRollback  tuiCommandKind = "rollback"
	tuiCommandKindNewThread tuiCommandKind = "new-thread"
	tuiCommandKindUseThread tuiCommandKind = "use-thread"
	tuiCommandKindHelp      tuiCommandKind = "help"
	tuiCommandKindRetry     tuiCommandKind = "retry"
)

type tuiParsedCommand struct {
	Raw         string
	Kind        tuiCommandKind
	Prompt      string
	Instruction string
	Title       string
	ThreadID    string
	TurnID      string
}

func parseTUICommand(raw string) (tuiParsedCommand, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return tuiParsedCommand{}, fmt.Errorf("empty command: enter text or /help")
	}

	if !strings.HasPrefix(trimmed, "/") {
		return tuiParsedCommand{
			Raw:    raw,
			Kind:   tuiCommandKindRun,
			Prompt: trimmed,
		}, nil
	}

	commandName, arg := splitSlashCommand(trimmed)
	switch commandName {
	case "run":
		if strings.TrimSpace(arg) == "" {
			return tuiParsedCommand{}, fmt.Errorf("missing argument: /run <prompt>")
		}
		return tuiParsedCommand{Raw: raw, Kind: tuiCommandKindRun, Prompt: strings.TrimSpace(arg)}, nil
	case "steer":
		if strings.TrimSpace(arg) == "" {
			return tuiParsedCommand{}, fmt.Errorf("missing argument: /steer <instruction>")
		}
		return tuiParsedCommand{Raw: raw, Kind: tuiCommandKindSteer, Instruction: strings.TrimSpace(arg)}, nil
	case "interrupt":
		if strings.TrimSpace(arg) != "" {
			return tuiParsedCommand{}, fmt.Errorf("unexpected argument: /interrupt")
		}
		return tuiParsedCommand{Raw: raw, Kind: tuiCommandKindInterrupt}, nil
	case "resume":
		if strings.TrimSpace(arg) != "" {
			return tuiParsedCommand{}, fmt.Errorf("unexpected argument: /resume")
		}
		return tuiParsedCommand{Raw: raw, Kind: tuiCommandKindResume}, nil
	case "fork":
		if strings.TrimSpace(arg) != "" {
			return tuiParsedCommand{}, fmt.Errorf("unexpected argument: /fork")
		}
		return tuiParsedCommand{Raw: raw, Kind: tuiCommandKindFork}, nil
	case "rollback":
		fields := strings.Fields(arg)
		if len(fields) > 1 {
			return tuiParsedCommand{}, fmt.Errorf("too many arguments: /rollback [turn_id]")
		}
		turnID := ""
		if len(fields) == 1 {
			turnID = strings.TrimSpace(fields[0])
		}
		return tuiParsedCommand{Raw: raw, Kind: tuiCommandKindRollback, TurnID: turnID}, nil
	case "new-thread":
		if strings.TrimSpace(arg) == "" {
			return tuiParsedCommand{}, fmt.Errorf("missing argument: /new-thread <title>")
		}
		return tuiParsedCommand{Raw: raw, Kind: tuiCommandKindNewThread, Title: strings.TrimSpace(arg)}, nil
	case "use-thread":
		if strings.TrimSpace(arg) == "" {
			return tuiParsedCommand{}, fmt.Errorf("missing argument: /use-thread <thread_id>")
		}
		if len(strings.Fields(arg)) > 1 {
			return tuiParsedCommand{}, fmt.Errorf("too many arguments: /use-thread <thread_id>")
		}
		return tuiParsedCommand{Raw: raw, Kind: tuiCommandKindUseThread, ThreadID: strings.TrimSpace(arg)}, nil
	case "help", "h":
		if strings.TrimSpace(arg) != "" {
			return tuiParsedCommand{}, fmt.Errorf("unexpected argument: /help")
		}
		return tuiParsedCommand{Raw: raw, Kind: tuiCommandKindHelp}, nil
	case "retry":
		if strings.TrimSpace(arg) != "" {
			return tuiParsedCommand{}, fmt.Errorf("unexpected argument: /retry")
		}
		return tuiParsedCommand{Raw: raw, Kind: tuiCommandKindRetry}, nil
	case "":
		return tuiParsedCommand{}, fmt.Errorf("unknown command %q (try /help)", strings.TrimSpace(trimmed))
	default:
		return tuiParsedCommand{}, fmt.Errorf("unknown command %q (try /help)", "/"+commandName)
	}
}

func splitSlashCommand(input string) (name, arg string) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(input), "/"))
	if trimmed == "" {
		return "", ""
	}
	idx := strings.IndexAny(trimmed, " \t")
	if idx < 0 {
		return strings.ToLower(strings.TrimSpace(trimmed)), ""
	}
	name = strings.ToLower(strings.TrimSpace(trimmed[:idx]))
	arg = strings.TrimSpace(trimmed[idx+1:])
	return name, arg
}

func renderTUICommandHelp() string {
	return strings.Join([]string{
		"TUI commands:",
		"- <text>: run turn with plain text prompt",
		"- /run <prompt>: run turn explicitly",
		"- /steer <instruction>: steer active/last turn",
		"- /interrupt: interrupt active/last turn",
		"- /resume: resume current thread session",
		"- /fork: fork current thread",
		"- /rollback [turn_id]: rollback to turn",
		"- /retry: retry from last failed run step",
		"- /new-thread <title>: create and focus new thread",
		"- /use-thread <thread_id>: focus an existing thread",
		"- /help: show this help",
	}, "\n")
}
