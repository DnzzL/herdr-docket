---
id: TASK-65
title: >-
  A verify run works in its own worktree cut from the PR head, not a tab on the
  human's checkout
status: Done
assignee: []
created_date: '2026-10-04 16:08'
updated_date: '2026-10-28 15:25'
labels:
  - enhancement
dependencies: []
priority: medium
ordinal: 65000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Today the reviewer runs in workspace: root, so every verify run is a tab on the project's primary checkout (DishNow: w15 on ~/Projects/DishNow-v2) — the folder the human and their interactive sessions work in. Isolation rests on one persona sentence ('never run git stash, git checkout… make your own worktree'); the reviewers comply, but each one rebuilds it by hand (git worktree add /tmp/verify*, git clone, git fetch pull/N/head:pr-N-verify), which costs setup time on a 45-minute budget and leaves refs, branches and registered /tmp worktrees behind in the human's repo. Wanted: the host provisions the isolation — a verify run gets a fresh worktree checked out at the PR's head commit, removed when the run settles — and the persona stops asking the agent to build it.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A verify run starts in a worktree whose HEAD is the PR's head commit, without the agent fetching or checking anything out
- [ ] #2 The run creates no branch, ref or worktree in the primary checkout that outlives it
- [ ] #3 A verify run never opens a tab on the primary checkout
- [ ] #4 The reviewer and plugin-reviewer personas lose the build-your-own-worktree paragraph; the shipped example in docs matches
- [ ] #5 A test through the host seam covers provisioning a verify worktree at a PR head and its removal
<!-- AC:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
plugin-dev run, first note — PLAN around the seam. host.Spec gains Head: the PR's head commit; a pinned run is provisioned in its own worktree cut at that commit whatever the agent's workspace mode says (so a verify run can never borrow a tab on the primary checkout). host.Session gains Verify; Host.Close discards a verify worktree and its branch (new ops WorktreeDiscard: git worktree remove --force + branch -D) — nothing the run made outlives it. runner: a verify run reads the PR head through gate.Forge, skips Inspect/guard (the verdict in the queue is the report, not the tree), and tears the worktree down however the run ended. Personas: reviewer.md + docs/examples.md (drift pair) and the live reviewer/plugin-reviewer AGENT.md lose the build-your-own-worktree paragraph. ADR 0014 + CHANGELOG entry. Building this unless you redirect.

fleet: run failed: waiting for the agent: agent wait: timeout: timed out waiting for agent status

fleet: the run's workspace wC4 (pane wC4:p1) is left open — jump in to resume.

Implementation done, PR opened: https://github.com/DnzzL/herdr-docket/pull/29 (branch cut from origin/main). Seam: host.Spec.Head pins a run to the PR head — the host cuts a worktree at it whatever the agent's workspace mode says (no tab reachable), host.Session.Verify marks it, Close discards worktree + branch (new ops WorktreeDiscard: git worktree remove --force + branch -D, the one git write behind the port, ADR 0015). Runner reads the head through gate.Forge, never Inspects a verify session, tears it down however the run ended; unreadable head fails the run with the reason. Personas: shipped reviewer.md + docs/examples.md (drift pair) and the live ~/fleet/agents/{reviewer,plugin-reviewer}/AGENT.md lost the build-your-own-worktree paragraph — kept the refusal to touch the primary checkout. Verified: full CI bar green; host + runner seam tests; real-git discard test; LIVE_HERDR=1 probe on real herdr — provision with primary open → worktree HEAD == PR head, no tab; Close → branch and registration gone. Caveat: the live persona edit describes provisioned worktrees before the merged binary ships — verify runs stay tabs until this PR merges.

