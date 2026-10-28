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
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/DnzzL/herdr-docket/internal/fleet"
	"github.com/DnzzL/herdr-docket/internal/gate"
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
// Runner, and an OS file lock in the state dir keeps a manual run from
// racing the daemon into the same agent, the same checkout — or the same
// task: the agent/checkout key protects the workspace, the per-task key
// (`run-taskid-<id>.lock`) protects the task itself, because a `fleet.yaml`
// edit re-routes work under a live run.
type Runner struct {
	host host.Host
	// settings is held for what a prompt needs to know about the queue a task
	// came from — the status words it writes. fleetDir is settings.Dir, kept
	// separate because every other use of it is just a path.
	settings fleet.Settings
	fleetDir string
	busy     map[string]*os.File
	// tasks is the task ids this process is running, by id: the lock keys
	// are sanitized and cannot be read back.
	tasks map[string]bool
	mu    sync.Mutex
	// The pipeline's reach beyond the host (ADR 0013), swappable in tests:
	// the forge the gate reads and merges through, the clock it waits on,
	// the roster the verifier is found in.
	forge  gate.Forge
	sleep  func(time.Duration)
	agents func() map[string]fleet.Agent
	// settle's own cadence: it reads the forge every settleEvery rather than
	// every tick, so its clock is guarded apart from the run slots above.
	settleMu   sync.Mutex
	lastSettle time.Time
	now        func() time.Time
}

// Option configures a Runner at construction. Production passes none; the
// options exist for tests that must answer the pipeline's reach beyond the
// host (ADR 0013) themselves.
type Option func(*Runner)

// WithForge answers this Runner's forge reads with f instead of the real
// forge. Construction-time on purpose: the forge is read from run and settle
// goroutines, so a setter on a live Runner would race them — a test builds
// its Runner with the fake already in it.
func WithForge(f gate.Forge) Option {
	return func(r *Runner) { r.forge = f }
}

// New returns a Runner working through h. The queue is not held here: it is
// passed to Run, so the task a run reads and the queue it writes back to are
// the same value by construction, even as the daemon rebuilds its source
// between ticks.
func New(h host.Host, settings fleet.Settings, opts ...Option) *Runner {
	r := &Runner{
		host: h, settings: settings, fleetDir: settings.Dir, busy: map[string]*os.File{}, tasks: map[string]bool{},
		forge: gate.GH{}, sleep: time.Sleep, now: time.Now, agents: func() map[string]fleet.Agent {
			a, _ := fleet.LoadAgents(settings.Dir)
			return a
		},
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Running reports whether this process holds the task's run — for the whole
// pipeline, not just one stage, so the daemon never hands a task back to its
// author while the verifier has it.
func (r *Runner) Running(taskID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, held := r.busy[taskKey(taskID)]
	return held
}

// InFlight is the task ids this process is running, whole pipelines
// included — what a run cap counts.
func (r *Runner) InFlight() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.tasks))
	for id := range r.tasks {
		out = append(out, id)
	}
	return out
}

func taskKey(id string) string { return "taskid-" + sanitize(id) }

// key keeps a run off the workspace another run may be mutating: the
// agent itself in worktree mode (each run gets a fresh checkout — the
// agent's serial identity is all there is to protect), the shared checkout
// for root mode. It says nothing about which task a run claims — the
// per-task flock in Run does that.
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

// acquire claims the run's slots, one per key: the in-process entry plus a
// non-blocking flock on <state>/run-<key>.lock for each. All or nothing, so a
// run that cannot take every key takes none; the taken key is named back so
// the refusal can say what was busy. flock dies with the process, so a
// crashed run never leaves a stale claim behind — and a settled run removes
// the lock file itself, so the state dir holds lock files only for runs in
// flight (the sweep takes the ones that settled before removal existed).
func (r *Runner) acquire(keys ...string) (taken string, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, key := range keys {
		if _, exist := r.busy[key]; exist {
			return key, false
		}
	}
	if err := os.MkdirAll(hostpath.StateDir(), 0o755); err != nil {
		log.Printf("state dir: %v", err)
		return "", false
	}
	held := map[string]*os.File{}
	for _, key := range keys {
		f, err := claim(key)
		switch {
		case err == nil:
			held[key] = f
			r.busy[key] = f
		case errors.Is(err, errBusy):
			releaseLocked(r.busy, held) // give back what this attempt took
			return key, false
		default:
			log.Printf("run lock: %v", err)
			releaseLocked(r.busy, held)
			return "", false
		}
	}
	return "", true
}

