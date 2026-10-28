package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/DnzzL/herdr-docket/internal/daemon"
	"github.com/DnzzL/herdr-docket/internal/fleet"
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
	elapsed := now.Sub(r.At)
	age := fmt.Sprintf("%s / %dm", shortDuration(elapsed), timeoutMinutes)
	switch {
	case stale(r, timeoutMinutes, now):
		age += " stale?"
	case r.Status == history.StatusTimedOut:
		age += " late"
	}
	trigger := string(r.Trigger)
	if trigger == "" {
		trigger = "-"
	}
	return fmt.Sprintf("%-10s %-12s %-8s %-18s %s", r.Task, r.Agent, trigger, age, r.WorkspaceID)
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

// paneCmd is the overlay: the daemon line, Your turn, the runs
// in flight and the last settled ones, redrawn every two seconds (the queue
// re-listed every 30) and cut to
// the terminal's width so a phone never wraps it.
// Keys: j/k or 1-9 select, x stops the selected run, p pauses or resumes the
// fleet, q quits. Everything it does is runsCmd, stopRun and SetPaused; it
// only draws and forwards keys. Single keypresses come from stty, so the
// pane needs no terminal library.
func paneCmd() error {
	saved, err := stty("-g")
	if err != nil {
		return fmt.Errorf("the pane needs a terminal: %w", err)
	}
	if _, err := stty("-icanon", "-echo", "min", "1"); err != nil {
		return err
	}
	defer stty(strings.TrimSpace(saved))
	fmt.Print("\033[?25l")
	defer fmt.Print("\033[?25h\n")

	keys := make(chan byte)
	go func() {
		buf := make([]byte, 1)
		for {
			if n, err := os.Stdin.Read(buf); err != nil || n == 0 {
				close(keys)
				return
			}
			keys <- buf[0]
		}
	}()
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()

	closer := host.New()
	reader := &paneReader{list: listQueue}
	sel, status := 0, ""
	for {
		d := reader.read(time.Now())
		live := inFlight(d.Runs)
		if sel >= len(live) {
			sel = max(len(live)-1, 0)
		}
		d.Sel, d.Status = sel, status
		var b strings.Builder
		b.WriteString("\033[H\033[2J")
		for _, line := range renderPane(d, termWidth()) {
			b.WriteString(line + "\r\n")
		}
		fmt.Print(b.String())

		select {
		case <-sigs:
			return nil
		case <-tick.C:
		case k, ok := <-keys:
			if !ok {
				return nil
			}
			switch {
			case k == 'q':
				return nil
			case k == 'j' && sel < len(live)-1:
				sel++
			case k == 'k' && sel > 0:
				sel--
			case k >= '1' && k <= '9' && int(k-'1') < len(live):
				sel = int(k - '1')
			case k == 'x' && len(live) > 0:
				var b strings.Builder
				if err := stopRecord(closer, live[sel], timeoutFor(agentTimeouts(), live[sel].Agent), &b); err != nil {
					status = err.Error()
				} else {
					status = strings.TrimSpace(b.String())
				}
			case k == 'p':
				paused := !runner.Paused()
				if err := runner.SetPaused(paused); err != nil {
					status = err.Error()
				} else if paused {
					status = "paused — runs in flight finish"
				} else {
					status = "resumed"
				}
			}
		}
	}
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
	Sel      int
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
// queue when its listing is older than queueEvery.
func (p *paneReader) read(now time.Time) paneData {
	if p.listed.IsZero() || now.Sub(p.listed) >= queueEvery {
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

// renderPane is one frame as lines, none wider than width: the daemon line,
// Your turn (questions, Blocked cards, held merges), the runs in flight, then
// the last settled runs.
func renderPane(d paneData, width int) []string {
	var lines []string
	add := func(s string) { lines = append(lines, text.Truncate(s, width)) }

	state := "running"
	if d.Paused {
		state = "PAUSED"
	}
	add(fmt.Sprintf("%s · fleet %s", d.Daemon, state))

	add("")
	add("Your turn")
	merges := heldMerges(d.Runs)
	var questions, blocked, held []string
	for _, t := range d.Queue {
		switch {
		case t.Question:
			questions = append(questions, "  ? "+t.ID+" "+t.Title)
		case !t.Blocked && t.Verdict != work.Blocked:
		case merges[t.ID] != "":
			held = append(held, "  ⇡ "+t.ID+" "+merges[t.ID])
		default:
			blocked = append(blocked, "  ! "+t.ID+" "+t.Title)
		}
	}
	turn := append(append(questions, blocked...), held...)
	for _, l := range turn {
		add(l)
	}
	if d.QueueErr != nil {
		add("  queue: " + d.QueueErr.Error())
	} else if len(turn) == 0 {
		add("  nothing: no Question, Blocked card or held merge")
	}

	live := inFlight(d.Runs)
	add("")
	add(fmt.Sprintf("In flight (%d)", len(live)))
	for i, r := range live {
		mark := "  "
		if i == d.Sel {
			mark = "> "
		}
		add(fmt.Sprintf("%s%d %s", mark, i+1, formatRun(r, timeoutFor(d.Timeouts, r.Agent), d.Now)))
	}
	if len(live) == 0 {
		add("  no run in flight")
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
		add("Settled")
		for _, l := range settled {
			add(l)
		}
	}

	if d.Status != "" {
		add("")
		add(d.Status)
	}
	add("")
	add("j/k select · x stop · p pause · q quit")
	return lines
}

// termWidth is the pane's width in columns, from stty; 80 when it cannot say.
func termWidth() int {
	out, err := stty("size")
	if err != nil {
		return 80
	}
	var rows, cols int
	if _, err := fmt.Sscan(out, &rows, &cols); err != nil || cols <= 0 {
		return 80
	}
	return cols
}

func stty(args ...string) (string, error) {
	cmd := exec.Command("stty", args...)
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	return string(out), err
}
