package main

import (
	"os"
	"path/filepath"

	"bytes"
	"errors"
	"github.com/DnzzL/herdr-docket/internal/fleet"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/DnzzL/herdr-docket/internal/gate"
	"github.com/DnzzL/herdr-docket/internal/history"
	"github.com/DnzzL/herdr-docket/internal/text"
	"github.com/DnzzL/herdr-docket/internal/work"
)

// fakeSource stands in for the queue so the CLI verbs are tested without a
// backend on disk. Writes are recorded to check the verb actually reached it.
type fakeSource struct {
	items []work.Task
	item  work.Task
	err   error

	created  []string // title, body, assignee
	criteria []string

	comments []string
	verdicts []work.Verdict
}

func (f *fakeSource) List() ([]work.Task, error)       { return f.items, f.err }
func (f *fakeSource) Get(id string) (work.Task, error) { return f.item, f.err }

func (f *fakeSource) Create(title, body, assignee string) (string, error) {
	f.created = []string{title, body, assignee}
	return "TASK-9", f.err
}

func (f *fakeSource) Comment(id, text string) error {
	f.comments = append(f.comments, text)
	return f.err
}

func (f *fakeSource) Close(id string, v work.Verdict) error {
	f.verdicts = append(f.verdicts, v)
	return f.err
}

func runTask(t *testing.T, src work.Source, args ...string) (string, error) {
	t.Helper()
	return runTaskIn(t, src, "", args...)
}

// runTaskIn is runTask with the fleet's default_source set, so the queue a
// create lands in can be tested without naming one on the command line.
func runTaskIn(t *testing.T, src work.Source, defaultQueue string, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := runTaskCmd(taskEnv{src: src, defaultQueue: defaultQueue, pipeline: func(string) fleet.Pipeline { return fleet.Pipeline{} }}, args, &out)
	return out.String(), err
}

// fakeForge answers for one pull request and records what was said on it.
type fakeForge struct {
	pr       gate.PR
	comments []string
	merged   []string
	err      error
}

func (f *fakeForge) PR(string) (gate.PR, error) { return f.pr, f.err }
func (f *fakeForge) Merge(url, head string) error {
	f.merged = append(f.merged, url+"@"+head)
	return nil
}
func (f *fakeForge) Comment(url, body string) error {
	f.comments = append(f.comments, body)
	return nil
}

// runTaskPiped runs a task verb in a queue whose pipeline names a verifier.
func runTaskPiped(t *testing.T, src work.Source, forge gate.Forge, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	env := taskEnv{src: src, forge: forge, pipeline: func(string) fleet.Pipeline { return fleet.Pipeline{Verifier: "rev"} }}
	err := runTaskCmd(env, args, &out)
	return out.String(), err
}

// The agent's view of the queue: open work and who it routes to. Closed tasks
// are noise unless asked for.
func TestTaskListShowsOpenItemsWithTheirRoutingKey(t *testing.T) {
	src := &fakeSource{items: []work.Task{
		{ID: "TASK-2", Title: "B", Assignee: "dev", Open: true, Phase: "To Do"},
		{ID: "TASK-1", Title: "A", Open: false, Phase: "Done"},
	}}
	got, err := runTask(t, src, "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "TASK-2") || !strings.Contains(got, "dev") {
		t.Fatalf("an open task and its assignee must show:\n%s", got)
	}
	if strings.Contains(got, "TASK-1") {
		t.Fatalf("closed work is not listed by default:\n%s", got)
	}
}

func TestTaskListAllIncludesClosedItems(t *testing.T) {
	src := &fakeSource{items: []work.Task{
		{ID: "TASK-2", Title: "B", Open: true, Phase: "To Do"},
		{ID: "TASK-1", Title: "A", Open: false, Phase: "Done"},
	}}
	got, err := runTask(t, src, "list", "--all")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "TASK-1") || !strings.Contains(got, "TASK-2") {
		t.Fatalf("--all must include closed work:\n%s", got)
	}
}

