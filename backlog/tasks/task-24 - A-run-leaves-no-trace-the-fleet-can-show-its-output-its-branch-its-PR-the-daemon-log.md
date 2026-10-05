---
id: TASK-24
title: >-
  A run leaves no trace the fleet can show: its output, its branch, its PR, the
  daemon log
status: Done
assignee: []
created_date: '2026-09-13 18:14'
updated_date: '2026-10-03 20:12'
labels: []
dependencies: []
priority: high
ordinal: 24000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The board pane is now a triage surface (ADR 0005): it shows what runs next and what is running, and it acts on both. Reading a task, pausing an agent, stopping a run and re-routing work all happen there, so the half of this task the board was the right place for has landed in TASK-27 and TASK-28.

What the board refused is still real work, and ADR 0005 draws the line it sits on the other side of: each of these is a report about work that has happened, not a decision about work that has not.

- The agent live output, one keystroke from the row, instead of a generated name discovered by hand.
- History that records what a run produced: its branch, whether it committed, a pull request url.
- The daemon log, readable from the CLI rather than a path someone has to guess.
- A run that ends failed reaching a human who was not watching.

Where the answer is that Herdr should expose something, say so and stop: a wrapper that reimplements its host is the failure mode on the other side.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 The agent live output is reachable from the fleet own surfaces without discovering a generated name by hand.
- [ ] #2 herdr-docket history records what a run produced - at minimum its branch and any pull request - rather than leaving it in prose inside a task note.
- [ ] #3 The daemon log is readable from the CLI.
- [ ] #4 A run that ends failed can reach a human who was not watching, or the task states why that belongs to Herdr.
<!-- AC:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
fleet: default_agent "plugin-dev" (fleet.yaml) is not a fleet agent.

fleet: the agent settled without reporting a verdict.

fleet: the run's workspace w52 (pane w52:p1) is left open — jump in to resume.

Stall sweep (fleet/TASK-17, docket board), pass 6 — cannot classify this under my three stall types; addressed to the human, moving on. Facts: in the human column since the run failed 2026-10-01 13:37 local ("the agent settled without reporting a verdict"); the history record does NOT flag uncommitted work (it records commits:1 on branch fleet/docket-task-24-a-run-leaves-no-trace-the-fleet-can-show-its-output-its-branch-its-PR-the-daemon-log-20261001-1326, but that branch is not on origin — git ls-remote empty); the task states no blocking question; it is not closed. Half the work appears landed: PR herdr-docket#3 ("a failed run speaks, the daemon log is one command away") merged 2026-10-01T13:12:46Z; workspace w52 kept open. Ask of the human: say what decides this task — are AC#1-#4 judged against PR #3 plus the later TASK-19/TASK-42 work, or is the w52 commit fetched? — or route it. Nudge only: I do not reopen, close, assign, or touch worktrees.

Stall sweep (fleet/TASK-17, docket board), pass 6 — cannot classify this under my three stall types; addressed to the human, moving on. Facts: in the human column since the run failed 2026-10-01 13:37 local (the agent settled without reporting a verdict); the history record does NOT flag uncommitted work (it records commits:1 on branch fleet/docket-task-24-a-run-leaves-no-trace-the-fleet-can-show-its-output-its-branch-its-PR-the-daemon-log-20261001-1326, but that branch is not on origin — git ls-remote empty); the task states no blocking question; it is not closed. Half the work appears landed: PR herdr-docket#3 (a failed run speaks, the daemon log is one command away) merged 2026-10-01T13:12:46Z; workspace w52 kept open. Ask of the human: say what decides this task — are AC1-4 judged against PR 3 plus the later TASK-19/TASK-42 work, or is the w52 commit fetched? — or route it. Nudge only: I do not reopen, close, assign, or touch worktrees.

Stall sweep (fleet/TASK-17), pass 6, stall type 2 — human column, no note answering what happens next. The run of 2026-10-01 13:37 closed this Failed with only two fleet lines ("default_agent plugin-dev is not a fleet agent"; "the agent settled without reporting a verdict") — no note says whether it is to be re-run, narrowed or accepted, so the four ACs sit unread. Facts a human may want before choosing, gathered this sweep without deciding anything: AC#2 looks reachable today — history.jsonl now records what a run produced, the fields branch and pull_request are populated (38 and 22 of the 90 records since 2026-10-01); AC#3 looks reachable — `herdr-docket logs [-n <lines>]` exists on the CLI. AC#1 and AC#4 I did not test. Who it waits on: the human — re-run, narrow, or leave. Nudge only: I do not reopen, close, assign, or edit.

Pass 6 record correction: the preceding stall-sweep nudge was posted twice by one run — my verification grep pattern omitted ', docket board' and I re-posted believing the first had failed. Identical content, one nudge, no CLI verb removes a note and no task file was edited by hand; ignore the duplicate.

Cleanup 2026-10-03: obsolete — the board pane is gone (PR #14); history records branch, commits, PR and now the verifier's verdict (PR #16), and logs reads the daemon log.
<!-- SECTION:NOTES:END -->

## Comments

<!-- COMMENTS:BEGIN -->
created: 2026-09-16 15:50
---
Split by ADR 0005 (the board is a triage surface): the board half is TASK-27 (reads truthfully) and TASK-28 (acts); this task keeps what the board refuses - live output, branch and PR in history, the daemon log, reaching a human after a failure.
---
<!-- COMMENTS:END -->
