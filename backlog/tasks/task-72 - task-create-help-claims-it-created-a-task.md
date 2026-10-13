---
id: TASK-72
title: task create --help claims it created a task
status: To Do
assignee: []
created_date: '2026-10-13 12:06'
updated_date: '2026-10-13 12:06'
labels: []
dependencies: []
ordinal: 72000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
herdr-docket task create --help prints 'created docket/ (in To Do)': the flag is taken as the title, backlog prints its own help, and the empty id is reported as a creation.
Done when: --help/-h prints the usage and creates nothing; an empty id from the queue is an error, never 'created'.
Verify by: a test that task create --help exits 0 with the usage and calls no Create; a test that an empty id fails.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 --help and -h print the usage and create nothing
- [ ] #2 An empty id from the queue is an error, not a creation
<!-- AC:END -->
