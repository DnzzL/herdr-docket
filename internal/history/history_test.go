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

// The budget window is a rolling 24h: a run that falls out no longer counts,
// only closing records count, and a record the fleet never stamped with an
// agent is ignored rather than attributed to nobody.
func TestUsageSinceCountsCompletedRunsInTheRollingWindow(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	now := time.Now()
	for _, r := range []Record{
		{RunID: "fresh", Task: "T", Agent: "dev", Status: StatusDone, At: now.Add(-time.Hour), DurationSeconds: 120},
		{RunID: "aged", Task: "T", Agent: "dev", Status: StatusDone, At: now.Add(-25 * time.Hour), DurationSeconds: 600},
		{RunID: "running", Task: "T", Agent: "dev", Status: StatusRunning, At: now.Add(-time.Minute)},
		{RunID: "other", Task: "T", Agent: "scribe", Status: StatusFailed, At: now.Add(-time.Minute), DurationSeconds: 30},
		{RunID: "stampless", Task: "T", Status: StatusDone, At: now.Add(-time.Minute), DurationSeconds: 90},
	} {
		if err := Append(r); err != nil {
			t.Fatal(err)
		}
	}
	u := UsageSince(now)
	if got := u["dev"]; got.Runs != 1 || got.Minutes != 2 {
		t.Fatalf("dev usage = %+v, want 1 run / 2 min (the aged run is outside the window)", got)
	}
	if got := u["scribe"]; got.Runs != 1 || got.Minutes != 1 {
		t.Fatalf("scribe usage = %+v, want 1 run / 1 min", got)
	}
	if _, ok := u[""]; ok {
		t.Fatal("a record with no agent must not be attributed to nobody")
	}
}
