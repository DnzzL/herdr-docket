# Assign hands work on; a clean re-route is a handoff, not abandonment

A PM that specs work and a dev that builds it had no way to pass a task between
them. The only handoff was "close this one and create a near-copy", which splits
one subject across two ids and loses the thread. `task assign <agent>` is the
handoff: the run ends without a verdict, the task stays open with its whole
history in one place, and the fleet routes it to the new agent on the next tick.

**The capability is optional and says so when it is missing.** `work.Assigner`
is like `work.Phaser` — a capability a source may have — but with the opposite
failure. A source that cannot show a phase is quieter for it; a source that
cannot re-route says so, because a re-route that silently did nothing would
leave the task with the wrong agent and look like it worked. Backlog.md replaces
the assignee rather than appending: the routing rule reads one agent off a task,
so a second name would never be routed to.

**A task assigned to somebody else after a clean run is handed on, not
abandoned.** `reconcile` used to close any task left open as `Failed`, so a PM
that handed one over got it closed under its feet and the next tick never saw
it. Now a task that is open, reassigned, and left by a run that did not crash is
read as handed on: it keeps its history, its workspace is torn down like a
`Done` (the next agent opens its own), and the next tick routes it. A run that
reassigned and *then* crashed is still an unreported run, and still gets the
verdict the mechanics imply.

**The verb is taught where an agent will read it** — in the prompt and in the
`task` skill — because an agent that does not know it exists will keep filing
near-copies.

**Status:** accepted

## Considered options

- **Close the task and create a follow-up copy.** Rejected: it splits one
  subject across two ids, and the new task starts without the first run's
  history — the exact thread the handoff exists to keep.
- **Append the new agent to the assignee.** Rejected: the fleet routes by one
  agent per task, so the appended name would never be picked up.
- **Close the handed-on task as `Done`.** Rejected: none of the work is done.
  The verdict word would lie, and the task would leave the open queue the new
  agent draws from.
- **Treat every open task as unreported, including a re-route.** Rejected: it
  closes the task the PM just handed to the dev, which is the bug this ADR
  fixes rather than the behaviour it chooses.

## Consequences

- A run can now end without a verdict, which the prompt calls out as the one
  way to leave a run besides closing the task.
- `reconcile` returns two values — the verdict and whether the task was handed
  on — because the caller's workspace decision (`cleanup`) differs: a handoff
  tears down like a `Done`.
- The history record for a handoff carries no verdict: the run finished
  cleanly, but the subject did not end. `StatusDone` with an empty verdict is
  the honest record.
- The port's vocabulary does not grow a "handed on" state. `Open` is still the
  only thing the fleet reads, and `Assign` writes only the assignee.
