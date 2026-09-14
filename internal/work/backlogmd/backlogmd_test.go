package backlogmd

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/DnzzL/herdr-docket/internal/work"
	"github.com/DnzzL/herdr-docket/internal/work/worktest"
)

// fakeClient is the adapter's seam: the Backlog.md client is a process
// boundary, so standing in for it tests the mapping without shelling out.
// Writes are recorded so the verbs can be checked without a project on disk.
type fakeClient struct {
	tasks []task
	view  view
	err   error

	created  []string // title, body, assignee
	stages   []string // statuses set, in order
	assigned []string // "<id>=<agent>", in order
	comments []string
}

func (f *fakeClient) List() ([]task, error)        { return f.tasks, f.err }
func (f *fakeClient) View(id string) (view, error) { return f.view, f.err }

func (f *fakeClient) Create(title, body, assignee string) (string, error) {
	f.created = []string{title, body, assignee}
	return "TASK-9", f.err
}

func (f *fakeClient) SetStatus(id, status string) error {
	f.stages = append(f.stages, status)
	return f.err
}

func (f *fakeClient) SetAssignee(id, agent string) error {
	f.assigned = append(f.assigned, id+"="+agent)
	return f.err
}

func (f *fakeClient) AppendNote(id, note string) error {
	f.comments = append(f.comments, note)
	return f.err
}

// The status word decides both things: whether the task is open, and which
// phase the board stands it under. The phases are written out as the fleet's
// words rather than read back off the task, so a change on either side of the
// translation has to be a deliberate one.
func TestListMapsEachStatusToOpenAndPhase(t *testing.T) {
	tasks := []task{
		{ID: "T-1", Title: "queued", Status: "To Do"},
		{ID: "T-2", Title: "running", Status: "In Progress"},
		{ID: "T-3", Title: "parked", Status: "Blocked"},
		{ID: "T-4", Title: "broken", Status: "Failed"},
		{ID: "T-5", Title: "finished", Status: "Done"},
	}
	items, err := newWith(&fakeClient{tasks: tasks}, Vocabulary{}).List()
	if err != nil {
		t.Fatal(err)
	}
	want := []bool{true, true, false, false, false}
	wantPhase := []string{"To Do", "In Progress", "Blocked", "Failed", "Done"}
	if len(items) != len(want) {
		t.Fatalf("got %d items, want %d", len(items), len(want))
	}
	for i, it := range items {
		if it.Open != want[i] {
			t.Errorf("%s (%s): Open = %v, want %v", it.ID, it.Phase, it.Open, want[i])
		}
		if it.Phase != wantPhase[i] {
			t.Errorf("%s: Phase = %q, want the fleet's word %q", it.ID, it.Phase, wantPhase[i])
		}
	}
}

// The priority words are this adapter's business, and their order is the part
// that matters: all the core does with a rank is compare it. A word Backlog.md
// adds later, or no priority at all, is the zero rank — the backend having no
// opinion — which has to sort behind every rank this adapter does know.
func TestBacklogPrioritiesRankInOrder(t *testing.T) {
	order := []string{"critical", "high", "medium", "low"}
	for i := 1; i < len(order); i++ {
		if a, b := rank(order[i-1]), rank(order[i]); a <= b {
			t.Errorf("%q = %d should outrank %q = %d", order[i-1], a, order[i], b)
		}
	}
	for _, p := range []string{"", "whenever"} {
		if r := rank(p); r != 0 {
			t.Errorf("rank(%q) = %d, want 0", p, r)
		}
	}
}

// The routing key is the first assignee: the one field pick reads to decide
// whose work this is.
func TestListCarriesTheRoutingKeyAndOrdering(t *testing.T) {
	items, err := newWith(&fakeClient{tasks: []task{{
		ID: "TASK-2", Title: "B", Status: "To Do", Priority: "high",
		Assignees: []string{"dev", "pm"}, Ordinal: 2000, CreatedAt: "2026-08-30T10:00:00Z",
	}}}, Vocabulary{}).List()
	if err != nil {
		t.Fatal(err)
	}
	want := work.Task{
		ID: "TASK-2", Title: "B", Assignee: "dev", Open: true, Phase: "To Do",
		// "high", as a rank the core can compare rather than a word it knows.
		Priority: 3,
		Ordinal:  2000, CreatedAt: "2026-08-30T10:00:00Z",
	}
	if !reflect.DeepEqual(items[0], want) {
		t.Fatalf("got %+v\nwant %+v", items[0], want)
	}
}

