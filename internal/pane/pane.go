// Package pane is the plugin's Herdr overlay pane: the fleet's board. It shows
// what the fleet works next and what it is working on now, and it acts on both.
// What it shows and what it refuses are decided in
// docs/adr/0008-the-board-is-a-triage-surface.md: it renders the fleet's Task,
// not the backend's page, and it never closes a task with a verdict.
package pane

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/DnzzL/herdr-docket/internal/fleet"
	"github.com/DnzzL/herdr-docket/internal/herdr"
	"github.com/DnzzL/herdr-docket/internal/history"
	"github.com/DnzzL/herdr-docket/internal/host"
	"github.com/DnzzL/herdr-docket/internal/pick"
	"github.com/DnzzL/herdr-docket/internal/runner"
	"github.com/DnzzL/herdr-docket/internal/text"
	"github.com/DnzzL/herdr-docket/internal/work"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	headerStyle   = lipgloss.NewStyle().Bold(true).Underline(true)
	selectedStyle = lipgloss.NewStyle().Reverse(true)
	dimStyle      = lipgloss.NewStyle().Faint(true)
	failStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	okStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	warnStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	runStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
)

// queueColors are the colours a queue's name is drawn in: enough to tell a
// fleet's queues apart, avoiding the three the board already means — red is
// failed, green is done, yellow is blocked. A queue keeps its colour because
// they are handed out in the sorted order of the names, not the order tasks
// happened to arrive in.
var queueColors = []lipgloss.Color{"4", "5", "6", "12", "13", "14"}

// row is one line of the board: a phase header, a spacer between groups, or
// a task under a header.
type row struct {
	header string
	spacer bool
	task   work.Task
	last   *history.Record
}

func (r row) selectable() bool { return r.header == "" && !r.spacer }

// The board's fixed columns: the id, the agent and the detail line. The title
// is the only column that flexes with the terminal, and the queue column
// exists only when a fleet has more than one queue.
const (
	idWidth     = 9
	whoWidth    = 16
	detailWidth = 32
)

// runningHeader heads the group a task with a run in flight is shown under. It
// is not a phase: nothing writes it, and the task keeps whatever phase its
// backend gave it. It sorts above every phase because what is happening now is
// the first thing a person opens the board to ask.
const runningHeader = "Running"

// inFlight reports whether a task has a run in flight, as its last history
// record tells it. A record no run ever closed — a daemon that died mid-run —
// reads as in flight here too, which is why the row marks it stale rather than
// drawing it as busy.
func inFlight(r *history.Record) bool {
	return r != nil && r.Status == history.StatusRunning
}

// rows lays the board out: the running group first, then every phase the queue
// actually uses, tasks under each, a header per non-empty group and a blank
// spacer between groups. An open phase is ordered the way the scheduler works
// it; a closed one keeps the order its backend gave. When query is non-empty
// only tasks whose id, queue or title contain it (case-insensitive) are
// included. Pure, so the layout is testable without a terminal.
func rows(items []work.Task, last map[string]*history.Record, query string) []row {
	query = strings.ToLower(query)
	var out []row
	var live []row
	for _, it := range items {
		if rec := last[it.ID]; inFlight(rec) && matches(it, query) {
			live = append(live, row{task: it, last: rec})
		}
	}
	sortByUrgency(live)
	if len(live) > 0 {
		out = append(out, row{header: runningHeader})
		out = append(out, live...)
	}
	for _, phase := range work.PhasesOf(items) {
		var group []row
		open := false
		for _, it := range items {
			if it.Phase != phase || !matches(it, query) || inFlight(last[it.ID]) {
				continue
			}
			open = open || it.Open
			group = append(group, row{task: it, last: last[it.ID]})
		}
		if len(group) == 0 {
			continue
		}
		// An open group is drawn in the order the scheduler works it; a closed
		// one keeps the order its backend gave, because urgency is meaningless
		// once a task has ended. Open is the port's own field, so a backend
		// with phases of its own is sorted by what it says about the work and
		// not by whether this file recognises the phase word.
		if open {
			sortByUrgency(group)
		}
		if len(out) > 0 {
			out = append(out, row{spacer: true})
		}
		heading := phase
		if heading == "" {
			heading = "(no phase)"
		}
		out = append(out, row{header: heading})
		out = append(out, group...)
	}
	return out
}

