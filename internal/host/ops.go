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
	WorktreeCreate(repo, branch, label string) (workspaceID, paneID string, err error)
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
	// WorktreeDirty reports whether the worktree at dir holds changes that
	// are not committed.
	WorktreeDirty(dir string) (bool, error)
	// CommitsAhead counts how many commits branch has beyond repo's own
	// checkout (HEAD).
	CommitsAhead(repo, branch string) (int, error)

	// HasCode reports whether err is a Herdr API error with the given code.
	// It travels with the ops so a fake can answer for its own errors.
	HasCode(err error, code string) bool
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

func (herdrOps) WorktreePath(repo, branch string) (string, error) {
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

func (herdrOps) WorktreeDirty(dir string) (bool, error) {
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
