package multi

import (
	"fmt"
	"strings"
	"testing"

	"github.com/DnzzL/herdr-docket/internal/work"
	"github.com/DnzzL/herdr-docket/internal/work/worktest"
)

// memSource is a sub-source in memory: enough to exercise the composite
// without a backend on disk. It is deliberately a correct little Source,
// because the conformance suite below holds the composite to it.
type memSource struct {
	items    map[string]work.Task
	order    []string
	next     int
	comments []string
	closed   []work.Verdict
	err      error
}

func newMem() *memSource { return &memSource{items: map[string]work.Task{}} }

func (m *memSource) List() ([]work.Task, error) {
	if m.err != nil {
		return nil, m.err
	}
	var out []work.Task
	for _, id := range m.order {
		out = append(out, m.items[id])
	}
	return out, nil
}

func (m *memSource) Get(id string) (work.Task, error) {
	if m.err != nil {
		return work.Task{}, m.err
	}
	it, ok := m.items[id]
	if !ok {
		return work.Task{}, fmt.Errorf("no task %q", id)
	}
	return it, nil
}

func (m *memSource) Create(title, body, assignee string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	m.next++
	id := fmt.Sprintf("T%d", m.next)
	m.items[id] = work.Task{ID: id, Title: title, Body: body, Assignee: assignee, Open: true}
	m.order = append(m.order, id)
	return id, nil
}

func (m *memSource) Comment(id, text string) error {
	if m.err != nil {
		return m.err
	}
	it, ok := m.items[id]
	if !ok {
		return fmt.Errorf("no task %q", id)
	}
	it.Notes += text + "\n"
	m.items[id] = it
	m.comments = append(m.comments, text)
	return nil
}

func (m *memSource) Close(id string, v work.Verdict) error {
	if m.err != nil {
		return m.err
	}
	if !v.Known() {
		return fmt.Errorf("unknown verdict %q", v)
	}
	it, ok := m.items[id]
	if !ok {
		return fmt.Errorf("no task %q", id)
	}
	it.Open = false
	m.items[id] = it
	m.closed = append(m.closed, v)
	return nil
}

// phaserSource is a sub-source that can show a phase — the Backlog.md-shaped
// one beside the Basecamp-shaped one that cannot.
type phaserSource struct {
	*memSource
	phases []work.Phase
}

func (p *phaserSource) SetPhase(id string, phase work.Phase) error {
	p.phases = append(p.phases, phase)
	return nil
}

// A composite over one queue is still a Source: the contract every adapter is
// held to runs against it exactly as it would against the backend beneath.
func TestCompositeSatisfiesTheSourceContract(t *testing.T) {
	worktest.Run(t, func(t *testing.T) work.Source {
		return New(map[string]work.Source{"q": newMem()})
	})
}

// Every open task from every queue shows, and each carries the prefix that
// says which queue it came from.
func TestListPrefixesIDsFromEverySource(t *testing.T) {
	a, b := newMem(), newMem()
	mustCreate(t, a, "A-task")
	mustCreate(t, b, "B-task")
	src := New(map[string]work.Source{"alpha": a, "beta": b})

	items, err := src.List()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, it := range items {
		got[it.ID] = true
	}
	for _, want := range []string{"alpha/T1", "beta/T1"} {
		if !got[want] {
			t.Fatalf("List is missing %q; got %v", want, got)
		}
	}
	// Names is the same order List walks: stable, so the board does not
	// reshuffle two projects' tasks between ticks.
	names := src.Names()
	if len(names) != 2 || names[0] != "alpha" || names[1] != "beta" {
		t.Fatalf("Names = %v, want the sorted [alpha beta]", names)
	}
}

// A prefixed id routes every write to the queue it named — Get, Comment and
// Close alike — and Get hands the task back under the prefixed id, not the
// backend's bare one.
func TestWritesRouteToTheSourceThePrefixNames(t *testing.T) {
	a, b := newMem(), newMem()
	mustCreate(t, a, "A-task")
	mustCreate(t, b, "B-task")
	src := New(map[string]work.Source{"alpha": a, "beta": b})

	if err := src.Comment("beta/T1", "on beta"); err != nil {
		t.Fatal(err)
	}
	if err := src.Close("beta/T1", work.Done); err != nil {
		t.Fatal(err)
	}
	it, err := src.Get("beta/T1")
	if err != nil {
		t.Fatal(err)
	}
	if it.ID != "beta/T1" {
		t.Errorf("Get returned id %q, want the prefixed %q", it.ID, "beta/T1")
	}
	if it.Open {
		t.Error("Close must have closed beta's task")
	}
	if len(a.comments) != 0 || len(a.closed) != 0 {
		t.Errorf("writes meant for beta reached alpha: %v %v", a.comments, a.closed)
	}
	if len(b.comments) != 1 || len(b.closed) != 1 {
		t.Errorf("beta did not receive the writes: %v %v", b.comments, b.closed)
	}
}

