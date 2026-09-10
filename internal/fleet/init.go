package fleet

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/DnzzL/herdr-fleet/internal/work/backlogmd"
)

// backlogStatuses is the lifecycle the Backlog.md adapter reads and writes,
// as YAML. Built from the adapter's own list rather than retyped, so a status
// added there cannot go missing here — the CLI refuses any status the project
// config does not declare. Written to the config file because the CLI has no
// `config set statuses`: the file is the only supported channel.
var backlogStatuses = statusesYAML()

func statusesYAML() string {
	quoted := make([]string, len(backlogmd.Statuses))
	for i, s := range backlogmd.Statuses {
		quoted[i] = `"` + s + `"`
	}
	return "statuses: [" + strings.Join(quoted, ", ") + "]"
}

const exampleAgent = `---
# model: claude-sonnet-5
workdir: ~/Projects/example
workspace: root        # or worktree: a fresh branch per run
# timeout_minutes: 60
---

You are the example fleet agent. Describe here who this agent is, what it
owns, and how it should work: the persona every one of its runs opens with.

Delete this folder or rename it to create your first real agent, then assign
it a task with the fleet CLI:  herdr-fleet task create "..." -a <agent-name>
`

// Init bootstraps the fleet directory: a git repo, a Backlog.md project with
// the fleet's statuses, and an example agent. Safe to re-run: existing pieces
// are left alone.
func Init(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); os.IsNotExist(err) {
		if out, err := command(dir, "git", "init"); err != nil {
			return fmt.Errorf("git init: %s", out)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "backlog")); os.IsNotExist(err) {
		if out, err := command(dir, "backlog", "init", "fleet", "--defaults"); err != nil {
			return fmt.Errorf("backlog init: %s", out)
		}
	}
	if err := patchStatuses(filepath.Join(dir, "backlog", "config.yml")); err != nil {
		return err
	}
	example := filepath.Join(dir, "agents", "example")
	if _, err := os.Stat(filepath.Join(dir, "agents")); os.IsNotExist(err) {
		if err := os.MkdirAll(example, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(example, "AGENT.md"), []byte(exampleAgent), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// patchStatuses rewrites the statuses list in the Backlog.md config. Leaves
// the file alone when the fleet statuses are already there.
func patchStatuses(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("backlog config: %w", err)
	}
	if strings.Contains(string(raw), backlogStatuses) {
		return nil
	}
	re := regexp.MustCompile(`(?m)^statuses:.*$`)
	if !re.Match(raw) {
		return fmt.Errorf("%s: no statuses line to patch", path)
	}
	return os.WriteFile(path, re.ReplaceAll(raw, []byte(backlogStatuses)), 0o644)
}

func command(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
