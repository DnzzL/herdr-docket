package runner

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DnzzL/herdr-fleet/internal/fleet"
	"github.com/DnzzL/herdr-fleet/internal/history"
	"github.com/DnzzL/herdr-fleet/internal/host"
	"github.com/DnzzL/herdr-fleet/internal/work"
)

type fakeHost struct {
	provisionErr error
	doErr        error
	closes       int
	spec         host.Spec
	// after lets the fake board change the task mid-run, the way a real agent
	// reports back through the fleet CLI.
	after func()
}

func (f *fakeHost) Provision(a host.Spec) (host.Session, error) {
	f.spec = a
	return host.Session{WorkspaceID: "ws", PaneID: "p"}, f.provisionErr
}
func (f *fakeHost) Close(host.Session) error { f.closes++; return nil }
func (f *fakeHost) Do(s host.Session, a host.Spec, timeout time.Duration) error {
	f.spec = a
	if f.after != nil {
		f.after()
	}
	return f.doErr
}

// fakeBoard is a queue in memory: enough for the runner to read a task back,
// show it as running, and close it.
type fakeBoard struct {
	items     map[string]work.Task
	notes     []string
	getErr    error
	phaseErr  error
	phaseSeen []work.Phase
}

func newBoard(id string) *fakeBoard {
	return &fakeBoard{items: map[string]work.Task{
		id: {ID: id, Title: "T", Phase: "To Do", Open: true},
	}}
}

func (b *fakeBoard) List() ([]work.Task, error) { return nil, nil }

func (b *fakeBoard) Get(id string) (work.Task, error) {
	if b.getErr != nil {
		return work.Task{}, b.getErr
	}
	it, ok := b.items[id]
	if !ok {
		return work.Task{}, fmt.Errorf("no such task %q", id)
	}
	return it, nil
}

func (b *fakeBoard) Create(title, body, assignee string) (string, error) { return "new", nil }

func (b *fakeBoard) Comment(id, text string) error {
	b.notes = append(b.notes, text)
	return nil
}

func (b *fakeBoard) Close(id string, v work.Verdict) error {
	it := b.items[id]
	it.Open, it.Verdict = false, v
	b.items[id] = it
	return nil
}

func (b *fakeBoard) SetPhase(id string, p work.Phase) error {
	b.phaseSeen = append(b.phaseSeen, p)
	if b.phaseErr != nil {
		return b.phaseErr
	}
	it := b.items[id]
	it.Phase = string(p)
	b.items[id] = it
	return nil
}

func (b *fakeBoard) Assign(id, agent string) error {
	it := b.items[id]
	it.Assignee = agent
	b.items[id] = it
	return nil
}

func (b *fakeBoard) verdict(id string) work.Verdict { return b.items[id].Verdict }
func (b *fakeBoard) open(id string) bool            { return b.items[id].Open }

func run(t *testing.T, h *fakeHost, b *fakeBoard) error {
	t.Helper()
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	r := New(h, "/fleet")
	return r.Run(b, work.Task{ID: "TASK-1", Title: "T", Open: true}, fleet.Agent{Name: "a", Workdir: "/w", Workspace: "root", Kind: "claude", TimeoutMinutes: 1, Persona: "P"}, "manual")
}

func TestHappyPathAgentReportsDone(t *testing.T) {
	b := newBoard("TASK-1")
	h := &fakeHost{after: func() { b.Close("TASK-1", work.Done) }}
	if err := run(t, h, b); err != nil {
		t.Fatal(err)
	}
	if b.verdict("TASK-1") != work.Done || b.open("TASK-1") {
		t.Fatalf("verdict = %q, open = %v", b.verdict("TASK-1"), b.open("TASK-1"))
	}
	if h.spec.Repo != "/w" || h.spec.Workspace != host.WorkspaceRoot || !strings.Contains(h.spec.Prompt, "TASK-1") {
		t.Fatalf("spec = %+v", h.spec)
	}
	if h.closes != 1 {
		t.Fatalf("a Done run must close its workspace, closes = %d", h.closes)
	}
}

func TestAgentSettledWithoutReportingIsAFailure(t *testing.T) {
	b := newBoard("TASK-1")
	if err := run(t, &fakeHost{}, b); err == nil {
		t.Fatal("want error")
	}
	if b.verdict("TASK-1") != work.Failed || b.open("TASK-1") {
		t.Fatalf("verdict = %q, open = %v", b.verdict("TASK-1"), b.open("TASK-1"))
	}
	if len(b.notes) == 0 || !strings.Contains(b.notes[0], "without reporting") {
		t.Fatalf("notes = %v", b.notes)
	}
	// The history must not call this run "done" when the queue says Failed.
	runs, err := history.Runs("TASK-1", 1)
	if err != nil || len(runs) != 1 || runs[0].Status != history.StatusFailed {
		t.Fatalf("history = %+v, %v", runs, err)
	}
}

func TestCancelledRunGoesBlockedNotFailed(t *testing.T) {
	b := newBoard("TASK-1")
	if err := run(t, &fakeHost{doErr: host.ErrCancelled}, b); err == nil {
		t.Fatal("want error")
	}
	if b.verdict("TASK-1") != work.Blocked {
		t.Fatalf("verdict = %q", b.verdict("TASK-1"))
	}
}

func TestNonDoneRunsKeepTheirWorkspaceAndNoteIt(t *testing.T) {
	b := newBoard("TASK-1")
	h := &fakeHost{doErr: errors.New("boom")}
	_ = run(t, h, b)
	if h.closes != 0 {
		t.Fatal("a Failed run must keep its workspace open")
	}
	if !strings.Contains(strings.Join(b.notes, " "), "workspace ws (pane p) is left open") {
		t.Fatalf("notes = %v", b.notes)
	}
}

