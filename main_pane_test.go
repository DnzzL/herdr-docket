package main

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/DnzzL/herdr-docket/internal/history"
	"github.com/DnzzL/herdr-docket/internal/runner"
	"github.com/DnzzL/herdr-docket/internal/work"
)

func paneFixture() paneData {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	long := strings.Repeat("a title far too long for any phone screen ", 3)
	return paneData{
		Daemon: "daemon down for 4 min",
		Now:    now,
		Queue: []work.Task{
			{ID: "TASK-1", Title: "Which store? " + long, Question: true, Phase: string(work.PhaseQuestion)},
			{ID: "TASK-2", Title: "Parked " + long, Open: true, Blocked: true, Phase: "Blocked"},
			{ID: "TASK-3", Title: "Held " + long, Verdict: work.Blocked, Phase: "Blocked"},
			{ID: "TASK-4", Title: "Plain todo " + long, Open: true, Phase: string(work.PhaseTodo)},
		},
		Runs: []history.Record{
			{RunID: "r9", Task: "TASK-9-with-a-long-id", Agent: "a-long-agent-name", Status: history.StatusRunning, At: now.Add(-3 * time.Minute), WorkspaceID: "workspace-with-a-long-id"},
			{RunID: "r3", Task: "TASK-3", Agent: "verifier", Status: history.StatusDone, At: now.Add(-time.Hour), PullRequest: "https://github.com/o/r/pull/12", Verification: "PASS"},
			{RunID: "r8", Task: "TASK-8", Agent: "dev", Status: history.StatusFailed, At: now.Add(-2 * time.Hour)},
		},
	}
}

func TestPaneNeverWrapsOnANarrowTerminal(t *testing.T) {
	lines, _ := renderPane(paneFixture(), 45)
	for _, line := range lines {
		if n := utf8.RuneCountInString(line); n > 45 {
			t.Errorf("%d columns: %q", n, line)
		}
	}
}

func TestPaneOpensOnTheDaemonLine(t *testing.T) {
	lines, _ := renderPane(paneFixture(), 80)
	if len(lines) == 0 || !strings.Contains(lines[0], "daemon down for 4 min") {
		t.Fatalf("first line %q", lines)
	}
}

func TestYourTurnListsQuestionsBlockedAndHeldMerges(t *testing.T) {
	lines, _ := renderPane(paneFixture(), 80)
	out := strings.Join(lines, "\n")
	turn := strings.Index(out, "Your turn")
	if turn < 0 {
		t.Fatalf("no Your turn section:\n%s", out)
	}
	for _, want := range []string{"? TASK-1 Which store?", "! TASK-2 Parked", "⇡ TASK-3 https://github.com/o/r/pull/12"} {
		i := strings.Index(out, want)
		if i < turn {
			t.Errorf("want %q under Your turn:\n%s", want, out)
		}
	}
	if strings.Contains(out, "TASK-4") {
		t.Errorf("a todo card is not the human's turn:\n%s", out)
	}
	if strings.Contains(out, "! TASK-3") {
		t.Errorf("a held merge is listed once, as a merge:\n%s", out)
	}
	inFlight, settled := strings.Index(out, "TASK-9"), strings.Index(out, "TASK-8")
	if inFlight < turn || settled < inFlight {
		t.Errorf("runs in flight then settled runs come after Your turn:\n%s", out)
	}
}

// A GitHub board's List spends rate-limited API calls, and the queue moves on
// a human's clock: the pane re-lists it every 30 s, while the runs — a local
// file — are re-read on every frame.
func TestThePaneListsTheQueueEvery30Seconds(t *testing.T) {
	lists := 0
	p := &paneReader{list: func() ([]work.Task, error) { lists++; return nil, nil }}
	t0 := time.Unix(1_700_000_000, 0)
	for _, at := range []time.Duration{0, 2 * time.Second, 4 * time.Second, 29 * time.Second, 30 * time.Second, 32 * time.Second} {
		p.read(t0.Add(at), false)
	}
	if lists != 2 {
		t.Fatalf("listed the queue %d times over 32 s of frames, want 2 (at 0 s and 30 s)", lists)
	}
}

// paneAt is the Home model on the fixture, sized to width x height the way
// bubbletea sizes it: with a WindowSizeMsg.
func paneAt(t *testing.T, d paneData, width, height int) paneModel {
	t.Helper()
	m, _ := paneModel{data: d, closer: &closer{}}.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return m.(paneModel)
}

// rowOf is the screen row the View draws the first line containing s on.
func rowOf(t *testing.T, m paneModel, s string) int {
	t.Helper()
	for y, line := range strings.Split(m.View(), "\n") {
		if strings.Contains(line, s) {
			return y
		}
	}
	t.Fatalf("%q not on screen:\n%s", s, m.View())
	return -1
}

