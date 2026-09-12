// Package prompt assembles what the agent is told: the agent's persona, the
// task in full, and the closing protocol — how to report back into the
// queue. Pure text in, text out, so the one contract the whole system
// depends on is pinned by a test.
package prompt

import (
	"fmt"
	"strings"

	"github.com/DnzzL/herdr-fleet/internal/fleet"
	"github.com/DnzzL/herdr-fleet/internal/work"
)

// Assemble builds the run prompt for one task. fleetDir is where the fleet
// lives — roster, wiring and backend config. The agent works in its own
// workdir, so the prompt names the fleet dir for context only: every call it
// makes goes through the fleet CLI, which finds the queue on its own.
func Assemble(a fleet.Agent, v work.Task, fleetDir string) string {
	var b strings.Builder
	b.WriteString(a.Persona)
	b.WriteString("\n\n")

	fmt.Fprintf(&b, "# Your task: %s — %s\n\n", v.ID, v.Title)
	if v.Body != "" {
		fmt.Fprintf(&b, "## Description\n\n%s\n\n", v.Body)
	}
	if len(v.Criteria) > 0 {
		b.WriteString("## Acceptance criteria\n\n")
		for _, c := range v.Criteria {
			box := "[ ]"
			if c.Checked {
				box = "[x]"
			}
			fmt.Fprintf(&b, "- %s #%d %s\n", box, c.Index, c.Text)
		}
		b.WriteString("\nThese boxes are the queue's record, not yours to edit. Your verdict on\neach one — met or not, and the evidence — belongs in the note that closes the\ntask.\n\n")
	}
	if v.Notes != "" {
		fmt.Fprintf(&b, "## Notes from previous runs\n\n%s\n\n", v.Notes)
	}

	// A prefixed id (myapp/TASK-12) means the fleet works several queues, and
	// a follow-up has to land in the queue this task came from. The prefix is
	// the source, so the prompt writes it in and the agent never has to know a
	// second queue exists.
	createSource := ""
	if i := strings.IndexByte(v.ID, '/'); i > 0 {
		createSource = " -s " + v.ID[:i]
	}

	fmt.Fprintf(&b, `## When you are done — required

The task lives in the fleet queue (fleet dir: %s), not in this repo. The
queue answers only to the herdr-fleet CLI — never edit a task by hand.

Say what happened as you go:

  herdr-fleet task note %s "<what you did and why>"

Then close the task exactly once, reporting exactly one verdict:

  herdr-fleet task done %s --note "<what you did, which criteria you met, and how you know>"
  herdr-fleet task fail %s --note "<why you could not do it>"
  herdr-fleet task block %s --note "<what a human must decide or unblock>"

A task you leave open is a task the fleet will pick up and run again, so don't
leave one open. Do not touch other agents' tasks.

This run has a time budget. If the task is too big to finish well within it,
do one coherent slice, record exactly where you stopped in the closing note,
then create the follow-up task for the rest and close this one done — a
finished slice with a good handoff beats a timed-out marathon.

If you find follow-up work, create a task for it instead of expanding this one:

  herdr-fleet task create "<title>" -d "<what and why>" -a %s%s
`, fleetDir, v.ID, v.ID, v.ID, v.ID, a.Name, createSource)
	return b.String()
}
