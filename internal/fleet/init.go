package fleet

import (
	_ "embed"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/DnzzL/herdr-docket/internal/hostpath"
	"github.com/DnzzL/herdr-docket/internal/work/backlogmd"
	"github.com/DnzzL/herdr-docket/internal/work/github"
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
// pipeline's two roles. No queue: that lives in a project repo (InitRepo).
// Safe to re-run: existing pieces are left alone. What it does to those two
// roles is reported to out: they are the files an owner edits, and a re-run
// must say it kept them.
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
	repo, err := repoDir(repo)
	if err != nil {
		return err
	}
	if err := Init(s, out); err != nil {
		return err
	}
	// A Backlog.md source with no dir is a fleet from before ADR 0023, whose
	// queue lived in the fleet dir: this repo is where it goes, statuses and all.
	c, found := s.find(func(c SourceConfig) bool {
		return c.kind() == kindBacklogmd && (c.Dir == "" || filepath.Clean(c.dirFrom(s.Dir)) == repo)
	})
	if err := initBacklog(repo, c.Statuses, out); err != nil {
		return err
	}
	if err := initTrackerDoc(repo, issueTracker, out); err != nil {
		return err
	}
	if found && c.Dir != "" {
		fmt.Fprintf(out, "fleet.yaml already names %s\n", repo)
		return nil
	}
	return addSource(filepath.Base(repo), newSource{Kind: kindBacklogmd, Dir: repo}, out)
}

// issueTrackerGithub is the GitHub flavour of the tracker doc: Specs and
// Tickets as issues and sub-issues on a Projects board. {{owner}},
// {{project}} and {{repo}} are filled in for the repo init prepares.
//
//go:embed issue-tracker-github.md
var issueTrackerGithub string

// InitGithubRepo prepares a GitHub-tracked repo to be the fleet's queue: the
// fleet, as Init lays it down; the GitHub tracker doc; and a github source
// naming board ("owner/number") and the repo its origin remote points at.
// No Backlog.md project: the issues are the queue. Safe to re-run.
func InitGithubRepo(s Settings, repo, board string, out io.Writer) error {
	b, err := github.ParseBoard(board)
	if err != nil {
		return fmt.Errorf("init: --github: %w", err)
	}
	repo, err = repoDir(repo)
	if err != nil {
		return err
	}
	url, err := command(repo, "git", "remote", "get-url", "origin")
	if err != nil {
		return fmt.Errorf("init: %s has no origin remote to name its GitHub repo: %s", repo, url)
	}
	name, err := github.RepoFromOrigin(url)
	if err != nil {
		return fmt.Errorf("init: %w", err)
	}
	if err := Init(s, out); err != nil {
		return err
	}
	doc := strings.NewReplacer("{{owner}}", b.Owner, "{{project}}", strconv.Itoa(b.Project), "{{repo}}", name).Replace(issueTrackerGithub)
	if err := initTrackerDoc(repo, doc, out); err != nil {
		return err
	}
	if _, found := s.find(func(c SourceConfig) bool { return c.kind() == kindGithub && b.Works(c.Github, name) }); found {
		fmt.Fprintf(out, "fleet.yaml already names %s on %s\n", name, b)
		return nil
	}
	cfg := b.Config(name)
	return addSource(filepath.Base(repo), newSource{Kind: kindGithub, Github: &cfg}, out)
}

// repoDir is the absolute path of the project repo init was given, which must
// be a directory.
func repoDir(repo string) (string, error) {
	repo, err := filepath.Abs(expandHome(repo))
	if err != nil {
		return "", err
	}
	if fi, err := os.Stat(repo); err != nil || !fi.IsDir() {
		return "", fmt.Errorf("init: %s is not a directory", repo)
	}
	return repo, nil
}

// find is the first source block of s that match accepts, one queue or several.
func (s Settings) find(match func(SourceConfig) bool) (SourceConfig, bool) {
	for _, c := range append([]SourceConfig{s.Source}, slices.Collect(maps.Values(s.Sources))...) {
		if match(c) {
			return c, true
		}
	}
	return SourceConfig{}, false
}

// initTrackerDoc writes the tracker doc the skills read. One the repo already
// has is its owner's: kept, and said so.
func initTrackerDoc(repo, body string, out io.Writer) error {
	const rel = "docs/agents/issue-tracker.md"
	path := filepath.Join(repo, rel)
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(out, "kept %s — it already exists\n", rel)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(out, "wrote %s\n", rel)
	return nil
}

// newSource is the source block init writes: only what init knows, so the
// owner's fleet.yaml gains no empty fields.
type newSource struct {
	Kind   string         `yaml:"kind"`
	Dir    string         `yaml:"dir,omitempty"`
	Github *github.Config `yaml:"github,omitempty"`
}

