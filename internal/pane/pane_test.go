package pane

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

func TestQueuesOfListsTheQueuesInAStableOrder(t *testing.T) {
	got := queuesOf([]work.Task{
		{ID: "zeta/T-1"}, {ID: "alpha/T-2"}, {ID: "zeta/T-3"},
	})
	want := []string{"alpha", "zeta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got := queuesOf([]work.Task{{ID: "TASK-1"}}); len(got) != 0 {
		t.Fatalf("a prefixless queue has no queue column: %v", got)
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

// The run in flight was launched by one agent and the task may have been
// re-routed since; the row counts against the agent that started it, because
// that is the number the daemon is enforcing.
func TestRunningRowCountsAgainstTheAgentThatStartedIt(t *testing.T) {
	task := work.Task{ID: "T-1", Open: true, Phase: "In Progress", Assignee: "ops"}
	m := model{
		tasks: []work.Task{task},
		last:  map[string]*history.Record{"T-1": {Status: history.StatusRunning, Agent: "dev", At: time.Now()}},
		agents: map[string]fleet.Agent{
			"dev": {Name: "dev", TimeoutMinutes: 45},
			"ops": {Name: "ops", TimeoutMinutes: 600},
		},
	}
	if got := m.timeoutFor(task); got != 45 {
		t.Fatalf("timeoutFor = %d, want the 45 the run was started with", got)
	}

	// A record that names no agent — one written before the fleet recorded it —
	// falls back to where the task routes now.
	rec := m.last["T-1"]
	rec.Agent = ""
	if got := m.timeoutFor(task); got != 600 {
		t.Fatalf("timeoutFor = %d, want the routed agent's 600", got)
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
// agent is prompted with. The body comes from Get, not from the row the board
// drew: a List carries what a board needs, and a detail view drawn from it says
// a task has no description and no notes — and looks right saying it.
func TestDetailViewReadsTheTaskNotTheRow(t *testing.T) {
	row := work.Task{ID: "myapp/TASK-12", Title: "Fix the parser", Open: true, Phase: "To Do", Assignee: "dev"}
	full := row
	full.Body = "the numbers are not moving"
	full.Criteria = []work.Criterion{{Index: 1, Text: "a test fails first"}}
	full.Notes = "tried once"
	src := &readingSource{rows: []work.Task{row}, full: full}
	m := model{dir: "/fleet", src: src, tasks: []work.Task{row}}
	m.rebuildRows()
	m.clampSel()

	m, cmd := press(m, "v")
	if cmd == nil {
		t.Fatal("v must read the task")
	}
	tm, ok := cmd().(detailMsg)
	if !ok {
		t.Fatal("v must ask the source for the task")
	}
	if len(src.gets) != 1 || src.gets[0] != "myapp/TASK-12" {
		t.Fatalf("read %v, want the row's full id", src.gets)
	}
	next, _ := m.Update(tm)
	m = next.(model)

	got := m.detailView()
	for _, want := range strings.Split(strings.TrimRight(text.TaskDetail(full), "\n"), "\n") {
		if !strings.Contains(got, want) {
			t.Fatalf("detail view is missing %q:\n%s", want, got)
		}
	}
}

// readingSource answers the two reads a backend offers: List for the board's
// row, Get for the task itself.
type readingSource struct {
	work.Source
	rows []work.Task
	full work.Task
	gets []string
}

func (r *readingSource) List() ([]work.Task, error) { return r.rows, nil }

func (r *readingSource) Get(id string) (work.Task, error) {
	r.gets = append(r.gets, id)
	return r.full, nil
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
	if m.mode != addingQueue {
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

// A header is one style, not two nested: a string that already carries escape
// codes and is handed to a second Render comes out with those codes drawn as
// text, so the board would read "[1;36mRunning" instead of a coloured heading.
// Colours are forced on here because the bug never shows on a dumb terminal,
// which is exactly why it survives an eyeball test in a plain pipe.
func TestAPhaseHeaderIsStyledInOnePass(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(1) // the ANSI profile: escape codes, no truecolor
	defer lipgloss.SetColorProfile(prev)

	escape := regexp.MustCompile("\x1b\\[[0-9;]*m")
	for _, phase := range []string{runningHeader, work.Done.Label(), work.Blocked.Label(), work.Failed.Label(), "To Do", "A phase of its own"} {
		got := phaseHeader(phase)
		if !escape.MatchString(got) {
			t.Errorf("%s: rendered with no style at all: %q", phase, got)
		}
		if rest := escape.ReplaceAllString(got, ""); strings.Contains(rest, "[") {
			t.Errorf("%s: an escape code is drawn as text: %q", phase, got)
		}
	}
}

func TestWhoCellMarksTheTwoWaysAnAgentCanBeWrong(t *testing.T) {
	dev := map[string]fleet.Agent{"dev": {Name: "dev"}, "paused": {Name: "paused", Disabled: true}}

	got := whoCell(dev, work.Task{Assignee: "nobody"})
	if !strings.Contains(got, "nobody?") {
		t.Errorf("an agent nobody has should be marked: %q", got)
	}
	got = whoCell(dev, work.Task{Assignee: "paused"})
	if !strings.Contains(got, "paused paused") {
		t.Errorf("a disabled agent should be marked paused: %q", got)
	}
	got = whoCell(dev, work.Task{Assignee: "dev"})
	if strings.Contains(got, "paused") || strings.Contains(got, "?") {
		t.Errorf("a plain agent should carry no mark: %q", got)
	}
	got = whoCell(nil, work.Task{})
	if !strings.Contains(got, "-") {
		t.Errorf("an unassigned task should read -: %q", got)
	}
}

// A terminal hands over several characters in one read whenever somebody types
// faster than the reader drains, or pastes. The prompt takes every rune in the
// message: reading only the one-rune ones silently eats most of a title.
func TestARunOfCharactersLandsInThePrompt(t *testing.T) {
	m := model{
		tasks: []work.Task{{ID: "T-1", Title: "Fix login", Open: true, Phase: "To Do"}},
		last:  map[string]*history.Record{},
	}
	m.rebuildRows()
	m.clampSel()

	m, _ = press(m, "/")
	m = types(m, "Fix login")
	if m.input != "Fix login" {
		t.Fatalf("search box holds %q, want %q", m.input, "Fix login")
	}
	if len(m.rows) != 2 { // the header and the one matching task
		t.Fatalf("filter kept %d rows, want the header and the match", len(m.rows))
	}

	m, _ = press(m, "esc", "a")
	m = types(m, "Fix the parser")
	m, _ = press(m, "enter")
	if m.pending != "Fix the parser" {
		t.Fatalf("the add flow kept %q, want %q", m.pending, "Fix the parser")
	}
}

// types sends what a terminal sends for a line of typing: a run of characters
// per word, and each space as its own event.
func types(m model, s string) model {
	for i, word := range strings.Split(s, " ") {
		if i > 0 {
			next, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
			m = next.(model)
		}
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(word)})
		m = next.(model)
	}
	return m
}

// The roster's columns are fixed and can outgrow a narrow terminal, so the run
// column is ordered for the cut: the timer first, the title last. A line that
// loses its end has still said which run is late.
func TestTheRosterPutsTheTimerBeforeTheTitle(t *testing.T) {
	it := work.Task{
		ID: "myapp/TASK-1", Open: true, Phase: "In Progress", Assignee: "dev",
		Title: "A title long enough that no terminal of this width could hold it all",
	}
	m := model{
		dir: "/fleet", width: 100,
		tasks:  []work.Task{it},
		last:   map[string]*history.Record{"myapp/TASK-1": {Status: history.StatusRunning, Agent: "dev", At: time.Now().Add(-12 * time.Minute)}},
		agents: map[string]fleet.Agent{"dev": {Name: "dev", Kind: "claude", TimeoutMinutes: 45}},
	}
	line := strings.Split(m.agentsView(), "\n")[2]
	if got := lipgloss.Width(line); got > m.width {
		t.Fatalf("the roster line is %d wide on a %d-wide terminal: %q", got, m.width, line)
	}
	timer, task := strings.Index(line, "12m / 45m"), strings.Index(line, "TASK-1")
	if timer < 0 {
		t.Fatalf("the roster lost the run's timer: %q", line)
	}
	if task >= 0 && task < timer {
		t.Fatalf("the title comes before the timer, so a narrow terminal cuts the timer: %q", line)
	}
}

// The board's Running row and the roster's busy cell count the same run the
// same way, and credit the agent running it rather than the agent the task
// routes to now: re-routing a task mid-run does not move the run.
func TestTheRosterCreditsTheAgentThatStartedTheRun(t *testing.T) {
	started := time.Now().Add(-12 * time.Minute)
	m := model{
		dir: "/fleet",
		agents: map[string]fleet.Agent{
			"dev": {Name: "dev", Kind: "claude", TimeoutMinutes: 45},
			"ops": {Name: "ops", Kind: "claude", TimeoutMinutes: 600},
		},
		tasks: []work.Task{{ID: "myapp/TASK-12", Title: "Fix the parser", Open: true, Phase: "In Progress", Assignee: "ops"}},
		last:  map[string]*history.Record{"myapp/TASK-12": {Status: history.StatusRunning, Agent: "dev", At: started}},
	}
	lines := strings.Split(m.agentsView(), "\n")
	dev, ops := lines[2], lines[3] // the roster sorts by name
	if !strings.Contains(dev, "busy") || !strings.Contains(dev, "12m / 45m") {
		t.Fatalf("the agent running it is not the one credited: %q", dev)
	}
	if !strings.Contains(ops, "idle") {
		t.Fatalf("the agent the task routes to is not the one running it: %q", ops)
	}
}