// sortByUrgency puts a group in the order the fleet works it: the scheduler's
// own comparison, so the board and the daemon agree about what is next.
func sortByUrgency(group []row) {
	sort.SliceStable(group, func(i, j int) bool { return pick.Before(group[i].task, group[j].task) })
}

// matches reports whether the task's id, queue or title contains query, which
// the caller has already lowercased. An empty query matches everything.
func matches(it work.Task, query string) bool {
	if query == "" {
		return true
	}
	return strings.Contains(strings.ToLower(it.ID), query) ||
		strings.Contains(strings.ToLower(it.Title), query)
}

// queuesOf lists the queues these tasks come from, in a stable order. Empty
// or one name means there is nothing on a row to distinguish, and the board
// draws no queue column at all.
func queuesOf(items []work.Task) []string {
	seen := map[string]bool{}
	var out []string
	for _, it := range items {
		name := work.SourceOf(it.ID)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// queueStyle colours a queue's name, so two queues stay apart at a glance
// even when the eye skips the text.
func queueStyle(queues []string, name string) lipgloss.Style {
	for i, p := range queues {
		if p == name {
			return lipgloss.NewStyle().Foreground(queueColors[i%len(queueColors)])
		}
	}
	return dimStyle
}

// phaseStyle colours a group heading by the fleet's own words for the common
// phases — the port owns those words, so this file spells none of them itself.
// A backend with phases of its own — or none — renders plain, which is why
// nothing depends on this: the board is readable either way.
func phaseStyle(phase string) lipgloss.Style {
	switch phase {
	case runningHeader:
		return runStyle
	case work.Done.Label(), string(work.PhaseInProgress):
		return okStyle
	case work.Failed.Label():
		return failStyle
	case work.Blocked.Label():
		return warnStyle
	}
	return lipgloss.NewStyle()
}

type mode int

const (
	browsing mode = iota
	addingTitle
	addingQueue
	addingAssignee
	assigningAgent
	searching
)

type view int

const (
	boardView view = iota
	agentsView
	detailView
)

type model struct {
	dir      string
	src      work.Source
	defaults fleet.Defaults
	tasks    []work.Task
	last     map[string]*history.Record
	usage    map[string]history.Usage
	rows     []row
	agents   map[string]fleet.Agent
	sel      int
	selID    string // the task the cursor is on, so a reorder cannot move it
	asel     int    // the agent the roster cursor is on
	width    int
	height   int
	status   string
	mode     mode
	view     view
	input    string
	pending  string // what the flow is about, while a prompt is open
	queue    string // the queue the add flow chose, empty for a single queue
	query    string // active task filter, live-edited while mode == searching
	detail   work.Task
	detailAt int // first line of the detail view on screen
	runs     *runner.Runner
}

type refreshMsg struct {
	tasks  []work.Task
	last   map[string]*history.Record
	usage  map[string]history.Usage
	agents map[string]fleet.Agent
	err    error
}
type tickMsg time.Time
type ranMsg struct{ err error }

// stoppedMsg is what came back from x. The pane can promise nothing about a
// run — the workspace is herdr's and the record is history's — so the answer
// decides what the board says. gone is the whole reason there are two answers:
// a workspace that was already gone means the row's `running` record outlives
// the run, and the row cannot correct itself.
type stoppedMsg struct {
	id   string
	gone bool
	err  error
}

// detailMsg is one task read in full, by id: an answer for the id it was asked
// about, even if the cursor has moved since.
type detailMsg struct {
	id   string
	task work.Task
	err  error
}

// Run starts the interactive board.
func Run() error {
	settings, err := fleet.LoadSettings()
	if err != nil {
		return err
	}
	src, err := fleet.NewSource(settings)
	if err != nil {
		return err
	}
	m := model{
		dir:      settings.Dir,
		src:      src,
		defaults: settings.Defaults(),
		runs:     runner.New(host.New(), settings),
	}
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func (m model) Init() tea.Cmd { return tea.Batch(refresh(m.src, m.dir), tick()) }

func tick() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func refresh(src work.Source, dir string) tea.Cmd {
	return func() tea.Msg {
		items, err := src.List()
		if err != nil {
			return refreshMsg{err: err}
		}
		last := map[string]*history.Record{}
		for _, it := range items {
			last[it.ID], _ = history.LastRun(it.ID)
		}
		agents, _ := fleet.LoadAgents(dir)
		return refreshMsg{tasks: items, last: last, usage: history.UsageSince(time.Now()), agents: agents}
	}
}

// rebuildRows recomputes m.rows from the current tasks and query. Call it
// whenever either changes.
func (m *model) rebuildRows() {
	m.rows = rows(m.tasks, m.last, m.query)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case refreshMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
			return m, nil
		}
		m.tasks, m.last, m.agents = msg.tasks, msg.last, msg.agents
		m.usage = msg.usage
		m.rebuildRows()
		m.clampSel()
	case tickMsg:
		return m, tea.Batch(refresh(m.src, m.dir), tick())
	case ranMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
		}
		return m, refresh(m.src, m.dir)
	case stoppedMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
		} else if msg.gone {
			// Said and left there: nothing was stopped, because there was
			// nothing left to stop. The record that still says running is a
			// run nobody is watching, and saying so is the only correction
			// this surface can make.
			m.status = msg.id + ": its workspace is already gone"
		} else {
			m.status = "closed " + msg.id + "'s workspace"
		}
		return m, refresh(m.src, m.dir)
	case detailMsg:
		// Ignore a read that lands after the reader closed the task or moved
		// on: the answer is about a task the pane is no longer showing.
		if m.view != detailView || m.detail.ID != msg.id {
			return m, nil
		}
		if msg.err != nil {
			m.status = msg.err.Error()
			return m, nil
		}
		m.detail, m.detailAt = msg.task, 0
	case tea.KeyMsg:
		if m.mode == searching {
			return m.updateSearch(msg)
		}
		if m.mode != browsing {
			return m.updateInput(msg)
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "g":
			if m.view == boardView {
				m.view = agentsView
			} else {
				m.view = boardView
			}
		}
		if m.view == agentsView {
			return m.updateAgents(msg)
		}
		if m.view == detailView {
			return m.updateDetail(msg)
		}
		switch msg.String() {
		case "j", "down":
			m.move(1)
		case "k", "up":
			m.move(-1)
		case "a":
			m.mode, m.input, m.status, m.queue = addingTitle, "", "", ""
		case "r":
			return m.runSelected()
		case "x":
			return m, m.stopSelected()
		case "v":
			if r := m.selected(); r != nil {
				m.view, m.detail, m.detailAt = detailView, r.task, 0
				// The row is a list entry: a List carries what a board draws,
				// and the body is a second read. Showing the row's own fields
				// would render a task with no description and no notes —
				// which is exactly what it looks like when the read fails.
				return m, getDetail(m.src, r.task.ID)
			}
		case "s":
			if r := m.selected(); r != nil {
				m.mode, m.input, m.pending, m.status = assigningAgent, "", r.task.ID, ""
			}
		case "enter":
			return m, m.jumpSelected()
		case "/":
			m.mode, m.input = searching, m.query
		case "esc":
			if m.query != "" {
				m.query = ""
				m.rebuildRows()
				m.clampSel()
			}
		}
		// No verdict key, by design: the pane routes work and never judges it.
		// See the refusal list in docs/adr/0008-the-board-is-a-triage-surface.md.
	}
	return m, nil
}

