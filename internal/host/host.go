// Package host is the seam between a run and the machine it happens on.
//
// Two methods: provision somewhere for the work to happen, then do the work.
// Behind them sits everything that used to be the runner's problem — which
// herdr error code means "retry", which means "the agent is gone", how long a
// fresh pane gets to become a shell, how a delegated command's exit status is
// recovered from a terminal that has no exit status.
//
// The point of the narrowness is that runner.Run becomes testable. Four of the
// ten releases before this seam existed fixed sequencing bugs in that function
// and not one of the fixes could be pinned by a test, because there was no
// interface between it and exec.Command.
package host

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/DnzzL/herdr-docket/internal/herdr"
)

// Spec is everything Provision and Do need to know about one run: where it
// happens, which agent does it, and what it is told. The fleet package builds
// one per task; this package never reads task files or AGENT.md itself.
type Spec struct {
	Name   string // used for workspace labels and branch names
	RunTag string // per-attempt suffix, see Tag: keeps retries from colliding
	Repo   string // absolute path the workspace opens on
	// Base names the git ref a worktree run branches from. Empty inherits
	// whatever the repo's own checkout has checked out — which is exactly the
	// accident the queue's default-branch answer exists to replace, so a
	// caller that knows which branch its work starts from names it.
	Base string
	// Head pins a run to one commit — the pull request's head a verifier
	// judges. A pinned run is provisioned in its own worktree cut at that
	// commit, whatever Workspace says: the bytes being judged are the PR's,
	// never the project's own checkout, and no tab on a human's workspace is
	// reachable. The commit must already be in the repo — a head the checkout
	// cannot resolve fails the provision rather than fetching into it.
	Head      string
	Workspace WorkspaceMode
	Agent     string // agent kind as understood by `herdr agent start --kind`
	Model     string
	Prompt    string
	MCPConfig string
	AgentArgs []string
}

// WorkspaceMode is how the run's workspace is provisioned.
type WorkspaceMode string

const (
	WorkspaceWorktree WorkspaceMode = "worktree" // fresh git worktree per run
	WorkspaceRoot     WorkspaceMode = "root"     // workspace on the repo root
)

// Session is a provisioned place for a run to happen: a workspace and the pane
// its agent will live in. Branch is the branch a worktree-mode run was cut on
// — only Provision names it, so it is the one place a downstream record can
// learn it from — and is empty in root mode, where there is no branch to claim.
// Repo is the checkout the workspace opened on; Inspect needs it to find the
// worktree a run actually works in.
type Session struct {
	WorkspaceID string
	PaneID      string
	Branch      string
	Repo        string
	// TabID is set when the run borrowed a workspace rather than opening one:
	// it owns this tab and nothing else. Closing the workspace instead would
	// end every other run in it, and the human's own work beside them, so
	// what a run owns travels with the session and onto its history record.
	TabID string
	// BaseCommit is the commit the run branched from, read at provision time —
	// before the agent can move anything — so a run that inherited a human's
	// unmerged work is visible in the record, not only on the forge.
	BaseCommit string
	// Verify marks a session cut at Spec.Head: a verifier's judging space.
	// Its report is the verdict in the queue, never anything left in the
	// tree, so Close discards its worktree and branch however the run ended —
	// and nothing reads it for a delivery to guard (there is none to lose).
	Verify bool
}

// Delivery is what a run's worktree produced, read from git rather than from
// the agent's word: Commits is how far the branch has moved beyond the repo's
// own checkout, Dirty is whether the worktree holds changes nobody committed —
// the state that is lost the moment the workspace is torn down.
type Delivery struct {
	Commits int
	Dirty   bool
}

