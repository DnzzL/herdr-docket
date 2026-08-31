// Package prompt assembles what the agent is told: the agent's persona, the
// task in full, and the closing protocol — how to report back into the
// backlog. Pure text in, text out, so the one contract the whole system
// depends on is pinned by a test.
package prompt

import (
	"fmt"
	"strings"

	"github.com/DnzzL/herdr-fleet/internal/backlog"
	"github.com/DnzzL/herdr-fleet/internal/fleet"
)

// Assemble builds the run prompt for one task. fleetDir is where the platform
// backlog lives — the agent works in its own workdir, so every backlog call
// it makes must carry BACKLOG_CWD.
func Assemble(a fleet.Agent, v backlog.View, fleetDir string) string {
	var b strings.Builder
	b.WriteString(a.Persona)
	b.WriteString("\n\n")

	fmt.Fprintf(&b, "# Your task: %s — %s\n\n", v.ID, v.Title)
	if v.Description != "" {
		fmt.Fprintf(&b, "## Description\n\n%s\n\n", v.Description)
	}
	if len(v.AcceptanceCriteria) > 0 {
		b.WriteString("## Acceptance criteria\n\n")
		for _, c := range v.AcceptanceCriteria {
			box := "[ ]"
			if c.Checked {
				box = "[x]"
			}
			fmt.Fprintf(&b, "- %s #%d %s\n", box, c.Index, c.Text)
		}
		b.WriteString("\n")
	}
	if v.ImplementationNotes != "" {
		fmt.Fprintf(&b, "## Notes from previous runs\n\n%s\n\n", v.ImplementationNotes)
	}

	edit := fmt.Sprintf("BACKLOG_CWD=%s backlog task edit %s", fleetDir, v.ID)
	fmt.Fprintf(&b, `## When you are done — required

This task lives in the fleet backlog at %s (not in this repo). Report back
with the backlog CLI, always prefixed with BACKLOG_CWD:

- Check off each acceptance criterion you met: %s --check-ac <index>
- Append what you did and why to the notes: %s --append-notes "<summary>"
- Then set the final status, exactly one of:
  - %s -s Done      — every criterion met
  - %s -s Failed    — you could not do it; say why in the notes
  - %s -s Blocked   — a human must decide or unblock something first

Never leave the task "In Progress": if you stop for any reason, set Failed or
Blocked with a note. Do not touch other fleet tasks' statuses.

This run has a time budget. If the task is too big to finish well within it,
do one coherent slice, record exactly where you stopped in the notes, then
create the follow-up task for the rest and set this one Done — a finished
slice with a good handoff beats a timed-out marathon.

If you find follow-up work, create a task for it instead of expanding this one:
BACKLOG_CWD=%s backlog task create "<title>" -d "<what and why>" -a %s
`, fleetDir, edit, edit, edit, edit, edit, fleetDir, a.Name)
	return b.String()
}
