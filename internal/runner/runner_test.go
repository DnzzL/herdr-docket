package runner

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DnzzL/herdr-fleet/internal/backlog"
	"github.com/DnzzL/herdr-fleet/internal/fleet"
	"github.com/DnzzL/herdr-fleet/internal/history"
	"github.com/DnzzL/herdr-fleet/internal/host"
)

type fakeHost struct {
	provisionErr error
	doErr        error
	closes       int
	spec         host.Spec
	// statusAfterDo lets the fake board change the task status mid-run, the
	// way a real agent reports back through the CLI.
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

type fakeBoard struct {
	status  map[string]string
	notes   []string
	viewErr error
}

func newBoard(status string) *fakeBoard {
	return &fakeBoard{status: map[string]string{"TASK-1": status}}
}
func (b *fakeBoard) View(id string) (backlog.View, error) {
	return backlog.View{Task: backlog.Task{ID: id, Title: "T", Status: b.status[id]}, Description: "d"}, b.viewErr
}
func (b *fakeBoard) SetStatus(id, status string) error { b.status[id] = status; return nil }
func (b *fakeBoard) AppendNote(id, note string) error  { b.notes = append(b.notes, note); return nil }

func run(t *testing.T, h *fakeHost, b *fakeBoard) error {
	t.Helper()
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	r := New(h, b, "/fleet")
	return r.Run(backlog.Task{ID: "TASK-1", Title: "T"}, fleet.Agent{Name: "a", Workdir: "/w", Workspace: "root", Kind: "claude", TimeoutMinutes: 1, Persona: "P"}, "manual")
}

func TestHappyPathAgentReportsDone(t *testing.T) {
	b := newBoard(backlog.StatusToDo)
	h := &fakeHost{after: func() { b.status["TASK-1"] = backlog.StatusDone }}
	if err := run(t, h, b); err != nil {
		t.Fatal(err)
	}
	if b.status["TASK-1"] != backlog.StatusDone {
		t.Fatalf("status = %s", b.status["TASK-1"])
	}
	if h.spec.Repo != "/w" || h.spec.Workspace != host.WorkspaceRoot || !strings.Contains(h.spec.Prompt, "TASK-1") {
		t.Fatalf("spec = %+v", h.spec)
	}
	if h.closes != 1 {
		t.Fatalf("a Done run must close its workspace, closes = %d", h.closes)
	}
}

func TestAgentSettledWithoutReportingIsAFailure(t *testing.T) {
	b := newBoard(backlog.StatusToDo)
	if err := run(t, &fakeHost{}, b); err == nil {
		t.Fatal("want error")
	}
	if b.status["TASK-1"] != backlog.StatusFailed {
		t.Fatalf("status = %s", b.status["TASK-1"])
	}
	if len(b.notes) == 0 || !strings.Contains(b.notes[0], "without reporting") {
		t.Fatalf("notes = %v", b.notes)
	}
	// The history must not call this run "done" when the board says Failed.
	runs, err := history.Runs("TASK-1", 1)
	if err != nil || len(runs) != 1 || runs[0].Status != history.StatusFailed {
		t.Fatalf("history = %+v, %v", runs, err)
	}
}

func TestCancelledRunGoesBlockedNotFailed(t *testing.T) {
	b := newBoard(backlog.StatusToDo)
	if err := run(t, &fakeHost{doErr: host.ErrCancelled}, b); err == nil {
		t.Fatal("want error")
	}
	if b.status["TASK-1"] != backlog.StatusBlocked {
		t.Fatalf("status = %s", b.status["TASK-1"])
	}
}

func TestNonDoneRunsKeepTheirWorkspaceAndNoteIt(t *testing.T) {
	b := newBoard(backlog.StatusToDo)
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
	b := newBoard(backlog.StatusToDo)
	if err := run(t, &fakeHost{doErr: errors.New("agent never started")}, b); err == nil {
		t.Fatal("want error")
	}
	if b.status["TASK-1"] != backlog.StatusFailed {
		t.Fatalf("status = %s", b.status["TASK-1"])
	}
	if !strings.Contains(strings.Join(b.notes, " "), "agent never started") {
		t.Fatalf("notes = %v", b.notes)
	}
}

func TestAgentsOwnVerdictIsRespectedEvenAfterAHostError(t *testing.T) {
	// Timeout expired but the agent had already set Blocked: keep its verdict.
	b := newBoard(backlog.StatusToDo)
	h := &fakeHost{doErr: errors.New("still working after 1m"),
		after: func() { b.status["TASK-1"] = backlog.StatusBlocked }}
	_ = run(t, h, b)
	if b.status["TASK-1"] != backlog.StatusBlocked {
		t.Fatalf("status = %s", b.status["TASK-1"])
	}
}

func TestRunsSerializePerCheckoutNotGlobally(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	b := &fakeBoard{status: map[string]string{"TASK-1": backlog.StatusToDo, "TASK-2": backlog.StatusToDo, "TASK-3": backlog.StatusToDo}}
	started, release := make(chan struct{}), make(chan struct{})
	h := &fakeHost{after: func() { close(started); <-release }}
	r := New(h, b, "/fleet")
	rootA := fleet.Agent{Name: "pm", Workdir: "/repo", Workspace: "root", TimeoutMinutes: 1}
	rootB := fleet.Agent{Name: "docs", Workdir: "/repo", Workspace: "root", TimeoutMinutes: 1}
	tree := fleet.Agent{Name: "dev", Workdir: "/repo", Workspace: "worktree", TimeoutMinutes: 1}

	first := make(chan struct{})
	go func() { defer close(first); r.Run(backlog.Task{ID: "TASK-1"}, rootA, "poll") }()
	<-started
	if !r.Busy() || r.CanRun(rootA) {
		t.Fatal("rootA's slot must be taken")
	}
	// Same agent again, and a different root agent on the same checkout: refused.
	if err := r.Run(backlog.Task{ID: "TASK-2"}, rootA, "poll"); err == nil {
		t.Fatal("same agent must be refused")
	}
	if err := r.Run(backlog.Task{ID: "TASK-2"}, rootB, "poll"); err == nil || r.CanRun(rootB) {
		t.Fatal("a root-mode agent sharing the checkout must be refused")
	}
	// A worktree agent on the same repo gets its own checkout: allowed.
	if !r.CanRun(tree) {
		t.Fatal("a worktree agent must be free to run")
	}
	h2 := &fakeHost{after: func() { b.status["TASK-3"] = backlog.StatusDone }}
	r.host = h2
	if err := r.Run(backlog.Task{ID: "TASK-3"}, tree, "poll"); err != nil {
		t.Fatalf("worktree run alongside a root run: %v", err)
	}
	close(release)
	// Wait for the run to finish inside the test: leaked past it, the goroutine
	// writes history with the test env torn down — into the real state dir.
	<-first
}
