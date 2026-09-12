// Package worktest is the contract every work.Source must satisfy.
//
// The fleet's correctness rests on a handful of promises the queue makes:
// created work comes back, the routing key survives the round trip, and
// closing with any verdict actually closes. An adapter runs this suite so a
// new backend is held to the same behaviour the fleet already depends on,
// rather than to whatever its author happened to try by hand.
//
// What the suite deliberately does not check: how an task looks in the
// backend, which status word records a verdict, or whether a phase survives.
// Those are the backend's business. The port promises behaviour, not shape.
package worktest

import (
	"strings"
	"testing"

	"github.com/DnzzL/herdr-fleet/internal/work"
)

// Run exercises a Source against the contract.
//
// newSource must return a Source over a fresh, empty backend on every call:
// the suite runs as subtests and each one needs somewhere clean to work. A
// backend that cannot be emptied between calls — a shared Basecamp project,
// say — should hand back a Source scoped to a workspace of its own.
func Run(t *testing.T, newSource func(t *testing.T) work.Source) {
	t.Helper()

	t.Run("an empty queue lists nothing", func(t *testing.T) {
		items, err := newSource(t).List()
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(items) != 0 {
			t.Fatalf("a fresh backend listed %d tasks, want none", len(items))
		}
	})

	t.Run("created work comes back from get", func(t *testing.T) {
		src := newSource(t)
		id, err := src.Create("Wire the gauge", "the numbers are not moving", "dev")
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if id == "" {
			t.Fatal("Create must hand back the new task's id")
		}
		it, err := src.Get(id)
		if err != nil {
			t.Fatalf("Get(%q): %v", id, err)
		}
		if it.ID != id {
			t.Errorf("Get returned %q, want the id it was created with, %q", it.ID, id)
		}
		if title := strings.TrimSpace(it.Title); title != "Wire the gauge" {
			t.Errorf("title = %q, want %q", title, "Wire the gauge")
		}
		if !strings.Contains(it.Body, "the numbers are not moving") {
			t.Errorf("body lost the description: %q", it.Body)
		}
		if !it.Open {
			t.Error("a freshly created task must be open")
		}
	})

	// The routing key is how a task finds its agent. If it does not survive a
	// round trip, work silently piles up unassigned.
	t.Run("the routing key survives the round trip", func(t *testing.T) {
		src := newSource(t)
		id, err := src.Create("Route me", "", "reviewer")
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		it, err := src.Get(id)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if it.Assignee != "reviewer" {
			t.Errorf("assignee = %q, want the routing key %q", it.Assignee, "reviewer")
		}
	})

	t.Run("open work shows up in list", func(t *testing.T) {
		src := newSource(t)
		id, err := src.Create("Visible", "", "dev")
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		items, err := src.List()
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		for _, it := range items {
			if it.ID == id {
				if !it.Open {
					t.Errorf("%s is listed but not open", id)
				}
				return
			}
		}
		t.Fatalf("created %s but List does not carry it", id)
	})

	t.Run("comments accumulate in order", func(t *testing.T) {
		src := newSource(t)
		id := mustCreate(t, src, "Talk to me", "dev")
		for _, text := range []string{"started", "halfway", "stuck on the API"} {
			if err := src.Comment(id, text); err != nil {
				t.Fatalf("Comment(%q): %v", text, err)
			}
		}
		it, err := src.Get(id)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		at := -1
		for _, text := range []string{"started", "halfway", "stuck on the API"} {
			i := strings.Index(it.Notes, text)
			if i < 0 {
				t.Fatalf("comment %q is missing from the notes: %q", text, it.Notes)
			}
			if i < at {
				t.Fatalf("comment %q is out of order in the notes: %q", text, it.Notes)
			}
			at = i
		}
	})

	// Every verdict closes. A blocked or failed task left open would be picked
	// up again on the next tick and run forever.
	t.Run("every verdict closes the task", func(t *testing.T) {
		for _, v := range []work.Verdict{work.Done, work.Failed, work.Blocked} {
			t.Run(string(v), func(t *testing.T) {
				src := newSource(t)
				id := mustCreate(t, src, "Finish me", "dev")
				if err := src.Close(id, v); err != nil {
					t.Fatalf("Close(%s): %v", v, err)
				}
				it, err := src.Get(id)
				if err != nil {
					t.Fatalf("Get after close: %v", err)
				}
				if it.Open {
					t.Errorf("after Close(%s) the task is still open", v)
				}
			})
		}
	})

	t.Run("an unknown verdict is refused and changes nothing", func(t *testing.T) {
		src := newSource(t)
		id := mustCreate(t, src, "Not so fast", "dev")
		if err := src.Close(id, work.Verdict("probably")); err == nil {
			t.Fatal("an unknown verdict must be refused, never treated as a close")
		}
		it, err := src.Get(id)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if !it.Open {
			t.Error("a refused verdict must leave the task open")
		}
	})

	t.Run("asking for work that does not exist fails", func(t *testing.T) {
		if _, err := newSource(t).Get("no-such-item-4f2a"); err == nil {
			t.Fatal("Get on an unknown id must fail, not return a zero task")
		}
	})
}

func mustCreate(t *testing.T, src work.Source, title, assignee string) string {
	t.Helper()
	id, err := src.Create(title, "", assignee)
	if err != nil {
		t.Fatalf("Create(%q): %v", title, err)
	}
	if id == "" {
		t.Fatalf("Create(%q) returned no id", title)
	}
	return id
}
