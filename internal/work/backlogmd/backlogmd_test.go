package backlogmd

import (
	"errors"
	"reflect"
	"testing"

	"github.com/DnzzL/herdr-fleet/internal/backlog"
	"github.com/DnzzL/herdr-fleet/internal/work"
)

// fakeClient is the adapter's seam: the Backlog.md client is a process
// boundary, so standing in for it tests the mapping without shelling out.
type fakeClient struct {
	tasks []backlog.Task
	view  backlog.View
	err   error
}

func (f fakeClient) List() ([]backlog.Task, error)        { return f.tasks, f.err }
func (f fakeClient) View(id string) (backlog.View, error) { return f.view, f.err }

// The native status word decides openness: a verdict closed Done, Failed or
// Blocked, and only the word survives for the board.
func TestListMapsEachStatusToOpenAndPhase(t *testing.T) {
	tasks := []backlog.Task{
		{ID: "T-1", Title: "queued", Status: "To Do"},
		{ID: "T-2", Title: "running", Status: "In Progress"},
		{ID: "T-3", Title: "parked", Status: "Blocked"},
		{ID: "T-4", Title: "broken", Status: "Failed"},
		{ID: "T-5", Title: "finished", Status: "Done"},
	}
	items, err := newWith(fakeClient{tasks: tasks}).List()
	if err != nil {
		t.Fatal(err)
	}
	want := []bool{true, true, false, false, false}
	if len(items) != len(want) {
		t.Fatalf("got %d items, want %d", len(items), len(want))
	}
	for i, it := range items {
		if it.Open != want[i] {
			t.Errorf("%s (%s): Open = %v, want %v", it.ID, it.Phase, it.Open, want[i])
		}
		if it.Phase != tasks[i].Status {
			t.Errorf("%s: Phase = %q, want the native word %q", it.ID, it.Phase, tasks[i].Status)
		}
	}
}

// The routing key is the first assignee: the one field pick reads to decide
// whose work this is.
func TestListCarriesTheRoutingKeyAndOrdering(t *testing.T) {
	items, err := newWith(fakeClient{tasks: []backlog.Task{{
		ID: "TASK-2", Title: "B", Status: "To Do", Priority: "high",
		Assignees: []string{"dev", "pm"}, Ordinal: 2000, CreatedAt: "2026-08-30T10:00:00Z",
	}}}).List()
	if err != nil {
		t.Fatal(err)
	}
	want := work.Item{
		ID: "TASK-2", Title: "B", Assignee: "dev", Open: true, Phase: "To Do",
		Priority: "high", Ordinal: 2000, CreatedAt: "2026-08-30T10:00:00Z",
	}
	if !reflect.DeepEqual(items[0], want) {
		t.Fatalf("got %+v\nwant %+v", items[0], want)
	}
}

func TestListPropagatesTheBackendError(t *testing.T) {
	if _, err := newWith(fakeClient{err: errors.New("backlog exploded")}).List(); err == nil {
		t.Fatal("a backend failure must not look like an empty queue")
	}
}

// Get is the full item: the list view plus what the prompt needs.
func TestGetCarriesBodyNotesAndCriteria(t *testing.T) {
	s := newWith(fakeClient{view: backlog.View{
		Task:        backlog.Task{ID: "TASK-2", Title: "B", Status: "In Progress", Assignees: []string{"dev"}},
		Description: "do it",
		AcceptanceCriteria: []backlog.Criterion{
			{Index: 1, Text: "works", Checked: false},
			{Index: 2, Text: "tested", Checked: true},
		},
		ImplementationNotes: "so far",
	}})
	it, err := s.Get("TASK-2")
	if err != nil {
		t.Fatal(err)
	}
	want := work.Item{
		ID: "TASK-2", Title: "B", Assignee: "dev", Open: true, Phase: "In Progress",
		Body:  "do it",
		Notes: "so far",
		Criteria: []work.Criterion{
			{Index: 1, Text: "works", Checked: false},
			{Index: 2, Text: "tested", Checked: true},
		},
	}
	if !reflect.DeepEqual(it, want) {
		t.Fatalf("got %+v\nwant %+v", it, want)
	}
}
