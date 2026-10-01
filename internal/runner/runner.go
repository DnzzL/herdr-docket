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

	"github.com/DnzzL/herdr-docket/internal/fleet"
	"github.com/DnzzL/herdr-docket/internal/history"
	"github.com/DnzzL/herdr-docket/internal/host"
	"github.com/DnzzL/herdr-docket/internal/hostpath"
	"github.com/DnzzL/herdr-docket/internal/prompt"
	"github.com/DnzzL/herdr-docket/internal/work"
)

// Runner runs tasks on a host, one at a time *per checkout*. The unit of
// mutual exclusion is what a run mutates: an agent in worktree mode gets its
// own checkout per run, so two such agents (or two runs of different agents)
// can fly in parallel; agents in root mode share their working copy, so runs
// keyed to the same root workdir are serialized. The claim holds across
// processes — the daemon, `herdr-docket run` and the board each have their own
// Runner, and an OS file lock per key in the state dir keeps a manual run
// from racing the daemon into the same agent or the same checkout.
type Runner struct {
	host host.Host
	// settings is held for what a prompt needs to know about the queue a task
	// came from — the status words it writes. fleetDir is settings.Dir, kept
	// separate because every other use of it is just a path.
	settings fleet.Settings
	fleetDir string
	busy     map[string]*os.File
	mu       sync.Mutex
}

