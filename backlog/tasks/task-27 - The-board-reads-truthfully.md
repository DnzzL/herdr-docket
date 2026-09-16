---
id: TASK-27
title: The board reads truthfully
status: Done
assignee: []
created_date: '2026-09-16 15:49'
updated_date: '2026-09-16 15:59'
labels: []
dependencies: []
priority: high
ordinal: 27000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The pane is the fleet board, and it does not currently tell the truth about the queue. Five faults, all of them on screen at once.

- In a fleet with several queues every task id is prefixed with its queue name (myapp/TASK-12, ADR 0003) and the id column is nine characters wide. The prefix is truncated away, so the id a person needs in order to run anything is the id the board hides.
- Tasks from every queue are interleaved inside each phase, with nothing naming which project a row belongs to.
- The board renders the order the backend returned. The daemon works the order pick.Next computes (priority, then ordinal, then oldest first). The two disagree silently, so the top row is not what runs next.
- Selection is an index into the row list. Rows already move under it as the queue changes; once the order is the scheduler order they move constantly, and the next r or x lands on whatever took that position.
- A task with a live run is indistinguishable from one waiting, and nothing says how long it has been going or against what timeout.

The decisions behind the fix are in docs/adr/0005-the-board-is-a-triage-surface.md. This task is the read-only half: it changes what the board shows, and nothing about what it can do. The writes (v, p, x, s, and the queue step in a) are a separate task, so a column-width change does not arrive carrying an irreversible key.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Each row shows the queue a task came from and the task local id; actions still route by the full prefixed id. A fleet with one queue shows no project column.
- [x] #2 Rows in an open phase are ordered by the same comparison pick.Next uses (priority desc, ordinal, createdAt); closed phases keep backend order. A test pins the board order against the scheduler preference.
- [x] #3 Selection is keyed by task id and survives a refresh that reorders rows; a task that leaves the list moves the cursor to the nearest surviving row, not to the top. The filter behaves the same way while typing.
- [x] #4 A task whose latest history record is running appears once, in a Running group ranked above To Do, with elapsed time against the agent timeout_minutes, marked stale past that timeout. It appears nowhere else.
- [x] #5 The pane package doc points at ADR 0005, which states what the board refuses as well as what it shows.
- [x] #6 Tests cover rows(), the order comparison and the cursor as pure functions, in the shape internal/pane/pane_test.go already uses.
<!-- AC:END -->
