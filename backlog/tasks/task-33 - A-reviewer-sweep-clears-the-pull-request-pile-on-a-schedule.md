---
id: TASK-33
title: >-
  A reviewer sweep clears the pull request pile on a schedule
status: To Do
assignee: []
created_date: '2026-09-21 19:12'
labels: []
dependencies: []
ordinal: 33000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The reviewer persona exists (example 4 in docs/examples.md) but nothing drives it on a schedule: reviews happen when a human notices a pile. The video's factory runs a review pass every few hours independent of any single fix, and that is what makes its merge step cheap. Here the shape is already documented — a sweep task that reviews the oldest un-reviewed PR and re-tasks itself for the rest — so the work is two things: make the sweep a first-class example beside the per-PR gate, and give it an automations entry so the fleet clears the pile at a fixed hour whether or not anyone shipped that day.

The reviewer's verdict stays evidence-first (file, line, or test run), and blocking findings still become follow-up tasks assigned to `dev`. What changes from the shipped example is the merge step: an approve-shaped verdict no longer waits for a human by default. The persona carries its own merge policy — small, verifiable diffs with the criteria re-derived go forward; the wide ones ask — which is the video's approval policy living in the one place our architecture already keeps policies: persona text. The fleet's code never merges and never will merge; ADR 0008 and the CONTRIBUTING line stand. What changes here is only that review and the verdict no longer depend on a human remembering to look.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 docs/examples.md shows the sweep as a task shape alongside the per-PR gate, with the self-re-tasking loop stated
- [ ] #2 One automations.yaml entry drives the sweep on a stated cron, and a demo sweep reviews a real pile to verdicts and follow-up tasks without a human starting it
- [ ] #3 The entry states who merges: the reviewer agent, per its persona's own policy — safe category and verdict conditions met means it merges and closes done naming the merge; anything else hands to a human column. Policies are text, not fleet code: the fleet never merges and never automerges as a system behavior, and the doc says where the policy lives
<!-- AC:END -->

## Comments

<!-- COMMENTS:BEGIN -->
created: 2026-09-21 19:30
---
The merge half of the persona wants the delivery record (TASK-24 §2) and the lost-work guard (TASK-23) shipped first: an agent that merges can do damage the queue can only catch after the fact, so until history says what a run produced, the verdict hears the agent alone. Sequenced, not enforced — the fleet does not read dependencies.
---
<!-- COMMENTS:END -->