func (m model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode, m.input, m.pending, m.queue = browsing, "", "", ""
	case "enter":
		switch m.mode {
		case addingTitle:
			if strings.TrimSpace(m.input) == "" {
				m.mode = browsing
				return m, nil
			}
			m.pending, m.input = strings.TrimSpace(m.input), ""
			// A composite source cannot invent a queue and refuses to try, so
			// the flow asks before it writes rather than failing after it.
			if len(m.queueNames()) > 1 {
				m.mode = addingQueue
			} else {
				m.mode = addingAssignee
			}
			return m, nil
		case addingQueue:
			m.queue, m.input, m.mode = strings.TrimSpace(m.input), "", addingAssignee
			return m, nil
		case assigningAgent:
			id, agent, src := m.pending, strings.TrimSpace(m.input), m.src
			m.mode, m.input, m.pending = browsing, "", ""
			return m, assign(src, id, agent)
		}
		title, assignee, queue, src := m.pending, strings.TrimSpace(m.input), m.queue, m.src
		m.mode, m.input, m.pending, m.queue = browsing, "", "", ""
		return m, create(src, queue, title, assignee)
	case "backspace":
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
	default:
		m.input += typed(msg)
	}
	return m, nil
}

// updateSearch handles keys while typing a task filter: every edit re-filters
// live via m.query, so the board updates as you type.
func (m model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode, m.input, m.query = browsing, "", ""
	case "enter":
		m.mode = browsing
	case "backspace":
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
	default:
		m.input += typed(msg)
	}
	m.query = m.input
	m.rebuildRows()
	m.clampSel()
	return m, nil
}

