package fleet

import "testing"

// One default agent is one project's intake. A fleet working two projects has
// two, because an agent carries its own workdir: a global default would open
// one project's checkout to do the other project's work.
func TestAnUnassignedTaskFallsToItsOwnProjectsAgent(t *testing.T) {
	d := Settings{
		DefaultAgent: "dev",
		Sources: map[string]SourceConfig{
			"notara":  {DefaultAgent: "notara-pm"},
			"dishnow": {DefaultAgent: "dishnow-pm"},
			"legacy":  {}, // names none: the fleet's own default still applies
		},
	}.Defaults()
	for _, tc := range []struct{ id, want string }{
		{"notara/NOT-92", "notara-pm"},
		{"dishnow/TASK-19", "dishnow-pm"},
		{"legacy/TASK-1", "dev"},
		{"TASK-1", "dev"}, // one queue, no prefix
		{"unknown/TASK-1", "dev"},
	} {
		if got := d.For(tc.id); got != tc.want {
			t.Errorf("%s: default = %q, want %q", tc.id, got, tc.want)
		}
	}
}

// No default anywhere means no default: an unassigned task may be a human
// still drafting, and the fleet leaves it alone. Adding per-source defaults
// must not invent one for a fleet that never asked.
func TestNoDefaultAnywhereLeavesUnassignedWorkAlone(t *testing.T) {
	d := Settings{Sources: map[string]SourceConfig{"notara": {}}}.Defaults()
	if got := d.For("notara/NOT-92"); got != "" {
		t.Errorf("default = %q, want none", got)
	}
	if got := (Settings{}).Defaults().For("TASK-1"); got != "" {
		t.Errorf("default = %q, want none", got)
	}
}

// One source: is one unnamed queue, so its ids carry no prefix and its
// settings live under the empty key SourceOf reads from a bare id. Reading
// only sources: left a singular-source fleet's triager and default
// unreachable, and its triaged cards were called untriaged (TASK-86).
func TestASingularSourceRoutesWithItsOwnSettings(t *testing.T) {
	d := Settings{
		DefaultAgent: "dev",
		Source:       SourceConfig{DefaultAgent: "own-pm", Triager: "own-triager"},
	}.Defaults()
	if got := d.TriagerFor("TASK-1"); got != "own-triager" {
		t.Errorf("triager = %q, want own-triager", got)
	}
	if got := d.For("TASK-1"); got != "own-pm" {
		t.Errorf("default = %q, want own-pm", got)
	}
}
