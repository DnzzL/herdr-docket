# A doubted delivery blocks, and git is read behind the host port

The failure that wrote this ADR: a run ended `done` with an accurate note and
every acceptance criterion argued, and its branch had zero commits — the work
existed only as uncommitted files in a worktree that was torn down the same
moment. The fleet reported success (TASK-23).

**The signal is loss, not intent.** "Done means commits" is wrong for a
`workspace: root` agent — editing the shared checkout uncommitted is its design,
and the human decides what lands in git — and for any run whose deliverable is
not code: a PM triaging a board, a reviewer posting a verdict, research. The one
state that cannot be legitimate is a disposable worktree about to be destroyed
holding changes nobody committed. Either those changes mattered, and destroying
them is the bug, or they did not, and the agent littered a checkout it was told
to work cleanly in. Both readings are the fleet's business.

**The check lives where the loss happens.** The runner's `cleanup` is the one
place that knows the verdict and the workspace at the same time, so the guard
runs immediately before it and can overrule the teardown, not mourn it.

**Git is read behind the host port.** The runner gains `host.Inspect(session)`
and nothing else: it never shells out to git, and never learns how a delivery is
read. The host already owned the workspace — it provisions it on a branch it
names and tears it down — so the facts about that workspace are its answers,
with the git plumbing in the host's own ops port and scriptable fakes above it.
A runner that executed `git status` itself would be a second owner of a
checkout it does not provision.

**A verdict nobody can verify is kept, but the workspace is not destroyed.**
Three outcomes:

- **Dirty worktree, run ends done.** The workspace is kept, the task is moved
  to `blocked` — the human-decides column — and both the task note and the run
  record say what was found. The alternative, overriding the verdict to
  `failed`, threw away real information: the agent may have committed the work
  to the base branch, or delivered through a mechanism git cannot see from one
  worktree. A doubted `done` is not a `failed`; it is a `done` a human has to
  confirm or refute in a workspace that is still there to be confirmed in.
- **Dirty worktree, run hands the task on or ends otherwise than `done`.** Same
  kept workspace, same note, no verdict change: the task is already going
  somewhere, and the note is for whoever picks it up.
- **The delivery cannot be read at all.** Workspace kept, verdict untouched,
  note says `unverified`. Blindness is not evidence, and refusing to guess is
  the same honesty that applies when git *can* be read.

**The second signal is recorded, not acted on.** A worktree run that ends
`done` with a clean tree and no commits ahead of its base is sometimes
legitimate (a review sweep, a triage whose deliverable is a note) and sometimes
an empty delivery. The record already states commits — `+0` shown, not omitted —
so the weaker signal is readable in history, but it never overrides a verdict:
there is no fact of loss to act on.

**The workspace mode infers what the run declares; agents do not.** There is no
`expects_commit` persona field. Root mode opts every agent on it out of the
guard, because its edits are the repo's own state by definition; worktree mode
opts every agent on it in, because its checkout is dispensed and destroyed per
run. A field for the agent to promise would be a claim the guard exists to
check independently — the check reads git precisely so it does not have to
believe one.

**Consequences:**

- A kept workspace keeps its slot: the same name collision a retry already
  survives exists until a human enters the workspace and resolves it, or closes
  it. That is chosen: a silently destroyed worktree was the bug.
- A queue that refuses a second close on a run the agent already made keeps its
  original verdict and still gets the note — the report is doubted either way,
  the queue simply will not carry the correction.
- Every worktree run is read before teardown now, one extra question per run
  against the host, which is where reading it was already possible and cheap.

**Status:** accepted, retroactive to the v0.7.0 release that shipped the guard
(ADR written when TASK-23 was verified against it). Rejected along the way:

- **Done means commits.** Wrong for root mode, reviewers and PM runs (see the
  signal above); the guard would have lied about most of the fleet.
- **Override the doubted verdict to `failed`.** Destroys more information than
  it adds; `blocked` says "a human decides" which is what has to happen.
- **Git state read by the runner with `exec.Command`.** A second owner of a
  workspace the host provisions and tears down, and an `exec.Command` seam the
  suite cannot script.
- **`expects_commit` persona field.** An agent's self-report replacing the
  mechanical fact the check exists to read.
- **Record the empty-delivery signal and skip the verdict.** No loss to catch;
  history's commit count already carries it.