// typed is the text a key adds to a prompt. A run of characters arrives as one
// message whenever the terminal hands over more than one byte at a time — a
// paste, or simply typing faster than the reader drains — and the run is split
// at every space, which arrives as its own event. Reading only the one-rune
// message is how a fast typist loses half a task title, and it is invisible
// until it happens to them.
func typed(msg tea.KeyMsg) string {
	switch msg.Type {
	case tea.KeySpace:
		return " "
	case tea.KeyRunes:
		return string(msg.Runes)
	}
	return ""
}

// getDetail reads one task in full. List gives the row and this gives the
// body: a pane that drew the row as the detail would show a task with no
// description and no notes, and would look right doing it.
func getDetail(src work.Source, id string) tea.Cmd {
	return func() tea.Msg {
		it, err := src.Get(id)
		return detailMsg{id: id, task: it, err: err}
	}
}

// create adds a task in the queue the person named, or in the only queue the
// source has. A MultiSource wants a queue name and cannot invent one — its own
// refusal to guess is why the pane asks for it up front.
func create(src work.Source, queue, title, assignee string) tea.Cmd {
	return func() tea.Msg {
		if queue != "" {
			if multi, ok := src.(work.MultiSource); ok {
				_, err := multi.CreateIn(queue, title, "", assignee)
				return ranMsg{err: err}
			}
		}
		_, err := src.Create(title, "", assignee)
		return ranMsg{err: err}
	}
}

// assign re-routes a task and nothing else. Routing is the board's business;
// a verdict is not, so there is no verb here for one.
func assign(src work.Source, id, agent string) tea.Cmd {
	return func() tea.Msg {
		a, ok := src.(work.Assigner)
		if !ok {
			return ranMsg{err: fmt.Errorf("%s: this queue decides routing on its own board, not here", id)}
		}
		return ranMsg{err: a.Assign(id, agent)}
	}
}

// queueNames is the queues the source offers, empty for a backend that has
// only one queue and cannot be asked which.
func (m model) queueNames() []string {
	if multi, ok := m.src.(work.MultiSource); ok {
		return multi.Names()
	}
	return nil
}

func (m model) updateAgents(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	names := m.agentList()
	// The roster is re-sorted on every refresh and agents come and go with it,
	// so the cursor is clamped here rather than trusted.
	if m.asel >= len(names) {
		m.asel = len(names) - 1
	}
	if m.asel < 0 {
		m.asel = 0
	}
	switch msg.String() {
	case "j", "down":
		if m.asel < len(names)-1 {
			m.asel++
		}
	case "k", "up":
		if m.asel > 0 {
			m.asel--
		}
	case "p":
		if len(names) == 0 {
			return m, nil
		}
		return m.toggleAgent(names[m.asel])
	}
	return m, nil
}

// toggleAgent pauses or resumes one agent by editing the disabled line of its
// AGENT.md — the persona below it is left byte for byte as it was. Pausing
// stops the daemon from picking work up; it does not touch a run already in
// flight, which is why the status line says so and points at x.
func (m model) toggleAgent(name string) (tea.Model, tea.Cmd) {
	a := m.agents[name]
	if err := fleet.SetDisabled(m.dir, name, !a.Disabled); err != nil {
		m.status = err.Error()
		return m, nil
	}
	if a.Disabled {
		m.status = name + " enabled — the daemon will schedule it again"
	} else {
		m.status = name + " paused — a run already in flight keeps going; x stops it"
	}
	// Re-read the fleet rather than patching m.agents: the file is the truth,
	// and a reload also proves the edit left an agent the fleet can still use.
	return m, refresh(m.src, m.dir)
}

func (m model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	lines := strings.Count(text.TaskDetail(m.detail), "\n")
	switch msg.String() {
	case "esc", "v":
		m.view = boardView
	case "j", "down":
		if m.detailAt < lines {
			m.detailAt++
		}
	case "k", "up":
		if m.detailAt > 0 {
			m.detailAt--
		}
	}
	return m, nil
}

// agentList is the roster in the order it is drawn: by name, so the cursor's
// position means the same thing between two frames.
func (m model) agentList() []string {
	names := agentNames(m.agents)
	sort.Strings(names)
	return names
}

