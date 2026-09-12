# Several queues behind one port, addressed by a prefixed id

A fleet that works more than one project needs more than one queue, and the
port (`work.Source`) holds exactly one. Rather than grow a plural port, a
composite Source multiplexes named sub-sources and keeps the singular one: the
daemon, `pick`, the runner, the board and the CLI are unchanged, because they
were always written against `work.Source` and that is still what they hold.

Two decisions came out of it, and both are visible to a user, so both are
recorded here.

**A task's id carries its source.** The composite prefixes every id with its
source's name — `myapp/TASK-12`, `bc/987654`. The prefix is not decoration; it
is the routing key. `Get`, `Comment`, `Close` and `SetPhase` split on the
first slash and dispatch on it, and a prefix naming no source is an error, never
a silent no-op. The id is the one value that travels everywhere the fleet
carries a task — the prompt, the board, history, an agent's follow-up — so the
routing belongs in it.

**The composite is a Source like any other**, and passes the adapter
conformance suite. That means it implements `Create`, even though choosing a
queue is always explicit in the real fleet: with a single sub-source `Create`
targets it, and with several it refuses, because guessing which queue a bare
`Create` meant is the silent misroute this arrangement exists to avoid.
Creating into one of several queues goes through `CreateIn`, the
`work.MultiSource` capability the CLI's `-s/--source` speaks.

**A named queue is a prefixed queue even when it is alone.** The singular
`source:` block is unchanged and produces no prefix, so every existing fleet
keeps its ids. `sources:` always produces a prefix, even with one entry,
because a name is a name. The two blocks are mutually exclusive.

**Status:** accepted

## Considered options

- **A plural port (`Sources() []Source`).** Rejected: every caller above the
  port — `pick`, the runner, the pane, the CLI, the daemon's one-source-per-tick
  rule — would learn a new shape, to move one dispatch out of the composite and
  into each of them.
- **A source-name field on `Task`.** Rejected: the port's vocabulary is
  backend-blind, and a field the prefix already carries would be a second
  source of truth that can disagree with the id.
- **Dispatch by a global id-to-source map.** Rejected: one more thing to keep
  in sync, and it hides the routing in state instead of in the id, where every
  surface already has it.

## Consequences

- **`List` costs one request per sub-source per poll.** ADR 0002 holds per
  source, so N sources are N pages: the daemon's fifteen-second poll and the
  board's five-second poll each pay N requests, not one. That is the honest
  cost of N queues, and it is written down so a slow poll has a known cause
  rather than a rediscovered one.
- One sub-source failing fails the whole `List` and skips the tick. A poll
  that hid one queue behind another's silence would be worse than one that
  stops.
- The prefix is a contract with agents too: a follow-up `task create` must
  carry `-s`, and the prompt derives it from the current task's id so the agent
  never has to know the name exists.
- An id that itself contains a slash would be ambiguous. No adapter produces
  one — Backlog.md ids are `TASK-12`, Basecamp ids are numeric — and this is
  where to look if one ever does.
