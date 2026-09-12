package pick

import (
	"testing"

	"github.com/DnzzL/herdr-fleet/internal/fleet"
	"github.com/DnzzL/herdr-fleet/internal/work"
)

func agents(names ...string) map[string]fleet.Agent {
	m := map[string]fleet.Agent{}
	for _, n := range names {
		m[n] = fleet.Agent{Name: n}
	}
	return m
}

// open is a task the fleet still owes work on, whatever phase it is showing.
// urgency is the rank a backend computed, not a word this package understands.
func open(id string, urgency int, assignee string) work.Task {
	return work.Task{ID: id, Title: id, Open: true, Priority: urgency, Assignee: assignee}
}

func closed(id, phase string) work.Task {
	return work.Task{ID: id, Title: id, Open: false, Phase: phase}
}

func TestPicksHighestPriorityOpenTaskWithKnownAssignee(t *testing.T) {
	res := Next([]work.Task{
		open("TASK-1", 1, "a"),
		open("TASK-2", 3, "a"),
		closed("TASK-4", "Done"),
	}, agents("a"), "")
	if res.Task == nil || res.Task.ID != "TASK-2" {
		t.Fatalf("got %+v", res.Task)
	}
	if res.Agent.Name != "a" {
		t.Fatalf("agent = %+v", res.Agent)
	}
}

// A rank is only ever compared, so the scale is the backend's business. What
// this package owns is the rule that a higher rank is more urgent — and that
// zero, the backend having no opinion, is the least urgent there is.
func TestAHigherRankIsMoreUrgentAndNoOpinionSortsLast(t *testing.T) {
	res := Next([]work.Task{
		open("unranked", 0, "a"),
		open("ranked", 1, "a"),
	}, agents("a"), "")
	if res.Task == nil || res.Task.ID != "ranked" {
		t.Fatalf("a ranked task should beat an unranked one: %+v", res.Task)
	}
}

// A task left mid-flight by a process that died still owes work. Picking it up
// again is what makes a crashed run self-heal; the run lock, not the phase, is
// what keeps two runs off the same task.
func TestWorkLeftMidFlightIsStillPickedUp(t *testing.T) {
	res := Next([]work.Task{
		{ID: "TASK-1", Title: "TASK-1", Open: true, Phase: "In Progress", Assignee: "a"},
	}, agents("a"), "")
	if res.Task == nil || res.Task.ID != "TASK-1" {
		t.Fatalf("an In Progress task must be picked up again: %+v", res.Task)
	}
}

func TestPriorityBeatsOrdinalWhichBeatsAge(t *testing.T) {
	res := Next([]work.Task{
		{ID: "T-1", Open: true, Assignee: "a", Ordinal: 1, CreatedAt: "2026-01-01"},
		{ID: "T-2", Open: true, Assignee: "a", Ordinal: 2, CreatedAt: "2025-01-01"},
	}, agents("a"), "")
	if res.Task.ID != "T-1" {
		t.Fatalf("ordinal should win: %v", res.Task.ID)
	}
	res = Next([]work.Task{
		{ID: "T-1", Open: true, Priority: 2, Assignee: "a", Ordinal: 1},
		{ID: "T-2", Open: true, Priority: 3, Assignee: "a", Ordinal: 2},
	}, agents("a"), "")
	if res.Task.ID != "T-2" {
		t.Fatalf("priority should win: %v", res.Task.ID)
	}
}

func TestUnassignedGoesToDefaultAgentWhenConfigured(t *testing.T) {
	items := []work.Task{open("T-1", 0, "")}
	if res := Next(items, agents("d"), "d"); res.Task == nil || res.Agent.Name != "d" {
		t.Fatalf("want default agent, got %+v", res)
	}
	// No default agent: an unassigned task is a human's draft, not fleet work.
	if res := Next(items, agents("d"), ""); res.Task != nil {
		t.Fatalf("want no pick, got %v", res.Task.ID)
	}
}

