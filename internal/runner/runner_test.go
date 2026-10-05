package runner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/DnzzL/herdr-docket/internal/fleet"
	"github.com/DnzzL/herdr-docket/internal/gate"
	"github.com/DnzzL/herdr-docket/internal/history"
	"github.com/DnzzL/herdr-docket/internal/host"
	"github.com/DnzzL/herdr-docket/internal/hostpath"
	"github.com/DnzzL/herdr-docket/internal/work"
)

type fakeHost struct {
	provisionErr error
	doErr        error
	closes       int
	spec         host.Spec
	// specs is every spec a run was provisioned with, in order — one per
	// stage of a pipeline, so a test can tell the author's spec from the
	// verifier's.
	specs []host.Spec
	// do, when set, replaces Do's body: a test scripts what happens at the
	// deadline — the agent still working, the task still open — instead of
	// declaring one answer for every call of the run.
	do func(a host.Spec) error
	// settle replaces what the agent does while the run keeps listening past
	// its deadline; without it the fake settles at once, the agent finishing
	// its work inside the extra window.
	settle  func() error
	settles int
	// window is the extra listening time the runner asked for, asserted by the
	// tests that pin how long a late agent gets.
	window time.Duration
	// session is what Provision answered, kept so a test can assert the run's
	// record names the branch the provision actually created.
	session    host.Session
	delivery   host.Delivery
	inspectErr error
	inspects   int
	// verifyInspects and verifyCloses count the calls a verify session (a run
	// pinned to a PR head, TASK-65) received: the author's own run inspects
	// and closes too, and the policy under test is per-session.
	verifyInspects int
	verifyCloses   int
	// after lets the fake board change the task mid-run, the way a real agent
	// reports back through the fleet CLI.
	after func()
	// notifies records what the runner asked the host to raise. See notify.
	notifies []notify
}

// notify is one desktop notification the fleet asked for.
type notify struct{ title, body, sound string }

func (f *fakeHost) Notify(title, body, sound string) error {
	f.notifies = append(f.notifies, notify{title, body, sound})
	return nil
}

func (f *fakeHost) Provision(a host.Spec) (host.Session, error) {
	f.spec = a
	f.specs = append(f.specs, a)
	s := host.Session{WorkspaceID: "ws", PaneID: "p", Repo: a.Repo, BaseCommit: "c0074fe"}
	// Mirrors production: a spec pinned to a commit is cut into a worktree on
	// a branch of its own whatever mode it names (the host derives it), and
	// the session carries the verify mark Close tears it down by (TASK-65).
	// The branch is derived the way production derives it — slug of the spec's
	// name plus the cut timestamp — not constructed as an arbitrary string: a
	// fake that hand-builds a value the real code derives is the boundary the
	// suite agreed to pretend about (TASK-26, TASK-38).
	if a.Workspace == host.WorkspaceWorktree || a.Head != "" {
		s.Branch = fmt.Sprintf("fleet/%s-%s", host.Slug(a.Name), time.Now().Format("20060102-1504"))
	}
	if a.Head != "" {
		s.Verify = true
	}
	f.session = s
	return s, f.provisionErr
}
func (f *fakeHost) Inspect(s host.Session) (host.Delivery, error) {
	f.inspects++
	if s.Verify {
		f.verifyInspects++
	}
	return f.delivery, f.inspectErr
}
func (f *fakeHost) Close(s host.Session) error {
	f.closes++
	if s.Verify {
		f.verifyCloses++
	}
	return nil
}
func (f *fakeHost) Do(s host.Session, a host.Spec, timeout time.Duration) error {
	f.spec = a
	if f.do != nil {
		return f.do(a)
	}
	if f.after != nil {
		f.after()
	}
	return f.doErr
}

func (f *fakeHost) Settle(_ host.Session, window time.Duration) error {
	f.settles++
	f.window = window
	if f.settle != nil {
		return f.settle()
	}
	return nil
}

// fakeBoard is a queue in memory: enough for the runner to read a task back,
// show it as running, and close it.
type fakeBoard struct {
	items     map[string]work.Task
	notes     []string
	getErr    error
	phaseErr  error
	phaseSeen []work.Phase
	// base/baseErr stand in for the queue's default-branch answer (TASK-45).
	base    string
	baseErr error
}

