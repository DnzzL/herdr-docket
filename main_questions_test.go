package main

import (
	"bytes"
	"io"
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

// A card a live run holds is not the owner's turn yet: questions hides it.
func TestQuestionsHidesACardALiveRunHolds(t *testing.T) {
	held := runHeld
	t.Cleanup(func() { runHeld = held })
	runHeld = func(id string) bool { return id == "TASK-2" }
	got := runQuestions(t, questionSource("Voice notes"), "")
	if strings.Contains(got, "TASK-2") || !strings.Contains(got, "no card") {
		t.Fatalf("a held card must not be offered, got %q", got)
	}
}

// A Question card with no round to answer is a triager's mistake: said,
// naming the card, with nothing sent.
func TestQuestionsOnACardWithNoRoundSaysSo(t *testing.T) {
	src := questionSource("Voice notes")
	src.items[0].Notes, src.item.Notes = "", ""
	err := answerQuestions(src, "", strings.NewReader(""), io.Discard)
	if err == nil || !strings.Contains(err.Error(), "TASK-2: no ## Round note") || len(src.comments) != 0 {
		t.Fatalf("err = %v, comments %v", err, src.comments)
	}
}

// Input that ends mid-round sends nothing: a half-answered round is not an
// answer.
func TestQuestionsInputEndingMidRoundSendsNothing(t *testing.T) {
	src := questionSource("Voice notes")
	err := answerQuestions(src, "", strings.NewReader("\n"), io.Discard)
	if err == nil || !strings.Contains(err.Error(), "input ended, nothing sent") || len(src.comments) != 0 {
		t.Fatalf("err = %v, comments %v", err, src.comments)
	}
}

// byID is a source whose Get answers each card its own notes.
type byID struct {
	*phasingSource
	cards map[string]work.Task
}

func (b byID) Get(id string) (work.Task, error) { return b.cards[id], nil }

// With several cards the owner picks one by number; a number off the list is
// asked again, not taken as the first card.
func TestQuestionsPicksTheCardByNumber(t *testing.T) {
	one := work.Task{ID: "TASK-1", Title: "one", Question: true, Notes: "## Round 1\n1. first?\n   A. yes (reco)"}
	two := work.Task{ID: "TASK-2", Title: "two", Question: true, Notes: "## Round 3\n1. second?\n   A. yes (reco)"}
	src := byID{&phasingSource{fakeSource: fakeSource{items: []work.Task{one, two}}}, map[string]work.Task{"TASK-1": one, "TASK-2": two}}
	var out bytes.Buffer
	if err := answerQuestions(src, "", strings.NewReader("9\n2\n\n\n"), &out); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	if want := []string{"## Answer 3\n1.A"}; !reflect.DeepEqual(src.comments, want) {
		t.Fatalf("commented %q, want %q — card 2's round 3", src.comments, want)
	}
}