// Host provisions somewhere for a run to happen, does the work there, and
// cleans the place up when the work is over.
type Host interface {
	Provision(a Spec) (Session, error)
	Do(s Session, a Spec, timeout time.Duration) error
	// Settle waits for the agent Do left working when the run's own clock ran
	// out. The deadline reports itself (ErrTimedOut), it does not end the run:
	// the runner records it and calls this to keep listening — nil when the
	// agent settles within the window, ErrCancelled when the pane is gone,
	// ErrTimedOut again when the window passes first.
	Settle(s Session, window time.Duration) error
	// Inspect reads what the run's workspace produced. It is asked before a
	// teardown, because it is the only answer that survives the teardown —
	// and it refuses a session with no branch rather than report a clean
	// delivery for a run that has none to report.
	Inspect(s Session) (Delivery, error)
	// Close tears the session's workspace down and retires the worktree
	// behind it (wholesale for verify, only what is safe for an author).
	// Called only when the run left nothing a human still needs to look at.
	Close(s Session) error
	// Notify raises Herdr's own desktop notification: the one way a run's
	// report reaches a human who was not watching. Best-effort by contract —
	// a caller logs a refusal and moves on, never re-decides what happened.
	// sound names the audio the popup plays, herdr's own words (SoundNone
	// for silence); which sound a stop deserves is policy, so the caller
	// says it and this port only carries it (ADR 0015).
	Notify(title, body, sound string) error
}

// The notification sounds herdr accepts, spelled the way its CLI spells
// them, so a caller never has to know the flag's vocabulary beyond this
// package.
const (
	SoundNone    = "none"
	SoundRequest = "request"
)

// ErrCancelled means the run's workspace was closed while it was working.
// Closing it is the gesture for calling a run off, so the run is reported
// cancelled rather than failed.
var ErrCancelled = errors.New("the run's workspace was closed")

// ErrTimedOut means the run's clock ran out with the agent still working.
// It reports a deadline, not an end: Do and Settle both answer with it, and
// the runner decides what a late run does (ADR 0014).
var ErrTimedOut = errors.New("the agent was still working")

// New returns the Host that drives the real Herdr.
func New() Host { return &live{ops: herdrOps{}, knobs: defaultKnobs()} }

// live is the production adapter: Herdr's CLI, a real clock, real sleeps.
type live struct {
	ops   ops
	knobs knobs
}

// knobs are the timings the choreography depends on. Fields rather than
// constants so the tests can drive the same code without sleeping.
type knobs struct {
	// paneReady is how long a freshly provisioned pane gets to become a shell.
	// Creating the workspace normally hands one back in milliseconds; the wait
	// exists for the run that starts as the machine wakes, where herdr answered
	// `worktree create` a quarter of an hour after it was asked and the pane's
	// shell was still not up.
	paneReady     time.Duration
	paneReadyPoll time.Duration
	// waitSlice bounds one herdr wait call. herdr reports a vanished pane only
	// when the call it was given returns, so this is how long a cancelled run
	// keeps its slot — not something to make long.
	waitSlice  time.Duration
	statusPoll time.Duration
	// promptSettle is how long the agent is watched after each nudge before
	// the next one; promptSettleLast gives the final attempt a little longer,
	// since there is nothing after it but giving up.
	promptSettle     time.Duration
	promptSettleLast time.Duration
	workflowPoll     time.Duration
}

func defaultKnobs() knobs {
	return knobs{
		paneReady:        2 * time.Minute,
		paneReadyPoll:    5 * time.Second,
		waitSlice:        30 * time.Second,
		statusPoll:       2 * time.Second,
		promptSettle:     20 * time.Second,
		promptSettleLast: 30 * time.Second,
		workflowPoll:     5 * time.Second,
	}
}

