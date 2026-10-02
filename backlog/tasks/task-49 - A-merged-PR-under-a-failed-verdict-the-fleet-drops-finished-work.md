---
id: TASK-49
title: 'A merged PR under a failed verdict: the fleet drops finished work'
status: To Do
assignee:
  - pm
created_date: '2026-10-01 15:17'
updated_date: '2026-10-02 08:14'
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
<!-- SECTION:NOTES:END -->
