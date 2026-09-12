---
id: TASK-14
title: 'The core''s prose says queue, phase and verdict'
status: To Do
assignee: []
created_date: '2026-09-12 08:26'
labels: []
dependencies: []
priority: low
type: chore
ordinal: 14000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Everything the fleet prints, logs, or hands an agent uses the fleet's words instead of the first backend's. The daemon, the prompt, the runner and the CLI say queue, phase and verdict; the port documents that a task's creation timestamp sorts chronologically as bytes; the code that genuinely scaffolds a Backlog.md project keeps its accurate Backlog.md names.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 the daemon's messages and package doc say queue, not backlog
- [ ] #2 the prompt no longer calls the queue a backlog, and the closing protocol calls a settled run a verdict
- [ ] #3 the CLI's tagline and usage text say queue and phase
- [ ] #4 the port documents that a task's creation timestamp sorts chronologically as bytes
- [ ] #5 the code that genuinely scaffolds a Backlog.md project keeps its accurate Backlog.md names
- [ ] #6 go test ./... is green
<!-- AC:END -->
