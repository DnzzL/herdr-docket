package pane

import (
	"testing"

	"github.com/DnzzL/herdr-fleet/internal/history"
	"github.com/DnzzL/herdr-fleet/internal/work"
)

func TestRowsGroupsByPhaseOrderAndSkipsEmptyPhases(t *testing.T) {
	got := rows([]work.Item{
		{ID: "T-1", Phase: "Done"},
		{ID: "T-2", Phase: "To Do"},
		{ID: "T-3", Phase: "To Do"},
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

// A backend whose phases the fleet has never heard of still gets a board:
// the group is there, it just sorts after the phases the fleet knows.
func TestRowsShowsAPhaseTheFleetDoesNotKnow(t *testing.T) {
	got := rows([]work.Item{
		{ID: "T-1", Phase: "Needs Triage"},
		{ID: "T-2", Phase: "In Progress"},
	}, map[string]*history.Record{}, "")
	var labels []string
	for _, r := range got {
		switch {
		case r.spacer:
			labels = append(labels, "")
		case r.header != "":
			labels = append(labels, r.header)
		default:
			labels = append(labels, r.task.ID)
		}
	}
	want := []string{"In Progress", "T-2", "", "Needs Triage", "T-1"}
	if len(labels) != len(want) {
		t.Fatalf("got %v, want %v", labels, want)
	}
	for i := range want {
		if labels[i] != want[i] {
			t.Fatalf("got %v, want %v", labels, want)
		}
	}
}

func TestRowsFiltersByQueryCaseInsensitive(t *testing.T) {
	items := []work.Item{
		{ID: "T-1", Title: "Fix login bug", Phase: "To Do"},
		{ID: "T-2", Title: "Add search", Phase: "To Do"},
		{ID: "T-3", Title: "Unrelated", Phase: "Done"},
	}
	got := rows(items, map[string]*history.Record{}, "SEARCH")
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
