---
id: TASK-86.3
title: A triage run grills or publishes
status: In Progress
assignee: []
created_date: '2026-10-28 13:29'
updated_date: '2026-10-28 13:30'
labels: []
dependencies:
  - TASK-86.1
  - TASK-86.2
parent_task_id: TASK-86
ordinal: 89000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
A triage run: the daemon starts the triager on a `triage` card with a prompt that carries the card, its notes (rounds and answers), and the `triager` role. `init` ships `roles/triager.md` (alongside dev and reviewer): the clear/uncertain rule, the Round format, one round = the whole current frontier of questions with a recommendation each (the grilling skill's rule), the final confirm round, the `grill` label, and how to publish (rewrite the card body as the spec, tickets per the repo's issue-tracker.md, assign the author, status To Do) or ask (`task note` the round, `task ask`). docs/examples.md copies the role (drift test). README and working-with-a-factory describe the grill as a card state.
<!-- SECTION:DESCRIPTION:END -->
