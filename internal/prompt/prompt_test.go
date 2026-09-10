package prompt

import (
	"strings"
	"testing"

	"github.com/DnzzL/herdr-fleet/internal/fleet"
	"github.com/DnzzL/herdr-fleet/internal/work"
)

func TestAssembleCarriesEveryFactTheAgentNeeds(t *testing.T) {
	got := Assemble(
		fleet.Agent{Name: "marketing", Persona: "You run publication."},
		work.Item{
			ID: "TASK-7", Title: "Draft launch post", Open: true,
			Body: "Write the Show HN post.",
			Criteria: []work.Criterion{
				{Index: 1, Text: "under 200 words", Checked: false},
				{Index: 2, Text: "links the repo", Checked: true},
			},
			Notes: "previous attempt stalled",
		},
		"/Users/x/fleet",
	)
	for _, want := range []string{
		"You run publication.",
		"TASK-7", "Draft launch post", "Write the Show HN post.",
		"[ ] #1 under 200 words", "[x] #2 links the repo",
		"previous attempt stalled",
		"herdr-fleet task note TASK-7",
		"herdr-fleet task done TASK-7",
		"herdr-fleet task fail TASK-7",
		"herdr-fleet task block TASK-7",
		"herdr-fleet task create",
		"-a marketing",
		"one coherent slice",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("prompt is missing %q:\n%s", want, got)
		}
	}
}

// The agent talks to the queue through the fleet's own CLI. It has no reason
// to know the backend, its credentials, or where its files live — and it must
// not be handed the backlog CLI again, because that is how the two backlogs in
// the Quick start get confused.
func TestAssembleNeverNamesTheBackend(t *testing.T) {
	got := Assemble(fleet.Agent{Name: "a", Persona: "P"},
		work.Item{ID: "T-1", Title: "t", Open: true, Body: "b"}, "/d")
	for _, absent := range []string{"BACKLOG_CWD", "--check-ac", "--append-notes", "backlog task"} {
		if strings.Contains(got, absent) {
			t.Fatalf("prompt still says %q:\n%s", absent, got)
		}
	}
}

// Criteria are the human's record of what "done" means. The agent reasons
// about them and answers in prose; it does not tick its own boxes.
func TestAssembleTellsTheAgentTheCriteriaAreReadOnly(t *testing.T) {
	got := Assemble(fleet.Agent{Name: "a", Persona: "P"},
		work.Item{ID: "T-1", Title: "t", Open: true,
			Criteria: []work.Criterion{{Index: 1, Text: "works"}}}, "/d")
	if !strings.Contains(got, "not yours to edit") {
		t.Fatalf("prompt does not say the criteria are read-only:\n%s", got)
	}
	if !strings.Contains(got, "which criteria you met") {
		t.Fatalf("prompt does not ask for the verdict in the closing note:\n%s", got)
	}
}

func TestAssembleOmitsEmptySections(t *testing.T) {
	got := Assemble(fleet.Agent{Name: "a", Persona: "P"},
		work.Item{ID: "T-1", Title: "t", Open: true}, "/d")
	for _, absent := range []string{"Acceptance criteria", "Notes from previous runs", "## Description"} {
		if strings.Contains(got, absent) {
			t.Fatalf("empty section %q rendered:\n%s", absent, got)
		}
	}
}
