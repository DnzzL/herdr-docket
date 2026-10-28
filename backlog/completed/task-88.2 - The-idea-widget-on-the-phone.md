---
id: TASK-88.2
title: The idea widget on the phone
status: Done
assignee: []
created_date: '2026-10-28 13:30'
updated_date: '2026-10-28 13:41'
labels: []
dependencies:
  - TASK-88.1
parent_task_id: TASK-88
ordinal: 96000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
`scripts/termux/idea`: Termux:Widget shortcut. Reads BOX (ssh target) from `~/.config/herdr-docket/idea.env`; lists queues with `ssh $BOX herdr-docket queues` into `termux-dialog radio`; asks the text with `termux-dialog text`; runs `ssh $BOX herdr-docket task create -s <queue> "<text>"`; toasts the new id. `--dry-run` prints the command. README "Day to day" section (phone and laptop loops, from the grilling session) and install steps.
<!-- SECTION:DESCRIPTION:END -->
