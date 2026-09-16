package text

import (
	"fmt"
	"strings"

	"github.com/DnzzL/herdr-docket/internal/work"
)

// TaskDetail renders one task the one way the fleet reads a task: the state
// line, then body, criteria and notes, plain — no markdown, no comment thread,
// no links rewritten. `herdr-docket task view` prints it and the pane's detail
// view shows it, because a second renderer is how two views of the same task
// drift apart.
//
// It renders the fleet's Task, not the backend's page: the fields here are the
// ones the prompt is built from, so what a person reads and what an agent is
// told cannot disagree.
func TaskDetail(it work.Task) string {
	var b strings.Builder
	state, who := "open", it.Assignee
	if !it.Open {
		state = "closed"
	}
	if who == "" {
		who = "-"
	}
	fmt.Fprintf(&b, "%s — %s\n%s · %s · %s\n", it.ID, it.Title, state, it.Phase, who)
	if it.Body != "" {
		fmt.Fprintf(&b, "\n## Description\n\n%s\n", it.Body)
	}
	if len(it.Criteria) > 0 {
		fmt.Fprintf(&b, "\n## Acceptance criteria\n\n")
		for _, c := range it.Criteria {
			box := "[ ]"
			if c.Checked {
				box = "[x]"
			}
			fmt.Fprintf(&b, "- %s #%d %s\n", box, c.Index, c.Text)
		}
	}
	if it.Notes != "" {
		fmt.Fprintf(&b, "\n## Notes\n\n%s\n", it.Notes)
	}
	return b.String()
}
