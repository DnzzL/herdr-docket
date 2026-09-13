package pick

import (
	"testing"
	"time"

	"github.com/DnzzL/herdr-fleet/internal/fleet"
	"github.com/DnzzL/herdr-fleet/internal/history"
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
	}, agents("a"), defaults(""))
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
	}, agents("a"), defaults(""))
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
	}, agents("a"), defaults(""))
	if res.Task == nil || res.Task.ID != "TASK-1" {
		t.Fatalf("an In Progress task must be picked up again: %+v", res.Task)
	}
}

func TestPriorityBeatsOrdinalWhichBeatsAge(t *testing.T) {
	res := Next([]work.Task{
		{ID: "T-1", Open: true, Assignee: "a", Ordinal: 1, CreatedAt: "2026-01-01"},
		{ID: "T-2", Open: true, Assignee: "a", Ordinal: 2, CreatedAt: "2025-01-01"},
	}, agents("a"), defaults(""))
	if res.Task.ID != "T-1" {
		t.Fatalf("ordinal should win: %v", res.Task.ID)
	}
	res = Next([]work.Task{
		{ID: "T-1", Open: true, Priority: 2, Assignee: "a", Ordinal: 1},
		{ID: "T-2", Open: true, Priority: 3, Assignee: "a", Ordinal: 2},
	}, agents("a"), defaults(""))
	if res.Task.ID != "T-2" {
		t.Fatalf("priority should win: %v", res.Task.ID)
	}
}

func TestUnassignedGoesToDefaultAgentWhenConfigured(t *testing.T) {
	items := []work.Task{open("T-1", 0, "")}
	if res := Next(items, agents("d"), defaults("d")); res.Task == nil || res.Agent.Name != "d" {
		t.Fatalf("want default agent, got %+v", res)
	}
	// No default agent: an unassigned task is a human's draft, not fleet work.
	if res := Next(items, agents("d"), defaults("")); res.Task != nil {
		t.Fatalf("want no pick, got %v", res.Task.ID)
	}
}

func TestUnknownAssigneeIsReportedNotRun(t *testing.T) {
	res := Next([]work.Task{open("T-1", 0, "ghost")}, agents("a"), defaults(""))
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
	res := Next([]work.Task{open("T-1", 3, "dev")}, parked, defaults(""))
	if res.Task != nil {
		t.Fatalf("parked agent must not run work, got %v", res.Task.ID)
	}
	if len(res.Unknown) != 0 {
		t.Fatalf("parked agent must not be reported unknown: %v", res.Unknown)
	}

	// The same holds through the default-agent path: an unassigned task must
	// not be picked up by a parked default agent.
	if res := Next([]work.Task{open("T-2", 0, "")}, parked, defaults("dev")); res.Task != nil {
		t.Fatalf("parked default agent must not steal an unassigned task, got %v", res.Task.ID)
	}
}

// An agent with a run in flight is busy, not missing. A sweep that hands off
// to itself creates the next task while its own run is still going, so
// reporting it Unknown would write a false "is not a fleet agent" note onto the
// ticket seconds before the daemon runs it.
func TestUnavailableAgentGetsNoWorkAndIsNotCalledUnknown(t *testing.T) {
	busy := map[string]fleet.Agent{"reviewer": {Name: "reviewer", Unavailable: true}}

	res := Next([]work.Task{open("T-1", 0, "reviewer")}, busy, defaults(""))
	if res.Task != nil {
		t.Fatalf("busy agent must not run work, got %v", res.Task.ID)
	}
	if len(res.Unknown) != 0 {
		t.Fatalf("busy agent must not be reported unknown: %v", res.Unknown)
	}

	if res := Next([]work.Task{open("T-2", 0, "")}, busy, defaults("reviewer")); res.Task != nil || len(res.Unknown) != 0 {
		t.Fatalf("busy default agent must not steal or be reported unknown: %+v", res)
	}
}

func TestUnavailableAgentLeavesTheRestOfTheQueueMoving(t *testing.T) {
	m := agents("dev")
	m["reviewer"] = fleet.Agent{Name: "reviewer", Unavailable: true}

	res := Next([]work.Task{
		open("T-1", 0, "reviewer"),
		open("T-2", 3, "dev"),
	}, m, defaults(""))
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
	}, defaults(""))
	if res.Task == nil || res.Task.ID != "T-2" {
		t.Fatalf("want T-2 to keep moving, got %+v", res.Task)
	}
}

// Closed is closed: however a task ended, the fleet leaves it alone.
func TestClosedWorkIsLeftAlone(t *testing.T) {
	res := Next([]work.Task{
		closed("T-1", "Blocked"),
		closed("T-2", "Failed"),
		closed("T-3", "Done"),
	}, agents("a"), defaults(""))
	if res.Task != nil {
		t.Fatalf("picked %v", res.Task.ID)
	}
	if len(res.Unknown) != 0 {
		t.Fatalf("closed work must not be reported as unknown: %v", res.Unknown)
	}
}

