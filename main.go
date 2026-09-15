// herdr-docket — the queue layer for Herdr agents: a shared task queue,
// AGENT.md personas as the workers, and a daemon that routes open tasks to
// real coding agents, one run at a time. The queue is a Backlog.md project, a
// Basecamp project or a GitHub Projects board; which one is configuration, and
// nothing above the adapter knows the difference.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/DnzzL/herdr-docket/internal/daemon"
	"github.com/DnzzL/herdr-docket/internal/fleet"
	"github.com/DnzzL/herdr-docket/internal/history"
	"github.com/DnzzL/herdr-docket/internal/host"
	"github.com/DnzzL/herdr-docket/internal/hostpath"
	"github.com/DnzzL/herdr-docket/internal/pane"
	"github.com/DnzzL/herdr-docket/internal/pick"
	"github.com/DnzzL/herdr-docket/internal/runner"
	"github.com/DnzzL/herdr-docket/internal/skill"
	"github.com/DnzzL/herdr-docket/internal/text"
	"github.com/DnzzL/herdr-docket/internal/work"
)

// Version is stamped by the release build; "dev" for local builds.
var Version = "dev"

const usage = `herdr-docket — a task queue worked by your Herdr agents

Usage:
  herdr-docket daemon           Run the worker (started by the plugin startup hook)
  herdr-docket init             Bootstrap the fleet dir (the local Backlog.md project + example agent)
  herdr-docket auth <queue>     Sign in to a hosted queue and store its credentials ([--token <pat>])
  herdr-docket list             List the queue, grouped by phase
  herdr-docket run <task-id>    Run one open task now, whatever its phase
  herdr-docket task list        List open work from the queue (--all for closed)
  herdr-docket task view <id>   Show one task: body, notes and criteria
  herdr-docket task create      Add work: "<title>" [-a <agent>] [-d "<body>"]
  herdr-docket task assign <id> <agent>  Hand a task to another agent
  herdr-docket task note <id>   Append to a task's notes
  herdr-docket task done|fail|block <id> [--note "..."]  Close with a verdict
  herdr-docket agent list       Show the agents and which are paused
  herdr-docket agent pause <n>  Stop scheduling an agent (a running task finishes)
  herdr-docket agent resume <n> Start scheduling it again
  herdr-docket history [id]     Show recent runs
  herdr-docket pane             Interactive board (used by the Herdr pane)
  herdr-docket install-skill    Teach your coding agent to write fleet tasks
  herdr-docket version          Print the version

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
	case "install-skill":
		target := ""
		if len(os.Args) > 2 {
			target = os.Args[2]
		}
		err = skill.Install(target)
	case "version":
		fmt.Println(Version)
	default:
		settings, _ := fleet.LoadSettings()
		fmt.Printf(usage, settings.Dir, hostpath.ConfigDir())
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "herdr-docket:", err)
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
	fmt.Println("- add work: herdr-docket task create \"...\" -a <agent>")
	fmt.Println("- the daemon (or `herdr-docket daemon`) picks tasks up from there")
	return nil
}

// authCmd is thin on purpose: which queues can be signed in to, and how, is
// fleet's business, not the CLI's. The name is a queue kind — basecamp,
// github — or the name of one source when a fleet has several.
func authCmd(args []string) error {
	const usage = `usage: herdr-docket auth <queue> [--token <pat>]
  queues that sign in: basecamp   a Launchpad app of your own, browser flow
                       github     a PAT with the project scope, or gh's token`
	name, token := "", ""
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "--token":
			if i++; i >= len(args) {
				return fmt.Errorf("auth: --token wants a personal access token\n%s", usage)
			}
			token = args[i]
		case strings.HasPrefix(arg, "--token="):
			token = strings.TrimPrefix(arg, "--token=")
		case name == "":
			name = arg
		default:
			return fmt.Errorf("auth: unexpected argument %q\n%s", arg, usage)
		}
	}
	if name == "" {
		return errors.New(usage)
	}
	// Settings rather than a constructed source: signing in is how a
	// half-configured queue gets *configured*, so building the adapter first
	// would refuse exactly the fleet that needs this command.
	settings, err := fleet.LoadSettings()
	if err != nil {
		return err
	}
	return fleet.Auth(settings, name, token, os.Stdout)
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
			// The routed agent, not the written one: with a default the two
			// differ, and a board that showed "-" would hide who is about to
			// pick the task up.
			who := pick.AssigneeFor(it, settings.Defaults())
			switch _, known := agents[who]; {
			case who == "":
				who = "-"
			case !known:
				who += " (unknown!)"
			case it.Assignee == "":
				who += " (default)"
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
// list and view to read, create to hand on follow-up work, assign to pass one
// on, note to say where things stand, and done/fail/block to close with a
// verdict.
func runTaskCmd(src work.Source, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: herdr-docket task list|view|create|assign|note|done|fail|block")
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
			return fmt.Errorf("usage: herdr-docket task view <id>")
		}
		return taskView(src, args[1], out)
	case "create":
		return taskCreate(src, args[1:], out)
	case "assign":
		if len(args) != 3 {
			return fmt.Errorf("usage: herdr-docket task assign <id> <agent>")
		}
		return taskAssign(src, args[1], args[2], out)
	case "note":
		if len(args) != 3 {
			return fmt.Errorf(`usage: herdr-docket task note <id> "<text>"`)
		}
		return src.Comment(args[1], args[2])
	case "done", "fail", "block":
		return taskClose(src, args[0], args[1:], out)
	}
	return fmt.Errorf("unknown task command %q", args[0])
}

