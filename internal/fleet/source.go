package fleet

import (
	"fmt"

	"github.com/DnzzL/herdr-fleet/internal/work"
	"github.com/DnzzL/herdr-fleet/internal/work/backlogmd"
)

// kindBacklogmd is the queue every fleet has unless it says otherwise: a
// Backlog.md project in the fleet dir.
const kindBacklogmd = "backlogmd"

// SourceConfig is the `source:` block of fleet.yaml: which backend the queue
// lives in. Absent, the fleet works the Backlog.md project in its own dir.
type SourceConfig struct {
	Kind string `yaml:"kind"`
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
func NewSource(s Settings) (work.Source, error) {
	switch s.Source.kind() {
	case kindBacklogmd:
		return backlogmd.New(s.Dir), nil
	}
	return nil, fmt.Errorf("unknown queue %q: the fleet speaks %s", s.Source.Kind, kindBacklogmd)
}
