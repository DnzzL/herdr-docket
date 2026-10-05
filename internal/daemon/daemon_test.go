package daemon

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DnzzL/herdr-docket/internal/fleet"
	"github.com/DnzzL/herdr-docket/internal/host"
	"github.com/DnzzL/herdr-docket/internal/runner"
	"github.com/DnzzL/herdr-docket/internal/work"
)

// memSource is a queue in memory: enough for a tick to read a task, claim it,
// comment and close it. A mutex because the run happens in a goroutine.
type memSource struct {
	mu      sync.Mutex
	items   map[string]work.Task
	notes   []string
	phase   map[string]work.Phase
	nextID  int
	withErr error
}

func newMemSource(tasks ...work.Task) *memSource {
	s := &memSource{items: map[string]work.Task{}, phase: map[string]work.Phase{}}
	for _, t := range tasks {
		s.items[t.ID] = t
	}
	return s
}

func (s *memSource) List() ([]work.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.withErr != nil {
		return nil, s.withErr
	}
	out := make([]work.Task, 0, len(s.items))
	for _, t := range s.items {
		out = append(out, t)
	}
	return out, nil
}

func (s *memSource) Get(id string) (work.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.items[id]
	if !ok {
		return work.Task{}, errNoTask
	}
	return t, nil
}

func (s *memSource) Create(title, body, assignee string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	id := "NEW-" + string(rune('0'+s.nextID))
	s.items[id] = work.Task{ID: id, Title: title, Assignee: assignee, Open: true}
	return id, nil
}

func (s *memSource) Comment(id, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notes = append(s.notes, text)
	return nil
}

func (s *memSource) Close(id string, v work.Verdict) error {
	if !v.Known() {
		return errNoTask
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t := s.items[id]
	t.Open, t.Verdict = false, v
	s.items[id] = t
	return nil
}

func (s *memSource) SetPhase(id string, p work.Phase) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.phase[id] = p
	return nil
}

func (s *memSource) closed(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.items[id]
	return ok && !t.Open
}

func (s *memSource) notesFor() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.notes...)
}

var errNoTask = errFmt("no such task")

type errFmt string

func (e errFmt) Error() string { return string(e) }

// fakeHost is a host that provisions instantly and runs nothing: the agent
// settles silently, so the runner's reconcile closes the task Failed. That is
// the write the tick's source must receive.
type fakeHost struct{}

func (fakeHost) Provision(host.Spec) (host.Session, error) {
	return host.Session{WorkspaceID: "ws", PaneID: "p"}, nil
}
func (fakeHost) Inspect(host.Session) (host.Delivery, error) {
	return host.Delivery{}, nil
}
func (fakeHost) Close(host.Session) error { return nil }
func (fakeHost) Do(host.Session, host.Spec, time.Duration) error {
	return nil
}
func (fakeHost) Settle(host.Session, time.Duration) error { return nil }
func (fakeHost) Notify(string, string, string) error      { return nil }

// withFleet swaps the daemon's reads for a fixed fleet: one root-mode agent,
// a fleet dir that need not exist, and a source per tick taken off the queue.
func withFleet(t *testing.T, agents map[string]fleet.Agent, sources ...work.Source) {
	t.Helper()
	loadSettings = func() (fleet.Settings, error) { return fleet.Settings{Dir: "/fleet"}, nil }
	loadAgents = func(string) (map[string]fleet.Agent, []fleet.Diagnostic) { return agents, nil }
	i := 0
	newSource = func(fleet.Settings) (work.Source, error) {
		s := sources[i]
		i++
		return s, nil
	}
	t.Cleanup(func() {
		loadSettings = fleet.LoadSettings
		loadAgents = fleet.LoadAgents
		newSource = fleet.NewSource
	})
}

// waitClosed waits for a run's write to land, so the test asserts on a run
// that has actually finished rather than one still in its goroutine.
func waitClosed(t *testing.T, s *memSource, id string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if s.closed(id) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("%s: no run closed the task", id)
}

// waitEnded waits for the run to let go of the task lock, not just for the
// task to be closed: reconcile closes it mid-attempt, and the run keeps
// working (the failure history, the cleanup) until Runner.Run returns. A tick
// that starts inside that window skips the task as already running
// (daemon.go, runs.Running), so no goroutine ever starts and the queue the
// second tick meant to write to never closes. Production never notices —
// ticks are seconds apart and the next one picks the task up — so it is the
// harness that must wait for the run, not the daemon.
func waitEnded(t *testing.T, runs *runner.Runner, id string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for runs.Running(id) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if runs.Running(id) {
		t.Fatalf("%s: run never released the task lock", id)
	}
}

