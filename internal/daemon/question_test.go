package daemon

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/DnzzL/herdr-docket/internal/fleet"
	"github.com/DnzzL/herdr-docket/internal/host"
	"github.com/DnzzL/herdr-docket/internal/hostpath"
	"github.com/DnzzL/herdr-docket/internal/runner"
	"github.com/DnzzL/herdr-docket/internal/work"
)

// notice is one popup a tick raised, as the human would hear it.
type notice struct {
	title, body, sound string
}

// notifyHost is the daemon's host with the popup kept — the Question pass's
// only visible effect. fails refuses that many popups first, the way a herdr
// that is not listening does.
type notifyHost struct {
	fakeHost
	mu      sync.Mutex
	notices []notice
	fails   int
}

func (h *notifyHost) Notify(title, body, sound string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.fails > 0 {
		h.fails--
		return errors.New("herdr: notification failed")
	}
	h.notices = append(h.notices, notice{title: title, body: body, sound: sound})
	return nil
}

func (h *notifyHost) seen() []notice {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]notice(nil), h.notices...)
}

// daemonOver is one daemon process — a runner and its reported map — whose
// queue is src and whose fleet is one agent. Calling it twice in a test is a
// restart: the state dir carries over, the memory does not.
func daemonOver(t *testing.T, h host.Host, src work.Source) (*runner.Runner, map[string]bool) {
	t.Helper()
	withFleet(t, map[string]fleet.Agent{
		"a": {Name: "a", Workdir: "/w", Workspace: "root", TimeoutMinutes: 1},
	}, src, src, src, src)
	return runner.New(h, fleet.Settings{Dir: "/fleet"}), map[string]bool{}
}

func questionCard(notes string) work.Task {
	return work.Task{
		ID: "docket/TASK-93", Title: "A card in Question raises a Herdr popup",
		Notes: notes, Open: true, Question: true, Assignee: "a",
	}
}

// setQuestion parks a card in Question with notes, or lifts it back to To Do.
func (s *memSource) setQuestion(id string, question bool, notes string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := s.items[id]
	t.Question, t.Notes = question, notes
	s.items[id] = t
}

func questionMarkers(t *testing.T) []string {
	t.Helper()
	marks, err := filepath.Glob(filepath.Join(hostpath.StateDir(), questionMarkerPrefix+"*"))
	if err != nil {
		t.Fatal(err)
	}
	return marks
}

const (
	round2 = "## Round 2\nAudio is sent as text today.\n1. Which one?\n   A. this (reco)\n"
	round3 = "## Round 3\nA second thought.\n1. And which one?\n   A. that (reco)\n"
)

func TestAQuestionCardAnnouncesItsRoundOnce(t *testing.T) {
	ownStateDir(t)
	src := newMemSource(questionCard(round2))
	h := &notifyHost{}
	runs, reported := daemonOver(t, h, src)

	evaluate(runs, h, reported)
	evaluate(runs, h, reported)

	seen := h.seen()
	if len(seen) != 1 {
		t.Fatalf("popups = %v, want one per round, not one per tick", seen)
	}
	if seen[0].title != "answer owed — docket/TASK-93" {
		t.Errorf("title = %q, want the card id a human can act on", seen[0].title)
	}
	if seen[0].body != "round 2 — A card in Question raises a Herdr popup" {
		t.Errorf("body = %q, want the round and the card's title", seen[0].body)
	}
	if seen[0].sound != host.SoundRequest {
		t.Errorf("sound = %q, want %q: this is a debt, like a merge a human owes", seen[0].sound, host.SoundRequest)
	}
}

func TestASecondRoundPopsAgain(t *testing.T) {
	ownStateDir(t)
	src := newMemSource(questionCard(round2))
	h := &notifyHost{}
	runs, reported := daemonOver(t, h, src)

	evaluate(runs, h, reported)
	src.setQuestion("docket/TASK-93", true, round2+round3)
	evaluate(runs, h, reported)

	seen := h.seen()
	if len(seen) != 2 {
		t.Fatalf("popups = %v, want one per round", seen)
	}
	if seen[1].body != "round 3 — A card in Question raises a Herdr popup" {
		t.Errorf("second body = %q, want the new round", seen[1].body)
	}
}

