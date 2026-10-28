---
id: TASK-28
title: 'The board acts: read a task, pause an agent, stop a run, re-route'
status: Done
assignee: []
created_date: '2026-09-16 15:49'
updated_date: '2026-09-16 15:59'
labels: []
dependencies: []
priority: high
ordinal: 28000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
With the board telling the truth (TASK-27), it can be given the actions that make it a control surface rather than a view. This is the other half of ADR 0005, and three of these keys write.

- v opens the selected task read-only: body, criteria and notes. The renderer is the one herdr-docket task view already uses, moved so both call it; a second renderer is how the two drift apart.
- p in the agents view pauses and resumes an agent through fleet.SetDisabled, which edits only the disabled line in AGENT.md. A run in flight is untouched and the key does not confirm, so the status line has to say what pause did not do.
- The agents view says whether an agent is busy and on what, so p is a decision rather than a blind toggle.
- x stops the run on the selected row by closing its workspace. The runner already treats a closed workspace as cancellation and ends the task Blocked; the pane makes the call and says stopping.
- s reassigns the selected task through work.Assigner, board only, so a red unknown assignee is fixable where it is seen. A backend that cannot reassign refuses in its own words.
- a asks which queue a task belongs in when the source is a work.MultiSource with more than one queue, typed, between the title and the assignee. Today that flow always fails with a message about --source.

No verdicts: the pane does not close a task as done, failed or blocked. That is the run report, and a human wanting a task dead is using the project own vocabulary on the project own board.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 v shows the selected task body, criteria and notes through the same renderer as herdr-docket task view; one renderer, two callers, covered by a test.
- [x] #2 p toggles disabled for the selected agent through fleet.SetDisabled, the persona file surviving the edit, and the status line says a run already in flight keeps going and that x is what stops it.
- [x] #3 The agents view shows each agent as busy with its in-flight task and elapsed time, or idle.
- [x] #4 Pressing r on a task whose agent is paused still runs it, and says the agent is paused.
- [x] #5 x closes the workspace of the run on the selected row with no confirmation, the status line says stopping, and the task ends Blocked through the existing cancellation path.
- [x] #6 s reassigns the selected task through work.Assigner from the board only, and reports the source own error for a backend that cannot reassign.
- [x] #7 a asks for the queue before the assignee when the source offers more than one, so a task can be created from the pane in a multi-queue fleet; a single-queue fleet keeps its two-step flow.
- [x] #8 The pane offers no way to close a task with a verdict.
<!-- AC:END -->
