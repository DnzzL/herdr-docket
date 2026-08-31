// Package pane is the plugin's Herdr overlay pane: a live board of the fleet
// backlog grouped by status, with one-key "run now", "jump to the run's
// workspace", and a minimal add-task flow.
package pane

import (
	"fmt"
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

// row is one line of the board: a status header or a task under it.
type row struct {
	header string
	task   backlog.Task
	last   *history.Record
}

// rows lays the board out: every fleet status in lifecycle order, tasks under
// each, headers for the statuses that have any. Pure, so the layout is
// testable without a terminal.
func rows(tasks []backlog.Task, last map[string]*history.Record) []row {
	var out []row
	for _, status := range backlog.Statuses {
		start := len(out)
		for _, t := range tasks {
			if t.Status == status {
				out = append(out, row{task: t, last: last[t.ID]})
			}
		}
		if len(out) > start {
			out = append(out[:start], append([]row{{header: status}}, out[start:]...)...)
		}
	}
	return out
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
)

type model struct {
	dir          string
	defaultAgent string
	rows         []row
	agents       map[string]fleet.Agent
	sel          int
	width        int
	height       int
	status       string
	mode         mode
	input        string
	pending      string // the title typed before the assignee is asked
	runs         *runner.Runner
}

type refreshMsg struct {
	rows   []row
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
		return refreshMsg{rows: rows(tasks, last), agents: agents}
	}
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
		m.rows, m.agents = msg.rows, msg.agents
		m.clampSel()
	case tickMsg:
		return m, tea.Batch(refresh(m.dir), tick())
	case ranMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
		}
		return m, refresh(m.dir)
	case tea.KeyMsg:
		if m.mode != browsing {
			return m.updateInput(msg)
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
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
			return ranMsg{err: backlog.New(dir).Create(title, "", assignee)}
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

func (m *model) move(d int) {
	for i := m.sel + d; i >= 0 && i < len(m.rows); i += d {
		if m.rows[i].header == "" {
			m.sel = i
			return
		}
	}
}

func (m *model) clampSel() {
	if m.sel >= len(m.rows) {
		m.sel = len(m.rows) - 1
	}
	if m.sel < 0 || (m.sel < len(m.rows) && m.rows[m.sel].header != "") {
		m.sel = 0
		m.move(1)
		if m.sel == 0 && len(m.rows) > 1 {
			m.sel = 1
		}
	}
}

func (m model) selected() *row {
	if m.sel >= 0 && m.sel < len(m.rows) && m.rows[m.sel].header == "" {
		return &m.rows[m.sel]
	}
	return nil
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
	var b strings.Builder
	b.WriteString(titleStyle.Render("fleet — " + m.dir))
	b.WriteString("\n\n")
	for i, r := range m.rows {
		if r.header != "" {
			fmt.Fprintf(&b, "%s\n", headerStyle.Render(statusStyle(r.header).Render(r.header)))
			continue
		}
		who := strings.Join(r.task.Assignees, ",")
		if who == "" {
			who = "-"
		} else if _, ok := m.agents[who]; !ok {
			who = failStyle.Render(who + "?")
		}
		detail := ""
		if r.last != nil {
			detail = dimStyle.Render(fmt.Sprintf("last run %s %s", r.last.Status, r.last.At.Format("Mon 15:04")))
		}
		line := fmt.Sprintf("  %-9s %-40s %-16s %s", r.task.ID, text.Truncate(r.task.Title, 40), who, detail)
		if i == m.sel {
			line = selectedStyle.Render(line)
		}
		b.WriteString(line + "\n")
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
	default:
		b.WriteString(dimStyle.Render("j/k move · r run · enter jump to run · a add · q quit"))
	}
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