func TestListPropagatesTheBackendError(t *testing.T) {
	if _, err := newWith(&fakeClient{err: errors.New("backlog exploded")}, Vocabulary{}).List(); err == nil {
		t.Fatal("a backend failure must not look like an empty queue")
	}
}

// Get is the full task: the list view plus what the prompt needs.
func TestGetCarriesBodyNotesAndCriteria(t *testing.T) {
	s := newWith(&fakeClient{view: view{
		task:        task{ID: "TASK-2", Title: "B", Status: "In Progress", Assignees: []string{"dev"}},
		Description: "do it",
		AcceptanceCriteria: []criterion{
			{Index: 1, Text: "works", Checked: false},
			{Index: 2, Text: "tested", Checked: true},
		},
		ImplementationNotes: "so far",
	}}, Vocabulary{})
	it, err := s.Get("TASK-2")
	if err != nil {
		t.Fatal(err)
	}
	want := work.Task{
		ID: "TASK-2", Title: "B", Assignee: "dev", Open: true, Phase: "In Progress",
		Body:  "do it",
		Notes: "so far",
		Criteria: []work.Criterion{
			{Index: 1, Text: "works", Checked: false},
			{Index: 2, Text: "tested", Checked: true},
		},
	}
	if !reflect.DeepEqual(it, want) {
		t.Fatalf("got %+v\nwant %+v", it, want)
	}
}

func TestCreatePassesTheRoutingKeyToTheBackend(t *testing.T) {
	f := &fakeClient{}
	id, err := newWith(f, Vocabulary{}).Create("Fix the thing", "because it is broken", "dev")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Fix the thing", "because it is broken", "dev"}; !reflect.DeepEqual(f.created, want) {
		t.Fatalf("created %v, want %v", f.created, want)
	}
	if id != "TASK-9" {
		t.Fatalf("Create must return the new id, got %q", id)
	}
}

// Every verdict closes: a blocked or failed task that stayed open would be
// picked up again on the next tick.
func TestCloseMapsEachVerdictToItsStatus(t *testing.T) {
	for _, tc := range []struct {
		verdict work.Verdict
		status  string
	}{
		{work.Done, statusDone},
		{work.Failed, statusFailed},
		{work.Blocked, statusBlocked},
	} {
		f := &fakeClient{}
		if err := newWith(f, Vocabulary{}).Close("T-1", tc.verdict); err != nil {
			t.Fatal(err)
		}
		if want := []string{tc.status}; !reflect.DeepEqual(f.stages, want) {
			t.Errorf("%s closed to %v, want %v", tc.verdict, f.stages, want)
		}
	}
}

func TestCloseRejectsAnUnknownVerdict(t *testing.T) {
	f := &fakeClient{}
	if err := newWith(f, Vocabulary{}).Close("T-1", work.Verdict("maybe")); err == nil {
		t.Fatal("an unknown verdict must be refused, never silently closed")
	}
	if len(f.stages) != 0 {
		t.Fatalf("a refused verdict must not touch the backend, got %v", f.stages)
	}
}

func TestCommentAppendsWithoutReplacing(t *testing.T) {
	f := &fakeClient{}
	if err := newWith(f, Vocabulary{}).Comment("T-1", "why it failed"); err != nil {
		t.Fatal(err)
	}
	if want := []string{"why it failed"}; !reflect.DeepEqual(f.comments, want) {
		t.Fatalf("comments %v, want %v", f.comments, want)
	}
}

