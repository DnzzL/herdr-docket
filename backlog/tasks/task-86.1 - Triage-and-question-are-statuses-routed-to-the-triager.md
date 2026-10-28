---
id: TASK-86.1
title: 'Triage and question are statuses, routed to the triager'
status: In Progress
assignee: []
created_date: '2026-10-28 13:29'
labels: []
dependencies: []
parent_task_id: TASK-86
ordinal: 87000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The status vocabulary gains two roles: `triage` (default `Triage`) and `question` (default `Question`), in work.Vocabulary, both adapters' status mapping, `init <repo>`'s status patching, and docs/queues.md. `source.triager: <agent>` names the triage agent (like `verifier:`). The pick routes a `triage` card to the triager whatever its assignee, skips `triage` cards when no triager is set, and never picks a `question` card.
<!-- SECTION:DESCRIPTION:END -->