func newBoard(id string) *fakeBoard {
	return &fakeBoard{items: map[string]work.Task{
		id: {ID: id, Title: "T", Phase: "To Do", Open: true},
	}}
}

func (b *fakeBoard) List() ([]work.Task, error) { return nil, nil }

// BaseBranch stands in for a queue that names the ref its worktree runs
// branch from; empty means no answer, which is what a hosted queue says.
func (b *fakeBoard) BaseBranch(string) (string, error) {
	if b.baseErr != nil {
		return "", b.baseErr
	}
	return b.base, nil
}

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
	return runWorkspace(t, h, b, "root")
}

func runWorktree(t *testing.T, h *fakeHost, b *fakeBoard) error {
	t.Helper()
	return runWorkspace(t, h, b, "worktree")
}

func runWorkspace(t *testing.T, h *fakeHost, b *fakeBoard, ws string) error {
	t.Helper()
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	r := New(h, fleet.Settings{Dir: "/fleet"})
	return r.Run(b, work.Task{ID: "TASK-1", Title: "T", Open: true}, fleet.Agent{Name: "a", Workdir: "/w", Workspace: ws, Kind: "claude", TimeoutMinutes: 1, Persona: "P"}, "manual")
}

// A task mid-run under one routing must not be startable under another,
// whatever `fleet.yaml` says in between. Seen live: a task unassigned and
// mid-run under default_agent dev was re-picked by pm one tick after a human
// flipped default_agent to pm — two workspaces, two agents, one task. The
// per-task flock is what refuses the second run, so it holds across
// processes too: the second Runner here stands in for `herdr-docket run` in
// another process, whose own busy map knows nothing of the first.
func TestATaskWithARunInFlightCannotBeStartedByAnotherRouting(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	b := newBoard("TASK-1")
	started, release := make(chan struct{}), make(chan struct{})
	h := &fakeHost{after: func() { close(started); <-release }}
	r := New(h, fleet.Settings{Dir: "/fleet"})
	dev := fleet.Agent{Name: "dev", Workdir: "/w", Workspace: "worktree", TimeoutMinutes: 1}
	pm := fleet.Agent{Name: "pm", Workdir: "/w", Workspace: "worktree", TimeoutMinutes: 1}

	first := make(chan struct{})
	go func() { defer close(first); r.Run(b, work.Task{ID: "TASK-1"}, dev, "poll") }()
	<-started

	// After the flip, the same open task routes to pm — held by a different
	// LockKey, so the agent/checkout lock says nothing about it. The per-task
	// lock must refuse it, and a refused run must touch nothing: no claim, no
	// comment, no history record.
	clamp := New(&fakeHost{}, fleet.Settings{Dir: "/fleet"})
	claimed, noted := len(b.phaseSeen), len(b.notes) // the first run claimed once
	err := clamp.Run(b, work.Task{ID: "TASK-1"}, pm, "manual")
	if err == nil {
		t.Fatal("a second routing of a task with a run in flight must be refused")
	}
	if len(b.phaseSeen) != claimed || len(b.notes) != noted {
		t.Errorf("a refused run must not claim or write: phases %v, notes %v", b.phaseSeen, b.notes)
	}

	// flock dies with the run: once the first run is over, the task is free
	// again — the self-heal path holds a crashed run's task open for whoever
	// routes it next.
	close(release)
	<-first
	finish := &fakeHost{after: func() { b.Close("TASK-1", work.Done) }}
	clamp.host = finish
	if err := clamp.Run(b, work.Task{ID: "TASK-1", Open: true}, pm, "manual"); err != nil {
		t.Fatalf("a task whose run is over must be startable again: %v", err)
	}
}