// Get is what the run prompt and a human both need: the body, the history of
// notes, and the criteria, which are read-only.
func TestTaskViewShowsBodyNotesAndCriteria(t *testing.T) {
	src := &fakeSource{item: work.Task{
		ID: "TASK-2", Title: "B", Assignee: "dev", Open: true, Phase: "To Do",
		Body: "do it", Notes: "so far",
		Criteria: []work.Criterion{{Index: 1, Text: "works"}},
	}}
	got, err := runTask(t, src, "view", "TASK-2")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"TASK-2", "do it", "so far", "works"} {
		if !strings.Contains(got, want) {
			t.Errorf("view must show %q:\n%s", want, got)
		}
	}
}

// Creating work is how a run hands on what it found, and how an automation
// seeds the queue: title, the routing key, and the why.
func TestTaskCreatePassesTitleBodyAndAssignee(t *testing.T) {
	src := &fakeSource{}
	got, err := runTask(t, src, "create", "Fix the thing", "-a", "dev", "-d", "because it is broken")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Fix the thing", "because it is broken", "dev"}; !reflect.DeepEqual(src.created, want) {
		t.Fatalf("created %v, want %v", src.created, want)
	}
	if !strings.Contains(got, "TASK-9") {
		t.Fatalf("create should name the new task:\n%s", got)
	}
}

func TestTaskCreateNeedsATitle(t *testing.T) {
	if _, err := runTask(t, &fakeSource{}, "create", "-a", "dev"); err == nil {
		t.Fatal("create without a title must be a usage error")
	}
}

// The CLI output says where the task landed, every time the fleet did not
// choose the column, and never implies a choice the queue made on its own
// (ADR-0012). A task stranded in a column the daemon never reads is exactly
// what a friend in the output would name.
func TestTaskCreateReportsWhereTheTaskLanded(t *testing.T) {
	t.Run("a queue that files into its pickup status, in the fleet's words", func(t *testing.T) {
		src := &multiSource{names: []string{"alpha"}, started: true}
		got, err := runTask(t, src, "create", "T", "-a", "dev", "-s", "alpha")
		if err != nil {
			t.Fatal(err)
		}
		if want := "created alpha/TASK-9 (in To Do)"; !strings.Contains(got, want) {
			t.Fatalf("output %q, want %q", got, want)
		}
	})
	t.Run("a queue without one, in its own words", func(t *testing.T) {
		src := &multiSource{names: []string{"beta"}, started: false}
		got, err := runTask(t, src, "create", "T", "-a", "dev", "-s", "beta")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, "did not choose") {
			t.Fatalf("output must say the fleet did not choose the column:\n%s", got)
		}
	})
	t.Run("one queue that can pick it up, uses its pickup status", func(t *testing.T) {
		src := &todoStarterSource{}
		got, err := runTask(t, src, "create", "T", "-a", "dev")
		if err != nil {
			t.Fatal(err)
		}
		if !src.started {
			t.Fatal("a create through the fleet CLI must reach the pickup capability, not plain Create")
		}
		if !strings.Contains(got, "in To Do") {
			t.Fatalf("output must name the landing:\n%s", got)
		}
	})
	t.Run("one queue that cannot, said so", func(t *testing.T) {
		got, err := runTask(t, &fakeSource{}, "create", "T", "-a", "dev")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, "did not choose") {
			t.Fatalf("output must say the fleet did not choose the column:\n%s", got)
		}
	})
}

// multiSource is a composite queue for the CLI tests: it records which source
// a create targeted, and whether the queue claimed to have filed the task
// into its own pickup column, so -s/--source and the landing report are
// checked without a backend.
type multiSource struct {
	names     []string
	started   bool
	createdIn []string
	err       error
}

func (m *multiSource) List() ([]work.Task, error)       { return nil, m.err }
func (m *multiSource) Get(id string) (work.Task, error) { return work.Task{}, m.err }
func (m *multiSource) Comment(id, text string) error    { return m.err }
func (m *multiSource) Close(id string, v work.Verdict) error {
	return m.err
}
func (m *multiSource) Create(title, body, assignee string) (string, error) {
	_, _, err := m.CreateIn("", title, body, assignee)
	return "", err
}
func (m *multiSource) Names() []string { return m.names }
func (m *multiSource) CreateIn(source, title, body, assignee string) (string, bool, error) {
	m.createdIn = []string{source, title, body, assignee}
	return source + "/TASK-9", m.started, m.err
}

