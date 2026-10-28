package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/DnzzL/herdr-docket/internal/hostpath"
)

// The heartbeat is the time of the daemon's last tick, in a file of the state
// dir: the daemon and every view of it (status, the pane, questions) are
// separate processes, and a pid alone cannot tell a live daemon from a hung one.
func heartbeatPath() string { return filepath.Join(hostpath.StateDir(), "heartbeat") }

// beat records a tick at at.
func beat(at time.Time) error {
	if err := os.MkdirAll(hostpath.StateDir(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(heartbeatPath(), []byte(strconv.FormatInt(at.Unix(), 10)+"\n"), 0o644)
}

// Liveness is the one-line answer to "is the daemon working?" at now:
// `daemon live · tick 3s ago`, `daemon down for 4 min` once the last tick is
// older than three tick intervals, `daemon never started`, or `daemon unknown ·
// heartbeat unreadable` when the file holds no time.
func Liveness(now time.Time) string {
	raw, err := os.ReadFile(heartbeatPath())
	if err != nil {
		return "daemon never started"
	}
	sec, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil {
		return "daemon unknown · heartbeat unreadable"
	}
	age := now.Sub(time.Unix(sec, 0))
	if age > 3*tickInterval {
		return "daemon down for " + ago(age)
	}
	return "daemon live · tick " + ago(age) + " ago"
}

// ago is an age a phone can read at a glance: seconds, minutes, then hours.
func ago(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", max(0, int(d.Seconds())))
	case d < time.Hour:
		return fmt.Sprintf("%d min", int(d.Minutes()))
	default:
		return fmt.Sprintf("%d h", int(d.Hours()))
	}
}
