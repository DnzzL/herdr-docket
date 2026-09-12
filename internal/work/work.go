// Package work is the backend-blind task queue the fleet speaks. A Source is
// wherever work actually lives — a Backlog.md project, a Basecamp list, or
// something not written yet — and a Task is one unit of it, in the fleet's
// own vocabulary. Nothing above an adapter knows which backend answered.
package work

import "sort"

// Task is one unit of work as the fleet sees it.
//
// Open is the only state the fleet reads: a task stays open until a verdict
// closes it, which is all a binary backend (a Basecamp to-do) can express.
// Phase is where it stands in the fleet's words, for display only — written
// best-effort, never read back; the adapter is what translates its backend's
// own label into one. Verdict is how a closed task ended, when the backend can
// still say: a rich backend keeps the verdict, a binary one only knows the
// task is closed and leaves it empty. Priority is how urgent the backend says
// this is, as a rank it computed: only the order matters, and zero is the
// backend having no opinion, which is the least urgent there is. Assignee is a
// plain routing key, not an account: each adapter decides what carries it.
// CreatedAt is the backend's own timestamp, kept as it was sent and compared as
// bytes: it is only the last tie-break, so a format that sorts chronologically
// as text is enough, and parsing it would only add a way for a bad date to lose
// a task.
type Task struct {
	ID        string
	Title     string
	Body      string
	Notes     string
	Assignee  string
	Open      bool
	Phase     string
	Verdict   Verdict
	Priority  int
	Ordinal   float64
	CreatedAt string
	Criteria  []Criterion
}

// Criterion is one acceptance criterion of a task. Criteria are read-only to
// the fleet: an adapter fills them in for the prompt, and the agent's verdict
// and evidence go in its closing comment, not back onto the boxes.
type Criterion struct {
	Index   int
	Text    string
	Checked bool
}

// Verdict is how a run ended. Close always closes the task, whatever the
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
// only question about a task is whether it is open, so a closed task is one
// it will never look at again — and a mistyped verdict would quietly throw
// the work away.
func (v Verdict) Known() bool {
	switch v {
	case Done, Failed, Blocked:
		return true
	}
	return false
}

// Label is how the board shows a verdict: the phase word a closed task stands
// under. A verdict is the word an agent types; this is the word a human reads.
// The two are joined here, which is why the phase order is built out of this
// rather than retyping the three endings beside it.
func (v Verdict) Label() string {
	switch v {
	case Done:
		return "Done"
	case Failed:
		return "Failed"
	case Blocked:
		return "Blocked"
	}
	return ""
}

// Phase is the fleet's word for where a task stands. It is shown to a human
// and decides nothing — Open is what the fleet reads — and an adapter
// translates its backend's own label into one of these on the way in. Display
// only, written best-effort, never read back.
type Phase string

// The two phases of queued work, in the fleet's words. How a task ended is a
// Verdict, and the board stands a closed task under its verdict's label, so
// the three endings are one vocabulary rather than two.
const (
	// PhaseTodo is work nobody has started.
	PhaseTodo Phase = "To Do"
	// PhaseInProgress is work a run has in hand right now, and the one phase
	// the fleet writes.
	PhaseInProgress Phase = "In Progress"
)

// Phaser is the optional capability of a source that can show a task as
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
	List() ([]Task, error)
	Get(id string) (Task, error)
	Create(title, body, assignee string) (string, error)
	Comment(id, text string) error
	Close(id string, verdict Verdict) error
}

// closedOrder is the order the board shows endings in: the ones that still
// want a human first, so they are seen, and the finished ones last. It is the
// fleet's own list of verdicts, which is what makes phaseOrder below derived
// rather than a second spelling of the same three words.
var closedOrder = []Verdict{Blocked, Failed, Done}

// phaseOrder is the order the fleet shows phases in, most actionable first:
// the work still to do, then how things ended. A default rather than a closed
// set — a backend with phases of its own gets them listed after these, in the
// order it reports them. Display only: nothing here decides what runs; Open
// does.
var phaseOrder = buildPhaseOrder()

func buildPhaseOrder() []string {
	order := []string{string(PhaseTodo), string(PhaseInProgress)}
	for _, v := range closedOrder {
		order = append(order, v.Label())
	}
	return order
}

// phaseRank sorts a phase for display. Phases the fleet does not know tie, and
// keep the order they arrived in.
func phaseRank(phase string) int {
	for i, p := range phaseOrder {
		if p == phase {
			return i
		}
	}
	return len(phaseOrder)
}

// PhasesOf lists the phases these tasks actually use, most actionable first —
// the order a board or a listing should show them in. Backend-blind: a phase
// the fleet does not know by name still gets its place, just after the ones it
// does, in the order it turned up.
func PhasesOf(items []Task) []string {
	var phases []string
	seen := map[string]bool{}
	for _, it := range items {
		if !seen[it.Phase] {
			seen[it.Phase] = true
			phases = append(phases, it.Phase)
		}
	}
	sort.SliceStable(phases, func(i, j int) bool { return phaseRank(phases[i]) < phaseRank(phases[j]) })
	return phases
}
