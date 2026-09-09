// Package pick decides which task the fleet works on next. Pure: tasks and
// agents in, a decision out. The daemon owns the side effects.
package pick

import (
	"sort"

	"github.com/DnzzL/herdr-fleet/internal/backlog"
	"github.com/DnzzL/herdr-fleet/internal/fleet"
)

// Result is what should happen now: at most one task to run (the worker is
// strictly one-at-a-time), plus the To Do tasks that name an agent nobody has.
type Result struct {
	Task    *backlog.Task
	Agent   fleet.Agent
	Unknown []backlog.Task
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

// Next picks the most urgent To Do task whose assignee is a known agent.
// Unassigned tasks go to defaultAgent when one is configured, and are left
// alone otherwise — an unassigned task may be a human still drafting.
func Next(tasks []backlog.Task, agents map[string]fleet.Agent, defaultAgent string) Result {
	todo := make([]backlog.Task, 0, len(tasks))
	for _, t := range tasks {
		if t.Status == backlog.StatusToDo {
			todo = append(todo, t)
		}
	}
	sort.SliceStable(todo, func(i, j int) bool {
		if a, b := priorityRank(todo[i].Priority), priorityRank(todo[j].Priority); a != b {
			return a < b
		}
		if todo[i].Ordinal != todo[j].Ordinal {
			return todo[i].Ordinal < todo[j].Ordinal
		}
		return todo[i].CreatedAt < todo[j].CreatedAt
	})

	var res Result
	for i, t := range todo {
		name := AssigneeFor(t, defaultAgent)
		if name == "" {
			continue
		}
		agent, ok := agents[name]
		if !ok {
			res.Unknown = append(res.Unknown, t)
			continue
		}
		if agent.Disabled || agent.Unavailable {
			// Parked or busy, not missing: the task stays in To Do unremarked,
			// and the rest of the queue keeps moving. Reporting it Unknown would
			// let the daemon write "is not a fleet agent" onto a ticket whose
			// agent is simply still working.
			continue
		}
		if res.Task == nil {
			res.Task = &todo[i]
			res.Agent = agent
		}
	}
	return res
}

// AssigneeFor is the one routing rule: the task's first assignee, or the
// configured default agent when it has none. Every surface that routes a task
// (daemon, board, CLI) must go through this, or they drift apart.
func AssigneeFor(t backlog.Task, defaultAgent string) string {
	if len(t.Assignees) > 0 {
		return t.Assignees[0]
	}
	return defaultAgent
}
