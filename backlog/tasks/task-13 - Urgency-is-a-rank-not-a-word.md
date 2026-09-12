---
id: TASK-13
title: 'Urgency is a rank, not a word'
status: To Do
assignee: []
created_date: '2026-09-12 08:26'
labels: []
dependencies: []
priority: high
type: task
ordinal: 13000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Which task runs next is decided by comparing ranks the backend computed, so the decision package knows no backend's priority words. Priority becomes an orderable rank whose zero value means the backend has no opinion and unprioritised work sorts last; Backlog.md's priority words live only inside the Backlog.md adapter; the decision package compares ranks and still states the fleet's ordering policy.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Priority carries an orderable rank, not a backend word; its zero value means the backend has no opinion and unprioritised work sorts last
- [ ] #2 Backlog.md's priority words exist only inside the Backlog.md adapter, which turns them into ranks
- [ ] #3 the decision package compares ranks and still states the fleet's ordering policy: urgency, then the backend's position, then oldest
- [ ] #4 Backlog.md ordering is identical to before; Basecamp ties and falls through exactly as before
- [ ] #5 go test ./... is green
<!-- AC:END -->
