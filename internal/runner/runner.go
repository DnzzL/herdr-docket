// Package runner executes one fleet task end to end: claim it on the board,
// provision a workspace, hand the agent its prompt, and reconcile the board
// with what actually happened. The worker is strictly one-at-a-time — that is
// the fleet's concurrency model, not a limitation to engineer away.
package runner

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/DnzzL/herdr-fleet/internal/backlog"
	"github.com/DnzzL/herdr-fleet/internal/fleet"
	"github.com/DnzzL/herdr-fleet/internal/history"
	"github.com/DnzzL/herdr-fleet/internal/host"
	"github.com/DnzzL/herdr-fleet/internal/prompt"
)

// Board is the slice of the backlog client a run needs. The agent talks to
// the same backlog itself, through the CLI; this is only the claim and the
// safety net around it.
type Board interface {
	View(id string) (backlog.View, error)
	SetStatus(id, status string) error
	AppendNote(id, note string) error
}

// Runner runs tasks on a host, one at a time.
type Runner struct {
	host     host.Host
	board    Board
	fleetDir string
	busy     bool
	mu       sync.Mutex
}

// New returns a Runner working through h against the board.
func New(h host.Host, b Board, fleetDir string) *Runner {
	return &Runner{host: h, board: b, fleetDir: fleetDir}
}

// Default returns a Runner driving the real Herdr and the real backlog CLI.
func Default(fleetDir string) *Runner {
	return New(host.New(), backlog.New(fleetDir), fleetDir)
}

// Busy reports whether a task is mid-run.
func (r *Runner) Busy() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.busy
}

func (r *Runner) setBusy(v bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if v && r.busy {
		return false
	}
	r.busy = v
	return true
}

// Run executes the task synchronously. A task offered while another is in
// flight is refused, not queued — it stays To Do and the next tick sees it.
func (r *Runner) Run(t backlog.Task, a fleet.Agent, trigger history.Trigger) error {
	if !r.setBusy(true) {
		return fmt.Errorf("%s: a task is already in flight", t.ID)
	}
	defer r.setBusy(false)

	id := history.NewID(t.ID, "")
	v, err := r.board.View(t.ID)
	if err != nil {
		record(id, t.ID, trigger, history.StatusFailed, host.Session{}, err.Error())
		return err
	}
	if err := r.board.SetStatus(t.ID, backlog.StatusInProgress); err != nil {
		record(id, t.ID, trigger, history.StatusFailed, host.Session{}, err.Error())
		return err
	}
	record(id, t.ID, trigger, history.StatusScheduled, host.Session{}, "")

	spec := host.Spec{
		Name:      t.ID + " " + t.Title,
		Repo:      a.Workdir,
		Workspace: host.WorkspaceMode(a.Workspace),
		Agent:     a.Kind,
		Model:     a.Model,
		Prompt:    prompt.Assemble(a, v, r.fleetDir),
		MCPConfig: a.MCPConfig,
		AgentArgs: a.AgentArgs,
	}
	session, err := r.host.Provision(spec)
	if err != nil {
		r.reconcile(t.ID, err)
		record(id, t.ID, trigger, history.StatusFailed, session, err.Error())
		return err
	}
	record(id, t.ID, trigger, history.StatusRunning, session, "")

	err = r.host.Do(session, spec, time.Duration(a.TimeoutMinutes)*time.Minute)
	r.reconcile(t.ID, err)
	switch {
	case errors.Is(err, host.ErrCancelled):
		record(id, t.ID, trigger, history.StatusCancelled, session, err.Error())
	case err != nil:
		record(id, t.ID, trigger, history.StatusFailed, session, err.Error())
	default:
		record(id, t.ID, trigger, history.StatusDone, session, "")
	}
	if err == nil {
		// The run mechanics worked, but "the agent settled" is only a success
		// if it reported a verdict; reconcile turned silence into Failed.
		if after, verr := r.board.View(t.ID); verr == nil && after.Status == backlog.StatusFailed {
			return fmt.Errorf("%s: the agent settled without reporting a status", t.ID)
		}
	}
	return err
}

// reconcile makes the board tell the truth after a run: the agent's own
// verdict stands, but a task left "In Progress" gets the outcome the run
// mechanics imply. A cancelled run (workspace closed under it) goes Blocked —
// somebody decided, a human should say what happens next. Anything else that
// leaves the task unreported is Failed.
func (r *Runner) reconcile(taskID string, runErr error) {
	v, err := r.board.View(taskID)
	if err != nil {
		log.Printf("%s: cannot re-read the task after the run: %v", taskID, err)
		return
	}
	if v.Status != backlog.StatusInProgress {
		return // the agent reported; its verdict stands
	}
	status, note := backlog.StatusFailed, "fleet: the agent settled without reporting a status."
	switch {
	case errors.Is(runErr, host.ErrCancelled):
		status, note = backlog.StatusBlocked, "fleet: the run's workspace was closed — called off by a human."
	case runErr != nil:
		note = "fleet: run failed: " + runErr.Error()
	}
	if err := r.board.AppendNote(taskID, note); err != nil {
		log.Printf("%s: append note: %v", taskID, err)
	}
	if err := r.board.SetStatus(taskID, status); err != nil {
		log.Printf("%s: set status %s: %v", taskID, status, err)
	}
}

func record(id, task string, trigger history.Trigger, status history.Status, s host.Session, errText string) {
	err := history.Append(history.Record{
		RunID: id, Task: task, Trigger: trigger, Status: status,
		At: time.Now(), WorkspaceID: s.WorkspaceID, PaneID: s.PaneID, Error: errText,
	})
	if err != nil {
		log.Printf("history append failed: %v", err)
	}
}