// Provision opens the workspace the automation asked for.
func (h *live) Provision(a Spec) (Session, error) {
	label := "fleet: " + a.Name
	var workspaceID, paneID, branch, tabID string
	var baseCommit string
	var verify bool
	var err error
	switch {
	case a.Head != "":
		// A pinned run (TASK-65): a verifier judges the PR's head commit, so
		// the worktree is cut from that commit and the mode the agent named
		// never reaches the switch below — root would have opened a tab on the
		// project's own checkout, which is the isolation this exists to take
		// out of the agent's hands.
		branch = fmt.Sprintf("fleet/%s-%s", slug(a.Name), time.Now().Format("20060102-1504"))
		baseCommit, err = h.ops.CommitAt(a.Repo, a.Head)
		if err != nil {
			return Session{}, err
		}
		workspaceID, paneID, err = h.ops.WorktreeCreate(a.Repo, branch, a.Head, label)
		verify = true
	case a.Workspace == WorkspaceWorktree:
		branch = fmt.Sprintf("fleet/%s-%s", slug(a.Name), time.Now().Format("20060102-1504"))
		// The cut commit is read before the workspace is even created — before
		// anything can move — from the ref the branch will be cut from (the
		// named base, or the checkout's HEAD when none was). A repo that
		// cannot name its own HEAD cannot be branched from honestly, so that
		// fails the provision rather than starting a run no one can later
		// account for.
		ref := a.Base
		if ref == "" {
			ref = "HEAD"
		}
		baseCommit, err = h.ops.CommitAt(a.Repo, ref)
		if err != nil {
			return Session{}, err
		}
		workspaceID, paneID, err = h.ops.WorktreeCreate(a.Repo, branch, a.Base, label)
	case a.Workspace == WorkspaceRoot:
		// A root run works the project's own checkout, which is usually
		// already open in a workspace. Borrow it as a tab rather than stacking
		// another workspace beside it: same directory, same thing on screen,
		// one entry in the sidebar instead of one per run. A project nobody
		// has open gets a workspace of its own, as before — the fleet never
		// creates the primary it would then be borrowing.
		if primary, perr := h.ops.PrimaryWorkspace(a.Repo); perr == nil && primary != "" {
			paneID, tabID, err = h.ops.TabCreate(primary, a.Repo, label)
			workspaceID = primary
			break
		}
		workspaceID, paneID, err = h.ops.WorkspaceCreate(a.Repo, label)
	default:
		err = fmt.Errorf("unknown workspace mode %q", a.Workspace)
	}
	return Session{WorkspaceID: workspaceID, PaneID: paneID, Branch: branch, Repo: a.Repo, TabID: tabID, BaseCommit: baseCommit, Verify: verify}, err
}

// Do runs the automation's work in the session and reports whether it worked.
// At the timeout it answers ErrTimedOut with the agent still working — the
// caller decides what a late run does, through Settle.
func (h *live) Do(s Session, a Spec, timeout time.Duration) error {
	return h.workFor(a).do(s, timeout)
}

// Settle keeps listening for the agent Do left working at the deadline. It
// waits exactly as Do waits — in slices, watching for cancellation, bounded
// by its own window — with nothing started and nothing submitted: the prompt
// is already in front of the agent.
func (h *live) Settle(s Session, window time.Duration) error {
	w := agentWork{ops: h.ops, knobs: h.knobs}
	return w.await(s, window)
}

// Inspect answers with what git says about the run's branch, not what anybody
// claims about it: the worktree's path is found by branch name, and both facts
// are read there. Every error is kept — a delivery that cannot be read must
// reach the caller as a refusal, never as a clean, zero-value answer.
func (h *live) Inspect(s Session) (Delivery, error) {
	if s.Branch == "" {
		return Delivery{}, fmt.Errorf("a run without a branch has no delivery to inspect")
	}
	path, err := h.ops.WorktreePath(s.Repo, s.Branch)
	if err != nil {
		return Delivery{}, err
	}
	dirty, err := h.ops.WorktreeDirty(path)
	if err != nil {
		return Delivery{}, err
	}
	commits, err := h.ops.CommitsAhead(s.Repo, s.Branch)
	if err != nil {
		return Delivery{}, err
	}
	return Delivery{Commits: commits, Dirty: dirty}, nil
}

