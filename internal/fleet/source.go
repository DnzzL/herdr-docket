package fleet

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"

	"github.com/DnzzL/herdr-docket/internal/work"
	"github.com/DnzzL/herdr-docket/internal/work/backlogmd"
	"github.com/DnzzL/herdr-docket/internal/work/github"
	"github.com/DnzzL/herdr-docket/internal/work/multi"
)

// The queues the fleet knows how to speak to.
const (
	kindBacklogmd = "backlogmd"
	kindGithub    = "github"
)

// SourceConfig is the `source:` block of fleet.yaml: which backend the queue
// lives in: a project repo's own Backlog.md (Dir) or a GitHub board.
type SourceConfig struct {
	Kind string `yaml:"kind"`
	// Dir is the project repo whose Backlog.md is the queue. Required for
	// backlogmd; a relative path is read from the fleet dir; ~ expands.
	Dir string `yaml:"dir"`
	// DefaultAgent picks up this queue's unassigned tasks, overriding the
	// fleet's own. An agent carries its workdir, so the project a task came
	// from is what decides which agent may take it unasked.
	DefaultAgent string `yaml:"default_agent"`
	// Verifier names the agent that judges an author's pull request before it
	// can merge (ADR 0013). Empty means no pipeline: a run ends on the
	// agent's own verdict, as it always has.
	Verifier string `yaml:"verifier"`
	// Triager names the agent that grills a card in the triage status — it
	// gets the card whatever its assignee. Empty means triage cards wait.
	Triager string `yaml:"triager"`
	// Merge is "auto" when the fleet may merge a verified PR on its own;
	// anything else stops at a PASS and leaves the merge to a human.
	Merge string `yaml:"merge"`
	// MaxRuns caps this queue's runs in flight, under the fleet's own cap: a
	// repo whose test suite cannot run twice at once says 1. 0 = no cap.
	MaxRuns int `yaml:"max_runs"`
	// Statuses is the project's own status words. Empty means the project
	// speaks the fleet's, which is true of one the fleet laid down itself.
	Statuses backlogmd.Vocabulary `yaml:"statuses"`
	// WorktreeBase names the git ref a run of this queue branches from
	// (TASK-45). Empty derives the repo's own default branch — origin/HEAD —
	// so that what a human left checked out in the main checkout is never
	// the base an agent builds on merely by being there.
	WorktreeBase string `yaml:"worktree_base"`
	// Github is the block kind github reads: which board, and which repository
	// its issues live in. Each adapter owns the shape of its own
	// configuration; this package only hands it over.
	Github github.Config `yaml:"github"`
}

// kind is the source kind with the default filled in.
func (c SourceConfig) kind() string {
	if c.Kind == "" {
		return kindBacklogmd
	}
	return c.Kind
}

// errNoRepo is the one wording for a Backlog.md queue that names no project
// repo. The queue is the repo's own tracker (ADR 0023); the fleet dir holds
// agents and roles, never a queue.
var errNoRepo = errors.New("source.dir must name a project repo — run `herdr-docket init <repo>`")

// check refuses a source block that cannot name a queue. Only what is knowable
// from the block itself: whether the repo exists is the adapter's to say.
func (c SourceConfig) check() error {
	if c.kind() == kindBacklogmd && c.Dir == "" {
		return errNoRepo
	}
	return nil
}

// NewSource builds the queue the fleet works from. This is the one place an
// adapter is constructed: the daemon, the CLI and the board all come through
// here, so they cannot disagree about which queue they are looking at.
//
// One named source: is one unnamed queue, exactly as before and with no id
// prefix anywhere. Several sources: are a composite, whose tasks carry their
// source's name as an id prefix.
func NewSource(s Settings) (work.Source, error) {
	if len(s.Sources) > 0 {
		subs := make(map[string]work.Source, len(s.Sources))
		for name, cfg := range s.Sources {
			sub, err := newQueue(cfg, s.Dir)
			if err != nil {
				return nil, fmt.Errorf("source %q: %w", name, err)
			}
			subs[name] = sub
		}
		return multi.New(subs), nil
	}
	return newQueue(s.Source, s.Dir)
}

// newQueue builds one adapter from one source block. Every adapter is
// constructed here and nowhere else.
func newQueue(c SourceConfig, dir string) (work.Source, error) {
	switch c.kind() {
	case kindBacklogmd:
		if err := c.check(); err != nil {
			return nil, err
		}
		if err := c.Statuses.Validate(); err != nil {
			return nil, err
		}
		return backlogmd.NewWithBase(c.dirFrom(dir), c.WorktreeBase, c.Statuses), nil
	case kindGithub:
		return github.New(c.Github)
	}
	return nil, unknownQueue(c.Kind)
}

// dirFrom resolves the project this source works against. A relative dir
// hangs off the fleet dir, so a fleet directory stays movable in one edit.
func (c SourceConfig) dirFrom(fleetDir string) string {
	dir := expandHome(c.Dir)
	if filepath.IsAbs(dir) {
		return dir
	}
	return filepath.Join(fleetDir, dir)
}

// unknownQueue is the one wording for a queue the fleet has never heard of.
// Which queues exist is stated once, in the switch above, and this is how both
// the config and the CLI report one that does not.
func unknownQueue(kind string) error {
	return fmt.Errorf("unknown queue %q: the fleet speaks %s or %s", kind, kindBacklogmd, kindGithub)
}

// Auth signs the fleet in to a queue that needs it, and says so plainly for
// one that does not. A local queue has no account to authenticate against,
// which is a fact about the queue rather than a failure.
//
// It takes the whole Settings rather than a kind, because signing in to a
// hosted queue means knowing *which* board or project it is, and that is
// configuration. The name is a source's own name when the fleet has several,
// which is also how a fleet with two boards signs in to one of them.
func Auth(s Settings, name, token string, out io.Writer) error {
	c := s.authSubject(name)
	switch c.kind() {
	case kindBacklogmd:
		return fmt.Errorf("%s is a local queue: there is nothing to sign in to", kindBacklogmd)
	case kindGithub:
		return github.Login(c.Github, agentNames(s), token, out)
	}
	return unknownQueue(name)
}

// authSubject is the source block a sign-in is about. A fleet with several
// sources names one by its own name; a fleet with one gets it when it is of
// the kind being signed in to. Anything else answers with a block of that kind
// and nothing in it, so that a queue which is not configured says *what* is
// missing rather than that it is unknown.
func (s Settings) authSubject(name string) SourceConfig {
	if named, ok := s.Sources[name]; ok {
		return named
	}
	if s.Source.kind() == name {
		return s.Source
	}
	return SourceConfig{Kind: name}
}

// agentNames is who a queue can route to: the agents this fleet describes,
// sorted so that signing in twice reads the same.
func agentNames(s Settings) []string {
	agents, _ := LoadAgents(s.Dir)
	names := make([]string, 0, len(agents))
	for name := range agents {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
