package pick

import (
	"testing"

	"github.com/DnzzL/herdr-fleet/internal/backlog"
	"github.com/DnzzL/herdr-fleet/internal/fleet"
)

func agents(names ...string) map[string]fleet.Agent {
	m := map[string]fleet.Agent{}
	for _, n := range names {
		m[n] = fleet.Agent{Name: n}
	}
	return m
}

func task(id, status, priority, assignee string) backlog.Task {
	t := backlog.Task{ID: id, Title: id, Status: status, Priority: priority}
	if assignee != "" {
		t.Assignees = []string{assignee}
	}
	return t
}

func TestPicksHighestPriorityToDoWithKnownAssignee(t *testing.T) {
	res := Next([]backlog.Task{
		task("TASK-1", "To Do", "low", "a"),
		task("TASK-2", "To Do", "high", "a"),
		task("TASK-3", "In Progress", "critical", "a"),
		task("TASK-4", "Done", "high", "a"),
	}, agents("a"), "")
	if res.Task == nil || res.Task.ID != "TASK-2" {
		t.Fatalf("got %+v", res.Task)
	}
	if res.Agent.Name != "a" {
		t.Fatalf("agent = %+v", res.Agent)
	}
}

func TestPriorityBeatsOrdinalWhichBeatsAge(t *testing.T) {
	res := Next([]backlog.Task{
		{ID: "T-1", Status: "To Do", Assignees: []string{"a"}, Ordinal: 1, CreatedAt: "2026-01-01"},
		{ID: "T-2", Status: "To Do", Assignees: []string{"a"}, Ordinal: 2, CreatedAt: "2025-01-01"},
	}, agents("a"), "")
	if res.Task.ID != "T-1" {
		t.Fatalf("ordinal should win: %v", res.Task.ID)
	}
	res = Next([]backlog.Task{
		{ID: "T-1", Status: "To Do", Priority: "medium", Assignees: []string{"a"}, Ordinal: 1},
		{ID: "T-2", Status: "To Do", Priority: "high", Assignees: []string{"a"}, Ordinal: 2},
	}, agents("a"), "")
	if res.Task.ID != "T-2" {
		t.Fatalf("priority should win: %v", res.Task.ID)
	}
}

func TestUnassignedGoesToDefaultAgentWhenConfigured(t *testing.T) {
	tasks := []backlog.Task{task("T-1", "To Do", "", "")}
	if res := Next(tasks, agents("d"), "d"); res.Task == nil || res.Agent.Name != "d" {
		t.Fatalf("want default agent, got %+v", res)
	}
	// No default agent: an unassigned task is a human's draft, not fleet work.
	if res := Next(tasks, agents("d"), ""); res.Task != nil {
		t.Fatalf("want no pick, got %v", res.Task.ID)
	}
}

func TestUnknownAssigneeIsReportedNotRun(t *testing.T) {
	res := Next([]backlog.Task{task("T-1", "To Do", "", "ghost")}, agents("a"), "")
	if res.Task != nil {
		t.Fatal("must not run a task for an unknown agent")
	}
	if len(res.Unknown) != 1 || res.Unknown[0].ID != "T-1" {
		t.Fatalf("unknown = %v", res.Unknown)
	}
}

func TestBlockedFailedAndDoneAreLeftAlone(t *testing.T) {
	res := Next([]backlog.Task{
		task("T-1", "Blocked", "high", "a"),
		task("T-2", "Failed", "high", "a"),
	}, agents("a"), "")
	if res.Task != nil {
		t.Fatalf("picked %v", res.Task.ID)
	}
}
