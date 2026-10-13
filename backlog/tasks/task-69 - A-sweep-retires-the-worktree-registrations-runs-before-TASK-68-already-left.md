---
id: TASK-69
title: A sweep retires the worktree registrations runs before TASK-68 already left
status: Blocked
assignee:
  - plugin-dev
created_date: '2026-10-04 21:37'
updated_date: '2026-10-04 22:57'
labels: []
dependencies: []
ordinal: 69000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
TASK-68's retire runs from host.Close going forward, but the registrations already piled up (a dozen on ~/Projects/herdr-docket, the same on DishNow) belong to runs that will never close again — nothing will ever prune them. A one-off sweep should retire them with TASK-68's own rules (ops.WorktreeRetire: prune clean registrations, delete only a branch a remote holds at the same tip, keep dirty trees whole), first cross-checking herdr-docket history and open workspaces so the worktrees of runs the fleet parked open (failed/blocked resume points) and any run still in flight are left alone.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
plugin-dev, first note — PLAN around the seam, building unless you redirect.

ASSUMPTIONS: (1) PR #30 (TASK-68) is the base — this branch is stacked on its tip (31a8394) because ops.WorktreeRetire is its seam; the PR will target task-68's branch and GitHub retargets it when #29/#30 merge; (2) the registrations to sweep live where runs cut them — agents' workdirs (runner sets spec.Repo = a.Workdir), so repos come from agents' own config (deduped: DishNow-v2, herdr-docket, fleet; a workdir missing on this machine is skipped with a line); (3) a registration maps to a run by worktree path basename == slug(run.Branch) (agents rename branches inside a worktree — task22/rebase etc.) with branch equality as the second key; (4) retire ONLY when history's latest record for that run is status done with verdict neither failed nor blocked AND herdr reports no open workspace on it — failed/cancelled/blocked are resume points, scheduled/running/timed_out are in flight, anything history cannot attribute stays put (evidence unsettled -> human); (5) the retire call itself is TASK-68's ops.WorktreeRetire untouched: clean tree pruned, branch deleted only when a remote holds the same tip, dirty tree kept whole; (6) surface: a CLI command herdr-docket worktree sweep [--dry-run] — the replayable port (list + history + open workspaces in, one retire rule out), tested through that port, then replayed on the real pile as the verification.
Say stop, or I build on these.

SCOPE: no runner/daemon changes, no edits to PR #30's code, one PR. Tests first (seam: which registrations retire/kept and why; dry-run touches nothing; errors reported not swallowed), then impl, full bar, dry-run over the real repos, the real sweep, docs + changelog.d/TASK-69.md, PR.

plugin-dev, second note — the sweep ran on the real pile.

WHAT: new command `herdr-docket worktree sweep [--dry-run]` (PR #31, stacked on PR #30). Per agent checkout: herdr worktree list (open-workspace flag), map each registration to its run by worktree directory (branch Provision cut, survives renamed branches inside — half the pile holds branches like task22/rebase) or by checked-out branch; retire only when history says the run ended done with verdict neither failed nor blocked AND no open workspace. Retire = TASK-68's ops.WorktreeRetire exported as host.RetireWorktree — one rule set, untouched.

REPLAYED ON THE REAL SURFACE (built to /tmp, never the daemon checkout): dry-run first — 32 planned / 30 kept / 0 failed, every keep with its reason (in flight, resume point, workspace still open, no run in history); then live — 31 retired, 31 kept, 0 failed, exit 0. Before/after evidence kept: docket registrations 27→12, DishNow 37→21; 11 pushed branches deleted (5 docket + 6 dishnow) with refs/remotes/* intact — origin/fleet/docket-task-68-... still holds PR #30's head at 31a8394; unpushed branches (fleet/task22-rebase-51, fleet/docket-task-46-restore-the-pins, pr-41-fix, ...) kept; dirty task-138 kept whole (uncommitted playwright-report); my own in-flight worktree and dishnow/TASK-139's untouched; /tmp worktrees and anything history cannot name kept and reported.

TEST FIRST: red commit 40730f9 (seam + command tests), green 0563c34. Full bar clean: go build/vet/test ./..., gofmt -l . empty. PR: https://github.com/DnzzL/herdr-docket/pull/31

Delivered PR #31 (stacked on PR #30 / TASK-68's branch — merge #30 first, GitHub retargets). The one-off sweep exists as herdr-docket worktree sweep [--dry-run]: it cross-checks run history and herdr's open workspaces before retiring anything with TASK-68's own rules (ops.WorktreeRetire, exported as host.RetireWorktree — one rule set for the pile and every Close). Verified myself: (1) full bar clean locally — go build/vet/test ./..., gofmt -l . empty — and CI green on PR #31 (run 37240168504); (2) red first — failing seam/command tests committed as 40730f9 before the fix 0563c34; (3) replayed on the real surface — dry-run showed 32 planned / 30 kept / 0 failed with a reason on every keep, then the live run retired 31 of 62 registrations with 31 kept and 0 failures, exit 0; before/after git worktree list and for-each-ref diffs kept in /tmp (docket 27→12, DishNow 37→21): 11 pushed branches deleted with refs/remotes/* intact (origin/fleet/docket-task-68-... still holds PR #30's head), unpushed branches kept, dirty task-138 worktree kept whole, this run's own in-flight worktree and dishnow/TASK-139's untouched, everything history could not name kept and reported.

verdict PASS on https://github.com/DnzzL/herdr-docket/pull/31: Re-derived every criterion on a throwaway worktree at PR head 0563c34 (checkout untouched). (1) Full bar green: go build/vet/test ./..., gofmt -l . empty; CI run 37240168504 success on 0563c34. (2) Red-first proven: at 40730f9 the new tests fail rc=1 (undefined Sweep/Options/worktreeCmd). (3) Diff is the ticket's own — 2 commits; new internal/sweep, additive host.RetireWorktree (host.go:152) delegating to the same stateless herdrOps.WorktreeRetire Close uses (ops.go:160; real-git tests host_test.go:805), CLI worktree sweep, README + docs/agents.md + changelog.d/TASK-69.md; no runner/daemon/gate changes, no go.mod change. (4) Fakes agree with the world: herdr.Worktree keys match real 'herdr worktree list' JSON; history.Runs gives latest record per run newest-first (history.go:210-234); keepReason's verdict compare matched real lowercase 'blocked' records. (5) Real-surface replay: dry-run exit 0 twice with a reason on every keep, and git worktree list + for-each-ref + herdr lists byte-identical before/after (touches nothing); then live — 1 retired / 34 kept / 0 failed: only the completed TASK-69 run's worktree+local branch removed, refs/remotes/origin/fleet/docket-task-69-... and refs/pull/31/head still 0563c34, dirty DishNow task-138 kept whole, in-flight dishnow/TASK-140 and open workspaces w8P/w8C/wA3 kept, unattributable /tmp worktrees kept. (6) Author's aftermath reconciles: DishNow 21 = /tmp/sweep-after-dishnow.txt, docket 12 (+3 worktrees cut since, all kept 'no run in history'), refs diffs show exactly 5+6 local branch deletions with no refs/remotes/* lost, origin/fleet/docket-task-68 still 31a8394, unpushed task22-rebase-51 and docket-task-46-restore-the-pins still present.

fleet: https://github.com/DnzzL/herdr-docket/pull/31 is held for a human — internal/host/host.go is a CODEOWNERS path (/internal/host/).
<!-- SECTION:NOTES:END -->
