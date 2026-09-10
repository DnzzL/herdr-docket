package work

import (
	"reflect"
	"testing"
)

// The board groups by whatever phase the backend reports, with the phases the
// fleet knows by name first. A backend with phases of its own still renders —
// it just sorts after, in the order it turned up.
func TestPhasesOfOrdersKnownPhasesFirstThenTheRest(t *testing.T) {
	items := []Item{
		{ID: "1", Phase: "Done"},
		{ID: "2", Phase: "Custom"},
		{ID: "3", Phase: "To Do"},
		{ID: "4", Phase: "In Progress"},
		{ID: "5", Phase: "Todo"},
	}
	want := []string{"To Do", "In Progress", "Done", "Custom", "Todo"}
	if got := PhasesOf(items); !reflect.DeepEqual(got, want) {
		t.Fatalf("PhasesOf = %v, want %v", got, want)
	}
}

// An empty phase is a phase: a binary backend has nothing to say about where
// its work sits, and the board still has to show it somewhere.
func TestPhasesOfKeepsTheEmptyPhase(t *testing.T) {
	items := []Item{{ID: "1", Phase: ""}, {ID: "2", Phase: ""}, {ID: "3", Phase: "To Do"}}
	want := []string{"To Do", ""}
	if got := PhasesOf(items); !reflect.DeepEqual(got, want) {
		t.Fatalf("PhasesOf = %v, want %v", got, want)
	}
}

func TestPhasesOfOnNothingIsNothing(t *testing.T) {
	if got := PhasesOf(nil); len(got) != 0 {
		t.Fatalf("PhasesOf(nil) = %v, want empty", got)
	}
}
