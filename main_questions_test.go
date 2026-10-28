package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/DnzzL/herdr-docket/internal/work"
)

const threeQuestionRound = "## Round 2\n" +
	"1. What happens to the audio when it is sent?\n   A. attached as a file (reco)\n   B. text in a comment\n" +
	"2. Who reads it first?\n   A. the triager (reco)\n   B. the owner\n" +
	"3. How long is it kept?\n   A. forever (reco)\n   B. 30 days\n"

func questionSource(title string) *phasingSource {
	card := work.Task{ID: "TASK-2", Title: title, Question: true, Phase: string(work.PhaseQuestion), Notes: threeQuestionRound}
	return &phasingSource{fakeSource: fakeSource{
		items: []work.Task{card, {ID: "TASK-3", Title: "busy", Open: true, Phase: string(work.PhaseInProgress)}},
		item:  card,
	}}
}

func runQuestions(t *testing.T, src work.Source, input string) string {
	t.Helper()
	var out bytes.Buffer
	if err := answerQuestions(src, "daemon live · tick 3s ago", strings.NewReader(input), &out); err != nil {
		t.Fatalf("answerQuestions: %v\n%s", err, out.String())
	}
	return out.String()
}

func TestQuestionsAnswersARoundWithEnterLettersAndFreeText(t *testing.T) {
	src := questionSource("Voice notes")
	got := runQuestions(t, src, "\nB\nkeep it\n\n")
	if want := []string{"## Answer 2\n1.A 2.B 3. keep it"}; !reflect.DeepEqual(src.comments, want) {
		t.Fatalf("commented %q, want %q\n%s", src.comments, want, got)
	}
	if want := []work.Phase{work.PhaseTriage}; !reflect.DeepEqual(src.phases, want) {
		t.Fatalf("phases %v, want %v", src.phases, want)
	}
	if !strings.HasPrefix(got, "daemon live") {
		t.Fatalf("first line must be the daemon line, got %q", got)
	}
	if !strings.Contains(got, "1.A 2.B 3. keep it") || !strings.Contains(got, "send? [Y/n]") {
		t.Fatalf("must show the assembled line and ask to send, got %q", got)
	}
}

func TestQuestionsFitsA45ColumnTerminal(t *testing.T) {
	src := questionSource(strings.Repeat("a very long card title that goes on ", 4))
	got := runQuestions(t, src, "\nb\nkeep it for as long as the law asks and not a day more\nn\n")
	for _, l := range strings.Split(got, "\n") {
		if utf8.RuneCountInString(l) > 45 {
			t.Errorf("line over 45 columns (%d): %q", utf8.RuneCountInString(l), l)
		}
	}
	if len(src.comments) != 0 {
		t.Fatalf("n must not send, commented %q", src.comments)
	}
}

func TestQuestionsWithNoQuestionCardsSaysSo(t *testing.T) {
	src := &phasingSource{fakeSource: fakeSource{items: []work.Task{{ID: "TASK-3", Open: true, Phase: string(work.PhaseTodo)}}}}
	got := runQuestions(t, src, "")
	if !strings.Contains(got, "no card") {
		t.Fatalf("must say nothing waits, got %q", got)
	}
}
