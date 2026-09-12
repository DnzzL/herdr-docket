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
	// StatusCancelled records a run whose workspace was closed under it.
	// Closing a run's workspace is how you call one off, so it is not a
	// failure: nothing broke, somebody decided.
	StatusCancelled Status = "cancelled"
)

// closes reports whether a status is a run's final one. Only these records
// carry a duration and a verdict — and only these count against a budget.
func (s Status) closes() bool {
	switch s {
	case StatusDone, StatusFailed, StatusCancelled:
		return true
	}
	return false
}

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
	RunID string `json:"run_id"`
	Task  string `json:"task"`
	// Agent is the fleet agent the run used. Empty on records written before
	// the fleet started recording it, which is why a budget ignores those.
	Agent   string    `json:"agent,omitempty"`
	Status  Status    `json:"status"`
	At      time.Time `json:"at"`
	Trigger Trigger   `json:"trigger,omitempty"`
	// DurationSeconds is how long the run took, on the closing record only.
	DurationSeconds int `json:"duration_seconds,omitempty"`
	// Verdict is how the run ended its task (done, failed, blocked), on the
	// closing record only and best-effort: a mechanics failure writes none.
	Verdict     string `json:"verdict,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	PaneID      string `json:"pane_id,omitempty"`
	Error       string `json:"error,omitempty"`
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

// each calls fn for every record on disk, in file order. A torn or
// unreadable line is skipped rather than losing the whole file: history is an
// audit trail, and a reader that gives up on one bad byte is worse than one
// that reads the rest.
func each(fn func(Record)) error {
	f, err := os.Open(path())
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		var r Record
		if json.Unmarshal(sc.Bytes(), &r) != nil {
			continue // tolerate a torn write rather than losing the file
		}
		fn(r)
	}
	return sc.Err()
}

// Runs returns the latest record per run, newest first, optionally filtered
// by task ID, capped at limit (0 = no cap).
func Runs(task string, limit int) ([]Record, error) {
	latest := map[string]Record{}
	order := []string{}
	err := each(func(r Record) {
		if task != "" && r.Task != task {
			return
		}
		if _, ok := latest[r.RunID]; !ok {
			order = append(order, r.RunID)
		}
		latest[r.RunID] = r
	})
	if err != nil {
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

// Usage is what one agent has spent in a rolling window: completed runs and
// minutes, with each run's duration rounded up to a whole minute.
type Usage struct {
	Runs    int
	Minutes int
}

// Window is how far back a budget looks. A rolling day, not a calendar one:
// "today" would reset at midnight and let a self-tasking agent spend the whole
// allowance twice in a minute around the boundary.
const Window = 24 * time.Hour

// UsageSince totals each agent's completed runs over the window ending at
// now. Records written before the fleet recorded an agent or a duration are
// ignored: an honest zero beats a guessed one, and a budget must never be
// spent against a number the log never carried.
func UsageSince(now time.Time) map[string]Usage {
	cutoff := now.Add(-Window)
	usage := map[string]Usage{}
	// A read failure is not this function's to report: a caller treats an empty
	// window as "no spend recorded", which is the safe default for a budget.
	_ = each(func(r Record) {
		if r.Agent == "" || !r.Status.closes() || r.At.Before(cutoff) {
			return
		}
		u := usage[r.Agent]
		u.Runs++
		u.Minutes += (r.DurationSeconds + 59) / 60
		usage[r.Agent] = u
	})
	return usage
}

// LastRun returns the most recent record for a task, or nil.
func LastRun(task string) (*Record, error) {
	runs, err := Runs(task, 1)
	if err != nil || len(runs) == 0 {
		return nil, err
	}
	return &runs[0], nil
}
