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

	"github.com/DnzzL/herdr-docket/internal/hostpath"
)

type Status string

const (
	StatusScheduled Status = "scheduled"
	StatusRunning   Status = "running"
	StatusDone      Status = "done"
	StatusFailed    Status = "failed"
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
	// TabID is set when the run borrowed its workspace instead of opening
	// one. The board stops a run from this record and not from a live
	// session, so what the run owns has to be written down: closing the
	// workspace of a borrowed tab ends every run in it.
	TabID string `json:"tab_id,omitempty"`
	Error string `json:"error,omitempty"`
	// Branch is the branch the run was provisioned on — written from the
	// session, never derived later. Commits is how far it moved beyond the
	// repo's own checkout, and Uncommitted says the worktree was about to be
	// torn down holding changes nobody committed: the one delivery fact the
	// fleet checks itself rather than trusting the agent's verdict.
	Branch      string `json:"branch,omitempty"`
	Commits     int    `json:"commits,omitempty"`
	Uncommitted bool   `json:"uncommitted,omitempty"`
	// PullRequest is where the run's work went out as, reported by the
	// agent's own `task close --pr` rather than looked up: the fleet knows
	// nothing of forges.
	PullRequest string `json:"pull_request,omitempty"`
}

// SetPullRequest records the pull request on the task's newest run record —
// a copy of it, PR field set, appended in place. A caller knows the task and
// not its run id, because the only callers are agents closing their own work
// through the CLI.
func SetPullRequest(task, url string) error {
	r, err := LastRun(task)
	if err != nil {
		return err
	}
	if r == nil {
		return fmt.Errorf("no run recorded for %s", task)
	}
	r.PullRequest = url
	return Append(*r)
}

// PullRequestFor returns the pull request recorded on a run, or "". It is how
// the closing record carries forward what the CLI stamped mid-run: the reader
// collapses to the latest record per run, so a closer that did not re-read it
// would silently drop the url behind a verdict.
func PullRequestFor(runID string) (string, error) {
	var pr string
	err := each(func(r Record) {
		if r.RunID == runID && r.PullRequest != "" {
			pr = r.PullRequest
		}
	})
	return pr, err
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
//
// The unit is the run, not the record. The log is append-only and a run owns
// several lines of it — and more than one of them can be a closing line, since
// stamping a pull request on a run that has already closed appends a copy of
// its closing record. Counting lines charged an agent twice for one run, which
// reads as a wrong number and behaves as a smaller budget: OverBudget parks an
// agent that still had spend. So collapse per run first, exactly as Runs does,
// and count what is left.
func UsageSince(now time.Time) map[string]Usage {
	cutoff := now.Add(-Window)
	latest := map[string]Record{}
	// A read failure is not this function's to report: a caller treats an empty
	// window as "no spend recorded", which is the safe default for a budget.
	_ = each(func(r Record) {
		if r.At.Before(cutoff) {
			return
		}
		latest[r.RunID] = r
	})
	usage := map[string]Usage{}
	for _, r := range latest {
		if r.Agent == "" || !r.Status.closes() {
			continue
		}
		u := usage[r.Agent]
		u.Runs++
		u.Minutes += (r.DurationSeconds + 59) / 60
		usage[r.Agent] = u
	}
	return usage
}

// LatestPerTask returns the newest run's latest record for each of ids, in a
// single pass over the log. It answers exactly what LastRun answers, for many
// tasks at once: the board draws a record per row and refreshes on a timer, so
// asking per task walked the whole file per row, and the cost of a refresh
// grew with the fleet's length times its history's — on a log nothing here
// ever truncates. A task with no run is absent from the map rather than nil in
// it, so a caller reads it the way it reads any other miss.
func LatestPerTask(ids []string) map[string]*Record {
	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	// Records arrive in file order, so a run id seen for the first time is the
	// newest run of its task — the same "latest run wins, latest record of it
	// wins" rule Runs applies, said once for every task.
	current := make(map[string]string, len(ids))
	seen := map[string]bool{}
	out := make(map[string]*Record, len(ids))
	_ = each(func(r Record) {
		if !want[r.Task] {
			return
		}
		if !seen[r.RunID] {
			seen[r.RunID] = true
			current[r.Task] = r.RunID
		}
		if current[r.Task] == r.RunID {
			rec := r
			out[r.Task] = &rec
		}
	})
	return out
}

// LastRun returns the most recent record for a task, or nil.
func LastRun(task string) (*Record, error) {
	runs, err := Runs(task, 1)
	if err != nil || len(runs) == 0 {
		return nil, err
	}
	return &runs[0], nil
}
