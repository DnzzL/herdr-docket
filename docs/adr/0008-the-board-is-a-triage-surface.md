# The board is a triage surface

The overlay pane began as a task list that could start a run, and TASK-24 asked
what it is: a list, or the fleet's dashboard. It is a third thing. A person
watching a fleet has one job — deciding what the fleet does next — and a board
that grows past that job becomes a worse copy of the backend's own interface,
which is the web UI this pane exists to make unnecessary.

**The board shows what runs next, what is running now, and can act on both.**
Everything below is that sentence applied to a decision that needed one.

**A running task is a group, not a second section.** A task whose latest
history record is `running` is shown in a `Running` group ranked above `To Do`,
and shown nowhere else. A task listed twice is a task with two places to press
a key, and the keys on that row — stop it, jump into it — are the ones a person
watches a run in order to press. A record older than the agent's own
`timeout_minutes` is marked stale rather than drawn as busy: the daemon writes
`running` at the start of a run and nothing beats afterwards, so a daemon that
died mid-run leaves a record that never closes, and a control surface showing
that as a live run is telling the one lie it cannot afford.

**The order is the scheduler's order.** `pick.Next` sorts open tasks by priority
descending, then the backend's ordinal, then oldest first. The board calls the
same comparison, so the top row of `To Do` is the task the daemon picks next.
A board that shows the backend's own order instead is a list of work rather
than a queue of work — it was, before this, and the two disagreed silently.
Closed phases stay chronological, because urgency is meaningless once a task
has ended.

**A task is identified by the id the rest of the fleet already uses.** The
composite prefixes every task with its queue's name ([ADR 0003](0003-several-queues-behind-one-port.md)),
which makes the id unique across a fleet of several projects and is the key
every command routes by. The board therefore selects rows by that id and never
by index: the list now reorders whenever a run starts or ends, and a cursor
held by position would land the next keypress on a different task. When the
selected task leaves the list, the cursor takes the nearest surviving row, not
the top.

**The queue a task came from is on the row.** A fleet working several queues
shows them together, and a board that mixes them without saying which is which
makes the reader remember a prefix they can no longer see. A row carries the
queue's name and the task's local id, in a colour that stays with the queue;
a fleet with a single queue shows no such column, because there is nothing to
distinguish.

**Reading a task is the board's job; rendering the backend is not.** The
detail view shows the fleet's `Task` — the body, criteria and notes the prompt
is built from — through the same renderer as `herdr-docket task view`. It does
not render markdown, tick criteria, print the comment thread or follow links.
That distinction is the whole reason this is not a web UI: the fleet knows the
task it hands its agents, and a person deciding whether to spend a run needs
exactly that, not the backend's page.

**The board routes work; it does not judge it.** Reassigning a task writes a
routing key, and creating one names the queue it belongs in. Neither is a
verdict. A verdict is a claim about work with evidence, and this fleet's rule is
that the run reports it ([CONTEXT.md](../../CONTEXT.md)); a human killing a task
is saying `wontfix` or `duplicate`, which are the project's own words and live
on the project's own board. The pane offers no done, no fail, no block.

**Pause is a scheduler fact and it never kills anything.** Pausing an agent
writes `disabled:` into its `AGENT.md`, which takes it out of `pick.Next` within
a tick and leaves every run in flight alone; `herdr-docket run` still reaches a
paused agent, because a person asking is not the scheduler. The board reads the
same fact where the work is listed: a row whose agent is paused carries the word
in its agent column, so a queue that is not moving says why. Stopping a run is a
different action with a different key, and it closes the run's workspace — the
cancellation path that already exists, which ends the task `Blocked` so a human
says what happens next.

**Status:** accepted

## Considered options

- **A fleet dashboard: what is running, what it produced, what needs a
  human.** Rejected as the pane's job, and kept as its own work. Runs produce
  branches, pull requests and output that a person wants, and TASK-24 lists
  them; but they are reports on finished work, and half a dashboard beside half
  a queue is worse than either. The board's job is the next decision.
- **A pinned running strip above the phase list.** Rejected: the same task
  lives in `In Progress`, so a strip duplicates it, and the copy that cannot be
  selected is the one with `x` and `enter` in reach.
- **Showing the full prefixed id instead of a project column.** Rejected: it
  makes every row start with the same characters the colour and column already
  say, and spends the width a title needs.
- **Marking in-flight tasks in place, in their phase group.** Rejected: the
  question "what is happening now" is the one a person opens the pane to ask,
  and making them scan a group to answer it costs more than a header does.
- **Rendering the backend's page: markdown, criteria boxes, the comment
  thread.** Rejected: it is the web UI, arriving one field at a time, and the
  adapter's own plain-text conversion is lossy where the backend is rich.
- **Human verdicts from the board.** Rejected: it competes with the run's own
  report and duplicates a vocabulary the fleet deliberately does not own.
- **A global fleet pause, separate from per-agent.** Rejected for now: it needs
  persisted state of its own and a second answer to what pause does to a run in
  flight, to duplicate what pausing the agents you have already does.
- **Sorting rows by project so a project's work sits together.** Rejected: the
  order would lie about urgency. Filtering by project is what the search field
  is for.

## Consequences

- **The pane can now change state, and three of its keys are irreversible or
  nearly so.** `x` closes a workspace, `s` rewrites a task's routing, `p`
  rewrites an agent's persona file. None of them confirm; pressing the key
  again is the recovery. This is why the writes landed after the read-only
  work rather than with it.
- **A task is shown in `Running` even when its record is stale**, and the mark
  says so. It stays there until something closes the record, which the daemon
  does on the next run of that task.
- **The detail view scrolls a line at a time and no further.** No pager, no
  follow-a-link, no markdown, no comment thread: a body longer than the screen
  is read with `j`/`k`, and a task whose reading needs more than that belongs
  in the backend.
- **Stopping a run is not instant, and the board does not pretend it is.** The
  pane closes the workspace; the runner notices, reconciles the task to
  `Blocked` and writes history. What the pane reports is what the close found —
  `closed TASK-12's workspace`, or `its workspace is already gone` — rather than
  a promise about a run it cannot see. A workspace that is gone while the record
  still says running is the daemon's to reconcile, and until it does the row is
  all the board has to go on.
