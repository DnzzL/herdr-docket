---
id: TASK-52
title: >-
  PR 10 (TASK-46): restore the 14 prompt-contract regression tests the PR
  deleted
status: Done
assignee:
  - plugin-reviewer
created_date: '2026-10-02 12:14'
updated_date: '2026-10-03 20:12'
labels: []
dependencies: []
ordinal: 52000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
PR 10 correctly fixes the TASK-46 prose failure (verified against all three acceptance criteria, CI green) but its diff deletes 14 regression tests from internal/prompt/prompt_test.go that no criterion asks it to delete — the failure mode the fleet brief names: a PR carrying work its ticket never asked for.

Fix: restore prompt_test.go to main's version, then append the PR's 4 new tests (TestTwoNamedEndingsAndTheChoiceIsTheTasksToDefer, TestNeitherEndingIsOptionalWithNothing, TestStopIsNotDoneUntilTheCommandRan, plus the contains/fold helpers — only TestAssembleCarriesEveryFactTheAgentNeeds is shared, keep it). I ran this exact fix in advance against the PR's own prompt.go: all 18 restored tests + 4 new ones pass unchanged (ok 0.008s), red/green evidence in the PR comment at https://github.com/DnzzL/herdr-docket/pull/10#issuecomment-5952071212.

After fixing, re-run CI-bar locally (go build ./..., go vet ./..., go test ./... -count=1, gofmt -l . empty) and push to the PR branch. Then re-review and merge per merge policy... and confirm TASK-46's acceptance criteria still hold with the tests back — two meta-runs, not TASK-46 re-run.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Restored main's prompt_test.go wholesale (15 regression pins including TestAssembleNeverNamesTheBackend, TestAssemblePrependsTheFleetBrief, TestThePromptNamesTheQueuesOwnWords, TestTheRoleBriefSitsBetweenTheFleetAndThePersona) and appended PR 10's contains/fold helpers plus the three new ending-pins verbatim; kept main's shared TestAssembleCarriesEveryFactTheAgentNeeds. Merged main (701984c) into the PR branch first so it is not stale. Committed 613642b — prompt_test.go only, no prompt.go or prose change — and pushed to the PR head branch, docket/TASK-52 done docket/TASK-52 https://github.com/DnzzL/herdr-docket/pull/10[613642b](https://github.com/DnzzL/herdr-docket/pull/10/commits/613642b). Verified myself: the three new pins red against main's prompt.go, all of internal/prompt green against the PR's prompt.go (the 18-test suite, ok 0.015s), full CI bar locally green (go build ./..., go vet ./..., go test ./... -count=1, gofmt -l . empty), GitHub CI success on 613642b, PR mergeable. TASK-46 criteria #1-#3 still hold with the tests back — the two meta-runs are the red-on-old/green-on-new evidence above. Answered the reviewer's blocking comment with this evidence: https://github.com/DnzzL/herdr-docket/pull/10#issuecomment-5957021592. Merge per policy is not mine to do (never merge own work; prompt.go means the policy routes it to a human). Handing to plugin-reviewer for the re-derivation the finding promised.

fleet: this run's delivery could not be read — workspace w7A (pane w7A:p1) is kept, unverified. The verdict stands.

fleet: the agent settled without reporting a verdict.

fleet: the run's workspace w1 (pane w1:pB) is left open — jump in to resume.

Stall sweep (fleet/TASK-21), pass 7, stall type 3 — your closing note promises 'Handing to plugin-reviewer for the re-derivation the finding promised'; that follow-up does not exist anywhere in the queue (live open queue checked: fleet/TASK-1/2/9/11/13/16/20, docket/TASK-49/50 — none covers PR #10). gh this sweep: PR #10 OPEN at head 613642b (your restore-tests commit), 0 reviews, 2 comments — the re-review never got a task. Your run also died 'settled without reporting a verdict' after the prompt fix (2026-10-02 18:52), which docket/TASK-49's note already books as post-fix instance 27/22, and the fleet line says workspace w7A kept, unverified. Who it waits on: the human — route the PR #10 re-review to plugin-reviewer (or fold it into the review sweep) and decide w7A. Nudge only: I do not reopen, close, assign, or file work of my own.

Cleanup 2026-10-03: folded into docket/TASK-46: its rework of PR #10 must restore the prompt-contract tests before the verifier will pass it.
<!-- SECTION:NOTES:END -->
