---
id: TASK-38
title: >-
  The host names the branch a run works on
status: To Do
assignee: []
created_date: '2026-09-21 19:40'
labels: []
dependencies: []
ordinal: 38000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
A prefactor, ahead of TASK-24 §2: the host already computes the branch a worktree-mode run is cut from — `host.Provision` builds it out of the task's name and the attempt's tag, hands it to `WorktreeCreate`, and then throws it away. Everything that glues it back on at close time — recording from the agent's note, re-deriving from the workspace — is the fake boundary that TASK-26 documents: only the host knows a branch it named itself.

The slice is one field, carried honestly: a worktree-mode session carries the branch it was provisioned on, and the run's record in history states it the moment it is running. Root mode carries no branch, because inventing one would be the fleet lying about its own output — a root-mode run's delivery is whatever the repo ends up carrying, and TASK-23 already draws that line.

Deliberately out of scope, because each is its own ticket: what the history *does* with the branch beyond naming it (TASK-24 §2), commits and the pull request (TASK-24 §2), the lost-work guard (TASK-23). Nothing further consumes the field yet — it lands first so the record's consumers never build on a stub.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A worktree-mode run's history record names the branch Provision created, from provision time, not derived after the fact
- [ ] #2 A root-mode run's record names no branch, and no record ever implies a branch it does not have
- [ ] #3 The host tests pin both cases, deriving the branch in the fake the way production does — the fake computes the branch from the same spec fields Provision reads, rather than constructing an arbitrary one
- [ ] #4 herdr-docket history prints it where human output already shows the run, and the change is a CHANGELOG line
<!-- AC:END -->
