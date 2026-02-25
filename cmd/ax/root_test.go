package ax

import "testing"

func TestRootRegistersMVPCommands(t *testing.T) {
	root := NewRootCmd()

	want := []string{"propose", "plan", "run", "verify", "archive", "discover", "quick", "state"}
	for _, name := range want {
		if _, _, err := root.Find([]string{name}); err != nil {
			t.Fatalf("expected command %q to be registered: %v", name, err)
		}
	}
}
