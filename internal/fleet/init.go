package fleet

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/DnzzL/herdr-docket/internal/hostpath"
	"github.com/DnzzL/herdr-docket/internal/work/backlogmd"
)

// statusesYAML writes a status list as the one-line form Backlog.md's own
// config uses. The CLI has no `config set statuses`: the file is the only
// supported channel.
func statusesYAML(words []string) string {
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

// The pipeline's two roles (ADR 0021): the author's method and the
// verifier's, shipped with plain init so a fleet has both from its first run.
// Verbatim as docs/examples.md shows them — a drift test pins the two.
//
//go:embed roles/dev.md
var devRole string

//go:embed roles/reviewer.md
var reviewerRole string

var pipelineRoles = []struct{ name, body string }{
	{"dev", devRole},
	{"reviewer", reviewerRole},
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
it a task with the fleet CLI:  herdr-docket task create "..." -a <agent-name>
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

// Init bootstraps the fleet directory: a git repo, an example agent and the
// pipeline's two roles. No queue: that lives in a project repo (InitRepo). Safe to re-run: existing pieces are left alone. What it
// does to those two roles is reported to out: they are the files an owner
// edits, and a re-run must say it kept them.
func Init(s Settings, out io.Writer) error {
	dir := s.Dir
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); os.IsNotExist(err) {
		if out, err := command(dir, "git", "init"); err != nil {
			return fmt.Errorf("git init: %s", out)
		}
	}
	if err := initBrief(dir); err != nil {
		return err
	}
	if err := initRoles(dir, out); err != nil {
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
// The pipeline's two roles land on every run that lacks them; a role the
// fleet already has is its owner's and is kept.
func initRoles(dir string, out io.Writer) error {
	path := filepath.Join(dir, "roles", "example.md")
	if _, err := os.Stat(filepath.Dir(path)); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(exampleRole), 0o644); err != nil {
			return err
		}
	}
	for _, r := range pipelineRoles {
		rolePath := filepath.Join(dir, "roles", r.name+".md")
		if _, err := os.Stat(rolePath); err == nil {
			fmt.Fprintf(out, "skipped roles/%s.md — it already exists\n", r.name)
			continue
		}
		if err := os.WriteFile(rolePath, []byte(r.body), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(out, "wrote roles/%s.md\n", r.name)
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

// issueTracker is what docs/agents/issue-tracker.md says in a repo init
// prepares: how the skills publish, fetch and resolve Specs and Tickets in
// its Backlog.md project. It is the seam those skills read.
//
//go:embed issue-tracker.md
var issueTracker string

// InitRepo prepares a project repo to be the fleet's queue (ADR 0023): the
// fleet itself, as Init lays it down; then in repo a Backlog.md project
// speaking the fleet's statuses, the tracker doc the skills read, and the
// repo named as a source in fleet.yaml. Safe to re-run: a second run changes
// nothing. What it did, and what it left, is reported to out.
func InitRepo(s Settings, repo string, out io.Writer) error {
	repo, err := filepath.Abs(expandHome(repo))
	if err != nil {
		return err
	}
	if fi, err := os.Stat(repo); err != nil || !fi.IsDir() {
		return fmt.Errorf("init: %s is not a directory", repo)
	}
	if err := Init(s, out); err != nil {
		return err
	}
	known, statuses := s.sourceFor(repo)
	if err := initBacklog(repo, statuses); err != nil {
		return err
	}
	if err := initTrackerDoc(repo, out); err != nil {
		return err
	}
	if known {
		fmt.Fprintf(out, "fleet.yaml already names %s\n", repo)
		return nil
	}
	return addSource(s, repo, out)
}

// sourceFor reports whether a Backlog.md source of s already works repo, and
// the status words it speaks there.
func (s Settings) sourceFor(repo string) (bool, backlogmd.Vocabulary) {
	all := s.Sources
	if len(all) == 0 {
		all = map[string]SourceConfig{"": s.Source}
	}
	for _, c := range all {
		if c.kind() == kindBacklogmd && c.Dir != "" && filepath.Clean(c.dirFrom(s.Dir)) == repo {
			return true, c.Statuses
		}
	}
	return false, backlogmd.Vocabulary{}
}

// initTrackerDoc writes the tracker doc the skills read. One the repo already
// has is its owner's: kept, and said so.
func initTrackerDoc(repo string, out io.Writer) error {
	const rel = "docs/agents/issue-tracker.md"
	path := filepath.Join(repo, rel)
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(out, "kept %s — it already exists\n", rel)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(issueTracker), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(out, "wrote %s\n", rel)
	return nil
}

// addSource names repo as the fleet's queue in fleet.yaml. A fleet.yaml with
// no queue yet gets a source: block appended, the rest of the file untouched.
// One that already names another queue is the owner's YAML: rewriting it
// (source: into sources:) would lose its comments and layout, so the block to
// paste is printed instead and the file is left as it is.
func addSource(s Settings, repo string, out io.Writer) error {
	path := filepath.Join(hostpath.ConfigDir(), "fleet.yaml")
	if len(s.Sources) > 0 || s.Source != (SourceConfig{}) {
		name := filepath.Base(repo)
		fmt.Fprintf(out, "fleet.yaml already names another queue; left it as it is. To add this repo, list it under sources: (turning a source: block into one entry of it):\n\nsources:\n  %s:\n    kind: backlogmd\n    dir: %s\n\n", name, repo)
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(raw) > 0 && raw[len(raw)-1] != '\n' {
		raw = append(raw, '\n')
	}
	raw = append(raw, fmt.Sprintf("source:\n  kind: backlogmd\n  dir: %s\n", repo)...)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(out, "named %s as the queue in %s\n", repo, path)
	return nil
}

// initBacklog lays down the repo's Backlog.md project when it has none, and
// makes sure its config knows the fleet's lifecycle.
func initBacklog(dir string, vocab backlogmd.Vocabulary) error {
	if _, err := os.Stat(filepath.Join(dir, "backlog")); os.IsNotExist(err) {
		if out, err := command(dir, "backlog", "init", filepath.Base(dir), "--defaults"); err != nil {
			return fmt.Errorf("backlog init: %s", out)
		}
	}
	return patchStatusesTo(filepath.Join(dir, "backlog", "config.yml"), backlogmd.OrDefault(vocab).List())
}

// patchStatuses makes the Backlog.md config declare every status the
// adapter reads and writes: the CLI refuses any status the project config
// does not declare.
func patchStatuses(path string) error { return patchStatusesTo(path, backlogmd.Statuses) }

// patchStatusesTo adds the statuses in want the config lacks, after the ones
// it has: a repo's own columns are its humans' board, never the fleet's to
// drop. Leaves the file alone when nothing is missing.
func patchStatusesTo(path string, want []string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("backlog config: %w", err)
	}
	var cfg struct {
		Statuses []string `yaml:"statuses"`
	}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	merged := cfg.Statuses
	for _, w := range want {
		if !slices.Contains(merged, w) {
			merged = append(merged, w)
		}
	}
	if len(merged) == len(cfg.Statuses) {
		return nil
	}
	re := regexp.MustCompile(`(?m)^statuses:.*$`)
	line := re.Find(raw)
	if line == nil || !strings.Contains(string(line), "[") {
		return fmt.Errorf("%s: no one-line statuses list to patch — set it by hand to %s", path, statusesYAML(merged))
	}
	return os.WriteFile(path, re.ReplaceAll(raw, []byte(statusesYAML(merged))), 0o644)
}

func command(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
