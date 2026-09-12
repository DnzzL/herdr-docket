// Package runner executes one fleet task end to end: claim it on the board,
// provision a workspace, hand the agent its prompt, and reconcile the board
// with what actually happened. The worker is strictly one-at-a-time — that is
// the fleet's concurrency model, not a limitation to engineer away.
package runner

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/DnzzL/herdr-fleet/internal/fleet"
	"github.com/DnzzL/herdr-fleet/internal/history"
	"github.com/DnzzL/herdr-fleet/internal/host"
	"github.com/DnzzL/herdr-fleet/internal/hostpath"
	"github.com/DnzzL/herdr-fleet/internal/prompt"
	"github.com/DnzzL/herdr-fleet/internal/work"
)

// Runner runs tasks on a host, one at a time *per checkout*. The unit of
// mutual exclusion is what a run mutates: an agent in worktree mode gets its
// own checkout per run, so two such agents (or two runs of different agents)
// can fly in parallel; agents in root mode share their working copy, so runs
// keyed to the same root workdir are serialized. The claim holds across
// processes — the daemon, `herdr-fleet run` and the board each have their own
// Runner, and an OS file lock per key in the state dir keeps a manual run
// from racing the daemon into the same agent or the same checkout.
type Runner struct {
	host     host.Host
	board    work.Source
	fleetDir string
	busy     map[string]*os.File
	mu       sync.Mutex
}

// New returns a Runner working through h against the board.
func New(h host.Host, b work.Source, fleetDir string) *Runner {
	return &Runner{host: h, board: b, fleetDir: fleetDir, busy: map[string]*os.File{}}
}

// LockKey names what a run of this agent would mutate: the shared checkout
// for root mode, the agent itself for worktree mode (each of its runs gets a
// fresh checkout — the agent's serial identity is all there is to protect).
func LockKey(a fleet.Agent) string {
	if a.Workspace == "root" {
		return "root-" + sanitize(a.Workdir)
	}
	return "agent-" + sanitize(a.Name)
}

// CanRun reports whether this process could start a run for the agent right
// now. Advisory — the daemon uses it to pick work without log-spamming
// refusals; Run still makes the authoritative claim.
func (r *Runner) CanRun(a fleet.Agent) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, taken := r.busy[LockKey(a)]
	return !taken
}

// Busy reports whether any task is mid-run in this process.
func (r *Runner) Busy() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.busy) > 0
}

// acquire claims the run slot for one key: the in-process entry plus a
// non-blocking flock on <state>/run-<key>.lock. flock dies with the process,
// so a crashed run never leaves a stale claim behind.
func (r *Runner) acquire(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, taken := r.busy[key]; taken {
		return false
	}
	if err := os.MkdirAll(hostpath.StateDir(), 0o755); err != nil {
		log.Printf("state dir: %v", err)
		return false
	}
	f, err := os.OpenFile(filepath.Join(hostpath.StateDir(), "run-"+key+".lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		log.Printf("run lock: %v", err)
		return false
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return false
	}
	r.busy[key] = f
	return true
}

func (r *Runner) release(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if f, ok := r.busy[key]; ok {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
		delete(r.busy, key)
	}
}

// sanitize keeps a lock-file name safe: lowercase alphanumerics and dashes.
func sanitize(s string) string {
	out := make([]rune, 0, len(s))
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			out = append(out, c)
		case c >= 'A' && c <= 'Z':
			out = append(out, c+32)
		default:
			out = append(out, '-')
		}
	}
	return string(out)
}

