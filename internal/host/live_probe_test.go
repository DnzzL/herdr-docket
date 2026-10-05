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