// The daemon once held one Source for the Runner and built another per tick,
// so pick read the new queue while claim/Comment/Close wrote to the old one.
// One source per evaluation is the fix: a tick's writes land on the queue it
// read from, even when the next tick's source is a different value.
func TestEachTickWritesToTheQueueItReadFrom(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	first := newMemSource(work.Task{ID: "TASK-1", Title: "T", Open: true, Assignee: "a"})
	second := newMemSource(work.Task{ID: "TASK-1", Title: "T", Open: true, Assignee: "a"})
	withFleet(t, map[string]fleet.Agent{
		"a": {Name: "a", Workdir: "/w", Workspace: "root", TimeoutMinutes: 1},
	}, first, second)

	runs := runner.New(fakeHost{}, fleet.Settings{Dir: "/fleet"})
	reported := map[string]bool{}

	evaluate(runs, reported)
	waitClosed(t, first, "TASK-1")
	if second.closed("TASK-1") {
		t.Fatal("the first tick must not write to the queue it never read from")
	}

	// The run that closed the first queue still holds the task lock until it
	// returns; evaluate again only once it is gone, or the in-flight check
	// silently drops the task and the second queue never sees a run.
	waitEnded(t, runs, "TASK-1")
	evaluate(runs, reported)
	waitClosed(t, second, "TASK-1")
}

// An agent works one checkout. Sending it a task from another project would
// run the work in the wrong repository and succeed at it — no error, no
// failed run, just a diff in the wrong place. So it never reaches the router:
// the task is left alone and told, once, what disagrees with what.
func TestWorkIsNeverRoutedToAnAgentOfAnotherProject(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	src := newMemSource(
		work.Task{ID: "dishnow/TASK-1", Title: "product work", Open: true, Assignee: "plugin-dev"},
	)
	withFleet(t, map[string]fleet.Agent{
		"plugin-dev": {Name: "plugin-dev", Workdir: "/w/docket", Workspace: "root", TimeoutMinutes: 1},
	}, src)
	loadSettings = func() (fleet.Settings, error) {
		return fleet.Settings{Dir: "/fleet", Sources: map[string]fleet.SourceConfig{
			"dishnow": {Dir: "/w/dishnow"},
			"docket":  {Dir: "/w/docket"},
		}}, nil
	}

	evaluate(runner.New(fakeHost{}, fleet.Settings{Dir: "/fleet"}), map[string]bool{})

	if src.closed("dishnow/TASK-1") {
		t.Error("a misrouted task must not be run, and must not be closed")
	}
	notes := strings.Join(src.notesFor(), " ")
	for _, want := range []string{"dishnow", "docket", "plugin-dev"} {
		if !strings.Contains(notes, want) {
			t.Errorf("the note must name what disagrees (%q missing): %q", want, notes)
		}
	}
}

// The refusal for an unknown assignee can carry the fix with it: the roster
// is in scope at the point of refusal, so the note names the agents that do
// answer and says reassign. It used to offer `add agents/<name>/AGENT.md` —
// an instruction-shaped option a model took during the 2026-09-25 review leg,
// writing a retired name back onto a task mid-run.
func TestAnUnknownAssigneeIsToldWhoDoesAnswer(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	src := newMemSource(work.Task{ID: "TASK-1", Title: "T", Open: true, Assignee: "dishnow-dev"})
	withFleet(t, map[string]fleet.Agent{
		"dev":      {Name: "dev", Workdir: "/w", Workspace: "root", TimeoutMinutes: 1},
		"reviewer": {Name: "reviewer", Workdir: "/w", Workspace: "root", TimeoutMinutes: 1},
	}, src, src)
	reported := map[string]bool{}
	runs := runner.New(fakeHost{}, fleet.Settings{Dir: "/fleet"})

	evaluate(runs, reported)
	evaluate(runs, reported)

	if src.closed("TASK-1") {
		t.Fatal("an unknown assignee must not run the task")
	}
	notes := src.notesFor()
	if len(notes) != 1 {
		t.Fatalf("want one refusal written once, got %d: %v", len(notes), notes)
	}
	note := notes[0]
	for _, name := range []string{"dev", "reviewer"} {
		if !strings.Contains(note, name) {
			t.Errorf("the note must list the roster (%q missing): %q", name, note)
		}
	}
	if !strings.Contains(note, "reassign") {
		t.Errorf("the note must frame reassignment as the action: %q", note)
	}
	for _, plant := range []string{"add agents/", "AGENT.md"} {
		if strings.Contains(note, plant) {
			t.Errorf("the note must not plant an agent-creation instruction (%q): %q", plant, note)
		}
	}
}

