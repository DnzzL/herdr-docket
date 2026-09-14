package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/DnzzL/herdr-docket/internal/hostpath"
)

// acquireLock keeps a single daemon alive per machine. Two daemons would race
// for every task, and a stale one is easy to end up with: the startup hook
// runs again on every Herdr server restart.
//
// The file is created with O_EXCL so two daemons starting in the same instant
// cannot both pass a read-then-write check; a lock whose process is dead is
// removed and the create retried once.
func acquireLock() (release func(), err error) {
	if err := os.MkdirAll(hostpath.StateDir(), 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(hostpath.StateDir(), "daemon.pid")

	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			fmt.Fprint(f, os.Getpid())
			f.Close()
			return func() { os.Remove(path) }, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
		raw, readErr := os.ReadFile(path)
		if readErr == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(raw))); err == nil && alive(pid) && pid != os.Getpid() {
				return nil, fmt.Errorf("another daemon is already running (pid %d)", pid)
			}
		}
		os.Remove(path) // dead owner (or unreadable garbage): reclaim
	}
	return nil, fmt.Errorf("could not claim %s", path)
}

func alive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}
