// Package pick decides which task the fleet works on next. Pure: tasks and
// agents in, a decision out. The daemon owns the side effects.
package pick

import (
	"sort"

	"github.com/DnzzL/herdr-docket/internal/fleet"
	"github.com/DnzzL/herdr-docket/internal/work"
)

// Result is what should happen now: at most one task to run (the worker is
// strictly one-at-a-time), plus the open tasks that name an agent nobody has.
type Result struct {
	Task    *work.Task
	Agent   fleet.Agent
	Unknown []work.Task
}

// Next picks the most urgent open task whose assignee is a known agent.
// Unassigned tasks go to the default agent for their queue when one is
// configured, and are left alone otherwise — an unassigned task may be a
// human still drafting.
//
// Open, not "To Do": a task left In Progress by a process that died still
// owes work, and picking it up again is what makes a crashed run self-heal.
// Blocked is the exception a run cannot serve: work parked on a human is
// read by running it, and the answer only a human can give does not come
// from another read — so a task the queue marks parked (work.Task.Blocked)
// is left alone, whatever its agent is doing.
// A task whose agent is mid-run is not re-picked — its agent is simply not
// free. And one task is kept to one run by the runner's per-task flock
// (`run-taskid-<id>.lock`), not by routing: the agent/checkout lock keys the
// agent or the checkout, so it protects the workspace, while the default
// routing is re-read every tick and a `fleet.yaml` edit can present an open
// task to a second agent while the first is mid-run.
func Next(items []work.Task, agents map[string]fleet.Agent, defaults fleet.Defaults) Result {
	// Blocked stays unrun: a parked task is standing in its own way, and the
	// word reaches pick through the task — not a literal.
	open := make([]work.Task, 0, len(items))
	for _, it := range items {
		if it.Open && !it.Blocked {
			open = append(open, it)
		}
	}
	// Most urgent first, then the backend's own order, then oldest first. The
	// ranks are each backend's to compute and this comparison is the fleet's to
	// own, so every backend gets the same policy rather than inventing one.
	sort.SliceStable(open, func(i, j int) bool { return Before(open[i], open[j]) })

	var res Result
	for i, it := range open {
		name := AssigneeFor(it, defaults)
		if name == "" {
			continue
		}
		agent, ok := agents[name]
		if !ok {
			res.Unknown = append(res.Unknown, it)
			continue
		}
		if agent.Disabled || agent.Unavailable {
			// Parked or busy, not missing: the task stays open unremarked,
			// and the rest of the queue keeps moving. Reporting it Unknown would
			// let the daemon write "is not a fleet agent" onto a ticket whose
			// agent is simply still working.
			continue
		}
		if res.Task == nil {
			res.Task = &open[i]
			res.Agent = agent
		}
	}
	return res
}

// Before reports whether a is worked before b: most urgent first, then the
// backend's own order, then oldest first. The ranks are each backend's to
// compute and this comparison is the fleet's to own, so every backend gets the
// same policy rather than inventing one — and the board calls this rather than
// the backend's list order, so the two agree about what is next. Exported for
// exactly that: a board that sorted its own way would be a list of work.
func Before(a, b work.Task) bool {
	if a.Priority != b.Priority {
		return a.Priority > b.Priority
	}
	if a.Ordinal != b.Ordinal {
		return a.Ordinal < b.Ordinal
	}
	return a.CreatedAt < b.CreatedAt
}

// AssigneeFor is the one routing rule: the task's assignee, or the default
// agent for the queue it came from when it has none. Every surface that routes
// a task (daemon, board, CLI) must go through this, or they drift apart.
func AssigneeFor(it work.Task, defaults fleet.Defaults) string {
	if it.Assignee != "" {
		return it.Assignee
	}
	return defaults.For(it.ID)
}
