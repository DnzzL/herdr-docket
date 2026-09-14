package fleet

import "github.com/DnzzL/herdr-docket/internal/work"

// Words are the status words one queue writes, in that queue's own language.
// The fleet already owns them — a source's statuses: block, or the default —
// and the prompt names them so an agent editing its project's board uses the
// word that board accepts. Before this, every persona retyped them by hand and
// nothing kept the two in step: a project that renamed a column left its agent
// writing a status the CLI refuses.
//
// Only the words the fleet writes itself. A board's triage, wontfix and
// waiting-on-a-human columns are not named here for the same reason they are
// not in the whitelist: the fleet never puts a task in them, and a vocabulary
// it does not write is not its to teach.
type Words struct {
	Todo, InProgress, Done, Failed, Blocked string
}

// Known reports whether there is anything worth telling an agent. A queue
// whose backend has no status words — a Basecamp to-do is done or not — has
// none, and the prompt simply says nothing about them.
func (w Words) Known() bool { return w.Todo != "" }

// WordsFor is the vocabulary of the queue a task came from. The prefix names
// the source; a fleet with one unnamed queue answers from its own source
// block. A backend that does not speak in statuses returns the zero Words.
func (s Settings) WordsFor(taskID string) Words {
	c, ok := s.sourceOf(taskID)
	if !ok || c.kind() != kindBacklogmd {
		return Words{}
	}
	v := c.Statuses.OrDefault()
	return Words{
		Todo:       v.Todo,
		InProgress: v.InProgress,
		Done:       v.Done,
		Failed:     v.Failed,
		Blocked:    v.Blocked,
	}
}

// sourceOf is the source block a task's id points at: the named one for a
// prefixed id, the fleet's single source otherwise.
func (s Settings) sourceOf(taskID string) (SourceConfig, bool) {
	if len(s.Sources) == 0 {
		return s.Source, true
	}
	c, ok := s.Sources[work.SourceOf(taskID)]
	return c, ok
}
