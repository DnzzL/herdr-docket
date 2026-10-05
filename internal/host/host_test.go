package host

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DnzzL/herdr-docket/internal/herdr"
)

// fakeOps scripts Herdr's answers. A nil field means "this call is fine and
// says nothing", which keeps each test to the calls it actually cares about.
type fakeOps struct {
	worktreeCreate  func(repo, branch, base, label string) (string, string, error)
	commitAt        func(repo, ref string) (string, error)
	workspaceCreate func(cwd, label string) (string, string, error)
	// primary is the workspace a repo is already open in; empty means none,
	// which is the fleet's cue to open one of its own.
	primary        string
	tabs           int
	tabCloses      int
	closedTab      string
	agentStart     func(name, kind, paneID string, args []string) error
	agentSubmit    func(target, text string) error
	submitPending  func(paneID string) error
	agentStatus    func(target string) (string, error)
	agentWait      func(target string, d time.Duration) error
	paneRun        func(paneID string, command ...string) error
	paneRead       func(paneID string, lines int) (string, error)
	lookPath       func(file string) error
	workspaceClose func(id string) error
	worktreePath   func(repo, branch string) (string, error)
	worktreeDirty  func(dir string) (bool, error)
	commitsAhead   func(repo, branch string) (int, error)
	// worktreeDiscard is the teardown of a verify worktree: path and branch
	// recorded so a test can assert exactly what was thrown away.
	worktreeDiscard func(repo, branch string) error
	discards        []string

	closes int

	starts   int
	submits  int
	pending  int
	waits    int
	reads    int
	notifies []string
}

func (f *fakeOps) WorktreeCreate(repo, branch, base, label string) (string, string, error) {
	if f.worktreeCreate == nil {
		return "w1", "w1:p1", nil
	}
	return f.worktreeCreate(repo, branch, base, label)
}

func (f *fakeOps) CommitAt(repo, ref string) (string, error) {
	if f.commitAt == nil {
		return "0000000", nil
	}
	return f.commitAt(repo, ref)
}

func (f *fakeOps) WorktreePath(repo, branch string) (string, error) {
	if f.worktreePath == nil {
		return "/wt/" + branch, nil
	}
	return f.worktreePath(repo, branch)
}

func (f *fakeOps) WorktreeDirty(dir string) (bool, error) {
	if f.worktreeDirty == nil {
		return false, nil
	}
	return f.worktreeDirty(dir)
}

func (f *fakeOps) CommitsAhead(repo, branch string) (int, error) {
	if f.commitsAhead == nil {
		return 0, nil
	}
	return f.commitsAhead(repo, branch)
}

func (f *fakeOps) WorktreeDiscard(repo, branch string) error {
	f.discards = append(f.discards, repo+"@"+branch)
	if f.worktreeDiscard == nil {
		return nil
	}
	return f.worktreeDiscard(repo, branch)
}

func (f *fakeOps) WorkspaceCreate(cwd, label string) (string, string, error) {
	if f.workspaceCreate == nil {
		return "w2", "w2:p1", nil
	}
	return f.workspaceCreate(cwd, label)
}

func (f *fakeOps) WorkspaceClose(id string) error {
	f.closes++
	if f.workspaceClose == nil {
		return nil
	}
	return f.workspaceClose(id)
}

func (f *fakeOps) AgentStart(name, kind, paneID string, args []string) error {
	f.starts++
	if f.agentStart == nil {
		return nil
	}
	return f.agentStart(name, kind, paneID, args)
}

func (f *fakeOps) AgentSubmit(target, text string) error {
	f.submits++
	if f.agentSubmit == nil {
		return nil
	}
	return f.agentSubmit(target, text)
}

func (f *fakeOps) AgentSubmitPending(paneID string) error {
	f.pending++
	if f.submitPending == nil {
		return nil
	}
	return f.submitPending(paneID)
}

func (f *fakeOps) AgentStatus(target string) (string, error) {
	if f.agentStatus == nil {
		return "working", nil
	}
	return f.agentStatus(target)
}

func (f *fakeOps) AgentWait(target string, d time.Duration) error {
	f.waits++
	if f.agentWait == nil {
		return nil
	}
	return f.agentWait(target, d)
}

func (f *fakeOps) PaneRun(paneID string, command ...string) error {
	if f.paneRun == nil {
		return nil
	}
	return f.paneRun(paneID, command...)
}

