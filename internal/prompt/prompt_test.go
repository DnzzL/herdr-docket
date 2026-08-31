package prompt

import (
	"strings"
	"testing"

	"github.com/DnzzL/herdr-fleet/internal/backlog"
	"github.com/DnzzL/herdr-fleet/internal/fleet"
)

func TestAssembleCarriesEveryFactTheAgentNeeds(t *testing.T) {
	got := Assemble(
		fleet.Agent{Name: "marketing", Persona: "You run publication."},
		backlog.View{
			Task:        backlog.Task{ID: "TASK-7", Title: "Draft launch post"},
			Description: "Write the Show HN post.",
			AcceptanceCriteria: []backlog.Criterion{
				{Index: 1, Text: "under 200 words", Checked: false},
				{Index: 2, Text: "links the repo", Checked: true},
			},
			ImplementationNotes: "previous attempt stalled",
		},
		"/Users/x/fleet",
	)
	for _, want := range []string{
		"You run publication.",
		"TASK-7", "Draft launch post", "Write the Show HN post.",
		"[ ] #1 under 200 words", "[x] #2 links the repo",
		"previous attempt stalled",
		`BACKLOG_CWD=/Users/x/fleet backlog task edit TASK-7`,
		"-s Done", "-s Failed", "-s Blocked",
		"--check-ac",
		"one coherent slice",
		"-a marketing",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("prompt is missing %q:\n%s", want, got)
		}
	}
}

func TestAssembleOmitsEmptySections(t *testing.T) {
	got := Assemble(fleet.Agent{Name: "a", Persona: "P"},
		backlog.View{Task: backlog.Task{ID: "T-1", Title: "t"}}, "/d")
	for _, absent := range []string{"Acceptance criteria", "Notes from previous runs", "## Description"} {
		if strings.Contains(got, absent) {
			t.Fatalf("empty section %q rendered:\n%s", absent, got)
		}
	}
}
