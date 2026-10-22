package backlogmd

import "github.com/DnzzL/herdr-docket/internal/work"

// Vocabulary is one project's status words, in the fleet's terms. Backlog.md
// validates every write against the statuses the project declares, and a
// project the fleet did not lay down has its own. The table itself is shared
// with every queue that keeps status words (ADR-0022); what is Backlog.md's
// alone is the default below.
type Vocabulary = work.Vocabulary

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

// OrDefault reads an absent statuses block as the fleet's own words, so a
// fleet that never heard of this setting keeps working exactly as before.
func OrDefault(v Vocabulary) Vocabulary {
	if v == (Vocabulary{}) {
		return DefaultVocabulary()
	}
	return v
}