func (f *fakeOps) PaneRead(paneID string, lines int) (string, error) {
	f.reads++
	if f.paneRead == nil {
		return "", nil
	}
	return f.paneRead(paneID, lines)
}

func (f *fakeOps) LookPath(file string) error {
	if f.lookPath == nil {
		return nil
	}
	return f.lookPath(file)
}

// HasCode is the real thing: the fakes return real *herdr.APIError values, so
// the code-recovery path under test is the one that ships.
func (f *fakeOps) HasCode(err error, code string) bool { return herdr.HasCode(err, code) }

// fast is defaultKnobs with the sleeps taken out.
func fast() knobs {
	return knobs{
		paneReady:        time.Minute,
		paneReadyPoll:    time.Millisecond,
		waitSlice:        30 * time.Second,
		statusPoll:       time.Millisecond,
		promptSettle:     5 * time.Millisecond,
		promptSettleLast: 5 * time.Millisecond,
		workflowPoll:     time.Millisecond,
	}
}

func apiErr(command, code string) error {
	return &herdr.APIError{Command: command, Code: code, Message: code}
}

// A workspace that is already gone is a torn-down workspace: the point was for
// it not to exist. The runner cleans up after a run through this port and has
// no reason to log the benign half of it as a failure. Both shapes herdr can
// hand back are covered — the code, which the low-level client recovers from,
// and the sentinel, which is what it reports — and a real failure still
// surfaces instead of being read as a success.
func TestCloseTreatsAGoneWorkspaceAsTornDown(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"herdr says the workspace is not there", apiErr("workspace close", herdr.CodeWorkspaceGone)},
		{"the client already read it as gone", herdr.ErrGone},
		{"nothing wrong", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ops := &fakeOps{workspaceClose: func(string) error { return tc.err }}
			h := &live{ops: ops, knobs: fast()}

			if err := h.Close(Session{WorkspaceID: "w1"}); err != nil {
				t.Fatalf("got %v, want a torn-down workspace", err)
			}
			if ops.closes != 1 {
				t.Fatalf("closes = %d, want 1", ops.closes)
			}
		})
	}

	ops := &fakeOps{workspaceClose: func(string) error { return errors.New("herdr: connection refused") }}
	h := &live{ops: ops, knobs: fast()}
	if err := h.Close(Session{WorkspaceID: "w1"}); err == nil {
		t.Fatal("a real failure must not be swallowed")
	}
}

func TestProvisionOpensAWorktreeOnABranchOfItsOwn(t *testing.T) {
	var gotBranch, gotLabel string
	ops := &fakeOps{commitAt: func(repo, ref string) (string, error) {
		if ref != "HEAD" {
			t.Errorf("CommitAt ref = %q, want HEAD with no base named", ref)
		}
		return "c0074fe", nil
	}, worktreeCreate: func(repo, branch, base, label string) (string, string, error) {
		gotBranch, gotLabel = branch, label
		if base != "" {
			t.Errorf("base = %q, want none with nothing named", base)
		}
		return "w7", "w7:p1", nil
	}}
	h := &live{ops: ops, knobs: fast()}

	s, err := h.Provision(Spec{
		Name: "Weekly sprint planning", Repo: "/x", Workspace: WorkspaceWorktree,
	})
	if err != nil {
		t.Fatal(err)
	}
	if s.WorkspaceID != "w7" || s.PaneID != "w7:p1" {
		t.Fatalf("session = %+v", s)
	}
	// git check-ref-format rejects spaces, so the name has to be slugged before
	// it can become a branch.
	if want := "fleet/weekly-sprint-planning-"; len(gotBranch) <= len(want) || gotBranch[:len(want)] != want {
		t.Errorf("branch = %q, want it to start with %q", gotBranch, want)
	}
	if gotLabel != "fleet: Weekly sprint planning" {
		t.Errorf("label = %q", gotLabel)
	}
	// The session carries the branch it was cut on: only the host ever knows
	// it, and everything downstream (the delivery record) reads it from here.
	if s.Branch != gotBranch || s.Branch == "" {
		t.Errorf("session branch = %q, want the branch created for it (%q)", s.Branch, gotBranch)
	}
	if s.Repo != "/x" {
		t.Errorf("session repo = %q, want the repo it opened on", s.Repo)
	}
}

