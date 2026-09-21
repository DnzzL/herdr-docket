// Package backlogmd adapts a Backlog.md project to work.Source. Backlog.md is
// the fleet's default queue: a git repo of markdown tasks, driven through the
// backlog CLI, one status word per task. Everything Backlog.md-specific lives
// here — nothing above an adapter knows the word "backlog".
package backlogmd

import (
	"fmt"
	"strings"

	"github.com/DnzzL/herdr-docket/internal/work"
)

// client is the slice of the Backlog.md CLI the adapter needs. The real one
// shells out; tests stand in for it, which is the only way to test the mapping
// without a backlog project on disk.
type client interface {
	List() ([]task, error)
	View(id string) (view, error)
	Create(title, body, assignee string) (string, error)
	SetStatus(id, status string) error
	SetAssignee(id, agent string) error
	AppendNote(id, note string) error
}

// Source is a work.Source backed by a Backlog.md project. The vocabulary is
// that project's own status words: the fleet's queue uses the fleet's, a
// project the fleet merely works uses whatever it already declares.
type Source struct {
	client client
	vocab  Vocabulary
}

// New returns a Source on the Backlog.md project at dir, speaking vocab. The
// zero Vocabulary is the fleet's own words.
func New(dir string, vocab Vocabulary) *Source { return newWith(newCLI(dir), vocab) }

func newWith(c client, vocab Vocabulary) *Source {
	return &Source{client: c, vocab: vocab.OrDefault()}
}

// List returns every task in the project, closed ones included — the board
// shows them, and only the queue cares about Open.
func (s *Source) List() ([]work.Task, error) {
	tasks, err := s.client.List()
	if err != nil {
		return nil, err
	}
	items := make([]work.Task, 0, len(tasks))
	for _, t := range tasks {
		items = append(items, s.asTask(t))
	}
	return items, nil
}

// Get returns one task in full: what List carries plus the body, the notes
// and the criteria the prompt needs.
func (s *Source) Get(id string) (work.Task, error) {
	v, err := s.client.View(id)
	if err != nil {
		return work.Task{}, err
	}
	it := s.asTask(v.task)
	it.Body = v.Description
	it.Notes = v.ImplementationNotes
	for _, c := range v.AcceptanceCriteria {
		it.Criteria = append(it.Criteria, work.Criterion{Index: c.Index, Text: c.Text, Checked: c.Checked})
	}
	return it, nil
}

// Create adds a task to the project and returns its new id.
func (s *Source) Create(title, body, assignee string) (string, error) {
	return s.client.Create(title, body, assignee)
}

// Comment appends to the task's notes. Backlog.md has no separate comment
// stream, and the notes are where a run's account of itself belongs.
func (s *Source) Comment(id, text string) error {
	return s.client.AppendNote(id, text)
}

// Close ends the task with a verdict. Backlog.md can say Done, Failed or
// Blocked, so the hardware word survives; the task is closed to the fleet
// either way.
func (s *Source) Close(id string, v work.Verdict) error {
	if !v.Known() {
		return fmt.Errorf("close %s: unknown verdict %q", id, v)
	}
	// status is total over the known verdicts, which the test beside this
	// one keeps it.
	return s.client.SetStatus(id, s.vocab.status(v))
}

// status is the translation from the port's verdict to the word that records
// it in this project. An adapter that can only say one thing about an ending
// (a binary completed flag) has no such table — it asks the port whether the
// verdict is known and records whatever it can.
func (v Vocabulary) status(verdict work.Verdict) string {
	switch verdict {
	case work.Done:
		return v.Done
	case work.Failed:
		return v.Failed
	case work.Blocked:
		return v.Blocked
	}
	return ""
}

// verdictOf is the same table read backwards: what a closed task's status
// word still says about how it ended. Backlog.md is rich enough to keep the
// verdict, which is how the runner knows whether to tear down a run's
// workspace or leave it open to resume. A binary backend cannot, and simply
// leaves Verdict empty. Ordered, not a map: two phases of a project may share
// one word, and which verdict that word means must not depend on map order.
func (v Vocabulary) verdictOf(status string) work.Verdict {
	switch status {
	case "":
		return ""
	case v.Done:
		return work.Done
	case v.Failed:
		return work.Failed
	case v.Blocked:
		return work.Blocked
	}
	return ""
}

