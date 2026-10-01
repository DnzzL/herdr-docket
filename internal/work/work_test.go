package work

import (
	"fmt"
	"reflect"
	"testing"
)

// The board groups by whatever phase the backend reports, with the phases the
// fleet knows by name first. A backend with phases of its own still renders —
// it just sorts after, in the order it turned up.
func TestPhasesOfOrdersKnownPhasesFirstThenTheRest(t *testing.T) {
	items := []Task{
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
	items := []Task{{ID: "1", Phase: ""}, {ID: "2", Phase: ""}, {ID: "3", Phase: "To Do"}}
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

// A prefixed id is read by people as the backend's own id: the queue name is
// the board's column, not something to retype next to it.
func TestLocalOfStripsTheQueueAndLeavesABareIdAlone(t *testing.T) {
	for _, tc := range []struct{ id, want string }{
		{"myapp/TASK-12", "TASK-12"},
		{"agency/987654", "987654"},
		{"TASK-12", "TASK-12"},
		{"", ""},
		{"myapp/", "myapp/"},
	} {
		if got := LocalOf(tc.id); got != tc.want {
			t.Errorf("LocalOf(%q) = %q, want %q", tc.id, got, tc.want)
		}
	}
}

// TASK-45: a runner asks the port which ref to branch from, through the
// capability — not a type switch sprinkled through it.
func TestBaseBranchOfAnswersThroughTheCapability(t *testing.T) {
	src := fakeBrancher{base: "origin/main", err: nil}
	if got := BaseBranchOf(src, "TASK-1"); got != "origin/main" {
		t.Fatalf("BaseBranchOf = %q, want origin/main", got)
	}
}

// An Adapterless queue has no ref to name, and an error is the same as none:
// the caller falls back to inheriting, so both surface as "".
func TestBaseBranchOfTreatsNoCapabilityAndNoAnswerTheSame(t *testing.T) {
	if got := BaseBranchOf(fakeBrancher{err: errNoBranch}, "TASK-1"); got != "" {
		t.Fatalf("BaseBranchOf with error = %q, want \"\"", got)
	}
	if got := BaseBranchOf(fakePlain{}, "TASK-1"); got != "" {
		t.Fatalf("BaseBranchOf without the capability = %q, want \"\"", got)
	}
}

type fakeBrancher struct {
	fakePlain
	base string
	err  error
}

func (f fakeBrancher) BaseBranch(string) (string, error) { return f.base, f.err }

type fakePlain struct{}

func (fakePlain) List() ([]Task, error)                         { return nil, nil }
func (fakePlain) Get(string) (Task, error)                      { return Task{}, nil }
func (fakePlain) Create(string, string, string) (string, error) { return "", nil }
func (fakePlain) Comment(string, string) error                  { return nil }
func (fakePlain) Close(string, Verdict) error                   { return nil }

var errNoBranch = fmt.Errorf("no repo behind this queue")
