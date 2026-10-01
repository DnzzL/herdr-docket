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
	Name      string // used for workspace labels and branch names
	RunTag    string // per-attempt suffix, see Tag: keeps retries from colliding
	Repo      string // absolute path the workspace opens on
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
	// Inspect reads what the run's workspace produced. It is asked before a
	// teardown, because it is the only answer that survives the teardown —
	// and it refuses a session with no branch rather than report a clean
	// delivery for a run that has none to report.
	Inspect(s Session) (Delivery, error)
	// Close tears the session's workspace down. Called only when the run left
	// nothing a human still needs to look at.
	Close(s Session) error
	// Notify raises Herdr's own desktop notification: the one way a run's
	// report reaches a human who was not watching. Best-effort by contract —
	// a caller logs a refusal and moves on, never re-decides what happened.
	Notify(title, body string) error
}

// ErrCancelled means the run's workspace was closed while it was working.
// Closing it is the gesture for calling a run off, so the run is reported
// cancelled rather than failed.
var ErrCancelled = errors.New("the run's workspace was closed")

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
	var err error
	switch a.Workspace {
	case WorkspaceWorktree:
		branch = fmt.Sprintf("fleet/%s-%s", slug(a.Name), time.Now().Format("20060102-1504"))
		workspaceID, paneID, err = h.ops.WorktreeCreate(a.Repo, branch, label)
	case WorkspaceRoot:
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
	return Session{WorkspaceID: workspaceID, PaneID: paneID, Branch: branch, Repo: a.Repo, TabID: tabID}, err
}

// Do runs the automation's work in the session and reports whether it worked.
func (h *live) Do(s Session, a Spec, timeout time.Duration) error {
	return h.workFor(a).do(s, timeout)
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

// Close tears the session's workspace down. A workspace that is already gone
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
		return nil
	}
	return err
}

// Notify raises Herdr's own desktop notification, verbatim on to the ops.
func (h *live) Notify(title, body string) error { return h.ops.Notify(title, body) }

// work is one way of getting a task's work done in a session. One adapter
// satisfies it today — an agent taking a prompt — and the seam stays because
// it is what makes the choreography testable.
type work interface {
	do(s Session, timeout time.Duration) error
}

func (h *live) workFor(a Spec) work {
	return agentWork{ops: h.ops, knobs: h.knobs, a: a}
}

// slug makes a name safe for a git branch: spaces and the characters
// git check-ref-format rejects would otherwise fail worktree creation.
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
