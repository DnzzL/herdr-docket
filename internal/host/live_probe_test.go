package host

// Live probe for TASK-65, run by hand against the real herdr and a real
// scratch repo — the fake cannot prove that a pinned provision opens a
// worktree whose HEAD is the PR head and that Close leaves nothing behind:
//
//	LIVE_HERDR=1 go test ./internal/host/ -run TestLiveVerifyProvisionAndClose -v
//
// Not a CI test: it creates a workspace in the session it runs in and
// removes it again.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DnzzL/herdr-docket/internal/herdr"
)

func TestLiveVerifyProvisionAndClose(t *testing.T) {
	if os.Getenv("LIVE_HERDR") == "" {
		t.Skip("set LIVE_HERDR=1 to probe the real herdr")
	}
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v (%s)", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "fleet@example.com")
	run("config", "user.name", "fleet")
	run("commit", "--allow-empty", "-qm", "first")
	prHead := run("rev-parse", "HEAD")
	run("commit", "--allow-empty", "-qm", "main moves on")

	// A primary workspace open on the repo — the tab a root-mode run would
	// borrow: the pinned provision must not touch it.
	var c herdr.Client
	primary, _, err := c.WorkspaceCreate(repo, "TASK-65 probe primary")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.WorkspaceClose(primary) }()

	h := New()
	s, err := h.Provision(Spec{
		Name: "TASK-65 live probe", Repo: repo, Workspace: WorkspaceRoot, Head: prHead,
	})
	if err != nil {
		t.Fatal(err)
	}
	closed := false
	defer func() {
		if !closed {
			_ = h.Close(s)
		}
	}()
	if s.TabID != "" || !s.Verify || s.Branch == "" {
		t.Fatalf("session = %+v, want a worktree of its own, no tab, marked verify", s)
	}
	if s.WorkspaceID == primary {
		t.Fatalf("workspace %s is the primary checkout — a verify run must never open there", primary)
	}
	if s.BaseCommit != prHead {
		t.Fatalf("BaseCommit = %q, want the PR head %q", s.BaseCommit, prHead)
	}
	wt := run("worktree", "list", "--porcelain")
	if !strings.Contains(wt, "refs/heads/"+s.Branch) {
		t.Fatalf("worktree list has no run branch:\n%s", wt)
	}
	path := ""
	for _, line := range strings.Split(wt, "\n") {
		if strings.HasPrefix(line, "worktree ") {
			path = strings.TrimPrefix(line, "worktree ")
		}
		if line == "branch refs/heads/"+s.Branch {
			break
		}
	}
	out, err := exec.Command("git", "-C", path, "rev-parse", "HEAD").Output()
	if err != nil || strings.TrimSpace(string(out)) != prHead {
		t.Fatalf("worktree HEAD = %q (%v), want %s", strings.TrimSpace(string(out)), err, prHead)
	}

	if err := h.Close(s); err != nil {
		t.Fatal(err)
	}
	closed = true
	after := run("worktree", "list", "--porcelain")
	if strings.Contains(after, s.Branch) {
		t.Errorf("worktree still registered after Close:\n%s", after)
	}
	if refs := run("for-each-ref", "--format=%(refname)", "refs/heads"); strings.Contains(refs, s.Branch) {
		t.Errorf("branch outlived the run: %s", refs)
	}
}

// Live probe for TASK-68, the author's path, against the real herdr:
//
//	LIVE_HERDR=1 go test ./internal/host/ -run TestLiveAuthorWorktreeProvisionAndClose -v
//
// A worktree-mode provision on a scratch repo with a bare origin, the push
// an agent makes before its PR, then Close: the registration must leave
// `git worktree list` and the pushed branch must go with it — while a
// second, unpushed run keeps its branch as the delivery's only copy.
func TestLiveAuthorWorktreeProvisionAndClose(t *testing.T) {
	if os.Getenv("LIVE_HERDR") == "" {
		t.Skip("set LIVE_HERDR=1 to probe the real herdr")
	}
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "init", "-q", "-b", "main")
	runGit(t, repo, "config", "user.email", "fleet@example.com")
	runGit(t, repo, "config", "user.name", "fleet")
	write(t, filepath.Join(repo, "base.txt"), "base\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-q", "-m", "first")
	origin := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, t.TempDir(), "init", "-q", "--bare", origin)
	runGit(t, repo, "remote", "add", "origin", origin)

	gitErr := func(root string, args ...string) error {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("git %v: %v (%s)", args, err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	registered := func(branch string) bool {
		out, err := exec.Command("git", "-C", repo, "worktree", "list", "--porcelain").CombinedOutput()
		if err != nil {
			t.Fatalf("git worktree list: %v (%s)", err, out)
		}
		return strings.Contains(string(out), "refs/heads/"+branch)
	}
	hasLocal := func(branch string) bool {
		return gitErr(repo, "rev-parse", "--verify", "refs/heads/"+branch) == nil
	}
	hasRemote := func(branch string) bool {
		return gitErr(repo, "ls-remote", "--exit-code", "origin", branch) == nil
	}

	h := New()
	s, err := h.Provision(Spec{Name: "TASK-68 live probe pushed", Repo: repo, Workspace: WorkspaceWorktree})
	if err != nil {
		t.Fatal(err)
	}
	closed := false
	defer func() {
		if !closed {
			_ = h.Close(s)
		}
	}()
	if s.Branch == "" || s.TabID != "" || !registered(s.Branch) {
		t.Fatalf("session = %+v, want a worktree of its own registered on %v", s, s.Branch != "")
	}
	// The delivery half: the agent pushes the run's branch before its PR.
	runGit(t, repo, "push", "-q", "-u", "origin", s.Branch)

	if err := h.Close(s); err != nil {
		t.Fatalf("Close = %v", err)
	}
	closed = true
	if registered(s.Branch) {
		t.Errorf("worktree still registered after Close: %s", s.Branch)
	}
	if hasLocal(s.Branch) {
		t.Errorf("pushed branch %s outlived the run locally", s.Branch)
	}
	if !hasRemote(s.Branch) {
		t.Errorf("pushed branch %s must still be on the origin — the retire never touches the remote", s.Branch)
	}

	// The other half: an unpushed run's branch is its only copy.
	s2, err := h.Provision(Spec{Name: "TASK-68 live probe unpushed", Repo: repo, Workspace: WorkspaceWorktree})
	if err != nil {
		t.Fatal(err)
	}
	closed2 := false
	defer func() {
		if !closed2 {
			_ = h.Close(s2)
		}
	}()
	if err := h.Close(s2); err != nil {
		t.Fatalf("Close = %v", err)
	}
	closed2 = true
	if registered(s2.Branch) {
		t.Errorf("worktree still registered after Close: %s", s2.Branch)
	}
	if !hasLocal(s2.Branch) {
		t.Errorf("unpushed branch %s was deleted — that would lose the delivery", s2.Branch)
	}
}