func TestUnknownAssigneeIsReportedNotRun(t *testing.T) {
	res := Next([]work.Task{open("T-1", 0, "ghost")}, agents("a"), "")
	if res.Task != nil {
		t.Fatal("must not run a task for an unknown agent")
	}
	if len(res.Unknown) != 1 || res.Unknown[0].ID != "T-1" {
		t.Fatalf("unknown = %v", res.Unknown)
	}
}

func TestDisabledAgentGetsNoWorkAndIsNotCalledUnknown(t *testing.T) {
	parked := map[string]fleet.Agent{"dev": {Name: "dev", Disabled: true}}

	// A parked agent exists: reporting it Unknown would make the daemon write
	// "assignee is not a fleet agent" onto the ticket, which is false.
	res := Next([]work.Task{open("T-1", 3, "dev")}, parked, "")
	if res.Task != nil {
		t.Fatalf("parked agent must not run work, got %v", res.Task.ID)
	}
	if len(res.Unknown) != 0 {
		t.Fatalf("parked agent must not be reported unknown: %v", res.Unknown)
	}

	// The same holds through the default-agent path: an unassigned task must
	// not be picked up by a parked default agent.
	if res := Next([]work.Task{open("T-2", 0, "")}, parked, "dev"); res.Task != nil {
		t.Fatalf("parked default agent must not steal an unassigned task, got %v", res.Task.ID)
	}
}

// An agent with a run in flight is busy, not missing. A sweep that hands off
// to itself creates the next task while its own run is still going, so
// reporting it Unknown would write a false "is not a fleet agent" note onto the
// ticket seconds before the daemon runs it.
func TestUnavailableAgentGetsNoWorkAndIsNotCalledUnknown(t *testing.T) {
	busy := map[string]fleet.Agent{"reviewer": {Name: "reviewer", Unavailable: true}}

	res := Next([]work.Task{open("T-1", 0, "reviewer")}, busy, "")
	if res.Task != nil {
		t.Fatalf("busy agent must not run work, got %v", res.Task.ID)
	}
	if len(res.Unknown) != 0 {
		t.Fatalf("busy agent must not be reported unknown: %v", res.Unknown)
	}

	if res := Next([]work.Task{open("T-2", 0, "")}, busy, "reviewer"); res.Task != nil || len(res.Unknown) != 0 {
		t.Fatalf("busy default agent must not steal or be reported unknown: %+v", res)
	}
}

func TestUnavailableAgentLeavesTheRestOfTheQueueMoving(t *testing.T) {
	m := agents("dev")
	m["reviewer"] = fleet.Agent{Name: "reviewer", Unavailable: true}

	res := Next([]work.Task{
		open("T-1", 0, "reviewer"),
		open("T-2", 3, "dev"),
	}, m, "")
	if res.Task == nil || res.Task.ID != "T-2" || res.Agent.Name != "dev" {
		t.Fatalf("idle agent's work should still run: %+v", res)
	}
	if len(res.Unknown) != 0 {
		t.Fatalf("the busy agent must not be reported unknown: %v", res.Unknown)
	}
}

func TestParkedAgentDoesNotBlockTheRestOfTheQueue(t *testing.T) {
	res := Next([]work.Task{
		open("T-1", 4, "dev"),
		open("T-2", 1, "scribe"),
	}, map[string]fleet.Agent{
		"dev":    {Name: "dev", Disabled: true},
		"scribe": {Name: "scribe"},
	}, "")
	if res.Task == nil || res.Task.ID != "T-2" {
		t.Fatalf("want T-2 to keep moving, got %+v", res.Task)
	}
}

// Closed is closed: however an task ended, the fleet leaves it alone.
func TestClosedWorkIsLeftAlone(t *testing.T) {
	res := Next([]work.Task{
		closed("T-1", "Blocked"),
		closed("T-2", "Failed"),
		closed("T-3", "Done"),
	}, agents("a"), "")
	if res.Task != nil {
		t.Fatalf("picked %v", res.Task.ID)
	}
	if len(res.Unknown) != 0 {
		t.Fatalf("closed work must not be reported as unknown: %v", res.Unknown)
	}
}
