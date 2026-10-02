---
id: TASK-35
title: Lookback files holistic follow-ups for recurring failure
status: Done
assignee: []
created_date: '2026-09-21 19:12'
updated_date: '2026-10-01 15:18'
labels: []
dependencies:
  - TASK-24
priority: high
ordinal: 35000
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

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
fleet: default_agent "plugin-dev" (fleet.yaml) is not a fleet agent.

Reconciliation 2026-10-01 — NOT closed. AC#1, #3 and #4 are met by d735a53 (the lookback persona states its window, its recurrence patterns and the believed-fixed junction; the weekly entry drives it and states the files-only refusal). AC#2 is NOT: it asks for a demo lookback over a real history that finds a recurrence and files a follow-up to pm. lookback has run ZERO times — history.jsonl holds no run for it at all, and its budget reads 0/1. Met the first Monday it runs and files.

AC#2 executed as the demo lookback. Read history.jsonl directly at ~/.local/state/herdr/plugins/dnzzl.herdr-docket/history.jsonl (201 records, 65 runs, 2026-09-20 -> 10-01) — never the CLI rendering. Found one pattern: the agent settled without reporting a verdict, 21 failed runs over six days, four agents (dev x14, reviewer x4, pm x2, plugin-dev x1); five of them had already recorded a PR. Forge check per AC#3, gh pr view on all five (DishNow-v2 35/36/38/39/41): every one MERGED, so the verdict lied in the work's favour; dishnow/TASK-30 still sits Failed today with its AC#2 fix merged, the live instance. Believed-fixed junction: the reporting fix 48b6e27 landed 01 Oct 15:12 (docket/TASK-42), first post-fix run docket/TASK-45 closed done at 16:08 — junction stated as partial in the follow-up. Filed ONE follow-up for the ONE pattern, assigned pm, through the CLI: docket/TASK-49. No task reopened, no verdict rewritten, no board hand-editing — queue operations only, per the persona's refusal.

Verdicts: AC#1 MET (d735a53: docs/examples.md section 7 states the 30-and-prior-30 window, the recurrence patterns, the one-follow-up-per-pattern rule — verified by reading the shipped persona in the fleet dir, agents/lookback/AGENT.md, identical bytes). AC#2 MET this run: demo lookback executed by this run, history.jsonl read directly (65 runs), one pattern found and one follow-up filed through the CLI assigned to pm — docket/TASK-49, evidence carried: task ids, run counts per day, agent counts, forge merge states from gh pr view. AC#3 MET in the demo: merge state read from the forge for all five PR-carrying failed runs, never inferred; where the record shows the fix landed after a failure (48b6e27 15:12 vs the last settled run 13:37) the follow-up says exactly that instead of implying the fix failed. AC#4 MET (d735a53: automations.yaml weekly entry states the files-only refusal — verified in the shipped persona). How I know: every number above comes out of history.jsonl parsed directly, every merge state out of gh pr view live this run, the follow-up id out of the CLI. Not verified: whether pm picks docket-queue tasks (pm works DishNow-v2 — if it cannot see this queue the follow-up needs a human nudge); the believed fix needs a full post-fix window to confirm, docket/TASK-49 asks for the re-count. No PR: this slice is queue evidence, not code; AC#1/#3/#4 landed with d735a53 previously.
<!-- SECTION:NOTES:END -->

## Comments

<!-- COMMENTS:BEGIN -->
created: 2026-09-21 19:12
---
Blocked by TASK-24 §2: the believed-fixed check is only as good as the delivery record it reads. The fleet does not read dependencies, so the statement is on the ticket, not enforced — do not start this one before TASK-24 closes.
---
<!-- COMMENTS:END -->
