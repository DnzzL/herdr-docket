---
id: TASK-32
title: The dev babysits its own pull request before closing done
status: To Do
assignee: []
created_date: '2026-09-21 19:12'
updated_date: '2026-10-01 09:38'
labels: []
dependencies: []
ordinal: 32000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The dev persona (example 2 in docs/examples.md) opens a PR per ticket and closes `done`. Nothing tells it to stay until the PR is in shape: a review comment unanswered, CI red, merge conflicts — the run ends and the human inherits the pile. The video's `babysit` is exactly this and it is one paragraph of persona, not a system: the closing note is earned only when CI is green and every comment is answered or refuted with a reason in the PR thread.

Scope guard: this is the dev persona's own behaviour, not a scheduled sweep — the sweep is TASK-33's job, and the reviewer persona stays the second pair of eyes. The persona edit also states the honest exit: if babysitting exceeds the run's budget, hand the task on with `task note` and `task assign` naming what is still on the PR, so the pile the human inherits is described, not just left.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 The dev persona in docs/examples.md states the babysit rule: no `done` while CI is red or a review comment is unanswered or refuted without a reason
- [ ] #2 The persona states what to do when babysitting outgrows the run's budget — hand on, not close
- [ ] #3 A run babysits end to end on a real PR: comments answered or refuted in-thread, CI green, only then the closing note
<!-- AC:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
fleet: default_agent "plugin-dev" (fleet.yaml) is not a fleet agent.

Reconciliation 2026-10-01 — NOT closed. AC#1 and AC#2 are met by d735a53 and 0f66131: the dev persona states the babysit rule and the hand-on-instead-of-close rule. AC#3 is NOT, and the record argues against it: on 09-25 five dev runs opened PRs (#36,#38,#39,#40,#41) and four were recorded as ending without reporting a verdict, so no run has been observed answering review comments in-thread and then closing on a green PR. Met when one run does that end to end.
<!-- SECTION:NOTES:END -->