// TASK-45: the ref named on the spec is the ref the worktree is cut from, and
// the session records the commit that produced — so a run branched from a
// human's mid-branch work is visible in the history, not only on the forge.
func TestProvisionBranchesFromTheNamedBaseAndRecordsItsCommit(t *testing.T) {
	var gotBase string
	var gotRef string
	ops := &fakeOps{commitAt: func(repo, ref string) (string, error) {
		gotRef = ref
		return "9a1b2c3", nil
	}, worktreeCreate: func(repo, branch, base, label string) (string, string, error) {
		gotBase = base
		return "w8", "w8:p1", nil
	}}
	h := &live{ops: ops, knobs: fast()}

	s, err := h.Provision(Spec{
		Name: "n", Repo: "/x", Workspace: WorkspaceWorktree, Base: "origin/main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotBase != "origin/main" {
		t.Errorf("worktree base = %q, want origin/main", gotBase)
	}
	if gotRef != "origin/main" {
		t.Errorf("cut commit read from ref %q, want origin/main", gotRef)
	}
	if s.BaseCommit != "9a1b2c3" {
		t.Errorf("session BaseCommit = %q, want the commit the run branched from", s.BaseCommit)
	}
}

// Inheriting says so with the record too: a run cut from the checkout's HEAD
// records what it inherited, so nobody has to wonder after the fact.
func TestProvisionRecordsTheInheritedCutCommit(t *testing.T) {
	var gotRef string
	ops := &fakeOps{commitAt: func(repo, ref string) (string, error) {
		gotRef = ref
		return "1111111", nil
	}}
	h := &live{ops: ops, knobs: fast()}

	s, err := h.Provision(Spec{Name: "n", Repo: "/x", Workspace: WorkspaceWorktree})
	if err != nil {
		t.Fatal(err)
	}
	if gotRef != "HEAD" {
		t.Errorf("cut commit read from ref %q, want HEAD", gotRef)
	}
	if s.BaseCommit != "1111111" {
		t.Errorf("session BaseCommit = %q, want the inherited commit", s.BaseCommit)
	}
}

// A repo that cannot even name its HEAD cannot be worktree'd honestly: failing
// the provision beats starting a run whose base no one can name.
func TestProvisionRefusesToStartAWorktreeItCannotNameTheBaseOf(t *testing.T) {
	ops := &fakeOps{commitAt: func(repo, ref string) (string, error) {
		return "", errors.New("not a git repository")
	}}
	h := &live{ops: ops, knobs: fast()}
	if _, err := h.Provision(Spec{Name: "n", Repo: "/x", Workspace: WorkspaceWorktree}); err == nil {
		t.Fatal("want an error when the cut commit cannot be read")
	}
}

func TestProvisionRootModeOpensTheRepoItself(t *testing.T) {
	called := false
	ops := &fakeOps{workspaceCreate: func(cwd, label string) (string, string, error) {
		called = true
		if cwd != "/repo" {
			t.Errorf("cwd = %q", cwd)
		}
		return "w9", "w9:p1", nil
	}}
	h := &live{ops: ops, knobs: fast()}

	s, err := h.Provision(Spec{
		Name: "n", Repo: "/repo", Workspace: WorkspaceRoot,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("root mode must not create a worktree")
	}
	// Root mode has no branch of its own — inventing one would be the fleet
	// claiming an output it cannot vouch for. The repo is still recorded.
	if s.Branch != "" {
		t.Errorf("branch = %q, want none for a root-mode run", s.Branch)
	}
	if s.Repo != "/repo" {
		t.Errorf("repo = %q, want the repo it opened on", s.Repo)
	}
}

func TestProvisionRefusesAWorkspaceModeItDoesNotKnow(t *testing.T) {
	// Load validates this, so reaching here means something bypassed it. Saying
	// so beats provisioning nothing and reporting success.
	h := &live{ops: &fakeOps{}, knobs: fast()}
	if _, err := h.Provision(Spec{Name: "n", Workspace: "sandbox"}); err == nil {
		t.Fatal("want an error for an unknown workspace mode")
	}
}

// TASK-65: a run pinned to one commit — a verifier standing on a PR's head —
// is provisioned in its own worktree cut at that commit, whatever workspace
// mode the agent names. The pin overrides the mode outright: even a root-mode
// verifier must never open a tab on the primary checkout, and the commit it
// stands on is the forge's answer, read before anything is cut.
func TestAVerifyRunIsPinnedToThePRHeadAndNeverBorrowsATab(t *testing.T) {
	var gotRef, gotBase, gotBranch string
	ops := &fakeOps{
		primary: "w15", // the project's checkout is open — the tab a root run would borrow
		commitAt: func(repo, ref string) (string, error) {
			gotRef = ref
			return "9c0ffee", nil
		},
		worktreeCreate: func(repo, branch, base, label string) (string, string, error) {
			gotBase, gotBranch = base, branch
			return "w7", "w7:p1", nil
		},
	}
	h := &live{ops: ops, knobs: fast()}

	s, err := h.Provision(Spec{
		Name: "TASK-1 fix", Repo: "/w/app", Workspace: WorkspaceRoot,
		Head: "0fba11c",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ops.tabs != 0 || s.TabID != "" {
		t.Errorf("a pinned run must never borrow a tab on the primary checkout: tabs=%d session=%+v", ops.tabs, s)
	}
	if gotBase != "0fba11c" || gotRef != "0fba11c" {
		t.Errorf("cut at base %q read from %q, want the PR head both ways", gotBase, gotRef)
	}
	if s.BaseCommit != "9c0ffee" {
		t.Errorf("session BaseCommit = %q, want the commit the run was cut at", s.BaseCommit)
	}
	if s.Branch == "" || s.Branch != gotBranch {
		t.Errorf("session branch = %q, want the branch provisioned (%q)", s.Branch, gotBranch)
	}
	if !s.Verify {
		t.Errorf("session = %+v, want it marked as a verify run's", s)
	}
}

// The pin is only honest if the commit is really in the repo: a head the
// checkout cannot resolve fails the provision instead of starting a run on
// whatever the branch happens to point at.
func TestProvisionRefusesAPRHeadTheRepoDoesNotHave(t *testing.T) {
	ops := &fakeOps{commitAt: func(repo, ref string) (string, error) {
		return "", errors.New("unknown revision 0fba11c")
	}}
	h := &live{ops: ops, knobs: fast()}
	if _, err := h.Provision(Spec{Name: "n", Repo: "/x", Workspace: WorkspaceRoot, Head: "0fba11c"}); err == nil {
		t.Fatal("want an error when the PR head is not in the repo")
	}
	if ops.tabs != 0 {
		t.Errorf("even a failed provision must not borrow a tab: tabs=%d", ops.tabs)
	}
}

// Close of a verify session is the removal half of TASK-65: the workspace is
// closed and its worktree and branch are discarded, even when the workspace
// is already gone (a run called off by closing it). Nothing the run created
// may outlive it — and a non-verify worktree run, whose branch is a delivery,
// must keep all three.
func TestCloseDiscardsAVerifyWorktreeAndItsBranch(t *testing.T) {
	for _, tc := range []struct {
		name      string
		closeErr  error
		session   Session
		wantDisca bool
	}{
		{"a settled run", nil, Session{WorkspaceID: "w1", Repo: "/x", Branch: "fleet/n-1", Verify: true}, true},
		{"a run whose workspace is already gone", herdr.ErrGone, Session{WorkspaceID: "w1", Repo: "/x", Branch: "fleet/n-1", Verify: true}, true},
		{"an author worktree, whose branch is the delivery", nil, Session{WorkspaceID: "w1", Repo: "/x", Branch: "fleet/n-2"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ops := &fakeOps{workspaceClose: func(string) error { return tc.closeErr }}
			h := &live{ops: ops, knobs: fast()}

			if err := h.Close(tc.session); err != nil {
				t.Fatalf("Close = %v", err)
			}
			if tc.wantDisca {
				if len(ops.discards) != 1 || ops.discards[0] != "/x@"+tc.session.Branch {
					t.Errorf("discards = %v, want %s's worktree and branch thrown away", ops.discards, tc.session.Branch)
				}
			} else if len(ops.discards) != 0 {
				t.Errorf("discards = %v, want none: an author's branch must outlive its run", ops.discards)
			}
		})
	}
}

func TestDoPicksTheAgentAdapterWhenThereIsAPrompt(t *testing.T) {
	ops := &fakeOps{}
	h := &live{ops: ops, knobs: fast()}

	if err := h.Do(Session{PaneID: "p"}, Spec{
		Agent: "claude", Prompt: "go", Name: "n",
	}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if ops.starts != 1 || ops.submits != 1 || ops.waits != 1 {
		t.Fatalf("starts=%d submits=%d waits=%d, want one of each", ops.starts, ops.submits, ops.waits)
	}
}

func TestSlugProducesValidBranchNames(t *testing.T) {
	cases := map[string]string{
		"Weekly sprint planning": "weekly-sprint-planning",
		"issue-triage":           "issue-triage",
		"Deps  bump!!":           "deps-bump",
		"  ~weird/name~  ":       "weird-name",
		"???":                    "automation",
	}
	for in, want := range cases {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAgentNameFitsHerdrsLimit(t *testing.T) {
	long := "a-very-long-automation-name-that-herdr-will-not-accept"
	got := agentName(long, "")
	if len(got) > 32 {
		t.Fatalf("agentName(%q) = %q, %d chars", long, got, len(got))
	}
	if got[len(got)-1] == '-' {
		t.Errorf("agentName(%q) = %q, want no trailing dash", long, got)
	}
}

// Herdr keeps agent names unique across every workspace, and a Failed or
// Blocked run leaves its workspace open as the place to resume. An untagged
// name therefore made every retry of a task collide with the workspace its
// failed attempt abandoned — the retry could not start while the pane it was
// meant to improve was still holding the name.
func TestAgentNameKeepsAttemptsAtATaskApart(t *testing.T) {
	first := agentName("TASK-34 Sweep notara PR review queue: !18-!27", "tl4367")
	second := agentName("TASK-34 Sweep notara PR review queue: !18-!27", "tl4368")
	if first == second {
		t.Fatalf("two attempts at one task share an agent name: %q", first)
	}
	for i, got := range []string{first, second} {
		tag := []string{"tl4367", "tl4368"}[i]
		if len(got) > 32 {
			t.Fatalf("agentName = %q, %d chars — over herdr's limit", got, len(got))
		}
		if !strings.HasPrefix(got, "task-34") {
			t.Errorf("agentName = %q, want the task still recognisable at the front", got)
		}
		if !strings.HasSuffix(got, tag) {
			t.Errorf("agentName = %q, want the run tag %q to survive truncation", got, tag)
		}
	}
}

func TestTagIsTheRunsSecondAndDistinguishesAttempts(t *testing.T) {
	a := Tag("TASK-34-1788981775983655000")
	b := Tag("TASK-34-1788981776983655000")
	if a == "" || a != "tl4367" {
		t.Fatalf("Tag = %q, want tl4367", a)
	}
	if a == b {
		t.Fatal("a second later must be a different tag")
	}
	if got := Tag("no-separator-here"); got != "" {
		t.Fatalf("Tag on an id with no numeric suffix = %q, want empty", got)
	}
}

func (f *fakeOps) PrimaryWorkspace(string) (string, error) { return f.primary, nil }

func (f *fakeOps) TabCreate(workspaceID, cwd, label string) (string, string, error) {
	f.tabs++
	return workspaceID + ":p9", workspaceID + ":t9", nil
}

func (f *fakeOps) TabClose(tabID string) error {
	f.tabCloses++
	f.closedTab = tabID
	return nil
}

// A root run works the project's own checkout, and that checkout is usually
// already open in a workspace. Borrowing it as a tab is what keeps a fleet of
// three projects from putting one sidebar entry per run in front of a human.
func TestARootRunBorrowsTheProjectsWorkspaceAsATab(t *testing.T) {
	ops := &fakeOps{primary: "w15"}
	h := &live{ops: ops}

	s, err := h.Provision(Spec{Name: "t", Repo: "/w/app", Workspace: WorkspaceRoot})
	if err != nil {
		t.Fatal(err)
	}
	if ops.tabs != 1 {
		t.Errorf("want one tab opened, got %d", ops.tabs)
	}
	if s.WorkspaceID != "w15" || s.TabID == "" {
		t.Errorf("session = %+v, want the borrowed workspace and a tab of its own", s)
	}
}

// The whole hazard in one test: a run that borrowed a workspace must give back
// only its tab. Closing the workspace would end the runs beside it and
// whatever the human had open there.
func TestClosingABorrowedWorkspaceClosesOnlyTheTab(t *testing.T) {
	ops := &fakeOps{primary: "w15"}
	h := &live{ops: ops}
	s, err := h.Provision(Spec{Name: "t", Repo: "/w/app", Workspace: WorkspaceRoot})
	if err != nil {
		t.Fatal(err)
	}

	if err := h.Close(s); err != nil {
		t.Fatal(err)
	}
	if ops.closes != 0 {
		t.Errorf("a borrowed workspace must never be closed: %d closes", ops.closes)
	}
	if ops.tabCloses != 1 || ops.closedTab != s.TabID {
		t.Errorf("want the run's own tab closed, got %d closes of %q", ops.tabCloses, ops.closedTab)
	}
}

// A project nobody has open gets a workspace of its own, exactly as before:
// the fleet never creates the primary it would then be borrowing.
func TestARootRunOpensItsOwnWorkspaceWhenTheProjectIsNotOpen(t *testing.T) {
	ops := &fakeOps{primary: ""}
	h := &live{ops: ops}
	s, err := h.Provision(Spec{Name: "t", Repo: "/w/app", Workspace: WorkspaceRoot})
	if err != nil {
		t.Fatal(err)
	}
	if ops.tabs != 0 || s.TabID != "" {
		t.Errorf("no primary means no borrowing: tabs=%d session=%+v", ops.tabs, s)
	}
	if err := h.Close(s); err != nil {
		t.Fatal(err)
	}
	if ops.closes != 1 || ops.tabCloses != 0 {
		t.Errorf("a workspace it opened is a workspace it closes: closes=%d tabCloses=%d", ops.closes, ops.tabCloses)
	}
}

// Worktree runs are untouched: herdr already groups them under the project by
// repo_key, so they keep their own workspace and close it as they always did.
func TestAWorktreeRunIsNotBorrowed(t *testing.T) {
	ops := &fakeOps{primary: "w15"}
	h := &live{ops: ops}
	s, err := h.Provision(Spec{Name: "t", Repo: "/w/app", Workspace: WorkspaceWorktree})
	if err != nil {
		t.Fatal(err)
	}
	if ops.tabs != 0 || s.TabID != "" || s.Branch == "" {
		t.Errorf("a worktree run keeps its own workspace: tabs=%d session=%+v", ops.tabs, s)
	}
}

func (f *fakeOps) Notify(title, body, sound string) error {
	f.notifies = append(f.notifies, title+"\x1f"+body+"\x1f"+sound)
	return nil
}

// The production git read behind CommitAt: a real repo answers with the
// commit its ref names, and a ref that does not exist is a refusal — not a
// zero commit. (TASK-45: a provision records the run's cut commit with this.)
func TestCommitAtReadsARealRepo(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v (%s)", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "fleet@example.com")
	run("config", "user.name", "fleet")
	run("commit", "--allow-empty", "-qm", "first")
	want := run("rev-parse", "HEAD")

	var ops herdrOps
	got, err := ops.CommitAt(dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("CommitAt = %q, want %q", got, want)
	}
	if _, err := ops.CommitAt(dir, "refs/heads/nope"); err == nil {
		t.Fatal("a ref that does not exist must be a refusal")
	}
}

// The production discard behind a verify run's Close, on a real repo: the
// worktree and the branch are both gone afterwards — the registration, the
// checkout directory and refs/heads — and a worktree that is already gone
// still loses its branch. AC #2 in one assertion: nothing the run created
// outlives it.
func TestWorktreeDiscardLeavesNoBranchOrWorktreeBehind(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) (string, error) {
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}
	must := func(args ...string) string {
		out, err := run(args...)
		if err != nil {
			t.Fatalf("git %s: %v (%s)", args, err, out)
		}
		return out
	}
	must("init", "-q", "-b", "main")
	must("config", "user.email", "fleet@example.com")
	must("config", "user.name", "fleet")
	must("commit", "--allow-empty", "-qm", "first")
	wt := filepath.Join(t.TempDir(), "verify")
	must("worktree", "add", "-q", "-b", "fleet/task-1-1", wt, "HEAD")

	var ops herdrOps
	if err := ops.WorktreeDiscard(dir, "fleet/task-1-1"); err != nil {
		t.Fatal(err)
	}
	list := must("worktree", "list", "--porcelain")
	if strings.Contains(list, wt) {
		t.Errorf("worktree still registered:\n%s", list)
	}
	if _, err := run("rev-parse", "--verify", "refs/heads/fleet/task-1-1"); err == nil {
		t.Error("the run's branch must be deleted with its worktree")
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Errorf("checkout dir still on disk: %v", err)
	}

	// Cancelled runs reach Close with the workspace already gone — the branch
	// is still there to discard.
	must("branch", "fleet/task-1-2")
	if err := ops.WorktreeDiscard(dir, "fleet/task-1-2"); err != nil {
		t.Fatal(err)
	}
	if _, err := run("rev-parse", "--verify", "refs/heads/fleet/task-1-2"); err == nil {
		t.Error("a branch whose worktree is already gone must still be deleted")
	}
}