// errBusy is claim's answer when the key's flock is already held — by a run
// of this process or of any other; the flock is what tells them apart, not
// this process's bookkeeping.
var errBusy = errors.New("the lock is held by another run")

// claim takes one key's flock, and only while the file it locked is still the
// file at the path. drop removes the file while it still holds the flock, so
// a descriptor opened just before that unlink must not walk away holding a
// claim on a file nothing else can find: it loses and claims again on the
// fresh file, which is the one every new opener sees. Without this check the
// unlink-while-held ordering would still leave a instant where two runs hold
// flocks on the same task — one on the unlinked file, one on its replacement.
func claim(key string) (*os.File, error) {
	path := lockPath(key)
	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
		if err != nil {
			return nil, err
		}
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			f.Close()
			return nil, errBusy
		}
		if sameFile(f, path) {
			return f, nil
		}
		f.Close() // the file was replaced under us; claim the one at the path
	}
	return nil, errBusy
}

// sameFile reports whether f's file is still the one at path — the flock
// protects a run only while the file it locked is the file every new opener
// finds.
func sameFile(f *os.File, path string) bool {
	fd, errF := f.Stat()
	p, errP := os.Stat(path)
	if errF != nil || errP != nil {
		return false
	}
	a, isA := fd.Sys().(*syscall.Stat_t)
	b, isB := p.Sys().(*syscall.Stat_t)
	return isA && isB && a.Dev == b.Dev && a.Ino == b.Ino
}

func lockPath(key string) string { return filepath.Join(hostpath.StateDir(), "run-"+key+".lock") }

// releaseLocked gives back the claims one acquire attempt took, file first:
// only ever called with r.mu held.
func releaseLocked(busy map[string]*os.File, held map[string]*os.File) {
	for key, f := range held {
		drop(lockPath(key), f)
		delete(busy, key)
	}
}

func (r *Runner) release(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if f, ok := r.busy[key]; ok {
		delete(r.busy, key)
		drop(lockPath(key), f)
	}
}

// drop gives back one held claim and retires its file: the unlink comes
// first, while the flock still excludes other runs — between the unlink and
// the close a new opener may already create and take the next file, and that
// is correct (this run has settled); what must never happen is a second run
// taking this file's claim, which the held flock refuses and claim's inode
// check catches from the other side. An unlink error leaves a file whose
// flock is dead — a leftover, which the next claim/release pair or the
// daemon's sweep takes.
func drop(path string, f *os.File) {
	os.Remove(path)
	syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	f.Close()
}

// SweepLocks removes the run lock files no run holds: leftovers of runs that
// settled before removal existed, and of crashes. A live claim — the
// daemon's or a manual `herdr-docket run`'s, its flock held across
// processes — passes its flock test and keeps its file. Called once, when
// the daemon starts: from then on a settled run drops its own file, and a
// crash's leftover goes the next time that key is claimed and released.
func SweepLocks() {
	dir := hostpath.StateDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "run-") || !strings.HasSuffix(name, ".lock") {
			continue
		}
		path := filepath.Join(dir, name)
		f, err := os.OpenFile(path, os.O_RDWR, 0o644)
		if err != nil {
			continue
		}
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			f.Close() // a live run holds it — a claim, not a leftover
			continue
		}
		drop(path, f)
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

// Run executes the task synchronously. Two locks are taken before anything
// happens, and a run that cannot take both is refused, not queued — the task
// stays open and the next tick sees it. The per-agent/checkout key keeps
// two runs off the same workspace — one agent's serial identity, root runs'
// shared checkouts. The per-task key (run-taskid-<id>.lock) keeps two runs off
// the same task however the routing disagrees: agents hold different keys,
// and a `fleet.yaml` edit re-reads the default routing every tick, so an open
// task can present itself to a second agent while the first is mid-run.
//
// In a queue with a verifier, an author's run that delivers a pull request goes
// on to the pipeline (ADR 0013) under the same task lock; the author's own
// slot is given back first, so the verifier can share its checkout.
func (r *Runner) Run(src work.Source, t work.Task, a fleet.Agent, trigger history.Trigger) error {
	key := LockKey(a)
	tk := taskKey(t.ID)
	taken, ok := r.acquire(key, tk)
	if !ok {
		if taken == tk {
			return fmt.Errorf("%s: a run is already in flight", t.ID)
		}
		return fmt.Errorf("%s: a run is already in flight for %s", t.ID, key)
	}
	defer r.release(tk)
	r.mu.Lock()
	r.tasks[t.ID] = true
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.tasks, t.ID)
		r.mu.Unlock()
	}()
	res, err := r.attempt(src, t, a, trigger, "")
	r.release(key)
	if err != nil || !res.delivered {
		return err
	}
	return r.pipeline(src, t, a, res.pr)
}

