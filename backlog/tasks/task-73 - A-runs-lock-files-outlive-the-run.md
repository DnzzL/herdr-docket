---
id: TASK-73
title: A run's lock files outlive the run
status: To Do
assignee: []
created_date: '2026-10-13 12:21'
updated_date: '2026-10-13 12:21'
labels: []
dependencies: []
ordinal: 73000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The state dir holds 76 run-taskid-*.lock files, one per task ever run (dishnow, docket, fleet), never removed. Harmless to correctness as far as known (not verified), but the dir grows with every task forever.
Done when: a task's lock file is removed once its run settles and nothing holds it, without opening a window where two runs can take the same task (ADR 0018); the existing files are swept once.
Verify by: a runner test that a settled run leaves no lock file and that a second run on the same task still waits while the first holds it; and live, the state dir carries no lock for a closed task.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A settled run leaves no lock file for its task
- [ ] #2 Two runs on one task still exclude each other
- [ ] #3 Locks left by earlier runs are swept
<!-- AC:END -->
