package main

import (
	"fmt"
	"io"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/DnzzL/herdr-docket/internal/daemon"
	"github.com/DnzzL/herdr-docket/internal/fleet"
	"github.com/DnzzL/herdr-docket/internal/grill"
	"github.com/DnzzL/herdr-docket/internal/history"
	"github.com/DnzzL/herdr-docket/internal/host"
	"github.com/DnzzL/herdr-docket/internal/runner"
	"github.com/DnzzL/herdr-docket/internal/text"
	"github.com/DnzzL/herdr-docket/internal/work"
)

// sessionCloser is the one thing stopping a run needs from the host: closing
// what the run owns. The runner already reads a closed workspace (or tab) as
// a cancellation and ends the task Blocked, so a stop writes no verdict.
type sessionCloser interface {
	Close(host.Session) error
}

// runsCmd lists the runs in flight, one per line, under a pause banner when
// the fleet is paused.
func runsCmd(_ []string, out io.Writer) error {
	live, err := history.InFlight()
	if err != nil {
		return err
	}
	if runner.Paused() {
		fmt.Fprintln(out, "fleet paused — no new runs; `herdr-docket resume` to start again")
	}
	if len(live) == 0 {
		fmt.Fprintln(out, "no run in flight")
		return nil
	}
	timeouts := agentTimeouts()
	now := time.Now()
	for _, r := range live {
		fmt.Fprintln(out, formatRun(r, timeoutFor(timeouts, r.Agent), now))
	}
	return nil
}

// agentTimeouts reads each agent's timeout_minutes. Best-effort: a fleet dir
// that does not load still lists runs, against the default timeout.
func agentTimeouts() map[string]int {
	out := map[string]int{}
	settings, err := fleet.LoadSettings()
	if err != nil {
		return out
	}
	agents, _ := fleet.LoadAgents(settings.Dir)
	for name, a := range agents {
		out[name] = a.TimeoutMinutes
	}
	return out
}

func timeoutFor(timeouts map[string]int, agent string) int {
	if m := timeouts[agent]; m > 0 {
		return m
	}
	return fleet.DefaultTimeoutMinutes
}

// staleGrace is the slack on a deadline before its record counts as stale:
// the runner writes timed_out at the deadline, give or take a poll.
const staleGrace = 2 * time.Minute

// stale reports a record no live run can still be behind. A running record
// past its timeout should have become timed_out; a timed_out one is listened
// for twice the timeout more (ADR 0014), then closed. Past that, the daemon
// died under the run, and the workspace id on the record may since name
// somebody else's workspace — so nothing acts on it.
func stale(r history.Record, timeoutMinutes int, now time.Time) bool {
	limit := time.Duration(timeoutMinutes) * time.Minute
	if r.Status == history.StatusTimedOut {
		limit *= 2
	}
	return now.Sub(r.At) > limit+staleGrace
}

// formatRun is one run in flight: task, agent, stage, elapsed against the
// timeout, and the workspace it lives in. A running record past its timeout
// is marked stale: the runner writes timed_out at the deadline, so a running
// record older than that is one a dead daemon never closed (ADR 0008).
func formatRun(r history.Record, timeoutMinutes int, now time.Time) string {
	trigger := string(r.Trigger)
	if trigger == "" {
		trigger = "-"
	}
	return fmt.Sprintf("%-10s %-12s %-8s %-18s %s", r.Task, r.Agent, trigger, runAge(r, timeoutMinutes, now), r.WorkspaceID)
}

// runAge is the age of one run in flight against its timeout, with the stale
// or late qualifier the pane draws in attention.
func runAge(r history.Record, timeoutMinutes int, now time.Time) string {
	age := fmt.Sprintf("%s / %dm", shortDuration(now.Sub(r.At)), timeoutMinutes)
	switch {
	case stale(r, timeoutMinutes, now):
		age += " stale?"
	case r.Status == history.StatusTimedOut:
		age += " late"
	}
	return age
}

func shortDuration(d time.Duration) string {
	if d < time.Minute {
		return "<1m"
	}
	mins := int(d.Minutes())
	if mins < 60 {
		return fmt.Sprintf("%dm", mins)
	}
	return fmt.Sprintf("%dh%02dm", mins/60, mins%60)
}

