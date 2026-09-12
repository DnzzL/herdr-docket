package fleet

import (
	"fmt"
	"io"

	"github.com/DnzzL/herdr-fleet/internal/work"
	"github.com/DnzzL/herdr-fleet/internal/work/backlogmd"
	"github.com/DnzzL/herdr-fleet/internal/work/basecamp"
	"github.com/DnzzL/herdr-fleet/internal/work/multi"
)

// The queues the fleet knows how to speak to.
const (
	kindBacklogmd = "backlogmd"
	kindBasecamp  = "basecamp"
)

// SourceConfig is the `source:` block of fleet.yaml: which backend the queue
// lives in. Absent, the fleet works the Backlog.md project in its own dir.
type SourceConfig struct {
	Kind string `yaml:"kind"`
	// Basecamp is the block that kind reads. Each adapter owns the shape of
	// its own configuration; this package only hands it over.
	Basecamp basecamp.Config `yaml:"basecamp"`
}

// kind is the source kind with the default filled in.
func (c SourceConfig) kind() string {
	if c.Kind == "" {
		return kindBacklogmd
	}
	return c.Kind
}

// local reports whether the queue lives in the fleet dir itself — which is
// the only kind of queue the fleet has anything to scaffold.
func (c SourceConfig) local() bool { return c.kind() == kindBacklogmd }

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
		return backlogmd.New(dir), nil
	case kindBasecamp:
		return basecamp.New(c.Basecamp)
	}
	return nil, unknownQueue(c.Kind)
}

// unknownQueue is the one wording for a queue the fleet has never heard of.
// Which queues exist is stated once, in the switch above, and this is how both
// the config and the CLI report one that does not.
func unknownQueue(kind string) error {
	return fmt.Errorf("unknown queue %q: the fleet speaks %s or %s", kind, kindBacklogmd, kindBasecamp)
}

// Auth signs the fleet in to a queue that needs it, and says so plainly for
// one that does not. A local queue has no account to authenticate against,
// which is a fact about the queue rather than a failure.
func Auth(kind string, out io.Writer) error {
	switch (SourceConfig{Kind: kind}).kind() {
	case kindBacklogmd:
		return fmt.Errorf("%s is a local queue: there is nothing to sign in to", kindBacklogmd)
	case kindBasecamp:
		return basecamp.Login(out)
	}
	return unknownQueue(kind)
}