// Routing is only a question for work the fleet could pick up. A closed task
// is nobody's to route, so a mismatch on one is not a problem to report — and
// reporting it writes a comment onto somebody's finished work, every tick,
// for as long as the fleet runs.
//
// TASK-53, one step wider than pick: the pick runs inside the daemon's tick,
// after the misroute scan, so a task parked on a human must be left out of
// everything a tick does with a queue — no run, no refusal comment, no
// close. Tested here rather than in pick because the comment-writing lives
// in this package.
func TestABlockedTaskIsNeverRunAndNeverTalkedTo(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	src := newMemSource(work.Task{
		ID: "fleet/TASK-1", Title: "T", Open: true, Blocked: true,
		Assignee: "dev", Phase: "Blocked",
	})
	withFleet(t, map[string]fleet.Agent{
		"dev": {Name: "dev", Workdir: "/dishnow", Workspace: "root", TimeoutMinutes: 1},
	}, src, src)
	loadSettings = func() (fleet.Settings, error) {
		// Sources make the misroute shape real: the agent works another
		// checkout, so the old code wrote "reassign it to an agent of fleet"
		// onto exactly this parked task — a human column the fleet must not
		// talk to any more than it runs.
		return fleet.Settings{Dir: "/fleet", Sources: map[string]fleet.SourceConfig{
			"dishnow": {Dir: "/dishnow"},
			"fleet":   {Dir: "/w/fleet"},
		}}, nil
	}

	evaluate(runner.New(fakeHost{}, fleet.Settings{Dir: "/fleet"}), map[string]bool{})
	evaluate(runner.New(fakeHost{}, fleet.Settings{Dir: "/fleet"}), map[string]bool{})

	if src.closed("fleet/TASK-1") {
		t.Fatal("parked work must not be run nor closed by a refusal")
	}
	if notes := src.notesFor(); len(notes) != 0 {
		t.Fatalf("a blocked task waits on its human, not on a comment from the fleet: %v", notes)
	}
}

// Routing is only a question for work the fleet could pick up. A closed task
// is nobody's to route, so a mismatch on one is not a problem to report — and
// reporting it writes a comment onto somebody's finished work, every tick,
// for as long as the fleet runs.
func TestAClosedTaskIsNeverCalledMisrouted(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	src := newMemSource(
		work.Task{ID: "dishnow/TASK-1", Title: "finished long ago", Open: false, Assignee: "stall"},
	)
	withFleet(t, map[string]fleet.Agent{
		"stall": {Name: "stall", Workdir: "/w/fleet", Workspace: "root", TimeoutMinutes: 1},
	}, src)
	loadSettings = func() (fleet.Settings, error) {
		return fleet.Settings{Dir: "/fleet", Sources: map[string]fleet.SourceConfig{
			"dishnow": {Dir: "/w/dishnow"},
			"fleet":   {Dir: "/w/fleet"},
		}}, nil
	}

	evaluate(runner.New(fakeHost{}, fleet.Settings{Dir: "/fleet"}), map[string]bool{})

	if n := len(src.notesFor()); n != 0 {
		t.Errorf("a closed task must be left alone, got %d notes: %v", n, src.notesFor())
	}
}

// gatedHost holds every run in Do until released, and counts what started.
type gatedHost struct {
	fakeHost
	mu      sync.Mutex
	started []string
	release chan struct{}
}

func (h *gatedHost) Provision(s host.Spec) (host.Session, error) {
	h.mu.Lock()
	h.started = append(h.started, s.Name)
	h.mu.Unlock()
	return host.Session{WorkspaceID: "ws", PaneID: "p"}, nil
}
func (h *gatedHost) Do(host.Session, host.Spec, time.Duration) error {
	<-h.release
	return nil
}

func (h *gatedHost) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.started)
}

// A task whose run this process holds — a pipeline between its worker and
// its verifier — is not offered to its worker again: the refusal would burn
// the worker's turn for the tick while its next task waited.
func TestATaskThisProcessIsRunningIsNotPickedAgain(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	src := newMemSource(
		work.Task{ID: "TASK-1", Title: "held", Open: true, Assignee: "a", Priority: 3},
		work.Task{ID: "TASK-2", Title: "next", Open: true, Assignee: "a"},
	)
	agents := map[string]fleet.Agent{
		"a": {Name: "a", Workdir: "/w/a", Workspace: "root", TimeoutMinutes: 1},
		"b": {Name: "b", Workdir: "/w/b", Workspace: "root", TimeoutMinutes: 1},
	}
	withFleet(t, agents, src)
	h := &gatedHost{release: make(chan struct{})}
	defer close(h.release)
	runs := runner.New(h, fleet.Settings{Dir: "/fleet"})

	// TASK-1 is in flight under b, the way a verifier stage holds it.
	go runs.Run(src, work.Task{ID: "TASK-1", Title: "held", Open: true}, agents["b"], "manual")
	for h.count() == 0 {
		time.Sleep(time.Millisecond)
	}

	evaluate(runs, map[string]bool{})
	deadline := time.Now().Add(2 * time.Second)
	for h.count() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if h.count() != 2 || !strings.Contains(h.started[1], "TASK-2") {
		t.Fatalf("started = %v, want a to take TASK-2 this tick", h.started)
	}
}

