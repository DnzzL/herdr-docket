// herdr-fleet — the backlog layer for Herdr agents: a Backlog.md project as
// the shared task queue, AGENT.md personas as the workers, and a daemon that
// routes To Do tasks to real coding agents, one run at a time.
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/DnzzL/herdr-fleet/internal/backlog"
	"github.com/DnzzL/herdr-fleet/internal/daemon"
	"github.com/DnzzL/herdr-fleet/internal/fleet"
	"github.com/DnzzL/herdr-fleet/internal/history"
	"github.com/DnzzL/herdr-fleet/internal/hostpath"
	"github.com/DnzzL/herdr-fleet/internal/pane"
	"github.com/DnzzL/herdr-fleet/internal/runner"
)

// Version is stamped by the release build; "dev" for local builds.
var Version = "dev"

const usage = `herdr-fleet — a Backlog.md task queue worked by your Herdr agents

Usage:
  herdr-fleet daemon           Run the worker (started by the plugin startup hook)
  herdr-fleet init             Bootstrap the fleet dir (backlog project + example agent)
  herdr-fleet list             List tasks by status, with the routed agent
  herdr-fleet run <task-id>    Run one task now (any status except In Progress)
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
			lines = append(lines, fmt.Sprintf("  %-10s %-30s %s", t.ID, truncate(t.Title, 30), who))
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
	name := settings.DefaultAgent
	if len(v.Assignees) > 0 {
		name = v.Assignees[0]
	}
	agent, ok := agents[name]
	if !ok {
		return fmt.Errorf("%s: assignee %q is not a fleet agent", v.ID, name)
	}
	fmt.Printf("running %s (%s) with agent %s\n", v.ID, v.Title, agent.Name)
	return runner.Default(settings.Dir).Run(v.Task, agent, history.TriggerManual)
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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