// busyWith returns the run an agent has in flight, from the same records the
// board's rows read, so the roster and the board cannot disagree about what is
// running. A task routes by its assignee, which is what the daemon picks with.
func (m model) busyWith(name string) (work.Task, *history.Record, bool) {
	for _, it := range m.tasks {
		rec := m.last[it.ID]
		if !inFlight(rec) {
			continue
		}
		// The agent that started the run, not the one the task routes to now:
		// re-routing a task while it runs does not move the run to the new
		// agent, and the roster would credit the wrong one. A record that names
		// no agent — one written before the fleet recorded it — falls back to
		// where the task routes.
		if who := rec.Agent; who != "" {
			if who == name {
				return it, rec, true
			}
			continue
		}
		if pick.AssigneeFor(it, m.defaults) == name {
			return it, rec, true
		}
	}
	return work.Task{}, nil, false
}

func (m *model) move(d int) {
	for i := m.sel + d; i >= 0 && i < len(m.rows); i += d {
		if m.rows[i].selectable() {
			m.sel, m.selID = i, m.rows[i].task.ID
			return
		}
	}
}

// clampSel keeps the cursor on the task it was on. Selection is by task id
// because the list reorders whenever a run starts or ends: a cursor held by
// position would land the next keypress — run, stop, re-route — on a different
// task. A task that has left the list takes the cursor to the nearest
// surviving row, never back to the top.
func (m *model) clampSel() {
	if m.selID != "" {
		for i, r := range m.rows {
			if r.selectable() && r.task.ID == m.selID {
				m.sel = i
				return
			}
		}
	}
	if m.sel >= len(m.rows) {
		m.sel = len(m.rows) - 1
	}
	if m.sel < 0 {
		m.sel = 0
	}
	m.nearest()
}

// nearest walks out from the cursor to the closest selectable row either way.
func (m *model) nearest() {
	if m.sel < len(m.rows) && m.rows[m.sel].selectable() {
		m.selID = m.rows[m.sel].task.ID
		return
	}
	for d := 1; d < len(m.rows); d++ {
		if i := m.sel + d; i < len(m.rows) && m.rows[i].selectable() {
			m.sel, m.selID = i, m.rows[i].task.ID
			return
		}
		if i := m.sel - d; i >= 0 && m.rows[i].selectable() {
			m.sel, m.selID = i, m.rows[i].task.ID
			return
		}
	}
	m.sel, m.selID = 0, ""
}

func (m model) selected() *row {
	if m.sel >= 0 && m.sel < len(m.rows) && m.rows[m.sel].selectable() {
		return &m.rows[m.sel]
	}
	return nil
}

// availableRows is how many list lines fit between the title and the footer.
// 0 means the terminal size isn't known yet (no WindowSizeMsg received) —
// callers should render everything unclipped in that case.
func (m model) availableRows() int {
	if m.height == 0 {
		return 0
	}
	footer := 1 // hint or input line
	if m.status != "" {
		footer++
	}
	avail := m.height - 2 /*title+blank*/ - 1 /*blank before footer*/ - footer
	if avail < 1 {
		avail = 1
	}
	return avail
}

func (m model) runSelected() (tea.Model, tea.Cmd) {
	r := m.selected()
	if r == nil {
		return m, nil
	}
	if !r.task.Open {
		m.status = r.task.ID + " is already closed"
		return m, nil
	}
	name := pick.AssigneeFor(r.task, m.defaults)
	agent, ok := m.agents[name]
	if !ok {
		m.status = fmt.Sprintf("%s: assignee %q is not a fleet agent", r.task.ID, name)
		return m, nil
	}
	task, runs, src := r.task, m.runs, m.src
	// A manual run beats a pause: the pause is a rule for the scheduler, and a
	// person pressing r is not the scheduler. Saying so keeps a run that starts
	// on a paused agent from looking like a bug later.
	if agent.Disabled {
		m.status = fmt.Sprintf("%s: agent %s is paused — running it anyway", task.ID, name)
	} else {
		m.status = "running " + task.ID
	}
	return m, func() tea.Msg { return ranMsg{err: runs.Run(src, task, agent, history.TriggerManual)} }
}

// runningDetail is the detail column of a run in flight: how long it has been
// going against the timeout its agent was given.
func (m model) runningDetail(r row) string { return runDetail(r.last, m.timeoutFor(r.task)) }

