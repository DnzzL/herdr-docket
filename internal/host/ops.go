package host

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/DnzzL/herdr-docket/internal/herdr"
)

// ops is this package's internal seam: the individual Herdr calls, one method
// each. It is unexported on purpose — callers of Host must not have to know
// that starting an agent can answer "the pane is not a shell yet", only that
// the work either happened or didn't. The tests script it.
type ops interface {
	WorktreeCreate(repo, branch, base, label string) (workspaceID, paneID string, err error)
	WorkspaceCreate(cwd, label string) (workspaceID, paneID string, err error)
	WorkspaceClose(workspaceID string) error

	// PrimaryWorkspace is the workspace a repo is already open in, or "" when
	// it is not open at all. TabCreate borrows that workspace for one run and
	// TabClose gives it back — closing the workspace would take the sibling
	// runs, and the human's own tab, with it.
	PrimaryWorkspace(repoRoot string) (workspaceID string, err error)
	TabCreate(workspaceID, cwd, label string) (paneID, tabID string, err error)
	TabClose(tabID string) error

	AgentStart(name, kind, paneID string, extraArgs []string) error
	AgentSubmit(target, text string) error
	AgentSubmitPending(paneID string) error
	AgentStatus(target string) (string, error)
	AgentWait(target string, timeout time.Duration) error

	// WorktreePath finds the directory a git worktree for branch checks out
	// of repo, or errors when no worktree holds the branch.
	WorktreePath(repo, branch string) (string, error)
	// WorktreeDiscard throws away a run's own worktree and the branch it was
	// cut on — the teardown of a verify session (TASK-65), never an author's
	// delivery. The tree is disposable by contract, so the removal is forced,
	// and the branch can only go once the worktree that held it is gone.
	WorktreeDiscard(repo, branch string) error
	// WorktreeRetire is the lighter teardown an author's run owes (TASK-68):
	// the registration and checkout are pruned when the tree is clean, and
	// the branch is deleted only when every commit it holds is already pushed
	// — an unpushed branch is the delivery's only copy. A dirty tree keeps
	// everything: uncommitted work is never destroyed by a teardown.
	WorktreeRetire(repo, branch string) error
	// WorktreeDirty reports whether the worktree at dir holds changes that
	// are not committed.
	WorktreeDirty(dir string) (bool, error)
	// CommitsAhead counts how many commits branch has beyond repo's own
	// checkout (HEAD).
	CommitsAhead(repo, branch string) (int, error)

	// CommitAt resolves repo's ref to the commit it names, read-only. It is
	// how a provision records what it branched from, and it answers only
	// after the fact — it never writes or moves a ref.
	CommitAt(repo, ref string) (string, error)

	// HasCode reports whether err is a Herdr API error with the given code.
	// It travels with the ops so a fake can answer for its own errors.
	HasCode(err error, code string) bool

	// Notify raises Herdr's desktop notification: a report to a human about
	// something that already happened, never a step of the work itself.
	// sound is herdr's own word for the audio (empty for none).
	Notify(title, body, sound string) error
}

// herdrOps is the production ops. The Herdr calls come from the embedded
// client; error-code matching is not Herdr's business, so it lives here.
type herdrOps struct {
	herdr.Client
}

func (herdrOps) HasCode(err error, code string) bool { return herdr.HasCode(err, code) }

func (herdrOps) PrimaryWorkspace(repoRoot string) (string, error) {
	var c herdr.Client
	return c.PrimaryWorkspace(repoRoot)
}

func (herdrOps) TabCreate(workspaceID, cwd, label string) (string, string, error) {
	var c herdr.Client
	return c.TabCreate(workspaceID, cwd, label)
}

func (herdrOps) TabClose(tabID string) error {
	var c herdr.Client
	return c.TabClose(tabID)
}

