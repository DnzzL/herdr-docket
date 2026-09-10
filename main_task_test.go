package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/DnzzL/herdr-fleet/internal/work"
)

// fakeSource stands in for the queue so the CLI verbs are tested without a
// backend on disk.
type fakeSource struct {
	items []work.Item
	item  work.Item
	err   error
}

func (f fakeSource) List() ([]work.Item, error)       { return f.items, f.err }
func (f fakeSource) Get(id string) (work.Item, error) { return f.item, f.err }

func runTask(t *testing.T, src work.Source, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := runTaskCmd(src, args, &out)
	return out.String(), err
}

// The agent's view of the queue: open work and who it routes to. Closed items
// are noise unless asked for.
func TestTaskListShowsOpenItemsWithTheirRoutingKey(t *testing.T) {
	src := fakeSource{items: []work.Item{
		{ID: "TASK-2", Title: "B", Assignee: "dev", Open: true, Phase: "To Do"},
		{ID: "TASK-1", Title: "A", Open: false, Phase: "Done"},
	}}
	got, err := runTask(t, src, "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "TASK-2") || !strings.Contains(got, "dev") {
		t.Fatalf("an open task and its assignee must show:\n%s", got)
	}
	if strings.Contains(got, "TASK-1") {
		t.Fatalf("closed work is not listed by default:\n%s", got)
	}
}

func TestTaskListAllIncludesClosedItems(t *testing.T) {
	src := fakeSource{items: []work.Item{
		{ID: "TASK-2", Title: "B", Open: true, Phase: "To Do"},
		{ID: "TASK-1", Title: "A", Open: false, Phase: "Done"},
	}}
	got, err := runTask(t, src, "list", "--all")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "TASK-1") || !strings.Contains(got, "TASK-2") {
		t.Fatalf("--all must include closed work:\n%s", got)
	}
}

// Get is what the run prompt and a human both need: the body, the history of
// notes, and the criteria, which are read-only.
func TestTaskViewShowsBodyNotesAndCriteria(t *testing.T) {
	src := fakeSource{item: work.Item{
		ID: "TASK-2", Title: "B", Assignee: "dev", Open: true, Phase: "To Do",
		Body: "do it", Notes: "so far",
		Criteria: []work.Criterion{{Index: 1, Text: "works"}},
	}}
	got, err := runTask(t, src, "view", "TASK-2")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"TASK-2", "do it", "so far", "works"} {
		if !strings.Contains(got, want) {
			t.Errorf("view must show %q:\n%s", want, got)
		}
	}
}

func TestTaskViewNeedsAnID(t *testing.T) {
	if _, err := runTask(t, fakeSource{}, "view"); err == nil {
		t.Fatal("view without an id must be a usage error")
	}
}

func TestTaskCmdRejectsAnUnknownVerb(t *testing.T) {
	if _, err := runTask(t, fakeSource{}, "frobnicate"); err == nil {
		t.Fatal("an unknown task verb must error")
	}
}

func TestTaskCmdSurfacesTheBackendError(t *testing.T) {
	if _, err := runTask(t, fakeSource{err: errors.New("backend down")}, "list"); err == nil {
		t.Fatal("a backend failure must reach the caller")
	}
}