// Assign re-routes the task to another agent. Backlog.md holds a list of
// assignees and the fleet reads the first, so assigning replaces the list
// rather than adding to it: one task, one agent, which is what the routing
// rule assumes.
func (s *Source) Assign(id, agent string) error { return s.client.SetAssignee(id, agent) }

// SetPhase shows the task as being worked on. This is the only phase the
// fleet ever writes, and it is display only — the run lock is what keeps two
// runs apart, so nothing reads this back. A project with no word for it is
// left alone: writing one its CLI rejects would fail a run over decoration.
func (s *Source) SetPhase(id string, phase work.Phase) error {
	if phase != work.PhaseInProgress {
		return fmt.Errorf("phase %q: the backlogmd adapter does not know it", phase)
	}
	if s.vocab.InProgress == "" {
		return nil
	}
	return s.client.SetStatus(id, s.vocab.InProgress)
}

// asTask maps a Backlog.md task onto the fleet's vocabulary. A task is open
// until a verdict has closed it: Done, Failed and Blocked are all closed to
// the fleet, and Phase carries the fleet's word for wherever it stands.
func (s *Source) asTask(t task) work.Task {
	return work.Task{
		ID:        t.ID,
		Title:     t.Title,
		Assignee:  assignee(t.Assignees),
		Open:      s.vocab.open(t.Status),
		Phase:     s.vocab.phase(t.Status),
		Verdict:   s.vocab.verdictOf(t.Status),
		Priority:  rank(t.Priority),
		Ordinal:   t.Ordinal,
		CreatedAt: t.CreatedAt,
	}
}

// phase reads a status in the fleet's words. A closed status stands under its
// verdict's word, because the three endings are one vocabulary and not two;
// an open one under the fleet's name for it. A status this project has that
// the fleet was never told about still shows, under its own name: the board
// renders the phases it does not know after the ones it does.
func (v Vocabulary) phase(status string) string {
	if verdict := v.verdictOf(status); verdict != "" {
		return verdict.Label()
	}
	switch status {
	case "":
		return ""
	case v.Todo:
		return string(work.PhaseTodo)
	case v.InProgress:
		return string(work.PhaseInProgress)
	}
	return status
}

// open reports whether a status is work the fleet may pick up. A whitelist,
// deliberately: a project has columns the fleet has no business in — triage,
// wontfix, waiting on a human — and reading anything unrecognised as open
// would put an agent on them.
func (v Vocabulary) open(status string) bool {
	if status == "" {
		return false
	}
	return status == v.Todo || status == v.InProgress
}

// rank reads a Backlog.md priority as a rank the core can compare. Higher is
// more urgent, and anything this adapter does not recognise — including no
// priority at all — is zero, which is the backend having no opinion. These are
// Backlog.md's own words, its built-in default, and they stop here.
func rank(priority string) int {
	switch priority {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	}
	return 0
}

// assignee is the task's routing key: the first name on its assignee list,
// with Backlog.md's sigil taken off. `@name` is how its CLI prints an assignee
// and how it accepts one, so the sigil lands in the file — TASK-91 on a real
// board carried '@dishnow-reviewer' — but an agent is a folder in the fleet dir
// and its name has no `@` in it. The translation belongs here, at the edge,
// with the rest of the backend's conventions: `pick` decides whose work a task
// is, the board prints the name, and neither should have to know how one
// backend decorates one. Doing it in pick would cover every backend at once,
// including those not written yet, and would teach the core a convention it has
// no business knowing. A name the fleet cannot match comes back as itself,
// unknown and readable — not stripped to nothing, which would hand the task to
// the default agent and look like routing.
func assignee(xs []string) string {
	if len(xs) == 0 {
		return ""
	}
	if name := strings.TrimPrefix(xs[0], "@"); name != "" {
		return name
	}
	return xs[0]
}