// Close tears the session's workspace down and retires the worktree
// provisioning left behind: wholesale for a verify session (TASK-65), and
// for an author only what is safe to delete — pushed branches and clean
// trees (TASK-68). A workspace that is already gone
// is torn down already: the runner reaches this from the cleanup path, where
// the only thing it could do with the distinction is log it. The pane is the
// caller for which it means something, and it holds the client directly.
func (h *live) Close(s Session) error {
	// A borrowed workspace is given back one tab at a time. Closing it whole
	// would end the runs sharing it and whatever the human had open there.
	if s.TabID != "" {
		err := h.ops.TabClose(s.TabID)
		if errors.Is(err, herdr.ErrGone) || h.ops.HasCode(err, herdr.CodeWorkspaceGone) {
			return nil
		}
		return err
	}
	err := h.ops.WorkspaceClose(s.WorkspaceID)
	if errors.Is(err, herdr.ErrGone) || h.ops.HasCode(err, herdr.CodeWorkspaceGone) {
		err = nil
	}
	// The teardown of what Provision created, whichever way the run settled.
	// A verify run's worktree and branch are discarded with it, even when the
	// workspace was already gone (a run called off by closing it): what the
	// run judged is recorded in the queue, and nothing it created may stay
	// registered in the human's repo. An author's run gets the lighter retire
	// (TASK-68): the registration pruned, the branch deleted only when every
	// commit it holds is pushed — and only once the workspace really closed,
	// since an agent that may still be in the directory must not have it
	// removed underneath it.
	if s.Verify && s.Branch != "" {
		if derr := h.ops.WorktreeDiscard(s.Repo, s.Branch); derr != nil && err == nil {
			err = derr
		}
	} else if err == nil && s.Branch != "" {
		if rerr := h.ops.WorktreeRetire(s.Repo, s.Branch); rerr != nil {
			err = rerr
		}
	}
	return err
}

// Notify raises Herdr's own desktop notification, verbatim on to the ops.
func (h *live) Notify(title, body, sound string) error {
	return h.ops.Notify(title, body, sound)
}

// work is one way of getting a task's work done in a session. One adapter
// satisfies it today — an agent taking a prompt — and the seam stays because
// it is what makes the choreography testable.
type work interface {
	do(s Session, timeout time.Duration) error
}

func (h *live) workFor(a Spec) work {
	return agentWork{ops: h.ops, knobs: h.knobs, a: a}
}

// Slug makes a name safe for a git branch: spaces and the characters
// git check-ref-format rejects would otherwise fail worktree creation.
// Exported as the branch-derivation rule Provision applies to Spec.Name, so
// a fake host computing the branch in a test produces the same shape the
// provisioned session carries — never a string invented for the test.
func Slug(name string) string { return slug(name) }

// slug is Slug's implementation, kept private for the package's own callers
// (the branch in Provision, agentName's name and tag).
func slug(name string) string {
	var b strings.Builder
	lastDash := true // also trims leading dashes
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
			lastDash = false
		case !lastDash:
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "automation"
	}
	return out
}

// agentName fits an automation name into Herdr's agent-name rules: lowercase,
// [a-z0-9-_], at most 32 characters. Herdr's registry is global, so the run tag
// is appended after the name is shortened for it — a name that simply lost its
// tail to the limit would collide with every earlier attempt at the task.
func agentName(name, tag string) string {
	s := slug(name)
	if tag = slug(tag); tag == "" {
		return s
	}
	if room := 32 - len(tag) - 1; len(s) > room {
		s = strings.Trim(s[:room], "-")
	}
	return s + "-" + tag
}

// Tag shortens a run id into something that fits an agent name: the run's
// second, in base36. Two attempts at one task always land in different seconds
// — a worker runs one task at a time — so the tag distinguishes a retry from
// the abandoned workspace it is retrying past. Empty when the id has no numeric
// suffix to read, which leaves the name untagged rather than wrong.
func Tag(runID string) string {
	i := strings.LastIndexByte(runID, '-')
	if i < 0 {
		return ""
	}
	ns, err := strconv.ParseInt(runID[i+1:], 10, 64)
	if err != nil {
		return ""
	}
	return strconv.FormatInt(ns/int64(time.Second), 36)
}
