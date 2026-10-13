---
id: TASK-62
title: >-
  A run that times out keeps its delivery — a late PR or verdict still reaches
  the pipeline
status: Blocked
assignee: []
created_date: '2026-10-04 15:52'
updated_date: '2026-10-04 16:48'
labels:
  - bug
dependencies: []
priority: high
ordinal: 62000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Symptom (dishnow, 2026-10-04): 9 runs ended 'failed: timed out waiting for agent status', each at exactly its agent's timeout_minutes (dev 5408s/90min, reviewer 2708s/45min). The timeout closes the task Failed but never stops the agent: it keeps working, then opens its PR and writes its verdict after the fleet stopped listening. Nothing picks that late delivery up, so the verifier never runs on a dev PR and the gate never runs on a reviewer PASS. Evidence: history.jsonl run dishnow/TASK-143-1791112799559477566 is both status:failed and verification:PASS; TASK-145's dev pane kept working 1h40 past its timeout (pi session 11:20Z→14:29Z) and delivered PR #61; 5 PRs (DishNow #46 #47 #59 #60 #61) never got a verifier, 2 PASS (#57 #58) were merged by hand. The timeout must stop being a silent drop: either the run keeps the agent it started and resumes the pipeline when that agent settles, or it ends the agent so nothing is delivered after the verdict. Pick one and record it in an ADR.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A dev run that passes its timeout and whose agent later settles with a PR and a done verdict goes to the verifier, exactly as an on-time run
- [ ] #2 A verify run that passes its timeout and whose agent later records PASS goes to the gate
- [ ] #3 A run whose agent never settles still ends failed and notifies, and no agent keeps working on a task the fleet has closed
- [ ] #4 history.jsonl tells a late delivery from an on-time one (the timeout is recorded, not erased)
- [ ] #5 A test drives the timeout path through the runner's seam with a fake host whose agent settles after the deadline
<!-- AC:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Picked the keep-the-agent option (the ticket's first): AC#1/#2 require late PRs/verdicts to reach verifier and gate, so ending the agent at the deadline is ruled out. Plan: host.ErrTimedOut sentinel + host.Settle (listen past the deadline); runner records history timed_out, listens one more window = 2x timeout (TASK-145 settled 99min past a 90min deadline — one window would still drop it), and on no-settle: task Failed + notify + close the workspace so nothing works on a closed task. ADR 0014 records it. Writing the seam tests first.

Picked option one — the run keeps the agent and resumes the pipeline; recorded in ADR 0014 (option two, ending the agent at the deadline, rejected: it destroys the very PRs the fleet exists to collect). Seam: host.Do answers ErrTimedOut at the deadline, host.Settle listens, the runner records timed_out and listens 2x the timeout more (TASK-145 settled 99min past a 90min deadline — one window would still drop it); never-settled → task Failed + notified + workspace closed so nothing works on a closed task, uncommitted loss said on record/task/notification. Criteria: #1 met — TestARunThatPassesItsDeadlineAndSettlesLateStillReachesTheVerifier: late dev PR goes through verifier to merge, exactly the on-time path; #2 met — TestAVerifyRunThatPassesItsDeadlineAndVerifiesLateStillReachesTheGate: late PASS reaches the gate; #3 met — TestARunWhoseAgentNeverSettlesEndsFailedNotifiesAndEndsTheAgent: exactly one window, Failed, one notification, workspace closed (and a Settle-cancelled run stays Blocked, quiet); #4 met — history.jsonl carries a status:timed_out record at the deadline and timed_out:true on the closing record while an on-time run sits unmarked beside it, and the real binary (built to /tmp) prints 'timed-out' in herdr-docket history; #5 met — the fake host's agent settles after the deadline through the runner's seam, red committed first (0f5bf51), green after the fix (308002f). Verified: go build/vet/test ./... + gofmt -l . all clean locally, CI build pass on PR #23. Not run against a live herdr agent — the seam is the surface the task names.

verdict PASS on https://github.com/DnzzL/herdr-docket/pull/23: Verified on a disposable worktree of pr-23-verify (a762444): go build/vet/test ./... and gofmt -l . all clean locally; CI build job (same four gates) pass. AC#1 met — runner_test.go:804 TestARunThatPassesItsDeadlineAndSettlesLateStillReachesTheVerifier drives the seam (fake do answers ErrTimedOut at the deadline, settle stamps the PR after it), asserts the late PR goes through verifier and gate to merged, late run done carrying timed_out, on-time verifier unmarked. AC#2 met — runner_test.go:863: verify run past deadline, PASS recorded in Settle, reaches the gate and merges; verify record done+timed_out, worker unmarked. AC#3 met — runner_test.go:905: never-settled ends Failed with exactly one 2x window (settles=1, runner.go:394 lateWindow), one notification saying never settled, workspace closed (runner.go:326 Close before reconcile closes the task) plus task note; runner_test.go:945: Settle-cancelled stays Blocked and quiet. AC#4 met — history_test.go:232 pins timed_out as a mark not an ending and the on-wire flag; AC#1 test greps history.jsonl for "status":"timed_out" and the collapsed closing record carries timed_out; I built the real binary to /tmp and ran herdr-docket history on a synthetic log: the late run line prints timed-out, the on-time line does not. AC#5 met — red-first reproduced myself: checkout of 0f5bf51 (test commit) fails exactly those 5 tests (4 runner + main_test.go:214 history line), green after 308002f. Scope clean: exactly 3 commits / 14 files vs origin/main, all ticket own, no drive-by work; ADR 0014 records the pick with the rejected alternative. Not run against a live herdr agent (the seam is the surface the task names, as the author also stated). Non-blocking observation for a follow-up: a PR/verdict stamped before the deadline is hidden from latest-record readers while the timed_out record is latest — self-heals, the closing record re-reads scan-all (PullRequestFor/VerificationFor).

fleet: https://github.com/DnzzL/herdr-docket/pull/23 is held for a human — internal/daemon/daemon_test.go is a CODEOWNERS path (/internal/daemon/).
<!-- SECTION:NOTES:END -->
