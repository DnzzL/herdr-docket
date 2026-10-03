package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunsCollapsesToLatestState(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	base := time.Now()
	steps := []Record{
		{RunID: "r1", Task: "triage", Status: StatusScheduled, At: base},
		{RunID: "r1", Task: "triage", Status: StatusRunning, At: base.Add(time.Second)},
		{RunID: "r1", Task: "triage", Status: StatusDone, At: base.Add(time.Minute)},
		{RunID: "r2", Task: "other", Status: StatusFailed, At: base.Add(2 * time.Minute), Error: "boom"},
	}
	for _, r := range steps {
		if err := Append(r); err != nil {
			t.Fatal(err)
		}
	}

	runs, err := Runs("", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(runs))
	}
	if runs[0].RunID != "r2" || runs[0].Status != StatusFailed {
		t.Fatalf("newest first expected, got %+v", runs[0])
	}
	if runs[1].Status != StatusDone {
		t.Fatalf("r1 should collapse to done, got %s", runs[1].Status)
	}

	last, err := LastRun("triage")
	if err != nil || last == nil || last.RunID != "r1" {
		t.Fatalf("LastRun(triage) = %+v, %v", last, err)
	}
}

// The closing record is the one that carries how long the run took and how it
// ended, so the audit trail answers both questions without reconstruction.
func TestClosingRecordCarriesDurationAndVerdict(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	if err := Append(Record{
		RunID: "r1", Task: "TASK-1", Agent: "dev", Status: StatusDone,
		At: time.Now(), DurationSeconds: 90, Verdict: "done",
	}); err != nil {
		t.Fatal(err)
	}
	r, err := LastRun("TASK-1")
	if err != nil || r == nil {
		t.Fatal(err)
	}
	if r.DurationSeconds != 90 || r.Verdict != "done" || r.Agent != "dev" {
		t.Fatalf("closing record = %+v", r)
	}
}

