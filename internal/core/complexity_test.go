package core

import "testing"

func TestResolveDepthHonorsUserOverride(t *testing.T) {
	d, source := ResolveDepth("simple fix", nil, "deep")
	if d != DepthDeep || source != DepthSourceUser {
		t.Fatalf("got %s/%s", d, source)
	}
}

func TestResolveDepthAuto(t *testing.T) {
	d, source := ResolveDepth("typo fix", []string{"cmd/ax/root.go"}, "")
	if d != DepthQuick || source != DepthSourceAuto {
		t.Fatalf("got %s/%s", d, source)
	}
}

func TestShouldEscalateQuick(t *testing.T) {
	if !ShouldEscalateQuick([]string{"cmd/ax/a.go", "internal/codex/b.go"}) {
		t.Fatal("expected cross-module escalation")
	}
	if !ShouldEscalateQuick([]string{"cmd/ax/a.go", "cmd/ax/b.go", "cmd/ax/c.go", "cmd/ax/d.go", "cmd/ax/e.go", "cmd/ax/f.go"}) {
		t.Fatal("expected >5 escalation")
	}
	if ShouldEscalateQuick([]string{"cmd/ax/a.go"}) {
		t.Fatal("did not expect escalation")
	}
}
