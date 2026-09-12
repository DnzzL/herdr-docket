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

// exampleFleet is the scaffold written once, as FLEET.md, so the shared
// brief's existence is discoverable. Every line is a comment: an untouched
// scaffold is invisible to the prompt, and the brief starts speaking only once
// a human writes real prose into it.
const exampleFleet = `# FLEET.md — the fleet's shared brief
#
# The body of this file is prepended to every agent's persona. Put here what
# is true of the whole fleet, not of one agent: what the product is, who the
# human is, and the rules that never change.
#
# An absent, empty, or entirely commented file changes nothing — this scaffold
# does not reach any agent until you write an uncommented line into it.
#
# A brief might read:
#
#   We build acme, a tiny CRM for freelancers. The human is Dana; talk to them
#   plainly. Never push to main, and always leave the queue truer than you
#   found it.
`

// Init bootstraps the fleet directory: a git repo, an example agent, and —
// when the queue is a local one — the Backlog.md project itself. Safe to
// re-run: existing pieces are left alone.
func Init(s Settings) error {
	dir := s.Dir
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); os.IsNotExist(err) {
		if out, err := command(dir, "git", "init"); err != nil {
			return fmt.Errorf("git init: %s", out)
		}
	}
	if s.Source.local() {
		if err := initBacklog(dir); err != nil {
			return err
		}
	}
	if err := initBrief(dir); err != nil {
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

// initBrief lays down the commented FLEET.md scaffold the first time init
// runs. An existing brief — the human's — is left alone.
func initBrief(dir string) error {
	path := filepath.Join(dir, "FLEET.md")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return os.WriteFile(path, []byte(exampleFleet), 0o644)
}

// initBacklog lays down the Backlog.md project and makes sure the project
// config knows the fleet's lifecycle.
func initBacklog(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, "backlog")); os.IsNotExist(err) {
		if out, err := command(dir, "backlog", "init", "fleet", "--defaults"); err != nil {
			return fmt.Errorf("backlog init: %s", out)
		}
	}
	return patchStatuses(filepath.Join(dir, "backlog", "config.yml"))
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
