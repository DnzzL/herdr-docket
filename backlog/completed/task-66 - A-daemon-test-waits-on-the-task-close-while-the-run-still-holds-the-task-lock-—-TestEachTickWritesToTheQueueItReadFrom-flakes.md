---
id: TASK-66
title: >-
  A daemon test waits on the task close while the run still holds the task lock
  — TestEachTickWritesToTheQueueItReadFrom flakes
status: Done
assignee:
  - plugin-dev
created_date: '2026-10-04 17:32'
updated_date: '2026-10-28 15:25'
labels: []
dependencies: []
ordinal: 66000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The test internal/daemon/daemon_test.go TestEachTickWritesToTheQueueItReadFrom fails at exactly 2.00s on its second waitClosed, intermittently, under full-suite load — observed on origin/main as well as PR 24 (TASK-63 verification, paired go test -count=200 stress runs failed on both trees).\n\nMechanism (tag instrumentation: 33/33 failures were the second waitClosed): waitClosed (daemon_test.go:149) returns when reconcile closes the task mid-attempt, but Runner.Run still holds the task lock until it returns; evaluate #2 then hits the runs.Running check (internal/daemon/daemon.go:144), silently skips the task, no goroutine ever starts, and the second source never closes — the test times out. The window between src.Close and lock release grew a little with TASK-63 (history.TaskPullRequest inside notifyFailed), but the race itself is pre-existing.\n\nProduction is unaffected (daemon ticks are seconds apart; the next tick picks the task up).\n\nFix direction: have the test wait for the run to end (e.g. Runner.Busy() false or the run's error result), not for the task to be closed; or drop the in-process Running check's effect on an immediate second tick in the test harness.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Reproduced the red first (pre-fix): under 6 CPU burners, go test ./internal/daemon -run TestEachTickWritesToTheQueueItReadFrom -count=1000 fails at 2.00s on the second waitClosed (daemon_test.go:184), exactly the ticket's mechanism. Fix is test-only: new waitEnded helper waits for Runner.Running(id) to go false (task lock released, Run returned) before the second evaluate, so the in-flight check can't silently drop the task. Post-fix: same stress 1000x green (5.6s), full suite -count=20 under load green, plain go test ./... green, go build/vet/gofmt clean. No production code touched; no CHANGELOG entry (not user-visible).

Test-only fix in internal/daemon/daemon_test.go: waitEnded now waits for Runner.Running(TASK-1) to drop (Run returned, task lock released) before the second evaluate, so the in-flight check can no longer skip the task and strand the second queue. Red first, kept: pre-fix stress (go test -run TestEachTickWritesToTheQueueItReadFrom -count=1000 under 6 CPU burners) failed at 2.00s on the second waitClosed, daemon_test.go:184 — the ticket's exact mechanism. Green after: same 1000x stress passes, full suite go test ./... -count=20 under the same load passes, plain go test ./... passes, go build/vet ./... and gofmt -l . clean. No production code changed; CI build check passed on PR 27.

verdict PASS on https://github.com/DnzzL/herdr-docket/pull/27: Re-derived every criterion, ran it myself. (1) Pre-fix red reproduced on origin/main: under 6 CPU burners, go test ./internal/daemon -run TestEachTickWritesToTheQueueItReadFrom -count=1000 fails at 2.00s on the second waitClosed (daemon_test.go:184) — the ticket's exact symptom. (2) Mechanism proven live, not assumed: instrumented the runs.Running skip (internal/daemon/daemon.go:144) in a disposable base worktree; 85/85 failures were each immediately preceded by that skip, all at ~2.00s on line 184. The chain is runner.go:211 (task key held until Run returns) vs waitClosed firing at src.Close inside reconcile. (3) Diff is the ticket's own: 1 commit (e8b4aa9), test-only, internal/daemon/daemon_test.go +23 — waitEnded (daemon_test.go:169) polls Runner.Running(TASK-1) false (2s deadline, explicit fatal) before evaluate #2, so daemon.go:144 can no longer silently drop the task; both original source-isolation assertions intact (daemon_test.go:197,205), so the seam it guards is still guarded. (4) Post-fix on PR head: same 1000x stress green (8.1s, load avg 25); go test ./... -count=20 under the same load EXIT:0; go test -race ./internal/daemon -count=50 and target -count=200 clean; plain go test ./... green; go build ./... / go vet ./... clean, gofmt -l . empty. Mergeable, CI build SUCCESS. No production code touched; no CHANGELOG entry (CONTRIBUTING.md:58 requires one only for user-visible changes). Safety fact: the fix only delays evaluate #2 until the task lock drops — the exact state production reaches on its next tick.

fleet: https://github.com/DnzzL/herdr-docket/pull/27 is held for a human — internal/daemon/daemon_test.go is a CODEOWNERS path (/internal/daemon/).

fleet: https://github.com/DnzzL/herdr-docket/pull/27 is merged — the pull request settles this task done.
<!-- SECTION:NOTES:END -->
