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
	}, map[string]*history.Record{})
	want := []string{"To Do", "T-2", "T-3", "Done", "T-1"}
	if len(got) != len(want) {
		t.Fatalf("got %d rows", len(got))
	}
	for i, w := range want {
		label := got[i].header
		if label == "" {
			label = got[i].task.ID
		}
		if label != w {
			t.Fatalf("row %d = %q, want %q", i, label, w)
		}
	}
}