// Run executes the task synchronously. A task offered while its agent (or,
// in root mode, its checkout) is in flight is refused, not queued — it stays
// open and the next tick sees it. The lock is the whole claim: a task routes
// to exactly one agent, so holding that agent's slot is holding the task.
func (r *Runner) Run(t work.Task, a fleet.Agent, trigger history.Trigger) error {
	key := LockKey(a)
	if !r.acquire(key) {
		return fmt.Errorf("%s: a run is already in flight for %s", t.ID, key)
	}
	defer r.release(key)

	rec := recorder{id: history.NewID(t.ID), task: t.ID, trigger: trigger}
	v, err := r.board.Get(t.ID)
	if err != nil {
		rec.record(history.StatusFailed, host.Session{}, err.Error())
		return err
	}
	r.claim(t.ID)
	rec.record(history.StatusScheduled, host.Session{}, "")

	spec := host.Spec{
		Name:      t.ID + " " + t.Title,
		RunTag:    host.Tag(rec.id),
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
		rec.record(history.StatusFailed, session, err.Error())
		return err
	}
	rec.record(history.StatusRunning, session, "")

	err = r.host.Do(session, spec, time.Duration(a.TimeoutMinutes)*time.Minute)
	final := r.reconcile(t.ID, err)
	r.cleanup(t.ID, session, final)
	switch {
	case errors.Is(err, host.ErrCancelled):
		rec.record(history.StatusCancelled, session, err.Error())
	case err != nil:
		rec.record(history.StatusFailed, session, err.Error())
	case final == work.Failed:
		// The run mechanics worked, but "the agent settled" is only a success
		// if it reported a verdict; reconcile turned silence into Failed and
		// the history must say the same.
		err = fmt.Errorf("%s: the agent settled without reporting a verdict", t.ID)
		rec.record(history.StatusFailed, session, err.Error())
	default:
		rec.record(history.StatusDone, session, "")
	}
	return err
}

// claim shows the task as being worked on, where the backend can say so. The
// write is display only and best-effort: a binary backend has no such state,
// and the run lock — not this — is what keeps two runs apart, so a backend
// that cannot say it is not a reason to refuse the run.
func (r *Runner) claim(id string) {
	p, ok := r.board.(work.Phaser)
	if !ok {
		return
	}
	if err := p.SetPhase(id, work.PhaseInProgress); err != nil {
		log.Printf("%s: mark as running: %v", id, err)
	}
}

// reconcile makes the task tell the truth after a run and returns the verdict
// it ended on. The agent's own verdict stands: if the agent closed the task,
// there is nothing to do. An task the agent left open gets the verdict the run
// mechanics imply — a cancelled run (workspace closed under it) goes Blocked,
// because somebody decided and a human should say what happens next; anything
// else that leaves the task unreported is Failed.
func (r *Runner) reconcile(taskID string, runErr error) work.Verdict {
	v, err := r.board.Get(taskID)
	if err != nil {
		log.Printf("%s: cannot re-read the task after the run: %v", taskID, err)
		return ""
	}
	if !v.Open {
		return v.Verdict // the agent reported; its verdict stands
	}
	verdict, note := work.Failed, "fleet: the agent settled without reporting a verdict."
	switch {
	case errors.Is(runErr, host.ErrCancelled):
		verdict, note = work.Blocked, "fleet: the run's workspace was closed — called off by a human."
	case runErr != nil:
		note = "fleet: run failed: " + runErr.Error()
	}
	if err := r.board.Comment(taskID, note); err != nil {
		log.Printf("%s: append note: %v", taskID, err)
	}
	if err := r.board.Close(taskID, verdict); err != nil {
		log.Printf("%s: close as %s: %v", taskID, verdict, err)
	}
	return verdict
}

// cleanup decides what happens to the run's workspace. A task that ended Done
// left nothing to look at — the work is in the repo and the notes — so the
// workspace is torn down. Anything else keeps its workspace open as the place
// to resume: the board's enter-jump lands there, and the note names it for
// anyone reading the ticket instead of the board.
func (r *Runner) cleanup(taskID string, s host.Session, final work.Verdict) {
	if s.WorkspaceID == "" {
		return
	}
	if final == work.Done {
		if err := r.host.Close(s); err != nil {
			log.Printf("%s: close workspace %s: %v", taskID, s.WorkspaceID, err)
		}
		return
	}
	note := fmt.Sprintf("fleet: the run's workspace %s (pane %s) is left open — jump in to resume.", s.WorkspaceID, s.PaneID)
	if err := r.board.Comment(taskID, note); err != nil {
		log.Printf("%s: append note: %v", taskID, err)
	}
}

// recorder pins one run's identity so every transition is logged the same way.
type recorder struct {
	id      string
	task    string
	trigger history.Trigger
}

func (r recorder) record(status history.Status, s host.Session, errText string) {
	err := history.Append(history.Record{
		RunID: r.id, Task: r.task, Trigger: r.trigger, Status: status,
		At: time.Now(), WorkspaceID: s.WorkspaceID, PaneID: s.PaneID, Error: errText,
	})
	if err != nil {
		log.Printf("history append failed: %v", err)
	}
}
