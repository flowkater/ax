package core

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func TestInspectLocksReportsHeldAndStale(t *testing.T) {
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

	locks, err := InspectLocks(tmp)
	if err != nil {
		t.Fatalf("InspectLocks while held: %v", err)
	}
	if len(locks) != 1 {
		t.Fatalf("expected 1 lock while held, got %d", len(locks))
	}
	if !locks[0].Held || locks[0].Stale {
		t.Fatalf("expected held non-stale lock, got %+v", locks[0])
	}

	close(release)
	if err := <-done; err != nil {
		t.Fatalf("holder error: %v", err)
	}

	lockPath := filepath.Join(tmp, ".ax", "locks", "state.lock")
	body, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("read lock metadata: %v", err)
	}
	meta := map[string]any{}
	if err := json.Unmarshal(body, &meta); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	meta["heartbeat_at"] = "2000-01-01T00:00:00Z"
	body, err = json.MarshalIndent(meta, "", "  ")
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}
	if err := os.WriteFile(lockPath, append(body, '\n'), 0o644); err != nil {
		t.Fatalf("write stale metadata: %v", err)
	}

	locks, err = InspectLocks(tmp)
	if err != nil {
		t.Fatalf("InspectLocks stale: %v", err)
	}
	if len(locks) != 1 {
		t.Fatalf("expected 1 lock metadata entry, got %d", len(locks))
	}
	if locks[0].Held {
		t.Fatalf("expected unlocked lock, got %+v", locks[0])
	}
	if !locks[0].Stale {
		t.Fatalf("expected stale metadata after heartbeat rewind, got %+v", locks[0])
	}
}

func TestReapStaleUnlockedLockRemovesFile(t *testing.T) {
	tmp := t.TempDir()
	lockPath := filepath.Join(tmp, ".ax", "locks", "state.lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatalf("mkdir lock dir: %v", err)
	}
	host, _ := os.Hostname()
	meta := lockMetadata{
		Name:        "state",
		PID:         os.Getpid() + 1000000,
		Hostname:    host,
		AcquiredAt:  "2000-01-01T00:00:00Z",
		HeartbeatAt: "2000-01-01T00:00:00Z",
	}
	body, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}
	if err := os.WriteFile(lockPath, append(body, '\n'), 0o644); err != nil {
		t.Fatalf("write lock metadata: %v", err)
	}

	reaped := reapStaleUnlockedLock(lockPath, host, time.Now().UTC())
	if !reaped {
		t.Fatalf("expected stale lock to be reaped")
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("expected lock file removed, stat err=%v", err)
	}
}

func TestReapStaleUnlockedLockDoesNotRemoveHeldLock(t *testing.T) {
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

	lockPath := filepath.Join(tmp, ".ax", "locks", "state.lock")
	host, _ := os.Hostname()
	reaped := reapStaleUnlockedLock(lockPath, host, time.Now().UTC())
	if reaped {
		t.Fatalf("expected held lock to not be reaped")
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("expected held lock file to exist: %v", err)
	}

	close(release)
	if err := <-done; err != nil {
		t.Fatalf("holder error: %v", err)
	}
}