// A paused fleet starts nothing: the tick does not even read the queue.
func TestAPausedFleetStartsNoRun(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	if err := runner.SetPaused(true); err != nil {
		t.Fatal(err)
	}
	src := newMemSource(work.Task{ID: "TASK-1", Title: "T", Open: true, Assignee: "a"})
	withFleet(t, map[string]fleet.Agent{
		"a": {Name: "a", Workdir: "/w", Workspace: "root", TimeoutMinutes: 1},
	}, src)
	runs := runner.New(fakeHost{}, fleet.Settings{Dir: "/fleet"})

	evaluate(runs, map[string]bool{})
	time.Sleep(20 * time.Millisecond)
	if runs.Running("TASK-1") || src.closed("TASK-1") {
		t.Fatal("a paused fleet must not start a run")
	}
}

// blockingHost holds every run in Do until release is closed, so a test can
// see what a tick starts while earlier runs are still in flight.
type blockingHost struct {
	fakeHost
	release chan struct{}
	mu      sync.Mutex
	started []string
}

func (h *blockingHost) Do(s host.Session, spec host.Spec, _ time.Duration) error {
	h.mu.Lock()
	h.started = append(h.started, spec.Prompt)
	h.mu.Unlock()
	<-h.release
	return nil
}

func (h *blockingHost) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.started)
}

func waitStarted(t *testing.T, h *blockingHost, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for h.count() < n && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond) // and no more than n
}

func threeAgents() map[string]fleet.Agent {
	return map[string]fleet.Agent{
		"a": {Name: "a", Workdir: "/w/a", Workspace: "root", TimeoutMinutes: 1},
		"b": {Name: "b", Workdir: "/w/b", Workspace: "root", TimeoutMinutes: 1},
		"c": {Name: "c", Workdir: "/w/c", Workspace: "root", TimeoutMinutes: 1},
	}
}

// max_runs caps the fleet as a whole: one machine runs that many agents at
// once, however many are idle — and runs from an earlier tick still count.
func TestMaxRunsCapsTheFleetAcrossTicks(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	src := newMemSource(
		work.Task{ID: "TASK-1", Title: "T1", Open: true, Assignee: "a"},
		work.Task{ID: "TASK-2", Title: "T2", Open: true, Assignee: "b"},
		work.Task{ID: "TASK-3", Title: "T3", Open: true, Assignee: "c"},
	)
	withFleet(t, threeAgents(), src, src)
	loadSettings = func() (fleet.Settings, error) { return fleet.Settings{Dir: "/fleet", MaxRuns: 2}, nil }
	h := &blockingHost{release: make(chan struct{})}
	defer close(h.release)
	runs := runner.New(h, fleet.Settings{Dir: "/fleet"})

	evaluate(runs, map[string]bool{})
	waitStarted(t, h, 2)
	if h.count() != 2 {
		t.Fatalf("first tick started %d runs, want 2", h.count())
	}
	evaluate(runs, map[string]bool{})
	waitStarted(t, h, 2)
	if h.count() != 2 {
		t.Fatalf("a tick with 2 runs in flight started more: %d", h.count())
	}
}

// A project's max_runs caps that project only: the other queue keeps going.
func TestMaxRunsPerProjectLeavesTheOtherProjectAlone(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	src := newMemSource(
		work.Task{ID: "app/TASK-1", Title: "T1", Open: true, Assignee: "a"},
		work.Task{ID: "app/TASK-2", Title: "T2", Open: true, Assignee: "b"},
		work.Task{ID: "lib/TASK-1", Title: "T3", Open: true, Assignee: "c"},
	)
	withFleet(t, threeAgents(), src)
	loadSettings = func() (fleet.Settings, error) {
		return fleet.Settings{Dir: "/fleet", Sources: map[string]fleet.SourceConfig{
			"app": {MaxRuns: 1},
			"lib": {},
		}}, nil
	}
	h := &blockingHost{release: make(chan struct{})}
	defer close(h.release)
	runs := runner.New(h, fleet.Settings{Dir: "/fleet"})

	evaluate(runs, map[string]bool{})
	waitStarted(t, h, 2)
	app := 0
	for _, id := range runs.InFlight() {
		if strings.HasPrefix(id, "app/") {
			app++
		}
	}
	if h.count() != 2 || app != 1 {
		t.Fatalf("started %d (app %d), want 2 with 1 from app", h.count(), app)
	}
}