// outcome is what one attempt left for the pipeline: whether the agent
// delivered — an author's PR, a verifier's verdict — and what.
type outcome struct {
	delivered      bool
	pr             string
	verdict, patch string
}

// attempt is one agent's run on the task, start to reconcile: the author's
// first run, a rework, or a verifier's run when pr names what it judges.
func (r *Runner) attempt(src work.Source, t work.Task, a fleet.Agent, trigger history.Trigger, pr string) (outcome, error) {
	verifying := pr != "" && trigger == history.TriggerVerify
	piped := r.settings.PipelineFor(t.ID).Verifier != ""
	rec := recorder{id: history.NewID(t.ID), task: t.ID, agent: a.Name, trigger: trigger, started: time.Now()}
	var errInspect error
	v, err := src.Get(t.ID)
	if err != nil {
		rec.record(history.StatusFailed, host.Session{}, err.Error())
		r.notifyFailed(t, err.Error())
		return outcome{}, err
	}
	r.claim(src, v)
	words := r.settings.WordsFor(t.ID)
	text := prompt.Assemble(a, v, r.fleetDir, words)
	switch {
	case verifying:
		text = prompt.Verify(a, v, pr, r.fleetDir)
	case piped:
		text = prompt.Deliver(a, v, pr, r.fleetDir, words)
	}
	rec.record(history.StatusScheduled, host.Session{}, "")

	spec := host.Spec{
		Name:      t.ID + " " + t.Title,
		RunTag:    host.Tag(rec.id),
		Repo:      a.Workdir,
		Workspace: host.WorkspaceMode(a.Workspace),
		// TASK-45: the branch a run cuts from is the queue's fact, asked through
		// the port's optional capability. No answer — a hosted queue, a repo with
		// no remote — inherits the checkout's HEAD, as before; the record still
		// names the commit either way (the host reads it at provision time).
		Base:      work.BaseBranchOf(src, t.ID),
		Agent:     a.Kind,
		Model:     a.Model,
		Prompt:    prompt.Runnable(text),
		MCPConfig: a.MCPConfig,
		AgentArgs: a.AgentArgs,
	}
	// A verify run is pinned to the PR's head commit (TASK-65): the forge is
	// asked for it here, and the host cuts the run's own worktree at that
	// commit — the agent fetches and checks out nothing, and no tab on the
	// project's primary checkout is reachable. A head that cannot be read is
	// a run that cannot start: the fleet fails it with the reason rather than
	// judging some other commit or falling back onto the checkout.
	var session host.Session
	if verifying {
		pulled, ferr := r.forge.PR(pr)
		switch {
		case ferr != nil:
			err = fmt.Errorf("the pull request could not be read: %w", ferr)
		case pulled.HeadSHA == "":
			err = errors.New("the pull request names no head commit")
		default:
			spec.Head = pulled.HeadSHA
		}
	}
	if err == nil {
		session, err = r.host.Provision(spec)
	}
	if err != nil {
		v, _, _ := r.reconcile(src, t.ID, a.Name, err, false)
		// No run happened past provisioning, so there is no delivery to
		// report beside the failure.
		rec.close(history.StatusFailed, session, v, host.Delivery{}, false, err.Error())
		r.notifyFailed(t, err.Error())
		return outcome{}, err
	}
	rec.record(history.StatusRunning, session, "")

	timeout := time.Duration(a.TimeoutMinutes) * time.Minute
	err = r.host.Do(session, spec, timeout)
	if errors.Is(err, host.ErrTimedOut) {
		// The clock ran out with the agent still working. The deadline is a
		// mark, not a drop: record it — a delivery from here on is late and
		// the log must be able to say so — then keep the agent and listen one
		// bounded window more (ADR 0014).
		rec.timedOut = true
		rec.record(history.StatusTimedOut, session, "")
		err = r.host.Settle(session, lateWindow(a))
	}
	// Read what the run produced while its workspace still exists: a
	// worktree-mode session has a branch, and the facts about it only survive
	// until cleanup tears the worktree down. A root-mode session has no
	// branch and is never asked, and neither is a verify session (TASK-65):
	// its report is the verdict in the queue, and the artifacts of a judge's
	// test run in the worktree would read as a delivery nobody committed.
	var d host.Delivery
	if session.Branch != "" && !session.Verify {
		d, errInspect = r.host.Inspect(session)
		if errInspect != nil {
			log.Printf("inspect delivery of %s: %v", t.ID, errInspect)
		}
	}
	// What the agent delivered through the CLI, read before reconcile: a
	// delivery keeps the task open on purpose, like a handoff. Read whatever
	// this runner believes about the queue — the CLI decides delivery from
	// fleet.yaml as it is now, and a PR left on an open task is a delivery
	// either way, never a silent run to close Failed.
	var out outcome
	if err == nil {
		if verifying {
			out.verdict, out.patch, _ = history.VerificationFor(rec.id)
			out.delivered = out.verdict != ""
		} else {
			out.pr, _ = history.PullRequestFor(rec.id)
			out.delivered = out.pr != ""
		}
	}
	// The agent never settled: the fleet gives it up. The agent goes before
	// the report — nothing may keep working on a task the fleet is closing —
	// so the note the task is about to get can tell the truth about where its
	// workspace ended (ADR 0014). What the worktree held rides on the record
	// and in the notification: destroyed and said, never silently.
	gaveUp := errors.Is(err, host.ErrTimedOut)
	if gaveUp {
		place := fmt.Sprintf("workspace %s (pane %s)", session.WorkspaceID, session.PaneID)
		if session.TabID != "" {
			place = fmt.Sprintf("tab on workspace %s (pane %s)", session.WorkspaceID, session.PaneID)
		}
		ending := fmt.Sprintf("the run's %s was closed so nothing keeps working on a closed task", place)
		if closeErr := r.host.Close(session); closeErr != nil {
			log.Printf("close %s to end the agent: %v", place, closeErr)
			ending = fmt.Sprintf("the run's %s could not be closed: %v — close it by hand", place, closeErr)
		}
		if d.Dirty {
			ending += ", and it held uncommitted changes nobody committed"
		}
		err = fmt.Errorf("%s: the agent never settled — %s past its timeout, %s", t.ID, lateWindow(a), ending)
	}
	final, handedOn, reported := r.reconcile(src, t.ID, a.Name, err, out.delivered)
	out.delivered = out.delivered && handedOn
	// The guard exists to keep a workspace a run left behind. A give-up
	// closed its own on purpose and said why on the task and in the
	// notification, so there is nothing left to keep — or to doubt.
	keep := false
	if !gaveUp {
		final, keep = r.guardDelivery(src, t.ID, session, final, handedOn, d, session.Branch != "" && errInspect != nil)
	}
	r.cleanup(src, t.ID, session, final, handedOn, keep, gaveUp)
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
	// A delivery the guard re-closed (uncommitted work left behind) is no
	// longer one: the task is a human's now.
	if final == work.Blocked {
		out.delivered = false
	}
	return out, err
}