// git is the git side of the ops: three reads, no writes. The fleet has no
// git of its own — these exist so Inspect can answer before a workspace is
// torn down, and every one of them is a question git already knows how to
// answer about a checkout it made.
func gitOutput(repo string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s in %s: %w (%s)", strings.Join(args, " "), repo, err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// worktreePath is the git read behind WorktreePath: the directory a worktree
// holding branch checks out of, or a refusal naming the repo and the branch.
func worktreePath(repo, branch string) (string, error) {
	out, err := gitOutput(repo, "worktree", "list", "--porcelain")
	if err != nil {
		return "", err
	}
	want := "branch refs/heads/" + branch
	var path string
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			path = strings.TrimPrefix(line, "worktree ")
		case line == want:
			if path == "" {
				break // unreachable in git's output order; guard anyway
			}
			return path, nil
		}
	}
	return "", fmt.Errorf("no worktree of %s holds branch %q", repo, branch)
}

func (herdrOps) WorktreePath(repo, branch string) (string, error) {
	return worktreePath(repo, branch)
}

// WorktreeDiscard is the write side of the git ops — the one the fleet owns:
// a verify run's worktree and branch are created by the host and destroyed by
// it, so nothing the run made is left registered in the human's repo (TASK-65,
// AC #2). A worktree that is already gone (herdr removed it, or the workspace
// was closed under the run) still loses its branch; a worktree that still
// exists is removed first, since git refuses to delete a branch checked out
// somewhere.
func (herdrOps) WorktreeDiscard(repo, branch string) error {
	if path, err := worktreePath(repo, branch); err == nil {
		if _, err := gitOutput(repo, "worktree", "remove", "--force", path); err != nil {
			return err
		}
	}
	_, err := gitOutput(repo, "branch", "-D", branch)
	return err
}

// WorktreeRetire is the lighter teardown an author run's Close owes
// (TASK-68): the registration and checkout go once the tree is clean, and
// the branch goes only when every commit it holds is already pushed — the
// agent's own push wrote the remote-tracking refs into the shared .git, so
// the question is answered offline (the fleet never fetches). A dirty tree
// keeps everything: uncommitted work is never destroyed by a teardown
// (TASK-23), it stays registered for a human to look at.
func (herdrOps) WorktreeRetire(repo, branch string) error {
	if path, err := worktreePath(repo, branch); err == nil {
		dirty, err := worktreeDirty(path)
		if err != nil {
			return err
		}
		if dirty {
			return nil
		}
		if _, err := gitOutput(repo, "worktree", "remove", path); err != nil {
			return err
		}
	}
	// The registration is gone — or was never there. Deleting a branch that
	// is still checked out somewhere fails in git, so a path that went wrong
	// above can never cost the branch.
	pushed, err := branchPushed(repo, branch)
	if err != nil {
		return err
	}
	if !pushed {
		return nil
	}
	_, err = gitOutput(repo, "branch", "-D", branch)
	return err
}

// branchPushed reports whether every commit branch holds is reachable from a
// remote-tracking ref: zero commits unique to it means the delivery, if any,
// is on the forge and the local branch is a copy.
func branchPushed(repo, branch string) (bool, error) {
	out, err := gitOutput(repo, "rev-list", "--count", branch, "--not", "--remotes")
	if err != nil {
		return false, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return false, fmt.Errorf("git rev-list reported %q unpushed commits for %q: %w", strings.TrimSpace(out), branch, err)
	}
	return n == 0, nil
}

func (herdrOps) WorktreeDirty(dir string) (bool, error) {
	return worktreeDirty(dir)
}

// worktreeDirty is the git read behind WorktreeDirty: whether the worktree
// at dir holds changes that are not committed.
func worktreeDirty(dir string) (bool, error) {
	out, err := gitOutput(dir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func (herdrOps) CommitsAhead(repo, branch string) (int, error) {
	out, err := gitOutput(repo, "rev-list", "--count", "HEAD.."+branch)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, fmt.Errorf("git rev-list reported %q commits for %q: %w", strings.TrimSpace(out), branch, err)
	}
	return n, nil
}

func (herdrOps) CommitAt(repo, ref string) (string, error) {
	out, err := gitOutput(repo, "rev-parse", ref+"^{commit}")
	if err != nil {
		return "", err
	}
	commit := strings.TrimSpace(out)
	if commit == "" {
		return "", fmt.Errorf("git rev-parse named no commit for %q in %s", ref, repo)
	}
	return commit, nil
}
