---
id: TASK-8
title: 'Contract: fold internal/backlog into internal/work/backlogmd'
status: To Do
assignee: []
created_date: '2026-09-10 20:10'
labels: []
dependencies:
  - TASK-6
ordinal: 8000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Once no consumer speaks backlog.*, move the Backlog.md client into the adapter package and delete internal/backlog, so there is exactly one client and no duplicate vocabulary. Nothing user-visible changes.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 internal/backlog is deleted and no import of it remains
- [ ] #2 internal/work/backlogmd owns the Backlog.md CLIClient and its tests
- [ ] #3 go build ./... and go test ./... green
<!-- AC:END -->
