package fleet

import "github.com/DnzzL/herdr-docket/internal/work"

// Defaults resolves which agent a task falls to. One value for the whole
// fleet is not enough once it works more than one project: an agent carries
// its own workdir, so a single global default would send one project's tasks
// into another project's checkout. The queue a task came from is the routing
// information, and a prefixed id already carries it.
//
// A triaged card is the one route the task does not decide: its queue's
// triager owns the grill whatever assignee the card names (TASK-86).
//
// Empty stays empty at every level. An unassigned task may be a human still
// drafting, and a fleet that configured no default gets none invented for it.
type Defaults struct {
	global     string
	bySource   map[string]string
	triagerFor map[string]string
}

// Defaults reads the settings' defaults once, so the resolution rule lives in
// one place rather than at each of the five call sites that route a task.
func (s Settings) Defaults() Defaults {
	d := Defaults{global: s.DefaultAgent}
	// One source: is one unnamed queue: its ids carry no prefix, so its
	// settings live under the empty key — the same key work.SourceOf reads
	// from a bare id. Reading only s.Sources here left a singular source's
	// triager and default unreachable (TASK-86).
	if len(s.Sources) == 0 {
		if s.Source.DefaultAgent != "" {
			d.bySource = map[string]string{"": s.Source.DefaultAgent}
		}
		if s.Source.Triager != "" {
			d.triagerFor = map[string]string{"": s.Source.Triager}
		}
		return d
	}
	for name, c := range s.Sources {
		if c.DefaultAgent != "" {
			if d.bySource == nil {
				d.bySource = make(map[string]string, len(s.Sources))
			}
			d.bySource[name] = c.DefaultAgent
		}
		if c.Triager != "" {
			if d.triagerFor == nil {
				d.triagerFor = make(map[string]string, len(s.Sources))
			}
			d.triagerFor[name] = c.Triager
		}
	}
	return d
}

// TriagerFor is the agent that grills a triaged card of the queue it came
// from. Empty means this queue names no triager: a triaged card is then
// nobody's to pick up, and the daemon says so rather than handing it to the
// assignee it happens to carry.
func (d Defaults) TriagerFor(taskID string) string {
	return d.triagerFor[work.SourceOf(taskID)]
}

// For is the agent an unassigned task in this queue falls to: its source's
// own default, or the fleet's. A bare id — one queue, no prefix — and a
// source that names no default of its own both fall back to the fleet's.
func (d Defaults) For(taskID string) string {
	if a, ok := d.bySource[work.SourceOf(taskID)]; ok {
		return a
	}
	return d.global
}
