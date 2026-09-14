---
id: TASK-24
title: You cannot tell what the fleet is doing without leaving the fleet
status: To Do
assignee: []
created_date: '2026-09-13 18:14'
labels: []
dependencies: []
priority: high
ordinal: 24000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Everything the fleet knows about a run is reachable — and almost none of it is reachable from the fleet's own tools. A whole evening of watching it work, and every question needed a different outside command:

| the question | what it took tonight |
| --- | --- |
| what is the agent doing right now? | `herdr agent list` to discover a generated name (`dishnow-task-36-audit-the-tlbau2`), then `herdr agent read <that>` |
| did the run produce anything? | `git log main..<branch>` in the project repo |
| is there a PR? | `gh pr list` — and the PR url only existed inside the closing note |
| why did it fail? | `tail /tmp/fleet-daemon.log`, a path nothing documents |
| what did the agent actually say? | `backlog task view <id> --plain`, reading past the description to the notes |

The fleet generated that agent name. It provisioned that workspace. It recorded that run. It has every one of those facts and surfaces none of them.

## What the surfaces show today

- `herdr-docket list`: id, truncated title, routed agent. Nothing about runs at all.
- `herdr-docket history`: timestamp, status, task, trigger, duration, and an error string. No branch, no PR, no workspace, no note.
- The board pane: two views, tasks and agents, `j/k move · r run · enter jump to run · a add · / search · g agents · q quit`. `enter` jumps into a workspace — which is the one good answer already there, and it only works if you already know which row is running.
- The daemon log goes to stdout, captured to a file by whoever started it. `herdr-docket` has no way to read its own daemon's log.

## What "better" means here, concretely

Not a redesign. The gap is that a run is invisible while it is the only interesting thing happening. Some of what would close it:

- A running task should be obviously running, with elapsed time against the agent's timeout — the board is where you would look, and it does not say.
- The agent's live output should be one keystroke from the row, not a name you have to go find. The fleet named that agent; it can hand it to `herdr agent read`.
- `history` should carry what a run produced: the branch, whether it committed, a PR url if one was opened. Today the PR url exists only in prose inside a task note.
- A finished run should be able to reach the human who was not watching. Herdr has a notification surface (herdr-focus-notify is installed on this machine); a run ending `failed` at 3am is exactly the case the fleet exists for and exactly the case nobody sees.
- The daemon's own log should be readable from the CLI. A daemon whose diagnostics live at a path the user has to guess is a daemon that debugs badly.

## The judgement call this needs first

Decide what the board pane IS before adding to it. Right now it is a task list that can start a run. It could be that, or it could be the fleet's dashboard — what is running, for how long, what it produced, what needs a human. Those are different products and the second one is what tonight kept needing. Pick one, and say so in the ADR, because half of each is worse than either.

Some of this is Herdr's to give rather than the fleet's. Where the answer is "Herdr should expose this", say so and stop — a wrapper that reimplements its host is the failure mode on the other side.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 From the board alone, a human can see that a run is in flight, which agent has it, and how long it has been running against its timeout
- [ ] #2 The agent's live output is reachable from the fleet's own surfaces without discovering a generated name by hand
- [ ] #3 herdr-docket history records what a run produced — at minimum its branch and any PR — rather than leaving it in prose inside a task note
- [ ] #4 An ADR states what the board pane is for, and the additions are the ones that decision implies
<!-- AC:END -->