// An id the fleet cannot route is an error, never a silent no-op: a typo must
// not look like work that happened.
func TestAnUnroutableIDIsAnError(t *testing.T) {
	src := New(map[string]work.Source{"alpha": newMem()})
	for _, id := range []string{"TASK-1", "ghost/T1", "/T1", "alpha/"} {
		if _, err := src.Get(id); err == nil {
			t.Errorf("Get(%q) must error", id)
		}
		if err := src.Comment(id, "x"); err == nil {
			t.Errorf("Comment(%q) must error", id)
		}
		if err := src.Close(id, work.Done); err == nil {
			t.Errorf("Close(%q) must error", id)
		}
		if err := src.SetPhase(id, work.PhaseInProgress); err == nil {
			t.Errorf("SetPhase(%q) must error", id)
		}
	}
}

// Phaser forwarding is per sub-source: the one that can show a phase gets it,
// the one that cannot is left quiet rather than failing the write.
func TestSetPhaseForwardsOnlyToSourcesThatCanShowAPhase(t *testing.T) {
	with := &phaserSource{memSource: newMem()}
	without := newMem()
	mustCreate(t, with.memSource, "phaseful")
	mustCreate(t, without, "phaseless")
	src := New(map[string]work.Source{"with": with, "without": without})

	if err := src.SetPhase("with/T1", work.PhaseInProgress); err != nil {
		t.Fatalf("a phaser sub-source must receive the phase: %v", err)
	}
	if len(with.phases) != 1 || with.phases[0] != work.PhaseInProgress {
		t.Fatalf("phase not forwarded: %v", with.phases)
	}
	if err := src.SetPhase("without/T1", work.PhaseInProgress); err != nil {
		t.Fatalf("a sub-source without phases is a no-op, not an error: %v", err)
	}
}

// Creating needs a target once there is more than one queue: a bare Create
// cannot guess, so it says so, and CreateIn reaches the named one.
func TestCreateNeedsATargetWhenSeveralQueuesExist(t *testing.T) {
	a, b := newMem(), newMem()
	src := New(map[string]work.Source{"alpha": a, "beta": b})

	if _, err := src.Create("T", "b", "dev"); err == nil {
		t.Fatal("a bare Create with several queues must refuse rather than guess")
	}
	id, err := src.CreateIn("beta", "T", "b", "dev")
	if err != nil {
		t.Fatal(err)
	}
	if id != "beta/T1" {
		t.Fatalf("CreateIn returned %q, want %q", id, "beta/T1")
	}
	if len(a.order) != 0 {
		t.Errorf("CreateIn(beta) wrote to alpha: %v", a.order)
	}
	if _, err := src.CreateIn("ghost", "T", "b", "dev"); err == nil {
		t.Fatal("CreateIn on an unknown source must error")
	}
}

// A composite with one queue still creates through a bare Create, so the CLI
// and the conformance suite need no special case for "only one source".
func TestCreateWithOneQueueTargetsIt(t *testing.T) {
	only := newMem()
	src := New(map[string]work.Source{"solo": only})
	id, err := src.Create("T", "b", "dev")
	if err != nil {
		t.Fatal(err)
	}
	if id != "solo/T1" {
		t.Fatalf("Create returned %q, want %q", id, "solo/T1")
	}
}

func mustCreate(t *testing.T, src *memSource, title string) string {
	t.Helper()
	id, err := src.Create(title, "", "dev")
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// Re-routing follows the prefix like every other verb. A backend with no
// assignee to write refuses out loud: a silent no-op would leave the task
// with the old agent and look like it moved.
func TestAssignRoutesOnThePrefixAndRefusesWhatCannotRoute(t *testing.T) {
	assignable := &fakeAssigner{Source: newMem()}
	s := New(map[string]work.Source{
		"myapp": assignable,
		"plain": newMem(),
	})
	if err := s.Assign("myapp/TASK-1", "dev"); err != nil {
		t.Fatalf("Assign: %v", err)
	}
	if assignable.got != "TASK-1=dev" {
		t.Fatalf("sub-source saw %q, want the bare id", assignable.got)
	}
	err := s.Assign("plain/TASK-1", "dev")
	if err == nil {
		t.Fatal("a source that cannot reassign must say so")
	}
	if !strings.Contains(err.Error(), "plain") {
		t.Fatalf("the error must name the source, got %q", err)
	}
	if err := s.Assign("TASK-1", "dev"); err == nil {
		t.Fatal("a bare id names no queue and must not be guessed")
	}
}

type fakeAssigner struct {
	work.Source
	got string
}

func (f *fakeAssigner) Assign(id, agent string) error {
	f.got = id + "=" + agent
	return nil
}
