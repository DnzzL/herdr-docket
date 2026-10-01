package fleet

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// ProjectAt names the queue a path belongs to: the source whose checkout the
// path is standing in. It is how a `task create` with no -s lands in the right
// queue without anybody saying so — the agent is already inside the project it
// is working, and so is a human typing in their own repo.
//
// A worktree is the case that makes this worth a function. A run works in
// ~/.herdr/worktrees/<repo>/<branch>, which is nowhere near the directory the
// queue names, so a path that matches nothing is handed to git: the common git
// dir of a linked worktree is the main repository's, and that is a path the
// sources can be matched against. Every dev run files from a worktree, so this
// is the normal path and not the exception.
//
// An empty answer is not a failure: the caller falls through to the fleet's
// default_source. Nothing here fails a command — a queue that cannot be
// guessed is a queue somebody must name.
func ProjectAt(path string, sources map[string]SourceConfig) string {
	if path == "" || len(sources) == 0 {
		return ""
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if name := nearest(path, sources); name != "" {
		return name
	}
	return nearest(mainRepoOf(path), sources)
}

// nearest returns the source whose dir is the path or its closest ancestor.
// Closest, because a fleet whose checkouts nest would otherwise file work into
// the parent repo rather than the one it is standing in.
func nearest(path string, sources map[string]SourceConfig) string {
	if path == "" {
		return ""
	}
	best, bestLen := "", -1
	for name, s := range sources {
		dir := filepath.Clean(expandHome(s.Dir))
		if dir == path || strings.HasPrefix(path, dir+string(filepath.Separator)) {
			if len(dir) > bestLen {
				best, bestLen = name, len(dir)
			}
		}
	}
	return best
}

// mainRepoOf is the repository a path belongs to, which for a linked worktree
// is not the directory it sits in. git answers with the common git dir — the
// main checkout's .git, absolute from a worktree and relative from the
// checkout itself — and the repository is its parent. Anything that is not a
// repository answers with nothing at all.
func mainRepoOf(path string) string {
	out, err := exec.Command("git", "-C", path, "rev-parse", "--git-common-dir").Output()
	if err != nil {
		return ""
	}
	gitDir := strings.TrimSpace(string(out))
	if gitDir == "" {
		return ""
	}
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(path, gitDir)
	}
	return filepath.Dir(filepath.Clean(gitDir))
}
