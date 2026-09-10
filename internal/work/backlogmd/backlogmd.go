// Package backlogmd adapts a Backlog.md project to work.Source. Backlog.md is
// the fleet's default queue: a git repo of markdown tasks, driven through the
// backlog CLI, one status word per task. Everything Backlog.md-specific lives
// here — nothing above an adapter knows the word "backlog".
package backlogmd

import (
	"fmt"

	"github.com/DnzzL/herdr-fleet/internal/work"
)

// client is the slice of the Backlog.md CLI the adapter needs. The real one
// shells out; tests stand in for it, which is the only way to test the mapping
// without a backlog project on disk.
type client interface {
	List() ([]task, error)
	View(id string) (view, error)
	Create(title, body, assignee string) (string, error)
	SetStatus(id, status string) error
	AppendNote(id, note string) error
}

// Source is a work.Source backed by a Backlog.md project.
type Source struct{ client client }

// New returns a Source on the Backlog.md project at dir.
func New(dir string) *Source { return newWith(newCLI(dir)) }

func newWith(c client) *Source { return &Source{client: c} }

// List returns every item in the project, closed ones included — the board
// shows them, and only the queue cares about Open.
func (s *Source) List() ([]work.Item, error) {
	tasks, err := s.client.List()
	if err != nil {
		return nil, err
	}
	items := make([]work.Item, 0, len(tasks))
	for _, t := range tasks {
		items = append(items, item(t))
	}
	return items, nil
}

// Get returns one item in full: what List carries plus the body, the notes
// and the criteria the prompt needs.
func (s *Source) Get(id string) (work.Item, error) {
	v, err := s.client.View(id)
	if err != nil {
		return work.Item{}, err
	}
	it := item(v.task)
	it.Body = v.Description
	it.Notes = v.ImplementationNotes
	for _, c := range v.AcceptanceCriteria {
		it.Criteria = append(it.Criteria, work.Criterion{Index: c.Index, Text: c.Text, Checked: c.Checked})
	}
	return it, nil
}

// Create adds an item to the project and returns its new id.
func (s *Source) Create(title, body, assignee string) (string, error) {
	return s.client.Create(title, body, assignee)
}

// Comment appends to the item's notes. Backlog.md has no separate comment
// stream, and the notes are where a run's account of itself belongs.
func (s *Source) Comment(id, text string) error {
	return s.client.AppendNote(id, text)
}

// Close ends the item with a verdict. Backlog.md can say Done, Failed or
// Blocked, so the hardware word survives; the item is closed to the fleet
// either way.
func (s *Source) Close(id string, v work.Verdict) error {
	status, ok := verdicts[v]
	if !ok {
		return fmt.Errorf("close %s: unknown verdict %q", id, v)
	}
	return s.client.SetStatus(id, status)
}

var verdicts = map[work.Verdict]string{
	work.Done:    statusDone,
	work.Failed:  statusFailed,
	work.Blocked: statusBlocked,
}

// verdictOf is the same table read backwards: what a closed item's status
// word still says about how it ended. Backlog.md is rich enough to keep the
// verdict, which is how the runner knows whether to tear down a run's
// workspace or leave it open to resume. A binary backend cannot, and simply
// leaves Verdict empty.
func verdictOf(status string) work.Verdict {
	for v, s := range verdicts {
		if s == status {
			return v
		}
	}
	return ""
}

// SetPhase shows the item as being worked on. Backlog.md has a state for it;
// this is the only phase the fleet ever writes, and it is display only — the
// run lock is what keeps two runs apart, so nothing reads this back.
func (s *Source) SetPhase(id string, phase work.Phase) error {
	status, ok := phases[phase]
	if !ok {
		return fmt.Errorf("phase %q: the backlogmd adapter does not know it", phase)
	}
	return s.client.SetStatus(id, status)
}

var phases = map[work.Phase]string{
	work.PhaseRunning: statusInProgress,
}

// item maps a Backlog.md task onto the fleet's vocabulary. A task is open
// until a verdict has closed it: Done, Failed and Blocked are all closed to
// the fleet, and the native word survives in Phase for the board to show.
func item(t task) work.Item {
	return work.Item{
		ID:        t.ID,
		Title:     t.Title,
		Assignee:  first(t.Assignees),
		Open:      open(t.Status),
		Phase:     t.Status,
		Verdict:   verdictOf(t.Status),
		Priority:  t.Priority,
		Ordinal:   t.Ordinal,
		CreatedAt: t.CreatedAt,
	}
}

func open(status string) bool {
	switch status {
	case statusDone, statusFailed, statusBlocked:
		return false
	}
	return true
}

func first(xs []string) string {
	if len(xs) == 0 {
		return ""
	}
	return xs[0]
}
