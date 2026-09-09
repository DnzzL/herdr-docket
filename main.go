// herdr-fleet — the backlog layer for Herdr agents: a Backlog.md project as
// the shared task queue, AGENT.md personas as the workers, and a daemon that
// routes To Do tasks to real coding agents, one run at a time.
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/DnzzL/herdr-fleet/internal/backlog"
	"github.com/DnzzL/herdr-fleet/internal/daemon"
	"github.com/DnzzL/herdr-fleet/internal/fleet"
	"github.com/DnzzL/herdr-fleet/internal/history"
	"github.com/DnzzL/herdr-fleet/internal/hostpath"
	"github.com/DnzzL/herdr-fleet/internal/pane"
	"github.com/DnzzL/herdr-fleet/internal/pick"
	"github.com/DnzzL/herdr-fleet/internal/runner"
	"github.com/DnzzL/herdr-fleet/internal/text"
)

// Version is stamped by the release build; "dev" for local builds.
var Version = "dev"

const usage = `herdr-fleet — a Backlog.md task queue worked by your Herdr agents

Usage:
  herdr-fleet daemon           Run the worker (started by the plugin startup hook)
  herdr-fleet init             Bootstrap the fleet dir (backlog project + example agent)
  herdr-fleet list             List tasks by status, with the routed agent
  herdr-fleet run <task-id>    Run one task now (any status except In Progress)
  herdr-fleet agent list       Show the agents and which are paused
  herdr-fleet agent pause <n>  Stop scheduling an agent (a running task finishes)
  herdr-fleet agent resume <n> Start scheduling it again
  herdr-fleet history [id]     Show recent runs
  herdr-fleet pane             Interactive board (used by the Herdr pane)
  herdr-fleet version          Print the version

Fleet dir: %s   (override: fleet.yaml in %s)
`

func main() {
	if len(os.Args) < 2 {
		settings, _ := fleet.LoadSettings()
		fmt.Printf(usage, settings.Dir, hostpath.ConfigDir())
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "daemon":
		err = daemon.Run()
	case "init":
		err = initCmd()
	case "list":
		err = list()
	case "run":
		err = runCmd(os.Args[2:])
	case "agent":
		err = agentCmd(os.Args[2:])
	case "history":
		err = historyCmd(os.Args[2:])
	case "pane":
		err = pane.Run()
	case "version":
		fmt.Println(Version)
	default:
		settings, _ := fleet.LoadSettings()
		fmt.Printf(usage, settings.Dir, hostpath.ConfigDir())
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "herdr-fleet:", err)
		os.Exit(1)
	}
}

func initCmd() error {
	settings, err := fleet.LoadSettings()
	if err != nil {
		return err
	}
	if err := fleet.Init(settings.Dir); err != nil {
		return err
	}
	fmt.Printf("fleet ready at %s\n", settings.Dir)
	fmt.Println("- describe your agents in agents/<name>/AGENT.md")
	fmt.Printf("- add work: BACKLOG_CWD=%s backlog task create \"...\" -a <agent>\n", settings.Dir)
	fmt.Println("- the daemon (or `herdr-fleet daemon`) picks tasks up from there")
	return nil
}

func list() error {
	settings, err := fleet.LoadSettings()
	if err != nil {
		return err
	}
	tasks, err := backlog.New(settings.Dir).List()
	if err != nil {
		return err
	}
	agents, _ := fleet.LoadAgents(settings.Dir)
	for _, status := range backlog.Statuses {
		var lines []string
		for _, t := range tasks {
			if t.Status != status {
				continue
			}
			who := strings.Join(t.Assignees, ",")
			if who == "" {
				who = "-"
			} else if _, ok := agents[t.Assignees[0]]; !ok {
				who += " (unknown!)"
			}
			lines = append(lines, fmt.Sprintf("  %-10s %-30s %s", t.ID, text.Truncate(t.Title, 30), who))
		}
		if len(lines) > 0 {
			fmt.Printf("%s:\n%s\n", status, strings.Join(lines, "\n"))
		}
	}
	return nil
}

