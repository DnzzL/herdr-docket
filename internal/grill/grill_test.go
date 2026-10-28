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
	n, qs, ok := LatestRound(notes)
	if !ok || n != 2 {
		t.Fatalf("LatestRound = %d, %v, want round 2", n, ok)
	}
	want := []Question{
		{N: 1, Text: "What happens to the audio when it is sent?", Options: []Option{
			{Letter: "A", Text: "attached as a file", Reco: true},
			{Letter: "B", Text: "text in a comment"},
			{Letter: "T", Text: "try and show me"},
		}},
		{N: 2, Text: "Keep it how long?"},
	}
	if !reflect.DeepEqual(qs, want) {
		t.Fatalf("questions =\n%+v\nwant\n%+v", qs, want)
	}
}

// A GitHub thread prefixes each comment with its author.
func TestLatestRoundReadsAnAuthoredThread(t *testing.T) {
	notes := "triager: ## Round 3\n1. Ship it?\n   A. yes (reco)\n\nowner: ## Answer 3\n1.A"
	n, qs, ok := LatestRound(notes)
	if !ok || n != 3 || len(qs) != 1 || len(qs[0].Options) != 1 {
		t.Fatalf("LatestRound = %d, %+v, %v", n, qs, ok)
	}
}

func TestLatestRoundWithoutARoundIsNotOK(t *testing.T) {
	if _, _, ok := LatestRound("just a note\n## Answer 1\n1.A"); ok {
		t.Fatal("no Round heading must not be a round")
	}
}