// todoStarterSource is a single queue that can file new work into its own
// pickup status, for the tests that check the CLI says which landing it got.
type todoStarterSource struct {
	fakeSource
	started bool
}

func (f *todoStarterSource) CreateTodo(title, body, assignee string) (string, error) {
	f.started = true
	return "TASK-9", nil
}

// With several queues a create has to name one; with one it is implied; and a
// -s on a fleet that has no queues to choose between is a mistake, not a
// silent ignore.
func TestTaskCreateChoosesTheSource(t *testing.T) {
	t.Run("several queues need -s", func(t *testing.T) {
		src := &multiSource{names: []string{"alpha", "beta"}}
		if _, err := runTask(t, src, "create", "T", "-a", "dev"); err == nil {
			t.Fatal("a create with several queues and no -s must refuse")
		}
		got, err := runTask(t, src, "create", "T", "-a", "dev", "-s", "beta")
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"beta", "T", "", "dev"}; !reflect.DeepEqual(src.createdIn, want) {
			t.Fatalf("created in %v, want %v", src.createdIn, want)
		}
		if !strings.Contains(got, "beta/TASK-9") {
			t.Fatalf("create should name the prefixed id:\n%s", got)
		}
	})

	t.Run("one queue is implied", func(t *testing.T) {
		src := &multiSource{names: []string{"solo"}}
		if _, err := runTask(t, src, "create", "T", "-a", "dev"); err != nil {
			t.Fatal(err)
		}
		if src.createdIn[0] != "solo" {
			t.Fatalf("the sole queue should be implied, got %q", src.createdIn[0])
		}
	})

	t.Run("-s is refused when there is nothing to choose", func(t *testing.T) {
		if _, err := runTask(t, &fakeSource{}, "create", "T", "-s", "nope"); err == nil {
			t.Fatal("-s on a single, unnamed queue must error")
		}
	})
}

// The three closing verbs are the verdict vocabulary: one word each, and the
// task is closed whatever it is.
func TestTaskCloseVerbsMapToVerdicts(t *testing.T) {
	for verb, want := range map[string]work.Verdict{
		"done": work.Done, "fail": work.Failed, "block": work.Blocked,
	} {
		src := &fakeSource{}
		if _, err := runTask(t, src, verb, "TASK-2"); err != nil {
			t.Fatalf("%s: %v", verb, err)
		}
		if !reflect.DeepEqual(src.verdicts, []work.Verdict{want}) {
			t.Errorf("%s closed with %v, want %v", verb, src.verdicts, want)
		}
	}
}

// A note is recorded before the task closes, so the reason is on the task
// even if closing it is the last thing that happens.
func TestTaskCloseWithANoteCommentsFirst(t *testing.T) {
	src := &fakeSource{}
	if _, err := runTask(t, src, "fail", "TASK-2", "--note", "the API is down"); err != nil {
		t.Fatal(err)
	}
	if want := []string{"the API is down"}; !reflect.DeepEqual(src.comments, want) {
		t.Fatalf("commented %v, want %v", src.comments, want)
	}
	if len(src.verdicts) != 1 {
		t.Fatalf("verdicts %v, want exactly one close", src.verdicts)
	}
}

func TestTaskNoteAppendsToTheItem(t *testing.T) {
	src := &fakeSource{}
	if _, err := runTask(t, src, "note", "TASK-2", "half done"); err != nil {
		t.Fatal(err)
	}
	if want := []string{"half done"}; !reflect.DeepEqual(src.comments, want) {
		t.Fatalf("commented %v, want %v", src.comments, want)
	}
	if len(src.verdicts) != 0 {
		t.Fatalf("note must not close the task, got %v", src.verdicts)
	}
}

