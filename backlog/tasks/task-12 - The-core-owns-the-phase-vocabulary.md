---
id: TASK-12
title: The core owns the phase vocabulary
status: To Do
assignee: []
created_date: '2026-09-12 08:26'
labels: []
dependencies: []
priority: medium
type: task
ordinal: 12000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The board shows phases in the fleet's words, and no backend's status word reaches the core. To Do and In Progress become the core's constants, the three endings derive from the verdict vocabulary's labels rather than a second hand-written list, and both adapters map their backend's labels into them. See docs/adr/0001-the-core-owns-the-vocabulary.md.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 the phase ordering table's three closed entries are derived from the verdict vocabulary, so they cannot drift
- [ ] #2 the pane styles phases from the core's words, not four hand-typed strings
- [ ] #3 both adapters map their backend's labels into the core's words
- [ ] #4 the board renders the same headings and colours as before: no behaviour change
- [ ] #5 go test ./... is green
<!-- AC:END -->
