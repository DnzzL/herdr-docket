---
id: TASK-53
title: The pick routes a Blocked task — a task parked on a human gets a run
status: Done
assignee:
  - plugin-dev
created_date: '2026-10-02 15:52'
updated_date: '2026-10-28 15:25'
labels: []
dependencies: []
ordinal: 53000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
pick.Next (internal/pick/pick.go) filters on it.Open, and Blocked is open: a task parked on a human question whose assignee exists is picked the moment the agent is free. Observed 2026-10-02 10:14 — fleet/TASK-1 (Blocked, reviewer) got a run that left no terminal record in history.jsonl and sits orphaned on the workspace list (docket/TASK-39 day-0 audit). Blocked means 'waits on a human'; routing it spends a run to read the same wall. Fix through the seam: pick.Next skips a task whose status is the queue's own Blocked word — the vocabulary differs per source, so the comparison goes through the fleet's words, not a literal. Seam question to settle in the same commit: work.Task currently exposes only Open, so the status word has to reach pick somehow (field, or the port gains a Blocked predicate). Test first in internal/pick: red on current Next (Blocked task picked), green after the skip — and every other open status still picked.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Picking up: reading the pick/work seams, then TDD the Blocked skip

Repro evidence from a live backlog, not code reading: the CLI's --status flag discards a file whose status is Blocked (the backlog CLI refused and the task vanished), so a Blocked backlogmd task is unreachable-in status only via manual writes; the fleet's own rejected word is 'needs human validation' — the same word dishnow's fleet.yaml maps Failed AND Blocked to. The remap bug is still at the pick seam: pick.Next only asks Open, and Blocked IS open in work.Task's terms — the task brief's red test (Blocked picked by Next) is the correct shape; the vocabulary whitelist is not a guard pick can lean on. Next: TDD the skip with a proper guard in internal/pick and the Blocked field through work.Task

PR open: https://github.com/DnzzL/herdr-docket/pull/13 — fix(pick): a task parked on a human is not picked. Seam: field on work.Task (Blocked bool), not a port predicate; adapters set it from the queue's own blocked word (github "Blocked" column, backlogmd vocabulary). pick skips parked; daemon's misroute scan no longer writes the reassign comment on parked work. Tests red then green at all three levels; build/vet/gofmt/test clean. Waiting on CI, then verdict.

Blocked was open, so a parked task picked a run the moment its agent freed up (fleet/TASK-1, 2026-10-02 10:14, reviewer). Fix: work.Task carries Blocked from the queue's own blocked word (github "Blocked" column, backlogmd vocabulary, binary backends leave it false like Verdict); pick.Next skips parked tasks; the daemon's misroute scan does not write the reassign comment onto a parked task, as it already does not for closed ones. CI green; awaiting review comment answers before handoff.

fleet: the run's worktree still holds uncommitted changes — workspace w7C (pane w7C:p1) is kept, and the task is now blocked — a human decides.

Stall sweep (fleet/TASK-21), pass 7, stall type 1 — new since pass 6: history.jsonl run docket/TASK-53-1790959806656211498 (plugin-dev, done 2026-10-02 19:39, verdict blocked) flags uncommitted=true with commits:1 and PR https://github.com/DnzzL/herdr-docket/pull/13 — the first new uncommitted=true record since 2026-09-29 (8 distinct flagged runs of 312 now). Your own closing line says workspace w7C (pane w7C:p1) is kept and 'a human decides'. No open task follows any of it: live open queue is fleet/TASK-1/2/9/11/13/16/20 and docket/TASK-49/50, and grep of those files finds no mention of PR #13. gh this sweep: PR #13 OPEN at head 80dab34, zero reviews. Who it waits on: the human — review/merge or abandon PR #13, and say keep/discard for w7C's uncommitted remainder. Nudge only: I do not reopen, close, assign, or file work of my own.

Re-queued 2026-10-03 (cleanup): PR #13 (https://github.com/DnzzL/herdr-docket/pull/13) is still open from before the pipeline (ADR 0013). Rebase it onto main (the board, budgets and pm are gone; prompt and runner changed in #14/#15), make it green, then deliver it with task done --pr. If main already does what it does, close the PR and say so in a block note instead.

