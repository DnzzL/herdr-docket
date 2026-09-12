---
id: TASK-18
title: 'Several queues behind one port: a composite Source, one per project'
status: To Do
assignee: []
created_date: '2026-09-12 15:08'
labels: []
dependencies:
  - TASK-17
priority: high
ordinal: 18000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Today one fleet is one queue: fleet.yaml has a single source: block, so every agent — whatever project it works on — reads the same Backlog.md project or the same Basecamp account. A project that tracks its work in Basecamp and another that tracks it in markdown cannot both be worked by one daemon. Each project should be able to keep its own tool.

The seam is already there. Everything above the port (daemon, pick, runner, pane, CLI) speaks work.Source and constructs it in exactly one place, fleet.NewSource (internal/fleet/source.go:42). So the change is an adapter, not a rewrite: a composite Source that holds named sub-sources.

- fleet.yaml grows a sources: map, name → the same block source: takes today. The singular source: stays valid and means one unnamed source: existing configs work unchanged, and no prefix appears anywhere.
- internal/work/multi: List concatenates every sub-source's List, each task ID prefixed with its source name (myapp/TASK-12, bc/987654). Get, Comment, Close, SetPhase split the prefix and dispatch. The composite implements Phaser and forwards only to sub-sources that do. A prefix naming no source is an error, never a silent no-op.
- Create needs a target: task create gets -s/--source, required when more than one source is configured. prompt.go already templates the follow-up command, so it writes the current task's source into it and the agent never has to think about it.
- Routing is unchanged: the assignee is still a global agent name. Two projects both wanting a pm name their agents myapp-pm and other-pm; scoping agents to sources is not part of this ticket.
- ADR 0002 holds per source: List is one page per sub-source, so N sources cost N requests per poll. Record that in an ADR alongside the ID-prefix decision.

Depends on TASK-17: with several sources, pick and run disagreeing on which Source they hold is no longer a corner case.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 fleet.yaml accepts sources: {name: <source block>}; the singular source: still works and produces no ID prefix
- [ ] #2 With two sources configured, herdr-fleet list and the pane show every open task from both, IDs prefixed by source name
- [ ] #3 task view, note, done|fail|block and the daemon's claim/close on a prefixed ID reach the right sub-source; an unknown prefix is an error
- [ ] #4 task create requires -s when several sources exist and the assembled prompt's follow-up command carries the current task's source
- [ ] #5 The composite passes the adapter conformance suite, and a unit test covers Phaser forwarding to a sub-source that has it beside one that does not
- [ ] #6 An ADR records the ID-prefix scheme and the N-requests-per-poll cost
- [ ] #7 README's Where the queue lives documents sources: with a two-project example
- [ ] #8 go test ./... is green
<!-- AC:END -->
