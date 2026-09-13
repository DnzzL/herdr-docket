// Package multi is a work.Source over several named queues. It is how one
// fleet works more than one project — a Backlog.md repo here, a Basecamp
// project there — without anything above the port learning a new shape.
//
// A task from a sub-source is addressed by a prefixed id: the source's name,
// a slash, then the id the backend gave it (myapp/TASK-12, bc/987654). The
// prefix is the routing information, so it travels with the task everywhere
// the fleet carries an id — which is why Get, Comment, Close and SetPhase all
// take back the same prefixed id List handed out. An id with no prefix, or
// one naming no configured source, is an error and never a silent no-op.
package multi

import (
	"fmt"
	"sort"

	"github.com/DnzzL/herdr-fleet/internal/work"
)

// Source multiplexes named queues behind one work.Source.
type Source struct {
	names []string
	subs  map[string]work.Source
}

// New builds a composite over subs. Names are sorted once so List and Names
// have a stable order regardless of how the map was built.
func New(subs map[string]work.Source) *Source {
	names := make([]string, 0, len(subs))
	for name := range subs {
		names = append(names, name)
	}
	sort.Strings(names)
	return &Source{names: names, subs: subs}
}

// Names lists the queues, in a stable order.
func (s *Source) Names() []string { return append([]string(nil), s.names...) }

// split separates a prefixed id into its source name and the backend's own
// id. An id with no prefix, or one naming no configured source, is an error:
// the fleet never guesses which queue an id meant.
func (s *Source) split(id string) (name, rest string, err error) {
	name = work.SourceOf(id)
	if name == "" {
		return "", "", fmt.Errorf("%s: not a queue-prefixed id (want <source>/<id>)", id)
	}
	rest = id[len(name)+1:]
	if _, ok := s.subs[name]; !ok {
		return "", "", fmt.Errorf("%s: no configured source %q", id, name)
	}
	return name, rest, nil
}

// List concatenates every sub-source's tasks, each id prefixed with its
// source. One source failing fails the whole list: a poll that hid one queue's
// work behind another's silence would be worse than one that stops.
func (s *Source) List() ([]work.Task, error) {
	var out []work.Task
	for _, name := range s.names {
		items, err := s.subs[name].List()
		if err != nil {
			return nil, fmt.Errorf("source %q: %w", name, err)
		}
		for _, it := range items {
			it.ID = name + "/" + it.ID
			out = append(out, it)
		}
	}
	return out, nil
}

// Get dispatches on the prefix and presents the task under the prefixed id
// the caller knows, not the backend's bare one.
func (s *Source) Get(id string) (work.Task, error) {
	name, rest, err := s.split(id)
	if err != nil {
		return work.Task{}, err
	}
	it, err := s.subs[name].Get(rest)
	if err != nil {
		return work.Task{}, err
	}
	it.ID = id
	return it, nil
}

func (s *Source) Comment(id, text string) error {
	name, rest, err := s.split(id)
	if err != nil {
		return err
	}
	return s.subs[name].Comment(rest, text)
}

func (s *Source) Close(id string, verdict work.Verdict) error {
	name, rest, err := s.split(id)
	if err != nil {
		return err
	}
	return s.subs[name].Close(rest, verdict)
}

// SetPhase forwards to the named sub-source when it can show a phase, and is
// a no-op when it cannot: a source without phases is quieter, not lesser. An
// unknown prefix is still an error — that is a real routing mistake, not a
// capability the backend happens to lack.
func (s *Source) SetPhase(id string, phase work.Phase) error {
	name, rest, err := s.split(id)
	if err != nil {
		return err
	}
	if p, ok := s.subs[name].(work.Phaser); ok {
		return p.SetPhase(rest, phase)
	}
	return nil
}

// Create targets the sole sub-source. With several queues there is no way to
// guess which one a bare Create meant, so it is an error — the CLI's -s names
// one through CreateIn. Working for one sub-source is what lets the composite
// pass the adapter conformance suite, which speaks only Source.
func (s *Source) Create(title, body, assignee string) (string, error) {
	if len(s.names) != 1 {
		return "", fmt.Errorf("this fleet has %d queues: name one with -s/--source", len(s.names))
	}
	return s.CreateIn(s.names[0], title, body, assignee)
}

// CreateIn creates work in the named queue and returns its prefixed id.
func (s *Source) CreateIn(name, title, body, assignee string) (string, error) {
	sub, ok := s.subs[name]
	if !ok {
		return "", fmt.Errorf("no configured source %q", name)
	}
	id, err := sub.Create(title, body, assignee)
	if err != nil {
		return "", err
	}
	return name + "/" + id, nil
}
