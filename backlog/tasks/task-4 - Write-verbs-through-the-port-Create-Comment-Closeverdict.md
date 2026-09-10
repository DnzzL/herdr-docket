---
id: TASK-4
title: 'Write verbs through the port: Create, Comment, Close(verdict)'
status: To Do
assignee: []
created_date: '2026-09-10 20:09'
labels: []
dependencies:
  - TASK-3
ordinal: 4000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Extend internal/work with Create(title, body, assignee), Comment(id, text) and Close(id, verdict), where verdict is one of done|failed|blocked and Close ALWAYS closes the backend item (a blocked or failed task must not sit open, or a binary backend re-picks it). Implement all three in the backlogmd adapter and expose them as `herdr-fleet task create`, `task note`, and `task done|fail|block`. The verdict text is recorded on the item; the agent never touches the backend.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 internal/work.Source gains Create, Comment and Close(id, verdict) with the verdict vocabulary
- [ ] #2 backlogmd maps done to Done, failed to Failed, blocked to Blocked, and records the verdict text
- [ ] #3 herdr-fleet task create|note|done|fail|block round-trip through the adapter
- [ ] #4 Close always closes: an unknown verdict is rejected, never silently ignored
- [ ] #5 go test ./... green
<!-- AC:END -->
