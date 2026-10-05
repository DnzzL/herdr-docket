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

	"github.com/DnzzL/herdr-docket/internal/fleet"
	"github.com/DnzzL/herdr-docket/internal/history"
	"github.com/DnzzL/herdr-docket/internal/host"
	"github.com/DnzzL/herdr-docket/internal/runner"
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

// formatRun is one run in flight: task, agent, stage, elapsed against the
// timeout, and the workspace it lives in. A running record past its timeout
// is marked stale: the runner writes timed_out at the deadline, so a running
// record older than that is one a dead daemon never closed (ADR 0008).
func formatRun(r history.Record, timeoutMinutes int, now time.Time) string {
	elapsed := now.Sub(r.At)
	age := fmt.Sprintf("%s / %dm", shortDuration(elapsed), timeoutMinutes)
	switch {
	case r.Status == history.StatusTimedOut:
		age += " late"
	case elapsed > time.Duration(timeoutMinutes)*time.Minute:
		age += " stale?"
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
	for _, r := range live {
		if r.Task == task {
			return stopRecord(c, r, out)
		}
	}
	return fmt.Errorf("%s has no run in flight", task)
}

func stopRecord(c sessionCloser, r history.Record, out io.Writer) error {
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
	var failed []string
	for _, r := range live {
		if err := stopRecord(c, r, out); err != nil {
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

// paneCmd is the overlay: the runs in flight, redrawn every two seconds.
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
	sel, status := 0, ""
	for {
		live, _ := history.InFlight()
		if sel >= len(live) {
			sel = max(len(live)-1, 0)
		}
		drawPane(live, sel, status)

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
				if err := stopRecord(closer, live[sel], &b); err != nil {
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

func drawPane(live []history.Record, sel int, status string) {
	var b strings.Builder
	b.WriteString("\033[H\033[2J")
	state := "running"
	if runner.Paused() {
		state = "PAUSED"
	}
	fmt.Fprintf(&b, "herdr-docket — fleet %s — %d in flight\r\n\r\n", state, len(live))
	timeouts := agentTimeouts()
	now := time.Now()
	for i, r := range live {
		mark := "  "
		if i == sel {
			mark = "> "
		}
		fmt.Fprintf(&b, "%s%d %s\r\n", mark, i+1, formatRun(r, timeoutFor(timeouts, r.Agent), now))
	}
	if len(live) == 0 {
		b.WriteString("  no run in flight\r\n")
	}
	if status != "" {
		fmt.Fprintf(&b, "\r\n%s\r\n", status)
	}
	b.WriteString("\r\nj/k select · x stop · p pause/resume · q quit\r\n")
	fmt.Print(b.String())
}

func stty(args ...string) (string, error) {
	cmd := exec.Command("stty", args...)
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	return string(out), err
}
