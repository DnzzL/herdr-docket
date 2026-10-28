package work

import "fmt"

// Vocabulary is one queue's status words, in the fleet's terms. A queue the
// fleet did not lay down has its own — notara says `ready-for-agent` where
// the fleet says `To Do` — and this is the whole translation: Backlog.md and
// Notara read the same `statuses:` block through it (ADR-0022).
//
// Read it as a whitelist: a status not named below is not the fleet's
// business. That matters more than it sounds — a project's triage and wontfix
// columns are statuses too, and a fleet that read them as open work would run
// an agent on them.
type Vocabulary struct {
	Todo       string `yaml:"todo"`
	InProgress string `yaml:"in_progress"`
	// Triage and Question are the grill's two words (TASK-86): a card in
	// Triage waits for the queue's triager, and one in Question waits for the
	// owner's answer. Both are open work — the card is not closed while it is
	// being grilled — and Question is the one status the fleet never picks up.
	// A queue that names neither is simply not grilled, and its triage column
	// (if it has one) stays the humans'.
	Triage   string `yaml:"triage"`
	Question string `yaml:"question"`
	Done     string `yaml:"done"`
	Failed   string `yaml:"failed"`
	Blocked  string `yaml:"blocked"`
}

// Validate refuses a half-written vocabulary. Every ending must have a word:
// a queue the fleet can pick from but cannot close leaves each task open when
// the run ends, and the daemon picks it straight back up. in_progress is the
// one word a project may leave out — it is display only, and a source without
// it is simply quieter. The zero vocabulary passes: what it means is the
// adapter's own default.
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
	// The grill's two words are one loop: a card sent to Question with no
	// Triage to come back to is work that never returns, and a Triage with no
	// Question to park in cannot ask. Both or neither.
	if (v.Triage == "") != (v.Question == "") {
		return fmt.Errorf("statuses: names one of triage/question — the grill needs both a word to wait for its triager in and one to wait for the owner's answer in")
	}
	return nil
}

// List is the vocabulary in board order, for whoever has to write it down.
// A word the project does not have is left out rather than written empty.
func (v Vocabulary) List() []string {
	out := make([]string, 0, 7)
	for _, s := range []string{v.Todo, v.InProgress, v.Triage, v.Question, v.Blocked, v.Failed, v.Done} {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// Status is the translation from the port's verdict to the word that records
// it in this project. An adapter that can only say one thing about an ending
// (a binary completed flag) has no such table — it asks the port whether the
// verdict is known and records whatever it can.
func (v Vocabulary) Status(verdict Verdict) string {
	switch verdict {
	case Done:
		return v.Done
	case Failed:
		return v.Failed
	case Blocked:
		return v.Blocked
	}
	return ""
}

// VerdictOf is the same table read backwards: what a closed task's status
// word still says about how it ended. A queue rich enough to keep the verdict
// is how the runner knows whether to tear down a run's workspace or leave it
// open to resume. Ordered, not a map: two phases of a project may share one
// word, and which verdict that word means must not depend on map order.
func (v Vocabulary) VerdictOf(status string) Verdict {
	switch status {
	case "":
		return ""
	case v.Done:
		return Done
	case v.Failed:
		return Failed
	case v.Blocked:
		return Blocked
	}
	return ""
}

// Phase reads a status in the fleet's words. A closed status stands under its
// verdict's word, because the three endings are one vocabulary and not two;
// an open one under the fleet's name for it. A status this project has that
// the fleet was never told about still shows, under its own name: the board
// renders the phases it does not know after the ones it does.
func (v Vocabulary) Phase(status string) string {
	if verdict := v.VerdictOf(status); verdict != "" {
		return verdict.Label()
	}
	switch status {
	case "":
		return ""
	case v.Todo:
		return string(PhaseTodo)
	case v.InProgress:
		return string(PhaseInProgress)
	case v.Triage:
		return string(PhaseTriage)
	case v.Question:
		return string(PhaseQuestion)
	}
	return status
}

// Open reports whether a status is work the fleet still owes. A whitelist,
// deliberately: a project has columns the fleet has no business in — wontfix,
// waiting on a human, a triage column it was never told about — and reading
// anything unrecognised as open would put an agent on them.
//
// The grill's two words are open: a triaged card is the triager's to work and
// a question card still owes an answer. What keeps a Question card from being
// run is Asking, not Open — the daemon reads it and never picks it (TASK-86).
func (v Vocabulary) Open(status string) bool {
	if status == "" {
		return false
	}
	return status == v.Todo || status == v.InProgress || v.Triaging(status) || v.Asking(status)
}

// Triaging reports whether a status is the word a card waits for the triager
// in. A queue that names no such word never grills, and no column of it is
// read as one by guess.
func (v Vocabulary) Triaging(status string) bool {
	return status != "" && status == v.Triage
}

// Asking reports whether a status is the word a card waits for the owner's
// answer in. The fleet never picks such a card: the answer only a person can
// give does not come from another run.
func (v Vocabulary) Asking(status string) bool {
	return status != "" && status == v.Question
}

// Word is the table read the other way: the project's word for a phase the
// fleet writes, empty when the project has none. The one phase↔word mapping
// every adapter with a status vocabulary writes through.
func (v Vocabulary) Word(p Phase) string {
	switch p {
	case PhaseTodo:
		return v.Todo
	case PhaseInProgress:
		return v.InProgress
	case PhaseTriage:
		return v.Triage
	case PhaseQuestion:
		return v.Question
	}
	return ""
}

// Parked reports whether a status is the one that waits on a human — this
// project's own blocked word. Written only where the queue names one, a
// task the vocabulary does not know at all is not parked by guess: the
// whitelist above already leaves it invisible to routing, and inventing a
// parked state for a column the fleet never mapped would be a second
// vocabulary, not a translation.
func (v Vocabulary) Parked(status string) bool {
	return status != "" && status == v.Blocked
}
