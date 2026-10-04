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
	// StatusTimedOut records the moment a run passed its deadline with the
	// agent still working. The run is not over — the fleet keeps the agent
	// and listens for it (ADR 0014) — so this is a mark on the way, not an
	// ending: only done, failed and cancelled close a run.
	StatusTimedOut Status = "timed_out"
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
	// TriggerVerify and TriggerRework are the pipeline's own stages (ADR
	// 0013): the verifier judging a delivered PR, and the author sent back
	// to it with the verifier's notes.
	TriggerVerify Trigger = "verify"
	TriggerRework Trigger = "rework"
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
	// TimedOut marks a run that passed its deadline: the agent kept working
	// past timeout_minutes and the run waited for it. Set on the record
	// written at the deadline and carried on the closing record, so a reader
	// — which collapses to a run's latest record — can still tell a late
	// delivery from an on-time one.
	TimedOut bool `json:"timed_out,omitempty"`
	// Branch is the branch the run was provisioned on — written from the
	// session, never derived later. Commits is how far it moved beyond the
	// repo's own checkout, and Uncommitted says the worktree was about to be
	// torn down holding changes nobody committed: the one delivery fact the
	// fleet checks itself rather than trusting the agent's verdict.
	Branch string `json:"branch,omitempty"`
	// BaseCommit is the commit the run branched from, read at provision time —
	// the base ref's own commit, whatever a human had checked out in the
	// main repo. A PR that carries anything past this commit shows exactly
	// where the surplus came from.
	BaseCommit  string `json:"base_commit,omitempty"`
	Commits     int    `json:"commits,omitempty"`
	Uncommitted bool   `json:"uncommitted,omitempty"`
	// PullRequest is where the run's work went out as, reported by the
	// agent's own `task close --pr` rather than looked up: the fleet knows
	// nothing of forges.
	PullRequest string `json:"pull_request,omitempty"`
	// Verification is a verifier run's verdict on the pull request (PASS or
	// FAIL), stamped by `task verdict`, and PatchID the `git patch-id` of the
	// diff it judged: the merge gate trusts a verdict only on that diff.
	Verification string `json:"verification,omitempty"`
	PatchID      string `json:"patch_id,omitempty"`
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

// TaskPullRequest returns the pull request most recently stamped on the
// task, or "". PullRequestFor answers what one run delivered; this answers
// what the task still has out — a verifier run failing on the author's PR
// records no PR of its own, and the PR an earlier run opened stays open
// whatever the runs after it record.
func TaskPullRequest(task string) (string, error) {
	var pr string
	err := each(func(r Record) {
		if r.Task == task && r.PullRequest != "" {
			pr = r.PullRequest
		}
	})
	return pr, err
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

// SetVerification records a verifier's verdict on the task's newest run —
// the verifier's own, since it is the run calling.
func SetVerification(task, verdict, patchID string) error {
	r, err := LastRun(task)
	if err != nil {
		return err
	}
	if r == nil {
		return fmt.Errorf("no run recorded for %s", task)
	}
	r.Verification, r.PatchID = verdict, patchID
	return Append(*r)
}

// VerificationFor returns the verdict and patch-id recorded on a run, or
// empty strings, for the same reason PullRequestFor exists.
func VerificationFor(runID string) (verdict, patchID string, err error) {
	err = each(func(r Record) {
		if r.RunID == runID && r.Verification != "" {
			verdict, patchID = r.Verification, r.PatchID
		}
	})
	return verdict, patchID, err
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