func (r *Runner) notifyFailed(t work.Task, reason string) {
	// A failure on top of a pull request the task still has out is the
	// harder stop: the PR stays open and only a human can move it. It says
	// so; without one there is no handover, and the failure is the whole
	// message.
	pr, err := history.TaskPullRequest(t.ID)
	if err != nil {
		log.Printf("%s: read the task's pull request: %v", t.ID, err)
	}
	if pr != "" {
		r.mergeNeeded(t, pr, reason)
		return
	}
	if err := r.host.Notify("fleet: run failed — "+t.ID, reason, host.SoundNone); err != nil {
		log.Printf("%s: notify: %v", t.ID, err)
	}
}

// lateWindow is how long a run keeps listening after its deadline before it
// gives the agent up: twice the run's own timeout. The deadline marks the
// run, it does not drop the delivery (ADR 0014) — and one more window of the
// same length would have missed every late delivery the fleet has actually
// seen (TASK-145 settled 99 minutes past its 90-minute deadline). Twice that
// bounds a hung agent at three times its timeout: failed, notified, and
// ended, so nothing works on a task the fleet has closed.
func lateWindow(a fleet.Agent) time.Duration {
	return 2 * time.Duration(a.TimeoutMinutes) * time.Minute
}

// mergeNeededLabel marks a PR the fleet stopped on so the merge it owes can
// be found again later — the forge never notifies anyone about the fleet's
// own actions, so the label is the durable half of the announcement.
const mergeNeededLabel = "merge-needed"

