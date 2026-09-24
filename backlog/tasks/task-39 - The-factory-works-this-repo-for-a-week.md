---
id: TASK-39
title: >-
  The factory works this repo for a week
status: To Do
assignee: []
created_date: '2026-09-21 19:40'
labels: []
dependencies:
  - TASK-24
  - TASK-31
  - TASK-32
  - TASK-33
  - TASK-34
  - TASK-35
ordinal: 39000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The loop's only honest acceptance test is this repo's own backlog, run for a week with the fleet's own factory: intake watching the issues, the dev carrying each PR to green with the delivery record saying what landed, the review sweep clearing the pile and its persona policy merging what it can defend, the stall sweep catching what stalls, the lookback reading the week and filing what repeats. A human still triages, still reads every verdict, still answers what the loop blocks on — the work this ticket buys is knowing the answers a week later without having watched.

The week is where the persona text stops being prose and earns its guardrails: a merge policy that fired on a real diff either defended itself or did not; a stall sweep's nudges were wanted or noise; an intake gate either filed things a human was glad to see or cluttered the triage column. The closing artifact is the finding list — what broke, what got muted by policy after the fact, whether the human's time went up or down — folded back into the factory doc wherever the week contradicted it.

Not in scope: a second source, a second repo, harder merge categories, any dashboard. One repo, one week, the numbers from history, and the write-downs.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 The cycle runs over this repo's own intake and backlog for at least seven days, at the schedules the factory doc states, without a human starting any run
- [ ] #2 Every PR that merged did so through the reviewer persona's policy, and every merge is named in the closing note of the task that produced it
- [ ] #3 At least one lookback follow-up files from a real recurrence pattern in that week, and it closes with its own evidence
- [ ] #4 A write-down names what the week proved wrong or left stranded — a failed persona claim, a policy loosened or tightened, a gap the queue could not say
<!-- AC:END -->
