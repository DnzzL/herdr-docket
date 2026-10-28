---
id: TASK-86.2
title: task ask and task answer move a card through its rounds
status: Done
assignee: []
created_date: '2026-10-28 13:29'
updated_date: '2026-10-28 13:34'
labels: []
dependencies: []
parent_task_id: TASK-86
ordinal: 88000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Two verbs on `herdr-docket task`. `ask <id>` moves a card to `question` (the triager calls it after posting a Round note). `answer <id> "<answers>"` finds the newest `## Round N` note, posts `## Answer N` + the answers as a note, and moves the card to `triage`; with no round it fails naming the card. Notes go through the existing note path (GitHub: trusted comments, spec 1).
<!-- SECTION:DESCRIPTION:END -->
