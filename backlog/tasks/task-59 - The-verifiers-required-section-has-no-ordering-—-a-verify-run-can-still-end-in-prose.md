---
id: TASK-59
title: >-
  The verifier's required section has no ordering — a verify run can still end
  in prose
status: Done
assignee:
  - plugin-dev
created_date: '2026-10-03 21:31'
updated_date: '2026-10-03 22:41'
labels: []
dependencies: []
ordinal: 59000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
TASK-46's ordering fix (PR 10) lives in protocol()'s section only: internal/prompt/prompt.go:202 states 'the ending command is the last tool call of the run: execute it, then summarise' and the prose-ending failure. verify()'s own required section (prompt.go:116-124, 'Record exactly one verdict...') has no such ordering statement, so a verifier run — the shape that replaced the reviewer persona in #15, and the exact run type TASK-46's description cites ('the review of PR 42 ... filed as a failure') — can still end with a conclusion in prose and be filed 'settled without reporting a verdict'. Out of TASK-46's scope (the ticket predates #15 and its ACs name done|fail|block and assign, which the verify section does not teach), so not blocking PR 10; same failure mode, one section over. Fix: state the ordering in verify()'s section too and pin it with a prompt test, same as TestStopIsNotDoneUntilTheCommandRan.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
verify()'s required section (prompt.go) gains the ordering statement protocol() got in PR 10: the verdict command is the run's last tool call, execute it then summarise, a prose-ending verify run is filed 'settled without reporting a verdict' (runner.go:329). Pinned by TestTheVerdictCommandIsTheLastToolCallOfAVerifyRun, committed red before the fix. CI green locally (build/vet/test/gofmt) and on the PR. PR: https://github.com/DnzzL/herdr-docket/pull/21

Stated the ordering in verify()'s required section (prompt.go: 'the verdict command is the last tool call of the run — execute it, then summarise', naming the 'settled without reporting a verdict' filing), and pinned it with TestTheVerdictCommandIsTheLastToolCallOfAVerifyRun in internal/prompt/prompt_test.go, same shape as TestStopIsNotDoneUntilTheCommandRan. Verified myself: the test fails against the original prompt.go (missing 'the last tool call of the run') and passes after the fix; go build/vet/test ./... and gofmt -l . all clean locally; GitHub Actions build green on PR 21.

verdict PASS on https://github.com/DnzzL/herdr-docket/pull/21: Re-derived both ACs on a disposable worktree of pr-21. (1) Ordering stated: internal/prompt/prompt.go:124-129 now sits inside verify()'s '## When you are done — required' section — 'The verdict command is the last tool call of the run: execute it, then summarise', naming the exact filing a prose ending gets (internal/runner/runner.go:329 'the agent settled without reporting a verdict', phrase matches). (2) Pinned: TestTheVerdictCommandIsTheLastToolCallOfAVerifyRun (internal/prompt/prompt_test.go:341), same shape as TestStopIsNotDoneUntilTheCommandRan (prompt_test.go:317) — contains + section-position checks. Red-before-green replayed myself: at 9e063e0 (test only, base prompt.go) the test FAILs; at ccacc88 it passes. Mutation: moving the statement out of the required section fails at prompt_test.go:359, so the position pin is real — verify() never calls protocol() (prompt.go:107), so the phrase can only come from the section under test. Bar on the branch: go build/vet/test ./... and gofmt -l . all clean in my own worktree; gh pr checks 21 build green (run 37157997473). Scope: diff vs origin/main is 3 files, 41 insertions, 0 deletions — two commits are the ticket's own (test, then fix+CHANGELOG); verdict command lines and every other prompt byte-identical by zero deletions. Safety fact: no code parses this text — grep over non-test Go finds the phrases only in prompt.go itself and runner.go's pre-existing error strings.

fleet: merged https://github.com/DnzzL/herdr-docket/pull/21 — verified PASS on patch 14e19319725998ce04901987a3040033a0902ef6, CI green.
<!-- SECTION:NOTES:END -->
