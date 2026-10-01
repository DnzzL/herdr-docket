---
id: TASK-45
title: A run inherits whatever branch the human left checked out
status: Done
assignee:
  - plugin-dev
created_date: '2026-10-01 09:42'
updated_date: '2026-10-01 14:07'
labels: []
dependencies: []
ordinal: 45000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Found dogfooding plugin-dev on 2026-10-01. The runner provisions a worktree from the main checkout's current HEAD, not from the queue's default branch. A human mid-branch therefore contaminates every agent run started while they are there.

Observed: the shell was left on fix/sweep-coverage-is-a-fact-about-the-pr with one unmerged commit. docket/TASK-23's run branched from it, and PR #2 against main carries TWO commits by two authors — the agent's ADR plus the human's unrelated fix, presented as part of the ticket. Nothing warned anybody; the run and the record both read clean.

Damage runs both ways: a PR that conflates authors and lands a human commit through an agent's PR without review of its own, and an agent inheriting half-finished human work as its base.

Close relative of TASK-38 (the host names the branch a run works on).

Acceptance criteria:
#1 A worktree run branches from the queue's default branch whatever the main checkout has checked out — a test pins it with the checkout on an unrelated branch.
#2 Where that is deliberately not wanted, the base is a stated choice, not an accident of where somebody's shell was.
#3 The run record names the commit the run branched from, so a contaminated PR is visible in history and not only on the forge.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Diagnosed: host.Provision's worktree leg calls ops.WorktreeCreate(repo, branch, label) with no base, so the CLI inherits the main checkout's current HEAD — verified live (worktree off a checkout mid-branch gets the branch tip). Design: fluently move the base through the seams like RunTag — work.BaseBrancher (optional, GetBaseBranch(id)), fleet.BacklogSource asking refs/remotes/origin/main at the workdir, host.Spec.Base set in runner.Run, herdr --base flag, history.BaseCommit + a CommitAt lookup in runner close. Walkthrough of history.Topology + the drift test before writing the first test. Not yet started editing.

Design written as ADR 0011 (docs/adr/0011-the-queue-names-the-branch-a-worktree-run-branches-from.md) in the run's worktree: work.BaseBrancher (optional capability next to Phaser/Assigner), backlog adapter derives origin/HEAD from its own dir at claim time, runner puts it into a new host.Spec.Base, host passes --base to herdr worktree create, session+record carry the base commit. worktree_base: on the source block for the deliberate override; no derived answer = inherit HEAD as today. Root mode untouched.

Layer 1 done, all green: work.BaseBrancher + work.BaseBranchOf helper (work.go), backlogmd default-branch read via git symbolic-ref refs/remotes/origin/HEAD in its own dir (cli.go DefaultBranch), worktree_base override through backlogmd.NewWith overrides map, multi composite forwards BaseBranch per queue, worktest contract subtest pins the capability stays optional. Tests: derived origin/main from a throwaway repo, override wins, no-remote and no-repo both report. Next: host side.

Implementation done, CI bar green (build/vet/test -count=1/gofmt all clean). Stack: work.BaseBrancher capability + runner asks it per task -> host.Spec.Base -> herdr worktree create --base (omitted when none) -> backlogmd derives origin/HEAD from the repo via symbolic-ref, with worktree_base: pin; multi forwards per prefix. Cut commit: host reads rev-parse before creating the worktree (CommitAt op), Session/Record.BaseCommit, shown as 'cut <sha>' in history. Tests pinned at every layer incl. checkout-on-unrelated-branch repr of TASK-23. Docs: queues.md worktree_base row, agents.md worktree prose, CHANGELOG, ADR 0011. Next: commit, PR, done.

A worktree run now branches from the queue's default branch (derived origin/HEAD via git symbolic-ref in the queue's own dir), or a stated worktree_base: ref on the source block; herdr worktree create gained --base. The run record carries the exact cut commit (base_commit, shown as 'cut <sha>' in history), so a PR built on a human's mid-branch work is visible from history alone. Root mode untouched by design. Signed off by: work.BaseBrancher capability + builder-of-base in runner, backlogmd adapter, multi forwarding, host CommitAt read before provision. CI green (build/vet/test/gofmt), PR #4 open and mergeable; ADR 0011 records the seam choice.
<!-- SECTION:NOTES:END -->
