package sweep

import (
	"errors"
	"strings"
	"testing"

	"github.com/DnzzL/herdr-docket/internal/herdr"
	"github.com/DnzzL/herdr-docket/internal/history"
)

// fakeLister scripts `herdr worktree list` for a repo: the registrations it
// holds right now, and per-repo failures. A retire that succeeds removes its
// entry here, the way git removes the real registration, so the sweep's
// post-check reads the same answer it would read from the machine.
type fakeLister struct {
	lists map[string][]herdr.Worktree
	errs  map[string]error
}

func (f *fakeLister) WorktreeList(repo string) ([]herdr.Worktree, error) {
	if err := f.errs[repo]; err != nil {
		return nil, err
	}
	return f.lists[repo], nil
}

func (f *fakeLister) drop(repo, branch string) {
	kept := f.lists[repo][:0:0]
	for _, w := range f.lists[repo] {
		if w.Branch != branch {
			kept = append(kept, w)
		}
	}
	f.lists[repo] = kept
}

// linked is a run's own registration: a linked worktree under a branch.
func linked(branch, path string) herdr.Worktree {
	return herdr.Worktree{Branch: branch, Path: path, IsLinkedWorktree: true}
}

// linkedOpen is one whose workspace is still open — herdr's durable signal
// that somebody may still be in it.
func linkedOpen(branch, path, workspace string) herdr.Worktree {
	w := linked(branch, path)
	w.OpenWorkspaceID = workspace
	return w
}

func run(id, branch string, status history.Status, verdict string) history.Record {
	return history.Record{RunID: id, Task: id, Branch: branch, Status: status, Verdict: verdict}
}

// dirFor is where herdr checks a branch out: the branch with its slashes
// flattened. The directory is what names the run when the branch inside has
// been renamed since.
func dirFor(branch string) string {
	return "/w/" + strings.ReplaceAll(branch, "/", "-")
}

// The fleet's rule for the pre-TASK-68 pile, pinned here: a registration is
// retired only for a run that ended done with nothing parked and whose
// workspace is closed. In flight, resume points, open workspaces and
// anything history cannot name all stay — the reasons are the report.
func TestSweepRetiresOnlyWhatHistoryAndWorkspacesClear(t *testing.T) {
	const repo = "/r"
	branches := struct {
		done, noVerdict, blocked, failed, running, renamed string
	}{
		done:      "fleet/docket-task-1-done-20261001-0101",
		noVerdict: "fleet/docket-task-2-quiet-20261001-0202",
		blocked:   "fleet/docket-task-3-blocked-20261001-0303",
		failed:    "fleet/docket-task-4-failed-20261001-0404",
		running:   "fleet/docket-task-5-running-20261001-0505",
		renamed:   "fleet/docket-task-6-renamed-20261001-0606",
	}
	runs := []history.Record{
		run("docket/TASK-1-1", branches.done, history.StatusDone, "done"),
		run("docket/TASK-2-2", branches.noVerdict, history.StatusDone, ""),
		run("docket/TASK-3-3", branches.blocked, history.StatusDone, "blocked"),
		run("docket/TASK-4-4", branches.failed, history.StatusFailed, "failed"),
		run("docket/TASK-5-5", branches.running, history.StatusRunning, ""),
		run("docket/TASK-6-6", branches.renamed, history.StatusDone, "done"),
	}
	lister := &fakeLister{lists: map[string][]herdr.Worktree{repo: {
		// The human's own checkout: never a run's leftover, never touched.
		{Branch: "main", Path: repo, IsLinkedWorktree: false},
		linked(branches.done, dirFor(branches.done)),
		linked(branches.noVerdict, dirFor(branches.noVerdict)),
		linked(branches.blocked, dirFor(branches.blocked)),
		linked(branches.failed, dirFor(branches.failed)),
		linked(branches.running, dirFor(branches.running)),
		// The agent renamed the branch inside — the directory still names the
		// run — and the workspace was left open: kept for the open workspace.
		linkedOpen("task22/rebase", dirFor(branches.renamed), "w8P"),
		// Nothing in history claims these; the evidence does not settle them.
		linked("pr41-fix", "/tmp/pr41"),
		linked("", "/tmp/wt-main"),
	}}}
	var retired []string
	retire := func(r, branch string) error {
		retired = append(retired, r+"@"+branch)
		lister.drop(r, branch)
		return nil
	}

	var buf strings.Builder
	if err := Sweep([]string{repo}, runs, lister, retire, Options{}, &buf); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	want := []string{repo + "@" + branches.done, repo + "@" + branches.noVerdict}
	if len(retired) != len(want) {
		t.Fatalf("retired %v, want exactly %v", retired, want)
	}
	for _, w := range want {
		if !contains(retired, w) {
			t.Errorf("retired %v, missing %s", retired, w)
		}
	}
	out := buf.String()
	for _, keep := range []string{
		branches.blocked + " — resume point (done/blocked)",
		branches.failed + " — resume point (failed/failed)",
		branches.running + " — run still in flight (docket/TASK-5-5)",
		"task22/rebase — workspace w8P still open",
		"pr41-fix — no run in history",
		"/tmp/wt-main — no run in history",
	} {
		if !strings.Contains(out, keep) {
			t.Errorf("report missing keep line %q:\n%s", keep, out)
		}
	}
	if strings.Contains(out, "  keep    main —") || strings.Contains(out, "  retired main\n") {
		t.Errorf("the primary checkout must not be reported as a registration:\n%s", out)
	}
	if !strings.Contains(out, "2 retired, 6 kept, 0 failed") {
		t.Errorf("summary missing or wrong:\n%s", out)
	}
}