// New returns a Runner working through h. The queue is not held here: it is
// passed to Run, so the task a run reads and the queue it writes back to are
// the same value by construction, even as the daemon rebuilds its source
// between ticks.
func New(h host.Host, settings fleet.Settings) *Runner {
	return &Runner{host: h, settings: settings, fleetDir: settings.Dir, busy: map[string]*os.File{}}
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
func (r *Runner) Run(src work.Source, t work.Task, a fleet.Agent, trigger history.Trigger) error {
	key := LockKey(a)
	if !r.acquire(key) {
		return fmt.Errorf("%s: a run is already in flight for %s", t.ID, key)
	}
	defer r.release(key)

	rec := recorder{id: history.NewID(t.ID), task: t.ID, agent: a.Name, trigger: trigger, started: time.Now()}
	var errInspect error
	v, err := src.Get(t.ID)
	if err != nil {
		rec.record(history.StatusFailed, host.Session{}, err.Error())
		r.notifyFailed(t, err.Error())
		return err
	}
	r.claim(src, t.ID)
	rec.record(history.StatusScheduled, host.Session{}, "")

	spec := host.Spec{
		Name:      t.ID + " " + t.Title,
		RunTag:    host.Tag(rec.id),
		Repo:      a.Workdir,
		Workspace: host.WorkspaceMode(a.Workspace),
		Agent:     a.Kind,
		Model:     a.Model,
		Prompt:    prompt.Assemble(a, v, r.fleetDir, r.settings.WordsFor(t.ID)),
		MCPConfig: a.MCPConfig,
		AgentArgs: a.AgentArgs,
	}
	session, err := r.host.Provision(spec)
	if err != nil {
		v, _, _ := r.reconcile(src, t.ID, a.Name, err)
		// No run happened past provisioning, so there is no delivery to
		// report beside the failure.
		rec.close(history.StatusFailed, session, v, host.Delivery{}, false, err.Error())
		r.notifyFailed(t, err.Error())
		return err
	}
	rec.record(history.StatusRunning, session, "")

	err = r.host.Do(session, spec, time.Duration(a.TimeoutMinutes)*time.Minute)
	// Read what the run produced while its workspace still exists: a
	// worktree-mode session has a branch, and the facts about it only survive
	// until cleanup tears the worktree down. A root-mode session has no
	// branch and is never asked.
	var d host.Delivery
	if session.Branch != "" {
		d, errInspect = r.host.Inspect(session)
		if errInspect != nil {
			log.Printf("inspect delivery of %s: %v", t.ID, errInspect)
		}
	}
	final, handedOn, reported := r.reconcile(src, t.ID, a.Name, err)
	final, keep := r.guardDelivery(src, t.ID, session, final, handedOn, d, session.Branch != "" && errInspect != nil)
	r.cleanup(src, t.ID, session, final, handedOn, keep)
	// Uncommitted is the fact the guard acted on: a worktree about to be torn
	// down holding changes nobody committed. It rides on the record whatever
	// the run ends as, because it is what the record qualifies.
	uncommitted := session.Branch != "" && errInspect == nil && d.Dirty
	switch {
	case errors.Is(err, host.ErrCancelled):
		rec.close(history.StatusCancelled, session, final, d, uncommitted, err.Error())
	case err != nil:
		rec.close(history.StatusFailed, session, final, d, uncommitted, err.Error())
		r.notifyFailed(t, err.Error())
	case final == work.Failed:
		// A task that ended Failed is a failed run either way, but why it
		// failed is the difference between a broken agent and a working one.
		// An agent that judged the ticket and reported it did its job — a
		// reviewer refusing to merge reaches here every time — and must not be
		// filed under the sentence reserved for one that closed nothing.
		if reported {
			err = fmt.Errorf("%s: the agent reported %s", t.ID, final)
		} else {
			err = fmt.Errorf("%s: the agent settled without reporting a verdict", t.ID)
		}
		rec.close(history.StatusFailed, session, final, d, uncommitted, err.Error())
		r.notifyFailed(t, err.Error())
	default:
		rec.close(history.StatusDone, session, final, d, uncommitted, "")
	}
	return err
}

func (r *Runner) notifyFailed(t work.Task, reason string) {
	if err := r.host.Notify("fleet: run failed — "+t.ID, reason); err != nil {
		log.Printf("%s: notify: %v", t.ID, err)
	}
}

// claim shows the task as being worked on, where the backend can say so. The
// write is display only and best-effort: a binary backend has no such state,
// and the run lock — not this — is what keeps two runs apart, so a backend
// that cannot say it is not a reason to refuse the run.
func (r *Runner) claim(src work.Source, id string) {
	p, ok := src.(work.Phaser)
	if !ok {
		return
	}
	if err := p.SetPhase(id, work.PhaseInProgress); err != nil {
		log.Printf("%s: mark as running: %v", id, err)
	}
}

// reconcile makes the task tell the truth after a run and returns the verdict
// it ended on, whether the agent handed the task to somebody else, and whether
// that verdict is the agent's own word or one the fleet had to invent. The
// agent's own verdict stands: if the agent closed the task, there is nothing
// to do. A task the agent left open gets the verdict the run mechanics imply
// — a cancelled run (workspace closed under it) goes Blocked, because somebody
// decided and a human should say what happens next; anything else that leaves
// the task unreported is Failed.
//
// reported is what tells those two Faileds apart afterwards. Without it a
// reviewer that refuses to merge and says so — its whole job — is recorded in
// the same words as an agent that closed nothing, and a fleet cannot be read.
func (r *Runner) reconcile(src work.Source, taskID, agent string, runErr error) (verdict work.Verdict, handedOn, reported bool) {
	v, err := src.Get(taskID)
	if err != nil {
		log.Printf("%s: cannot re-read the task after the run: %v", taskID, err)
		return "", false, false
	}
	if !v.Open {
		return v.Verdict, false, true // the agent reported; its verdict stands
	}
	// A task now assigned to somebody else was handed on, not abandoned: it is
	// open on purpose, with a new owner and its whole history in one place.
	// Closing it here would undo the handoff and the next tick would never
	// route it. Only on a clean run — a task reassigned by an agent that then
	// crashed is still an unreported task.
	if runErr == nil && v.Assignee != "" && v.Assignee != agent {
		return "", true, true
	}
	verdict, note := work.Failed, "fleet: the agent settled without reporting a verdict."
	switch {
	case errors.Is(runErr, host.ErrCancelled):
		verdict, note = work.Blocked, "fleet: the run's workspace was closed — called off by a human."
	case runErr != nil:
		note = "fleet: run failed: " + runErr.Error()
	}
	if err := src.Comment(taskID, note); err != nil {
		log.Printf("%s: append note: %v", taskID, err)
	}
	if err := src.Close(taskID, verdict); err != nil {
		log.Printf("%s: close as %s: %v", taskID, verdict, err)
	}
	return verdict, false, false
}

// guardDelivery is TASK-23's check, made before the one moment where the
// evidence disappears: a disposable worktree about to be torn down holding
// changes nobody committed. It returns the verdict the run should end on and
// whether the workspace must be kept whatever cleanup thinks.
//
// Proven loss of the run's own report: the task is moved to the human-decides
// column and told why — the agent said done, the worktree disagreed, and the
// fleet's job is to say so where a human will read it. A queue that refuses a
// second close keeps its verdict and gets the same note: the report is still
// doubted, the queue simply will not carry the correction.
//
// An unverifiable delivery keeps the workspace and changes nothing: blindness
// is not evidence. Root-mode runs never reach here — they have no branch, and
// their edits are the repo's own state by design.
func (r *Runner) guardDelivery(src work.Source, taskID string, s host.Session, final work.Verdict, handedOn bool, d host.Delivery, unverifiable bool) (work.Verdict, bool) {
	if s.WorkspaceID == "" || s.Branch == "" || (final != work.Done && !handedOn) {
		return final, false
	}
	workspace := fmt.Sprintf("workspace %s (pane %s)", s.WorkspaceID, s.PaneID)
	if unverifiable {
		note := fmt.Sprintf("fleet: this run's delivery could not be read — %s is kept, unverified. The verdict stands.", workspace)
		if err := src.Comment(taskID, note); err != nil {
			log.Printf("%s: append note: %v", taskID, err)
		}
		return final, true
	}
	if !d.Dirty {
		return final, false
	}
	verdict := final
	after := "the verdict stands as reported"
	if final == work.Done {
		if err := src.Close(taskID, work.Blocked); err != nil {
			log.Printf("%s: re-close as blocked: %v", taskID, err)
		} else {
			verdict, after = work.Blocked, "the task is now blocked — a human decides"
		}
	}
	note := fmt.Sprintf("fleet: the run's worktree still holds uncommitted changes — %s is kept, and %s.",
		workspace, after)
	if err := src.Comment(taskID, note); err != nil {
		log.Printf("%s: append note: %v", taskID, err)
	}
	return verdict, true
}

// cleanup decides what happens to the run's workspace. A task that ended Done
// left nothing to look at — the work is in the repo and the notes — so the
// workspace is torn down, and a task handed to another agent is the same: the
// next agent opens its own. Anything else keeps its workspace open as the
// place to resume: the board's enter-jump lands there, and the note names it
// for anyone reading the ticket instead of the board. keep is the guard's
// answer: a workspace it kept has already said why on the task, so it is left
// alone here rather than described twice.
func (r *Runner) cleanup(src work.Source, taskID string, s host.Session, final work.Verdict, handedOn, keep bool) {
	if s.WorkspaceID == "" || keep {
		return
	}
	if final == work.Done || handedOn {
		if err := r.host.Close(s); err != nil {
			log.Printf("%s: close workspace %s: %v", taskID, s.WorkspaceID, err)
		}
		return
	}
	note := fmt.Sprintf("fleet: the run's workspace %s (pane %s) is left open — jump in to resume.", s.WorkspaceID, s.PaneID)
	if err := src.Comment(taskID, note); err != nil {
		log.Printf("%s: append note: %v", taskID, err)
	}
}

// recorder pins one run's identity so every transition is logged the same way.
type recorder struct {
	id      string
	task    string
	agent   string
	trigger history.Trigger
	started time.Time
}

func (r recorder) record(status history.Status, s host.Session, errText string) {
	r.appendWith(status, s, "", 0, errText, host.Delivery{}, false, "")
}

// close is record for the run's final transition: it is where the duration,
// the verdict and the delivery are known, so it is where they are written
// down. The pull request the agent stamped mid-run is carried forward here —
// the reader keeps only the latest record per run, so a closer that did not
// re-read it would drop the url behind the verdict. Append-only stays
// append-only: a record from before these fields existed reads as zero and
// empty.
func (r recorder) close(status history.Status, s host.Session, verdict work.Verdict, d host.Delivery, unverified bool, errText string) {
	pr, err := history.PullRequestFor(r.id)
	if err != nil {
		log.Printf("carry pull request for %s: %v", r.id, err)
	}
	r.appendWith(status, s, string(verdict), int(time.Since(r.started).Seconds()), errText, d, unverified, pr)
}

// appendWith is the one writer: every transition of a run is this call with
// the fields known at that moment, so the record shape lives in exactly one
// place and an in-flight run can only ever differ by what its caller passed.
// Branch travels on every record that has a session — the provision names it
// once and the rest of the run only ever reports it back.
func (r recorder) appendWith(status history.Status, s host.Session, verdict string, seconds int, errText string, d host.Delivery, unverified bool, pullRequest string) {
	err := history.Append(history.Record{
		RunID: r.id, Task: r.task, Agent: r.agent, Trigger: r.trigger, Status: status,
		At: time.Now(), WorkspaceID: s.WorkspaceID, PaneID: s.PaneID, TabID: s.TabID, Branch: s.Branch,
		DurationSeconds: seconds, Verdict: verdict, Error: errText,
		Commits: d.Commits, Uncommitted: unverified, PullRequest: pullRequest,
	})
	if err != nil {
		log.Printf("history append failed: %v", err)
	}
}