// addSource names src as one of the fleet's queues in fleet.yaml, editing the
// owner's YAML in place so its comments and order survive:
//   - no queue yet: a source: block;
//   - a Backlog.md source with no dir (before ADR 0023) and src a Backlog.md
//     repo: the dir is set on that source;
//   - one source: it becomes sources:, each entry named after its repo, and
//     the old one stays default_source so new tasks land where they did;
//   - sources: one more entry, named name or, when that is taken, name-2…
func addSource(name string, src newSource, out io.Writer) error {
	path := filepath.Join(hostpath.ConfigDir(), "fleet.yaml")
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if len(doc.Content) == 0 {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("%s: not a mapping of settings", path)
	}
	var block yaml.Node
	if err := block.Encode(src); err != nil {
		return err
	}
	var said string
	single, many := value(root, "source"), value(root, "sources")
	switch {
	case many != nil:
		if entry := legacy(values(many), src); entry != nil {
			setKey(entry, "dir", src.Dir)
			said = "named " + src.Dir + " on the source that had no dir"
			break
		}
		name = freeName(many, name)
		many.Content = append(many.Content, str(name), &block)
		said = "added " + name + " to sources"
	case single != nil && single.Kind == yaml.MappingNode && len(single.Content) > 0:
		if legacy([]*yaml.Node{single}, src) != nil {
			setKey(single, "dir", src.Dir)
			said = "named " + src.Dir + " on the source that had no dir"
			break
		}
		var old SourceConfig
		if err := single.Decode(&old); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		oldName := sourceName(old)
		sources := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{str(oldName), single}}
		name = freeName(sources, name)
		sources.Content = append(sources.Content, str(name), &block)
		key := root.Content[slices.Index(root.Content, single)-1]
		key.Value = "sources"
		root.Content[slices.Index(root.Content, single)] = sources
		if value(root, "default_source") == nil {
			root.Content = append(root.Content, str("default_source"), str(oldName))
		}
		said = "turned source: into sources: " + oldName + " and " + name
	case single != nil:
		root.Content[slices.Index(root.Content, single)] = &block
		said = "named " + name + " as the queue"
	default:
		root.Content = append(root.Content, str("source"), &block)
		said = "named " + name + " as the queue"
	}
	var buf strings.Builder
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(buf.String()), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(out, "%s in %s\n", said, path)
	return nil
}

// value is the node mapping m holds under key, nil when it has none.
func value(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// setKey sets key to v in mapping m, adding it when m has none.
func setKey(m *yaml.Node, key, v string) {
	if n := value(m, key); n != nil {
		*n = *str(v)
		return
	}
	m.Content = append(m.Content, str(key), str(v))
}

// str is a string scalar: the encoder quotes it when YAML would misread it.
func str(v string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v} }

// values is every value of mapping m, in order.
func values(m *yaml.Node) []*yaml.Node {
	var vs []*yaml.Node
	for i := 1; i < len(m.Content); i += 2 {
		vs = append(vs, m.Content[i])
	}
	return vs
}

// legacy is the first of blocks that is a Backlog.md source with no dir, when
// src is a Backlog.md repo to name on it; nil otherwise.
func legacy(blocks []*yaml.Node, src newSource) *yaml.Node {
	if src.Dir == "" {
		return nil
	}
	for _, b := range blocks {
		var c SourceConfig
		if b.Kind == yaml.MappingNode && b.Decode(&c) == nil && c.kind() == kindBacklogmd && c.Dir == "" {
			return b
		}
	}
	return nil
}

// sourceName is what an existing source is called once it has neighbours:
// the repo it works.
func sourceName(c SourceConfig) string {
	switch {
	case c.kind() == kindGithub && c.Github.Repo != "":
		return path.Base(c.Github.Repo)
	case c.kind() == kindBacklogmd && c.Dir != "":
		return filepath.Base(c.Dir)
	}
	return c.kind()
}

// freeName is name, or name-2, name-3… when sources already has it.
func freeName(sources *yaml.Node, name string) string {
	for i, n := 2, name; ; i++ {
		if value(sources, n) == nil {
			return n
		}
		n = name + "-" + strconv.Itoa(i)
	}
}

// initBacklog lays down the repo's Backlog.md project when it has none, and
// makes sure its config knows the fleet's lifecycle. backlog init also writes
// its agent guidelines into the repo's AGENTS.md: a file in the human's repo,
// so it is said.
func initBacklog(dir string, vocab backlogmd.Vocabulary, out io.Writer) error {
	if _, err := os.Stat(filepath.Join(dir, "backlog")); os.IsNotExist(err) {
		if msg, err := command(dir, "backlog", "init", filepath.Base(dir), "--defaults"); err != nil {
			return fmt.Errorf("backlog init: %s", msg)
		}
		fmt.Fprintln(out, "ran backlog init: wrote backlog/ and Backlog.md's guidelines into AGENTS.md")
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
