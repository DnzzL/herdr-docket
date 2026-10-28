---
id: TASK-30
title: The board promises a stop it did not make
status: Done
assignee: []
created_date: '2026-09-21 13:12'
updated_date: '2026-09-21 13:17'
labels: []
dependencies: []
priority: high
ordinal: 30000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The code review of TASK-27/28 found that `x` on a stale row reports `stopping TASK-2…` and never corrects itself. The promise is permanent because nothing follows it: `herdr.WorkspaceClose` returns nil when herdr answers `workspace_not_found` — deliberately, the point was for the workspace not to exist — so the pane cannot tell "I closed it" from "there was nothing left to close". A row whose record says running but whose workspace is long gone is a run nobody is watching, and the board goes on claiming a stop that cannot happen.

ADR 0008's consequence says the board shows `stopping` until the next refresh brings the truth. The refresh never brought it: refreshMsg leaves the status line alone.

## What to do

`herdr.WorkspaceClose` reports the workspace as already gone, exactly as `Focus` already does with the same code — the two calls answering the same herdr error differently is what left the pane without the fact it needed. The host port keeps its own contract (a gone workspace is a torn-down one, because the runner is not the caller that cares) and the pane says what it found: the workspace was closed, or it was already gone. Neither sentence promises anything the pane cannot deliver.

Not in scope: a `running` history record that no process is behind is the daemon's truth to restore (a run in flight when the daemon died keeps its record forever). That belongs to the reconciliation work, not to the pane.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 herdr.WorkspaceClose reports ErrGone when herdr says the workspace is gone, with a test that crosses a real process boundary
- [x] #2 The host port still treats a gone workspace as torn down, so the runner does not log a benign case as a failure, pinned by a test
- [x] #3 x on a run whose workspace is already gone says what it found instead of promising a stop, pinned by a test that runs the returned command against a fake herdr
- [x] #4 x on a run whose workspace was closed says so and refreshes, pinned by a test
- [x] #5 ADR 0008's consequence bullet describes what the pane now does, and the CHANGELOG entry says why
<!-- AC:END -->
