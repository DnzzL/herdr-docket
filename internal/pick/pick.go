// Package pick decides which task the fleet works on next. Pure: tasks and
// agents in, a decision out. The daemon owns the side effects.
package pick

import (
	"fmt"
	"sort"
	"strings"

	"github.com/DnzzL/herdr-fleet/internal/fleet"
	"github.com/DnzzL/herdr-fleet/internal/work"
)

// Result is what should happen now: at most one task to run (the worker is
// strictly one-at-a-time), plus the open tasks that name an agent nobody has.
type Result struct {
	Task    *work.Task
	Agent   fleet.Agent
	Unknown []work.Task
}

// Next picks the most urgent open task whose assignee is a known agent.
// Unassigned tasks go to defaultAgent when one is configured, and are left
// alone otherwise — an unassigned task may be a human still drafting.
//
// Open, not "To Do": a task left In Progress by a process that died still
// owes work, and picking it up again is what makes a crashed run self-heal.
// A task whose agent is mid-run is not re-picked — its agent is simply not
// free — and the run lock is what actually keeps two runs off the same task.
func Next(items []work.Task, agents map[string]fleet.Agent, defaultAgent string) Result {
	open := make([]work.Task, 0, len(items))
	for _, it := range items {
		if it.Open {
			open = append(open, it)
		}
	}
	// Most urgent first, then the backend's own order, then oldest first. The
	// ranks are each backend's to compute and this comparison is the fleet's to
	// own, so every backend gets the same policy rather than inventing one.
	sort.SliceStable(open, func(i, j int) bool {
		if a, b := open[i].Priority, open[j].Priority; a != b {
			return a > b
		}
		if open[i].Ordinal != open[j].Ordinal {
			return open[i].Ordinal < open[j].Ordinal
		}
		return open[i].CreatedAt < open[j].CreatedAt
	})

	var res Result
	for i, it := range open {
		name := AssigneeFor(it, defaultAgent)
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

// AssigneeFor is the one routing rule: the task's assignee, or the configured
// default agent when it has none. Every surface that routes a task (daemon,
// board, CLI) must go through this, or they drift apart.
func AssigneeFor(it work.Task, defaultAgent string) string {
	if it.Assignee != "" {
		return it.Assignee
	}
	return defaultAgent
}

// OverBudget reports whether an agent has spent a configured budget in the
// rolling window. A zero limit is unbounded: the fleet never invents one, and
// an agent whose budget is unset schedules exactly as it did before budgets
// existed. Reaching a limit spends it — the run that would hit the quota is
// the one that waits.
func OverBudget(a fleet.Agent, runs, minutes int) bool {
	if a.RunsPerDay > 0 && runs >= a.RunsPerDay {
		return true
	}
	if a.MinutesPerDay > 0 && minutes >= a.MinutesPerDay {
		return true
	}
	return false
}

// BudgetLine renders an agent's spend against its limits for the roster and
// the board, e.g. "4/6 runs today, 130/180 min". Empty for an agent with no
// budget, because unbounded is not a number to show.
func BudgetLine(a fleet.Agent, runs, minutes int) string {
	var parts []string
	if a.RunsPerDay > 0 {
		parts = append(parts, fmt.Sprintf("%d/%d runs today", runs, a.RunsPerDay))
	}
	if a.MinutesPerDay > 0 {
		parts = append(parts, fmt.Sprintf("%d/%d min", minutes, a.MinutesPerDay))
	}
	return strings.Join(parts, ", ")
}
