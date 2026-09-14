package fleet

import (
	"testing"

	"github.com/DnzzL/herdr-docket/internal/work/backlogmd"
)

func vocab(todo, inProgress, done, failed, blocked string) backlogmd.Vocabulary {
	return backlogmd.Vocabulary{Todo: todo, InProgress: inProgress, Done: done, Failed: failed, Blocked: blocked}
}

// The words follow the task's queue, because two projects behind one fleet do
// not have to speak the same language — that is the whole point of statuses:.
func TestWordsFollowTheTasksQueue(t *testing.T) {
	s := Settings{Sources: map[string]SourceConfig{
		"notara": {Statuses: vocab("ready-for-agent", "", "done", "ready-for-human", "needs-info")},
		"plain":  {}, // no statuses block: the fleet's own words
		"bc":     {Kind: kindBasecamp},
	}}
	if got := s.WordsFor("notara/NOT-92").Todo; got != "ready-for-agent" {
		t.Errorf("notara todo = %q", got)
	}
	if got := s.WordsFor("notara/NOT-92").InProgress; got != "" {
		t.Errorf("a project with no word for work in hand teaches none, got %q", got)
	}
	if got := s.WordsFor("plain/TASK-1").Todo; got != "To Do" {
		t.Errorf("plain todo = %q, want the fleet's own word", got)
	}
	// A backend with no status words has nothing to teach, and the prompt must
	// not invent Backlog.md's words for it.
	if w := s.WordsFor("bc/987"); w.Known() {
		t.Errorf("basecamp has no vocabulary, got %+v", w)
	}
	if w := s.WordsFor("nosuch/TASK-1"); w.Known() {
		t.Errorf("an unknown queue teaches nothing, got %+v", w)
	}
}

// One unnamed queue is the fleet before sources: existed, and still answers.
func TestOneQueueAnswersFromItsOwnSourceBlock(t *testing.T) {
	if got := (Settings{}).WordsFor("TASK-1").Todo; got != "To Do" {
		t.Errorf("todo = %q, want the default", got)
	}
}
