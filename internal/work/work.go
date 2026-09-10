// Package work is the backend-blind task queue the fleet speaks. A Source is
// wherever work actually lives — a Backlog.md project, a Basecamp list, or
// something not written yet — and an Item is one unit of it, in the fleet's
// own vocabulary. Nothing above an adapter knows which backend answered.
package work

import "sort"

// Item is one unit of work as the fleet sees it.
//
// Open is the only state the fleet reads: an item stays open until a verdict
// closes it, which is all a binary backend (a Basecamp to-do) can express.
// Phase is the backend's own status word, carried for display only — written
// best-effort, never read back. Verdict is how a closed item ended, when the
// backend can still say: a rich backend keeps the verdict, a binary one only
// knows the item is closed and leaves it empty. Assignee is a plain routing
// key, not an account: each adapter decides what carries it.
type Item struct {
	ID        string
	Title     string
	Body      string
	Notes     string
	Assignee  string
	Open      bool
	Phase     string
	Verdict   Verdict
	Priority  string
	Ordinal   float64
	CreatedAt string
	Criteria  []Criterion
}

// Criterion is one acceptance criterion of an item. Criteria are read-only to
// the fleet: an adapter fills them in for the prompt, and the agent's verdict
// and evidence go in its closing comment, not back onto the boxes.
type Criterion struct {
	Index   int
	Text    string
	Checked bool
}

// Verdict is how a run ended. Close always closes the item, whatever the
// verdict: a blocked or failed task left open would be handed straight back
// by the next tick, and a binary backend has no other word for it.
type Verdict string

const (
	Done    Verdict = "done"
	Failed  Verdict = "failed"
	Blocked Verdict = "blocked"
)

// Known reports whether v is one of the verdicts the port writes. An adapter
// must refuse anything else rather than treat a typo as a close: the fleet's
// only question about an item is whether it is open, so a closed item is one
// it will never look at again — and a mistyped verdict would quietly throw
// the work away.
func (v Verdict) Known() bool {
	switch v {
	case Done, Failed, Blocked:
		return true
	}
	return false
}

// Phase is the fleet's word for what a run is doing, for the backends that
// can show it. Display only, written best-effort, never read back.
type Phase string

// PhaseRunning is the one phase the fleet writes: this item is being worked
// on right now.
const PhaseRunning Phase = "running"

// Phaser is the optional capability of a source that can show an item as
// being worked on. Backlog.md has an In Progress state to write; a Basecamp
// to-do is simply done or not, and the run lock is what actually keeps two
// runs apart — so a source without this is not a lesser source, just a
// quieter one.
type Phaser interface {
	SetPhase(id string, phase Phase) error
}

// Source is the port the fleet's queue lives behind. Picking, running and the
// board speak only this.
type Source interface {
	List() ([]Item, error)
	Get(id string) (Item, error)
	Create(title, body, assignee string) (string, error)
	Comment(id, text string) error
	Close(id string, verdict Verdict) error
}

// PhaseOrder is the order the fleet shows phases in, most actionable first.
// A default rather than a closed set: a backend with phases of its own gets
// them listed after these, in the order it reports them. Display only —
// nothing here decides what runs; Open does.
var PhaseOrder = []string{"To Do", "In Progress", "Blocked", "Failed", "Done"}

// PhaseRank sorts a phase for display. Phases the fleet does not know tie,
// and keep the order they arrived in.
func PhaseRank(phase string) int {
	for i, p := range PhaseOrder {
		if p == phase {
			return i
		}
	}
	return len(PhaseOrder)
}

// PhasesOf lists the phases these items actually use, most actionable first —
// the order a board or a listing should show them in. Backend-blind: a phase
// the fleet does not know by name still gets its place, just after the ones it
// does, in the order it turned up.
func PhasesOf(items []Item) []string {
	var phases []string
	seen := map[string]bool{}
	for _, it := range items {
		if !seen[it.Phase] {
			seen[it.Phase] = true
			phases = append(phases, it.Phase)
		}
	}
	sort.SliceStable(phases, func(i, j int) bool { return PhaseRank(phases[i]) < PhaseRank(phases[j]) })
	return phases
}
