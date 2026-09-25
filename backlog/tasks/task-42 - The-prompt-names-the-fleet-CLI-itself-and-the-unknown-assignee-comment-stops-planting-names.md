---
id: TASK-42
title: >-
  The prompt names the fleet CLI itself, and the unknown-assignee comment stops planting names
status: To Do
assignee: []
created_date: '2026-09-25 13:35'
labels: []
dependencies: []
ordinal: 42000
priority: high
---

## Description

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

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 The assembled prompt states an invocation of the fleet CLI that works when the bare command is not on PATH — the running binary's own path — and a prompt test pins it
- [ ] #2 The unknown-assignee comment lists the valid agent names and tells the reader to reassign to one of them; it no longer reads as an invitation to create the missing agent, and a daemon test pins the wording's shape (roster present, reassign framed as the action)
- [ ] #3 Nothing else about the refusal changes: still written once per task and unknown name, still only for names no agent answers to
<!-- AC:END -->
