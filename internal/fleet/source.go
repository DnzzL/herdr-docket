package fleet

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"

	"github.com/DnzzL/herdr-docket/internal/work"
	"github.com/DnzzL/herdr-docket/internal/work/backlogmd"
	"github.com/DnzzL/herdr-docket/internal/work/basecamp"
	"github.com/DnzzL/herdr-docket/internal/work/github"
	"github.com/DnzzL/herdr-docket/internal/work/multi"
)

// The queues the fleet knows how to speak to.
const (
	kindBacklogmd = "backlogmd"
	kindBasecamp  = "basecamp"
	kindGithub    = "github"
)

// SourceConfig is the `source:` block of fleet.yaml: which backend the queue
// lives in. Absent, the fleet works the Backlog.md project in its own dir.
type SourceConfig struct {
	Kind string `yaml:"kind"`
	// Dir points a local queue at a project other than the fleet's own —
	// which is what a fleet working several projects' own backlogs is. Empty
	// is the fleet dir; a relative path is read from it; ~ expands.
	Dir string `yaml:"dir"`
	// DefaultAgent picks up this queue's unassigned tasks, overriding the
	// fleet's own. An agent carries its workdir, so the project a task came
	// from is what decides which agent may take it unasked.
	DefaultAgent string `yaml:"default_agent"`
	// Statuses is the project's own status words. Empty means the project
	// speaks the fleet's, which is true of one the fleet laid down itself.
	Statuses backlogmd.Vocabulary `yaml:"statuses"`
	// Basecamp is the block that kind reads. Each adapter owns the shape of
	// its own configuration; this package only hands it over.
	Basecamp basecamp.Config `yaml:"basecamp"`
	// Github is the block kind github reads: which board, and which repository
	// its issues live in.
	Github github.Config `yaml:"github"`
}

// kind is the source kind with the default filled in.
func (c SourceConfig) kind() string {
	if c.Kind == "" {
		return kindBacklogmd
	}
	return c.Kind
}

// local reports whether the queue lives in the fleet dir itself — which is
// the only kind of queue the fleet has anything to scaffold. A source that
// names a project of its own is somebody else's repo: the fleet works it, it
// does not lay it down.
func (c SourceConfig) local() bool { return c.kind() == kindBacklogmd && c.Dir == "" }

// ownQueues is the queues living in the fleet dir, which are the ones init
// has anything to create. Several sources or one, a queue that points
// somewhere else is not among them.
func (s Settings) ownQueues() []SourceConfig {
	if len(s.Sources) > 0 {
		var own []SourceConfig
		for _, c := range s.Sources {
			if c.local() {
				own = append(own, c)
			}
		}
		return own
	}
	if s.Source.local() {
		return []SourceConfig{s.Source}
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
		if err := c.Statuses.Validate(); err != nil {
			return nil, err
		}
		return backlogmd.New(c.dirFrom(dir), c.Statuses), nil
	case kindBasecamp:
		return basecamp.New(c.Basecamp)
	case kindGithub:
		return github.New(c.Github)
	}
	return nil, unknownQueue(c.Kind)
}

// dirFrom resolves the project this source works against. A source that names
// no dir is the fleet's own queue, exactly as before; a relative one hangs off
// the fleet dir, so a fleet directory stays movable in one edit.
func (c SourceConfig) dirFrom(fleetDir string) string {
	if c.Dir == "" {
		return fleetDir
	}
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
	return fmt.Errorf("unknown queue %q: the fleet speaks %s, %s or %s", kind, kindBacklogmd, kindBasecamp, kindGithub)
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
	case kindBasecamp:
		return basecamp.Login(out)
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