// taskAssign re-routes a task. This is how a PM hands specced work to a dev
// without closing it: one task, one thread, one id. A queue that routes work
// some other way says so rather than accepting the call and doing nothing.
//
// The agent name is not checked against agents/ — same as `task create -a`.
// An unknown assignee is visible where it matters: `herdr-docket list` marks
// it, and the picker leaves the task alone rather than guessing.
func taskAssign(src work.Source, id, agent string, out io.Writer) error {
	a, ok := src.(work.Assigner)
	if !ok {
		return fmt.Errorf("%s: this queue routes work another way and cannot be reassigned", id)
	}
	if err := a.Assign(id, agent); err != nil {
		return err
	}
	fmt.Fprintf(out, "%s assigned to %s\n", id, agent)
	return nil
}

func taskCreate(src work.Source, args []string, out io.Writer) error {
	const usage = `usage: herdr-docket task create "<title>" [-a <agent>] [-d "<body>"] [-s <source>]`
	var title, body, assignee, queue string
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
		case "-s", "--source":
			v, err := flagValue(args, &i, args[i])
			if err != nil {
				return err
			}
			queue = v
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
	id, err := createTask(src, queue, title, body, assignee)
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

// createTask chooses the queue a new task lands in. A fleet with several
// queues routes by -s/--source: required when there is more than one, implied
// when there is a single named one, and refused when the queue is not a
// composite at all (a lone source has nothing to choose).
func createTask(src work.Source, queue, title, body, assignee string) (string, error) {
	ms, ok := src.(work.MultiSource)
	if !ok {
		if queue != "" {
			return "", fmt.Errorf("task create: -s/--source is only for a fleet with several queues")
		}
		return src.Create(title, body, assignee)
	}
	if queue == "" {
		names := ms.Names()
		if len(names) != 1 {
			return "", fmt.Errorf("task create: this fleet has %d queues (%s) — choose one with -s/--source", len(names), strings.Join(names, ", "))
		}
		queue = names[0]
	}
	return ms.CreateIn(queue, title, body, assignee)
}

// closeVerbs is the CLI's spelling of the verdict vocabulary: the verbs are
// short, the verdicts are the words the port carries.
var closeVerbs = map[string]work.Verdict{
	"done":  work.Done,
	"fail":  work.Failed,
	"block": work.Blocked,
}

// taskClose runs one of the three closing verbs. The note is recorded first,
// so the reason is on the task by the time it closes.
func taskClose(src work.Source, verb string, args []string, out io.Writer) error {
	verdict, ok := closeVerbs[verb]
	if !ok {
		return fmt.Errorf("unknown closing verb %q", verb)
	}
	usage := fmt.Sprintf(`usage: herdr-docket task %s <id> [--note "<text>"]`, verb)
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
		return fmt.Errorf("usage: herdr-docket run <task-id>")
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
	agent, err := routedAgent(agents, it, settings.Defaults())
	if err != nil {
		return fmt.Errorf("%s: %w", it.ID, err)
	}
	fmt.Printf("running %s (%s) with agent %s\n", it.ID, it.Title, agent.Name)
	return runner.New(host.New(), settings).Run(board, it, agent, history.TriggerManual)
}

// routedAgent resolves the agent a task runs with, for surfaces acting on one
// task at a human's request. Unlike pick.Next it ignores Disabled: `run` is
// explicit intent, so pausing an agent parks the scheduler without forbidding
// the work. Every manual route goes through here, or the two drift apart.
func routedAgent(agents map[string]fleet.Agent, it work.Task, defaults fleet.Defaults) (fleet.Agent, error) {
	name := pick.AssigneeFor(it, defaults)
	agent, ok := agents[name]
	if !ok {
		return fleet.Agent{}, fmt.Errorf("assignee %q is not a fleet agent", name)
	}
	return agent, nil
}

// agentCmd is the per-agent pause: list, pause, resume. A pause is a scheduler
// change only — a run already in flight keeps its timeout, and `herdr-docket run`
// still reaches a paused agent, because that call is human intent.
func agentCmd(args []string) error {
	settings, err := fleet.LoadSettings()
	if err != nil {
		return err
	}
	if len(args) == 0 {
		return fmt.Errorf("usage: herdr-docket agent list|pause|resume <name>")
	}

	switch args[0] {
	case "list":
		return agentList(settings.Dir)
	case "pause", "resume":
		if len(args) != 2 {
			return fmt.Errorf("usage: herdr-docket agent %s <name>", args[0])
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
// A budgeted agent also shows what it has spent in the rolling window.
func agentList(dir string) error {
	agents, diags := fleet.LoadAgents(dir)
	usage := history.UsageSince(time.Now())
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
		u := usage[name]
		line := fmt.Sprintf("%-14s %-8s %s", a.Name, status, a.Workdir)
		if b := pick.BudgetLine(a, u.Runs, u.Minutes); b != "" {
			line += "  " + b
		}
		fmt.Println(line)
	}
	for _, d := range diags {
		fmt.Fprintf(os.Stderr, "herdr-docket: %s\n", d)
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
		fmt.Println(formatHistory(r))
	}
	return nil
}

// formatHistory is one run's line: when, how the mechanics ended it, which
// task and trigger, then how long it took and the verdict the agent reported.
// Duration and verdict are on the closing record only, so an in-flight run
// shows neither.
func formatHistory(r history.Record) string {
	line := fmt.Sprintf("%s  %-9s %-10s %s", r.At.Format(time.DateTime), r.Status, r.Task, r.Trigger)
	if r.DurationSeconds > 0 {
		line += "  " + (time.Duration(r.DurationSeconds) * time.Second).String()
	}
	if r.Verdict != "" {
		line += "  " + r.Verdict
	}
	if r.Error != "" {
		line += "  " + r.Error
	}
	return line
}
