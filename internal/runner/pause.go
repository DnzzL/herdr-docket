package runner

import (
	"os"
	"path/filepath"

	"github.com/DnzzL/herdr-docket/internal/hostpath"
)

// The fleet-wide pause is a file in the state dir, not a field anywhere: the
// daemon, `herdr-docket run` and the pane are separate processes, it has to
// hold across a daemon restart, and it must not touch the persona files a
// per-agent pause writes, so resuming the fleet never resumes an agent a
// human parked on its own.
//
// Paused stops what the fleet starts by itself — a tick's runs and a
// pipeline's next stage. A run already in flight finishes, and `run` still
// starts one, because that call is human intent.
func pausePath() string { return filepath.Join(hostpath.StateDir(), "paused") }

// Paused reports whether the fleet is paused.
func Paused() bool {
	_, err := os.Stat(pausePath())
	return err == nil
}

// SetPaused pauses (true) or resumes (false) the fleet. Either is idempotent.
func SetPaused(paused bool) error {
	if !paused {
		if err := os.Remove(pausePath()); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(hostpath.StateDir(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(pausePath(), nil, 0o644)
}
