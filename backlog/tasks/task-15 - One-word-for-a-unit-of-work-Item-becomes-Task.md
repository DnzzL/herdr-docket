---
id: TASK-15
title: 'One word for a unit of work: Item becomes Task'
status: To Do
assignee: []
created_date: '2026-09-12 08:26'
labels: []
dependencies: []
priority: low
type: chore
ordinal: 15000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The Go type for a task is called Task, matching CONTEXT.md and the CLI's task verbs. A mechanical, compiler-checked rename with no behaviour change.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 work.Item is work.Task everywhere it is referenced
- [ ] #2 no behaviour change
- [ ] #3 go test ./... is green
<!-- AC:END -->
