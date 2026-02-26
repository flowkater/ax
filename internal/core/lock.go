package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	defaultStateLockTimeout = 30 * time.Second
	stateLockPollInterval   = 25 * time.Millisecond
	lockHeartbeatInterval   = 5 * time.Second
	lockStaleAfter          = 30 * time.Second
)

type lockMetadata struct {
	Name        string `json:"name"`
	PID         int    `json:"pid"`
	Hostname    string `json:"hostname"`
	AcquiredAt  string `json:"acquired_at"`
	HeartbeatAt string `json:"heartbeat_at"`
	ReleasedAt  string `json:"released_at,omitempty"`
}

// LockInfo is a debug-friendly lock status summary.
type LockInfo struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	PID         int    `json:"pid,omitempty"`
	Hostname    string `json:"hostname,omitempty"`
	AcquiredAt  string `json:"acquired_at,omitempty"`
	HeartbeatAt string `json:"heartbeat_at,omitempty"`
	Held        bool   `json:"held"`
	Stale       bool   `json:"stale"`
}

// WithStateLock serializes mutating command flows for shared runtime safety.
func WithStateLock(base string, fn func() error) error {
	return WithNamedLock(base, "state", defaultStateLockTimeout, fn)
}

// WithNamedLock acquires an exclusive file lock for the duration of fn.
func WithNamedLock(base, name string, timeout time.Duration, fn func() error) error {
	if timeout <= 0 {
		timeout = defaultStateLockTimeout
	}
	lockPath := filepath.Join(base, stateDirName, "locks", name+".lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	deadline := time.Now().Add(timeout)
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("E_LOCK_TIMEOUT: state lock wait exceeded %s", timeout)
		}
		time.Sleep(stateLockPollInterval)
	}
	defer func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	}()

	hostname, _ := os.Hostname()
	acquired := time.Now().UTC()
	meta := lockMetadata{
		Name:        name,
		PID:         os.Getpid(),
		Hostname:    hostname,
		AcquiredAt:  acquired.Format(time.RFC3339),
		HeartbeatAt: acquired.Format(time.RFC3339),
	}
	_ = writeLockMetadata(lockPath, meta)

	stopHeartbeat := make(chan struct{})
	defer close(stopHeartbeat)
	go func() {
		ticker := time.NewTicker(lockHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				meta.HeartbeatAt = time.Now().UTC().Format(time.RFC3339)
				_ = writeLockMetadata(lockPath, meta)
			case <-stopHeartbeat:
				return
			}
		}
	}()

	defer func() {
		meta.HeartbeatAt = time.Now().UTC().Format(time.RFC3339)
		meta.ReleasedAt = meta.HeartbeatAt
		_ = writeLockMetadata(lockPath, meta)
	}()

	return fn()
}

func writeLockMetadata(path string, meta lockMetadata) error {
	body, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return os.WriteFile(path, body, 0o644)
}

func readLockMetadata(path string) (lockMetadata, bool) {
	body, err := os.ReadFile(path)
	if err != nil {
		return lockMetadata{}, false
	}
	var meta lockMetadata
	if err := json.Unmarshal(body, &meta); err != nil {
		return lockMetadata{}, false
	}
	return meta, true
}

func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func isLockHeld(path string) bool {
	f, err := os.OpenFile(path, os.O_RDWR, 0o644)
	if err != nil {
		return false
	}
	defer f.Close()
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == nil {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		return false
	}
	return errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN)
}

// InspectLocks returns status summaries for lock files in .ax/locks.
func InspectLocks(base string) ([]LockInfo, error) {
	lockDir := filepath.Join(base, stateDirName, "locks")
	entries, err := os.ReadDir(lockDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []LockInfo{}, nil
		}
		return nil, err
	}

	hostname, _ := os.Hostname()
	now := time.Now().UTC()
	out := make([]LockInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".lock") {
			continue
		}
		path := filepath.Join(lockDir, entry.Name())
		held := isLockHeld(path)
		meta, ok := readLockMetadata(path)
		info := LockInfo{
			Name: strings.TrimSuffix(entry.Name(), ".lock"),
			Path: filepath.ToSlash(filepath.Join(".ax", "locks", entry.Name())),
			Held: held,
		}
		if ok {
			info.PID = meta.PID
			info.Hostname = meta.Hostname
			info.AcquiredAt = meta.AcquiredAt
			info.HeartbeatAt = meta.HeartbeatAt

			staleByHeartbeat := false
			if hb, err := time.Parse(time.RFC3339, meta.HeartbeatAt); err == nil {
				staleByHeartbeat = now.Sub(hb) > lockStaleAfter
			}
			staleByDeadPID := meta.Hostname == hostname && meta.PID > 0 && !isProcessAlive(meta.PID)
			info.Stale = staleByDeadPID || staleByHeartbeat
			if held {
				info.Stale = false
			}
		}
		out = append(out, info)
	}
	return out, nil
}
