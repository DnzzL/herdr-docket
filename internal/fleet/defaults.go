package fleet

import "github.com/DnzzL/herdr-docket/internal/work"

// Defaults resolves which agent an unassigned task falls to. One value for
// the whole fleet is not enough once it works more than one project: an agent
// carries its own workdir, so a single global default would send one
// project's tasks into another project's checkout. The queue a task came from
// is the routing information, and a prefixed id already carries it.
//
// Empty stays empty at every level. An unassigned task may be a human still
// drafting, and a fleet that configured no default gets none invented for it.
type Defaults struct {
	global   string
	bySource map[string]string
}

// Defaults reads the settings' defaults once, so the resolution rule lives in
// one place rather than at each of the five call sites that route a task.
func (s Settings) Defaults() Defaults {
	d := Defaults{global: s.DefaultAgent}
	for name, c := range s.Sources {
		if c.DefaultAgent == "" {
			continue
		}
		if d.bySource == nil {
			d.bySource = make(map[string]string, len(s.Sources))
		}
		d.bySource[name] = c.DefaultAgent
	}
	return d
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
