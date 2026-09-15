# herdr-docket

A shared task queue worked by your coding agents. The queue lives in a backend
(a Backlog.md project, a Basecamp project or a GitHub Projects project) and the
fleet keeps no copy of it.

## The queue

**Queue**:
The shared list of work the fleet draws from, living entirely in one backend.
It is the source of truth: the fleet reads it, writes to it, and keeps no
mirror of it.
_Avoid_: backlog, board, sprint

**Task**:
One unit of work in the queue.
_Avoid_: item, ticket, card, issue, to-do

A backend with a vocabulary of its own keeps that word inside its adapter — a
GitHub adapter says *issue*, because that is what GitHub calls the thing a
person will go and read — and nothing above the adapter uses it.

**Open**:
A task that still owes work. The only thing the fleet reads to decide whether
to pick a task up.
_Avoid_: unfinished, active, pending, in-progress

**Phase**:
Where a task stands, in the fleet's words: _To Do_ and _In Progress_, plus the
three endings it shares with Verdict. Display only — nothing about a phase
decides what runs.
_Avoid_: status, state, column

**Verdict**:
How a run ended its task: _done_, _failed_ or _blocked_. Best-effort — a
backend that cannot record one leaves it empty.
_Avoid_: result, outcome, resolution

**Priority**:
How soon a task wants to run, as the queue backend ranks it. The fleet carries
the rank and never the backend's word for it.
_Avoid_: severity, urgency, weight

**Criteria**:
A task's acceptance criteria. Read-only to the fleet: a run never edits them,
and the verdict and its evidence go in the closing note.
_Avoid_: checklist, steps, subtasks

## The workers

**Fleet**:
The named agents that work one queue.

**Agent**:
One named worker: the parameters a run needs, plus the persona the prompt
opens with.
_Avoid_: worker, bot

**Assignee**:
The agent a task names as its owner. A task naming nobody goes to the default
agent when one is configured, and waits for a human otherwise.
_Avoid_: owner, runner

**Run**:
One execution of a task by an agent. Strictly one at a time per checkout.
_Avoid_: job, session, execution

**Board**:
The interactive view of the queue.

**Project**:
What a backend calls the container its queue lives in: a GitHub Projects v2
project, say, which is the queue itself rather than a view of it. A backend
whose queue is a board says *board* in its own documentation, because that is
what its users call it; the fleet's _Board_ is always the view.
