# The queue names the branch a worktree run branches from

The failure this answers: a human left their checkout of the queue's project
mid-branch with one unmerged commit, and the next agent run branched from that
commit. The worktree got the human's half-finished work as its base; the run's
pull request then carried two commits by two authors, one of them a human fix
nobody had asked the fleet to land. Nothing warned anybody — the run read
clean, the record read clean (TASK-45's finding).

**The base is the queue's, not the checkout's.** A run of a task inherits the
task's queue, and the queue names the project the work happens in. What branch
that work starts from is therefore a fact about the queue — which is why the
default is the queue's default branch and not whatever its main checkout
happens to have checked out when a run starts. A worktree is disposable; the
branch it forks is not a side effect to inherit.

**The queue tells the runner; the host stays base-blind.** The runner already
holds the work.Source a task came from, so the port gains one optional
capability next to `Phaser` and `Assigner`:

```go
// BaseBrancher is the optional capability of a queue that knows which git ref
// its work branches from.
type BaseBrancher interface {
    BaseBranch(id string) (string, error)
}
```

The Backlog.md adapter reads its project's own repo (the adapter already knows
the dir — it is the fleet dir or the source's `dir:`). `git symbolic-ref
refs/remotes/origin/HEAD` names the remote's default branch (`origin/main`);
its failure is not fatal — a repo with no remote just has no answer, and the
run branches from the checkout's HEAD exactly as it always did. The runner puts
the answer into `host.Spec.Base`, and the host passes `--base` to `herdr
worktree create`. Root mode never sees it: it has no branch, and borrowing the
checkout it stands in is its design.

**A queue that wants something else says so.** `worktree_base:` on the source
block names the ref. It is config under the port, like `statuses:` — the
core never learns a git word, the adapter does; and like an unassigned task
left alone, a queue that names nothing gets the derived default rather than an
invention.

**Where the base lands in the record.** The commit the base ref resolved to at
provision time is recorded on the run: `host.Session.BaseCommit` (git is read
behind the host port, so the host answers for it), carried through
`runner.appendWith` into `history.Record.BaseCommit`, and read at closing
through `history.CommitAt(runID)` like `PullRequestFor`. A PR that carries a
human's commit is then a diff a reader can compute — `base_commit..branch` —
from the record alone, without opening the forge.

**Rejected alternative:** deriving the default branch in the host at provision
time. It would have put a git read behind the host port's provision answer and
left the job of *which* ref a project's work starts from with the host —
which does not know what queue the task came from.
