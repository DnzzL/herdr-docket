// Package sweep retires the worktree registrations runs left behind before
// TASK-68's Close learned to retire them (TASK-69): a pass over the fleet's
// checkouts that clears the pile of `fleet/…` worktrees whose runs already
// ended and will never close again.
//
// The rules of what may go are TASK-68's own, unchanged: the registration is
// pruned when the tree is clean, the branch is deleted only when a remote
// holds its exact tip, a dirty tree stays whole (host.RetireWorktree). What
// this package adds is the cross-check the one-off pile needs and a single
// Close never did: which run a registration belongs to, how that run ended,
// and whether an open workspace still sits on it. Only a run history clears
// — done, nothing parked, workspace closed — is swept; everything else is
// somebody's to decide, and the report says which reason kept it.
package sweep

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/DnzzL/herdr-docket/internal/herdr"
	"github.com/DnzzL/herdr-docket/internal/history"
	"github.com/DnzzL/herdr-docket/internal/host"
	"github.com/DnzzL/herdr-docket/internal/work"
)

// Options is how the sweep was asked for.
type Options struct {
	// DryRun prints the plan and retires nothing — the report a human
	// reviews before the real pass.
	DryRun bool
}

// Lister is `herdr worktree list` for one repo: every registration the
// primary checkout holds, and for each whether an open workspace still sits
// on it — herdr's durable answer to "did anybody come back to look".
type Lister interface {
	WorktreeList(repo string) ([]herdr.Worktree, error)
}

// Retire retires one registration with TASK-68's rules — the production one
// is host.RetireWorktree, kept a function so the sweep's decisions can be
// pinned without git.
type Retire func(repo, branch string) error

// Run is the production sweep: the live run history, herdr's worktree lists,
// and TASK-68's retire.
func Run(repos []string, opts Options, out io.Writer) error {
	runs, err := history.Runs("", 0)
	if err != nil {
		return err
	}
	return Sweep(repos, runs, herdr.Client{}, host.RetireWorktree, opts, out)
}

// Sweep walks the registrations of each repo and retires the ones history
// and open workspaces clear. Every registration that stays gets a line with
// its reason; every failure gets a line and counts toward the returned error,
// so a caller hears about what the sweep could not do instead of about what
// it managed to.
func Sweep(repos []string, runs []history.Record, list Lister, retire Retire, opts Options, out io.Writer) error {
	var retired, kept, failed int
	for _, repo := range repos {
		wts, err := list.WorktreeList(repo)
		if err != nil {
			fmt.Fprintf(out, "  error   %s — %v\n", repo, err)
			failed++
			continue
		}
		fmt.Fprintln(out, repo)
		var todo []herdr.Worktree
		for _, w := range wts {
			if !w.IsLinkedWorktree {
				continue // the human's own checkout, never a run's leftover
			}
			name := displayName(w)
			if why := keepReason(runFor(w, runs), w); why != "" {
				kept++
				fmt.Fprintf(out, "  keep    %s — %s\n", name, why)
				continue
			}
			if opts.DryRun {
				retired++ // counted as what it would retire
				fmt.Fprintf(out, "  plan    retire %s\n", name)
				continue
			}
			todo = append(todo, w)
		}
		if opts.DryRun || len(todo) == 0 {
			continue
		}
		// Retire, then re-read the repo: the report says what git says
		// happened, not what was attempted. A registration still present
		// after a nil retire is a dirty tree WorktreeRetire left whole; the
		// re-read failing leaves every outcome unknown rather than claimed.
		errs := make(map[string]error, len(todo))
		for _, w := range todo {
			if err := retire(repo, w.Branch); err != nil {
				errs[w.Path] = err
			}
		}
		after, lerr := list.WorktreeList(repo)
		present := map[string]bool{}
		for _, w := range after {
			present[w.Path] = true
		}
		for _, w := range todo {
			name := displayName(w)
			switch {
			case errs[w.Path] != nil:
				failed++
				fmt.Fprintf(out, "  error   %s — %v\n", name, errs[w.Path])
			case lerr != nil:
				failed++
				fmt.Fprintf(out, "  error   %s — outcome unknown, the worktrees cannot be re-read: %v\n", name, lerr)
			case present[w.Path]:
				kept++
				fmt.Fprintf(out, "  keep    %s — the tree is not clean, kept whole\n", name)
			default:
				retired++
				fmt.Fprintf(out, "  retired %s\n", name)
			}
		}
	}
	if opts.DryRun {
		fmt.Fprintf(out, "\ndry run: %d would be retired, %d kept, %d failed\n", retired, kept, failed)
	} else {
		fmt.Fprintf(out, "\nsweep: %d retired, %d kept, %d failed\n", retired, kept, failed)
	}
	if failed > 0 {
		return fmt.Errorf("worktree sweep: %d failures — see the report above", failed)
	}
	return nil
}

// runFor finds the run a registration belongs to, newest run first. Two
// keys, because the pile shows both shapes: the worktree's directory carries
// the branch Provision cut (slashes flattened, which is how herdr names the
// checkout) and survives an agent renaming the branch inside — half the
// registrations in the pile hold a branch like `task22/rebase` — while the
// checked-out branch catches a worktree whose directory history does not
// name. Nothing matched is an answer too: evidence that does not settle a
// registration keeps it.
func runFor(w herdr.Worktree, runs []history.Record) *history.Record {
	base := filepath.Base(w.Path)
	for i := range runs {
		b := runs[i].Branch
		if b == "" {
			continue
		}
		if strings.ReplaceAll(b, "/", "-") == base || b == w.Branch {
			return &runs[i]
		}
	}
	return nil
}

// keepReason answers why a registration must be left alone, or "" when the
// sweep may retire it. Only a run that ended done with nothing parked and
// whose workspace is closed is cleared — the state TASK-68's Close produces
// for every future run, and the one thing the pre-TASK-68 pile is full of.
// The fleet parks failed and blocked runs open as resume points, a run still
// going owns its worktree, and a registration history cannot name may be
// somebody's handiwork: all three stay for a human to decide.
func keepReason(r *history.Record, w herdr.Worktree) string {
	switch {
	case r == nil:
		return "no run in history"
	case r.Status == history.StatusScheduled,
		r.Status == history.StatusRunning,
		r.Status == history.StatusTimedOut:
		return fmt.Sprintf("run still in flight (%s)", r.RunID)
	case r.Status == history.StatusFailed,
		r.Status == history.StatusCancelled,
		r.Verdict == string(work.Failed),
		r.Verdict == string(work.Blocked):
		return fmt.Sprintf("resume point (%s/%s)", r.Status, verdictOrDash(r.Verdict))
	case w.OpenWorkspaceID != "":
		return fmt.Sprintf("workspace %s still open", w.OpenWorkspaceID)
	}
	return ""
}

func verdictOrDash(verdict string) string {
	if verdict == "" {
		return "-"
	}
	return verdict
}

// displayName is a registration's name in the report: its branch, or the
// path for a detached worktree that carries no branch to name it by.
func displayName(w herdr.Worktree) string {
	if w.Branch == "" {
		return w.Path
	}
	return w.Branch
}
