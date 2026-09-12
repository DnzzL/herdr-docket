---
id: TASK-19
title: History records how long a run took and how it ended
status: Done
assignee: []
created_date: '2026-09-12 15:08'
updated_date: '2026-09-12 15:26'
labels: []
dependencies: []
priority: medium
ordinal: 19000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
history.jsonl is the fleet's only audit trail and it cannot answer two plain questions: how long did this run take, and what verdict did the agent give. Status says done/failed/cancelled from the run mechanics' point of view; the agent's verdict (done, failed, blocked) is only in the queue. Duration is derivable from the scheduled and running/final records of the same run_id, but only by a reader that reconstructs it — nothing writes it down.

Write them at the point the runner already knows them (runner.Run's final switch): DurationSeconds on the closing record, Verdict on the closing record. Append-only stays append-only; old records without the fields are read as zero/empty. herdr-fleet history shows both. This is the data TASK-19's budget reads, so it goes first.

While in the file: StatusMissed and StatusInvalid (history.go:22-31) are herdr-automations vocabulary, never written by the fleet. Remove them, or say in a comment why the fleet keeps them.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 The closing history record of a run carries duration_seconds and verdict; records written before this ticket still load
- [ ] #2 herdr-fleet history shows duration and verdict per run
- [ ] #3 StatusMissed and StatusInvalid are either removed or their presence is explained in a comment
- [ ] #4 go test ./... is green
<!-- AC:END -->
