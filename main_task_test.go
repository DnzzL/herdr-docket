package main

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/DnzzL/herdr-fleet/internal/work"
)

// fakeSource stands in for the queue so the CLI verbs are tested without a
// backend on disk. Writes are recorded to check the verb actually reached it.
type fakeSource struct {
	items []work.Task
	item  work.Task
	err   error

	created  []string // title, body, assignee
	comments []string
	verdicts []work.Verdict
}

func (f *fakeSource) List() ([]work.Task, error)       { return f.items, f.err }
func (f *fakeSource) Get(id string) (work.Task, error) { return f.item, f.err }

func (f *fakeSource) Create(title, body, assignee string) (string, error) {
	f.created = []string{title, body, assignee}
	return "TASK-9", f.err
}

func (f *fakeSource) Comment(id, text string) error {
	f.comments = append(f.comments, text)
	return f.err
}

func (f *fakeSource) Close(id string, v work.Verdict) error {
	f.verdicts = append(f.verdicts, v)
	return f.err
}

func runTask(t *testing.T, src work.Source, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := runTaskCmd(src, args, &out)
	return out.String(), err
}

// The agent's view of the queue: open work and who it routes to. Closed tasks
// are noise unless asked for.
func TestTaskListShowsOpenItemsWithTheirRoutingKey(t *testing.T) {
	src := &fakeSource{items: []work.Task{
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
	src := &fakeSource{items: []work.Task{
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
	src := &fakeSource{item: work.Task{
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

// Creating work is how a run hands on what it found, and how an automation
// seeds the queue: title, the routing key, and the why.
func TestTaskCreatePassesTitleBodyAndAssignee(t *testing.T) {
	src := &fakeSource{}
	got, err := runTask(t, src, "create", "Fix the thing", "-a", "dev", "-d", "because it is broken")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Fix the thing", "because it is broken", "dev"}; !reflect.DeepEqual(src.created, want) {
		t.Fatalf("created %v, want %v", src.created, want)
	}
	if !strings.Contains(got, "TASK-9") {
		t.Fatalf("create should name the new task:\n%s", got)
	}
}

func TestTaskCreateNeedsATitle(t *testing.T) {
	if _, err := runTask(t, &fakeSource{}, "create", "-a", "dev"); err == nil {
		t.Fatal("create without a title must be a usage error")
	}
}

// The three closing verbs are the verdict vocabulary: one word each, and the
// task is closed whatever it is.
func TestTaskCloseVerbsMapToVerdicts(t *testing.T) {
	for verb, want := range map[string]work.Verdict{
		"done": work.Done, "fail": work.Failed, "block": work.Blocked,
	} {
		src := &fakeSource{}
		if _, err := runTask(t, src, verb, "TASK-2"); err != nil {
			t.Fatalf("%s: %v", verb, err)
		}
		if !reflect.DeepEqual(src.verdicts, []work.Verdict{want}) {
			t.Errorf("%s closed with %v, want %v", verb, src.verdicts, want)
		}
	}
}

// A note is recorded before the task closes, so the reason is on the task
// even if closing it is the last thing that happens.
func TestTaskCloseWithANoteCommentsFirst(t *testing.T) {
	src := &fakeSource{}
	if _, err := runTask(t, src, "fail", "TASK-2", "--note", "the API is down"); err != nil {
		t.Fatal(err)
	}
	if want := []string{"the API is down"}; !reflect.DeepEqual(src.comments, want) {
		t.Fatalf("commented %v, want %v", src.comments, want)
	}
	if len(src.verdicts) != 1 {
		t.Fatalf("verdicts %v, want exactly one close", src.verdicts)
	}
}

func TestTaskNoteAppendsToTheItem(t *testing.T) {
	src := &fakeSource{}
	if _, err := runTask(t, src, "note", "TASK-2", "half done"); err != nil {
		t.Fatal(err)
	}
	if want := []string{"half done"}; !reflect.DeepEqual(src.comments, want) {
		t.Fatalf("commented %v, want %v", src.comments, want)
	}
	if len(src.verdicts) != 0 {
		t.Fatalf("note must not close the task, got %v", src.verdicts)
	}
}

func TestTaskVerbsNeedAnID(t *testing.T) {
	for _, verb := range []string{"view", "note", "done", "fail", "block"} {
		if _, err := runTask(t, &fakeSource{}, verb); err == nil {
			t.Errorf("%s without an id must be a usage error", verb)
		}
	}
}

func TestTaskCloseRejectsAnUnknownFlag(t *testing.T) {
	if _, err := runTask(t, &fakeSource{}, "done", "TASK-2", "--wrong"); err == nil {
		t.Fatal("an unknown flag must error rather than be ignored")
	}
}

func TestTaskCmdRejectsAnUnknownVerb(t *testing.T) {
	if _, err := runTask(t, &fakeSource{}, "frobnicate"); err == nil {
		t.Fatal("an unknown task verb must error")
	}
}

func TestTaskCmdSurfacesTheBackendError(t *testing.T) {
	if _, err := runTask(t, &fakeSource{err: errors.New("backend down")}, "list"); err == nil {
		t.Fatal("a backend failure must reach the caller")
	}
}

func TestTaskCloseSurfacesTheBackendError(t *testing.T) {
	if _, err := runTask(t, &fakeSource{err: errors.New("backend down")}, "done", "TASK-2"); err == nil {
		t.Fatal("a failed close must reach the caller")
	}
}
