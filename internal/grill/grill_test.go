package grill

import (
	"reflect"
	"strings"
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

// An answer line is what `task answer` posts and the triager reads: a letter
// sticks to its number, free text follows it after a space.
func TestLineJoinsLettersAndFreeText(t *testing.T) {
	got := Line([]Answer{{N: 1, Letter: "A"}, {N: 2, Text: "  only the owner \n"}, {N: 3, Letter: "T"}})
	if want := "1.A 2. only the owner 3.T"; got != want {
		t.Fatalf("Line = %q, want %q", got, want)
	}
}

// The confirm round lists the triager's assumptions above its one question.
// Numbered assumptions are context, not questions: a question is the line a
// run of lettered options hangs from, numbered or not (TASK-93..95 shipped
// rounds the TUI could not send).
func TestLatestRoundTakesOnlyALineWithOptionsAsAQuestion(t *testing.T) {
	for name, notes := range map[string]string{
		"numbered question":   "## Round 2\nSettled.\n\nASSUMPTIONS:\n1. Hierarchy is a heading per block.\n2. Colour is semantic.\n\n3. Build on these assumptions?\n   A. yes (reco)\n   B. change …\n",
		"unnumbered question": "## Round 2\nSettled.\n\nASSUMPTIONS:\n1. Hierarchy is a heading per block.\n2. Colour is semantic.\n\nBuild on these assumptions?\nA. yes (reco)\nB. change …\n",
	} {
		t.Run(name, func(t *testing.T) {
			r, ok := LatestRound(notes)
			if !ok {
				t.Fatal("no round")
			}
			if len(r.Questions) != 1 {
				t.Fatalf("questions = %+v, want only the confirm question", r.Questions)
			}
			q := r.Questions[0]
			if q.Text != "Build on these assumptions?" || len(q.Options) != 2 || !q.Options[0].Reco {
				t.Errorf("question = %+v", q)
			}
			if !strings.Contains(r.Context, "2. Colour is semantic.") || !strings.Contains(r.Context, "ASSUMPTIONS:") {
				t.Errorf("context lost the assumptions: %q", r.Context)
			}
			if got := Line([]Answer{{N: q.N, Letter: "A"}}); !strings.HasSuffix(got, ".A") {
				t.Errorf("answer line = %q", got)
			}
		})
	}
}