// A budget is scheduling policy, not a config toggle: an agent inside its
// limit runs exactly as before, one that has reached a limit waits, and an
// agent with no limit is unbounded. Reaching a limit spends it — the run that
// would cross the line is the one that waits.
func TestBudgetDecidesWhetherAnAgentStillRuns(t *testing.T) {
	for _, tc := range []struct {
		name          string
		runs, minutes int
		limit         fleet.Agent
		spent         bool
	}{
		{"under the run budget", 5, 0, fleet.Agent{RunsPerDay: 6}, false},
		{"at the run budget", 6, 0, fleet.Agent{RunsPerDay: 6}, true},
		{"past the run budget", 7, 0, fleet.Agent{RunsPerDay: 6}, true},
		{"under the minute budget", 0, 130, fleet.Agent{MinutesPerDay: 180}, false},
		{"at the minute budget", 0, 180, fleet.Agent{MinutesPerDay: 180}, true},
		{"no budget at all is unbounded", 1000, 100000, fleet.Agent{}, false},
		{"a run budget leaves minutes alone", 0, 100000, fleet.Agent{RunsPerDay: 6}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := OverBudget(tc.limit, tc.runs, tc.minutes); got != tc.spent {
				t.Fatalf("OverBudget = %v, want %v", got, tc.spent)
			}
		})
	}
}

// The roster reads a budgeted agent's spend against its limits; an agent with
// no budget says nothing, because "unbounded" is not a number.
func TestBudgetLineRendersOnlyWhatIsBudgeted(t *testing.T) {
	a := fleet.Agent{Name: "dev", RunsPerDay: 6, MinutesPerDay: 180}
	if got := BudgetLine(a, 4, 130); got != "4/6 runs today, 130/180 min" {
		t.Fatalf("BudgetLine = %q", got)
	}
	if got := BudgetLine(fleet.Agent{Name: "dev", RunsPerDay: 6}, 4, 130); got != "4/6 runs today" {
		t.Fatalf("a runs-only budget should show only runs, got %q", got)
	}
	if got := BudgetLine(fleet.Agent{Name: "dev"}, 4, 130); got != "" {
		t.Fatalf("an unbudgeted agent should show no line, got %q", got)
	}
}

// The window rolls, so a budget re-opens on its own: the same six runs that
// spend a six-run budget today leave it whole once one slides out of the last
// 24h. OverBudget reads the count; UsageSince is what makes it a day and not
// a flag nobody ever clears.
func TestABudgetReopensAsOldRunsLeaveTheWindow(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	now := time.Now()
	for i := 0; i < 6; i++ {
		at := now.Add(-time.Minute)
		if i == 0 {
			at = now.Add(-25 * time.Hour) // this one has aged out
		}
		if err := history.Append(history.Record{
			RunID: "run-" + string(rune('0'+i)), Task: "T", Agent: "dev",
			Status: history.StatusDone, At: at, DurationSeconds: 60,
		}); err != nil {
			t.Fatal(err)
		}
	}
	a := fleet.Agent{Name: "dev", RunsPerDay: 6}
	if u := history.UsageSince(now)["dev"]; OverBudget(a, u.Runs, u.Minutes) {
		t.Fatalf("five runs inside the window must not spend a six-run budget: %+v", u)
	}
}

// Two projects behind one fleet means two intakes. An agent carries its own
// workdir, so a single default would hand dishnow's work to an agent that
// opens the notara checkout — the routing has to follow the task's queue.
func TestUnassignedWorkGoesToItsOwnProjectsIntake(t *testing.T) {
	d := fleet.Settings{
		DefaultAgent: "dev",
		Sources: map[string]fleet.SourceConfig{
			"notara":  {DefaultAgent: "notara-pm"},
			"dishnow": {DefaultAgent: "dishnow-pm"},
		},
	}.Defaults()
	for _, tc := range []struct{ id, want string }{
		{"dishnow/TASK-19", "dishnow-pm"},
		{"notara/NOT-92", "notara-pm"},
		{"loose/TASK-1", "dev"}, // a queue with no intake of its own
	} {
		res := Next([]work.Task{open(tc.id, 0, "")}, agents("dev", "notara-pm", "dishnow-pm"), d)
		if res.Task == nil || res.Agent.Name != tc.want {
			t.Errorf("%s: routed to %+v, want %s", tc.id, res.Agent.Name, tc.want)
		}
	}
}

// defaults is the routing default as the settings express it — one name for
// the whole fleet, which is what these cases are about.
func defaults(name string) fleet.Defaults {
	return fleet.Settings{DefaultAgent: name}.Defaults()
}
