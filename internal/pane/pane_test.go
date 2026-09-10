package pane

import (
	"testing"

	"github.com/DnzzL/herdr-fleet/internal/backlog"
	"github.com/DnzzL/herdr-fleet/internal/history"
)

func TestRowsGroupsByLifecycleOrderAndSkipsEmptyStatuses(t *testing.T) {
	got := rows([]backlog.Task{
		{ID: "T-1", Status: "Done"},
		{ID: "T-2", Status: "To Do"},
		{ID: "T-3", Status: "To Do"},
	}, map[string]*history.Record{}, "")
	want := []string{"To Do", "T-2", "T-3", "", "Done", "T-1"}
	if len(got) != len(want) {
		t.Fatalf("got %d rows", len(got))
	}
	for i, w := range want {
		if w == "" {
			if !got[i].spacer {
				t.Fatalf("row %d = %+v, want spacer", i, got[i])
			}
			continue
		}
		label := got[i].header
		if label == "" {
			label = got[i].task.ID
		}
		if label != w {
			t.Fatalf("row %d = %q, want %q", i, label, w)
		}
	}
}

func TestRowsFiltersByQueryCaseInsensitive(t *testing.T) {
	tasks := []backlog.Task{
		{ID: "T-1", Title: "Fix login bug", Status: "To Do"},
		{ID: "T-2", Title: "Add search", Status: "To Do"},
		{ID: "T-3", Title: "Unrelated", Status: "Done"},
	}
	got := rows(tasks, map[string]*history.Record{}, "SEARCH")
	var ids []string
	for _, r := range got {
		if r.header == "" && !r.spacer {
			ids = append(ids, r.task.ID)
		}
	}
	if len(ids) != 1 || ids[0] != "T-2" {
		t.Fatalf("got %v, want only T-2", ids)
	}
}
