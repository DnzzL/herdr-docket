---
id: TASK-73
title: A run's lock files outlive the run
status: Done
assignee: []
created_date: '2026-10-13 12:21'
updated_date: '2026-10-28 15:43'
labels: []
dependencies: []
ordinal: 73000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The state dir holds 76 run-taskid-*.lock files, one per task ever run (dishnow, docket, fleet), never removed. Harmless to correctness as far as known (not verified), but the dir grows with every task forever.
Done when: a task's lock file is removed once its run settles and nothing holds it, without opening a window where two runs can take the same task (ADR 0018); the existing files are swept once.
Verify by: a runner test that a settled run leaves no lock file and that a second run on the same task still waits while the first holds it; and live, the state dir carries no lock for a closed task.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A settled run leaves no lock file for its task
- [ ] #2 Two runs on one task still exclude each other
- [ ] #3 Locks left by earlier runs are swept
<!-- AC:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
#45 landed the lock removal + sweep; the remaining hole was in drop, and this run closes it. drop unlinked the lock file by path, so the sweep's open-to-flock gap could flock a dead inode after the path was reclaimed and remove the next run's live claim — two runs on one task. drop now unlinks only while its descriptor is still the file at the path (same dev+inode check claim makes); ADR 0020 records the window. Verified: go build/vet/gofmt clean, go test ./... green; the new sweep-race test fails on the unguarded drop and passes with it; live state dir has 4 run-*.lock files, each refused by flock -n (all held by live runs), and no lock file for any closed task (104, fleet/TASK-1, myapp).

Closes the sweep race left in #45: drop unlinked by path, so a sweep resuming after a run settled could remove the next run's live claim. drop now unlinks only while its descriptor is still the file at the path; ADR 0020 records the window. Evidence: go build/vet/gofmt clean, go test ./... green; TestASweepPausedBeforeItsFlockCannotRemoveTheNextRunsClaim fails without the guard, passes with it; live state dir has 4 lock files all held by live runs (flock -n refuses) and none for a closed task. Criteria: #1 met (TestASettledRunLeavesNoLockFileBehind), #2 met (mid-run refusal test + guard), #3 met (SweepLocks at daemon start, tested; no leftover on disk).

verdict PASS on https://github.com/DnzzL/herdr-docket/pull/67: Bar on head 6b8f1c4: go build ./..., go vet ./..., gofmt -l . (empty), go test ./... all green. AC#1 met: TestASettledRunLeavesNoLockFileBehind (runner_test.go:305) runs a real Run mid-flight, stats run-taskid-task-1.lock live, settles, then globs run-*.lock = 0. AC#2 met: same test refuses a second Run mid-run and asserts no phase/note written; new TestASweepPausedBeforeItsFlockCannotRemoveTheNextRunsClaim (runner_test.go:383) replays the sweep window with real claim/drop (opens descriptor, drop A, claim B, sweep flocks dead inode, drop) and asserts B's file survives and a third claim is refused (errBusy). AC#3 met: SweepLocks called at daemon start (internal/daemon/daemon.go:62), TestSweepLocksTakesOnlyWhatNoRunHolds proves stale removed / held kept; live state dir /home/thomas/.local/state/herdr/plugins/dnzzl.herdr-docket has 4 run-*.lock, each refused by flock -n (all held by live runs: agent-plugin-dev, root-..., taskid-docket-task-73, taskid-docket-task-89), and closed tasks TASK-23/TASK-32 have none. Risk-area check: I reverted the guard in a throwaway copy (/tmp/v73) and the new sweep-race test fails with 'run B's claim must still stand on disk: ... no such file', passes with the guard — the test would have been red before the fix and drives real code, not a fake. Guard is correct: drop's sameFile (runner.go:265) mirrors claim's dev+inode check, sweep cannot flock while a live run holds the flock, so no TOCTOU onto a reclaimed path. Scope is the ticket's own 3 files, no unrelated edits; ADR 0020 records the window and guard. Lock tests stable at -count=10.

fleet: https://github.com/DnzzL/herdr-docket/pull/67 is held for a human — internal/runner/runner.go is a CODEOWNERS path (/internal/runner/).

Delivered by ADR 0020: internal/runner/runner.go drop() unlinks the lock under the flock; the daemon sweep takes leftovers.
<!-- SECTION:NOTES:END -->
