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
