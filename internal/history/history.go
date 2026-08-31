// Package history persists one JSONL record per run under the plugin state
// dir. Append-only: readers reconstruct the latest state per run ID.
package history

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/DnzzL/herdr-fleet/internal/hostpath"
)

type Status string

const (
	StatusScheduled Status = "scheduled"
	StatusRunning   Status = "running"
	StatusDone      Status = "done"
	StatusFailed    Status = "failed"
	StatusSkipped   Status = "skipped"
	// StatusMissed records an occurrence the scheduler could not run at all —
	// typically the machine was asleep past the catch-up window. Recording it
	// is the point: a silently absent run is worse than a visible failure.
	StatusMissed Status = "missed"
	// StatusInvalid records an automation the scheduler could not even read —
	// a broken entry in automations.yaml. It is not a run that failed, it is a
	// run that was never possible, and it belongs in the log for the same
	// reason StatusMissed does: the board is where you look.
	StatusInvalid Status = "invalid"
	// StatusCancelled records a run whose workspace was closed under it.
	// Closing a run's workspace is how you call one off, so it is not a
	// failure: nothing broke, somebody decided.
	StatusCancelled Status = "cancelled"
)

// Trigger is why a run started.
type Trigger string

const (
	// TriggerPoll: the daemon found the task To Do with a known assignee.
	TriggerPoll Trigger = "poll"
	// TriggerManual: somebody asked for it — `r` on the board, or `run`.
	TriggerManual Trigger = "manual"
)

// NewID names a run in the log: the task plus nanoseconds, unique enough for
// a strictly serial worker.
func NewID(task string) string {
	return fmt.Sprintf("%s-%d", task, time.Now().UnixNano())
}

type Record struct {
	RunID       string    `json:"run_id"`
	Task        string    `json:"task"`
	Status      Status    `json:"status"`
	At          time.Time `json:"at"`
	Trigger     Trigger   `json:"trigger,omitempty"`
	WorkspaceID string    `json:"workspace_id,omitempty"`
	PaneID      string    `json:"pane_id,omitempty"`
	Error       string    `json:"error,omitempty"`
}

func path() string { return filepath.Join(hostpath.StateDir(), "history.jsonl") }

// Append writes one record; failures are returned but callers generally just
// log them — history must never break a run.
func Append(r Record) error {
	if err := os.MkdirAll(hostpath.StateDir(), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	line, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(f, string(line))
	return err
}

// Runs returns the latest record per run, newest first, optionally filtered
// by task ID, capped at limit (0 = no cap).
func Runs(task string, limit int) ([]Record, error) {
	f, err := os.Open(path())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	latest := map[string]Record{}
	order := []string{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		var r Record
		if json.Unmarshal(sc.Bytes(), &r) != nil {
			continue // tolerate a torn write rather than losing the file
		}
		if task != "" && r.Task != task {
			continue
		}
		if _, ok := latest[r.RunID]; !ok {
			order = append(order, r.RunID)
		}
		latest[r.RunID] = r
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	out := make([]Record, 0, len(order))
	for i := len(order) - 1; i >= 0; i-- {
		out = append(out, latest[order[i]])
		if limit > 0 && len(out) == limit {
			break
		}
	}
	return out, nil
}

// LastRun returns the most recent record for a task, or nil.
func LastRun(task string) (*Record, error) {
	runs, err := Runs(task, 1)
	if err != nil || len(runs) == 0 {
		return nil, err
	}
	return &runs[0], nil
}
