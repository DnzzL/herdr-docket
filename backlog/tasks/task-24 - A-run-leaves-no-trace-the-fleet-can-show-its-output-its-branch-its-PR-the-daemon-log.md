---
id: TASK-24
title: >-
  A run leaves no trace the fleet can show: its output, its branch, its PR, the
  daemon log
status: To Do
assignee: []
created_date: '2026-09-13 18:14'
updated_date: '2026-09-16 15:50'
labels: []
dependencies: []
priority: high
ordinal: 24000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The board pane is now a triage surface (ADR 0005): it shows what runs next and what is running, and it acts on both. Reading a task, pausing an agent, stopping a run and re-routing work all happen there, so the half of this task the board was the right place for has landed in TASK-27 and TASK-28.

What the board refused is still real work, and ADR 0005 draws the line it sits on the other side of: each of these is a report about work that has happened, not a decision about work that has not.

- The agent live output, one keystroke from the row, instead of a generated name discovered by hand.
- History that records what a run produced: its branch, whether it committed, a pull request url.
- The daemon log, readable from the CLI rather than a path someone has to guess.
- A run that ends failed reaching a human who was not watching.

Where the answer is that Herdr should expose something, say so and stop: a wrapper that reimplements its host is the failure mode on the other side.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 The agent live output is reachable from the fleet own surfaces without discovering a generated name by hand.
- [ ] #2 herdr-docket history records what a run produced - at minimum its branch and any pull request - rather than leaving it in prose inside a task note.
- [ ] #3 The daemon log is readable from the CLI.
- [ ] #4 A run that ends failed can reach a human who was not watching, or the task states why that belongs to Herdr.
<!-- AC:END -->

## Comments

<!-- COMMENTS:BEGIN -->
created: 2026-09-16 15:50
---
Split by ADR 0005 (the board is a triage surface): the board half is TASK-27 (reads truthfully) and TASK-28 (acts); this task keeps what the board refuses - live output, branch and PR in history, the daemon log, reaching a human after a failure.
---
<!-- COMMENTS:END -->