// stopRun stops the task's run in flight by closing what its record says it
// owns: a borrowed tab, else its workspace.
func stopRun(c sessionCloser, task string, out io.Writer) error {
	live, err := history.InFlight()
	if err != nil {
		return err
	}
	timeouts := agentTimeouts()
	for _, r := range live {
		if r.Task == task {
			return stopRecord(c, r, timeoutFor(timeouts, r.Agent), out)
		}
	}
	return fmt.Errorf("%s has no run in flight", task)
}

func stopRecord(c sessionCloser, r history.Record, timeoutMinutes int, out io.Writer) error {
	if stale(r, timeoutMinutes, time.Now()) {
		return fmt.Errorf("%s: the record is stale (%s old, past its timeout) — no live run is behind it, and %s may now be another workspace; close it by hand if it is the run's", r.Task, shortDuration(time.Since(r.At)), r.WorkspaceID)
	}
	if r.WorkspaceID == "" && r.TabID == "" {
		return fmt.Errorf("%s: the run's record names no workspace to close", r.Task)
	}
	if err := c.Close(host.Session{WorkspaceID: r.WorkspaceID, PaneID: r.PaneID, TabID: r.TabID}); err != nil {
		return fmt.Errorf("%s: %w", r.Task, err)
	}
	fmt.Fprintf(out, "stopped %s — the run ends cancelled and the task Blocked\n", r.Task)
	return nil
}