// Append-only means a record written before these fields existed must still
// load: the fields read as zero and empty, never as an error.
func TestRecordsFromBeforeTheNewFieldsStillLoad(t *testing.T) {
	state := t.TempDir()
	t.Setenv("HERDR_PLUGIN_STATE_DIR", state)
	old := `{"run_id":"old","task":"TASK-1","status":"done","at":"2026-09-01T10:00:00Z","trigger":"poll"}` + "\n"
	if err := os.WriteFile(filepath.Join(state, "history.jsonl"), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := LastRun("TASK-1")
	if err != nil || r == nil {
		t.Fatalf("an old record must still load: %v", err)
	}
	if r.DurationSeconds != 0 || r.Verdict != "" || r.Agent != "" {
		t.Fatalf("absent fields must read as zero/empty, got %+v", r)
	}
}

// The delivery record is the fleet's own evidence about a run — what branch
// it produced, how far that branch moved, and whether the worktree was about
// to be destroyed holding uncommitted changes. It lives on the closing record
// beside the verdict it qualifies.
func TestClosingRecordCarriesTheDelivery(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	if err := Append(Record{
		RunID: "r1", Task: "TASK-1", Status: StatusDone, At: time.Now(),
		Branch: "fleet/x-1", Commits: 3, Uncommitted: true, Verdict: "done",
	}); err != nil {
		t.Fatal(err)
	}
	r, err := LastRun("TASK-1")
	if err != nil || r == nil {
		t.Fatal(err)
	}
	if r.Branch != "fleet/x-1" || r.Commits != 3 || !r.Uncommitted {
		t.Fatalf("delivery on closing record = %+v", r)
	}
}

// The pull request arrives from the agent's CLI, which knows the task but not
// its run id: SetPullRequest stamps the task's newest record, and the closing
// writer picks the url up through PullRequestFor, so the latest state a
// reader sees carries both the verdict and the PR it shipped as.
func TestPullRequestLandsOnTheRunAndSurvivesTheClosingRecord(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	if err := Append(Record{
		RunID: "r1", Task: "TASK-1", Status: StatusRunning, At: time.Now(), Branch: "fleet/x-1",
	}); err != nil {
		t.Fatal(err)
	}
	if err := SetPullRequest("TASK-1", "https://example.com/pr/7"); err != nil {
		t.Fatal(err)
	}
	r, err := LastRun("TASK-1")
	if err != nil || r == nil || r.PullRequest != "https://example.com/pr/7" {
		t.Fatalf("after SetPullRequest = %+v, %v", r, err)
	}
	if r.Status != StatusRunning {
		t.Fatalf("status = %s, want the record it copied", r.Status)
	}

	pr, err := PullRequestFor("r1")
	if err != nil || pr != "https://example.com/pr/7" {
		t.Fatalf("PullRequestFor(r1) = %q, %v", pr, err)
	}

	// The closing record is written later, with the PR carried forward: a
	// reader must not have to merge two lines to know how the run ended.
	if err := Append(Record{
		RunID: "r1", Task: "TASK-1", Status: StatusDone, At: time.Now(),
		Verdict: "done", DurationSeconds: 12, Branch: "fleet/x-1", Commits: 2,
		PullRequest: pr,
	}); err != nil {
		t.Fatal(err)
	}
	got, _ := LastRun("TASK-1")
	if got.Verdict != "done" || got.PullRequest == "" || got.Commits != 2 {
		t.Fatalf("closing record = %+v", got)
	}
}

// A task with no run at all is an error: the CLI reports it, never invents a
// run to hang the url on.
func TestSetPullRequestOnATaskWithNoRunFails(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	if err := SetPullRequest("TASK-404", "https://example.com/pr/1"); err == nil {
		t.Fatal("want an error when no run exists for the task")
	}
}

func mustAppend(t *testing.T, r Record) {
	t.Helper()
	if err := Append(r); err != nil {
		t.Fatal(err)
	}
}

// The board draws one history record per row and refreshes every few seconds.
// Asking for them one task at a time walked the whole log per row, so the cost
// of a refresh grew with the fleet's length times its history's — measured at
// 1.4s for 80 tasks, on a log the plugin never truncates. One pass answers the
// whole board, and it must answer it identically to LastRun.
func TestLatestPerTaskAnswersTheWholeBoardInOnePass(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	base := time.Now()
	steps := []Record{
		// TASK-1: an old run, then a newer one still in flight.
		{RunID: "r1", Task: "myapp/TASK-1", Status: StatusRunning, At: base},
		{RunID: "r1", Task: "myapp/TASK-1", Status: StatusFailed, At: base.Add(time.Minute)},
		{RunID: "r3", Task: "myapp/TASK-1", Status: StatusRunning, At: base.Add(2 * time.Minute)},
		// TASK-2: one run, closed, with a url stamped on afterwards.
		{RunID: "r2", Task: "myapp/TASK-2", Status: StatusRunning, At: base},
		{RunID: "r2", Task: "myapp/TASK-2", Status: StatusDone, At: base.Add(time.Minute)},
		{RunID: "r2", Task: "myapp/TASK-2", Status: StatusDone, At: base.Add(time.Minute), PullRequest: "https://example.com/pr/1"},
		// A task the board is not showing.
		{RunID: "r9", Task: "other/TASK-9", Status: StatusDone, At: base},
	}
	for _, r := range steps {
		mustAppend(t, r)
	}

	ids := []string{"myapp/TASK-1", "myapp/TASK-2", "myapp/TASK-404"}
	got := LatestPerTask(ids)

	for _, id := range ids {
		want, err := LastRun(id)
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case want == nil && got[id] != nil:
			t.Errorf("%s: LastRun has no record, the pass invented %+v", id, got[id])
		case want == nil:
		case got[id] == nil:
			t.Errorf("%s: LastRun found %s, the pass found nothing", id, want.Status)
		case got[id].RunID != want.RunID || got[id].Status != want.Status || got[id].PullRequest != want.PullRequest:
			t.Errorf("%s: pass has %+v, LastRun has %+v", id, *got[id], *want)
		}
	}
	// A task nobody asked about is not carried back.
	if _, ok := got["other/TASK-9"]; ok {
		t.Error("the pass returned a task the board did not ask for")
	}
}
