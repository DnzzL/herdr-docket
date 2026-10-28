---
id: TASK-74
title: >-
  The daemon's settle call is untested — the wiring can vanish under a green
  suite
status: Done
assignee:
  - plugin-dev
created_date: '2026-10-13 13:46'
updated_date: '2026-10-14 09:54'
labels: []
dependencies: []
ordinal: 74000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Follow-up from TASK-70's verification (PR #42). internal/daemon/daemon.go:146 spawns go runs.Settle(src, tasks) each tick — the only production entry to the settle pass. runner.Settle is tested through the seam (internal/runner/settle_test.go, red-before at 9993758) and proven live, but no test asserts that evaluate invokes it: delete that line and the whole suite stays green while the queue goes back to lying, which is TASK-70's own bug (the HasCode pattern). A daemon-level test is blocked today because Runner.forge is package-private (internal/runner/runner.go:53) — daemon tests cannot answer a fake PR state, so a blocked task + history PR record would hit the real gh. Fix shape: a forge injection seam on runner.Runner (constructor option or exported test setter), then a daemon test: held task in memSource + history.Append with a PR + fake forge MERGED + evaluate -> task closed done.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
run start: read daemon.go evaluate (the go runs.Settle line), runner.go forge field, settle_test.go, daemon_test.go harness. Picking the constructor-option shape for the seam (WithForge) over an exported setter: injection at construction can't race a live run, setter could be called mid-run. Daemon test: held task in memSource + history PR record + fake forge MERGED via runner.WithForge, evaluate, assert closed done + forge answered. Test first, then red-by-deleting-the-settle-line as evidence.

Verdict on the fix shape, criterion by criterion (PR #46).
1. Forge injection seam on runner.Runner — MET. Chose the constructor-option shape offered by the task: runner.WithForge(gate.Forge), runner.New now variadic (internal/runner/runner.go). Construction-time on purpose: run and settle goroutines read r.forge, an exported setter on a live Runner would race them. Both production callers (daemon.go:64, main.go:709) pass no option, so the forge stays gate.GH{} — behaviour unchanged outside tests.
2. Daemon test: held task in memSource + history.Append with PR + fake forge MERGED + evaluate -> task closed done — MET. TestATickSettlesAHeldTaskWhosePullRequestIsMerged (internal/daemon/daemon_test.go): fleet/TASK-1 Open+Blocked, PR record in a temp state dir, settleForge answering MERGED through runner.WithForge, one evaluate; asserts verdict done and exactly 1 forge read (proof it answered through the seam, never gh). Blocked task is skipped by pick, so no run/comment can produce the close — only the settle wiring can.
3. The suite could not see the wiring deleted — MET, red-before evidence: removed the go runs.Settle(src, tasks) line from daemon.go -> the new test FAILS with 'no run closed the task' (the rest of the suite was green with that line gone, which was the bug); restored byte-identical (empty git diff).
4. Bar — MET, run once each on the final tree: go build ./... ok, go vet ./... ok, go test ./... all packages ok, gofmt -l . empty. Also go test -race on the new test + the settle/blocked neighbours: ok. CI: PR #46 build job passed.
Not done/considered: no changelog.d fragment (no user-visible change — test + test-only seam), no ADR (ADR 0013 already owns the swappable forge; this only makes it reachable from another package).

Forge injection seam (runner.WithForge, constructor option — no setter race with run/settle goroutines) plus the daemon-level test the task asked for: held task in memSource + PR in history + fake forge MERGED + one evaluate -> closed done, forge read exactly once through the seam (never gh). Red-before proven: deleting the go runs.Settle line from evaluate makes the new test fail ('no run closed the task'), restored byte-identical. Bar run once each on the final tree: go build ./... ok, go vet ./... ok, go test ./... all green, gofmt -l . empty; go test -race on the new test green; CI build job on PR #46 passed. No changelog fragment (not user-visible), no new ADR (0013 already owns the swappable forge).

verdict PASS on https://github.com/DnzzL/herdr-docket/pull/46: All 4 criteria re-derived on head 15b8193. (1) Seam: runner.WithForge constructor option, runner.New variadic (internal/runner/runner.go:63-94); production callers daemon.go:64 and main.go:709 pass no option, forge stays gate.GH{} — behaviour unchanged outside tests. (2) Test TestATickSettlesAHeldTaskWhosePullRequestIsMerged (internal/daemon/daemon_test.go:210): Blocked task in memSource + history PR in temp state dir + fake forge MERGED through WithForge + one evaluate -> asserts verdict done AND forge.reads==1; pick skips Blocked (daemon.go:179) and pipeline Close(done) (pipeline.go:122) is unreachable for a blocked task, so only the settle wiring can close it, and the single forge read proves gh was never touched. (3) Red-before: deleted go runs.Settle(src, tasks) (daemon.go:146) -> new test FAILS 'no run closed the task' (daemon_test.go:233) while the rest of go test ./... stays green (the TASK-70/HasCode bug); restored byte-identical (git diff HEAD empty, git status clean). (4) Bar on the final tree: go build ./... ok, go vet ./... ok, gofmt -l . empty, go test ./... all packages ok (re-run after restore), go test -race on the new test green. Scope: single commit, test + test-only seam only.

fleet: https://github.com/DnzzL/herdr-docket/pull/46 is held for a human — the pull request is merged, not open.

fleet: https://github.com/DnzzL/herdr-docket/pull/46 is merged — the pull request settles this task done.
<!-- SECTION:NOTES:END -->
