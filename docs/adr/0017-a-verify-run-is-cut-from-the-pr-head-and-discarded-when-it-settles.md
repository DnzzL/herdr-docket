# A verify run is cut from the PR head, and discarded when it settles

The failure this answers: every verify run was a tab on the project's primary
checkout — the folder a human and their interactive sessions work in — with
the whole isolation resting on one persona sentence ("never run `git stash` …
make your own worktree"). The reviewers complied, and each one rebuilt the
isolation by hand: `git worktree add /tmp/verify*`, a clone, a
`git fetch pull/N/head:pr-N-verify` — setup minutes out of a 45-minute budget,
and refs, branches and registered `/tmp` worktrees left behind in the human's
repo for good (TASK-65).

**The host provisions the isolation; the persona stops asking for it.** A
verify run is pinned to the PR's head commit. `Spec.Head` carries the forge's
answer — `headRefOid`, read by the runner through the forge seam before
provisioning — and a pinned spec is provisioned in a worktree cut at that
commit *whatever workspace mode the agent names*: the mode never reaches the
switch, so a root-mode verifier structurally cannot borrow a tab on the
primary checkout (AC #1, #3).

**The head must already be in the repo; the fleet does not fetch.** The
author pushed the PR's branch from a checkout of this very repo, so the
commit is reachable locally through `origin/<branch>`. A head the checkout
cannot resolve fails the provision with the reason, rather than starting a
run on some other commit. Rejected: `git fetch pull/N/head` behind the host
port — it writes `FETCH_HEAD` and usually a ref into the human's repo, the
exact residue the task exists to remove, and it needs PR-URL parsing the
host has no business doing (the forge is the runner's seam, and a fork PR
that does fail the provision is a report a human can settle).

**A verify worktree is read for nothing, and removed however the run
ended.** Delivery normally survives as a fact read from the worktree before
teardown, and a dirty worktree blocks a `done` (ADR 0009) — for a verifier
both would be wrong: its report is the verdict in the queue, and a judge's
test artifacts would read as a delivery nobody committed. So a verify
session is never `Inspect`ed, never guarded, and `Close` discards the
worktree *and* the branch (`git worktree remove --force`, then
`git branch -D`) whether the run settled, failed, was cancelled by closing
its workspace, or timed out (AC #2, #5). This is the one git write behind
the host port: 0009's rule was about not believing an agent's word for a
delivery — the discard is teardown of what the provision itself created, and
only the host knows the path and the branch.

**Rejected alternatives:**

- **Leave it to the persona** — the status quo: hand-built worktrees cost
  the run's budget and leak refs into the human's repo.
- **`herdr worktree remove --workspace`** — answers only while the workspace
  lives (a cancelled run's is already gone), and the branch survives it
  either way; git reaches both states.
- **A detached checkout** — herdr invents a branch of its own when none is
  named, so a branch is unavoidable; deleting it at teardown is the
  contract instead of pretending it was never created.
- **Forcing the agent's `workspace:` to `worktree` in the runner** — one
  layer that a future caller could skip; the pin lives in the host, where
  provisioning happens.

**Status:** accepted.