// The adapter is held to the port's contract, not just to the mapping of a
// canned response. Every backend the fleet speaks to runs the same suite, so
// a new adapter is judged by behaviour rather than by its author's taste.
func TestSourceMeetsTheContract(t *testing.T) {
	worktest.Run(t, func(t *testing.T) work.Source { return newWith(&memClient{}, Vocabulary{}) })
}

// memClient is a Backlog.md project in miniature: enough state for the
// contract to be exercised end to end, rather than one canned reply per test.
type memClient struct {
	seq   int
	order []string
	tasks map[string]*memTask
}

type memTask struct {
	task     task
	body     string
	notes    string
	criteria []criterion
}

func (m *memClient) put(t *memTask) {
	if m.tasks == nil {
		m.tasks = map[string]*memTask{}
	}
	m.order = append(m.order, t.task.ID)
	m.tasks[t.task.ID] = t
}

func (m *memClient) find(id string) (*memTask, error) {
	mt, ok := m.tasks[id]
	if !ok {
		return nil, fmt.Errorf("no such task %q", id)
	}
	return mt, nil
}

func (m *memClient) List() ([]task, error) {
	tasks := make([]task, 0, len(m.order))
	for _, id := range m.order {
		tasks = append(tasks, m.tasks[id].task)
	}
	return tasks, nil
}

func (m *memClient) View(id string) (view, error) {
	mt, err := m.find(id)
	if err != nil {
		return view{}, err
	}
	return view{
		task:                mt.task,
		Description:         mt.body,
		AcceptanceCriteria:  mt.criteria,
		ImplementationNotes: mt.notes,
	}, nil
}

func (m *memClient) Create(title, body, assignee string) (string, error) {
	m.seq++
	t := &memTask{task: task{ID: fmt.Sprintf("TASK-%d", m.seq), Title: title, Status: statusToDo}}
	if assignee != "" {
		t.task.Assignees = []string{assignee}
	}
	t.body = body
	m.put(t)
	return t.task.ID, nil
}

func (m *memClient) SetStatus(id, status string) error {
	mt, err := m.find(id)
	if err != nil {
		return err
	}
	mt.task.Status = status
	return nil
}

func (m *memClient) SetAssignee(id, agent string) error {
	mt, err := m.find(id)
	if err != nil {
		return err
	}
	mt.task.Assignees = nil
	if agent != "" {
		mt.task.Assignees = []string{agent}
	}
	return nil
}

func (m *memClient) AppendNote(id, note string) error {
	mt, err := m.find(id)
	if err != nil {
		return err
	}
	if mt.notes != "" {
		mt.notes += "\n\n"
	}
	mt.notes += note
	return nil
}

// The port owns the verdict vocabulary; this adapter owns the translation into
// Backlog.md's status words. Adding a verdict to the port without a status to
// record it would otherwise close the task into an empty status.
func TestEveryKnownVerdictHasAStatus(t *testing.T) {
	for _, v := range []work.Verdict{work.Done, work.Failed, work.Blocked} {
		if !v.Known() {
			t.Fatalf("%q is a port verdict but Known() denies it", v)
		}
		if DefaultVocabulary().status(v) == "" {
			t.Errorf("verdict %q has no Backlog.md status to record it as", v)
		}
	}
	if work.Verdict("probably").Known() {
		t.Error("Known() must not accept a verdict the port does not write")
	}
}

// A PM specs work and hands it to a dev. Assigning replaces the assignee
// rather than adding one: the routing rule reads a single agent off a task,
// so a second name would silently never be routed to.
func TestAssignReplacesTheAgent(t *testing.T) {
	m := &memClient{}
	s := newWith(m, Vocabulary{})
	id, err := s.Create("Spec the thing", "", "pm")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Assign(id, "dev"); err != nil {
		t.Fatal(err)
	}
	it, err := s.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if it.Assignee != "dev" {
		t.Fatalf("assignee = %q, want dev", it.Assignee)
	}
	if got := m.tasks[id].task.Assignees; len(got) != 1 {
		t.Fatalf("assignees = %v, want exactly one", got)
	}
}
