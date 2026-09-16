package pane

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/DnzzL/herdr-docket/internal/fleet"
	"github.com/DnzzL/herdr-docket/internal/history"
	"github.com/DnzzL/herdr-docket/internal/text"
	"github.com/DnzzL/herdr-docket/internal/work"
)

func TestRowsGroupsByPhaseOrderAndSkipsEmptyPhases(t *testing.T) {
	got := rows([]work.Task{
		{ID: "T-1", Phase: "Done"},
		{ID: "T-2", Phase: "To Do"},
		{ID: "T-3", Phase: "To Do"},
	}, map[string]*history.Record{}, "")
	want := []string{"To Do", "T-2", "T-3", "", "Done", "T-1"}
	if len(got) != len(want) {
		t.Fatalf("got %d rows", len(got))
	}
	for i, w := range want {
		if w == "" {
			if !got[i].spacer {
				t.Fatalf("row %d = %+v, want spacer", i, got[i])
			}
			continue
		}
		label := got[i].header
		if label == "" {
			label = got[i].task.ID
		}
		if label != w {
			t.Fatalf("row %d = %q, want %q", i, label, w)
		}
	}
}

// A backend whose phases the fleet has never heard of still gets a board:
// the group is there, it just sorts after the phases the fleet knows.
func TestRowsShowsAPhaseTheFleetDoesNotKnow(t *testing.T) {
	got := rows([]work.Task{
		{ID: "T-1", Phase: "Needs Triage"},
		{ID: "T-2", Phase: "In Progress"},
	}, map[string]*history.Record{}, "")
	var labels []string
	for _, r := range got {
		switch {
		case r.spacer:
			labels = append(labels, "")
		case r.header != "":
			labels = append(labels, r.header)
		default:
			labels = append(labels, r.task.ID)
		}
	}
	want := []string{"In Progress", "T-2", "", "Needs Triage", "T-1"}
	if len(labels) != len(want) {
		t.Fatalf("got %v, want %v", labels, want)
	}
	for i := range want {
		if labels[i] != want[i] {
			t.Fatalf("got %v, want %v", labels, want)
		}
	}
}

func TestRowsFiltersByQueryCaseInsensitive(t *testing.T) {
	items := []work.Task{
		{ID: "T-1", Title: "Fix login bug", Phase: "To Do"},
		{ID: "T-2", Title: "Add search", Phase: "To Do"},
		{ID: "T-3", Title: "Unrelated", Phase: "Done"},
	}
	got := rows(items, map[string]*history.Record{}, "SEARCH")
	var ids []string
	for _, r := range got {
		if r.header == "" && !r.spacer {
			ids = append(ids, r.task.ID)
		}
	}
	if len(ids) != 1 || ids[0] != "T-2" {
		t.Fatalf("got %v, want only T-2", ids)
	}
}

