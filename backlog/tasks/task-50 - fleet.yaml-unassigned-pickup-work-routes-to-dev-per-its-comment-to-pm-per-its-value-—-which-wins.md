---
id: TASK-50
title: >-
  fleet.yaml: unassigned pickup work routes to dev per its comment, to pm per
  its value — which wins?
status: To Do
assignee:
  - pm
created_date: '2026-10-01 17:30'
updated_date: '2026-10-02 08:14'
labels: []
dependencies: []
ordinal: 50000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
One question for a human: should unassigned work in the pickup column go to dev (as the comment says) or pm (as the value says)?

Evidence: ~/.config/herdr/plugins/config/dnzzl.herdr-docket/fleet.yaml, dishnow source block — comment: 'Unassigned work in the pickup column goes to dev: an assignee is a decision, a missing one is delegation (intake's gate-passers rely on this).' value directly below: 'default_agent: pm'. Contradiction is live; TASK-106 finding 2 surfaced it ('Decide which side is correct') and no task ever homed it (grep: only the closed intake reports mention it).

Gate: fails for dev — the fix is a decision, not a defect with a repro; picking either side silently changes routing of every unassigned dishnow task. Answer decides which way; then the config change itself is a one-line edit.

Filed by fleet/TASK-8 (intake, 2026-10-01).
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
fleet: docket/TASK-50 is docket work but pm works dishnow — reassign it to an agent of docket, or point pm at that checkout.

fleet: docket/TASK-50 is docket work but pm works dishnow — reassign it to an agent of docket, or point pm at that checkout.

fleet: docket/TASK-50 is docket work but pm works dishnow — reassign it to an agent of docket, or point pm at that checkout.
<!-- SECTION:NOTES:END -->