func runCmd(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: herdr-fleet run <task-id>")
	}
	settings, err := fleet.LoadSettings()
	if err != nil {
		return err
	}
	board := backlog.New(settings.Dir)
	v, err := board.View(args[0])
	if err != nil {
		return err
	}
	if v.Status == backlog.StatusInProgress {
		return fmt.Errorf("%s is already In Progress", v.ID)
	}
	agents, _ := fleet.LoadAgents(settings.Dir)
	agent, err := routedAgent(agents, v.Task, settings.DefaultAgent)
	if err != nil {
		return fmt.Errorf("%s: %w", v.ID, err)
	}
	fmt.Printf("running %s (%s) with agent %s\n", v.ID, v.Title, agent.Name)
	return runner.Default(settings.Dir).Run(v.Task, agent, history.TriggerManual)
}

// routedAgent resolves the agent a task runs with, for surfaces acting on one
// task at a human's request. Unlike pick.Next it ignores Disabled: `run` is
// explicit intent, so pausing an agent parks the scheduler without forbidding
// the work. Every manual route goes through here, or the two drift apart.
func routedAgent(agents map[string]fleet.Agent, t backlog.Task, defaultAgent string) (fleet.Agent, error) {
	name := pick.AssigneeFor(t, defaultAgent)
	agent, ok := agents[name]
	if !ok {
		return fleet.Agent{}, fmt.Errorf("assignee %q is not a fleet agent", name)
	}
	return agent, nil
}

// agentCmd is the per-agent pause: list, pause, resume. A pause is a scheduler
// change only — a run already in flight keeps its timeout, and `herdr-fleet run`
// still reaches a paused agent, because that call is human intent.
func agentCmd(args []string) error {
	settings, err := fleet.LoadSettings()
	if err != nil {
		return err
	}
	if len(args) == 0 {
		return fmt.Errorf("usage: herdr-fleet agent list|pause|resume <name>")
	}

	switch args[0] {
	case "list":
		return agentList(settings.Dir)
	case "pause", "resume":
		if len(args) != 2 {
			return fmt.Errorf("usage: herdr-fleet agent %s <name>", args[0])
		}
		paused := args[0] == "pause"
		if err := fleet.SetDisabled(settings.Dir, args[1], paused); err != nil {
			return err
		}
		verb := "resumed"
		if paused {
			verb = "paused"
		}
		fmt.Printf("%s %s — the daemon notices within one tick (~15s)\n", args[1], verb)
		if paused {
			fmt.Println("a run already in flight keeps going; `run` still works")
		}
		return nil
	}
	return fmt.Errorf("unknown agent command %q", args[0])
}

// agentList reports the agents as the fleet dir holds them: paused is a fact
// about the persona file, so it stays true even with the daemon not running.
func agentList(dir string) error {
	agents, diags := fleet.LoadAgents(dir)
	names := make([]string, 0, len(agents))
	for name := range agents {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		a := agents[name]
		status := "active"
		if a.Disabled {
			status = "paused"
		}
		fmt.Printf("%-14s %-8s %s\n", a.Name, status, a.Workdir)
	}
	for _, d := range diags {
		fmt.Fprintf(os.Stderr, "herdr-fleet: %s\n", d)
	}
	return nil
}

func historyCmd(args []string) error {
	task := ""
	if len(args) > 0 {
		task = args[0]
	}
	runs, err := history.Runs(task, 30)
	if err != nil {
		return err
	}
	for _, r := range runs {
		line := fmt.Sprintf("%s  %-9s %-10s %s", r.At.Format(time.DateTime), r.Status, r.Task, r.Trigger)
		if r.Error != "" {
			line += "  " + r.Error
		}
		fmt.Println(line)
	}
	return nil
}