func TestARestartDoesNotRepeatAnAnnouncement(t *testing.T) {
	ownStateDir(t)
	src := newMemSource(questionCard(round2))
	first := &notifyHost{}
	runs, reported := daemonOver(t, first, src)
	evaluate(runs, first, reported)

	// The plugin re-execs on upgrade: a new daemon, a new host, the same state
	// dir. The round was already announced, so it stays quiet.
	second := &notifyHost{}
	runs, reported = daemonOver(t, second, src)
	evaluate(runs, second, reported)

	if got := len(second.seen()); got != 0 {
		t.Fatalf("popups after a restart = %v, want none: the marker remembers", second.seen())
	}
}

func TestAQuestionCardWithoutARoundIsSilent(t *testing.T) {
	ownStateDir(t)
	src := newMemSource(questionCard("no round here, just a note"))
	h := &notifyHost{}
	runs, reported := daemonOver(t, h, src)

	evaluate(runs, h, reported)

	if got := len(h.seen()); got != 0 {
		t.Fatalf("popups = %v, want none: a blank round is not a question", h.seen())
	}
	if marks := questionMarkers(t); len(marks) != 0 {
		t.Fatalf("markers = %v, want none for a card with nothing to answer", marks)
	}
}

func TestAFailedNotifyRetriesOnTheNextTick(t *testing.T) {
	ownStateDir(t)
	src := newMemSource(questionCard(round2))
	h := &notifyHost{fails: 1}
	runs, reported := daemonOver(t, h, src)

	evaluate(runs, h, reported)
	if got := len(h.seen()); got != 0 {
		t.Fatalf("popups = %v, want the refusal to stay unrecorded", h.seen())
	}
	if marks := questionMarkers(t); len(marks) != 0 {
		t.Fatalf("markers = %v, want none until the popup returns clean", marks)
	}

	evaluate(runs, h, reported)
	if got := len(h.seen()); got != 1 {
		t.Fatalf("popups = %v, want the retry to land", h.seen())
	}
}

func TestALeavingQuestionForgetsAndAPopOnReturn(t *testing.T) {
	ownStateDir(t)
	src := newMemSource(questionCard(round2))
	h := &notifyHost{}
	runs, reported := daemonOver(t, h, src)

	evaluate(runs, h, reported)
	// Answered: the card leaves Question, so the debt is paid and its marker
	// goes with it.
	src.setQuestion("docket/TASK-93", false, round2)
	evaluate(runs, h, reported)
	if marks := questionMarkers(t); len(marks) != 0 {
		t.Fatalf("markers after leaving Question = %v, want none", marks)
	}

	// Back to the same round — a new debt, a new popup.
	src.setQuestion("docket/TASK-93", true, round2)
	evaluate(runs, h, reported)

	if got := len(h.seen()); got != 2 {
		t.Fatalf("popups = %v, want the returned round to speak again", h.seen())
	}
	if marks := questionMarkers(t); len(marks) != 1 {
		t.Fatalf("markers = %v, want exactly the current round's", marks)
	}
}

func TestAPausedFleetStillAnnouncesAQuestion(t *testing.T) {
	ownStateDir(t)
	if err := runner.SetPaused(true); err != nil {
		t.Fatal(err)
	}
	src := newMemSource(questionCard(round2))
	h := &notifyHost{}
	runs, reported := daemonOver(t, h, src)

	evaluate(runs, h, reported)
	evaluate(runs, h, reported)

	if got := len(h.seen()); got != 1 {
		t.Fatalf("popups while paused = %v, want the waiting to be heard once", h.seen())
	}
	if runs.Running("docket/TASK-93") {
		t.Fatal("a paused fleet must still start no run")
	}
}