// timeoutFor is the timeout the run in flight is held to: the timeout of the
// agent that started it, which is not necessarily the agent the task routes to
// now. A task reassigned mid-run keeps the run it has — the daemon enforces the
// number it launched with — so the board reads the record's agent first and the
// routing rule only for a record that does not name one.
func (m model) timeoutFor(it work.Task) int {
	name := pick.AssigneeFor(it, m.defaults)
	if rec := m.last[it.ID]; rec != nil && rec.Agent != "" {
		name = rec.Agent
	}
	if a, ok := m.agents[name]; ok && a.TimeoutMinutes > 0 {
		return a.TimeoutMinutes
	}
	return fleet.DefaultTimeoutMinutes
}

// runDetail is a run in flight as one cell: elapsed against timeout. Past the
// timeout it is marked stale rather than drawn as busy: nothing beats a
// heartbeat, so a daemon that died under a run leaves a record that never
// closes, and a control surface showing that as live work is telling the one
// lie it cannot afford.
func runDetail(rec *history.Record, timeout int) string {
	elapsed := time.Since(rec.At)
	raw := fmt.Sprintf("%s / %dm", duration(elapsed), timeout)
	if elapsed > time.Duration(timeout)*time.Minute {
		return warnStyle.Render(text.Truncate("stale "+raw, detailWidth))
	}
	return dimStyle.Render(text.Truncate(raw, detailWidth))
}

// duration is elapsed time the way a one-line column reads it: "<1m" under a
// minute, minutes under an hour, then hours and minutes.
func duration(d time.Duration) string {
	if d < time.Minute {
		return "<1m"
	}
	mins := int(d.Minutes())
	if mins < 60 {
		return fmt.Sprintf("%dm", mins)
	}
	return fmt.Sprintf("%dh%02dm", mins/60, mins%60)
}

func (m *model) jumpSelected() tea.Cmd {
	r := m.selected()
	if r == nil || r.last == nil || r.last.WorkspaceID == "" {
		m.status = "no run to jump to"
		return nil
	}
	last := *r.last
	return func() tea.Msg {
		var c herdr.Client
		if err := c.Focus(last.WorkspaceID, last.PaneID); err != nil {
			return refreshMsg{err: err}
		}
		return nil
	}
}

// stopSelected stops the run on the cursor by closing its workspace. The
// runner already reads a closed workspace as cancellation and ends the task
// Blocked, so the pane makes the call and says so; there is no confirmation
// because the key is the confirmation and a run left going is a workspace left
// open. Nothing here writes a verdict: the cancellation path does that.
func (m *model) stopSelected() tea.Cmd {
	r := m.selected()
	if r == nil {
		return nil
	}
	if !inFlight(r.last) || r.last.WorkspaceID == "" {
		m.status = "no run to stop"
		return nil
	}
	id, workspace := r.task.ID, r.last.WorkspaceID
	m.status = "stopping " + id + "…"
	return func() tea.Msg {
		var c herdr.Client
		err := c.WorkspaceClose(workspace)
		if errors.Is(err, herdr.ErrGone) {
			return stoppedMsg{id: id, gone: true}
		}
		return stoppedMsg{id: id, err: err}
	}
}

func (m model) View() string {
	switch m.view {
	case agentsView:
		return m.agentsView()
	case detailView:
		return m.detailView()
	}
	return m.boardView()
}

