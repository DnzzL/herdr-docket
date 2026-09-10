// Package pick decides which task the fleet works on next. Pure: tasks and
// agents in, a decision out. The daemon owns the side effects.
package pick

import (
	"sort"

	"github.com/DnzzL/herdr-fleet/internal/fleet"
	"github.com/DnzzL/herdr-fleet/internal/work"
)

// Result is what should happen now: at most one task to run (the worker is
// strictly one-at-a-time), plus the open tasks that name an agent nobody has.
type Result struct {
	Task    *work.Item
	Agent   fleet.Agent
	Unknown []work.Item
}

// priorityRank orders Backlog.md priorities; unset sorts last.
func priorityRank(p string) int {
	switch p {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	case "low":
		return 3
	}
	return 4
}

// Next picks the most urgent open task whose assignee is a known agent.
// Unassigned tasks go to defaultAgent when one is configured, and are left
// alone otherwise — an unassigned task may be a human still drafting.
//
// Open, not "To Do": a task left In Progress by a process that died still
// owes work, and picking it up again is what makes a crashed run self-heal.
// A task whose agent is mid-run is not re-picked — its agent is simply not
// free — and the run lock is what actually keeps two runs off the same task.
func Next(items []work.Item, agents map[string]fleet.Agent, defaultAgent string) Result {
	open := make([]work.Item, 0, len(items))
	for _, it := range items {
		if it.Open {
			open = append(open, it)
		}
	}
	sort.SliceStable(open, func(i, j int) bool {
		if a, b := priorityRank(open[i].Priority), priorityRank(open[j].Priority); a != b {
			return a < b
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
func AssigneeFor(it work.Item, defaultAgent string) string {
	if it.Assignee != "" {
		return it.Assignee
	}
	return defaultAgent
}
