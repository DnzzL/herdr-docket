---
id: TASK-88.1
title: A new card without an assignee goes to triage
status: Done
assignee: []
created_date: '2026-10-28 13:30'
updated_date: '2026-10-28 13:39'
labels: []
dependencies:
  - TASK-86.1
parent_task_id: TASK-88
ordinal: 95000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
`task create` with no `-a` on a queue that has a `triager` files the card into the `triage` status; without a triager it keeps today's behaviour (ADR 0012). `herdr-docket queues` lists the queue names one per line (for scripts).
<!-- SECTION:DESCRIPTION:END -->