func (m model) boardView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("fleet — " + m.dir))
	if m.query != "" && m.mode != searching {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  (filter: %q)", m.query)))
	}
	b.WriteString("\n\n")

	// Fixed columns (id, queue, who, detail) plus their separating spaces, so
	// the title is the only column that flexes with terminal width. Detail is
	// capped to fit "last run <longest status> Mon 15:04" (~30 chars).
	queues := queuesOf(m.tasks)
	queueWidth := 0
	if len(queues) > 1 {
		queueWidth = 6
		for _, p := range queues {
			if n := len([]rune(p)); n > queueWidth {
				queueWidth = n
			}
		}
		if queueWidth > 12 {
			queueWidth = 12
		}
	}
	fixed := idWidth + whoWidth + detailWidth + 5
	if queueWidth > 0 {
		fixed += queueWidth + 1
	}
	titleWidth := 40
	if m.width > 0 {
		if titleWidth = m.width - fixed; titleWidth < 15 {
			titleWidth = 15
		} else if titleWidth > 80 {
			titleWidth = 80
		}
	}
	rowFormat := fmt.Sprintf("  %%-%ds %%-%ds %%s %%s", idWidth, titleWidth)
	if queueWidth > 0 {
		rowFormat = fmt.Sprintf("  %%-%ds %%s %%-%ds %%s %%s", idWidth, titleWidth)
	}

	start, end := 0, len(m.rows)
	if avail := m.availableRows(); avail > 0 && len(m.rows) > avail {
		content := avail - 2 // reserve room for the "more above/below" lines
		if content < 1 {
			content = 1
		}
		start = m.sel - content/2
		if start < 0 {
			start = 0
		}
		end = start + content
		if end > len(m.rows) {
			end = len(m.rows)
			start = end - content
			if start < 0 {
				start = 0
			}
		}
	}
	if start > 0 {
		fmt.Fprintf(&b, "%s\n", dimStyle.Render(fmt.Sprintf("  ↑ %d more above", start)))
	}
	for i := start; i < end; i++ {
		r := m.rows[i]
		if r.spacer {
			b.WriteString("\n")
			continue
		}
		if r.header != "" {
			fmt.Fprintf(&b, "%s\n", phaseHeader(r.header))
			continue
		}
		// A styled cell is padded by its own style, not by the format verb: %-16s
		// counts escape bytes and would stagger every row with a colour on it.
		who := whoCell(m.agents, r.task)
		detail := ""
		switch {
		case inFlight(r.last):
			detail = m.runningDetail(r)
		case r.last != nil:
			raw := fmt.Sprintf("last run %s %s", r.last.Status, r.last.At.Format("Mon 15:04"))
			detail = dimStyle.Render(text.Truncate(raw, detailWidth))
		}
		fields := []any{text.Truncate(work.LocalOf(r.task.ID), idWidth)}
		if queueWidth > 0 {
			name := work.SourceOf(r.task.ID)
			fields = append(fields, queueStyle(queues, name).Width(queueWidth).Render(text.Truncate(name, queueWidth)))
		}
		fields = append(fields, text.Truncate(r.task.Title, titleWidth), who, detail)
		line := fmt.Sprintf(rowFormat, fields...)
		if i == m.sel {
			line = selectedStyle.Render(line)
		}
		b.WriteString(line + "\n")
	}
	if end < len(m.rows) {
		fmt.Fprintf(&b, "%s\n", dimStyle.Render(fmt.Sprintf("  ↓ %d more below", len(m.rows)-end)))
	}
	if len(m.rows) == 0 {
		b.WriteString(dimStyle.Render("  no tasks — press a to add one\n"))
	}
	b.WriteString("\n")
	switch m.mode {
	case addingTitle:
		b.WriteString("new task title: " + m.input + "▌\n")
	case addingQueue:
		b.WriteString(fmt.Sprintf("queue for %q (%s): %s▌\n",
			m.pending, strings.Join(m.queueNames(), ", "), m.input))
	case addingAssignee:
		b.WriteString(fmt.Sprintf("assignee for %q (%s): %s▌\n",
			m.pending, strings.Join(m.agentList(), ", "), m.input))
	case assigningAgent:
		b.WriteString(fmt.Sprintf("assign %s to (%s): %s▌\n",
			m.pending, strings.Join(m.agentList(), ", "), m.input))
	case searching:
		b.WriteString("search: " + m.input + "▌\n")
	default:
		b.WriteString(dimStyle.Render("j/k move · r run · x stop · enter jump · v read · s assign · a add · / search · g agents · q quit"))
	}
	if m.status != "" {
		b.WriteString("\n" + warnStyle.Render(m.status))
	}
	return b.String()
}

// detailView shows one task read-only, through the same renderer the CLI's
// task view prints: the fleet's Task — what the prompt is built from — not the
// backend's page, which would offer links, markdown and a comment thread the
// pane deliberately has no way to use.
func (m model) detailView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("fleet — " + m.dir))
	b.WriteString("\n\n")
	lines := strings.Split(strings.TrimRight(text.TaskDetail(m.detail), "\n"), "\n")
	start, end := 0, len(lines)
	if avail := m.availableRows(); avail > 0 && len(lines) > avail {
		start = m.detailAt
		if start > len(lines)-1 {
			start = len(lines) - 1
		}
		end = start + avail
		if end > len(lines) {
			end = len(lines)
		}
	}
	for _, line := range lines[start:end] {
		b.WriteString(line + "\n")
	}
	if end < len(lines) {
		fmt.Fprintf(&b, "%s\n", dimStyle.Render(fmt.Sprintf("  ↓ %d more below", len(lines)-end)))
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("j/k scroll · esc board · g agents · q quit"))
	if m.status != "" {
		b.WriteString("\n" + warnStyle.Render(m.status))
	}
	return b.String()
}

