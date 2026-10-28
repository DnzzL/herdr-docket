---
id: TASK-87.1
title: The daemon says it is alive
status: In Progress
assignee: []
created_date: '2026-10-28 13:29'
labels: []
dependencies: []
parent_task_id: TASK-87
ordinal: 91000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The daemon writes a heartbeat (time of its last tick) in its state dir every tick. `herdr-docket status` prints `daemon live · tick <age> ago`, or `daemon down for <age>` when the heartbeat is older than 3 tick intervals, or `daemon never started`. The runs pane and `questions` reuse the same line.
<!-- SECTION:DESCRIPTION:END -->