// A dry run is the report a human reviews before the sweep: same plan, same
// keeps, not one git write.
func TestSweepDryRunPlansAndTouchesNothing(t *testing.T) {
	const repo = "/r"
	const branch = "fleet/docket-task-1-done-20261001-0101"
	runs := []history.Record{run("docket/TASK-1-1", branch, history.StatusDone, "done")}
	lister := &fakeLister{lists: map[string][]herdr.Worktree{repo: {
		linked(branch, dirFor(branch)),
	}}}
	retired := 0
	retire := func(r, b string) error { retired++; return nil }

	var buf strings.Builder
	if err := Sweep([]string{repo}, runs, lister, retire, Options{DryRun: true}, &buf); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if retired != 0 {
		t.Errorf("dry run retired %d registrations, want 0", retired)
	}
	out := buf.String()
	if !strings.Contains(out, "plan    retire "+branch) {
		t.Errorf("dry run does not plan the retirement:\n%s", out)
	}
	if !strings.Contains(out, "1 would be retired, 0 kept, 0 failed") {
		t.Errorf("dry-run summary missing or wrong:\n%s", out)
	}
}

// A dirty tree is not pruned: WorktreeRetire leaves the registration whole
// (TASK-23's guarantee), and the report says kept — not retired — because it
// reads what happened, not what was attempted.
func TestSweepKeepsWhatTheRetireCouldNotPrune(t *testing.T) {
	const repo = "/r"
	const branch = "fleet/docket-task-1-dirty-20261001-0101"
	runs := []history.Record{run("docket/TASK-1-1", branch, history.StatusDone, "done")}
	lister := &fakeLister{lists: map[string][]herdr.Worktree{repo: {
		linked(branch, dirFor(branch)),
	}}}
	retire := func(r, b string) error { return nil } // dirty: nothing removed

	var buf strings.Builder
	if err := Sweep([]string{repo}, runs, lister, retire, Options{}, &buf); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "keep    "+branch+" — the tree is not clean, kept whole") {
		t.Errorf("the unpruned registration is not reported as kept:\n%s", out)
	}
	if !strings.Contains(out, "0 retired, 1 kept, 0 failed") {
		t.Errorf("summary counts it as retired or failed:\n%s", out)
	}
}

