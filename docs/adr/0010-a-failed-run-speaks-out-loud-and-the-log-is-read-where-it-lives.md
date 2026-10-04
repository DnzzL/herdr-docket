# A failed run speaks out loud, and the log is read where it lives

The failure this answers: a morning sweep started a run while nobody had the
fleet open, the agent never started, and the task sat in Failed until someone
happened to look. The record in `herdr-docket history` carried the truth the
whole time — that is TASK-23's work, and it is why the facts exist — but a
record is only ever read by somebody who came back. A failed run asks for the
one gesture a record cannot make: making itself heard.

**The channel is Herdr's own notification, asked for once.** `herdr
notification show` already exists and is already pointed at the human running
the fleet; the plugin adds a second channel — email, webhook, anything — the
day it also adds a place to configure it, a queue to drain, and a rate limit.
The fleet reports through the host rather than owning a pipe, the same way it
copies no queue state ([CONTRIBUTING.md](../../CONTRIBUTING.md)). Rejected
alternative: a `herdr-docket notify` verb every caller could forget.

**It travels behind the host port.** The runner gains `host.Notify(title,
body)` and nothing else: it does not know the CLI invocation or the position,
and it never learns that a notification can be refused — the
refusal is logged and the run's verdict already stands. A caller that
re-decides anything on a notification's failure is mis-specified. The fake
hosts record what was asked for, so "a failed run asked exactly once" is a
test, not a review. (Amended by ADR 0015: the sound did join the call —
which sound a stop deserves is policy, so the runner names it and the port
still owns the invocation.)

**Only failed runs speak.** Done is the fleet working as designed — a board
watched by a person with the fleet open; a notification on every success
teaches that person to mute the channel, and then the failure it exists for
is muted too. Cancelled is a human deciding, and the person deciding is
watching. The threshold is the status `failed`, whatever produced it: a
mechanics error, a provisioning failure, or an agent that settled without
reporting a verdict — the fleet judges the run failed by the words it already
wrote in the record, and says them again in the notification's body.

**The log is read, not mirrored.** The daemon's stderr has been redirected
into the state dir `daemon.log` by the plugin since it started existing; the
CLI now tails it (`herdr-docket logs`) instead of anyone remembering the path.
A second capture of the daemon's output — the runner copying lines into
history, a log command with its own file — was rejected rather than built:
one log, one owner, one reader surface.

**The live output stays where the board already put it.** Enter on a running
row focuses the workspace and the agent's pane, from the workspace and pane
ids the run record carries — no generated agent name is ever needed. A CLI
that would replay pane output as text was considered and rejected: it is
`herdr pane read` with a wrapper around it, and the output it belongs to is
live by definition. The pinned test traces the jump's argv at the herdr
process boundary, in the pane package, to keep "without discovering a
generated name" a property under structure, not a paragraph.

## Consequences

- **A notification that cannot fire loses nothing.** The verdict, the record
  and the task note are the run's report; the notification is the run's
  voice. Muted, broken or absent, the audit trail is whole.
- **The fake hosts grew one method.** `Notify` joins the port, so any future
  Host implementation answers for its own channel or returns an error the
  runner logs.
- **A failed run's body is the record's sentence.** When the fleet learns to
  write a better failure, it edits one place.
