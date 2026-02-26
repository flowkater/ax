package core

import (
	"testing"
	"time"
)

func TestParseRuntimeMode(t *testing.T) {
	for _, tc := range []struct {
		in      string
		want    RuntimeMode
		wantErr bool
	}{
		{"", RuntimeModeSingle, false},
		{"single", RuntimeModeSingle, false},
		{"shared", RuntimeModeShared, false},
		{"worktree", RuntimeModeWorktree, false},
		{"auto", RuntimeModeAuto, false},
		{"invalid", "", true},
	} {
		got, err := ParseRuntimeMode(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("expected error for mode %q", tc.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("parse mode %q: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("mode %q => %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestResolveRuntimeModeAutoToShared(t *testing.T) {
	got, err := ResolveRuntimeMode("auto")
	if err != nil {
		t.Fatalf("resolve auto: %v", err)
	}
	if got != RuntimeModeShared {
		t.Fatalf("expected auto=>shared, got %s", got)
	}
}

func TestNewSessionID(t *testing.T) {
	id := NewSessionID(time.Unix(1700000000, 0))
	if id == "" {
		t.Fatal("expected non-empty session id")
	}
}
