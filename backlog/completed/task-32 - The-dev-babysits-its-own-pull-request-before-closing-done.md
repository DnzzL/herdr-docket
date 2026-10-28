---
id: TASK-32
title: The dev babysits its own pull request before closing done
status: Done
assignee: []
created_date: '2026-09-21 19:12'
updated_date: '2026-10-02 09:44'
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

AC#3 evidence arrived since the 10-01 reconciliation: the rebase run on fleet/TASK-12 (closed done 10-02 10:53, history poll run) babysat PR DnzzL/DishNow-v2#42 end to end. Verified live, not from the notes alone: gh pr view 42 — head 24c98c9, mergeStateStatus CLEAN, checks 4/4 SUCCESS (Lint/Type Check/Test/Build, GH run 36985691369); the reviewer run on fleet/TASK-11 left one review comment with two gates (no CI on head, DIRTY conflicts); the run answered it in-thread (issue comment 5948517808: conflicts resolved taking PR side, Collections kept, CI green on 24c98c9, mergeable CLEAN) before any closing note on fleet/TASK-12. dishnow/TASK-65 carries the closing note with the PR url. AC#1/#2 remain met by d735a53 and 0f66131, both ancestors of origin/main, checked with merge-base --is-ancestor. No new code needed from this run; no PR.

Verified from the queue and live GitHub state, not re-done: AC#1 met (docs/examples.md persona states no done while CI red or comment unanswered; commit d735a53, ancestor of origin/main), AC#2 met (hand-on-instead-of-close via task note + task assign; commit 0f66131, ancestor of origin/main), AC#3 now met — the run babysat PR DnzzL/DishNow-v2#42 end to end: reviewer comment answered in-thread (comment 5948517808) after CI went 4/4 green on head 24c98c9 (run 36985691369) with mergeStateStatus CLEAN, and only then the closing note on fleet/TASK-12 (done 10-02 10:53); dishnow/TASK-65 carries the PR-url note. Evidence gathered live with gh pr view 42 and the run history. No scope crept onto TASK-33; reviewer unchanged.
<!-- SECTION:NOTES:END -->
