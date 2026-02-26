package ax

import "testing"

func TestRootRegistersCommands(t *testing.T) {
	root := NewRootCmd()
	if !root.SilenceUsage {
		t.Fatal("expected SilenceUsage=true for normalized error output")
	}

	want := []string{"propose", "plan", "run", "verify", "archive", "discover", "quick", "state", "review", "compound"}
	for _, name := range want {
		if _, _, err := root.Find([]string{name}); err != nil {
			t.Fatalf("expected command %q to be registered: %v", name, err)
		}
	}
}