// pauseCmd pauses the fleet; --now also stops every run in flight. Every
// stop is attempted even when one fails, so one dead workspace never leaves
// the rest spending.
func pauseCmd(c sessionCloser, args []string, out io.Writer) error {
	now := false
	for _, a := range args {
		if a != "--now" {
			return fmt.Errorf("usage: herdr-docket pause [--now]")
		}
		now = true
	}
	if err := runner.SetPaused(true); err != nil {
		return err
	}
	fmt.Fprintln(out, "fleet paused — the daemon starts no run and no pipeline stage until `herdr-docket resume`")
	fmt.Fprintln(out, "schedules (herdr-automations) are not paused: disable them there")
	if !now {
		fmt.Fprintln(out, "runs in flight finish; `pause --now` stops them")
		return nil
	}
	live, err := history.InFlight()
	if err != nil {
		return err
	}
	timeouts := agentTimeouts()
	var failed []string
	for _, r := range live {
		timeout := timeoutFor(timeouts, r.Agent)
		if stale(r, timeout, time.Now()) {
			fmt.Fprintf(out, "left %s: its record is stale, no live run is behind it\n", r.Task)
			continue
		}
		if err := stopRecord(c, r, timeout, out); err != nil {
			failed = append(failed, err.Error())
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("could not stop: %s", strings.Join(failed, "; "))
	}
	return nil
}

func resumeCmd(out io.Writer) error {
	if err := runner.SetPaused(false); err != nil {
		return err
	}
	fmt.Fprintln(out, "fleet resumed — the daemon notices within one tick (~15s)")
	return nil
}

// paneCmd is the overlay: the Home screen, a bubbletea app (ADR 0024) in the
// alt screen with mouse cell motion, so a click or a Termux tap reaches it.
func paneCmd() error {
	return runPane(newPane(listQueue, func() (work.Source, error) {
		_, src, err := settingsAndSource()
		return src, err
	}))
}

func newPane(list func() ([]work.Task, error), source func() (work.Source, error)) paneModel {
	return paneModel{reader: &paneReader{list: list}, closer: host.New(), source: source}
}

func runPane(m paneModel) error {
	_, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	return err
}

// paneEvery is how often the pane redraws; the queue is re-listed on its own,
// slower clock (queueEvery).
const paneEvery = 2 * time.Second

// wheelStep is how many lines one wheel notch scrolls, Home and Round alike.
const wheelStep = 2

// paneTick carries the frame's clock, so a test drives time by sending one.
type paneTick time.Time

// paneDataMsg is a read of Home's data, done off the Update; relisted says
// the queue was listed afresh.
type paneDataMsg struct {
	data     paneData
	relisted bool
}

// cardMsg is a card fetched for its Round, done off the Update.
type cardMsg struct {
	id   string
	card work.Task
	src  work.Source
	err  error
}

// statusMsg is the line Home shows once a command finished.
type statusMsg string

// openRoundMsg asks for the Round screen of the question card ID: a tap on
// the selected `?` row (the highlighted first row counts), or Enter on it.
type openRoundMsg struct{ ID string }

// paneModel is Home: the daemon line, Your turn, the runs in flight and the
// last settled ones. Everything it does is stopRecord, SetPaused and
// openRoundMsg; it only draws, scrolls and forwards keys and clicks. Clicks
// map to rows by hand from renderPane's row list: no zone library.
// Keys: j/k or 1-9 select, Enter opens, x stops the selected run, p pauses or
// resumes the fleet, q quits.
type paneModel struct {
	reader        *paneReader
	closer        sessionCloser
	data          paneData
	width, height int
	sel           int
	top           int
	status        string

	// source opens the queue a Round reads its card from and answers on.
	source func() (work.Source, error)
	// open is the card whose Round shows first: `questions` with one card.
	open string
	// round is the Round screen while one is open; it takes the keys and
	// clicks, Home keeps its clock.
	round *roundModel

	// relist makes the next read list the queue whatever its age, and hidden
	// keeps the cards an answer just moved off Your turn until it does.
	relist bool
	hidden map[string]bool

	// lines and rows are Home as last laid out: what View draws and where a
	// tap lands, from the same pass.
	lines []string
	rows  []paneRow
}

func (m paneModel) Init() tea.Cmd {
	tick := func() tea.Msg { return paneTick(time.Now()) }
	if m.open != "" {
		id := m.open
		return tea.Batch(tick, func() tea.Msg { return openRoundMsg{ID: id} })
	}
	return tick
}

// Update handles one message and lays Home out once, after it.
func (m paneModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case paneTick:
		// The next tick waits for this read, so reads never overlap.
		if m.reader == nil {
			return m, nil
		}
		r, now, relist := m.reader, time.Time(msg), m.relist
		m.relist = false
		return m, func() tea.Msg { return paneDataMsg{data: r.read(now, relist), relisted: relist} }
	case paneDataMsg:
		m.data = msg.data
		if msg.relisted {
			m.hidden = nil
		}
		return m.laidOut(false), tea.Tick(paneEvery, func(t time.Time) tea.Msg { return paneTick(t) })
	case statusMsg:
		m.status = string(msg)
		return m.laidOut(false), nil
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.round != nil {
			r, _ := m.round.Update(msg)
			m.round = &r
		}
		return m.laidOut(false), nil
	case openRoundMsg:
		m.status = "opening " + msg.ID + "…"
		return m.laidOut(false), fetchCard(m.source, msg.ID)
	case cardMsg:
		return m.openRound(msg).laidOut(false), nil
	case roundDoneMsg:
		m.round, m.status = nil, msg.status
		if msg.answered != "" {
			// Optimistic: the card leaves Your turn now; the next read,
			// a fresh listing, confirms.
			hidden := map[string]bool{msg.answered: true}
			for id := range m.hidden {
				hidden[id] = true
			}
			m.hidden, m.relist = hidden, true
		}
		return m.laidOut(false), nil
	}
	if m.round != nil {
		r, cmd := m.round.Update(msg)
		m.round = &r
		return m, cmd
	}
	switch msg := msg.(type) {
	case tea.MouseMsg:
		m, cmd := m.mouse(msg)
		return m.laidOut(false), cmd
	case tea.KeyMsg:
		m, cmd := m.key(msg)
		return m.laidOut(true), cmd
	}
	return m, nil
}

func (m paneModel) mouse(msg tea.MouseMsg) (paneModel, tea.Cmd) {
	if msg.Action != tea.MouseActionPress {
		return m, nil
	}
	switch msg.Button {
	case tea.MouseButtonWheelDown:
		m.top += wheelStep
	case tea.MouseButtonWheelUp:
		m.top -= wheelStep
	case tea.MouseButtonLeft:
		for i, r := range m.rows {
			if r.Line != m.top+msg.Y {
				continue
			}
			if i == m.sel {
				return m.act()
			}
			m.sel = i
		}
	}
	return m, nil
}

