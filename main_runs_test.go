package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/DnzzL/herdr-docket/internal/history"
	"github.com/DnzzL/herdr-docket/internal/host"
)

type closer struct{ closed []host.Session }

func (c *closer) Close(s host.Session) error {
	c.closed = append(c.closed, s)
	return nil
}

func appendAll(t *testing.T, recs ...history.Record) {
	t.Helper()
	for _, r := range recs {
		if err := history.Append(r); err != nil {
			t.Fatal(err)
		}
	}
}

// Stop closes what the run owns, as its record says: the borrowed tab, not
// the workspace it shares with other runs.
func TestStopClosesTheTabARunBorrowed(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	appendAll(t, history.Record{RunID: "r1", Task: "TASK-1", Status: history.StatusRunning, At: time.Now(), WorkspaceID: "ws", PaneID: "p", TabID: "tab"})
	c := &closer{}
	var out bytes.Buffer
	if err := stopRun(c, "TASK-1", &out); err != nil {
		t.Fatal(err)
	}
	if len(c.closed) != 1 || c.closed[0].TabID != "tab" || c.closed[0].WorkspaceID != "ws" {
		t.Fatalf("closed %+v", c.closed)
	}
	if !strings.Contains(out.String(), "TASK-1") {
		t.Fatalf("stop must say what it stopped: %q", out.String())
	}
}

func TestStopRefusesATaskWithNoRunInFlight(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	appendAll(t,
		history.Record{RunID: "r1", Task: "TASK-1", Status: history.StatusRunning, At: time.Now(), WorkspaceID: "ws"},
		history.Record{RunID: "r1", Task: "TASK-1", Status: history.StatusDone, At: time.Now()},
	)
	c := &closer{}
	err := stopRun(c, "TASK-1", &bytes.Buffer{})
	if err == nil || len(c.closed) != 0 {
		t.Fatalf("err %v, closed %+v", err, c.closed)
	}
}

// pause --now is the soft pause plus a stop of every run in flight.
func TestPauseNowStopsEveryRunInFlight(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	appendAll(t,
		history.Record{RunID: "r1", Task: "TASK-1", Status: history.StatusRunning, At: time.Now(), WorkspaceID: "ws1"},
		history.Record{RunID: "r2", Task: "TASK-2", Status: history.StatusTimedOut, At: time.Now(), WorkspaceID: "ws2"},
	)
	c := &closer{}
	if err := pauseCmd(c, []string{"--now"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if len(c.closed) != 2 {
		t.Fatalf("closed %+v", c.closed)
	}
	var out bytes.Buffer
	if err := runsCmd(nil, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "paused") {
		t.Fatalf("runs must say the fleet is paused: %q", out.String())
	}
}

// A running record past its agent's timeout is one nothing closed — a daemon
// that died under it. The line says so rather than drawing it as live work.
func TestRunsMarksARecordPastItsTimeoutStale(t *testing.T) {
	now := time.Now()
	fresh := formatRun(history.Record{Task: "TASK-1", Agent: "dev", Status: history.StatusRunning, At: now.Add(-5 * time.Minute)}, 30, now)
	old := formatRun(history.Record{Task: "TASK-2", Agent: "dev", Status: history.StatusRunning, At: now.Add(-45 * time.Minute)}, 30, now)
	if strings.Contains(fresh, "stale") || !strings.Contains(fresh, "5m") {
		t.Fatalf("fresh: %q", fresh)
	}
	if !strings.Contains(old, "stale") {
		t.Fatalf("old: %q", old)
	}
}

// A record past its run's timeout is one nothing closed: its workspace id may
// since name somebody else's workspace. Stop refuses it rather than closing
// whatever answers to that id now.
func TestStopRefusesAStaleRecord(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	appendAll(t, history.Record{RunID: "r1", Task: "TASK-1", Status: history.StatusRunning, At: time.Now().Add(-100 * time.Hour), WorkspaceID: "w1F"})
	c := &closer{}
	err := stopRun(c, "TASK-1", &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "stale") || len(c.closed) != 0 {
		t.Fatalf("err %v, closed %+v", err, c.closed)
	}
}

func TestPauseNowSkipsStaleRecords(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	appendAll(t,
		history.Record{RunID: "r1", Task: "TASK-1", Status: history.StatusRunning, At: time.Now(), WorkspaceID: "ws1"},
		history.Record{RunID: "r2", Task: "TASK-2", Status: history.StatusRunning, At: time.Now().Add(-100 * time.Hour), WorkspaceID: "w1F"},
	)
	c := &closer{}
	var out bytes.Buffer
	if err := pauseCmd(c, []string{"--now"}, &out); err != nil {
		t.Fatal(err)
	}
	if len(c.closed) != 1 || c.closed[0].WorkspaceID != "ws1" {
		t.Fatalf("closed %+v", c.closed)
	}
	if !strings.Contains(out.String(), "TASK-2") {
		t.Fatalf("pause --now must name the stale record it left: %q", out.String())
	}
}

// A timed-out run is still listened for — the late window is twice the
// timeout (ADR 0014) — so it is stale only past that.
func TestATimedOutRunIsStaleOnlyPastItsLateWindow(t *testing.T) {
	now := time.Now()
	late := history.Record{Status: history.StatusTimedOut, At: now.Add(-50 * time.Minute)}
	gone := history.Record{Status: history.StatusTimedOut, At: now.Add(-70 * time.Minute)}
	if stale(late, 30, now) || !stale(gone, 30, now) {
		t.Fatalf("late %v, gone %v", stale(late, 30, now), stale(gone, 30, now))
	}
}
