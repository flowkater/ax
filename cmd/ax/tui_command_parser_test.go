package ax

import "testing"

func TestParseTUICommandPlainText(t *testing.T) {
	cmd, err := parseTUICommand("implement this task")
	if err != nil {
		t.Fatalf("parse plain text: %v", err)
	}
	if cmd.Kind != tuiCommandKindRun {
		t.Fatalf("expected run kind, got %q", cmd.Kind)
	}
	if cmd.Prompt != "implement this task" {
		t.Fatalf("unexpected prompt: %q", cmd.Prompt)
	}
}

func TestParseTUICommandSlashCommands(t *testing.T) {
	cases := []struct {
		name string
		line string
		kind tuiCommandKind
	}{
		{name: "run", line: "/run ship it", kind: tuiCommandKindRun},
		{name: "steer", line: "/steer tighten tests", kind: tuiCommandKindSteer},
		{name: "interrupt", line: "/interrupt", kind: tuiCommandKindInterrupt},
		{name: "resume", line: "/resume", kind: tuiCommandKindResume},
		{name: "fork", line: "/fork", kind: tuiCommandKindFork},
		{name: "rollback", line: "/rollback turn-1", kind: tuiCommandKindRollback},
		{name: "new-thread", line: "/new-thread Feature Work", kind: tuiCommandKindNewThread},
		{name: "use-thread", line: "/use-thread th-123", kind: tuiCommandKindUseThread},
		{name: "help", line: "/help", kind: tuiCommandKindHelp},
		{name: "retry", line: "/retry", kind: tuiCommandKindRetry},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, err := parseTUICommand(tc.line)
			if err != nil {
				t.Fatalf("parse %q: %v", tc.line, err)
			}
			if cmd.Kind != tc.kind {
				t.Fatalf("kind mismatch for %q: got=%q want=%q", tc.line, cmd.Kind, tc.kind)
			}
		})
	}
}

func TestParseTUICommandErrors(t *testing.T) {
	cases := []string{
		"",
		"/unknown",
		"/run",
		"/steer",
		"/interrupt now",
		"/use-thread",
		"/rollback a b",
	}
	for _, line := range cases {
		t.Run(line, func(t *testing.T) {
			if _, err := parseTUICommand(line); err == nil {
				t.Fatalf("expected parse failure for %q", line)
			}
		})
	}
}
