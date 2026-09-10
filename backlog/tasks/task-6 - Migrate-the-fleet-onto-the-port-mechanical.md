---
id: TASK-6
title: Migrate the fleet onto the port (mechanical)
status: To Do
assignee: []
created_date: '2026-09-10 20:10'
labels: []
dependencies:
  - TASK-4
ordinal: 6000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Move pick, runner, prompt, daemon, main and pane off backlog.Task/backlog.View and onto work.Item/work.Source. Control flow reads Item.Open only; the runner closes with a verdict through the port. This ticket is pure type and call-site movement: the prompt text it emits stays byte-identical, so a run against a Backlog.md fleet behaves exactly as before.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 no package other than internal/work/backlogmd imports internal/backlog directly
- [ ] #2 pick routes on Item.Assignee (a plain string) and Item.Open, with the unknown/parked/busy distinctions preserved
- [ ] #3 runner.Board is replaced by work.Source; claim and reconcile use Open, and the final verdict goes through Close
- [ ] #4 the run prompt is unchanged for a Backlog.md fleet (asserted by the prompt test)
- [ ] #5 existing test suites (adapted to the new types) are green
<!-- AC:END -->