// One registration that cannot be retired is a line and a non-zero exit, not
// a sweep that stops: the rest of the pile still gets swept, and the failure
// reaches the caller.
func TestSweepReportsARetireErrorAndKeepsGoing(t *testing.T) {
	const repo = "/r"
	const bad = "fleet/docket-task-1-bad-20261001-0101"
	const good = "fleet/docket-task-2-good-20261001-0202"
	runs := []history.Record{
		run("docket/TASK-1-1", bad, history.StatusDone, "done"),
		run("docket/TASK-2-2", good, history.StatusDone, "done"),
	}
	lister := &fakeLister{lists: map[string][]herdr.Worktree{repo: {
		linked(bad, dirFor(bad)),
		linked(good, dirFor(good)),
	}}}
	retire := func(r, b string) error {
		if b == bad {
			return errors.New("git refused")
		}
		lister.drop(r, b)
		return nil
	}

	var buf strings.Builder
	err := Sweep([]string{repo}, runs, lister, retire, Options{}, &buf)
	if err == nil {
		t.Fatal("a failed retirement must fail the sweep")
	}
	out := buf.String()
	if !strings.Contains(out, "error   "+bad+" — git refused") {
		t.Errorf("the failure is not reported:\n%s", out)
	}
	if !strings.Contains(out, "retired "+good) {
		t.Errorf("the sweep stopped instead of continuing:\n%s", out)
	}
	if !strings.Contains(out, "1 retired, 0 kept, 1 failed") {
		t.Errorf("summary counts wrong:\n%s", out)
	}
}

// A repo whose worktrees cannot even be listed is one error line; the other
// repos still get swept, and the caller hears about it.
func TestSweepReportsARepoItCannotListAndSweepsTheRest(t *testing.T) {
	const broken, fine = "/broken", "/fine"
	const branch = "fleet/docket-task-1-done-20261001-0101"
	runs := []history.Record{run("docket/TASK-1-1", branch, history.StatusDone, "done")}
	lister := &fakeLister{
		lists: map[string][]herdr.Worktree{
			fine: {linked(branch, dirFor(branch))},
		},
		errs: map[string]error{broken: errors.New("herdr: no such repo")},
	}
	retire := func(r, b string) error { lister.drop(r, b); return nil }

	var buf strings.Builder
	err := Sweep([]string{broken, fine}, runs, lister, retire, Options{}, &buf)
	if err == nil {
		t.Fatal("an unlistable repo must fail the sweep")
	}
	out := buf.String()
	if !strings.Contains(out, "error   /broken — herdr: no such repo") {
		t.Errorf("the repo failure is not reported:\n%s", out)
	}
	if !strings.Contains(out, "retired "+branch) {
		t.Errorf("one broken repo stopped the sweep:\n%s", out)
	}
	if !strings.Contains(out, "1 retired, 0 kept, 1 failed") {
		t.Errorf("summary counts wrong:\n%s", out)
	}
}

// The branch may have been renamed inside the worktree — the directory still
// names the run Provision cut, and that is what maps it back to history.
func TestSweepMatchesTheRunByWorktreeDirectoryWhenTheBranchWasRenamed(t *testing.T) {
	const repo = "/r"
	const provBranch = "fleet/docket-task-22-20261001-1814"
	runs := []history.Record{run("docket/TASK-22-1", provBranch, history.StatusDone, "done")}
	lister := &fakeLister{lists: map[string][]herdr.Worktree{repo: {
		linked("task22/rebase", dirFor(provBranch)),
	}}}
	var retired []string
	retire := func(r, b string) error {
		retired = append(retired, b)
		lister.drop(r, b)
		return nil
	}

	var buf strings.Builder
	if err := Sweep([]string{repo}, runs, lister, retire, Options{}, &buf); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if len(retired) != 1 || retired[0] != "task22/rebase" {
		t.Errorf("retired %v, want the registration's own branch task22/rebase", retired)
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
