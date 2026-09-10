// Package pane is the plugin's Herdr overlay pane: a live board of the fleet
// backlog grouped by status, with one-key "run now", "jump to the run's
// workspace", and a minimal add-task flow.
package pane

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/DnzzL/herdr-fleet/internal/backlog"
	"github.com/DnzzL/herdr-fleet/internal/fleet"
	"github.com/DnzzL/herdr-fleet/internal/herdr"
	"github.com/DnzzL/herdr-fleet/internal/history"
	"github.com/DnzzL/herdr-fleet/internal/pick"
	"github.com/DnzzL/herdr-fleet/internal/runner"
	"github.com/DnzzL/herdr-fleet/internal/text"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	headerStyle   = lipgloss.NewStyle().Bold(true).Underline(true)
	selectedStyle = lipgloss.NewStyle().Reverse(true)
	dimStyle      = lipgloss.NewStyle().Faint(true)
	failStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	okStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	warnStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
)

// row is one line of the board: a status header, a spacer between groups, or
// a task under a header.
type row struct {
	header string
	spacer bool
	task   backlog.Task
	last   *history.Record
}

func (r row) selectable() bool { return r.header == "" && !r.spacer }

// rows lays the board out: every fleet status in lifecycle order, tasks under
// each, a header per non-empty group and a blank spacer between groups. When
// query is non-empty only tasks whose ID or title contain it (case-insensitive)
// are included. Pure, so the layout is testable without a terminal.
func rows(tasks []backlog.Task, last map[string]*history.Record, query string) []row {
	query = strings.ToLower(query)
	var out []row
	for _, status := range backlog.Statuses {
		var group []row
		for _, t := range tasks {
			if t.Status != status || !matches(t, query) {
				continue
			}
			group = append(group, row{task: t, last: last[t.ID]})
		}
		if len(group) == 0 {
			continue
		}
		if len(out) > 0 {
			out = append(out, row{spacer: true})
		}
		out = append(out, row{header: status})
		out = append(out, group...)
	}
	return out
}

// matches reports whether the task's ID or title contains query, which the
// caller has already lowercased. An empty query matches everything.
func matches(t backlog.Task, query string) bool {
	if query == "" {
		return true
	}
	return strings.Contains(strings.ToLower(t.ID), query) ||
		strings.Contains(strings.ToLower(t.Title), query)
}

func statusStyle(s string) lipgloss.Style {
	switch s {
	case backlog.StatusDone:
		return okStyle
	case backlog.StatusFailed:
		return failStyle
	case backlog.StatusBlocked:
		return warnStyle
	case backlog.StatusInProgress:
		return okStyle
	}
	return lipgloss.NewStyle()
}

type mode int

const (
	browsing mode = iota
	addingTitle
	addingAssignee
	searching
)

type view int

const (
	boardView view = iota
	agentsView
)

type model struct {
	dir          string
	defaultAgent string
	tasks        []backlog.Task
	last         map[string]*history.Record
	rows         []row
	agents       map[string]fleet.Agent
	sel          int
	width        int
	height       int
	status       string
	mode         mode
	view         view
	input        string
	pending      string // the title typed before the assignee is asked
	query        string // active task filter, live-edited while mode == searching
	runs         *runner.Runner
}

type refreshMsg struct {
	tasks  []backlog.Task
	last   map[string]*history.Record
	agents map[string]fleet.Agent
	err    error
}
type tickMsg time.Time
type ranMsg struct{ err error }

// Run starts the interactive board.
func Run() error {
	settings, err := fleet.LoadSettings()
	if err != nil {
		return err
	}
	m := model{dir: settings.Dir, defaultAgent: settings.DefaultAgent, runs: runner.Default(settings.Dir)}
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func (m model) Init() tea.Cmd { return tea.Batch(refresh(m.dir), tick()) }

func tick() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func refresh(dir string) tea.Cmd {
	return func() tea.Msg {
		tasks, err := backlog.New(dir).List()
		if err != nil {
			return refreshMsg{err: err}
		}
		last := map[string]*history.Record{}
		for _, t := range tasks {
			last[t.ID], _ = history.LastRun(t.ID)
		}
		agents, _ := fleet.LoadAgents(dir)
		return refreshMsg{tasks: tasks, last: last, agents: agents}
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
		m.rebuildRows()
		m.clampSel()
	case tickMsg:
		return m, tea.Batch(refresh(m.dir), tick())
	case ranMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
		}
		return m, refresh(m.dir)
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
		if m.view != boardView {
			return m, nil
		}
		switch msg.String() {
		case "j", "down":
			m.move(1)
		case "k", "up":
			m.move(-1)
		case "a":
			m.mode, m.input, m.status = addingTitle, "", ""
		case "r":
			return m.runSelected()
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
	}
	return m, nil
}

func (m model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode, m.input = browsing, ""
	case "enter":
		if m.mode == addingTitle {
			if strings.TrimSpace(m.input) == "" {
				m.mode = browsing
				return m, nil
			}
			m.pending, m.input, m.mode = strings.TrimSpace(m.input), "", addingAssignee
			return m, nil
		}
		title, assignee := m.pending, strings.TrimSpace(m.input)
		m.mode, m.input, m.pending = browsing, "", ""
		dir := m.dir
		return m, func() tea.Msg {
			_, err := backlog.New(dir).Create(title, "", assignee)
			return ranMsg{err: err}
		}
	case "backspace":
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
	default:
		if len(msg.String()) == 1 || msg.String() == "space" {
			s := msg.String()
			if s == "space" {
				s = " "
			}
			m.input += s
		}
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
		if len(msg.String()) == 1 || msg.String() == "space" {
			s := msg.String()
			if s == "space" {
				s = " "
			}
			m.input += s
		}
	}
	m.query = m.input
	m.rebuildRows()
	m.clampSel()
	return m, nil
}

