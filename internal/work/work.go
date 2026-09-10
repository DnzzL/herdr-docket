// Package work is the backend-blind task queue the fleet speaks. A Source is
// wherever work actually lives — a Backlog.md project, a Basecamp list, or
// something not written yet — and an Item is one unit of it, in the fleet's
// own vocabulary. Nothing above an adapter knows which backend answered.
package work

// Item is one unit of work as the fleet sees it.
//
// Open is the only state the fleet reads: an item stays open until a verdict
// closes it, which is all a binary backend (a Basecamp to-do) can express.
// Phase is the backend's own status word, carried for display only — written
// best-effort, never read back. Assignee is a plain routing key, not an
// account: each adapter decides what carries it.
type Item struct {
	ID        string
	Title     string
	Body      string
	Notes     string
	Assignee  string
	Open      bool
	Phase     string
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

// Source is the port the fleet's queue lives behind. Picking, running and the
// board speak only this.
type Source interface {
	List() ([]Item, error)
	Get(id string) (Item, error)
}
