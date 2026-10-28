# The repo's own tracker is the queue, and the spec is the parent task

The failure this answers: a card lived in one store (a Backlog.md project in
the fleet dir, or one of four adapters) and its spec in another
(`.scratch/<slug>/` in the repo). Two stores to read, two to keep in step, and
four adapters to keep alive for a plugin one person runs. Each adapter cost
code, tests and docs that no user of this fleet read.

**The queue is the project repo's own tracker: Backlog.md or GitHub.**
Backlog.md is the local default; GitHub is the hosted one, kept for adoption.
Basecamp and Notara are removed. A Backlog.md source names its repo with
`source.dir`; there is no queue in the fleet dir any more.
`herdr-docket init <repo>` sets the repo up: `backlog init`, the fleet's
statuses, and a `docs/agents/issue-tracker.md` that tells Pocock's skills where
to publish.

**The spec is the parent task; its tickets are subtasks.** `to-spec` writes
the spec as a task (an issue on GitHub) assigned to an agent: that task is the
card. `to-tickets` writes each ticket as a subtask (a sub-issue), with its
blockers as dependencies. ADR 0021 holds: the card is the only task that
triggers a run, and the tickets are the implementer's plan.

**`work.Task` carries `Parent`, and the pick skips any task that has one.**
That is all the daemon knows of the graph: enough to never route a ticket as
a card. Dependencies stay the implementer's business.

**Rejected.** *Keep four adapters*: cost without a user. *A fleet-dir queue
with specs in `.scratch/`*: two stores for one piece of work, and `.scratch`
is invisible to `backlog browser` and to GitHub. *A structured `Spec` field on
`work.Task`*: the role reads the spec, not the daemon, so a parent link is
enough. *Tickets in a status the pick ignores*: depends on every repo's status
words, where a parent link holds on any board.

Amends [0021](0021-the-unit-of-work-is-a-spec-run-by-one-implementer.md): the
spec and its tickets move from `.scratch` into the repo's tracker. Supersedes
[0022](0022-a-notara-database-is-a-queue-and-the-status-table-is-shared.md).

**Status:** accepted