func (m *model) move(d int) {
	for i := m.sel + d; i >= 0 && i < len(m.rows); i += d {
		if m.rows[i].selectable() {
			m.sel = i
			return
		}
	}
}

func (m *model) clampSel() {
	if m.sel >= len(m.rows) {
		m.sel = len(m.rows) - 1
	}
	if m.sel < 0 || (m.sel < len(m.rows) && !m.rows[m.sel].selectable()) {
		m.sel = 0
		m.move(1)
		if m.sel == 0 && len(m.rows) > 1 {
			m.sel = 1
		}
	}
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
	if r.task.Status == backlog.StatusInProgress {
		m.status = r.task.ID + " is already In Progress"
		return m, nil
	}
	name := pick.AssigneeFor(r.task, m.defaultAgent)
	agent, ok := m.agents[name]
	if !ok {
		m.status = fmt.Sprintf("%s: assignee %q is not a fleet agent", r.task.ID, name)
		return m, nil
	}
	task, runs := r.task, m.runs
	m.status = "running " + task.ID
	return m, func() tea.Msg { return ranMsg{err: runs.Run(task, agent, history.TriggerManual)} }
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

func (m model) View() string {
	if m.view == agentsView {
		return m.agentsView()
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

	// Fixed columns (id, who, detail) plus their separating spaces, so the
	// title is the only column that flexes with terminal width. Detail is
	// capped at 32 to fit "last run <longest status> Mon 15:04" (~30 chars).
	const idWidth, whoWidth, detailWidth = 9, 16, 32
	titleWidth := 40
	if m.width > 0 {
		if titleWidth = m.width - (idWidth + whoWidth + detailWidth + 5); titleWidth < 15 {
			titleWidth = 15
		} else if titleWidth > 80 {
			titleWidth = 80
		}
	}
	rowFormat := fmt.Sprintf("  %%-%ds %%-%ds %%-%ds %%s", idWidth, titleWidth, whoWidth)

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
			fmt.Fprintf(&b, "%s\n", headerStyle.Render(statusStyle(r.header).Render(r.header)))
			continue
		}
		who := strings.Join(r.task.Assignees, ",")
		if who == "" {
			who = "-"
		} else if _, ok := m.agents[who]; !ok {
			who = failStyle.Render(text.Truncate(who, whoWidth-1) + "?")
		} else {
			who = text.Truncate(who, whoWidth)
		}
		detail := ""
		if r.last != nil {
			raw := fmt.Sprintf("last run %s %s", r.last.Status, r.last.At.Format("Mon 15:04"))
			detail = dimStyle.Render(text.Truncate(raw, detailWidth))
		}
		line := fmt.Sprintf(rowFormat, text.Truncate(r.task.ID, idWidth), text.Truncate(r.task.Title, titleWidth), who, detail)
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
	case addingAssignee:
		b.WriteString(fmt.Sprintf("assignee for %q (%s): %s▌\n",
			m.pending, strings.Join(agentNames(m.agents), ", "), m.input))
	case searching:
		b.WriteString("search: " + m.input + "▌\n")
	default:
		b.WriteString(dimStyle.Render("j/k move · r run · enter jump to run · a add · / search · g agents · q quit"))
	}
	if m.status != "" {
		b.WriteString("\n" + warnStyle.Render(m.status))
	}
	return b.String()
}

// agentsView renders the read-only fleet roster: name, kind/model, workspace
// mode and whether the daemon will schedule it.
func (m model) agentsView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("fleet agents — " + m.dir))
	b.WriteString("\n\n")
	if len(m.agents) == 0 {
		b.WriteString(dimStyle.Render("  no agents — add one under agents/<name>/AGENT.md\n"))
	}
	names := agentNames(m.agents)
	sort.Strings(names)
	for _, name := range names {
		a := m.agents[name]
		status := okStyle.Render("enabled")
		if a.Disabled {
			status = warnStyle.Render("disabled")
		}
		line := fmt.Sprintf("  %-16s %-10s %-24s %-9s %s", name, a.Kind, text.Truncate(a.Model, 24), a.Workspace, status)
		b.WriteString(line + "\n")
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("g board · q quit"))
	if m.status != "" {
		b.WriteString("\n" + warnStyle.Render(m.status))
	}
	return b.String()
}

func agentNames(agents map[string]fleet.Agent) []string {
	names := make([]string, 0, len(agents))
	for n := range agents {
		names = append(names, n)
	}
	return names
}
