package host

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// runGit runs one git command in repo with an identity, failing the test on
// error. Every fact these tests assert is produced by the real git binary —
// a fake that hand-builds delivery facts would be exactly the boundary the
// suite agreed to pretend about (TASK-26).
func runGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestInspectReportsWhatAWorktreeRunProduced pins the three states the guard
// and the delivery record read: a fresh worktree, a worktree with changes
// nobody committed, and one whose work has landed on its branch.
func TestInspectReportsWhatAWorktreeRunProduced(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed; the delivery facts cannot be read")
	}
	repo := t.TempDir()
	runGit(t, repo, "init", "-q", "-b", "main")
	write(t, filepath.Join(repo, "base.txt"), "base\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-q", "-m", "base")

	wt := filepath.Join(t.TempDir(), "wt")
	runGit(t, repo, "worktree", "add", "-q", "-b", "fleet/run-1", wt)

	h := &live{ops: herdrOps{}, knobs: fast()}
	s := Session{Repo: repo, Branch: "fleet/run-1"}

	// A fresh worktree: nothing ahead, nothing dirty.
	d, err := h.Inspect(s)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if d.Commits != 0 || d.Dirty {
		t.Errorf("fresh worktree: got %+v, want zero commits and clean", d)
	}

	// Edits nobody committed: the exact state that must not be destroyed.
	write(t, filepath.Join(wt, "base.txt"), "base\nchanged\n")
	d, err = h.Inspect(s)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if !d.Dirty {
		t.Errorf("uncommitted edits: got %+v, want Dirty", d)
	}
	if d.Commits != 0 {
		t.Errorf("uncommitted edits: got %d commits, want 0 — they are not on any branch yet", d.Commits)
	}

	// Committed: the work exists on the branch, the tree is clean again.
	runGit(t, wt, "add", ".")
	runGit(t, wt, "commit", "-q", "-m", "the work")
	d, err = h.Inspect(s)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if d.Dirty || d.Commits != 1 {
		t.Errorf("committed work: got %+v, want clean with 1 commit", d)
	}
}

// A root-mode run has no branch — there is nothing to inspect, and answering
// as if there were would be the fleet inventing an output.
func TestInspectRefusesASessionWithoutABranch(t *testing.T) {
	h := &live{ops: herdrOps{}, knobs: fast()}
	if _, err := h.Inspect(Session{Repo: "/x"}); err == nil {
		t.Fatal("want an error for a session with no branch")
	}
}

// A branch that no worktree answers to is an error, not a clean bill: the
// worktree is where uncommitted work would live.
func TestInspectErrorsWhenTheWorktreeIsGone(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	repo := t.TempDir()
	runGit(t, repo, "init", "-q", "-b", "main")
	write(t, filepath.Join(repo, "base.txt"), "base\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-q", "-m", "base")

	h := &live{ops: herdrOps{}, knobs: fast()}
	if _, err := h.Inspect(Session{Repo: repo, Branch: "fleet/nope"}); err == nil {
		t.Fatal("want an error for a branch no worktree holds")
	}
}
