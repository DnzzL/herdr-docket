package backlogmd

import "fmt"

// Vocabulary is one project's status words, in the fleet's terms. Backlog.md
// validates every write against the statuses the project declares, and a
// project the fleet did not lay down has its own — notara says
// `ready-for-agent` where the fleet says `To Do`, and has no word at all for
// work in hand. This is the whole translation, and it stops here.
//
// Read it as a whitelist: a status not named below is not the fleet's
// business. That matters more than it sounds — a project's triage and wontfix
// columns are statuses too, and a fleet that read them as open work would run
// an agent on them.
type Vocabulary struct {
	Todo       string `yaml:"todo"`
	InProgress string `yaml:"in_progress"`
	Done       string `yaml:"done"`
	Failed     string `yaml:"failed"`
	Blocked    string `yaml:"blocked"`
}

// DefaultVocabulary is the lifecycle `fleet init` writes into a project it
// laid down itself, and what a source that names no statuses gets.
func DefaultVocabulary() Vocabulary {
	return Vocabulary{
		Todo:       statusToDo,
		InProgress: statusInProgress,
		Done:       statusDone,
		Failed:     statusFailed,
		Blocked:    statusBlocked,
	}
}

// orDefault reads an absent statuses block as the fleet's own words, so a
// fleet that never heard of this setting keeps working exactly as before.
func (v Vocabulary) OrDefault() Vocabulary {
	if v == (Vocabulary{}) {
		return DefaultVocabulary()
	}
	return v
}

// Validate refuses a half-written vocabulary. Every ending must have a word:
// a queue the fleet can pick from but cannot close leaves each task open when
// the run ends, and the daemon picks it straight back up. in_progress is the
// one word a project may leave out — it is display only, and a source without
// it is simply quieter.
func (v Vocabulary) Validate() error {
	if v == (Vocabulary{}) {
		return nil
	}
	for _, f := range []struct{ name, word string }{
		{"todo", v.Todo},
		{"done", v.Done},
		{"failed", v.Failed},
		{"blocked", v.Blocked},
	} {
		if f.word == "" {
			return fmt.Errorf("statuses: names no %s — every phase and ending the fleet writes needs a word this project accepts", f.name)
		}
	}
	return nil
}

// List is the vocabulary in board order, for whoever has to write it down.
// A word the project does not have is left out rather than written empty.
func (v Vocabulary) List() []string {
	out := make([]string, 0, 5)
	for _, s := range []string{v.Todo, v.InProgress, v.Blocked, v.Failed, v.Done} {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
