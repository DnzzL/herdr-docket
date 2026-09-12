package main

import (
	"strings"
	"testing"
	"time"

	"github.com/DnzzL/herdr-fleet/internal/fleet"
	"github.com/DnzzL/herdr-fleet/internal/history"
	"github.com/DnzzL/herdr-fleet/internal/work"
)

// `herdr-fleet run` is explicit human intent: it must reach a parked agent
// anyway, so pausing parks the scheduler rather than forbidding the work.
func TestManualRunReachesADisabledAgent(t *testing.T) {
	agents := map[string]fleet.Agent{"dev": {Name: "dev", Disabled: true}}
	task := work.Task{ID: "TASK-9", Assignee: "dev", Open: true}

	agent, err := routedAgent(agents, task, "")
	if err != nil {
		t.Fatalf("manual run must bypass the pause: %v", err)
	}
	if agent.Name != "dev" || !agent.Disabled {
		t.Fatalf("want the parked dev, got %+v", agent)
	}
}

func TestManualRunStillRejectsAnUnknownAgent(t *testing.T) {
	agents := map[string]fleet.Agent{"dev": {Name: "dev"}}
	if _, err := routedAgent(agents, work.Task{ID: "T", Assignee: "ghost"}, ""); err == nil {
		t.Fatal("an unknown assignee must still error")
	}
}

// The default-agent route resolves the same way, parked or not.
func TestManualRunResolvesTheDefaultAgent(t *testing.T) {
	agents := map[string]fleet.Agent{"dev": {Name: "dev", Disabled: true}}
	agent, err := routedAgent(agents, work.Task{ID: "T"}, "dev")
	if err != nil || agent.Name != "dev" {
		t.Fatalf("default agent should route: %v %+v", err, agent)
	}
}

// Budgets are scheduling policy, not a lock: a manual run reaches an
// over-budget agent the same way it reaches a parked one, because pressing
// the button is human intent. The budget is decided in the daemon, so routing
// a one-off run must not consult it.
func TestManualRunReachesAnOverBudgetAgent(t *testing.T) {
	agents := map[string]fleet.Agent{"dev": {Name: "dev", RunsPerDay: 6, MinutesPerDay: 180}}
	agent, err := routedAgent(agents, work.Task{ID: "TASK-9", Assignee: "dev"}, "")
	if err != nil || agent.Name != "dev" {
		t.Fatalf("manual run must not consult the budget: %v %+v", err, agent)
	}
}

// The history line answers both audit questions on one row: how long the run
// took and how it ended, when the closing record carries them.
func TestHistoryLineShowsDurationAndVerdict(t *testing.T) {
	r := history.Record{
		At:              time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC),
		Status:          history.StatusDone,
		Task:            "TASK-1",
		Trigger:         "poll",
		DurationSeconds: 90,
		Verdict:         "done",
	}
	line := formatHistory(r)
	for _, want := range []string{"1m30s", "done", "TASK-1", "poll"} {
		if !strings.Contains(line, want) {
			t.Fatalf("history line %q missing %q", line, want)
		}
	}
}

// A run still in flight has no closing record, so its line shows neither a
// duration nor a verdict — just the mechanics.
func TestHistoryLineOmitsWhatTheRunHasNotReported(t *testing.T) {
	r := history.Record{At: time.Now(), Status: history.StatusRunning, Task: "TASK-1", Trigger: "poll"}
	line := formatHistory(r)
	if strings.Contains(line, "0s") || strings.Contains(line, "done") {
		t.Fatalf("an open run must not invent duration or verdict: %q", line)
	}
}
