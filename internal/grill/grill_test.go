package grill

import (
	"reflect"
	"testing"
)

// The newest round wins, its questions numbered and its options lettered,
// the triager's recommendation marked. The note thread is a backlogmd one:
// notes one after the other, each starting with its heading.
func TestLatestRoundReadsTheNewestRound(t *testing.T) {
	notes := "## Round 1\n1. Old question?\n   A. yes\n\n## Answer 1\n1.A\n\n## Round 2\n" +
		"1. What happens to the audio when it is sent?\n" +
		"   A. attached as a file (reco)\n" +
		"   B. text in a comment\n" +
		"   T. try and show me\n" +
		"2. Keep it how long?\n"
	r, ok := LatestRound(notes)
	if !ok || r.N != 2 || r.Context != "" {
		t.Fatalf("LatestRound = %+v, %v, want round 2 without context", r, ok)
	}
	want := []Question{
		{N: 1, Text: "What happens to the audio when it is sent?", Options: []Option{
			{Letter: "A", Text: "attached as a file", Reco: true},
			{Letter: "B", Text: "text in a comment"},
			{Letter: "T", Text: "try and show me"},
		}},
		{N: 2, Text: "Keep it how long?"},
	}
	if !reflect.DeepEqual(r.Questions, want) {
		t.Fatalf("questions =\n%+v\nwant\n%+v", r.Questions, want)
	}
}

// A GitHub thread prefixes each comment with its author.
func TestLatestRoundReadsAnAuthoredThread(t *testing.T) {
	notes := "triager: ## Round 3\n1. Ship it?\n   A. yes (reco)\n\nowner: ## Answer 3\n1.A"
	r, ok := LatestRound(notes)
	if !ok || r.N != 3 || len(r.Questions) != 1 || len(r.Questions[0].Options) != 1 {
		t.Fatalf("LatestRound = %+v, %v", r, ok)
	}
}

// The lines between the heading and question 1 are the round's context:
// blank lines dropped, each line trimmed, the author prefix ignored.
func TestLatestRoundReadsItsContext(t *testing.T) {
	notes := "triager: ## Round 4\n\n  The upload sends audio as text today.  \nFiles: upload.go, notes.go\n\n" +
		"1. Ship it?\n   A. yes (reco)\n"
	r, ok := LatestRound(notes)
	if want := "The upload sends audio as text today.\nFiles: upload.go, notes.go"; !ok || r.Context != want {
		t.Fatalf("Context = %q, want %q", r.Context, want)
	}
	if len(r.Questions) != 1 || r.Questions[0].Text != "Ship it?" {
		t.Fatalf("questions = %+v", r.Questions)
	}
}

func TestLatestRoundWithoutARoundIsNotOK(t *testing.T) {
	if _, ok := LatestRound("just a note\n## Answer 1\n1.A"); ok {
		t.Fatal("no Round heading must not be a round")
	}
}
