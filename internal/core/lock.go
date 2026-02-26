package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const (
	defaultStateLockTimeout = 30 * time.Second
	stateLockPollInterval   = 25 * time.Millisecond
)

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
	return fn()
}
