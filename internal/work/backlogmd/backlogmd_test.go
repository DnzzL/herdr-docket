package backlogmd

import (
	"errors"
	"fmt"
	"os/exec"
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
	// branch is what DefaultBranch answers; empty means the queue has no
	// repo to read one from.
	branch string

	created  []string // title, body, assignee, status
	stages   []string // statuses set, in order
	assigned []string // "<id>=<agent>", in order
	comments []string
}

func (f *fakeClient) List() ([]task, error)        { return f.tasks, f.err }
func (f *fakeClient) View(id string) (view, error) { return f.view, f.err }

func (f *fakeClient) Create(title, body, assignee, status string) (string, error) {
	f.created = []string{title, body, assignee, status}
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

// DefaultBranch is what the real CLI would read from the repo. Empty dir
// means no repo behind the queue (every test thus far), and err lets a test
// stand in for a repo with no remote to ask.
func (f *fakeClient) DefaultBranch(dir string) (string, error) {
	if f.branch == "" {
		return "", fmt.Errorf("%s has no remote default branch to name", dir)
	}
	return f.branch, nil
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
	items, err := NewWith("", &fakeClient{tasks: tasks}, Vocabulary{}, nil).List()
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

// Parked, not closed (TASK-53): the queue's own blocked word rides into the
// port as task.Blocked — even though the same word closes the task in the
// fleet's verdict read-back — because nothing else tells pick "a human
// answers this one". Written only where the project named a blocked word:
// a status the fleet was never told about stays a stranger, and the
// whitelist above already keeps it out of routing.
func TestParkedWordCarriesIntoThePortAsBlocked(t *testing.T) {
	tasks := []task{
		{ID: "T-1", Title: "parked", Status: "needs human validation"},
		{ID: "T-2", Title: "claimed", Status: "To Do"},
		{ID: "T-3", Title: "unheard-of", Status: "wontfix"},
	}
	items, err := NewWith("", &fakeClient{tasks: tasks}, Vocabulary{
		Todo: "To Do", InProgress: "In Progress", Done: "Done",
		Failed: "wontfix", Blocked: "needs human validation",
	}, nil).List()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
	wantBlocked := []bool{true, false, false}
	for i, it := range items {
		if it.Blocked != wantBlocked[i] {
			t.Errorf("%s (%s): Blocked = %v, want %v", it.ID, it.Phase, it.Blocked, wantBlocked[i])
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
// whose work this is. A person writes an assignee as `@name` — that is how
// Backlog.md prints one and how its CLI accepts one — so the sigil reaches the
// file: TASK-91 on a real board carried '@dishnow-reviewer'. An agent is a
// folder in the fleet dir and its name has no `@` in it, so the sigil stops at
// the adapter, which is where the backend's conventions are translated. A name
// the fleet cannot match still comes out as that name — unknown and readable
// rather than silently unassigned, or silently handed to the default agent.
func TestAnAssigneeIsReadWithoutBacklogmdsAtSign(t *testing.T) {
	for _, tc := range []struct{ stored, want string }{
		{"@dishnow-reviewer", "dishnow-reviewer"},
		{"dishnow-reviewer", "dishnow-reviewer"},
		{"@thomas", "thomas"},
		{"@", "@"},
	} {
		items, err := NewWith("", &fakeClient{tasks: []task{{
			ID: "T-1", Title: "t", Status: "To Do", Assignees: []string{tc.stored},
		}}}, Vocabulary{}, nil).List()
		if err != nil {
			t.Fatal(err)
		}
		if items[0].Assignee != tc.want {
			t.Errorf("stored %q: Assignee = %q, want %q", tc.stored, items[0].Assignee, tc.want)
		}
	}
}

// The second read has to translate the same way: a task opened in the detail
// view is the one a person presses r on, so a sigil surviving here would route
// the run to nobody while the row above it looked routed.
func TestGetReadsAnAssigneeWithoutTheAtSignToo(t *testing.T) {
	s := NewWith("", &fakeClient{view: view{
		task: task{ID: "TASK-2", Title: "B", Status: "In Progress", Assignees: []string{"@dev"}},
	}}, Vocabulary{}, nil)
	it, err := s.Get("TASK-2")
	if err != nil {
		t.Fatal(err)
	}
	if it.Assignee != "dev" {
		t.Fatalf("Assignee = %q, want %q", it.Assignee, "dev")
	}
}

func TestListCarriesTheRoutingKeyAndOrdering(t *testing.T) {
	items, err := NewWith("", &fakeClient{tasks: []task{{
		ID: "TASK-2", Title: "B", Status: "To Do", Priority: "high",
		Assignees: []string{"dev", "pm"}, Ordinal: 2000, CreatedAt: "2026-08-30T10:00:00Z",
	}}}, Vocabulary{}, nil).List()
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
	if _, err := NewWith("", &fakeClient{err: errors.New("backlog exploded")}, Vocabulary{}, nil).List(); err == nil {
		t.Fatal("a backend failure must not look like an empty queue")
	}
}

// Get is the full task: the list view plus what the prompt needs.
func TestGetCarriesBodyNotesAndCriteria(t *testing.T) {
	s := NewWith("", &fakeClient{view: view{
		task:        task{ID: "TASK-2", Title: "B", Status: "In Progress", Assignees: []string{"dev"}, Labels: []string{"critical"}},
		Description: "do it",
		AcceptanceCriteria: []criterion{
			{Index: 1, Text: "works", Checked: false},
			{Index: 2, Text: "tested", Checked: true},
		},
		ImplementationNotes: "so far",
	}}, Vocabulary{}, nil)
	it, err := s.Get("TASK-2")
	if err != nil {
		t.Fatal(err)
	}
	want := work.Task{
		ID: "TASK-2", Title: "B", Assignee: "dev", Open: true, Phase: "In Progress",
		Labels: []string{"critical"},
		Body:   "do it",
		Notes:  "so far",
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
	id, err := NewWith("", f, Vocabulary{}, nil).Create("Fix the thing", "because it is broken", "dev")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Fix the thing", "because it is broken", "dev", ""}; !reflect.DeepEqual(f.created, want) {
		t.Fatalf("created %v, want %v", f.created, want)
	}
	if id != "TASK-9" {
		t.Fatalf("Create must return the new id, got %q", id)
	}
}

// A create from the fleet's mouth files into the project's own pickup status —
// the word its mapping calls To Do, in the project's own vocabulary — so the
// next tick can claim it (AC#1). One backend write: the status rides the same
// create, and no task ever exists in a column it does not belong in.
func TestCreateTodoFilesIntoTheProjectOwnPickupStatus(t *testing.T) {
	t.Run("the fleet's own words", func(t *testing.T) {
		f := &fakeClient{}
		id, err := NewWith("", f, Vocabulary{}, nil).CreateTodo("Spec it", "why", "pm")
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"Spec it", "why", "pm", statusToDo}; !reflect.DeepEqual(f.created, want) {
			t.Fatalf("created %v, want %v", f.created, want)
		}
		if id != "TASK-9" {
			t.Fatalf("CreateTodo must return the new id, got %q", id)
		}
	})

	t.Run("a project's own words", func(t *testing.T) {
		f := &fakeClient{}
		v := Vocabulary{Todo: "Ready", InProgress: "WIP", Done: "Closed", Failed: "Broke", Blocked: "Held"}
		if _, err := NewWith("", f, v, nil).CreateTodo("Spec it", "", "pm"); err != nil {
			t.Fatal(err)
		}
		if got := f.created[3]; got != "Ready" {
			t.Fatalf("the fleet's word must not leak to the backend: asked for %q, want the project's %q", got, "Ready")
		}
	})

	// plain Create stays at the backend's default — the capability is the
	// only verb that chooses a column.
	t.Run("plain create does not choose a column", func(t *testing.T) {
		f := &fakeClient{}
		NewWith("", f, DefaultVocabulary(), nil).Create("T", "", "")
		if got := f.created[3]; got != "" {
			t.Fatalf("plain Create named a status %q, want the backend's default", got)
		}
	})
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
		if err := NewWith("", f, Vocabulary{}, nil).Close("T-1", tc.verdict); err != nil {
			t.Fatal(err)
		}
		if want := []string{tc.status}; !reflect.DeepEqual(f.stages, want) {
			t.Errorf("%s closed to %v, want %v", tc.verdict, f.stages, want)
		}
	}
}

func TestCloseRejectsAnUnknownVerdict(t *testing.T) {
	f := &fakeClient{}
	if err := NewWith("", f, Vocabulary{}, nil).Close("T-1", work.Verdict("maybe")); err == nil {
		t.Fatal("an unknown verdict must be refused, never silently closed")
	}
	if len(f.stages) != 0 {
		t.Fatalf("a refused verdict must not touch the backend, got %v", f.stages)
	}
}

// ...and the landing survives a readback: work created for the fleet comes
// back open and standing under the fleet's own word for it, which is the
// only thing the next tick reads before claiming.
func TestCreateTodoWorkComesBackClaimable(t *testing.T) {
	v := Vocabulary{Todo: "Ready", InProgress: "WIP", Done: "Closed", Failed: "Broke", Blocked: "Held"}
	s := NewWith("", &memClient{}, v, nil)
	id, err := s.CreateTodo("Route me", "why", "reviewer")
	if err != nil {
		t.Fatal(err)
	}
	it, err := s.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if !it.Open || it.Phase != string(work.PhaseTodo) {
		t.Fatalf("the task the fleet filed is not claimable: open=%v phase=%q", it.Open, it.Phase)
	}
}

func TestCommentAppendsWithoutReplacing(t *testing.T) {
	f := &fakeClient{}
	if err := NewWith("", f, Vocabulary{}, nil).Comment("T-1", "why it failed"); err != nil {
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
	worktest.Run(t, func(t *testing.T) work.Source { return NewWith("", &memClient{}, Vocabulary{}, nil) })
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

func (m *memClient) Create(title, body, assignee, status string) (string, error) {
	m.seq++
	if status == "" {
		status = statusToDo
	}
	t := &memTask{task: task{ID: fmt.Sprintf("TASK-%d", m.seq), Title: title, Status: status}}
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

// DefaultBranch stands in for a repo with no remote: the contract suite's
// queue has no default branch to name, which is the no-answer path.
func (m *memClient) DefaultBranch(string) (string, error) {
	return "", fmt.Errorf("no repo behind this queue")
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
	s := NewWith("", m, Vocabulary{}, nil)
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

// gitInit makes a throwaway repo for the reads DefaultBranch does itself:
// the real adapter has no exec seam for git (it calls git directly), so the
// test stands one repo up and asserts on its actual refs. `remote add` alone
// does not create the remote's HEAD ref — git does that on a real clone — so
// the last line is what a clone produces and the test produces by hand.
func gitInit(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v (%s)", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "fleet@example.com")
	run("config", "user.name", "fleet")
	run("commit", "--allow-empty", "-qm", "first")
	run("remote", "add", "origin", dir)
	run("fetch", "-q", "origin")
	run("update-ref", "refs/remotes/origin/main", "main")
	run("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	return dir
}

// TASK-45: the queue's default branch, derived from its own repo — with the
// checkout left on an unrelated branch, the very shape TASK-23's contamination
// took: the human checked out a feature branch, the run still must cut from
// origin/main.
func TestBaseBranchIsDerivedFromTheQueueRepo(t *testing.T) {
	dir := gitInit(t)
	run := exec.Command("git", "-C", dir, "checkout", "-qb", "human/feature")
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("git checkout -b human/feature: %v (%s)", err, out)
	}
	s := NewWith(dir, newCLI(""), Vocabulary{}, nil)
	ref, err := s.BaseBranch("TASK-1")
	if err != nil {
		t.Fatal(err)
	}
	if ref != "origin/main" {
		t.Fatalf("BaseBranch = %q, want origin/main", ref)
	}
}

// The override (fleet.yaml's worktree_base:) wins over the derived answer:
// a queue deliberately worked against a named ref gets that ref, verbatim.
func TestBaseBranchOverrideWinsOverDerived(t *testing.T) {
	s := NewWith(gitInit(t), &fakeClient{}, Vocabulary{}, map[string]string{"": "upstream/next"})
	ref, err := s.BaseBranch("TASK-1")
	if err != nil {
		t.Fatal(err)
	}
	if ref != "upstream/next" {
		t.Fatalf("BaseBranch = %q, want the configured upstream/next", ref)
	}
}

// A repo with no remote has no default branch to name: the runner inherits
// the checkout's HEAD, exactly the behaviour that preceded this capability.
func TestBaseBranchSaysWhenTheRepoHasNoAnswer(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("git", "-C", dir, "init", "-q")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v (%s)", err, out)
	}
	s := NewWith(dir, newCLI(""), Vocabulary{}, nil)
	if _, err := s.BaseBranch("TASK-1"); err == nil {
		t.Fatal("want an error for a repo with no remote HEAD")
	}
}

// A queue with no repo at all (the fleet dir is not one, say) says the same:
// no answer, not a guess.
func TestBaseBranchRefusesAQueueWithNoRepo(t *testing.T) {
	s := NewWith("", newCLI(""), Vocabulary{}, nil)
	if _, err := s.BaseBranch("TASK-1"); err == nil {
		t.Fatal("want an error when there is no repo behind the queue")
	}
}

// The real CLI reads refs, not the client seam: pin the whole path through
// the exec-backed default branch read.
func TestCliDefaultBranchReadsTheRemoteHead(t *testing.T) {
	dir := gitInit(t)
	ref, err := newCLI(dir).DefaultBranch(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ref != "origin/main" {
		t.Fatalf("DefaultBranch = %q, want origin/main", ref)
	}
}
