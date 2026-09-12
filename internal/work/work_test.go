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

// The three endings are one vocabulary: the board stands a closed task under
// the same words the fleet writes its verdicts in. The order is built from the
// verdicts rather than retyped beside them, so the two cannot drift.
func TestPhaseOrderIsTheOpenPhasesThenTheVerdicts(t *testing.T) {
	want := []string{"To Do", "In Progress", "Blocked", "Failed", "Done"}
	if !reflect.DeepEqual(phaseOrder, want) {
		t.Fatalf("phaseOrder = %v, want %v", phaseOrder, want)
	}
}

// A verdict with no phase word to stand under would drop a closed task off the
// board entirely. Adding a verdict means giving it one, which is why this list
// exists and why phaseOrder is derived rather than hand-written.
func TestEveryKnownVerdictHasAPhaseToStandUnder(t *testing.T) {
	for _, v := range []Verdict{Done, Failed, Blocked} {
		if !v.Known() {
			t.Errorf("%q is not a verdict the port writes", v)
		}
		label := v.Label()
		if label == "" {
			t.Errorf("%q has no phase word to stand under", v)
			continue
		}
		if rank := phaseRank(label); rank >= len(phaseOrder) {
			t.Errorf("%q stands under %q, which the board has no place for: %v", v, label, phaseOrder)
		}
	}
}

func TestPhasesOfOnNothingIsNothing(t *testing.T) {
	if got := PhasesOf(nil); len(got) != 0 {
		t.Fatalf("PhasesOf(nil) = %v, want empty", got)
	}
}
