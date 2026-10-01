package fleet

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s", args, out)
	}
}

// A path belongs to the queue whose checkout it is standing in. The case that
// matters most is the one that does not look like it belongs at all: a
// worktree run works in ~/.herdr/worktrees/…, nowhere near the repo the queue
// names, and it is where every dev run files its follow-ups from.
func TestProjectAtNamesTheQueueAPathStandsIn(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "myapp")
	other := filepath.Join(root, "other")
	for _, d := range []string{repo, other} {
		if err := os.MkdirAll(filepath.Join(d, "sub", "deep"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	git(t, repo, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "one")
	wt := filepath.Join(root, "elsewhere", "wt-1")
	git(t, repo, "worktree", "add", "-q", "-b", "fleet/x", wt)
	// git tracks no empty directory, so the worktree has none of repo's: make
	// the nested path the run would actually be standing in.
	if err := os.MkdirAll(filepath.Join(wt, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	sources := map[string]SourceConfig{
		"app":   {Dir: repo},
		"other": {Dir: other},
	}

	for _, c := range []struct{ name, path, want string }{
		{"the checkout itself", repo, "app"},
		{"nested inside it", filepath.Join(repo, "sub", "deep"), "app"},
		{"a worktree of it", wt, "app"},
		{"nested inside a worktree", filepath.Join(wt, "sub"), "app"},
		{"a different source", other, "other"},
		{"somewhere else entirely", root, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := ProjectAt(c.path, sources); got != c.want {
				t.Errorf("ProjectAt(%s) = %q, want %q", c.path, got, c.want)
			}
		})
	}
}

// Two queues where one checkout sits inside the other: the nearer one owns the
// path, or a fleet whose repos nest would file work into its own parent.
func TestProjectAtPrefersTheNearestCheckout(t *testing.T) {
	root := t.TempDir()
	inner := filepath.Join(root, "outer", "inner")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	sources := map[string]SourceConfig{
		"outer": {Dir: filepath.Join(root, "outer")},
		"inner": {Dir: inner},
	}
	if got := ProjectAt(inner, sources); got != "inner" {
		t.Errorf("ProjectAt = %q, want the nearest checkout", got)
	}
}

// A path that is not a repo at all must answer "no project", not blow up: the
// caller falls through to default_source and the fleet keeps working.
func TestProjectAtIsQuietWhereThereIsNoRepo(t *testing.T) {
	if got := ProjectAt(t.TempDir(), map[string]SourceConfig{"app": {Dir: "/nowhere"}}); got != "" {
		t.Errorf("ProjectAt = %q, want empty", got)
	}
}