func click(m paneModel, y int) (paneModel, tea.Cmd) {
	next, cmd := m.Update(tea.MouseMsg{X: 4, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	return next.(paneModel), cmd
}

func key(m paneModel, k string) (paneModel, tea.Cmd) {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
	if k == "enter" {
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	}
	next, cmd := m.Update(msg)
	return next.(paneModel), cmd
}

func TestHomeNeverWrapsAt45Columns(t *testing.T) {
	m := paneAt(t, paneFixture(), 45, 40)
	for _, line := range strings.Split(m.View(), "\n") {
		if n := lipgloss.Width(line); n > 45 {
			t.Errorf("%d columns: %q", n, line)
		}
	}
}

// A tap is a click (ADR 0024): a tap selects a row, a tap on the selected
// row acts. The highlighted first row counts as selected, so one tap on it
// opens its Round.
func TestOneTapOnTheHighlightedQuestionOpensItsRound(t *testing.T) {
	m := paneAt(t, paneFixture(), 45, 40)
	_, cmd := click(m, rowOf(t, m, "? TASK-1"))
	if cmd == nil {
		t.Fatalf("one tap on the highlighted row opened nothing:\n%s", m.View())
	}
	if got, ok := cmd().(openRoundMsg); !ok || got.ID != "TASK-1" {
		t.Fatalf("want openRoundMsg{TASK-1}, got %#v", cmd())
	}
}

func TestATapOnAnotherRowOnlySelectsIt(t *testing.T) {
	m := paneAt(t, paneFixture(), 45, 40)
	m, cmd := click(m, rowOf(t, m, "! TASK-2"))
	if cmd != nil {
		t.Fatalf("the first tap only selects, got %v", cmd())
	}
	if !strings.Contains(m.View(), "> ! TASK-2") {
		t.Fatalf("TASK-2 not selected:\n%s", m.View())
	}
}

func TestEnterOnAQuestionOpensItsRound(t *testing.T) {
	m := paneAt(t, paneFixture(), 80, 40)
	m, _ = click(m, rowOf(t, m, "? TASK-1"))
	_, cmd := key(m, "enter")
	if cmd == nil {
		t.Fatal("Enter opened nothing")
	}
	if got, ok := cmd().(openRoundMsg); !ok || got.ID != "TASK-1" {
		t.Fatalf("want openRoundMsg{TASK-1}, got %#v", cmd())
	}
}

func TestTheWheelScrollsAPaneTallerThanTheScreen(t *testing.T) {
	m := paneAt(t, paneFixture(), 80, 5)
	first := strings.Split(m.View(), "\n")[0]
	next, _ := m.Update(tea.MouseMsg{Y: 2, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown})
	m = next.(paneModel)
	lines := strings.Split(m.View(), "\n")
	if len(lines) > 5 {
		t.Fatalf("%d lines on a 5-row screen", len(lines))
	}
	if lines[0] == first {
		t.Fatalf("wheel down did not scroll: still %q", first)
	}
	// A click lands on what is drawn where it lands, scrolled or not.
	m, _ = click(m, rowOf(t, m, "? TASK-1"))
	if !strings.Contains(m.View(), "> ? TASK-1") {
		t.Fatalf("click after scroll missed TASK-1:\n%s", m.View())
	}
}

// The keys the raw-ANSI pane had keep working: j/k and 1-9 select a run,
// x stops it, p pauses and resumes the fleet, q quits.
func TestTheOldKeysStillWork(t *testing.T) {
	now := time.Now()
	d := paneData{Now: now, Runs: []history.Record{
		{RunID: "r1", Task: "TASK-1", Agent: "dev", Status: history.StatusRunning, At: now, WorkspaceID: "ws1"},
		{RunID: "r2", Task: "TASK-2", Agent: "dev", Status: history.StatusRunning, At: now, WorkspaceID: "ws2"},
	}}
	c := &closer{}
	next, _ := paneModel{data: d, closer: c}.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	m := next.(paneModel)

	m, _ = key(m, "j")
	m = drain(key(m, "x"))
	if len(c.closed) != 1 || c.closed[0].WorkspaceID != "ws2" {
		t.Fatalf("j then x closed %+v, want ws2", c.closed)
	}
	m, _ = key(m, "k")
	m = drain(key(m, "x"))
	if len(c.closed) != 2 || c.closed[1].WorkspaceID != "ws1" {
		t.Fatalf("k then x closed %+v, want ws1", c.closed)
	}
	m, _ = key(m, "2")
	if !strings.Contains(m.View(), "> 2 TASK-2") {
		t.Fatalf("2 did not select the second run:\n%s", m.View())
	}

	m, _ = key(m, "p")
	if !runner.Paused() {
		t.Fatal("p did not pause the fleet")
	}
	m, _ = key(m, "p")
	if runner.Paused() {
		t.Fatal("p again did not resume the fleet")
	}

	_, cmd := key(m, "q")
	if cmd == nil || cmd() != tea.Quit() {
		t.Fatal("q did not quit")
	}
}

// Before the terminal says its size there is nothing to fit: a frame drawn
// at a guessed width wraps on a phone and leaves ghost lines.
func TestNoFrameBeforeTheTerminalSize(t *testing.T) {
	m := paneModel{data: paneFixture(), closer: &closer{}}
	m = send(m, paneTick(paneFixture().Now))
	if v := m.View(); v != "" {
		t.Fatalf("drew before the size:\n%s", v)
	}
	if v := send(m, tea.WindowSizeMsg{Width: 45, Height: 20}).View(); v == "" {
		t.Fatal("nothing drawn once sized")
	}
}
