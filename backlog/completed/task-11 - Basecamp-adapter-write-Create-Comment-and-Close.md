---
id: TASK-11
title: 'Basecamp adapter (write): Create, Comment and Close'
status: Done
assignee: []
created_date: '2026-09-10 20:10'
updated_date: '2026-09-10 20:47'
labels: []
dependencies:
  - TASK-10
ordinal: 11000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Complete the Basecamp adapter. task create posts a to-do into the agent's list (the container is the routing carrier); task note posts a comment; task done|fail|block completes the to-do and comments the verdict, so a blocked or failed item never sits open and never gets re-picked. The adapter passes the shared conformance suite.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Create posts into the list that carries the assignee and returns the new item's id
- [x] #2 Comment appends a comment to the to-do
- [x] #3 Close completes the to-do and records the verdict as a comment, for all three verdicts
- [x] #4 the basecamp adapter passes worktest.Contract
- [x] #5 go test ./... green
<!-- AC:END -->
