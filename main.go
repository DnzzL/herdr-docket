// herdr-fleet — the backlog layer for Herdr agents: a shared task queue,
// AGENT.md personas as the workers, and a daemon that routes open tasks to
// real coding agents, one run at a time. The queue is a Backlog.md project or
// a Basecamp project; which one is configuration, and nothing above the
// adapter knows the difference.
package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/DnzzL/herdr-fleet/internal/daemon"
	"github.com/DnzzL/herdr-fleet/internal/fleet"
	"github.com/DnzzL/herdr-fleet/internal/history"
	"github.com/DnzzL/herdr-fleet/internal/host"
	"github.com/DnzzL/herdr-fleet/internal/hostpath"
	"github.com/DnzzL/herdr-fleet/internal/pane"
	"github.com/DnzzL/herdr-fleet/internal/pick"
	"github.com/DnzzL/herdr-fleet/internal/runner"
	"github.com/DnzzL/herdr-fleet/internal/text"
	"github.com/DnzzL/herdr-fleet/internal/work"
)

// Version is stamped by the release build; "dev" for local builds.
var Version = "dev"

const usage = `herdr-fleet — a task queue worked by your Herdr agents

Usage:
  herdr-fleet daemon           Run the worker (started by the plugin startup hook)
  herdr-fleet init             Bootstrap the fleet dir (backlog project + example agent)
  herdr-fleet auth <queue>     Sign in to a hosted queue and store its credentials
  herdr-fleet list             List the queue, grouped by phase
  herdr-fleet run <task-id>    Run one open task now, whatever its status
  herdr-fleet task list        List open work from the queue (--all for closed)
  herdr-fleet task view <id>   Show one task: body, notes and criteria
  herdr-fleet task create      Add work: "<title>" [-a <agent>] [-d "<body>"]
  herdr-fleet task note <id>   Append to a task's notes
  herdr-fleet task done|fail|block <id> [--note "..."]  Close with a verdict
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
	case "auth":
		err = authCmd(os.Args[2:])
	case "list":
		err = list()
	case "run":
		err = runCmd(os.Args[2:])
	case "task":
		err = taskCmd(os.Args[2:])
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
	if err := fleet.Init(settings); err != nil {
		return err
	}
	fmt.Printf("fleet ready at %s\n", settings.Dir)
	fmt.Println("- describe your agents in agents/<name>/AGENT.md")
	fmt.Println("- add work: herdr-fleet task create \"...\" -a <agent>")
	fmt.Println("- the daemon (or `herdr-fleet daemon`) picks tasks up from there")
	return nil
}

// authCmd is thin on purpose: which queues can be signed in to, and how, is
// fleet's business, not the CLI's.
func authCmd(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: herdr-fleet auth <queue>   (queues that sign in: basecamp)")
	}
	return fleet.Auth(args[0], os.Stdout)
}

// settingsAndSource resolves the fleet's configuration and the queue it names
// — one shape for every command that needs both.
func settingsAndSource() (fleet.Settings, work.Source, error) {
	s, err := fleet.LoadSettings()
	if err != nil {
		return fleet.Settings{}, nil, err
	}
	src, err := fleet.NewSource(s)
	return s, src, err
}

func list() error {
	settings, src, err := settingsAndSource()
	if err != nil {
		return err
	}
	items, err := src.List()
	if err != nil {
		return err
	}
	agents, _ := fleet.LoadAgents(settings.Dir)
	for _, phase := range work.PhasesOf(items) {
		var lines []string
		for _, it := range items {
			if it.Phase != phase {
				continue
			}
			who := it.Assignee
			if who == "" {
				who = "-"
			} else if _, ok := agents[who]; !ok {
				who += " (unknown!)"
			}
			lines = append(lines, fmt.Sprintf("  %-10s %-30s %s", it.ID, text.Truncate(it.Title, 30), who))
		}
		if len(lines) > 0 {
			heading := phase
			if heading == "" {
				heading = "(no phase)"
			}
			fmt.Printf("%s:\n%s\n", heading, strings.Join(lines, "\n"))
		}
	}
	return nil
}

// taskCmd resolves the fleet's queue through the configured adapter and runs
// the agent-facing verbs. It is the only queue surface an agent is given, and
// it is backend-blind: the adapter decides what answers.
func taskCmd(args []string) error {
	_, src, err := settingsAndSource()
	if err != nil {
		return err
	}
	return runTaskCmd(src, args, os.Stdout)
}

// runTaskCmd is the task verbs with the source injected, so the CLI's shape is
// tested without a backend on disk. It is the agent's whole write surface:
// list and view to read, create to hand on follow-up work, note to say where
// things stand, and done/fail/block to close with a verdict.
func runTaskCmd(src work.Source, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: herdr-fleet task list|view|create|note|done|fail|block")
	}
	switch args[0] {
	case "list":
		all := false
		for _, a := range args[1:] {
			if a != "--all" {
				return fmt.Errorf("task list: unknown flag %q", a)
			}
			all = true
		}
		return taskList(src, out, all)
	case "view":
		if len(args) != 2 {
			return fmt.Errorf("usage: herdr-fleet task view <id>")
		}
		return taskView(src, args[1], out)
	case "create":
		return taskCreate(src, args[1:], out)
	case "note":
		if len(args) != 3 {
			return fmt.Errorf(`usage: herdr-fleet task note <id> "<text>"`)
		}
		return src.Comment(args[1], args[2])
	case "done", "fail", "block":
		return taskClose(src, args[0], args[1:], out)
	}
	return fmt.Errorf("unknown task command %q", args[0])
}

func taskCreate(src work.Source, args []string, out io.Writer) error {
	const usage = `usage: herdr-fleet task create "<title>" [-a <agent>] [-d "<body>"]`
	var title, body, assignee string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-a", "--assignee":
			v, err := flagValue(args, &i, args[i])
			if err != nil {
				return err
			}
			assignee = v
		case "-d", "--description":
			v, err := flagValue(args, &i, args[i])
			if err != nil {
				return err
			}
			body = v
		default:
			if title != "" {
				return fmt.Errorf("task create: unexpected argument %q\n%s", args[i], usage)
			}
			title = args[i]
		}
	}
	if title == "" {
		return fmt.Errorf("task create: a title is required\n%s", usage)
	}
	id, err := src.Create(title, body, assignee)
	if err != nil {
		return err
	}
	if id == "" {
		fmt.Fprintln(out, "created")
		return nil
	}
	fmt.Fprintf(out, "created %s\n", id)
	return nil
}

// closeVerbs is the CLI's spelling of the verdict vocabulary: the verbs are
// short, the verdicts are the words the port carries.
var closeVerbs = map[string]work.Verdict{
	"done":  work.Done,
	"fail":  work.Failed,
	"block": work.Blocked,
}

// taskClose runs one of the three closing verbs. The note is recorded first,
// so the reason is on the item by the time it closes.
func taskClose(src work.Source, verb string, args []string, out io.Writer) error {
	verdict, ok := closeVerbs[verb]
	if !ok {
		return fmt.Errorf("unknown closing verb %q", verb)
	}
	usage := fmt.Sprintf(`usage: herdr-fleet task %s <id> [--note "<text>"]`, verb)
	var id, note string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-n", "--note":
			v, err := flagValue(args, &i, args[i])
			if err != nil {
				return err
			}
			note = v
		default:
			if id != "" {
				return fmt.Errorf("task %s: unexpected argument %q\n%s", verb, args[i], usage)
			}
			id = args[i]
		}
	}
	if id == "" {
		return fmt.Errorf("task %s: an id is required\n%s", verb, usage)
	}
	if note != "" {
		if err := src.Comment(id, note); err != nil {
			return err
		}
	}
	if err := src.Close(id, verdict); err != nil {
		return err
	}
	fmt.Fprintf(out, "%s %s\n", id, verb)
	return nil
}

// flagValue consumes the value after a flag, advancing the loop index.
func flagValue(args []string, i *int, flag string) (string, error) {
	if *i+1 >= len(args) {
		return "", fmt.Errorf("%s needs a value", flag)
	}
	*i++
	return args[*i], nil
}

func taskList(src work.Source, out io.Writer, all bool) error {
	items, err := src.List()
	if err != nil {
		return err
	}
	for _, it := range items {
		if !it.Open && !all {
			continue
		}
		who := it.Assignee
		if who == "" {
			who = "-"
		}
		fmt.Fprintf(out, "%-10s %-30s %-12s %s\n", it.ID, text.Truncate(it.Title, 30), who, it.Phase)
	}
	return nil
}

func taskView(src work.Source, id string, out io.Writer) error {
	it, err := src.Get(id)
	if err != nil {
		return err
	}
	state, who := "open", it.Assignee
	if !it.Open {
		state = "closed"
	}
	if who == "" {
		who = "-"
	}
	fmt.Fprintf(out, "%s — %s\n%s · %s · %s\n", it.ID, it.Title, state, it.Phase, who)
	if it.Body != "" {
		fmt.Fprintf(out, "\n## Description\n\n%s\n", it.Body)
	}
	if len(it.Criteria) > 0 {
		fmt.Fprintf(out, "\n## Acceptance criteria\n\n")
		for _, c := range it.Criteria {
			box := "[ ]"
			if c.Checked {
				box = "[x]"
			}
			fmt.Fprintf(out, "- %s #%d %s\n", box, c.Index, c.Text)
		}
	}
	if it.Notes != "" {
		fmt.Fprintf(out, "\n## Notes\n\n%s\n", it.Notes)
	}
	return nil
}

func runCmd(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: herdr-fleet run <task-id>")
	}
	settings, board, err := settingsAndSource()
	if err != nil {
		return err
	}
	it, err := board.Get(args[0])
	if err != nil {
		return err
	}
	if !it.Open {
		return fmt.Errorf("%s is already closed — reopen it first if it is still work", it.ID)
	}
	agents, _ := fleet.LoadAgents(settings.Dir)
	agent, err := routedAgent(agents, it, settings.DefaultAgent)
	if err != nil {
		return fmt.Errorf("%s: %w", it.ID, err)
	}
	fmt.Printf("running %s (%s) with agent %s\n", it.ID, it.Title, agent.Name)
	return runner.New(host.New(), board, settings.Dir).Run(it, agent, history.TriggerManual)
}

// routedAgent resolves the agent a task runs with, for surfaces acting on one
// task at a human's request. Unlike pick.Next it ignores Disabled: `run` is
// explicit intent, so pausing an agent parks the scheduler without forbidding
// the work. Every manual route goes through here, or the two drift apart.
func routedAgent(agents map[string]fleet.Agent, it work.Item, defaultAgent string) (fleet.Agent, error) {
	name := pick.AssigneeFor(it, defaultAgent)
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
