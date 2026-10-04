package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DnzzL/herdr-docket/internal/sweep"
)

func TestWorktreeSweepWantsTheSweepWord(t *testing.T) {
	for _, args := range [][]string{nil, {"clean"}, {"--dry-run"}} {
		var buf strings.Builder
		err := worktreeCmd(args, &buf)
		if err == nil || !strings.Contains(err.Error(), "usage: herdr-docket worktree sweep") {
			t.Errorf("worktreeCmd(%v) = %v, want a usage error", args, err)
		}
	}
}

func TestWorktreeSweepRejectsAnArgumentItDoesNotKnow(t *testing.T) {
	var buf strings.Builder
	err := worktreeCmd([]string{"sweep", "--force"}, &buf)
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Errorf("worktree sweep --force = %v, want it refused by name", err)
	}
	if !strings.Contains(err.Error(), "usage: herdr-docket worktree sweep") {
		t.Errorf("the refusal must show the usage: %v", err)
	}
}

// The sweep works the checkouts the fleet's agents actually run in — the
// repos runs cut their worktrees from — deduplicated, sorted, and with a
// workdir missing on this machine skipped out loud rather than erroring the
// whole sweep. The fleet's own sweep call is stubbed: this pins what the
// command hands it, not what the machine answers.
func TestWorktreeSweepHandsTheSweepTheCheckoutsTheAgentsWorkIn(t *testing.T) {
	existing := t.TempDir()
	fleetDir := t.TempDir()
	writeAgent := func(name, front string) {
		dir := filepath.Join(fleetDir, "agents", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "AGENT.md"), []byte(front+"persona\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeAgent("a", "---\nworkdir: "+existing+"\n---\n")
	writeAgent("b", "---\nworkdir: "+filepath.Join(t.TempDir(), "gone")+"\n---\n")
	writeAgent("c", "---\nworkdir: "+existing+"\n---\n") // same checkout as a
	writeAgent("d", "---\nworkdir: [broken\n---\n")      // does not load

	cfg := t.TempDir()
	t.Setenv("HERDR_PLUGIN_CONFIG_DIR", cfg)
	if err := os.WriteFile(filepath.Join(cfg, "fleet.yaml"), []byte("dir: "+fleetDir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	old := sweepWorktrees
	defer func() { sweepWorktrees = old }()
	var gotRepos []string
	var gotOpts sweep.Options
	sweepWorktrees = func(repos []string, opts sweep.Options, out io.Writer) error {
		gotRepos, gotOpts = repos, opts
		return nil
	}

	var buf strings.Builder
	if err := worktreeCmd([]string{"sweep", "--dry-run"}, &buf); err != nil {
		t.Fatalf("worktree sweep: %v", err)
	}
	if len(gotRepos) != 1 || gotRepos[0] != existing {
		t.Errorf("sweep got repos %v, want the one checkout the agents share (%s)", gotRepos, existing)
	}
	if !gotOpts.DryRun {
		t.Error("--dry-run did not reach the sweep")
	}
	out := buf.String()
	if !strings.Contains(out, "skip") || !strings.Contains(out, "no checkout here") {
		t.Errorf("the missing workdir is not reported:\n%s", out)
	}
	if !strings.Contains(out, "agent d") {
		t.Errorf("the agent that did not load is not reported:\n%s", out)
	}
}

// A sweep that fails says so: the error is the command's, not swallowed.
func TestWorktreeSweepPassesTheSweepsFailureOn(t *testing.T) {
	fleetDir := t.TempDir()
	dir := filepath.Join(fleetDir, "agents", "a")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "AGENT.md"), []byte("---\nworkdir: "+t.TempDir()+"\n---\npersona\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := t.TempDir()
	t.Setenv("HERDR_PLUGIN_CONFIG_DIR", cfg)
	if err := os.WriteFile(filepath.Join(cfg, "fleet.yaml"), []byte("dir: "+fleetDir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	old := sweepWorktrees
	defer func() { sweepWorktrees = old }()
	sweepWorktrees = func(repos []string, opts sweep.Options, out io.Writer) error {
		return errors.New("boom")
	}

	var buf strings.Builder
	err := worktreeCmd([]string{"sweep"}, &buf)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("worktree sweep = %v, want the sweep's failure", err)
	}
}
