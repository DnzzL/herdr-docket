package host

import (
	"testing"
	"time"

	"github.com/DnzzL/herdr-fleet/internal/herdr"
)

// fakeOps scripts Herdr's answers. A nil field means "this call is fine and
// says nothing", which keeps each test to the calls it actually cares about.
type fakeOps struct {
	worktreeCreate  func(repo, branch, label string) (string, string, error)
	workspaceCreate func(cwd, label string) (string, string, error)
	agentStart      func(name, kind, paneID string, args []string) error
	agentSubmit     func(target, text string) error
	submitPending   func(paneID string) error
	agentStatus     func(target string) (string, error)
	agentWait       func(target string, d time.Duration) error
	paneRun         func(paneID string, command ...string) error
	paneRead        func(paneID string, lines int) (string, error)
	lookPath        func(file string) error

	starts  int
	submits int
	pending int
	waits   int
	reads   int
}

func (f *fakeOps) WorktreeCreate(repo, branch, label string) (string, string, error) {
	if f.worktreeCreate == nil {
		return "w1", "w1:p1", nil
	}
	return f.worktreeCreate(repo, branch, label)
}

func (f *fakeOps) WorkspaceCreate(cwd, label string) (string, string, error) {
	if f.workspaceCreate == nil {
		return "w2", "w2:p1", nil
	}
	return f.workspaceCreate(cwd, label)
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

func TestProvisionOpensAWorktreeOnABranchOfItsOwn(t *testing.T) {
	var gotBranch, gotLabel string
	ops := &fakeOps{worktreeCreate: func(repo, branch, label string) (string, string, error) {
		gotBranch, gotLabel = branch, label
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

	if _, err := h.Provision(Spec{
		Name: "n", Repo: "/repo", Workspace: WorkspaceRoot,
	}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("root mode must not create a worktree")
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
	got := agentName(long)
	if len(got) > 32 {
		t.Fatalf("agentName(%q) = %q, %d chars", long, got, len(got))
	}
	if got[len(got)-1] == '-' {
		t.Errorf("agentName(%q) = %q, want no trailing dash", long, got)
	}
}
