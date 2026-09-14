package daemon

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DnzzL/herdr-docket/internal/fleet"
	"github.com/DnzzL/herdr-docket/internal/history"
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
func (fakeHost) Close(host.Session) error { return nil }
func (fakeHost) Do(host.Session, host.Spec, time.Duration) error {
	return nil
}

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

	evaluate(runs, reported)
	waitClosed(t, second, "TASK-1")
}

// A spent agent is Unavailable, not unknown: its task stays open with no
// comment written, exactly like a busy agent. The queue keeps moving.
func TestAnAgentPastItsBudgetStartsNoRun(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	now := time.Now()
	for i := 0; i < 6; i++ {
		if err := history.Append(history.Record{
			RunID: "run-" + string(rune('0'+i)), Task: "TASK-0", Agent: "a",
			Status: history.StatusDone, At: now.Add(-time.Duration(i) * time.Minute),
			DurationSeconds: 60, Verdict: "done",
		}); err != nil {
			t.Fatal(err)
		}
	}
	src := newMemSource(work.Task{ID: "TASK-1", Title: "T", Open: true, Assignee: "a"})
	withFleet(t, map[string]fleet.Agent{
		"a": {Name: "a", Workdir: "/w", Workspace: "root", TimeoutMinutes: 1, RunsPerDay: 6},
	}, src)

	evaluate(runner.New(fakeHost{}, fleet.Settings{Dir: "/fleet"}), map[string]bool{})

	if src.closed("TASK-1") {
		t.Fatal("an agent past its budget must not run")
	}
	if notes := src.notesFor(); len(notes) != 0 {
		t.Fatalf("the task must stay open unremarked, got notes %v", notes)
	}
	// A budgeted agent is not an unknown one: the daemon must not write the
	// "is not a fleet agent" note it writes for a real routing mistake.
	for _, n := range src.notesFor() {
		if strings.Contains(n, "not a fleet agent") {
			t.Fatalf("a spent agent must not be reported unknown: %q", n)
		}
	}
}
