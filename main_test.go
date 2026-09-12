package main

import (
	"testing"

	"github.com/DnzzL/herdr-fleet/internal/fleet"
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