// flock, not a flag: the claim is a lock on a file another process opens, so
// a second open file description on the same lock file — what another
// process, or another Runner, holds — cannot take it while the run is live.
// In-process here, but the contention is exactly what `herdr-docket run` in
// a real second process hits, on the same state dir.
func TestTheTaskClaimIsAnFlockASecondHolderCannotTake(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	b := newBoard("TASK-1")
	started, release := make(chan struct{}), make(chan struct{})
	startedDir := make(chan string, 1)
	h := &fakeHost{after: func() {
		dir := hostpath.StateDir()
		startedDir <- dir
		close(started)
		<-release
	}}
	r := New(h, fleet.Settings{Dir: "/fleet"})
	first := make(chan struct{})
	go func() {
		defer close(first)
		r.Run(b, work.Task{ID: "TASK-1"}, fleet.Agent{Name: "dev", Workdir: "/w", Workspace: "worktree", TimeoutMinutes: 1}, "poll")
	}()
	<-started

	// A second holder — another process, another Runner — opens the same
	// lock file and must be refused by the flock.
	state := <-startedDir
	other, err := os.OpenFile(filepath.Join(state, "run-taskid-task-1.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatalf("open the lock file: %v", err)
	}
	defer other.Close()
	if err := syscall.Flock(int(other.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
		t.Fatal("a second holder must not take the task lock while a run is live")
	}

	close(release)
	<-first
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

func TestAFailedRunReachesAHumanNotWatching(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	b := newBoard("TASK-1")
	h := &fakeHost{doErr: errors.New("agent never started")}
	forge := &fakeForge{}
	r := New(h, fleet.Settings{Dir: "/fleet"})
	r.forge = forge
	_ = r.Run(b, work.Task{ID: "TASK-1", Title: "T", Open: true},
		fleet.Agent{Name: "a", Workdir: "/w", Workspace: "root", Kind: "claude", TimeoutMinutes: 1, Persona: "P"}, "manual")
	if len(h.notifies) != 1 {
		t.Fatalf("a failed run must ask for exactly one notification, saw %v", h.notifies)
	}
	n := h.notifies[0]
	if !strings.Contains(n.title, "TASK-1") || !strings.Contains(n.body, "agent never started") {
		t.Fatalf("notification = %q / %q, want the task and the reason", n.title, n.body)
	}
	// TASK-63 #3: with no pull request out there is no merge to owe — the
	// popup stays the run's own, silent, and the PR marking never happens.
	if n.sound != host.SoundNone {
		t.Errorf("sound = %q, want %q — the run failed, nothing is being asked for", n.sound, host.SoundNone)
	}
	if len(forge.labels) != 0 {
		t.Errorf("labels = %v, want none: a failed run with no PR marks nothing", forge.labels)
	}
}

// TASK-63 #4 (third case): a run that failed with a pull request still out
// leaves work only a human can move — GitHub will never ping them, because
// the fleet opens PRs under their own account. The stop announces the merge
// it owes, by PR rather than by task, and marks the PR so it can be found.
func TestAFailedRunWithAPRAnnouncesTheMergeTheHumanOwes(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	b := newBoard("TASK-1")
	h := &fakeHost{
		doErr: errors.New("agent never started"),
		after: func() {
			if err := history.SetPullRequest("TASK-1", prURL); err != nil {
				t.Error(err)
			}
		},
	}
	forge := &fakeForge{}
	r := New(h, fleet.Settings{Dir: "/fleet"})
	r.forge = forge
	if err := r.Run(b, work.Task{ID: "TASK-1", Title: "T", Open: true},
		fleet.Agent{Name: "a", Workdir: "/w", Workspace: "root", Kind: "claude", TimeoutMinutes: 1, Persona: "P"}, "manual"); err == nil {
		t.Fatal("want the failed run's error")
	}
	if len(h.notifies) != 1 {
		t.Fatalf("one stop, one popup, saw %v", h.notifies)
	}
	n := h.notifies[0]
	if n.title != "merge needed — o/r#7" || n.sound != host.SoundRequest {
		t.Errorf("popup = %+v, want 'merge needed — o/r#7' in the request sound", n)
	}
	if !strings.Contains(n.body, "agent never started") {
		t.Errorf("body = %q, want the reason the run stopped", n.body)
	}
	if len(forge.labels) != 1 || forge.labels[0] != "merge-needed" {
		t.Errorf("labels = %v, want the PR marked merge-needed", forge.labels)
	}
}

func TestADoneRunSendsNoNotification(t *testing.T) {
	// Done is the fleet working as designed: a notification for it would be
	// the one that makes the human mute the rest.
	b := newBoard("TASK-1")
	h := &fakeHost{after: func() { b.Close("TASK-1", work.Done) }}
	if err := run(t, h, b); err != nil {
		t.Fatal(err)
	}
	if len(h.notifies) != 0 {
		t.Fatalf("a done run must not notify, saw %v", h.notifies)
	}
}

func TestACancelledRunSendsNoNotification(t *testing.T) {
	// Cancelled is a human deciding, not a run breaking: the person who
	// cancelled is the one watching.
	b := newBoard("TASK-1")
	h := &fakeHost{doErr: host.ErrCancelled}
	_ = run(t, h, b)
	if len(h.notifies) != 0 {
		t.Fatalf("a cancelled run must not notify, saw %v", h.notifies)
	}
}

func TestAProvisioningFailureNotifiesToo(t *testing.T) {
	b := newBoard("TASK-1")
	h := &fakeHost{provisionErr: errors.New("repo gone")}
	_ = run(t, h, b)
	if len(h.notifies) != 1 || !strings.Contains(h.notifies[0].body, "repo gone") {
		t.Fatalf("notification = %v, want the provisioning failure", h.notifies)
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
	r := New(h, fleet.Settings{Dir: "/fleet"})
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

// The closing record states what the run produced — the branch the session was
// provisioned on, the commit count git reports, and the pull request the agent
// stamped through the CLI — so the delivery never lives only in prose on a
// task. A worktree run with no commits shows +0 rather than staying silent.
func TestClosingRecordStatesWhatTheRunProduced(t *testing.T) {
	b := newBoard("TASK-1")
	h := &fakeHost{delivery: host.Delivery{Commits: 2}}
	h.after = func() {
		// The record written the moment the run is running already names
		// the branch Provision created — the fact travels from provision
		// time, it is not glued back on when the run closes.
		if r, err := history.LastRun("TASK-1"); err != nil {
			t.Errorf("read the running record: %v", err)
		} else if r.Status != history.StatusRunning || r.Branch != h.session.Branch {
			t.Errorf("running record = %+v, want status running naming branch %q", r, h.session.Branch)
		}
		b.Close("TASK-1", work.Done)
		if err := history.SetPullRequest("TASK-1", "https://example.com/pr/9"); err != nil {
			t.Errorf("set PR: %v", err)
		}
	}
	if err := runWorktree(t, h, b); err != nil {
		t.Fatal(err)
	}
	r, err := history.LastRun("TASK-1")
	if err != nil || r == nil {
		t.Fatal(err)
	}
	if r.Branch == "" || r.Branch != h.session.Branch {
		t.Errorf("branch = %q, want the branch Provision named (%q)", r.Branch, h.session.Branch)
	}
	// The fake derived it from the spec, as production does: the name of the
	// task is in it, not an invented constant.
	if want := "fleet/" + host.Slug("TASK-1 T") + "-"; !strings.HasPrefix(h.session.Branch, want) {
		t.Errorf("branch = %q, want it derived from the spec name as %q+tag", h.session.Branch, want)
	}
	if r.Commits != 2 {
		t.Errorf("commits = %d, want the count Inspect reported", r.Commits)
	}
	if r.PullRequest != "https://example.com/pr/9" {
		t.Errorf("pull request = %q, want the url the agent stamped carried forward", r.PullRequest)
	}
	if h.inspects != 1 {
		t.Errorf("inspects = %d, want exactly one — a worktree run is read before it is torn down", h.inspects)
	}
}

// A root-mode run claims no branch: nothing is inspected, and the record
// implies no delivery it cannot vouch for.
func TestARootRunClaimsNoBranchAndIsNeverInspected(t *testing.T) {
	b := newBoard("TASK-1")
	h := &fakeHost{}
	h.after = func() {
		// No branch is claimed at any point of the run — the record written
		// while it is running, and the closing one, both stay silent about a
		// branch a root-mode workspace never had.
		if r, err := history.LastRun("TASK-1"); err != nil {
			t.Errorf("read the running record: %v", err)
		} else if r.Status != history.StatusRunning || r.Branch != "" || r.Commits != 0 || r.Uncommitted {
			t.Errorf("running record = %+v, want a running record claiming no delivery", r)
		}
		b.Close("TASK-1", work.Done)
	}
	if err := run(t, h, b); err != nil {
		t.Fatal(err)
	}
	r, err := history.LastRun("TASK-1")
	if err != nil || r == nil {
		t.Fatal(err)
	}
	if r.Branch != "" || r.Commits != 0 || r.Uncommitted {
		t.Errorf("root record = %+v, want no delivery fields at all", r)
	}
	if h.inspects != 0 {
		t.Errorf("inspects = %d, want none — there is no branch to inspect", h.inspects)
	}
}

// TASK-23's guard: a disposable worktree about to be destroyed holding
// changes nobody committed cannot be read as a success. The fleet keeps the
// workspace, says so on the task, records the fact, and puts the task in the
// board's human-decides column — because the report is doubted, not the
// mechanics.
func TestADoneRunHoldingUncommittedWorkIsKeptAndSaid(t *testing.T) {
	b := newBoard("TASK-1")
	h := &fakeHost{
		delivery: host.Delivery{Dirty: true},
		after:    func() { b.Close("TASK-1", work.Done) },
	}
	if err := runWorktree(t, h, b); err != nil {
		t.Fatal(err)
	}
	if h.closes != 0 {
		t.Errorf("workspace closed with uncommitted work in it, closes = %d", h.closes)
	}
	if got := b.verdict("TASK-1"); got != work.Blocked {
		t.Errorf("verdict = %q, want blocked — a human decides", got)
	}
	if !hasNoteContaining(b, "uncommitted") {
		t.Errorf("the task must say what was found, notes = %v", b.notes)
	}
	r, err := history.LastRun("TASK-1")
	if err != nil || r == nil {
		t.Fatal(err)
	}
	if !r.Uncommitted {
		t.Errorf("run record = %+v, want the run history to say the worktree was uncommitted", r)
	}
}

// The legitimate empty run: a worktree-mode run that cleaned up after itself
// and produced no commits is a real outcome — teardown proceeds as it always
// did, and the record still names the branch it did not move.
func TestACleanWorktreeRunWithNoCommitsTearsDownAsUsual(t *testing.T) {
	b := newBoard("TASK-1")
	h := &fakeHost{after: func() { b.Close("TASK-1", work.Done) }}
	if err := runWorktree(t, h, b); err != nil {
		t.Fatal(err)
	}
	if h.closes != 1 {
		t.Errorf("closes = %d, want the normal teardown", h.closes)
	}
	if len(b.notes) != 0 {
		t.Errorf("a clean empty run owes no explanation, notes = %v", b.notes)
	}
	r, err := history.LastRun("TASK-1")
	if err != nil || r == nil {
		t.Fatal(err)
	}
	if r.Branch != h.session.Branch || r.Commits != 0 || r.Uncommitted {
		t.Errorf("record = %+v, want the branch named (%q), +0 and no uncommitted flag", r, h.session.Branch)
	}
	if b.verdict("TASK-1") != work.Done {
		t.Errorf("verdict = %q, want done — nothing was at risk", b.verdict("TASK-1"))
	}
}

// A root-mode run has no disposable worktree: there is nothing for the guard
// to protect and nothing for it to ask about. Its edits are the repo's own
// state, and the persona decides what happens to them.
func TestARootRunIsNeverGuardedBecauseItHasNoWorktreeToLose(t *testing.T) {
	b := newBoard("TASK-1")
	h := &fakeHost{
		delivery: host.Delivery{Dirty: true},
		after:    func() { b.Close("TASK-1", work.Done) },
	}
	if err := run(t, h, b); err != nil {
		t.Fatal(err)
	}
	if h.inspects != 0 {
		t.Errorf("inspects = %d, want none", h.inspects)
	}
	if h.closes != 1 || b.verdict("TASK-1") != work.Done || len(b.notes) != 0 {
		t.Errorf("closes = %d, verdict = %q, notes = %v: a root run must be untouched",
			h.closes, b.verdict("TASK-1"), b.notes)
	}
}

// A delivery that cannot be read is kept, not guessed: the workspace stays
// open with a note, and the agent's verdict stands, because the fleet has no
// evidence of loss — only of its own blindness.
func TestAWorktreeWhoseDeliveryCannotBeReadIsKeptAndSaid(t *testing.T) {
	b := newBoard("TASK-1")
	h := &fakeHost{
		inspectErr: errors.New("no worktree holds the branch Provision named"),
		after:      func() { b.Close("TASK-1", work.Done) },
	}
	if err := runWorktree(t, h, b); err != nil {
		t.Fatal(err)
	}
	if h.closes != 0 {
		t.Errorf("closes = %d, want the workspace kept when delivery is unverifiable", h.closes)
	}
	if got := b.verdict("TASK-1"); got != work.Done {
		t.Errorf("verdict = %q, want done — the fleet has no evidence to doubt it with", got)
	}
	if !hasNoteContaining(b, "could not be read") {
		t.Errorf("the task must say the delivery is unverifiable, notes = %v", b.notes)
	}
}

// A handoff with uncommitted work in the worktree: the task stays open on its
// new owner, but the workspace it would have inherited gone is kept and said.
func TestHandedOffWorkWithUncommittedChangesKeepsItsWorkspace(t *testing.T) {
	b := newBoard("TASK-1")
	h := &fakeHost{
		delivery: host.Delivery{Dirty: true},
		after:    func() { b.Assign("TASK-1", "reviewer") },
	}
	if err := runWorktree(t, h, b); err != nil {
		t.Fatal(err)
	}
	if h.closes != 0 {
		t.Errorf("closes = %d, want the worktree kept for whoever picks the task up", h.closes)
	}
	if !b.open("TASK-1") {
		t.Error("a handed-on task must stay open")
	}
	if !hasNoteContaining(b, "uncommitted") {
		t.Errorf("notes = %v, want the handoff to say what the workspace still holds", b.notes)
	}
}

func hasNoteContaining(b *fakeBoard, want string) bool {
	for _, n := range b.notes {
		if strings.Contains(n, want) {
			return true
		}
	}
	return false
}

// An agent that reports `fail` did its job: it ran, it judged, it said so.
// The run is recorded as failed because the task is, but the reason must not
// accuse it of silence — that sentence is reserved for an agent that closed
// nothing, and it is the whole way a person tells a broken agent from a
// correctly refused ticket. Observed on a live fleet where every blocked
// review — the reviewer doing exactly what its persona asks — was filed as
// "the agent settled without reporting a verdict".
func TestAReportedFailureIsNotCalledSilence(t *testing.T) {
	b := newBoard("TASK-1")
	h := &fakeHost{after: func() { b.Close("TASK-1", work.Failed) }}
	err := run(t, h, b)
	if err == nil {
		t.Fatal("a task that ended Failed is still a failed run")
	}
	if strings.Contains(err.Error(), "without reporting") {
		t.Errorf("the agent reported; the run must not call it silence: %v", err)
	}
	runs, rerr := history.Runs("TASK-1", 1)
	if rerr != nil || len(runs) != 1 {
		t.Fatalf("history = %+v, %v", runs, rerr)
	}
	if runs[0].Status != history.StatusFailed {
		t.Errorf("history status = %q, want failed", runs[0].Status)
	}
	if strings.Contains(runs[0].Error, "without reporting") {
		t.Errorf("the record repeats the accusation: %q", runs[0].Error)
	}
}

// The same for a verdict of blocked, which is what a reviewer writes when it
// refuses to merge and hands the decision to a human. A fleet whose config
// points failed and blocked at one column reads it back as Failed, so this is
// the common case, not the rare one.
func TestAReportedBlockIsNotCalledSilence(t *testing.T) {
	b := newBoard("TASK-1")
	h := &fakeHost{after: func() { b.Close("TASK-1", work.Blocked) }}
	err := run(t, h, b)
	if err != nil && strings.Contains(err.Error(), "without reporting") {
		t.Errorf("a blocked task was reported by its agent: %v", err)
	}
	runs, _ := history.Runs("TASK-1", 1)
	if len(runs) == 1 && strings.Contains(runs[0].Error, "without reporting") {
		t.Errorf("the record repeats the accusation: %q", runs[0].Error)
	}
}

// TASK-45: the queue's default-branch answer reaches the host spec, and the
// record carries the commit the run actually branched from — so a PR built on
// a human's mid-branch work is visible in the history, not only on the forge.
func TestAWorktreeRunBranchesFromTheQueueDefaultAndRecordsIt(t *testing.T) {
	b := newBoard("TASK-1")
	b.base = "origin/main"
	h := &fakeHost{after: func() { b.Close("TASK-1", work.Done) }}
	if err := runWorktree(t, h, b); err != nil {
		t.Fatal(err)
	}
	if h.spec.Base != "origin/main" {
		t.Fatalf("spec base = %q, want the queue's origin/main", h.spec.Base)
	}
	runs, err := history.Runs("TASK-1", 1)
	if err != nil || len(runs) != 1 {
		t.Fatalf("history = %+v, %v", runs, err)
	}
	if runs[0].BaseCommit != "c0074fe" {
		t.Fatalf("record BaseCommit = %q, want c0074fe", runs[0].BaseCommit)
	}
}

// A queue with no answer inherits as before — but the record still names the
// commit the run cut from, so the inheritance is a fact on file either way.
func TestAQueueWithoutAnAnswerInheritsButStillRecordsTheCutCommit(t *testing.T) {
	b := newBoard("TASK-1")
	h := &fakeHost{after: func() { b.Close("TASK-1", work.Done) }}
	if err := runWorktree(t, h, b); err != nil {
		t.Fatal(err)
	}
	if h.spec.Base != "" {
		t.Fatalf("spec base = %q, want none", h.spec.Base)
	}
	runs, err := history.Runs("TASK-1", 1)
	if err != nil || len(runs) != 1 {
		t.Fatalf("history = %+v, %v", runs, err)
	}
	if runs[0].BaseCommit != "c0074fe" {
		t.Fatalf("record BaseCommit = %q, want c0074fe", runs[0].BaseCommit)
	}
}

// TASK-62 (#1 #4 #5): the deadline marks a run, it does not drop it. The
// worker's clock runs out with the agent still working — the fake host
// settles it after the deadline, through the runner's seam — and the PR it
// stamps on the way out goes to the verifier exactly as an on-time one.
// The log carries the timeout: a `timed_out` record at the deadline, and
// timed_out on the closing record the reader collapses to, so history.jsonl
// tells the late worker from the on-time verifier beside it.
func TestARunThatPassesItsDeadlineAndSettlesLateStillReachesTheVerifier(t *testing.T) {
	p := newPipeline(t, "auto", gate.Pass)
	calls := 0
	p.host.do = func(host.Spec) error {
		calls++
		if calls == 1 {
			// The worker's clock runs out with the agent still working: the
			// task must still be open — nothing may have closed it yet.
			if !p.board.open("TASK-1") {
				t.Error("the deadline must not close the task; the run is still listening")
			}
			return fmt.Errorf("waiting for the agent: %w", host.ErrTimedOut)
		}
		p.host.after() // the verifier runs on time
		return nil
	}
	p.host.settle = func() error {
		p.host.after() // the worker finishes after the deadline: its PR is stamped here
		return nil
	}
	if err := p.run(); err != nil {
		t.Fatalf("a late delivery must run its pipeline like an on-time one: %v", err)
	}
	if len(p.forge.merged) != 1 || p.forge.merged[0] != prURL+"@h1" {
		t.Fatalf("merged = %v, want the late PR verified and merged", p.forge.merged)
	}
	if p.verifierRuns != 1 {
		t.Fatalf("verifier runs = %d, want 1", p.verifierRuns)
	}
	if p.host.settles != 1 || p.host.window != 2*time.Minute {
		t.Fatalf("settles = %d, window = %s: want one listen of twice the run's one-minute timeout",
			p.host.settles, p.host.window)
	}
	runs, err := history.Runs("TASK-1", 0)
	if err != nil || len(runs) != 2 {
		t.Fatalf("history = %+v, %v: want the worker's run and the verifier's", runs, err)
	}
	// Newest first: the on-time verifier, then the late worker.
	onTime, late := runs[0], runs[1]
	if late.Trigger != history.TriggerManual || late.Status != history.StatusDone || !late.TimedOut {
		t.Errorf("late worker run = %+v, want done carrying timed_out", late)
	}
	if onTime.Trigger != history.TriggerVerify || onTime.TimedOut {
		t.Errorf("verifier run = %+v, want the on-time run unmarked", onTime)
	}
	// The deadline itself is an event in the file, not only a flag: the
	// closing record alone would not say when the run went late.
	raw, rerr := os.ReadFile(filepath.Join(hostpath.StateDir(), "history.jsonl"))
	if rerr != nil {
		t.Fatal(rerr)
	}
	if !strings.Contains(string(raw), `"status":"timed_out"`) {
		t.Errorf("history.jsonl has no timed_out record:\n%s", raw)
	}
}

// TASK-62 (#2): the pipeline's own stage gets the same second chance — a
// verify run past its deadline that records PASS afterwards still reaches
// the gate, exactly like an on-time verdict.
func TestAVerifyRunThatPassesItsDeadlineAndVerifiesLateStillReachesTheGate(t *testing.T) {
	p := newPipeline(t, "auto", gate.Pass)
	calls := 0
	p.host.do = func(host.Spec) error {
		calls++
		if calls == 1 {
			p.host.after() // the worker delivers on time
			return nil
		}
		return fmt.Errorf("waiting for the agent: %w", host.ErrTimedOut) // the verifier is late
	}
	p.host.settle = func() error {
		// The verifier records its verdict after the deadline.
		if err := history.SetVerification("TASK-1", gate.Pass, "p1"); err != nil {
			t.Errorf("set verification: %v", err)
		}
		return nil
	}
	if err := p.run(); err != nil {
		t.Fatalf("a late verdict must reach the gate: %v", err)
	}
	if len(p.forge.merged) != 1 || p.forge.merged[0] != prURL+"@h1" {
		t.Fatalf("merged = %v, want the gate to merge on the late PASS", p.forge.merged)
	}
	runs, err := history.Runs("TASK-1", 0)
	if err != nil || len(runs) != 2 {
		t.Fatalf("history = %+v, %v: want the worker's run and the verifier's", runs, err)
	}
	verify, worker := runs[0], runs[1]
	if verify.Trigger != history.TriggerVerify || verify.Status != history.StatusDone || !verify.TimedOut {
		t.Errorf("verify run = %+v, want done carrying timed_out", verify)
	}
	if worker.TimedOut {
		t.Errorf("worker run = %+v, want the on-time delivery unmarked", worker)
	}
}

// TASK-62 (#3): an agent that never settles is the run's true failure. The
// task ends Failed, a human is told, and the workspace — the agent's whole
// existence — is closed, so nothing keeps working on a task the fleet has
// closed. The fleet listens once and gives up: no endless waiting, no second
// window. The timeout rides on the closing record either way (#4).
func TestARunWhoseAgentNeverSettlesEndsFailedNotifiesAndEndsTheAgent(t *testing.T) {
	b := newBoard("TASK-1")
	h := &fakeHost{
		do: func(host.Spec) error {
			return fmt.Errorf("waiting for the agent: %w", host.ErrTimedOut)
		},
		settle: func() error {
			return fmt.Errorf("the agent was still working after 2m0s: %w", host.ErrTimedOut)
		},
	}
	err := run(t, h, b)
	if err == nil || !strings.Contains(err.Error(), "never settled") {
		t.Fatalf("err = %v, want the run to end failed saying the agent never settled", err)
	}
	if b.open("TASK-1") || b.verdict("TASK-1") != work.Failed {
		t.Fatalf("task = %+v, want closed failed", b.items["TASK-1"])
	}
	if h.closes != 1 {
		t.Fatalf("closes = %d, want the workspace closed — the agent must not outlive the task", h.closes)
	}
	if h.settles != 1 {
		t.Fatalf("settles = %d, want exactly one late window: listen once, then give up", h.settles)
	}
	if len(h.notifies) != 1 || !strings.Contains(h.notifies[0].body, "never settled") {
		t.Fatalf("notifies = %v, want one notification saying the agent never settled", h.notifies)
	}
	if !hasNoteContaining(b, "closed so nothing keeps working") {
		t.Errorf("notes = %v, want the task told why its workspace was closed", b.notes)
	}
	runs, rerr := history.Runs("TASK-1", 1)
	if rerr != nil || len(runs) != 1 {
		t.Fatalf("history = %+v, %v", runs, rerr)
	}
	if runs[0].Status != history.StatusFailed || !runs[0].TimedOut {
		t.Errorf("closing record = %+v, want failed carrying timed_out", runs[0])
	}
}

// TASK-62: the late window ends the way every window ends — a human closing
// the workspace is a decision, not a failure, wherever it happens.
func TestAnAgentThatVanishesInTheLateWindowGoesBlockedNotFailed(t *testing.T) {
	b := newBoard("TASK-1")
	h := &fakeHost{
		do:     func(host.Spec) error { return fmt.Errorf("waiting for the agent: %w", host.ErrTimedOut) },
		settle: func() error { return host.ErrCancelled },
	}
	_ = run(t, h, b)
	if b.verdict("TASK-1") != work.Blocked {
		t.Fatalf("verdict = %q, want blocked — a human called it off", b.verdict("TASK-1"))
	}
	if len(h.notifies) != 0 {
		t.Fatalf("a cancelled run stays quiet, notifies = %v", h.notifies)
	}
}
