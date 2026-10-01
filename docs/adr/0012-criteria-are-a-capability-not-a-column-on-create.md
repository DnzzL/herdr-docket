# Criteria are a capability, not a column on Create

The failure this answers (TASK-22): `herdr-docket task create` could not carry
acceptance criteria, so an agent handing work on had two ways to say what
"done" means — fold it into the description prose, or reach past the fleet CLI
and call the queue's own tool (`backlog task create --ac`) directly. The second
is exactly what the fleet CLI exists to make unnecessary: a persona writing the
queue by hand bypasses routing, id handling, and the port's contract, and the
next backend pays for it.

**Writing was a widening, and widening was wrong.** Criteria on `Create` would
have joined `title, body, assignee` on the `work.Source` port, which every
adapter must answer for. A Basecamp to-do has no description field for a bar,
a GitHub issue body would grow a second meaning buried in a signature, and an
adapter that cannot comply would have to invent a silent drop — the one
outcome the fleet must never get with acceptance criteria. The port already
had the right shape for "not every queue can do this": optional capabilities
beside `Source` (ADR 0006's `Phaser` and `Assigner`, ADR 0011's `BaseBrancher`).

**The fourth capability follows them.**

```go
// CriterionWriter is the optional capability of a queue that can store
// acceptance criteria on an existing task — the write side of the criteria
// every task already carries read-only.
type CriterionWriter interface {
    WriteCriteria(id string, criteria []string) error
}
```

The id is the task's own — prefixed where the fleet is a composite — so a
`MultiSource` forwards exactly as `Assign` does: find the sub-source, ask it,
and if it has no capability, say so in its name rather than answering for it.

**Capacity decides, per adapter, and each says it out loud.**

- **backlogmd** — `task edit --ac`, one flag per criterion, appended onto the
  freshly created task's (empty) list. Criteria write and read back through
  one field.
- **github** — the issue body's markdown task list is already what the adapter
  reads criteria out of, so writing appends `- [ ]` lines to that body. An
  existing bar is added to, not replaced.
- **basecamp** — a step exists only on the to-do's own POST; there is no
  endpoint and no field. Basecamp therefore does not take the capability: the
  composite and CLI answer for it instead, refusing with a message that names
  where the bar goes — the description — and carrying the words back, so
  nothing drops silently.

**The CLI is where knowledge lives.** `task create` parses repeatable `--ac`
flags, refuses an empty one (an empty criterion stores nothing), creates the
task through the port, then calls `CriterionWriter` once. A queue without the
capability is told at creation, not discovered later by whoever picks the task
up: the words to put in `-d` come back with the error. The agent never needs
the queue's own CLI, which is the whole point — shipped personas no longer
have anything to route around.

**Alternative rejected: widening `Create`.** It makes a capability a column:
every adapter carries a parameter most cannot use, and the failure mode for
those that cannot is exactly the silent drop this refuses to allow.

## Known gaps

- A criterion a backend cannot number is numbered by position; a human
  reordering steps reorders criteria too.
- backlogmd's `--ac` appends on a shared `task edit`; the write is correct
  only because Create precedes it on the same task. A later need to replace
  criteria on an existing task wants the edit verb's replace-all form, which
  the CLI offers one-task-at-a-time.