// A note the agent cannot see succeed is a note it will retry, rephrase and
// doubt (TASK-57 spent a turn reading its own silent writes as failures), so
// the verb confirms the write the way done/fail/block confirm theirs — and a
// failure still prints nothing but the error.
func TestTaskNoteSaysItWrote(t *testing.T) {
	src := &fakeSource{}
	got, err := runTask(t, src, "note", "TASK-2", "half done")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "TASK-2 noted") {
		t.Fatalf("note must confirm the write, got %q", got)
	}

	failing := &fakeSource{err: errors.New("the queue refused")}
	got, err = runTask(t, failing, "note", "TASK-2", "half done")
	if err == nil {
		t.Fatal("a refused note must error")
	}
	if got != "" {
		t.Fatalf("a failed note prints the error and nothing else, got %q", got)
	}
}

func TestTaskVerbsNeedAnID(t *testing.T) {
	for _, verb := range []string{"view", "note", "done", "fail", "block"} {
		if _, err := runTask(t, &fakeSource{}, verb); err == nil {
			t.Errorf("%s without an id must be a usage error", verb)
		}
	}
}

func TestTaskCloseRejectsAnUnknownFlag(t *testing.T) {
	if _, err := runTask(t, &fakeSource{}, "done", "TASK-2", "--wrong"); err == nil {
		t.Fatal("an unknown flag must error rather than be ignored")
	}
}

func TestTaskCmdRejectsAnUnknownVerb(t *testing.T) {
	if _, err := runTask(t, &fakeSource{}, "frobnicate"); err == nil {
		t.Fatal("an unknown task verb must error")
	}
}

func TestTaskCmdSurfacesTheBackendError(t *testing.T) {
	if _, err := runTask(t, &fakeSource{err: errors.New("backend down")}, "list"); err == nil {
		t.Fatal("a backend failure must reach the caller")
	}
}

func TestTaskCloseSurfacesTheBackendError(t *testing.T) {
	if _, err := runTask(t, &fakeSource{err: errors.New("backend down")}, "done", "TASK-2"); err == nil {
		t.Fatal("a failed close must reach the caller")
	}
}

// One renderer, two callers: the CLI verb and the pane's detail view read the
// same task the same way, so the verb prints exactly what the shared renderer
// returns and neither can drift without this failing.
func TestTaskViewIsTheSharedRenderer(t *testing.T) {
	it := work.Task{
		ID: "TASK-2", Title: "B", Assignee: "dev", Open: true, Phase: "To Do",
		Body: "do it", Notes: "so far",
		Criteria: []work.Criterion{{Index: 1, Text: "works"}},
	}
	got, err := runTask(t, &fakeSource{item: it}, "view", "TASK-2")
	if err != nil {
		t.Fatal(err)
	}
	if want := text.TaskDetail(it); got != want {
		t.Fatalf("task view drifted from the shared renderer:\ngot  %q\nwant %q", got, want)
	}
}

