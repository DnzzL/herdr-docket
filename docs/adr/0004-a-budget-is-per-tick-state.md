# A budget is per-tick state; the run log is the only ledger

An agent that keeps creating follow-up work for itself is the design working —
a run hands on what it found instead of expanding its own scope — and it is
also an unbounded loop. `timeout_minutes` caps one run, not the day, so two
optional fields cap the day over a rolling 24 hours: `runs_per_day` and
`minutes_per_day`. What makes a cap safe to leave running is where its numbers
live and who writes them.

**A budget is state, not config.** A spent agent is marked unavailable for the
tick only — exactly like a busy one — and the window rolls it back open on its
own as old runs age out. The fleet never writes `disabled: true` on a human's
file: pausing is a human's word, and a ceiling that edited the config would
need a matching "unpause" nobody asked for and could get wrong. Reaching a limit
spends it; the run that would cross the line waits.

**The window rolls, it does not reset.** A calendar "today" would reset at
midnight and let a self-tasking agent spend the whole allowance twice in a
minute around the boundary. A rolling day has no boundary to be gamed.

**A read failure is an empty window, not an error.** `UsageSince` treats an
unreadable run log as "no spend recorded", which is the safe default for a
budget — the alternative spends against a number the log never carried. Records
written before the fleet recorded an agent or a duration are ignored for the
same reason: an honest zero beats a guessed one.

**The log is append-only and has one writer.** Every transition of a run is
`recorder.append`, so the record shape lives in exactly one place and a budget
counts a ledger it can trust: append-only, tolerant of a torn write, and read as
the latest record per run.

**Status:** accepted

## Considered options

- **Mark a spent agent `disabled: true` in its file.** Rejected: the config is
  a human's, and a ceiling that wrote to it would toggle a human's own word off
  when the window rolled — a silence the fleet invented, not one a person chose.
- **Reset the budget at midnight.** Rejected: an agent that self-tasks spends
  the day's allowance either side of midnight and gets two.
- **Fail the run that would cross the line.** Rejected: nothing broke, and a
  task closed `Failed` for a policy ceiling loses work the agent could still do
  tomorrow. Waiting spends nothing and writes nothing.
- **Count a partial or unknown run as full spend.** Rejected: a record without
  a duration or an agent is from before these fields existed, and counting it
  would spend the budget against a number nobody recorded.

## Consequences

- A budget gates poll-triggered runs only. `herdr-docket run TASK-12` still
  starts an over-budget agent's run, because pressing the button is human
  intent — the same rule that lets a manual run reach a paused agent.
- A spent agent is indistinguishable from a busy one to the queue: its `To Do`
  tasks stay open with nothing written on them. `agent list` and the board's
  `g` view are where the spend is actually shown.
- The budget depends on history being honest, so the two decisions live
  together: the single-writer rule above is what lets `UsageSince` read a number
  it did not have to guess.
