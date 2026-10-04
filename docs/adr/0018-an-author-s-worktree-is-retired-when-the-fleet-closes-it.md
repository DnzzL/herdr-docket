# An author's worktree is retired when the fleet closes it

The failure this answers: every worktree-mode run ended with its workspace
closed but its git worktree still registered and its `fleet/…` branch still
present in the primary checkout — `git worktree list` on the plugin's own
repo showed a dozen runs from three days, with the same pile on DishNow.
`host.Close` closed the workspace and nothing else inspected or removed the
rest (TASK-68). A verify run already got its teardown from
[ADR 0017](0017-a-verify-run-is-cut-from-the-pr-head-and-discarded-when-it-settles.md);
an author's run had no rule at all.

**An author's branch is a delivery, so the bar is what would be lost.**
`Close` retires what `Provision` created — the registration and checkout go
once the tree is clean, and the branch goes only when a remote holds its
exact tip: the agent's push wrote `refs/remotes/<remote>/<branch>` into the
shared `.git`, so the answer comes from refs the push itself updated and the
fleet still fetches nothing (0017's rule). A matching tip means the whole
chain the branch holds is on that remote — the local branch is a copy and
dies with its registration. An unpushed branch, or one whose tip moved
locally after the push, is the delivery's only copy and stays as a ref in
the primary, where `git log` and a later push still reach it.

**A dirty tree keeps everything, and a workspace that would not close keeps
its directory.** Uncommitted work is never destroyed by a teardown —
[ADR 0009](0009-a-doubted-delivery-blocks-and-git-is-read-behind-the-host-port.md)'s
guarantee — so a dirty worktree is left registered with its branch for a
human to look at. And while `WorkspaceClose` may have failed, the agent may
still be working in the directory being removed, so a failed close retires
nothing; the runner already logs the close failure.

**What already keeps the doubtful cases never reaches this.** `guardDelivery`
(TASK-23) runs before `Close` and keeps the workspace whole when the tree is
dirty or the delivery unverifiable, with the reason on the task — those runs
are untouched here by construction. Runs the fleet deliberately parks open
(failed, blocked, cancelled — the "left open, jump in to resume" note) never
reach `Close` either: the worktree *is* the resume point, and retiring it
would gut the note.

**Rejected alternatives:**

- **Keep the workspace and registration like TASK-23 does for every author
  run** — that is the pile: TASK-23's keep is evidence-driven (doubt), and
  applying it to every finished run means a repo that accumulates registered
  worktrees forever with no one assigned to prune them.
- **The verify run's full discard (`remove --force` + `branch -D`)** — right
  for judged space whose report is the verdict, wrong for a delivery: it
  would delete an unpushed branch and force away uncommitted work the guard
  never saw (the give-up path closes without one).
- **Reading "pushed" as reachability from any remote ref** (`rev-list
  --count <branch> --not --remotes`) — a branch whose tip is already on the
  remote behind *another* ref name looks pushed and would be deleted under
  that rule; the live probe caught it. What is deleted is the branch the
  push actually made: a same-named remote-tracking ref at the same tip.
- **`herdr worktree remove --workspace`** — same answer as in 0017: it only
  works while the workspace lives and never touches the branch; git reaches
  both states after the fact.

**Status:** accepted.