func (m paneModel) key(msg tea.KeyMsg) (paneModel, tea.Cmd) {
	rows := m.rows
	runs := rowsFrom(rows, runRow)
	k := msg.String()
	switch {
	case k == "q" || k == "ctrl+c":
		return m, tea.Quit
	case k == "j" || k == "down":
		m.sel++
	case k == "k" || k == "up":
		m.sel--
	case k == "enter":
		return m.act()
	case len(k) == 1 && k[0] >= '1' && k[0] <= '9' && runs+int(k[0]-'1') < len(rows):
		m.sel = runs + int(k[0]-'1')
	case k == "x" && m.sel < len(rows) && rows[m.sel].Run != nil:
		r, c, timeout := *rows[m.sel].Run, m.closer, timeoutFor(m.data.Timeouts, rows[m.sel].Run.Agent)
		m.status = "stopping " + r.Task + "…"
		return m, func() tea.Msg {
			var b strings.Builder
			if err := stopRecord(c, r, timeout, &b); err != nil {
				return statusMsg(err.Error())
			}
			return statusMsg(strings.TrimSpace(b.String()))
		}
	case k == "p":
		paused := !runner.Paused()
		if err := runner.SetPaused(paused); err != nil {
			m.status = err.Error()
		} else if paused {
			m.status = "paused — runs in flight finish"
		} else {
			m.status = "resumed"
		}
	}
	return m, nil
}

// act is Enter, or a tap, on the selected row: a question opens its Round. A
// run has no such action; stopping one stays on x, never on a tap.
func (m paneModel) act() (paneModel, tea.Cmd) {
	if m.sel < len(m.rows) && m.rows[m.sel].Kind == questionRow {
		id := m.rows[m.sel].ID
		return m, func() tea.Msg { return openRoundMsg{ID: id} }
	}
	return m, nil
}

// rowsFrom is the index of the first row of kind, len(rows) when none.
func rowsFrom(rows []paneRow, kind rowKind) int {
	for i, r := range rows {
		if r.Kind == kind {
			return i
		}
	}
	return len(rows)
}

// laidOut lays Home out once for the model's state: the selection kept on a
// row and marked, the scroll inside the frame (first moved to show the
// selected row when follow).
func (m paneModel) laidOut(follow bool) paneModel {
	d := m.data
	d.Status = m.status
	if len(m.hidden) > 0 {
		d.Queue = nil
		for _, t := range m.data.Queue {
			if !m.hidden[t.ID] {
				d.Queue = append(d.Queue, t)
			}
		}
	}
	w := m.width
	if w <= 0 {
		w = 80
	}
	lines, rows := renderPane(d, w)
	m.sel = max(min(m.sel, len(rows)-1), 0)
	if m.sel < len(rows) {
		l := rows[m.sel].Line
		lines[l] = paneStyle.accent("> ") + strings.TrimPrefix(lines[l], "  ")
		if follow && m.height > 0 {
			if l < m.top {
				m.top = l
			} else if l >= m.top+m.height {
				m.top = l - m.height + 1
			}
		}
	}
	m.top = max(min(m.top, len(lines)-m.height), 0)
	m.lines, m.rows = lines, rows
	return m
}

// fetchCard reads card id off the Update: the queue may be a GitHub board.
func fetchCard(source func() (work.Source, error), id string) tea.Cmd {
	return func() tea.Msg {
		if source == nil {
			return cardMsg{id: id, err: fmt.Errorf("%s: no queue to read the card from", id)}
		}
		src, err := source()
		if err != nil {
			return cardMsg{id: id, err: err}
		}
		t, err := src.Get(id)
		return cardMsg{id: id, card: t, src: src, err: err}
	}
}

// openRound opens the fetched card's newest Round; a card that could not be
// read, or holds no round, stays on Home with the reason.
func (m paneModel) openRound(c cardMsg) paneModel {
	if c.err != nil {
		m.status = c.err.Error()
		return m
	}
	round, ok := grill.LatestRound(c.card.Notes)
	if !ok || len(round.Questions) == 0 {
		m.status = c.id + ": no ## Round note to answer"
		return m
	}
	src, id := c.src, c.id
	send := func(line string) (string, error) {
		var b strings.Builder
		err := taskAnswer(src, id, line, &b)
		return b.String(), err
	}
	r := newRound(id, c.card.Title, round, send, m.width, m.height)
	m.round, m.status = &r, ""
	return m
}

