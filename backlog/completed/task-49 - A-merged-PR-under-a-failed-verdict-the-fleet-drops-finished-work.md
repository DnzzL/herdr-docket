---
id: TASK-49
title: 'A merged PR under a failed verdict: the fleet drops finished work'
status: Done
assignee:
  - pm
created_date: '2026-10-01 15:17'
updated_date: '2026-10-03 17:57'
labels: []
dependencies: []
ordinal: 49000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Pattern filed by the demo lookback (docket/TASK-35), read straight from history.jsonl: 201 records, 65 runs, 2026-09-20 to 10-01. The prior-30 window is empty — history starts 20 Sep.

PATTERN — a run does the work, then never reports a verdict. Signature "the agent settled without reporting a verdict": 21 failed runs in six days (25 Sep x9, 26 Sep x8, 28 Sep x2, 30 Sep x1, 01 Oct x1) across four agents (dev x14, reviewer x4, pm x2, plugin-dev x1). Five had already opened their pull request. Forge, not inference (gh pr view): all five MERGED — TASK-30/PR 35 (10-01), TASK-99/PR 36 (26 Sep), TASK-101/PR 38 (10-01), TASK-22/PR 39 (28 Sep), TASK-16/PR 41 (29 Sep). The verdict said failed where the forge says shipped.

LIVE INSTANCE — dishnow/TASK-30 still sits Failed while its AC#2 fix is merged (PR 35, 10-01 08:00) and docket/TASK-125 closed Done on that same PR. TASK-16/22/99/101 were recovered by later runs; nothing systematic did it.

BELIEVED-FIXED JUNCTION — the reporting half is fixed by 48b6e27 (01 Oct 15:12: every prompt names the fleet CLI path, docket/TASK-42). First post-fix run, docket/TASK-45, closed done at 16:08. The junction stays partial: one day of post-fix evidence.

ASK (one holistic fix, pm judges — the lookback files, never reopens or closes):
1. Recover diverged tasks: where the latest run recorded a PR the forge says merged while the verdict says failed, comment the forge evidence on the task; pm or the human decides reopen/close. dishnow/TASK-30 first.
2. Make divergence countable: work-shipped-verdict-missing should be readable off the delivery record (history.jsonl branch/commits/PullRequest) so the sweep can surface it, comment-only.
3. Re-count this signature after a full post-fix window; docket/TASK-42 keeps the reporting-verb root cause (its second half — the unknown-assignee comment planting retired names — is still open).
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
fleet: docket/TASK-49 is docket work but pm works dishnow — reassign it to an agent of docket, or point pm at that checkout.

fleet: docket/TASK-49 is docket work but pm works dishnow — reassign it to an agent of docket, or point pm at that checkout.

fleet: docket/TASK-49 is docket work but pm works dishnow — reassign it to an agent of docket, or point pm at that checkout.

Day-0 evidence anchor for the Monday close (posted by docket/TASK-39's run, 2026-10-02): 26 'settled without reporting a verdict' records total in ~/.local/state/herdr/plugins/dnzzl.herdr-docket/history.jsonl; the last is 2026-10-01 13:37 (docket/TASK-24). The prompt fix landed 2026-10-01 15:12 (48b6e27, docket/TASK-42). Since then: zero settled-without-verdict records across 14 terminal runs on 2026-10-02 — one day, no recurrence; the 30-day counts and the close are the lookback's. One related instance stays open on the other half of the same seam: docket/TASK-46's reviewer run (2026-10-02 14:14) reached its verdict — review posted on PR 10, follow-up TASK-52 filed — and the run still recorded failed. Counts for your window; this note is the day-0 anchor, nothing closed.

fleet: docket/TASK-49 is docket work but pm works dishnow — reassign it to an agent of docket, or point pm at that checkout.

re-review datapoint (docket/TASK-55's run, 2026-10-02): one post-fix instance landed after the day-0 anchor was posted — docket/TASK-52's plugin-reviewer run of 18:52, failed with 'the agent settled without reporting a verdict' (118 s, 1 tick, no verdict line). That makes the book 27 records / 22 distinct runs against the anchor-time 26 / 21, and the 'zero recurrences in the day after' snapshot no longer holds as of tonight — re-count from scratch when closing, do not extend the anchor's numbers.

fleet: docket/TASK-49 is docket work but pm works dishnow — reassign it to an agent of docket, or point pm at that checkout.

fleet: docket/TASK-49 is docket work but pm works dishnow — reassign it to an agent of docket, or point pm at that checkout.

fleet: docket/TASK-49 is docket work but pm works dishnow — reassign it to an agent of docket, or point pm at that checkout.

fleet: docket/TASK-49 is docket work but pm works dishnow — reassign it to an agent of docket, or point pm at that checkout.

Closed by structure, 2026-10-03: in a queue with a verifier (dishnow, docket) a PR left on an open task is a delivery — the runner hands it to the verifier and never closes it Failed (ADR 0013, PR #15). Item 1: the one live instance, dishnow/TASK-30, carries the forge evidence as a note for a human. Item 2 is moot (the verdict and PR ride on history; 'herdr-docket history' shows both). Item 3 stays the lookback's job.
<!-- SECTION:NOTES:END -->
