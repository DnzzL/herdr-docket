package text

import (
	"strings"
	"testing"

	"github.com/DnzzL/herdr-docket/internal/work"
)

func TestTaskDetailRendersStateBodyCriteriaAndNotes(t *testing.T) {
	got := TaskDetail(work.Task{
		ID:       "myapp/TASK-12",
		Title:    "Fix the parser",
		Open:     true,
		Phase:    "In Progress",
		Assignee: "dev",
		Body:     "the numbers are not moving",
		Criteria: []work.Criterion{
			{Index: 1, Text: "a test fails first"},
			{Index: 2, Text: "and then passes", Checked: true},
		},
		Notes: "fleet: tried once",
	})
	for _, want := range []string{
		"myapp/TASK-12 — Fix the parser",
		"open · In Progress · dev",
		"## Description",
		"the numbers are not moving",
		"## Acceptance criteria",
		"- [ ] #1 a test fails first",
		"- [x] #2 and then passes",
		"## Notes",
		"fleet: tried once",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("detail is missing %q:\n%s", want, got)
		}
	}
}

// A task with nothing but a title still reads as a task: no empty sections, and
// a closed one says so, because that is the one word a person checks first.
func TestTaskDetailOmitsEmptySectionsAndSaysClosed(t *testing.T) {
	got := TaskDetail(work.Task{ID: "T-1", Title: "Ship it", Phase: "Done"})
	if strings.Contains(got, "##") {
		t.Errorf("a bare task should render no sections:\n%s", got)
	}
	if !strings.Contains(got, "closed · Done · -") {
		t.Errorf("got %q, want a closed state line with no assignee", got)
	}
}
