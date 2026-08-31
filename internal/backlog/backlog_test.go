package backlog

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// fakeRun records calls and plays back canned outputs.
type fakeRun struct {
	calls [][]string
	out   map[string]string // keyed on the first two args joined by a space
	err   error
}

func (f *fakeRun) run(args ...string) ([]byte, error) {
	f.calls = append(f.calls, args)
	if f.err != nil {
		return nil, f.err
	}
	key := strings.Join(args[:min(2, len(args))], " ")
	return []byte(f.out[key]), nil
}

const listJSON = `{"schemaVersion":1,"kind":"task-list","tasks":[
 {"id":"TASK-2","title":"B","status":"To Do","priority":"high","assignees":["backlog"],"labels":[],"ordinal":2000,"createdAt":"2026-08-30T10:00:00Z"},
 {"id":"TASK-1","title":"A","status":"Done","priority":null,"assignees":[],"labels":["x"],"ordinal":1000,"createdAt":"2026-08-29T10:00:00Z"}]}`

func TestListDecodesTasks(t *testing.T) {
	f := &fakeRun{out: map[string]string{"task list": listJSON}}
	c := &Client{Dir: "/tmp/x", run: f.run}
	tasks, err := c.List()
	if err != nil {
		t.Fatal(err)
	}
	want := []Task{
		{ID: "TASK-2", Title: "B", Status: "To Do", Priority: "high", Assignees: []string{"backlog"}, Labels: []string{}, Ordinal: 2000, CreatedAt: "2026-08-30T10:00:00Z"},
		{ID: "TASK-1", Title: "A", Status: "Done", Assignees: []string{}, Labels: []string{"x"}, Ordinal: 1000, CreatedAt: "2026-08-29T10:00:00Z"},
	}
	if !reflect.DeepEqual(tasks, want) {
		t.Fatalf("got %+v", tasks)
	}
	if got := f.calls[0]; !reflect.DeepEqual(got, []string{"task", "list", "--json"}) {
		t.Fatalf("called %v", got)
	}
}

func TestViewDecodesDescriptionAndCriteria(t *testing.T) {
	f := &fakeRun{out: map[string]string{"task view": `{"kind":"task-view","task":
	 {"id":"TASK-2","title":"B","status":"To Do","description":"do it",
	  "acceptanceCriteria":[{"index":1,"text":"works","checked":false}],
	  "implementationNotes":"so far"}}`}}
	c := &Client{run: f.run}
	v, err := c.View("TASK-2")
	if err != nil {
		t.Fatal(err)
	}
	if v.Description != "do it" || len(v.AcceptanceCriteria) != 1 || v.AcceptanceCriteria[0].Text != "works" {
		t.Fatalf("got %+v", v)
	}
}

func TestSetStatusAndAppendNote(t *testing.T) {
	f := &fakeRun{out: map[string]string{}}
	c := &Client{run: f.run}
	if err := c.SetStatus("TASK-2", StatusInProgress); err != nil {
		t.Fatal(err)
	}
	if err := c.AppendNote("TASK-2", "a note"); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"task", "edit", "TASK-2", "-s", "In Progress", "--plain"},
		{"task", "edit", "TASK-2", "--append-notes", "a note", "--plain"},
	}
	if !reflect.DeepEqual(f.calls, want) {
		t.Fatalf("calls = %v", f.calls)
	}
}

func TestErrorsPropagate(t *testing.T) {
	f := &fakeRun{err: errors.New("boom")}
	c := &Client{run: f.run}
	if _, err := c.List(); err == nil {
		t.Fatal("want error")
	}
}