// View draws nothing until the first WindowSizeMsg: a frame at a guessed
// width would wrap on a narrower terminal and leave ghost lines.
func (m paneModel) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	if m.round != nil {
		return m.round.View()
	}
	lines := m.lines
	if m.height > 0 && len(lines) > m.height {
		top := max(min(m.top, len(lines)-m.height), 0)
		lines = lines[top : top+m.height]
	}
	return strings.Join(lines, "\n")
}

// paneData is everything one frame of the pane shows, read once per refresh.
// Runs is the latest record per run, newest first, as history.Runs gives it.
type paneData struct {
	Daemon   string
	Paused   bool
	Queue    []work.Task
	QueueErr error
	Runs     []history.Record
	Timeouts map[string]int
	Now      time.Time
	Status   string
}

// settledShown is how many settled runs the pane keeps under the live ones.
const settledShown = 5

// queueEvery is how often the pane re-lists the queue. A GitHub board's
// List spends rate-limited API calls and the queue moves on a human's clock;
// the runs are a local file, re-read on every frame.
const queueEvery = 30 * time.Second

// paneReader reads the pane's frames, keeping the last queue listing until
// queueEvery has passed.
type paneReader struct {
	list   func() ([]work.Task, error)
	listed time.Time
	queue  []work.Task
	err    error
}

// listQueue is the pane's queue: one List, nothing per task.
func listQueue() ([]work.Task, error) {
	_, src, err := settingsAndSource()
	if err != nil {
		return nil, err
	}
	return src.List()
}

// read reads one frame at now: the heartbeat, the history file once, and the
// queue when its listing is older than queueEvery, or relist asks.
func (p *paneReader) read(now time.Time, relist bool) paneData {
	if relist || p.listed.IsZero() || now.Sub(p.listed) >= queueEvery {
		p.queue, p.err = p.list()
		p.listed = now
	}
	d := paneData{Daemon: daemon.Liveness(now), Paused: runner.Paused(), Timeouts: agentTimeouts(), Now: now,
		Queue: p.queue, QueueErr: p.err}
	d.Runs, _ = history.Runs("", 0)
	return d
}

func inFlight(runs []history.Record) []history.Record {
	var out []history.Record
	for _, r := range runs {
		if r.Status.InFlight() {
			out = append(out, r)
		}
	}
	return out
}

// heldMerges maps each task to the PR a verifier passed, from the newest PR
// and newest verdict recorded on the task: a Blocked task with one is a merge
// the gate held for a human (ADR 0015), not a question.
func heldMerges(runs []history.Record) map[string]string {
	pr, verdict := map[string]string{}, map[string]string{}
	for _, r := range runs {
		if _, ok := pr[r.Task]; !ok && r.PullRequest != "" {
			pr[r.Task] = r.PullRequest
		}
		if _, ok := verdict[r.Task]; !ok && r.Verification != "" {
			verdict[r.Task] = r.Verification
		}
	}
	out := map[string]string{}
	for task, url := range pr {
		if verdict[task] == "PASS" {
			out[task] = url
		}
	}
	return out
}

// rowKind is what a selectable line of Home is, and so what acting on it
// does.
type rowKind int

const (
	questionRow rowKind = iota // `?` a Question card: a tap opens its Round
	blockedRow                 // `!` a Blocked card
	mergeRow                   // `⇡` a merge the gate held for a human
	runRow                     // a run in flight: x stops it
)

// paneRow is a line of the frame a click or a key can select: a Your turn
// card or a run in flight.
type paneRow struct {
	Line int
	Kind rowKind
	ID   string
	Run  *history.Record
}