PR #29, CI green (run 37234646765: go build, go vet, go test, gofmt all clean; local bar replayed after the rebase onto origin/main). Verdicts per criterion:
#1 MET — the runner reads the PR head through gate.Forge and puts it on host.Spec.Head; the host cuts the verify worktree at that commit (CommitAt ref and herdr --base are both the head; session BaseCommit = head), so the agent fetches and checks out nothing — the prompt now says so ('no fetching and no checkout to do first'). Evidence: TestAVerifyRunIsPinnedToThePRHeadAndNeverBorrowsATab (host seam), TestAVerifyRunIsProvisionedAtThePRHeadAndNeverInspected (runner seam, only the verify stage carries the forge head).
#2 MET — Close on a verify session discards worktree and branch through the new ops WorktreeDiscard (git worktree remove --force, then git branch -D), including when the workspace is already gone (cancelled run). Evidence: TestCloseDiscardsAVerifyWorktreeAndItsBranch, and the real-git TestWorktreeDiscardLeavesNoBranchOrWorktreeBehind asserting the registration, the checkout dir and refs/heads are all gone.
#3 MET — Spec.Head is provisioned before the workspace-mode switch ever runs, so a verify run cannot reach the tab-borrowing branch regardless of the agent's workspace: root. Evidence: same host test asserts tabs=0 while the primary workspace is open, plus TestProvisionRefusesAPRHeadTheRepoDoesNotHave (even a failed provision never borrows a tab).
#4 MET — internal/factory/personas/reviewer.md and docs/examples.md example 4 edited verbatim (TestWhatWeShipIsWhatTheDocsShow drift test green); the live ~/fleet/agents/reviewer/AGENT.md and plugin-reviewer/AGENT.md lost the same paragraph (the never-touch-the-primary-checkout refusal is kept); grep finds no build-your-own-worktree left in any reviewer persona. Only the pm example keeps it — pm still runs in root and needs it.
#5 MET — through the host seam: provision-at-a-PR-head and its removal are covered by TestAVerifyRunIsPinnedToThePRHeadAndNeverBorrowsATab + TestCloseDiscardsAVerifyWorktreeAndItsBranch + TestProvisionRefusesAPRHeadTheRepoDoesNotHave + the real-git discard test; runner policy by TestAVerifyRunIsProvisionedAtThePRHeadAndNeverInspected, TestAVerifyWorktreeIsTornDownEvenWhenTheRunEndsWithoutAVerdict and TestAVerifyRunThatCannotReadThePRHeadFailsSayingWhy (unreadable head fails with 'the pull request could not be read', no fallback). Replayed on the real surface: LIVE_HERDR=1 go test ./internal/host/ -run TestLiveVerifyProvisionAndClose PASS — real provision with the primary open (worktree HEAD == PR head, no tab), Close then leaves git worktree list at the primary alone with the branch ref gone.
Design: ADR 0017 (renumbered on rebase — 0015/0016 were taken), changelog fragment changelog.d/TASK-65.md per ADR 0016, CHANGELOG.md untouched.
GAPS: not done — an end-to-end verify run on the merged binary; this PR's own verification runs pre-merge, so until merge+restart the live personas describe incoming behaviour while runs still land on a tab. not tested — a fork PR whose head object is absent locally (provision fails with the reason by design; no live case here). untouched — author worktree runs still leave branch + registration behind; filed as docket/TASK-68.

State of the delivery, for whoever picks this up next — the evidence does not settle what happens next, so it is yours. What happened: this run passed its 45-minute budget and the fleet closed the task Failed at 21:04 ('run failed: waiting for the agent: agent wait: timeout'; the daemon runs the pre-TASK-62 binary, so a late delivery cannot reach the pipeline the way the new one does). The PR was opened at 21:03 and stamped at 21:12 by 'task done --pr' — one verdict, done, with the per-criterion evidence in the note above. Consequence: the verify stage never started, so plugin-reviewer has NOT judged PR #29 yet; the task sits in needs human validation because my run failed, not because anything is wrong with the PR (CI green, run 37234646765). Next move is a human's or the stall sweep's: reopen/assign so plugin-reviewer judges https://github.com/DnzzL/herdr-docket/pull/29, or review it by hand. Two facts to carry: (1) the branch was rebased onto origin/main mid-run (main gained the changelog-fragment convention — my entry is changelog.d/TASK-65.md, ADR renumbered 0017, CHANGELOG.md untouched); (2) my workspace wC4 is still open from the failed cleanup — close it when done, nothing in it is needed (everything is pushed). Follow-up for author-run leftovers filed as docket/TASK-68.

fleet: https://github.com/DnzzL/herdr-docket/pull/29 is merged — the pull request settles this task done.
<!-- SECTION:NOTES:END -->
