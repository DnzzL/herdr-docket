---
id: TASK-42
title: >-
  The prompt names the fleet CLI itself, and the unknown-assignee comment stops
  planting names
status: Done
assignee:
  - plugin-dev
created_date: '2026-09-25 13:35'
updated_date: '2026-10-01 16:13'
labels: []
dependencies: []
priority: high
ordinal: 42000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
<!-- SECTION:DESCRIPTION:BEGIN -->
<!-- SECTION:DESCRIPTION:BEGIN -->
<!-- SECTION:DESCRIPTION:BEGIN -->
<!-- SECTION:DESCRIPTION:BEGIN -->
<!-- SECTION:DESCRIPTION:BEGIN -->
Both halves were found by the factory's first live review leg (2026-09-25)
and both are mitigated in FLEET.md for now — this task makes them structural.

**The prompt does not say where the CLI is.** Every agent session that day
hit `herdr-docket: command not found` — the plugin binary is not on the
panes' PATH — and burned twenty to thirty seconds rediscovering
`~/Projects/herdr-docket/bin` before the first verb of its protocol. Intake
got lucky, the reviewer had to hunt. The binary assembling the prompt is the
binary; it knows where it lives (`os.Executable()`), so the protocol section
should name an invocation that works: the absolute path, offered as the
fallback when the bare command is not on PATH.

**The unknown-assignee comment plants the name it warns about.** The daemon's
refusal quotes the ghost (`assignee "dishnow-dev" is not a fleet agent`) and
then offers `add agents/dishnow-dev/AGENT.md` — an instruction-shaped option
that a model may well take. One did: during the review leg a retired name
was written back onto a task mid-run (mtime pinned into the run window; no
session log shows the exact call, so the writer is unproven — the honeypot is
not). The comment can carry the fix with it: the agents map is already in
scope at the refusal, so name the roster and make the instruction
"reassign to one of these" — the create-an-agent option is a human's
decision and can read as one.
<!-- SECTION:DESCRIPTION:END -->
<!-- SECTION:DESCRIPTION:END -->
<!-- SECTION:DESCRIPTION:END -->
<!-- SECTION:DESCRIPTION:END -->
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 The assembled prompt states an invocation of the fleet CLI that works when the bare command is not on PATH — the running binary's own path — and a prompt test pins it
- [ ] #2 The unknown-assignee comment lists the valid agent names and tells the reader to reassign to one of them; it no longer reads as an invitation to create the missing agent, and a daemon test pins the wording's shape (roster present, reassign framed as the action)
- [ ] #3 Nothing else about the refusal changes: still written once per task and unknown name, still only for names no agent answers to
<!-- AC:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
fleet: default_agent "plugin-dev" (fleet.yaml) is not a fleet agent.

Half delivered by 01dbc01 (this session, squashed into 48b6e27): the prompt now names an invocation an agent can run — absolute when the bare name is not on PATH, prose left alone. That half was the fleet's dominant failure: 17 of 18 failures in a fortnight were runs that did the work and then hit 'herdr-docket: command not found' on their closing verb. Proven live — docket/TASK-45 at 16:08 closed Done with its PR url, the first clean report after a fortnight of silence.

STILL OPEN: the second half. The unknown-assignee comment still plants a retired name in the task body, which is how dishnow/TASK-65 and docket/TASK-26 ended up routed to agents that do not exist.

Second half of the task: the daemon's unknown-assignee refusal now names the roster and says reassign, instead of offering 'add agents/<name>/AGENT.md' — the instruction-shaped option a model took during the 2026-09-25 review leg. New daemon test (TestAnUnknownAssigneeIsToldWhoDoesAnswer) pins the shape: each agent name present, reassign framed as the action, no agent-creation recipe; one refusal per task+name over two ticks. Refusal timing and unknown-only trigger untouched. Build/vet/test/gofmt clean; PR #8 open and green. First half (prompt naming the CLI by absolute path, pinned by internal/prompt/cli_test.go) was already in 48b6e27.

Both halves done, PR #8 open and green (build pass).
#1 met: already delivered in 48b6e27 — internal/prompt/prompt.go cliFor names the binary's absolute path when the bare command is not on PATH, rewritten into the indented command lines only; internal/prompt/cli_test.go pins both branches.
#2 met: the unknown-assignee refusal now reads 'assignee "X" is not a fleet agent — reassign the task to one of: dev, pm, ...' listing the agents in scope, sorted; the 'add agents/<name>/AGENT.md' recipe is gone. TestAnUnknownAssigneeIsToldWhoDoesAnswer pins: roster names listed, reassign is the action, no 'add agents/' and no 'AGENT.md', one note over two ticks.
#3 met: still written once per task+unknown-name (repeated tick wrote nothing more), still only for names no agent answers to — the pick tests that a parked/budget-spent agent is never called unknown, and the misroute note path, are unchanged and green.
The default_agent (fleet.yaml) variant of the note is untouched, as criterion 2 names only the unknown-assignee comment. Verified locally: go build, go vet, go test, gofmt -l all clean; CI on PR #8 green.

fleet: this run's delivery could not be read — workspace w5R (pane w5R:p1) is kept, unverified. The verdict stands.
<!-- SECTION:NOTES:END -->