func TestHostFailureMarksTheTaskFailedWithTheReason(t *testing.T) {
	b := newBoard("TASK-1")
	if err := run(t, &fakeHost{doErr: errors.New("agent never started")}, b); err == nil {
		t.Fatal("want error")
	}
	if b.verdict("TASK-1") != work.Failed {
		t.Fatalf("verdict = %q", b.verdict("TASK-1"))
	}
	if !strings.Contains(strings.Join(b.notes, " "), "agent never started") {
		t.Fatalf("notes = %v", b.notes)
	}
}

func TestAgentsOwnVerdictIsRespectedEvenAfterAHostError(t *testing.T) {
	// Timeout expired but the agent had already closed it Blocked: keep that.
	b := newBoard("TASK-1")
	h := &fakeHost{doErr: errors.New("still working after 1m"),
		after: func() { b.Close("TASK-1", work.Blocked) }}
	_ = run(t, h, b)
	if b.verdict("TASK-1") != work.Blocked {
		t.Fatalf("verdict = %q", b.verdict("TASK-1"))
	}
}

// The claim is display only. A backend that cannot show a phase — a Basecamp
// to-do — must run exactly the same, and one whose phase write fails must not
// lose the run over it.
func TestAPhaseWriteIsBestEffort(t *testing.T) {
	b := newBoard("TASK-1")
	b.phaseErr = errors.New("this backend has no phases")
	h := &fakeHost{after: func() { b.Close("TASK-1", work.Done) }}
	if err := run(t, h, b); err != nil {
		t.Fatalf("a failed phase write must not fail the run: %v", err)
	}
	if b.verdict("TASK-1") != work.Done {
		t.Fatalf("verdict = %q", b.verdict("TASK-1"))
	}
	if len(b.phaseSeen) != 1 || b.phaseSeen[0] != work.PhaseInProgress {
		t.Fatalf("the run should have marked itself running once, saw %v", b.phaseSeen)
	}
}

func TestRunsSerializePerCheckoutNotGlobally(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	b := newBoard("TASK-1")
	b.items["TASK-2"] = work.Task{ID: "TASK-2", Open: true, Phase: "To Do"}
	b.items["TASK-3"] = work.Task{ID: "TASK-3", Open: true, Phase: "To Do"}
	started, release := make(chan struct{}), make(chan struct{})
	h := &fakeHost{after: func() { close(started); <-release }}
	r := New(h, "/fleet")
	rootA := fleet.Agent{Name: "pm", Workdir: "/repo", Workspace: "root", TimeoutMinutes: 1}
	rootB := fleet.Agent{Name: "docs", Workdir: "/repo", Workspace: "root", TimeoutMinutes: 1}
	tree := fleet.Agent{Name: "dev", Workdir: "/repo", Workspace: "worktree", TimeoutMinutes: 1}

	first := make(chan struct{})
	go func() { defer close(first); r.Run(b, work.Task{ID: "TASK-1"}, rootA, "poll") }()
	<-started
	if !r.Busy() || r.CanRun(rootA) {
		t.Fatal("rootA's slot must be taken")
	}
	// Same agent again, and a different root agent on the same checkout: refused.
	if err := r.Run(b, work.Task{ID: "TASK-2"}, rootA, "poll"); err == nil {
		t.Fatal("same agent must be refused")
	}
	if err := r.Run(b, work.Task{ID: "TASK-2"}, rootB, "poll"); err == nil || r.CanRun(rootB) {
		t.Fatal("a root-mode agent sharing the checkout must be refused")
	}
	// A worktree agent on the same repo gets its own checkout: allowed.
	if !r.CanRun(tree) {
		t.Fatal("a worktree agent must be free to run")
	}
	h2 := &fakeHost{after: func() { b.Close("TASK-3", work.Done) }}
	r.host = h2
	if err := r.Run(b, work.Task{ID: "TASK-3"}, tree, "poll"); err != nil {
		t.Fatalf("worktree run alongside a root run: %v", err)
	}
	close(release)
	// Wait for the run to finish inside the test: leaked past it, the goroutine
	// writes history with the test env torn down — into the real state dir.
	<-first
}

// A PM specs a task and hands it to the dev. The task is open on purpose, with
// a new owner — closing it as "settled without reporting" would undo the
// handoff, and the next tick would never route it to anyone.
func TestATaskHandedToAnotherAgentStaysOpen(t *testing.T) {
	b := newBoard("TASK-1")
	h := &fakeHost{after: func() { _ = b.Assign("TASK-1", "dev") }}
	if err := run(t, h, b); err != nil {
		t.Fatal(err)
	}
	if !b.open("TASK-1") {
		t.Fatal("a handed-on task must stay open for its new agent")
	}
	if v := b.verdict("TASK-1"); v != "" {
		t.Fatalf("verdict = %q, want none: the task did not end", v)
	}
	if h.closes != 1 {
		t.Fatalf("a handed-on run tears its workspace down, closes = %d", h.closes)
	}
}

// Reassignment is not an escape hatch from reporting. An agent that handed the
// task on and then crashed left it unreported all the same.
func TestAHandoffDoesNotExcuseAFailedRun(t *testing.T) {
	b := newBoard("TASK-1")
	h := &fakeHost{doErr: errors.New("boom"), after: func() { _ = b.Assign("TASK-1", "dev") }}
	if err := run(t, h, b); err == nil {
		t.Fatal("want error")
	}
	if b.open("TASK-1") || b.verdict("TASK-1") != work.Failed {
		t.Fatalf("verdict = %q, open = %v", b.verdict("TASK-1"), b.open("TASK-1"))
	}
}