// A run in flight is the first thing the board answers, and the task appears
// once: under Running, not under the phase it also carries. Two rows for one
// task would be two places to press the same key.
func TestRowsShowsARunningTaskOnceUnderRunning(t *testing.T) {
	items := []work.Task{
		{ID: "T-1", Title: "running", Open: true, Phase: "In Progress"},
		{ID: "T-2", Title: "waiting", Open: true, Phase: "To Do"},
	}
	last := map[string]*history.Record{
		"T-1": {Task: "T-1", Status: history.StatusRunning},
		"T-2": {Task: "T-2", Status: history.StatusDone},
	}
	got := labels(rows(items, last, ""))
	want := []string{"Running", "T-1", "", "To Do", "T-2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// A record nobody closed — a daemon that died mid-run — still keeps the task
// in Running: the board marks it stale rather than hiding a run that may have
// happened.
func TestRowsKeepsAStaleRunInRunning(t *testing.T) {
	last := map[string]*history.Record{"T-1": {Status: history.StatusRunning}}
	got := rows([]work.Task{{ID: "T-1", Phase: "To Do"}}, last, "")
	if len(got) < 2 || got[0].header != runningHeader || got[1].task.ID != "T-1" {
		t.Fatalf("got %+v", got)
	}
}

// The order the scheduler would work them in, which is what makes the top row
// of To Do the task the daemon picks next.
func TestRowsOrdersAnOpenPhaseAsTheSchedulerWould(t *testing.T) {
	items := []work.Task{
		{ID: "T-1", Open: true, Phase: "To Do", Priority: 1},
		{ID: "T-2", Open: true, Phase: "To Do", Priority: 9},
		{ID: "T-3", Open: true, Phase: "To Do", Priority: 5},
	}
	got := labels(rows(items, map[string]*history.Record{}, ""))
	want := []string{"To Do", "T-2", "T-3", "T-1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// An ending is not urgent, so a closed phase keeps what the backend said.
func TestRowsLeavesAClosedPhaseInBackendOrder(t *testing.T) {
	items := []work.Task{
		{ID: "T-1", Phase: "Done", Priority: 1},
		{ID: "T-2", Phase: "Done", Priority: 9},
	}
	got := labels(rows(items, map[string]*history.Record{}, ""))
	want := []string{"Done", "T-1", "T-2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// The queue a task came from is on the row, so it can be searched by name.
func TestMatchesFindsTheQueueName(t *testing.T) {
	it := work.Task{ID: "myapp/TASK-12", Title: "Fix the parser", Phase: "To Do"}
	if !matches(it, "myapp") {
		t.Fatal("the queue name should match")
	}
	if matches(it, "agency") {
		t.Fatal("another queue's name should not match")
	}
}

func TestProjectsOfListsTheQueuesInAStableOrder(t *testing.T) {
	got := projectsOf([]work.Task{
		{ID: "zeta/T-1"}, {ID: "alpha/T-2"}, {ID: "zeta/T-3"},
	})
	want := []string{"alpha", "zeta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got := projectsOf([]work.Task{{ID: "TASK-1"}}); len(got) != 0 {
		t.Fatalf("a prefixless queue has no project column: %v", got)
	}
}

// The cursor is on a task, not on a row number: runs starting and ending
// reorder the list under it, and the next keypress must not land elsewhere.
func TestCursorFollowsItsTaskWhenTheListReorders(t *testing.T) {
	m := model{
		tasks: []work.Task{
			{ID: "T-1", Open: true, Phase: "To Do", Priority: 1},
			{ID: "T-2", Open: true, Phase: "To Do", Priority: 2},
		},
		last: map[string]*history.Record{},
	}
	m.rebuildRows()
	m.clampSel()
	if m.selID != "T-2" {
		t.Fatalf("cursor starts on T-2, the urgent one: %q", m.selID)
	}
	m.tasks[0].Priority = 9 // T-1 becomes the one that runs next
	m.rebuildRows()
	m.clampSel()
	if m.selID != "T-2" || m.rows[m.sel].task.ID != "T-2" {
		t.Fatalf("cursor moved to %q, want T-2", m.rows[m.sel].task.ID)
	}
}

// A task that leaves the list takes the cursor to the nearest surviving row,
// never back to the top: a keypress after a task closes is still aimed where
// the hand left it.
func TestCursorOnAVanishedTaskTakesTheNearestRow(t *testing.T) {
	m := model{
		tasks: []work.Task{
			{ID: "T-1", Open: true, Phase: "To Do"},
			{ID: "T-2", Open: true, Phase: "To Do"},
			{ID: "T-3", Open: true, Phase: "To Do"},
		},
		last: map[string]*history.Record{},
	}
	m.rebuildRows()
	m.clampSel()
	m.move(1) // onto T-2
	if m.selID != "T-2" {
		t.Fatalf("cursor = %q, want T-2", m.selID)
	}
	m.tasks = []work.Task{m.tasks[0], m.tasks[2]} // T-2 closed elsewhere
	m.rebuildRows()
	m.clampSel()
	if m.rows[m.sel].task.ID != "T-3" {
		t.Fatalf("cursor landed on %q, want T-3", m.rows[m.sel].task.ID)
	}
}

// Typing a filter must not throw the cursor away from the task it is on.
func TestFilteringKeepsTheCursorOnItsTask(t *testing.T) {
	m := model{
		tasks: []work.Task{
			{ID: "T-1", Title: "Fix login", Open: true, Phase: "To Do"},
			{ID: "T-2", Title: "Add search", Open: true, Phase: "To Do"},
		},
		last: map[string]*history.Record{},
	}
	m.rebuildRows()
	m.clampSel()
	m.move(1) // onto T-2
	m.query = "search"
	m.rebuildRows()
	m.clampSel()
	if m.rows[m.sel].task.ID != "T-2" {
		t.Fatalf("cursor landed on %q, want T-2", m.rows[m.sel].task.ID)
	}
}

func TestDurationReadsAsAOneLineColumn(t *testing.T) {
	for _, tc := range []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "<1m"},
		{12 * time.Minute, "12m"},
		{64 * time.Minute, "1h04m"},
		{3 * time.Hour, "3h00m"},
	} {
		if got := duration(tc.d); got != tc.want {
			t.Errorf("duration(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

// labels reads a row list the way the board reads it: group headers, task ids
// and a blank for each spacer.
func labels(rows []row) []string {
	var out []string
	for _, r := range rows {
		switch {
		case r.spacer:
			out = append(out, "")
		case r.header != "":
			out = append(out, r.header)
		default:
			out = append(out, r.task.ID)
		}
	}
	return out
}

// --- the acting half (TASK-28) ---

// One renderer, two callers: the pane shows exactly what the CLI prints, so the
// task a person reads in the pane cannot be a different task from the one the
// agent is prompted with.
func TestDetailViewIsTheSharedRenderer(t *testing.T) {
	it := work.Task{
		ID: "myapp/TASK-12", Title: "Fix the parser", Open: true, Phase: "To Do",
		Assignee: "dev", Body: "the numbers are not moving",
		Criteria: []work.Criterion{{Index: 1, Text: "a test fails first"}},
		Notes:    "tried once",
	}
	m := model{dir: "/fleet", detail: it}
	got := m.detailView()
	for _, want := range strings.Split(strings.TrimRight(text.TaskDetail(it), "\n"), "\n") {
		if !strings.Contains(got, want) {
			t.Fatalf("detail view is missing %q:\n%s", want, got)
		}
	}
}

// p rewrites one line of AGENT.md: the persona below it is still there, and the
// status line says what pause did not do, because a run in flight keeps going.
func TestPausingAnAgentKeepsThePersonaAndSaysWhatItDidNotDo(t *testing.T) {
	dir := t.TempDir()
	writeAgent(t, dir, "dev", "---\nworkdir: "+dir+"\n---\nYou are the dev persona.\n")
	m := model{dir: dir, agents: map[string]fleet.Agent{"dev": {Name: "dev"}}}

	next, _ := m.toggleAgent("dev")
	got := next.(model)
	if !strings.Contains(got.status, "in flight keeps going") || !strings.Contains(got.status, "x stops it") {
		t.Fatalf("status must say what pause did not do: %q", got.status)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "agents", "dev", "AGENT.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "disabled: true") {
		t.Fatalf("pause did not reach the file:\n%s", raw)
	}
	if !strings.Contains(string(raw), "You are the dev persona.") {
		t.Fatalf("pause ate the persona:\n%s", raw)
	}
}

// The roster says what an agent is doing, or that it is free: p is a decision,
// not a blind toggle.
func TestAgentsViewSaysBusyWithItsTaskAndIdle(t *testing.T) {
	started := time.Now().Add(-12 * time.Minute)
	m := model{
		dir: "/fleet",
		agents: map[string]fleet.Agent{
			"dev":    {Name: "dev", Kind: "claude", TimeoutMinutes: 60},
			"writer": {Name: "writer", Kind: "claude", TimeoutMinutes: 60},
		},
		tasks: []work.Task{{ID: "myapp/TASK-12", Title: "Fix the parser", Open: true, Phase: "In Progress", Assignee: "dev"}},
		last:  map[string]*history.Record{"myapp/TASK-12": {Status: history.StatusRunning, At: started}},
	}
	got := m.agentsView()
	for _, want := range []string{"busy", "TASK-12", "myapp", "Fix the parser", "12m / 60m", "idle"} {
		if !strings.Contains(got, want) {
			t.Fatalf("roster is missing %q:\n%s", want, got)
		}
	}
}

// A manual run beats a pause — the pause is a rule for the scheduler — and the
// status line says so, so a run on a paused agent does not look like a bug.
func TestRunningAPausedAgentsTaskStillRunsAndSaysSo(t *testing.T) {
	m := model{
		agents: map[string]fleet.Agent{"dev": {Name: "dev", Disabled: true}},
		tasks:  []work.Task{{ID: "T-1", Open: true, Phase: "To Do", Assignee: "dev"}},
		last:   map[string]*history.Record{},
	}
	m.rebuildRows()
	m.clampSel()
	next, cmd := m.runSelected()
	if next.(model).status != "T-1: agent dev is paused — running it anyway" {
		t.Fatalf("status = %q", next.(model).status)
	}
	if cmd == nil {
		t.Fatal("r must still return the run command")
	}
}

// x is the key for a run that is going nowhere: it says stopping straight away
// and hands back the call that closes the workspace. The Blocked verdict that
// follows is the runner's, not the board's.
func TestStopSaysStoppingAndOnlyWithARunInFlight(t *testing.T) {
	m := model{
		tasks: []work.Task{{ID: "T-1", Open: true, Phase: "To Do"}},
		last:  map[string]*history.Record{"T-1": {Status: history.StatusRunning, WorkspaceID: "ws-1"}},
	}
	m.rebuildRows()
	m.clampSel()
	cmd := m.stopSelected()
	if m.status != "stopping T-1…" {
		t.Fatalf("status = %q, want it to say stopping", m.status)
	}
	if cmd == nil {
		t.Fatal("stopping a run must return the call that closes its workspace")
	}

	idle := model{tasks: []work.Task{{ID: "T-1", Open: true, Phase: "To Do"}}, last: map[string]*history.Record{}}
	idle.rebuildRows()
	idle.clampSel()
	if cmd := idle.stopSelected(); cmd != nil || idle.status != "no run to stop" {
		t.Fatalf("nothing to stop: cmd=%v status=%q", cmd, idle.status)
	}
}

// s re-routes through the port's Assigner, and a backend that cannot reassign
// says so in its own words rather than being papered over.
func TestAssignReachesTheSourcesAssignerAndReportsARefusal(t *testing.T) {
	src := &reassigningSource{}
	if msg := assign(src, "myapp/TASK-12", "writer")().(ranMsg); msg.err != nil {
		t.Fatalf("assign: %v", msg.err)
	}
	if len(src.got) != 2 || src.got[0] != "myapp/TASK-12" || src.got[1] != "writer" {
		t.Fatalf("assigner got %v", src.got)
	}
	msg := assign(&recordingSource{}, "T-1", "dev")().(ranMsg)
	if msg.err == nil || !strings.Contains(msg.err.Error(), "routing") {
		t.Fatalf("a source that cannot reassign must refuse: %v", msg.err)
	}
}

// With several queues the add flow asks which one before it writes: a composite
// cannot guess and the write would fail after the fact.
func TestAddFlowAsksForTheQueueOnlyWhenThereAreSeveral(t *testing.T) {
	multi := &multiQueueSource{names: []string{"alpha", "zeta"}}
	m := model{src: multi, agents: map[string]fleet.Agent{"dev": {Name: "dev"}}}
	m.mode, m.input = addingTitle, "Fix the parser"
	m, _ = press(m, "enter")
	if m.mode != addingProject {
		t.Fatalf("mode = %v, want the queue step", m.mode)
	}
	m.input = "zeta"
	m, _ = press(m, "enter")
	if m.mode != addingAssignee {
		t.Fatalf("mode = %v, want the assignee step", m.mode)
	}
	m.input = "dev"
	m, cmd := press(m, "enter")
	if cmd == nil {
		t.Fatal("the add flow must write")
	}
	if msg := cmd().(ranMsg); msg.err != nil {
		t.Fatalf("create: %v", msg.err)
	}
	if multi.created[0] != "zeta" || multi.created[1] != "Fix the parser" || multi.created[2] != "dev" {
		t.Fatalf("created in %v, want zeta/Fix the parser/dev", multi.created)
	}

	// One queue, no question: the two-step flow it has always had.
	single := model{src: &recordingSource{}}
	single.mode, single.input = addingTitle, "Fix the parser"
	single, _ = press(single, "enter")
	if single.mode != addingAssignee {
		t.Fatalf("a single-queue fleet must not be asked for a queue: mode = %v", single.mode)
	}
}

// The board routes work and never judges it: no key closes a task with a
// verdict, which is the refusal ADR 0005 names.
func TestNoKeyClosesATask(t *testing.T) {
	src := &recordingSource{}
	m := model{
		dir:   "/fleet",
		src:   src,
		tasks: []work.Task{{ID: "T-1", Open: true, Phase: "To Do", Assignee: "dev"}},
		last:  map[string]*history.Record{},
		agents: map[string]fleet.Agent{
			"dev": {Name: "dev"},
		},
	}
	m.rebuildRows()
	m.clampSel()
	for _, key := range []string{"j", "k", "r", "x", "v", "s", "a", "d", "f", "b", "D", "F", "B", "p", "/", "g", "esc"} {
		var next tea.Model = m
		next, _ = next.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
		m = next.(model)
	}
	if len(src.closed) != 0 {
		t.Fatalf("the board closed %v", src.closed)
	}
}

// press sends keys and returns the model, so a flow can be walked the way a
// person walks it: enter ends a step, esc abandons one.
func press(m model, keys ...string) (model, tea.Cmd) {
	var cmd tea.Cmd
	var next tea.Model = m
	for _, k := range keys {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		if k == "enter" {
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		}
		next, cmd = next.Update(msg)
	}
	return next.(model), cmd
}

func writeAgent(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, "agents", name)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "AGENT.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// reassigningSource is a queue with a routing write and no verdicts at all.
type reassigningSource struct {
	work.Source
	got []string
}

func (r *reassigningSource) Assign(id, agent string) error {
	r.got = []string{id, agent}
	return nil
}

// recordingSource watches for the one write the board must never make.
type recordingSource struct {
	work.Source
	closed []string
}

func (r *recordingSource) Close(id string, v work.Verdict) error {
	r.closed = append(r.closed, id)
	return nil
}

type multiQueueSource struct {
	work.Source
	names   []string
	created []string
}

func (m *multiQueueSource) Names() []string { return m.names }

func (m *multiQueueSource) CreateIn(name, title, body, assignee string) (string, error) {
	m.created = []string{name, title, assignee}
	return name + "/TASK-9", nil
}