// agentsView renders the fleet roster: name, kind/model, workspace mode,
// whether the daemon will schedule it, and what it is doing right now. p is a
// decision rather than a blind toggle only because the last column is here.
func (m model) agentsView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("fleet agents — " + m.dir))
	b.WriteString("\n\n")
	if len(m.agents) == 0 {
		b.WriteString(dimStyle.Render("  no agents — add one under agents/<name>/AGENT.md\n"))
	}
	for i, name := range m.agentList() {
		a := m.agents[name]
		// disabled is the word the persona file uses; the status line is where
		// p explains what pausing did and did not do.
		status := okStyle.Render("enabled")
		if a.Disabled {
			status = warnStyle.Render("disabled")
		}
		line := fmt.Sprintf("  %-16s %-10s %-24s %-9s %s", name, a.Kind, text.Truncate(a.Model, 24), a.Workspace, status)
		if u := m.usage[name]; pick.BudgetLine(a, u.Runs, u.Minutes) != "" {
			line += "  " + dimStyle.Render(pick.BudgetLine(a, u.Runs, u.Minutes))
		}
		// The run goes last and takes what is left of the terminal after the
		// columns above it, the way the board sizes its title. Before the
		// terminal has reported a size, guess wide rather than draw a column of
		// ellipses.
		room := 60
		if m.width > 0 {
			room = m.width - lipgloss.Width(line) - 2
		}
		line += "  " + m.busyCell(name, room)
		if i == m.asel {
			line = selectedStyle.Render(line)
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("j/k move · p pause/resume · g board · q quit"))
	if m.status != "" {
		b.WriteString("\n" + warnStyle.Render(m.status))
	}
	return b.String()
}

// phaseHeader is a group heading: the phase's colour, the header's bold and
// underline, and one Render. Two nested Renders draw the inner escape codes as
// text instead of colouring the word, which is how a board reads
// "[1;36mRunning" — and it never shows on a plain pipe, so nothing catches it
// but a real terminal.
func phaseHeader(phase string) string {
	return phaseStyle(phase).Inherit(headerStyle).Render(phase)
}

// whoCell is the agent column: the name the task is routed to, coloured when
// the name says something the board should not have to be asked about. A red ?
// is work routed to a name nobody has — it will never be picked up. A yellow
// paused is an agent the daemon is skipping: the row is not next, whatever its
// urgency, until somebody resumes it (r still runs it by hand). An unassigned
// task is dim, because a default agent the fleet configured is the scheduler's
// business and not a claim on the row.
func whoCell(agents map[string]fleet.Agent, it work.Task) string {
	name := it.Assignee
	if name == "" {
		return dimStyle.Width(whoWidth).Render("-")
	}
	style := lipgloss.NewStyle().Width(whoWidth)
	switch a, ok := agents[name]; {
	case !ok:
		style, name = failStyle.Width(whoWidth), text.Truncate(name, whoWidth-1)+"?"
	case a.Disabled:
		mark := " paused"
		style, name = warnStyle.Width(whoWidth), text.Truncate(name, whoWidth-len(mark))+mark
	default:
		name = text.Truncate(name, whoWidth)
	}
	return style.Render(name)
}

// busyCell is the roster's last column: the task an agent has in flight and
// how long it has been on it, or idle. It reads the same history records the
// board's Running group does.
func (m model) busyCell(name string, room int) string {
	it, rec, busy := m.busyWith(name)
	if !busy {
		return dimStyle.Render("idle")
	}
	// The timer comes before the task: a terminal too narrow for both cuts the
	// title, and the number that says the run is late is the one that stays. It
	// is timeoutFor, the same number the board's Running row counts against, so
	// the two views cannot disagree about a run.
	head := runStyle.Render("busy") + " " + runDetail(rec, m.timeoutFor(it))
	budget := room - lipgloss.Width(head) - 1
	if budget < 8 { // less than an id and a space is not a task column
		return head
	}
	parts := []string{work.LocalOf(it.ID)}
	if src := work.SourceOf(it.ID); src != "" {
		parts = append(parts, src)
	}
	parts = append(parts, it.Title)
	return head + " " + text.Truncate(strings.Join(parts, " "), budget)
}

func agentNames(agents map[string]fleet.Agent) []string {
	names := make([]string, 0, len(agents))
	for n := range agents {
		names = append(names, n)
	}
	return names
}
