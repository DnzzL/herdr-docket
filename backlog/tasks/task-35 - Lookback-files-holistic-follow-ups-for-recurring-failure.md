---
id: TASK-35
title: >-
  Lookback files holistic follow-ups for recurring failure
status: To Do
assignee: []
created_date: '2026-09-21 19:12'
labels: []
dependencies:
  - TASK-24
ordinal: 35000
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The loop's last stage is the one that makes it self-improving, and the only one that needs the fleet's own records: a weekly scan over `history.jsonl` and the queue that asks what keeps coming back. The same complaint reported after a believed fix landed, tasks reopened days after a clean run, brittleness a patch loop keeps repricing — the video runs this weekly against a 30-and-prior-30 window and files one holistic fix per pattern instead of letting the pile re-derive it ticket by ticket.

The delivery record TASK-24 §2 makes is what turns this from sentiment to evidence: a "learned it was fixed, does it still hurt" junction has to be able to read, in one place, whether the run that claimed the fix produced a branch and commits and where the PR is. Before that record exists the persona can still run — recurrence in the queue's own words, reopened work, repeated verdicts — but the believed-fixed check stays partial and the sweep files fewer, shallower follow-ups.

The persona does not read history as prose: it reads the state file directly (`history.jsonl`, schema in internal/history). It files one follow-up per pattern assigned to `pm` — triage first, nothing skips intake — and it never reopens or rewrites a closed task; a believed-fixed that comes back gets re-opened as a comment on the original plus a new task, when the shape allows it.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A persona in docs/examples.md states the lookback method: the window it reads, the recurrence patterns it looks for, and the one-follow-up-per-pattern rule
- [ ] #2 A demo lookback over a real history finds at least one recurrence and files its follow-up assigned to pm, reading history.jsonl directly rather than the human CLI output
- [ ] #3 The believed-fixed junction is honest about its evidence: where the delivery record says the fix never shipped, the follow-up says that rather than implying the fix failed — and merge state is read from the forge (`gh pr view`), never inferred from a PR url or a verdict alone
- [ ] #4 One automations.yaml entry drives it weekly, and the entry states the plain scope refusal: the sweep files tasks and comments, never reopens or closes them
<!-- AC:END -->

## Comments

<!-- COMMENTS:BEGIN -->
created: 2026-09-21 19:12
---
Blocked by TASK-24 §2: the believed-fixed check is only as good as the delivery record it reads. The fleet does not read dependencies, so the statement is on the ticket, not enforced — do not start this one before TASK-24 closes.
---
<!-- COMMENTS:END -->
