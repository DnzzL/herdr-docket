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
var backlogStatuses = statusesYAML(backlogmd.DefaultVocabulary())

func statusesYAML(v backlogmd.Vocabulary) string {
	words := v.List()
	quoted := make([]string, len(words))
	for i, s := range words {
		quoted[i] = `"` + s + `"`
	}
	return "statuses: [" + strings.Join(quoted, ", ") + "]"
}

const exampleRole = `# roles/example.md — a role every agent that names it opens with
#
# Name it from an AGENT.md frontmatter:
#
#   role: example
#
# A role is the method two posts share when they do the same job in different
# repos — how to read a ticket, when to stop, what "done" means to you. What is
# true of one repo belongs in that agent's persona; what is true of the whole
# fleet belongs in FLEET.md.
#
# A role an agent names but that has no file grounds that agent, on purpose: a
# run assembled without its method looks exactly like a run that had one.
`

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
	for _, q := range s.ownQueues() {
		if err := initBacklog(dir, q.Statuses); err != nil {
			return err
		}
	}
	if err := initBrief(dir); err != nil {
		return err
	}
	if err := initRoles(dir); err != nil {
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

// initRoles lays down the roles/ dir with a commented example the first time
// init runs. A role is optional and this scaffold reaches nobody until an
// agent names it — but a directory nobody can see is a feature nobody uses.
func initRoles(dir string) error {
	path := filepath.Join(dir, "roles", "example.md")
	if _, err := os.Stat(filepath.Dir(path)); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(exampleRole), 0o644)
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
func initBacklog(dir string, vocab backlogmd.Vocabulary) error {
	if _, err := os.Stat(filepath.Join(dir, "backlog")); os.IsNotExist(err) {
		if out, err := command(dir, "backlog", "init", "fleet", "--defaults"); err != nil {
			return fmt.Errorf("backlog init: %s", out)
		}
	}
	return patchStatusesTo(filepath.Join(dir, "backlog", "config.yml"), statusesYAML(vocab.OrDefault()))
}

// patchStatuses rewrites the statuses list in the Backlog.md config with the
// fleet's own lifecycle. Leaves the file alone when they are already there.
func patchStatuses(path string) error { return patchStatusesTo(path, backlogStatuses) }

func patchStatusesTo(path, statuses string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("backlog config: %w", err)
	}
	if strings.Contains(string(raw), statuses) {
		return nil
	}
	re := regexp.MustCompile(`(?m)^statuses:.*$`)
	if !re.Match(raw) {
		return fmt.Errorf("%s: no statuses line to patch", path)
	}
	return os.WriteFile(path, re.ReplaceAll(raw, []byte(statuses)), 0o644)
}

func command(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
