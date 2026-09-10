// Package backlogmd adapts a Backlog.md project to work.Source. Backlog.md is
// the fleet's default queue: a git repo of markdown tasks, driven through the
// backlog CLI, one status word per task.
package backlogmd

import (
	"github.com/DnzzL/herdr-fleet/internal/backlog"
	"github.com/DnzzL/herdr-fleet/internal/work"
)

// Client is the slice of the Backlog.md client the adapter needs. The real
// one shells out to the CLI; tests stand in for it, which is the only way to
// test the mapping without a backlog project on disk.
type Client interface {
	List() ([]backlog.Task, error)
	View(id string) (backlog.View, error)
}

// Source is a work.Source backed by a Backlog.md project.
type Source struct{ client Client }

// New returns a Source on the Backlog.md project at dir.
func New(dir string) *Source { return newWith(backlog.New(dir)) }

func newWith(c Client) *Source { return &Source{client: c} }

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
	it := item(v.Task)
	it.Body = v.Description
	it.Notes = v.ImplementationNotes
	for _, c := range v.AcceptanceCriteria {
		it.Criteria = append(it.Criteria, work.Criterion{Index: c.Index, Text: c.Text, Checked: c.Checked})
	}
	return it, nil
}

// item maps a Backlog.md task onto the fleet's vocabulary. A task is open
// until a verdict has closed it: Done, Failed and Blocked are all closed to
// the fleet, and the native word survives in Phase for the board to show.
func item(t backlog.Task) work.Item {
	return work.Item{
		ID:        t.ID,
		Title:     t.Title,
		Assignee:  first(t.Assignees),
		Open:      open(t.Status),
		Phase:     t.Status,
		Priority:  t.Priority,
		Ordinal:   t.Ordinal,
		CreatedAt: t.CreatedAt,
	}
}

func open(status string) bool {
	switch status {
	case backlog.StatusDone, backlog.StatusFailed, backlog.StatusBlocked:
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
