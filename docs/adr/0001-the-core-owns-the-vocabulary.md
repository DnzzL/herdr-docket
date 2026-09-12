# The core owns the vocabulary; a backend translates

A task's *phase* is shown to a human, so the core owns the words and every
adapter maps its backend's labels into them. A task's *priority* is read to
decide which task runs and shown to nobody, so the port carries only a rank the
adapter computes and the core owns no priority word at all. The rule that
settles both: a vocabulary the core must **show** belongs to the core; a value
the core must only **order** belongs to whoever understands it.

**Status:** accepted

## Considered options

- **The core owns a priority vocabulary too, mirroring phases.** Rejected: four
  urgency levels that are never shown and only ever compared is vocabulary for
  its own sake. Phases needed core words because a human reads them.
- **Fold urgency and position into one ordering key the adapter produces.**
  Rejected: it compresses two distinct concepts into an opaque number, and hands
  each adapter the "does urgency outrank position?" decision — so two backends
  could disagree about the fleet's own ordering policy.
- **Carry creation time as a time.Time instead of a documented string.**
  Rejected: parsing adds a failure mode (a bad date that today sorts oddly would
  become an error that drops work) and forces a timezone guess on Backlog.md's
  local-time format. A documented promise that the bytes sort chronologically
  gets the same correctness with neither cost.

## Consequences

- A third backend's obligations are unchanged: satisfy the ten contract
  subtests. Phase was and remains display-only — nothing in the port promises a
  phase survives or that any particular word exists, and unknown words still
  render, after the ones the fleet knows.
- The closed entries in the phase ordering table are derived from the verdict
  vocabulary rather than hand-written, so the three endings cannot drift
  between the two lists.
- A verdict has two spellings on purpose: the machine word an agent types
  (`done`) and the label the board shows (`Done`), joined in one place.
