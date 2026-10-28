package main

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/DnzzL/herdr-docket/internal/work"
)

const contextRound = "## Round 2\n" +
	"Audio is sent as text today; upload.go builds the note, and the linked card TASK-7 waits on this.\n" +
	"1. What happens to the audio when it is sent?\n   A. attached as a file (reco)\n   B. text in a comment\n" +
	"2. Who reads it first?\n   A. the triager (reco)\n   B. the owner\n" +
	"3. How long is it kept?\n   A. forever (reco)\n   B. thirty days\n"

// roundPane is the pane on a question card, sized, with its Round open the
// way Home opens it: an openRoundMsg.
func roundPane(t *testing.T, width, height int) (paneModel, *phasingSource) {
	t.Helper()
	return roundPaneOn(t, contextRound, width, height)
}

func roundPaneOn(t *testing.T, notes string, width, height int) (paneModel, *phasingSource) {
	t.Helper()
	card := work.Task{ID: "TASK-2", Title: "Voice notes", Question: true, Phase: string(work.PhaseQuestion), Notes: notes}
	src := &phasingSource{fakeSource: fakeSource{items: []work.Task{card}, item: card}}
	m := paneModel{data: paneData{Queue: []work.Task{card}}, closer: &closer{},
		source: func() (work.Source, error) { return src, nil }}
	m = send(m, tea.WindowSizeMsg{Width: width, Height: height})
	m = send(m, openRoundMsg{ID: "TASK-2"})
	return m, src
}

func send(m paneModel, msg tea.Msg) paneModel {
	next, _ := m.Update(msg)
	return next.(paneModel)
}

// at is where s starts on screen: the row and the cell column.
func at(t *testing.T, m paneModel, s string) (x, y int) {
	t.Helper()
	for y, line := range strings.Split(m.View(), "\n") {
		if i := strings.Index(line, s); i >= 0 {
			return lipgloss.Width(line[:i]), y
		}
	}
	t.Fatalf("%q not on screen:\n%s", s, m.View())
	return 0, 0
}

