---
id: TASK-63
title: >-
  A merge the human owes is announced as one — herdr popup and a merge-needed
  label
status: Blocked
assignee: []
created_date: '2026-10-04 15:52'
updated_date: '2026-10-04 17:33'
labels:
  - enhancement
dependencies: []
priority: high
ordinal: 63000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Today a PR that needs the human reaches them as a desktop popup titled 'fleet: run failed — <task>' or 'fleet: held — <task>', with no sound and no PR link in the title, and nothing on the PR itself. GitHub cannot ping the human either: the fleet opens PRs under the human's own account, and GitHub never notifies you of your own actions. Wanted: every time the fleet stops with an open PR that only a human can move (gate hold — critical label, CODEOWNERS path, merge not auto, refused merge — a verifier FAIL past maxRounds, or a failed run that left a PR open) it raises one popup that says so and marks the PR so it can be found later.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Each such stop raises one herdr popup titled 'merge needed — <repo>#<number>' with the reason as body and the 'request' sound
- [ ] #2 The same stop adds the label merge-needed to the PR (created in the repo if missing); a PR the gate merges never carries it
- [ ] #3 A failed run with no PR keeps today's 'run failed' popup and gets no label
- [ ] #4 Tests through the runner seam cover a CODEOWNERS hold, a FAIL past maxRounds and a failed run with a PR
<!-- AC:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Starting: reading the runner/gate/host seams. Plan — one popup 'merge needed — <repo>#<number>' (reason body, request sound) plus merge-needed label at every human-only stop (hold, FAIL past maxRounds, failed run with a PR); gate clears the label when it merges; failed run without PR keeps the run-failed popup. Port change: host.Notify gains a sound arg; gate.Forge gains AddLabel/RemoveLabel. Tests first through the runner seam, then ADR+CHANGELOG.

All four criteria met. #1: every human-only stop (gate hold — critical/CODEOWNERS/merge never/refused merge —, FAIL past maxRounds, failed run with a PR) now routes through one runner helper that raises exactly one popup titled 'merge needed — <repo>#<number>' with the reason as body and the request sound; the old held popup is that stop now (tests: TestACodeownersPathIsHeld, TestTwoFailsBlockTheTaskForAHuman, TestAFailedRunWithAPRAnnouncesTheMergeTheHumanOwes). #2: the same helper adds label merge-needed via gate.Forge.AddLabel (gh label create best-effort then pr edit --add-label, so created in the repo if missing), and the gate removes it as it merges so a merged PR never carries it (TestTheGateClearsTheLabelOffAPRItMerges, TestAPassOnAGreenPRMergesAndClosesDone asserts no label ever added). #3: TestAFailedRunReachesAHumanNotWatching asserts the run-failed title, SoundNone and zero labels. #4: those tests run through the runner seam with fake host/board/forge. Verified myself: go build/vet/test + gofmt-l all clean locally, CI build job green on PR 24, herdr notification show with --sound request answered shown:true, gh label list and gh pr view --json labels read probes match the adapter parser. Not replayed live: an end-to-end daemon hold and the actual gh label write — left read-only, the first real hold creates the label; argv for that write is pinned by tests.

verdict PASS on https://github.com/DnzzL/herdr-docket/pull/24: Checked PR 24 (head 03af899, merge-base 570ded5 = origin/main) in my own throwaway worktree, clean tree; worktree removed after. Bar green: go build ./..., go vet ./..., go test ./..., gofmt -l . all clean; CI build job (same four steps) green; one commit whose message is the ticket's own, 18 files all in scope (code, tests, ADR 0014, ADR 0010 amendment, CHANGELOG, factory.md).

#1 MET. Every human-only stop funnels through Runner.mergeNeeded (internal/runner/runner.go:375): all gate holds via hold (internal/runner/pipeline.go:129; sites pipeline.go:84,93,100 incl. critical/CODEOWNERS/merge-never via gate.Decide, pipeline.go:103 refused merge), FAIL past maxRounds (pipeline.go:50), failed run with an open PR via notifyFailed (runner.go:344-356; PR read from the real history store, history.TaskPullRequest, internal/history/history.go:127). Popup title 'merge needed — <repo>#<number>' built by gate.RepoRef, reason as body, host.SoundRequest (internal/host/host.go:111). Exactly one popup per stop: TestACodeownersPathIsHeld (pipeline_test.go:213), TestTwoFailsBlockTheTaskForAHuman (pipeline_test.go:171), TestAFailedRunWithAPRAnnouncesTheMergeTheHumanOwes (runner_test.go:354) each assert notifies==1 plus exact title+sound. Mutation SoundRequest->SoundNone turned all three red. Live: herdr notification show 'merge needed — DnzzL/herdr-docket#24' --sound request -> shown:true (herdr accepts none|done|request).

#2 MET. Same helper calls forge.AddLabel (runner.go:381) -> GH.AddLabel (internal/gate/forge.go:83): best-effort 'gh label create <label> --repo <r>' then 'gh pr edit <url> --add-label', so the label is created in the repo if missing; argv pinned by TestAddLabelCreatesTheLabelInRepoThenAppliesItToThePR (gate/forge_test.go:53), flags confirmed against gh label create/gh pr edit --help. The gate clears it at pipeline.go:107, on the repo's only merge call site (pipeline.go:102): TestTheGateClearsTheLabelOffAPRItMerges (pipeline_test.go:141) went red when I deleted the RemoveLabel call; TestAPassOnAGreenPRMergesAndClosesDone (pipeline_test.go:116) asserts a never-held merged PR carries zero labels. The real gh label write was left read-only by me (verifier never mutates the repo); the argv is pinned by tests.

#3 MET. With no PR, notifyFailed keeps 'fleet: run failed — <task>', SoundNone, no label (runner.go:357). TestAFailedRunReachesAHumanNotWatching (runner_test.go:324) asserts title, sound and zero labels; live herdr --sound none -> shown:true.

#4 MET. The three named cases run through the runner seam (Runner.Run with fake host/board/forge and the real history store in a temp state dir): TestACodeownersPathIsHeld, TestTwoFailsBlockTheTaskForAHuman, TestAFailedRunWithAPRAnnouncesTheMergeTheHumanOwes, all PASS. Stubbing TaskPullRequest to return nothing turned the third red, so the wiring is real, not a fake agreeing with the code.

Not blocking, follow-up filed as docket/TASK-66 for plugin-dev: internal/daemon TestEachTickWritesToTheQueueItReadFrom is flaky — observed on this PR and on origin/main without it (paired go test -count=200 stress runs failed on both trees). Mechanism proven by tag instrumentation (33/33 failures were the second waitClosed): waitClosed (daemon_test.go:149) returns when reconcile closes the task mid-attempt while Run still holds the task lock, so the second evaluate silently skips the task at internal/daemon/daemon.go:144 and nothing closes the second source (fails at exactly 2.00s). The race predates this PR; TASK-63 only widens its window by one history read. Production unaffected: the daemon ticks seconds apart and the next tick retries. Also noted: herdr rate-limits popups shown within seconds (second show returned shown:false rate_limited) — platform behaviour, not this PR.

fleet: https://github.com/DnzzL/herdr-docket/pull/24 is held for a human — internal/daemon/daemon_test.go is a CODEOWNERS path (/internal/daemon/).
<!-- SECTION:NOTES:END -->