Rebased PR #13 onto main (board/budgets/pm gone, verifier+gate in): one conflict in CHANGELOG (kept both sides), a duplicated-comment typo in work.go fixed. Replayed the red: reverted the pick skip -> TestBlockedIsNotPicked fails; restored -> passes. go build/vet/test/gofmt all clean on the rebased tree. Force-pushed to the PR head; waiting on CI.

Rebased PR #13 onto main (726e8c7), force-pushed, CI green. Verified myself: (1) red replay — reverted the pick skip, TestBlockedIsNotPicked fails with the blocked task picked; restored, passes; (2) every other open status still routes (pick suite green); (3) adapters set Blocked only from each queue's own word (backlogmd vocabulary test, github Blocked-column test); (4) daemon leaves parked tasks un-commented tick after tick; (5) go build ./... / go vet ./... / gofmt -l . / go test ./... all clean on the rebased tree. CHANGELOG conflict resolved by keeping both entries; fixed a duplicated-comment line in work.go.

verdict PASS on https://github.com/DnzzL/herdr-docket/pull/13: Verified on head 726e8c7 in my own disposable worktree, never the queue checkout. Bar: go build/vet/test ./... and gofmt -l all clean; CI build pass; merge-tree vs main 570ded5 clean (rc 0, CHANGELOG auto-merges). ACs re-derived by replay, not the PR's claims: (1) seam = work.Task.Blocked field (work.go:42), pick.Next skips it with no status literal (pick.go:42); (2) red replay: reverted pick.go:42 to it.Open, TestBlockedIsNotPicked failed with the blocked task picked (pick_test.go:282), restored, green — and the full 15-test pick suite stays green, so every other open status still routes (TestPicksHighestPriorityOpenTaskWithKnownAssignee, TestWorkLeftMidFlightIsStillPickedUp, TestUnassignedGoesToDefaultAgentWhenConfigured); (3) every adapter sets Blocked from its own queue's word: backlogmd via the configured vocabulary (backlogmd.go:256; backlogmd_test.go:97 maps the non-literal word 'needs human validation' — red when backlogmd.go:208 is removed), github from the board's Blocked column (github.go:178 — pre-fix state open=true blocked=false reproduced by removing the line, github_test.go:142 red), basecamp leaves it false; (4) daemon change red-before too: revert daemon.go:153 and the reassign comment lands on the parked task twice (daemon_test.go:296 fails), restored, green. One commit, 10 files, all the ticket's own. Correction for the record (does not change the verdict): the grounding observation fleet/TASK-1 2026-10-02 10:14 was test-suite pollution, not the daemon routing a parked task — the history record is the only one of 405 with fake workspace ws/pane p (fakeHost, daemon_test.go:117), its agent reviewer contradicts the real assignee intake (file at fleet commit a0f5535, updated 10-01 11:10), the never-rotated daemon.log has zero TASK-1 start lines, and the run-root--w-dishnow.lock mtime 10:14 matches the fixture added in 81a47b8 committed 10:14:42 — 10s after the record; the running daemon (PID 3897393 since Sep 25, d735a53 code) could not have picked it anyway: backlogmd open() has excluded Blocked since 4813f7d (backlogmd.go:245), live-checked against the real backlog CLI which reports status Blocked verbatim. So the fix is load-bearing for github boards (issue-open + Blocked column) and belt-and-braces for this fleet's three backlogmd queues. Non-blocking follow-up for plugin-dev: github.go:40 hardcodes the parked column name while in_progress is configurable — a board whose parked column is not spelled Blocked cannot be skipped.

fleet: https://github.com/DnzzL/herdr-docket/pull/13 is held for a human — internal/daemon/daemon.go is a CODEOWNERS path (/internal/daemon/).

fleet: https://github.com/DnzzL/herdr-docket/pull/13 is merged — the pull request settles this task done.
<!-- SECTION:NOTES:END -->