// renderPane is one frame as lines, none wider than width: the daemon line,
// Your turn (questions, Blocked cards, held merges), the runs in flight, then
// the last settled runs. rows are its selectable lines, in order, each
// starting with two spaces for the selection mark.
func renderPane(d paneData, width int) (lines []string, rows []paneRow) {
	// Text is wrapped or truncated plain first, then styled: an SGR sequence
	// never reaches a width helper and no sequence is ever split (ADR 0025).
	add := func(s string) { lines = append(lines, text.Truncate(s, width)) }
	addStyled := func(s string, style func(string) string) {
		lines = append(lines, style(text.Truncate(s, width)))
	}
	row := func(kind rowKind, id string, run *history.Record, mark, s string) {
		rows = append(rows, paneRow{Line: len(lines), Kind: kind, ID: id, Run: run})
		add("  " + mark + s)
	}

	state := "running"
	if d.Paused {
		state = "PAUSED"
	}
	daemonLine := fmt.Sprintf("%s · fleet %s", d.Daemon, state)
	if daemonDown(d.Daemon) {
		addStyled(daemonLine, paneStyle.attention)
	} else {
		add(daemonLine)
	}

	add("")
	addStyled("Your turn", paneStyle.accent)
	merges := heldMerges(d.Runs)
	var questions, blocked, held []work.Task
	for _, t := range d.Queue {
		switch {
		case t.Question:
			questions = append(questions, t)
		case !t.Blocked && t.Verdict != work.Blocked:
		case merges[t.ID] != "":
			held = append(held, t)
		default:
			blocked = append(blocked, t)
		}
	}
	turn := append(append(questions, blocked...), held...)
	// A shared id column so every title in Your turn starts at one place.
	idWidth := 0
	for _, t := range turn {
		idWidth = max(idWidth, ansi.StringWidth(t.ID))
	}
	for _, t := range turn {
		id := t.ID + strings.Repeat(" ", idWidth-ansi.StringWidth(t.ID))
		switch {
		case t.Question:
			row(questionRow, t.ID, nil, "? ", id+" "+t.Title)
		case merges[t.ID] != "":
			row(mergeRow, t.ID, nil, "⇡ ", id+" "+merges[t.ID])
		default:
			row(blockedRow, t.ID, nil, "! ", id+" "+t.Title)
		}
	}
	if d.QueueErr != nil {
		add("  queue: " + d.QueueErr.Error())
	} else if len(turn) == 0 {
		addStyled("  nothing: no Question, Blocked card or held merge", paneStyle.faint)
	}
	if len(turn) > 0 {
		// Where the marks are drawn is where they are explained, wrapped
		// rather than cut on a phone.
		for _, l := range wrap("  ? question · ! blocked · ⇡ merge held", width) {
			addStyled(l, paneStyle.faint)
		}
	}

	live := inFlight(d.Runs)
	add("")
	addStyled(fmt.Sprintf("In flight (%d)", len(live)), paneStyle.accent)
	for i, r := range live {
		rows = append(rows, paneRow{Line: len(lines), Kind: runRow, ID: r.Task, Run: &live[i]})
		timeout := timeoutFor(d.Timeouts, r.Agent)
		line := text.Truncate(fmt.Sprintf("  %d %s", i+1, formatRun(r, timeout, d.Now)), width)
		if stale(r, timeout, d.Now) {
			line = paintFirst(line, runAge(r, timeout, d.Now), paneStyle.attention)
		}
		lines = append(lines, line)
	}
	if len(live) == 0 {
		addStyled("  no run in flight", paneStyle.faint)
	}

	var settled []string
	for _, r := range d.Runs {
		if r.Status.InFlight() || r.Status == history.StatusScheduled {
			continue
		}
		settled = append(settled, fmt.Sprintf("  %s %s %s %s ago", r.Task, r.Agent, r.Status, shortDuration(d.Now.Sub(r.At))))
		if len(settled) == settledShown {
			break
		}
	}
	if len(settled) > 0 {
		add("")
		addStyled("Settled", paneStyle.accent)
		for _, l := range settled {
			addStyled(l, paneStyle.faint)
		}
	}

	if d.Status != "" {
		add("")
		add(d.Status)
	}
	add("")
	add("j/k/tap select · ⏎/tap selected open · x stop · p pause · q quit")
	return lines, rows
}

// daemonDown reports the liveness lines that mean nobody is routing: the
// pane draws them in attention.
func daemonDown(liveness string) bool {
	return !strings.HasPrefix(liveness, "daemon live")
}
