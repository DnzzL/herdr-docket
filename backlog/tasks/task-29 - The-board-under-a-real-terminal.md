---
id: TASK-29
title: The board under a real terminal
status: Done
assignee: []
created_date: '2026-09-16 16:18'
updated_date: '2026-09-16 16:18'
labels: []
dependencies: []
priority: high
ordinal: 29000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Findings from running the pane for real against a throwaway two-queue fleet (tmux, 132x30), rather than from reading it. All of them are in the reading path, and none is visible to a test that drives the model by hand.

1. Nested styles. headerStyle.Render(phaseStyle(h).Render(h)) hands a string that already carries escape codes to a second Render, which draws them as text, so the board printed [1;36mRunning instead of a coloured heading. It never shows through a pipe (no colour profile), which is why eyeballing the output in a non-tty misses it. phaseHeader is one style, one Render.

2. The detail view drew the row. List carries what a board draws, and v showed those fields, so a task read in the pane had no description and no notes — and looked right doing it. It reads Get now, and an answer for a task the reader has left behind is dropped.

3. A prompt lost most of what was typed. A run of characters arrives as one KeyMsg — a paste, or typing faster than the reader drains — and the prompt kept only the one-rune messages, so a fast typist lost most of a title and the search box looked dead after /. Spaces arrive as their own KeySpace event, never inside the run.

4. The agent column could not say paused. It is the one fact that explains a queue that is not moving, and the key that sets it (p) is on a different view.

5. The running row's timeout came from the task's current routing rule, so re-routing a task mid-run changed the number its in-flight run was being held to. It reads the record's agent first: that is the number the daemon launched with.

6. The roster's run column was a fixed 36-character truncation, so the promise that the elapsed time is never cut was not kept — a long title pushed the timer off the terminal. The column now takes what the fixed columns leave, timer first, title last.

The first three are covered by tests that fail without the fix. The rest are one-line changes with a test each.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 A phase header renders with its colour, not with escape codes as text
- [x] #2 v reads the task with Get, and an answer for a task the reader has left behind is ignored
- [x] #3 A multi-rune key message lands in the prompt whole, spaces included
- [x] #4 The agent column marks a paused agent, and the running row counts against the agent that started it
- [x] #5 The roster's run column keeps the timer when the terminal is narrow
- [x] #6 go test ./... is green
<!-- AC:END -->