// tap is a left click on the first cell of s, then the commands it returns
// fed back the way bubbletea would.
func tap(t *testing.T, m paneModel, s string) paneModel {
	t.Helper()
	x, y := at(t, m, s)
	next, cmd := m.Update(tea.MouseMsg{X: x + 1, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	return drain(next.(paneModel), cmd)
}

func drain(m paneModel, cmd tea.Cmd) paneModel {
	for cmd != nil {
		msg := cmd()
		if msg == nil || msg == tea.Quit() {
			return m
		}
		var next tea.Model
		next, cmd = m.Update(msg)
		m = next.(paneModel)
	}
	return m
}

func TestTappingBOnQuestion2ThenSendPostsTheRound(t *testing.T) {
	m, src := roundPane(t, 45, 60)
	m = tap(t, m, "B the owner")
	m = tap(t, m, "[ Send ]")
	if want := []string{"## Answer 2\n1.A 2.B 3.A"}; !reflect.DeepEqual(src.comments, want) {
		t.Fatalf("commented %q, want %q\n%s", src.comments, want, m.View())
	}
	if !strings.Contains(m.View(), "TASK-2: answered round 2") || strings.Contains(m.View(), "[ Send ]") {
		t.Fatalf("want Home with the confirmation:\n%s", m.View())
	}
}

func TestAt45ColumnsNoLineOverflowsAndButtonsStackOnePerLine(t *testing.T) {
	m, _ := roundPane(t, 45, 60)
	for _, line := range strings.Split(m.View(), "\n") {
		if n := lipgloss.Width(line); n > 45 {
			t.Errorf("%d columns: %q", n, line)
		}
	}
	for _, line := range strings.Split(m.View(), "\n") {
		if strings.Count(line, "[") > 1 {
			t.Errorf("two buttons on one line at 45 columns: %q", line)
		}
	}
	if _, y := at(t, m, "Audio is sent as text today"); y == 0 {
		t.Errorf("the context comes after the header:\n%s", m.View())
	}
}

func TestAt120ColumnsAQuestionsButtonsSitInline(t *testing.T) {
	m, _ := roundPane(t, 120, 40)
	_, ya := at(t, m, "A the triager")
	_, yb := at(t, m, "B the owner")
	_, yp := at(t, m, "or in plain words…")
	if ya != yb {
		t.Fatalf("A on row %d, B on row %d:\n%s", ya, yb, m.View())
	}
	if _, y := at(t, m, "A attached as a file"); y != yp {
		t.Fatalf("question 1's plain-words button not inline:\n%s", m.View())
	}
}

func TestThePlainWordsButtonTakesAFreeAnswer(t *testing.T) {
	m, src := roundPane(t, 45, 60)
	// question 3's plain-words button is the last one on screen
	lines := strings.Split(m.View(), "\n")
	y := -1
	for i, l := range lines {
		if strings.Contains(l, "or in plain words…") {
			y = i
		}
	}
	next, _ := m.Update(tea.MouseMsg{X: 4, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	m = next.(paneModel)
	if !strings.Contains(m.View(), "3.–") {
		t.Fatalf("an empty text field is no answer yet:\n%s", m.View())
	}
	for _, r := range "keep it a year" {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		if r == ' ' {
			msg = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
		}
		m = send(m, msg)
	}
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = tap(t, m, "[ Send ]")
	if want := []string{"## Answer 2\n1.A 2.A 3. keep it a year"}; !reflect.DeepEqual(src.comments, want) {
		t.Fatalf("commented %q, want %q\n%s", src.comments, want, m.View())
	}
}

// A live run still reads the card: Send says so and stays on the Round.
func TestSendOnACardALiveRunHoldsSaysSoAndStays(t *testing.T) {
	held := runHeld
	t.Cleanup(func() { runHeld = held })
	runHeld = func(id string) bool { return id == "TASK-2" }
	m, src := roundPane(t, 45, 60)
	m = tap(t, m, "[ Send ]")
	if len(src.comments) != 0 {
		t.Fatalf("sent %q under a live run", src.comments)
	}
	if !strings.Contains(m.View(), "a run holds the card") || !strings.Contains(m.View(), "[ Send ]") {
		t.Fatalf("want the lock message on the Round:\n%s", m.View())
	}
}

func TestOneTapOnTheHighlightedQuestionRowOpensItsRound(t *testing.T) {
	m, _ := roundPane(t, 45, 60)
	m = send(m, tea.KeyMsg{Type: tea.KeyEsc})
	m = drain(m, func() tea.Msg { return roundDoneMsg{} })
	if strings.Contains(m.View(), "round 2") {
		t.Fatalf("Esc stayed on the Round:\n%s", m.View())
	}
	m = tap(t, m, "? TASK-2")
	if !strings.Contains(m.View(), "TASK-2 Voice notes  round 2") {
		t.Fatalf("one tap did not open the Round:\n%s", m.View())
	}
}

func TestKeysAloneAnswerARound(t *testing.T) {
	m, src := roundPane(t, 80, 40)
	for _, k := range []tea.KeyMsg{
		{Type: tea.KeyDown}, {Type: tea.KeyRunes, Runes: []rune("B")},
		{Type: tea.KeyRunes, Runes: []rune("j")}, {Type: tea.KeyRunes, Runes: []rune("l")}, {Type: tea.KeyEnter},
		{Type: tea.KeyRunes, Runes: []rune("j")}, {Type: tea.KeyEnter},
	} {
		next, cmd := m.Update(k)
		m = drain(next.(paneModel), cmd)
	}
	if want := []string{"## Answer 2\n1.A 2.B 3.B"}; !reflect.DeepEqual(src.comments, want) {
		t.Fatalf("commented %q, want %q\n%s", src.comments, want, m.View())
	}
}

// opens is the card the pane's first frame opens a Round on, "" for Home.
func opens(m paneModel) string {
	msg := m.Init()()
	batch, _ := msg.(tea.BatchMsg)
	for _, cmd := range batch {
		if o, ok := cmd().(openRoundMsg); ok {
			return o.ID
		}
	}
	return ""
}

// `questions` on a TTY: one card waiting goes straight to its Round, several
// start on Home with their `?` rows.
func TestQuestionsOnATerminalOpensTheRoundOfTheOneCard(t *testing.T) {
	src := questionSource("Voice notes")
	m, ok, err := questionsPane(src)
	if err != nil || !ok || opens(m) != "TASK-2" {
		t.Fatalf("ok %v err %v opens %q, want TASK-2", ok, err, opens(m))
	}
	src.items = append(src.items, work.Task{ID: "TASK-5", Title: "other", Question: true, Phase: string(work.PhaseQuestion)})
	m, ok, err = questionsPane(src)
	if err != nil || !ok || opens(m) != "" {
		t.Fatalf("ok %v err %v opens %q, want Home", ok, err, opens(m))
	}
	src.items = nil
	if _, ok, _ = questionsPane(src); ok {
		t.Fatal("no card waiting: the line prompt says so, no pane")
	}
}

const wideRound = "## Round 1\n" +
	"1. 音声はどこに保存しますか？ The audio goes where once the note is sent to the card?\n" +
	"   A. 添付ファイルとしてカードに保存する 📎 (reco)\n   B. 東京のサーバー\n" +
	"2. Keep it?\n   A. yes (reco)\n   B. no\n"

// A CJK or emoji option is two cells a character: it stays inside the
// screen, and its tap box is where it is drawn.
func TestAWideOptionFitsAndItsTapLands(t *testing.T) {
	for _, width := range []int{45, 120} {
		m, src := roundPaneOn(t, wideRound, width, 40)
		for _, line := range strings.Split(m.View(), "\n") {
			if n := lipgloss.Width(line); n > width {
				t.Errorf("at %d: %d cells: %q", width, n, line)
			}
		}
		m = tap(t, m, "B 東京")
		m = tap(t, m, "[ Send ]")
		if want := []string{"## Answer 1\n1.B 2.A"}; !reflect.DeepEqual(src.comments, want) {
			t.Fatalf("at %d: commented %q, want %q\n%s", width, src.comments, want, m.View())
		}
	}
}

// A wrapped question reads as one block: its continuation lines sit under
// its text, not under its number.
func TestAWrappedQuestionContinuesUnderItsText(t *testing.T) {
	m, _ := roundPane(t, 30, 60)
	lines := strings.Split(m.View(), "\n")
	_, y := at(t, m, "1. What happens")
	if next := lines[y+1]; !strings.HasPrefix(next, "   ") || strings.TrimSpace(next) == "" || strings.Contains(next, "[") {
		t.Fatalf("continuation %q not indented under the text:\n%s", next, m.View())
	}
}

// French puts a space before : ? ! ; — a wrap must not leave one alone at
// the start of a line.
func TestFrenchPunctuationNeverStartsALine(t *testing.T) {
	q := "Où garde-t-on le son une fois envoyé"
	q += strings.Repeat("x", 42-len([]rune(q))) // "1. " + q fills 45 cells
	notes := "## Round 1\n1. " + q + " ?\n   A. oui (reco)\n2. Et ensuite : on le garde ; ou pas !\n   A. oui (reco)\n"
	for _, width := range []int{45, 30, 21} {
		m, _ := roundPaneOn(t, notes, width, 60)
		for _, line := range strings.Split(m.View(), "\n") {
			if f := strings.Fields(line); len(f) > 0 && strings.ContainsAny(f[0], ":?!;") && len([]rune(f[0])) == 1 {
				t.Errorf("at %d a line starts with %q:\n%s", width, f[0], m.View())
			}
		}
	}
}
