---
id: TASK-34
title: >-
  A stall sweep nudges runs that ended with work outstanding
status: To Do
assignee: []
created_date: '2026-09-21 19:12'
labels: []
dependencies: []
ordinal: 34000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The runner's reconcile turns silence into `Failed` and a closed workspace into `Blocked`, but the video's failure mode sits one step past that: work that ended with an actionable remainder and nobody picks it up again — a PR opened without a verdict, a `Blocked` task nobody read after the morning, a follow-up a run forgot to file. The video schedules a watchdog four times a day whose whole job is to find those and nudge them; here the honest version is cheaper because reconcile already caught the crash, so the sweep looks only for the quiet remainder: closed tasks whose verdict and notes promise work nobody took, and open work parked for a human that a human never answered.

The sweep is one persona plus one automations entry. It nudges with the fleet's own verbs — `task note` to say what was found, `task assign` to put idle work back on a capable agent, `task create` only when a nudge is not the right shape — and it never reopens a verdict or overrides a human column. A second reviewer it is not: anything ambiguous goes to a human note, because the sweep's value is being noticed early, not being right unattended.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A persona in docs/examples.md describes the sweep: what counts as a stall — the verifiable kinds and nothing wider — and what its nudge verbs are
- [ ] #2 One automations.yaml entry drives it, and a demo sweep finds a deliberately parked remainder and nudges it without touching verdicts or human columns
- [ ] #3 The persona states the hard edge: it never reopens a closed task, never respecces, and anything ambiguous goes to a human note
<!-- AC:END -->
