package backlogmd

import (
	"strings"
	"testing"
)

// A project that keeps its own status words is still a queue the fleet can
// work. The words travel in the config; nothing above the adapter learns them.
func TestAProjectsOwnWordsMapOntoTheFleetsPhases(t *testing.T) {
	v := Vocabulary{
		Todo:   "ready-for-agent",
		Done:   "done",
		Failed: "ready-for-human",
		// no in_progress, no blocked: notara has neither
	}
	s := newWith(&fakeClient{tasks: []task{
		{ID: "T-1", Status: "ready-for-agent"},
		{ID: "T-2", Status: "done"},
		{ID: "T-3", Status: "needs-triage"},
		{ID: "T-4", Status: "wontfix"},
	}}, v)
	items, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	// Whitelist, not blacklist: a status the fleet was never told about is
	// none of its business. Read the other way round, the fleet would pick up
	// triage and wontfix and run an agent on them.
	wantOpen := []bool{true, false, false, false}
	wantPhase := []string{"To Do", "Done", "needs-triage", "wontfix"}
	for i, it := range items {
		if it.Open != wantOpen[i] {
			t.Errorf("%s (%s): Open = %v, want %v", it.ID, it.Phase, it.Open, wantOpen[i])
		}
		if it.Phase != wantPhase[i] {
			t.Errorf("%s: Phase = %q, want %q", it.ID, it.Phase, wantPhase[i])
		}
	}
}

// A project with no word for work in hand simply is not shown work in hand.
// Writing one anyway means writing a status the project's CLI rejects, which
// fails a run over something that is display only.
func TestAPhaseWithNoWordIsNotWritten(t *testing.T) {
	c := &fakeClient{}
	s := newWith(c, Vocabulary{Todo: "ready-for-agent", Done: "done", Failed: "ready-for-human"})
	if err := s.SetPhase("T-1", "In Progress"); err != nil {
		t.Fatalf("a phase the project has no word for is not an error: %v", err)
	}
	if len(c.stages) != 0 {
		t.Fatalf("wrote %v, want nothing written", c.stages)
	}
}

// The four words the fleet cannot work without. A queue it can pick from but
// not close ends every run by leaving the task open, and the daemon picks it
// straight back up.
func TestAVocabularyMustNameEveryEnding(t *testing.T) {
	for _, tc := range []struct {
		name string
		v    Vocabulary
		want string
	}{
		{"no todo", Vocabulary{Done: "d", Failed: "f", Blocked: "b"}, "todo"},
		{"no done", Vocabulary{Todo: "t", Failed: "f", Blocked: "b"}, "done"},
		{"no failed", Vocabulary{Todo: "t", Done: "d", Blocked: "b"}, "failed"},
		{"no blocked", Vocabulary{Todo: "t", Done: "d", Failed: "f"}, "blocked"},
	} {
		err := tc.v.Validate()
		if err == nil {
			t.Errorf("%s: want an error", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: the error must name %q, got %q", tc.name, tc.want, err)
		}
	}
	full := Vocabulary{Todo: "t", Done: "d", Failed: "f", Blocked: "b"}
	if err := full.Validate(); err != nil {
		t.Errorf("a vocabulary naming every ending is valid, got %v", err)
	}
	// An empty block is not a half-written one: it is the fleet's own words.
	if err := (Vocabulary{}).Validate(); err != nil {
		t.Errorf("no statuses block at all is the default, got %v", err)
	}
	if got := (Vocabulary{}).OrDefault(); got != DefaultVocabulary() {
		t.Errorf("orDefault = %+v, want the fleet's own words", got)
	}
}
