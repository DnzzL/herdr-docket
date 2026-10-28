---
id: TASK-1
title: Add per-agent disabled pause to herdr-docket
status: Done
assignee: []
created_date: '2026-09-09 13:28'
updated_date: '2026-09-09 14:00'
labels: []
dependencies: []
priority: high
ordinal: 1000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Give fleet a per-agent pause, mirroring herdr-automations' 'disabled: true': the entry stays on disk but out of the scheduler. The AGENT.md frontmatter is already re-read every tick (evaluate() -> fleet.LoadAgents, daemon.go:82), so a pause takes effect within one poll (~15s) with no daemon restart.

Design decisions already taken (do not re-open):
- Kill nothing: Disabled removes an agent from scheduling, never from a run already in flight. TASK running when you pause keeps its full timeout budget.
- herdr-docket run <id> is explicit human intent: it BYPASSES Disabled, so pause parks the scheduler rather than forbidding work.
- Pause is CLI-only in this slice. The board groups rows by STATUS (pane.go:282 r.header), not by agent, so there is no agent row to put a 'p' toggle on. An agent section + toggle is a follow-up once a second real agent exists.

Hard implementation constraint: automations writes YAML with config.Save() on a pure-YAML file. AGENT.md is YAML frontmatter + a markdown persona + hand-written comments ('# worktree (default): fresh branch per run'). A yaml.Marshal round-trip destroys the comments and risks the persona. The field must be written by surgical line editing between the existing '---' fences, reusing splitFrontmatter (internal/fleet/agent.go:98) as the reader.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Agent struct gains Disabled bool with yaml tag 'disabled' (internal/fleet/agent.go); absent key loads false, 'disabled: true' loads true
- [ ] #2 daemon skips disabled agents when building the 'available' map (daemon.go ~96) so BOTH an explicit assignee and the fleet.yaml default_agent cannot route work to it
- [ ] #3 SetDisabled edits only the frontmatter line: re-running it preserves the persona body, the attached comments, and every other key; no yaml.Marshal round-trip anywhere
- [ ] #4 CLI gains 'herdr-docket agent list|pause|resume <name>' with usage text that states the one-tick (~15s) delay before a pause is picked up
- [ ] #5 a run already in flight is untouched by pause/resume; it finishes its own timeout and still reconciles its task
- [ ] #6 'herdr-docket run <task-id>' starts a task whose agent is disabled, and a test locks that behavior in
- [ ] #7 tests cover: frontmatter parse of Disabled, non-destructive SetDisabled round-trip, a tick that routes nothing to a disabled agent, and an unassigned task NOT stolen by default_agent=disabled-agent
- [ ] #8 README documents the field in the Writing-an-agent example and states the running-run semantics; no pane change in this slice
<!-- AC:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Filter landed in pick.Next, not the daemon's available map. AC 2 named daemon.go, but removing a parked agent from that map makes its tasks resolve to Unknown, and the daemon then writes 'assignee "dev" is not a fleet agent' on them — a note that is true of no parked agent. Skipping inside Next gets the behaviour the AC asked for (no work via explicit assignee or default_agent) with no false write-up; daemon.go needed no change, which matches its own doc comment that deciding is pick's job.
AC 5 has no test: it holds because nothing was added that can stop a run, and a test asserting absent code is tautological. Covered instead by the README and by SetDisabled touching only the persona file.
Verified live on a throwaway fleet (isolated via HERDR_PLUGIN_CONFIG_DIR): pause added one line, resume left the file byte-identical (cmp), agent list flipped active->paused. The live ~/fleet dir and the running daemon's binary were deliberately not touched mid battle-test.
<!-- SECTION:NOTES:END -->
