---
id: TASK-70
title: A task left short settles when its pull request does
status: Done
assignee: []
created_date: '2026-10-13 12:06'
updated_date: '2026-10-28 15:28'
labels: []
dependencies: []
ordinal: 70000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
A held task stays Blocked forever after the human merges or closes its PR: TASK-48, 53, 57, 62, 63, 66 and 69 are Blocked although PRs #12, #13, #19, #23, #24, #27, #31 are merged. The same goes for a task closed failed whose PR merged anyway: TASK-22, 65, 67 and 68 sit in 'needs human validation' with PRs #9, #29, #28, #30 merged (the pattern TASK-49 named). The queue lies and the human's columns fill with noise.
Done when: every few minutes (not every tick) the fleet reads, through the forge, the PR of each task that is Blocked or failed and has one; MERGED closes the task done; a held task whose PR is CLOSED unmerged closes failed with a note saying a human closed it; OPEN leaves it alone. The fleet never merges here. CONTEXT.md's Hold entry says a hold waits on its PR and settles with it.
Verify by: a runner test through the seam for MERGED, CLOSED and OPEN, Blocked and failed; and live, the eleven tasks above close done on the first pass.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A held task whose PR is merged closes done
- [ ] #2 A held task whose PR is closed unmerged closes failed, saying a human closed it
- [ ] #3 An open PR leaves its task Blocked
- [ ] #4 The PR state is read every few minutes, not every tick
- [ ] #5 CONTEXT.md's Hold says a hold settles with its PR
- [ ] #6 A failed task whose PR is merged closes done
<!-- AC:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
This run found the feature already delivered and merged as PR #42 (d62ebd4); the gap was live, not code: the eleven named tasks stayed Blocked/failed after merges, then the running daemon's settle pass closed them at 15:25-15:26 today with the note 'fleet: <PR> is merged — the pull request settles this task done.' Current board, read back through the CLI: TASK-48/53/57/62/63/66/69/22/65/67/68 all Done. Settle tests on main pass (go test ./internal/runner/ -run ... -v), full bar clean (go build ./..., go vet ./..., go test ./..., gofmt -l . empty). No new code was needed for this slice; the merged PR #42 carries it.

Delivered already: the settle pass shipped as PR https://github.com/DnzzL/herdr-docket/pull/42 (d62ebd4), and this run verified it rather than rewriting it. Verdict on each criterion, all met:
#1 MET — TestAHeldTaskWhosePullRequestIsMergedClosesDone passes through the runner seam; live, the eleven tasks now read Done with the fleet's 'is merged — the pull request settles this task done' note.
#2 MET — TestAHeldTaskWhosePullRequestIsClosedUnmergedClosesFailedSayingAHumanClosedIt passes; the note says 'closed unmerged by a human'.
#3 MET — TestAnOpenPullRequestLeavesItsHeldTaskBlocked passes; open PR writes no note and closes nothing.
#4 MET — TestThePullRequestIsReadEveryFewMinutesAndNotEveryTick passes: 8 fifteen-second ticks = 1 forge read, 4 more minutes = 2.
#5 MET — CONTEXT.md:154-158 says a hold waits on its pull request and settles with it (merged done, closed unmerged failed, open stands).
#6 MET — TestAFailedTaskWhosePullRequestIsMergedClosesDone passes; live, previously-failed TASK-22/65/67/68 are Done.
Bar: go build ./..., go vet ./..., go test ./... all pass; gofmt -l . empty. No new code change was required — main already carries the feature; the live pass at 15:25-15:26 closed TASK-48/53/57/62/63/66/69/22/65/67/68 exactly as required. No fresh PR: nothing to change, and #42 is already merged.
<!-- SECTION:NOTES:END -->
