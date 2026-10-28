---
id: TASK-23
title: A run that reports done and commits nothing is not a run the fleet can trust
status: Done
assignee: []
created_date: '2026-09-13 18:13'
updated_date: '2026-10-01 09:40'
labels: []
dependencies: []
priority: high
ordinal: 23000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The fleet's promise is that it works while nobody watches. Tonight it broke that promise quietly: TASK-39 on the dishnow board was closed `done`, its closing note was accurate and detailed, its acceptance criteria were argued one by one — and its branch had zero commits. The work existed only as uncommitted files in a worktree whose workspace had just been torn down for ending `done`. A human caught it with `git log main..<branch>`. The fleet reported success.

That is the failure mode that decides whether this project is worth existing. Everything else — the queue, the routing, the roles — is scaffolding around "an agent worked while you slept". If the fleet cannot tell a delivery from an empty branch, the verification is human, and unattended operation is a claim rather than a feature.

## The signal, and what it is not

Do not reach for "done means commits". It is wrong for most of the fleet:

- A `workspace: root` agent is often SUPPOSED to leave edits uncommitted — notara-pm's persona says so explicitly, because the human decides what lands in git.
- A PM triaging a board, a reviewer posting a verdict, a research task: none of them touch code, and a `done` with no commit is correct.

The unambiguous signal is narrower and it is not about intent, it is about loss:

**A disposable worktree about to be destroyed, with uncommitted changes in it.** That is work the agent produced and the fleet is about to delete. It cannot be a false positive: either those changes mattered, in which case losing them is a bug, or they did not, in which case the agent left litter in a checkout it was told to work cleanly in.

A weaker second signal, worth recording but probably not worth overriding a verdict: a `workspace: worktree` run that ends `done` with no commits ahead of its base AND a clean tree. Sometimes legitimate (nothing needed changing), sometimes an empty delivery.

## The decisions this opens

- Where the check lives. `cleanup` (runner.go) already decides whether to tear a workspace down on `Done` — it is the one place that knows the verdict and the workspace, and it is where the loss happens.
- What it does when it fires. Override the verdict to `failed`? Keep `done` but refuse to tear the workspace down, and say so on the task? The second loses nothing and lies about nothing, but it leaves a task closed on work nobody merged.
- How it reads the git state. The runner does not shell out to git today and the host owns the workspace. Adding git knowledge to the runner is a real widening of what it knows — decide whether it belongs there or behind the host port.
- Whether an agent declares what it produces (`expects_commit`) or whether the workspace mode is enough to infer it.

## Not in scope

Do not build a general "did the agent do a good job" checker. This is one narrow, mechanical fact: files that exist and are about to stop existing.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A run whose disposable worktree still holds uncommitted changes when it ends cannot silently destroy them — the fleet says so on the task and in the run history
- [ ] #2 A root-mode run that deliberately leaves edits uncommitted is untouched by the check, and a test pins that
- [ ] #3 A PM or reviewer run that legitimately produces no commit is untouched, and a test pins that
- [ ] #4 herdr-docket history shows whether a run produced commits, so the answer does not require leaving the fleet's own tools
<!-- AC:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
fleet: default_agent "plugin-dev" (fleet.yaml) is not a fleet agent.

The guard shipped before this run: v0.7.0 already contains it (runner.go guardDelivery + host.Inspect behind the host port), so this run verified the four criteria against the shipped code rather than rebuilt it, and wrote down the structural decisions the task left open as ADR 0009 (PR #2). Criterion by criterion: #1 met — guardDelivery keeps the worktree, re-closes a done verdict as blocked, notes the task, and history records uncommitted=true; pinned by TestADoneRunHoldingUncommittedWorkIsKeptAndSaid and seen live on dishnow/TASK-120 and TASK-17 in history. #2 met — TestARootRunIsNeverGuardedBecauseItHasNoWorktreeToLose, TestARootRunClaimsNoBranchAndIsNeverInspected, TestInspectRefusesASessionWithoutABranch. #3 met — the guard reads the workspace mode, not the persona; TestACleanWorktreeRunWithNoCommitsTearsDownAsUsual pins a no-commit worktree run ending done untouched. #4 met — history records branch, commits (+0 shown, not omitted) and pull request per run, visible in herdr-docket history and pinned by TestClosingRecordStatesWhatTheRunProduced. CI (build) green on the PR. The second signal from the description is recorded (commit count in history) but never overrides a verdict, as argued in the ADR.

The guard this task describes shipped in v0.7.0, on main, before this run: runner.go's guardDelivery keeps a disposable worktree holding uncommitted changes, re-closes a done verdict as blocked, says so on the task, and the run history records uncommitted=true — with git's reads behind the host port (host.Inspect), not in the runner. All four criteria verified met: #1 pinned by TestADoneRunHoldingUncommittedWorkIsKeptAndSaid and confirmed live in history (dishnow/TASK-120 and TASK-17 caught with !uncommitted); #2 pinned by TestARootRunIsNeverGuardedBecauseItHasNoWorktreeToLose / TestARootRunClaimsNoBranchAndIsNeverInspected / TestInspectRefusesASessionWithoutABranch; #3 pinned by TestACleanWorktreeRunWithNoCommitsTearsDownAsUsual (guard keys on workspace mode, not persona); #4 pinned by TestClosingRecordStatesWhatTheRunProduced — history shows branch, commits and PR per run. The only thing missing was the record of the structural decisions the task opened, written as ADR 0009 in PR #2, which is this run's delivery. Whole-tree go build/vet/test/gofmt clean; CI green on the PR. Not verified by me: nothing claimed untested — every criterion has a pinning test I ran.
<!-- SECTION:NOTES:END -->
