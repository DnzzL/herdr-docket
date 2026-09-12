// Package daemon is the long-running worker started by the plugin's startup
// hook. Every tick it re-reads the fleet (agents and settings), polls the
// platform queue, and runs the most urgent routed task — one at a time.
// It re-executes itself when the plugin binary is upgraded underneath it.
//
// Deciding is pick's job; this package is the part with the side effects.
package daemon

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DnzzL/herdr-fleet/internal/fleet"
	"github.com/DnzzL/herdr-fleet/internal/history"
	"github.com/DnzzL/herdr-fleet/internal/host"
	"github.com/DnzzL/herdr-fleet/internal/pick"
	"github.com/DnzzL/herdr-fleet/internal/runner"
)

// tickInterval is how often the queue is polled. Short enough that a task
// you just wrote is picked up while you are still watching the board.
const tickInterval = 15 * time.Second

// The daemon's reads of the outside world, as variables so a tick can be
// exercised without a fleet dir or a backend on disk. Production never
// replaces them.
var (
	loadSettings = fleet.LoadSettings
	loadAgents   = fleet.LoadAgents
	newSource    = fleet.NewSource
)

func Run() error {
	log.SetPrefix("[herdr-fleet] ")

	release, err := acquireLock()
	if err != nil {
		return err
	}
	defer release()

	settings, err := fleet.LoadSettings()
	if err != nil {
		return fmt.Errorf("fleet.yaml: %w", err)
	}
	if _, err := os.Stat(settings.Dir); err != nil {
		return fmt.Errorf("fleet dir %s does not exist — run `herdr-fleet init` first", settings.Dir)
	}
	log.Printf("daemon starting, fleet=%s", settings.Dir)

	// Fail fast on a queue the fleet cannot build, but keep no handle on it:
	// every tick builds its own, so the writes always land on the queue the
	// tick read from.
	if _, err := fleet.NewSource(settings); err != nil {
		return err
	}
	runs := runner.New(host.New(), settings.Dir)
	binary := binaryStamp()
	reported := map[string]bool{}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	tick := time.NewTicker(tickInterval)
	defer tick.Stop()

	evaluate(runs, reported) // don't wait a tick to notice what is already To Do

	for {
		select {
		case <-tick.C:
			if stamp := binaryStamp(); stamp != binary && stamp != "" {
				restart(release, runs)
			}
			evaluate(runs, reported)
		case s := <-sigs:
			log.Printf("received %v, shutting down", s)
			return nil
		}
	}
}

// evaluate polls the queue and starts every run that can start now: at
// most one per agent (and one per shared checkout for root-mode agents).
// reported keeps the unknown-assignee noise down to one comment per task per
// daemon lifetime: re-noting an unfixed typo every 15 seconds would bury the
// task in comments.
func evaluate(runs *runner.Runner, reported map[string]bool) {
	settings, err := loadSettings()
	if err != nil {
		log.Printf("fleet.yaml error, skipping this tick: %v", err)
		return
	}
	agents, diags := loadAgents(settings.Dir)
	for _, d := range diags {
		if !reported[d.String()] {
			reported[d.String()] = true
			log.Printf("agent not loaded — %s", d)
		}
	}

	// One source per evaluation: the tasks picked from below and the queue
	// claimed, commented and closed on are the same value, so a config change
	// (or a fleet of several queues) can never split the read from the writes.
	src, err := newSource(settings)
	if err != nil {
		log.Printf("queue error, skipping this tick: %v", err)
		return
	}
	tasks, err := src.List()
	if err != nil {
		log.Printf("queue poll failed: %v", err)
		return
	}

	// The router sees every agent, with a busy flag: an agent that cannot take a
	// run this tick is unavailable, not unknown. Dropping it from the map here
	// is what used to write "is not a fleet agent" onto a ticket whose agent was
	// still working — which is every self-queueing sweep, seconds after it
	// creates its own next task.
	usage := history.UsageSince(time.Now())
	for name, a := range agents {
		u := usage[name]
		a.Unavailable = !runs.CanRun(a) || pick.OverBudget(a, u.Runs, u.Minutes)
		agents[name] = a
	}

	res := pick.Next(tasks, agents, settings.DefaultAgent)
	for _, t := range res.Unknown {
		name := pick.AssigneeFor(t, settings.DefaultAgent)
		note := fmt.Sprintf("fleet: assignee %q is not a fleet agent — fix the assignee or add agents/%s/AGENT.md.", name, name)
		if t.Assignee == "" {
			note = fmt.Sprintf("fleet: default_agent %q (fleet.yaml) is not a fleet agent.", name)
		}
		key := t.ID + "/" + name
		if reported[key] {
			continue
		}
		reported[key] = true
		log.Printf("%s: %s", t.ID, note)
		if err := src.Comment(t.ID, note); err != nil {
			log.Printf("%s: append note: %v", t.ID, err)
		}
	}
	for res.Task != nil {
		t, agent := *res.Task, res.Agent
		log.Printf("%s: starting (%s, agent %s)", t.ID, t.Title, agent.Name)
		go func() {
			if err := runs.Run(src, t, agent, history.TriggerPoll); err != nil {
				log.Printf("run %s: %v", t.ID, err)
			}
		}()
		// The started agent is spoken for; anything sharing its lock key is
		// too. Mark them unavailable (never delete — that would make pick call a
		// live agent unknown) and re-pick among what is left for this tick.
		for name, a := range agents {
			if runner.LockKey(a) == runner.LockKey(agent) {
				a.Unavailable = true
				agents[name] = a
			}
		}
		remaining := tasks[:0:0]
		for _, x := range tasks {
			if x.ID != t.ID {
				remaining = append(remaining, x)
			}
		}
		tasks = remaining
		res = pick.Next(tasks, agents, settings.DefaultAgent)
	}
}

// restart re-executes the daemon so a plugin upgrade takes effect without
// waiting for the Herdr server to be restarted.
func restart(release func(), runs *runner.Runner) {
	if runs.Busy() {
		return // let the in-flight run finish; we'll notice again next tick
	}
	exe, err := os.Executable()
	if err != nil {
		log.Printf("cannot locate the new binary: %v", err)
		return
	}
	log.Printf("binary changed, re-executing %s", exe)
	release() // the new process takes the lock
	if err := syscall.Exec(exe, os.Args, os.Environ()); err != nil {
		// Exec replaced nothing, and the lock is gone: without reclaiming it a
		// second daemon could now start and race this one for every task.
		log.Printf("re-exec failed, continuing with the old build: %v", err)
		if _, err := acquireLock(); err != nil {
			log.Printf("could not re-take the daemon lock: %v", err)
		}
	}
}

func binaryStamp() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	st, err := os.Stat(exe)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d-%d", st.ModTime().UnixNano(), st.Size())
}