// The pull request is where a run's work went out as. The agent knows it at
// the moment it closes, and that is the only moment the fleet learns it: the
// flag stamps it onto the run's history record so the delivery never lives in
// prose on the task. The queue's close comes first — a history that cannot
// write is reported, not allowed to hold the verdict hostage.
func TestTaskCloseRecordsThePullRequest(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	if err := history.Append(history.Record{
		RunID: "r1", Task: "TASK-2", Status: history.StatusRunning, At: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	src := &fakeSource{}
	const url = "https://example.com/pr/9"
	if _, err := runTask(t, src, "done", "TASK-2", "--note", "shipped", "--pr", url); err != nil {
		t.Fatal(err)
	}
	if len(src.verdicts) != 1 || src.verdicts[0] != work.Done {
		t.Fatalf("verdicts = %v, want the close to stand", src.verdicts)
	}
	r, err := history.LastRun("TASK-2")
	if err != nil || r == nil || r.PullRequest != url {
		t.Fatalf("run record = %+v, %v, want the PR stamped on it", r, err)
	}
}

// A close outside any recorded run still closes: the PR cannot be stamped
// anywhere, and the agent is told so rather than left believing it landed.
func TestTaskCloseWithoutARecordedRunSaysThePRCouldNotBeStamped(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	src := &fakeSource{}
	out, err := runTask(t, src, "done", "TASK-2", "--pr", "https://example.com/pr/9")
	if err != nil {
		t.Fatalf("a history miss must not fail the close: %v", err)
	}
	if len(src.verdicts) != 1 {
		t.Fatalf("verdicts = %v, want the close to stand", src.verdicts)
	}
	if !strings.Contains(out, "pull request") {
		t.Fatalf("output %q must say the PR could not be recorded", out)
	}
}

// A fleet grows a second queue years after its prompts were written, and every
// one of them calls `task create` without -s. default_source is what keeps the
// day the second queue appears from being the day every automation starts
// refusing — and an explicit -s still wins over it.
func TestCreateFallsBackToTheFleetsDefaultSource(t *testing.T) {
	t.Run("the default is used when nothing names a queue", func(t *testing.T) {
		src := &multiSource{names: []string{"alpha", "beta"}}
		if _, err := runTaskIn(t, src, "beta", "create", "T", "-a", "dev"); err != nil {
			t.Fatal(err)
		}
		if src.createdIn[0] != "beta" {
			t.Errorf("created in %q, want the default_source", src.createdIn[0])
		}
	})

	t.Run("-s still wins", func(t *testing.T) {
		src := &multiSource{names: []string{"alpha", "beta"}}
		if _, err := runTaskIn(t, src, "beta", "create", "T", "-s", "alpha"); err != nil {
			t.Fatal(err)
		}
		if src.createdIn[0] != "alpha" {
			t.Errorf("created in %q, want the queue -s named", src.createdIn[0])
		}
	})

	t.Run("several queues and no default still refuses", func(t *testing.T) {
		src := &multiSource{names: []string{"alpha", "beta"}}
		_, err := runTaskIn(t, src, "", "create", "T")
		if err == nil {
			t.Fatal("want a refusal")
		}
		if !strings.Contains(err.Error(), "default_source") {
			t.Errorf("the refusal must name the way out: %v", err)
		}
	})
}

// Where a create lands when nothing names a queue: the project the command is
// standing in wins, because it is true without anybody maintaining it, and the
// fleet's declared default is the fallback for a caller standing nowhere.
func TestQueueForPrefersTheProjectYouAreStandingIn(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "app")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	s := fleet.Settings{
		DefaultSource: "fleet",
		Sources: map[string]fleet.SourceConfig{
			"app":   {Dir: app},
			"fleet": {Dir: filepath.Join(root, "fleet")},
		},
	}
	if got := queueFor(app, s); got != "app" {
		t.Errorf("inside the app checkout the queue is %q, want app", got)
	}
	if got := queueFor(root, s); got != "fleet" {
		t.Errorf("standing nowhere the queue is %q, want the default_source", got)
	}
	if got := queueFor("", fleet.Settings{Sources: s.Sources}); got != "" {
		t.Errorf("no project and no default is %q, want empty so the caller refuses", got)
	}
}

// In a queue with a verifier, done --pr is a delivery: the PR is recorded on
// the run and the task stays open for the pipeline.
func TestDoneWithAPRInAVerifiedQueueDeliversWithoutClosing(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	if err := history.Append(history.Record{RunID: "r1", Task: "TASK-2", Status: history.StatusRunning, At: time.Now()}); err != nil {
		t.Fatal(err)
	}
	src := &fakeSource{}
	const url = "https://example.com/pr/9"
	out, err := runTaskPiped(t, src, &fakeForge{}, "done", "TASK-2", "--pr", url, "--note", "verified by X")
	if err != nil {
		t.Fatal(err)
	}
	if len(src.verdicts) != 0 {
		t.Fatalf("verdicts = %v, want the task left open", src.verdicts)
	}
	if r, _ := history.LastRun("TASK-2"); r == nil || r.PullRequest != url {
		t.Fatalf("run record = %+v, want the PR on it", r)
	}
	if len(src.comments) != 1 || !strings.Contains(out, "delivered") {
		t.Fatalf("comments = %v, out = %q", src.comments, out)
	}
}

// Without --pr the same queue closes as always: a task with no code to merge.
func TestDoneWithoutAPRInAVerifiedQueueStillCloses(t *testing.T) {
	src := &fakeSource{}
	if _, err := runTaskPiped(t, src, &fakeForge{}, "done", "TASK-2"); err != nil {
		t.Fatal(err)
	}
	if len(src.verdicts) != 1 || src.verdicts[0] != work.Done {
		t.Fatalf("verdicts = %v", src.verdicts)
	}
}

// A delivery the runner cannot read is no delivery: with no run to stamp, the
// agent is told it failed instead of the task sitting open unseen.
func TestADeliveryWithNoRecordedRunFails(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	if _, err := runTaskPiped(t, &fakeSource{}, &fakeForge{}, "done", "TASK-2", "--pr", "https://example.com/pr/9"); err == nil {
		t.Fatal("want an error when the PR cannot be recorded")
	}
}

// The verdict is pinned to the diff the forge reports now, recorded on the
// verifier's run, and said on the PR in a line a human can grep.
func TestVerdictRecordsThePatchItJudgedAndSaysSoOnThePR(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	if err := history.Append(history.Record{RunID: "v1", Task: "TASK-2", Agent: "rev", Status: history.StatusRunning, At: time.Now()}); err != nil {
		t.Fatal(err)
	}
	src := &fakeSource{}
	forge := &fakeForge{pr: gate.PR{PatchID: "p42", State: "OPEN"}}
	if _, err := runTaskPiped(t, src, forge, "verdict", "TASK-2", "pass", "--pr", "https://example.com/pr/9", "-n", "replayed the repro"); err != nil {
		t.Fatal(err)
	}
	if v, p, _ := history.VerificationFor("v1"); v != "PASS" || p != "p42" {
		t.Fatalf("verification = %q %q", v, p)
	}
	if len(forge.comments) != 1 || !strings.Contains(forge.comments[0], "docket-verdict: PASS patch-id=p42") {
		t.Fatalf("PR comments = %q", forge.comments)
	}
	if len(src.verdicts) != 0 {
		t.Fatalf("a verdict must not close the task: %v", src.verdicts)
	}
}

func TestVerdictRefusesAMissingPROrAnUnknownWord(t *testing.T) {
	for _, args := range [][]string{
		{"verdict", "TASK-2", "PASS"},
		{"verdict", "TASK-2", "MAYBE", "--pr", "u"},
		{"verdict", "TASK-2"},
	} {
		if _, err := runTaskPiped(t, &fakeSource{}, &fakeForge{}, args...); err == nil {
			t.Errorf("%v: want an error", args)
		}
	}
}

// The bar a run hands on must survive the create. Criteria are the thing the
// prompt insists a follow-up carries; a CLI that cannot spell them is the gap
// personas route around.
func (f *fakeSource) WriteCriteria(id string, criteria []string) error {
	f.criteria = criteria
	return nil
}

func TestTaskCreateCarriesCriteria(t *testing.T) {
	src := &fakeSource{}
	_, err := runTask(t, src, "create", "Hand the fix on", "-a", "dev",
		"-d", "why", "--ac", "compiles", "--ac", "the probe stops flaking")
	if err != nil {
		t.Fatal(err)
	}
	if len(src.criteria) != 2 || src.criteria[0] != "compiles" || src.criteria[1] != "the probe stops flaking" {
		t.Fatalf("criteria = %v, want two in order", src.criteria)
	}
}

func TestTaskCreateRefusesAnEmptyCriterion(t *testing.T) {
	if _, err := runTask(t, &fakeSource{}, "create", "T", "--ac", "   "); err == nil {
		t.Fatal("an empty criterion must be refused, not stored")
	}
}

// Degradation is stated, never silent: a queue with no place for the bar
// refuses and names where the words go instead, so the caller's next create
// puts them in -d — and no criterion quietly becomes prose nobody reads.
func TestTaskCreateStatesWhatADegradingQueueCannotStore(t *testing.T) {
	_, err := runTask(t, &bareSource{}, "create", "T", "-d", "body", "--ac", "the bar")
	if err == nil {
		t.Fatal("a queue without CriterionWriter must not pretend the word was stored")
	}
	if !strings.Contains(err.Error(), "does not store acceptance criteria") ||
		!strings.Contains(err.Error(), "the bar") {
		t.Fatalf("refusal must name the loss and carry the words back:\n%v", err)
	}
}

// bareSource is a queue with no criteria place — the Basecamp shape.
type bareSource struct{ work.Source }

func (f bareSource) Create(title, body, assignee string) (string, error) {
	return "T-1", nil
}
