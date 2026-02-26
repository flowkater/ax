package core

import (
	"strings"
	"testing"
	"time"
)

func TestWithNamedLockTimesOutWhenHeld(t *testing.T) {
	tmp := t.TempDir()

	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)

	go func() {
		err := WithNamedLock(tmp, "state", time.Second, func() error {
			close(started)
			<-release
			return nil
		})
		done <- err
	}()

	<-started
	err := WithNamedLock(tmp, "state", 100*time.Millisecond, func() error { return nil })
	if err == nil {
		t.Fatal("expected lock timeout error")
	}
	if !strings.Contains(err.Error(), "E_LOCK_TIMEOUT") {
		t.Fatalf("expected E_LOCK_TIMEOUT, got %v", err)
	}

	close(release)
	if err := <-done; err != nil {
		t.Fatalf("holder lock failed: %v", err)
	}
}
