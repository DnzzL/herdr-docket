---
id: TASK-17
title: 'One source per process: the runner writes to the queue the daemon read'
status: To Do
assignee: []
created_date: '2026-09-12 15:08'
labels: []
dependencies: []
priority: high
ordinal: 17000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
internal/daemon/daemon.go:46 builds the Runner once, at startup, with fleet.NewSource(settings). daemon.go:94 rebuilds a fresh Source on every tick and picks from that one. Two sources in one process: if fleet.yaml changes the queue (kind, Basecamp lists, and soon which sources exist), pick reads the new queue while claim/Comment/Close write to the old one. Today it is a config-drift bug nobody has hit; with several sources behind the port (TASK-18) it becomes the normal case, so it goes first.

One source per evaluation, and every side effect of that evaluation goes through it. Either Run takes the Source as a parameter (pick and run agree by construction) or the daemon rebuilds the Runner when the settings it was built from change. Prefer the parameter: the Runner already holds nothing about the source but the reference.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 The daemon uses exactly one Source per tick: the tasks it picks from and the one it claims, comments and closes on are the same value
- [ ] #2 A test changes the source between two ticks and shows the run's writes land on the new queue
- [ ] #3 herdr-fleet run and the pane are unchanged in behaviour, and still build their source through fleet.NewSource
- [ ] #4 go test ./... is green
<!-- AC:END -->
