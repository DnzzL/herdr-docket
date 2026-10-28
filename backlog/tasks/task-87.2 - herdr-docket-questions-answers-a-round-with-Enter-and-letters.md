---
id: TASK-87.2
title: herdr-docket questions answers a round with Enter and letters
status: In Progress
assignee: []
created_date: '2026-10-28 13:30'
updated_date: '2026-10-28 13:30'
labels: []
dependencies:
  - TASK-86.2
parent_task_id: TASK-87
ordinal: 92000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
`herdr-docket questions`: lists `question` cards across queues (id, title, round number). Choosing one shows the newest round question by question: Enter takes the option marked (reco), a letter takes that option, any other text is a free answer; then shows the assembled answer line and asks to send. Sending calls the same code as `task answer`. Plain stdin/stdout, no dependency, nothing over 45 columns.
<!-- SECTION:DESCRIPTION:END -->
