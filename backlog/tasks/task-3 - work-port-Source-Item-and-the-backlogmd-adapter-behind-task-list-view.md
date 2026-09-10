---
id: TASK-3
title: 'work port: Source/Item and the backlogmd adapter behind task list/view'
status: Done
assignee: []
created_date: '2026-09-10 20:09'
updated_date: '2026-09-10 20:11'
labels: []
dependencies: []
ordinal: 3000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Introduce the backend-blind seam the redesign rests on. A new internal/work package defines Item, Criterion, Source (List, Get) and the Verdict vocabulary; internal/work/backlogmd implements Source on top of the existing internal/backlog client. A new `herdr-fleet task list` / `task view <id>` CLI reads through the port. Additive only: no existing consumer changes, behaviour identical.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 internal/work defines Item{ID,Title,Body,Assignee,Open,Phase,Priority,Ordinal,CreatedAt,Criteria} and Source{List,Get}
- [ ] #2 internal/work/backlogmd implements Source by wrapping backlog.Client, mapping native status onto Open plus Phase
- [ ] #3 herdr-fleet task list lists open items with their routing key; herdr-fleet task view <id> prints title, body and criteria
- [ ] #4 the adapter is tested through the existing Client.run exec seam; no test reaches into internals
- [ ] #5 go build ./... and go test ./... are green
<!-- AC:END -->