// mergeNeeded announces the one stop a human alone can move: the PR stays
// open and only they can take it further. GitHub cannot — the fleet opens
// PRs under the human's own account, and GitHub never notifies you of your
// own actions — so the stop speaks here: one popup, the reason as its body,
// the request sound, and the PR labelled merge-needed (created in the repo
// if missing). Both asks are best-effort the way every notification is (ADR
// 0010): the note on the task and the board's Blocked column are the report;
// this is the voice over it.
func (r *Runner) mergeNeeded(t work.Task, pr, reason string) {
	ref := pr
	if repo, number, err := gate.RepoRef(pr); err == nil {
		ref = repo + "#" + number
	}
	if err := r.host.Notify("merge needed — "+ref, reason, host.SoundRequest); err != nil {
		log.Printf("%s: notify: %v", t.ID, err)
	}
	if err := r.forge.AddLabel(pr, mergeNeededLabel); err != nil {
		log.Printf("%s: label %s merge-needed: %v", t.ID, pr, err)
	}
}

// claim shows the task as being worked on, where the backend can say so. The
// write is display only and best-effort: a binary backend has no such state,
// and the run lock — not this — is what keeps two runs apart, so a backend
// that cannot say it is not a reason to refuse the run.
//
// A triaged card is left where it is (TASK-86): its status is what routes the
// run, and painting over it would leave a card that is mid-grill looking like
// ordinary work in hand — to the next tick and to the person reading the board.
func (r *Runner) claim(src work.Source, t work.Task) {
	if t.Triaging {
		return
	}
	p, ok := src.(work.Phaser)
	if !ok {
		return
	}
	if err := p.SetPhase(t.ID, work.PhaseInProgress); err != nil {
		log.Printf("%s: mark as running: %v", t.ID, err)
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
//
// delivered is the pipeline's open-on-purpose: an author that handed in its PR
// or a verifier that recorded its verdict left the task open for the next
// stage, exactly as a handoff does.
func (r *Runner) reconcile(src work.Source, taskID, agent string, runErr error, delivered bool) (verdict work.Verdict, handedOn, reported bool) {
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
	if runErr == nil && (delivered || (v.Assignee != "" && v.Assignee != agent)) {
		return "", true, true
	}
	// A card the run left waiting for the owner's answer is the grill's own
	// open-on-purpose (TASK-86): the triager asked, and closing it here would
	// end the idea instead of parking it. A card still in triage is not that —
	// the run neither asked nor specced it — so it falls through to Failed and
	// a human reopens it rather than the next tick spinning on the same card.
	if runErr == nil && v.Asking {
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
// their edits are the repo's own state by design — and verify sessions never
// carry anything in: they are never Inspect'ed, so there is no finding here
// but the zero one.
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
// alone here rather than described twice. ended says the workspace is already
// gone — closed to end an agent that never settled — and the failure note
// said why; this owes it silence, not a second account.
func (r *Runner) cleanup(src work.Source, taskID string, s host.Session, final work.Verdict, handedOn, keep, ended bool) {
	if ended || s.WorkspaceID == "" || keep {
		return
	}
	// A verify run's worktree is judged space (TASK-65): nothing in it a
	// human resumes, and nothing in it the record does not already carry —
	// so it is discarded however the run ended, with no "jump in to resume"
	// note left pointing at a checkout that is about to be gone.
	if s.Verify {
		if err := r.host.Close(s); err != nil {
			log.Printf("%s: close the verify worktree: %v", taskID, err)
		}
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
	// verification and patch are what `task verdict` stamped mid-run, carried
	// onto the closing record like the pull request.
	verification, patch string
	// timedOut is set the moment the run passes its deadline: every record
	// from then on carries it, the closing one included, so a reader that
	// collapses a run to its latest record still sees it went late.
	timedOut bool
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
	r.verification, r.patch, _ = history.VerificationFor(r.id)
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
		At: time.Now(), WorkspaceID: s.WorkspaceID, PaneID: s.PaneID, TabID: s.TabID, Branch: s.Branch, BaseCommit: s.BaseCommit,
		DurationSeconds: seconds, Verdict: verdict, Error: errText,
		Commits: d.Commits, Uncommitted: unverified, PullRequest: pullRequest,
		Verification: r.verification, PatchID: r.patch, TimedOut: r.timedOut,
	})
	if err != nil {
		log.Printf("history append failed: %v", err)
	}
}
