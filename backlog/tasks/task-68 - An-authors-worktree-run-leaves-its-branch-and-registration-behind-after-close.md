---
id: TASK-68
title: An author's worktree run leaves its branch and registration behind after close
status: needs human validation
assignee:
  - plugin-dev
created_date: '2026-10-04 21:11'
updated_date: '2026-10-04 23:31'
labels: []
dependencies: []
ordinal: 68000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Every worktree-mode run (dev, plugin-dev) ends with its workspace closed but its git worktree still registered and its fleet/... branch still present in the primary checkout — git worktree list on ~/Projects/herdr-docket alone shows a dozen runs from the last three days, and the same pile exists on DishNow. Nothing inspects or removes them: host.Close only closes the workspace. TASK-65 added ops.WorktreeDiscard for verify sessions (nothing there but judging space); an author's branch is a delivery, so what an author run owes after its PR is pushed is a decision — prune the registration and delete only pushed branches, or keep the workspace like TASK-23 does — and then the same discard, or a lighter one, should run for author runs too.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
plugin-dev, first note — PLAN around the seam, building unless you redirect.

ASSUMPTIONS: (1) TASK-65's PR #29 (open, CI green) is the base — this branch is stacked on its commit, PR will target the task-65 branch; (2) 'after close' = where the fleet itself closes the author's workspace (Done/handedOn cleanup and the give-up path in runner.attempt); runs the fleet deliberately parks open (failed/blocked/cancelled stay as resume points) are untouched; (3) a PR is pushed from the run's own fleet/... branch, so 'pushed' is checkable offline from remote-tracking refs — the fleet fetches nothing (ADR 0017's rule).

DECISION (ADR 0018 to follow): an author run's Close owes a lighter discard than a verify run's. New ops.WorktreeRetire(repo, branch): prune the worktree registration when the tree is clean; delete the branch only when every commit it holds is already on a remote (git rev-list --count <branch> --not --remotes == 0); a dirty tree keeps everything (TASK-23's guarantee — nothing uncommitted is ever destroyed); if the workspace close itself failed, retire is skipped (the agent may still be living in that directory). host.Close calls it for non-verify sessions with a branch, after a successful WorkspaceClose; verify sessions keep TASK-65's full WorktreeDiscard. Cancelled/failed runs keep their workspace-open resume note — no runner behaviour changes.

TEST FIRST: fakeOps seam test pinning which sessions Close retires (author-yes, root-no, close-failure-no, verify-keeps-discard) + a real-git test proving registration gone / unpushed branch kept / pushed branch gone / dirty tree untouched. Then impl, full bar (build/vet/test/gofmt), LIVE_HERDR probe replay, ADR 0018 + changelog.d/TASK-68.md.

Decision + delivery — PR #30 (stacked on PR #29 / TASK-65's branch, its ops.WorktreeDiscard is the base this builds on): https://github.com/DnzzL/herdr-docket/pull/30

DECISION (ADR 0018): an author run owes a lighter discard than a verify run's. New ops.WorktreeRetire, called from host.Close for non-verify sessions with a branch after a successful WorkspaceClose: clean tree → git worktree remove (registration + checkout); branch deleted only when a remote holds its exact tip — the same-named refs/remotes/<remote>/<branch> the agent's own push wrote, so the fleet still fetches nothing; dirty tree → everything kept (TASK-23's guarantee); workspace close failed → nothing touched (the agent may still be in the directory). Runs the fleet parks open (failed/blocked/cancelled resume points) never reach Close and stay as they were — no runner behaviour changed. TASK-23's guard remains the keeper of doubtful deliveries, so what reaches Close has already been judged safe to end.

Red first: fa02092 (failing seam + real-git tests), green in 8242a87. The live probe caught a real bug before the PR: reachability-from-any-remote-ref deleted an unpushed branch whose tip sat on the remote under another ref name; c56cea2 pins the same-tip rule instead. Docs: ADR 0018, changelog.d/TASK-68.md, docs/agents.md worktree paragraph. Existing registrations runs already left behind are out of scope here — filed as docket/TASK-69.

Verified myself: full bar clean locally (go build/vet/test ./..., gofmt -l . empty) and CI green on PR #30 (run 37236793013). Seam: TestCloseRetiresAnAuthorWorktreeWhenTheWorkspaceIsReallyClosed pins who gets retired (author yes incl. ErrGone; root/tab/close-failure/verify no) and TestCloseSurfacesAFailedRetire pins error propagation. Real git: TestWorktreeRetirePrunesTheRegistrationAndDeletesOnlyWhatIsSafe — unpushed kept, pushed deleted, partially-pushed kept, twin-ref kept, dirty tree kept whole. Replayed on the real surface: LIVE_HERDR=1 go test ./internal/host/ — TestLiveAuthorWorktreeProvisionAndClose PASS (provision on a scratch repo, push, Close → git worktree list free of the registration, pushed branch gone locally yet still on origin, unpushed run's branch kept) and TASK-65's TestLiveVerifyProvisionAndClose still PASS on the stack. Red-before-green kept: the failing tests are commit fa02092, the fix 8242a87. PR stacked on #29 (TASK-65) — merge #29 first; GitHub retargets #30 to main. Pre-existing pile filed as docket/TASK-69.

fleet: run failed: waiting for the agent: agent wait: timeout: timed out waiting for agent status

fleet: the run's workspace w1 (pane w1:pW) is left open — jump in to resume.

verdict PASS on https://github.com/DnzzL/herdr-docket/pull/30: Re-derived criteria from the ticket, verified on the real surface (throwaway worktree at head 31a8394, human checkout untouched; PR diff vs base 8db5ea4 identical to gh pr diff 30, 8 files, 4 commits all the ticket's own). Full bar green: go build/vet/test ./..., gofmt -l . empty; CI run 37236793013 success at that headSha. Red-first proven at both stages: fa02092 fails exactly the three new tests (host_test.go:518/537, :847 — WorktreeRetire stub ops.go:155) with the rest of the suite green; 8242a87 green; the c56cea2 twin test transplanted onto 8242a87 fails at host_test.go:905, so the same-tip fix was red before it too. Seam: TestCloseRetiresAnAuthorWorktreeWhenTheWorkspaceIsReallyClosed (host_test.go:489) pins author+ErrGone retire, root/tab/close-failure/verify not (host.go:317-319), TestCloseSurfacesAFailedRetire pins error propagation. Real git: TestWorktreeRetirePrunesTheRegistrationAndDeletesOnlyWhatIsSafe (host_test.go:805) — unpushed kept, pushed deleted, partially-pushed kept, twin-ref kept, dirty kept whole. Live replay on real herdr: LIVE_HERDR=1 go test ./internal/host/ — TestLiveAuthorWorktreeProvisionAndClose and TestLiveVerifyProvisionAndClose both PASS. Independent delivery-chain proof: after retire deletes a pushed local branch, refs/remotes holds the exact tip, so the verifier still cuts its worktree at the PR head — survives git gc --prune=now. Scope: runner diff comment-only (no runner behaviour change; parked runs keep their resume note), docs done (ADR 0018 with rejected alternatives, changelog.d/TASK-68.md, docs/agents.md), pre-existing pile filed as docket/TASK-69. Non-blocking, checked: dirty = git status --porcelain (ops.go:219), so gitignored files go with the removal — required (DishNow node_modules would pin every worktree) and no delivery at risk since ignored files can never be pushed.
<!-- SECTION:NOTES:END -->
