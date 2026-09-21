# Worked examples

[Back to the README](../README.md)

Four agents that run in production, each one a complete `AGENT.md` plus the
queue wiring it needs. [Writing an agent](agents.md) is the reference — the
fields, the runtimes, the budgets. This is what it looks like in use.

They compose: the PM clears a column, the dev works what the PM cleared, the
reviewer gates the dev's PRs, and the marketer owns something long-lived. The
last section is the four of them as one fleet.

## 1. A PM that triages your project's backlog

The fleet queue routes agents; your project's own `backlog/` tracks its dev
work. A PM agent bridges the two — this one is in production on a real app:

```markdown
---
workdir: ~/Projects/myapp
workspace: root
timeout_minutes: 30
---

You are the product manager of MyApp, built by ONE person in their spare
time. Your bias is ruthless focus: a small app that does its job well beats
a big app that does everything badly.

Triage this repo's backlog (`backlog` CLI from the repo root). Tasks sit in
`needs-triage`; route each to exactly one of:

- `wontfix` — out of scope, or overkill: value doesn't justify complexity.
- `ready for agent` — a coding agent can do it autonomously: clear scope,
  testable, cheap to revert. Sharpen the acceptance criteria while you're at it.
- `needs human validation` — real product judgment required. State the ONE
  question the human must answer to unblock it.

Read the relevant code before judging effort — don't guess. Every verdict is
a comment with 2–4 sentences of reasoning. Never delete a task; never write code.
```

`workspace: root` because triage *is* the project's own backlog: the columns it
moves are files in the checkout, and a column moved inside a disposable worktree
goes away with the worktree.

The wiring is what makes the triage mean something. The project keeps its own
words, and the fleet's `todo` names the one column it is allowed to pick up:

```yaml
sources:
  myapp:
    dir: ~/Projects/myapp
    statuses:
      todo: ready for agent        # what the PM clears is what the fleet works
      done: done
      failed: needs human validation   # the fleet's "a human now" is your word
      blocked: needs-info
```

`needs-triage`, `wontfix` and everything else on that board stay yours: a status
the fleet has no name for is not its business. See
[Where the queue lives](queues.md).

Then, whenever the inbox fills up:

```bash
herdr-docket task create "Triage the backlog" -a pm \
  -d "Route every needs-triage task, and say why in a comment on each one."
```

First real run of this agent: 4 tickets triaged with `file:line` evidence,
acceptance criteria tightened, one description rewritten because it named the
wrong error type — it had read the code. A follow-up audit run re-checked all
35 tickets against the code and caught two "ready" tickets whose work had
already landed.

## 2. A dev that works `ready for agent` tickets

```markdown
---
workdir: ~/Projects/myapp
workspace: worktree
timeout_minutes: 90
---

You are a senior developer on MyApp. Pick up exactly the work your task
describes — the PM has already scoped it. Tests green before you stop, and
the project's own conventions win over your habits. If the ticket turns out
to be bigger than it looked, do one coherent slice and create a follow-up
task for the rest.
```

Every run lands on its own `fleet/…` branch and opens a PR. That PR is the
artifact a human — or the reviewer in Example 4 — reads, merges, or deletes.

The dev is also the example of a run that *hands on* instead of expanding: the
follow-up task it creates for the rest of a big ticket is work it deliberately
did not do, and the queue is where it went instead of a longer run.

## 3. A marketer that owns a strategy

Long-lived missions live in the persona, not in tasks:

```markdown
---
workdir: ~/Projects/myapp
workspace: root
---

You own MyApp's publication strategy. Read docs/strategy.md first — it is
your memory between runs; update it when the plan changes. Tone: concrete,
no superlatives. When a task is too big for one run, split it into tasks
assigned to yourself — the queue is your plan.
```

Seed it once — *"Write the publication strategy, then decompose it into
tasks"* — and it fans out: each follow-up run is one concrete step (draft the
Show HN post, prepare the launch thread…), created by the agent itself.

An agent that splits its own work is exactly the loop `runs_per_day` and
`minutes_per_day` exist for; see [Budgeting an agent](agents.md#budgeting-an-agent).

## 4. A reviewer that gates the dev's PRs

The dev above opens a PR per ticket. Somebody still has to read it, and
"somebody" is usually the one person who has no time for it. A second persona
closes that loop — it verifies, it does not fix:

```markdown
---
workdir: ~/Projects/myapp
workspace: root
timeout_minutes: 45
---

You are the code-review gate for MyApp. `dev` implements tickets on `fleet/…`
branches and opens PRs; you say in public whether the result is fit to merge.
You never merge, and you never write code.

A review is worth exactly its evidence: every claim names a file, a line, or a
test you ran. Read the ticket before the diff, then re-derive every acceptance
criterion yourself — never trust the author's checkboxes. Met / not met /
unverifiable, and unverifiable is not met. Correctness and scope are hard
gates; taste is not, and a finding you would not block on files no task.

The verdict is a PR review with one `VERDICT:` line. Every blocking finding
carries the concrete fix and becomes a follow-up task assigned to `dev` — a PR
comment is not a queue. You may uncheck an acceptance criterion you proved
false; you may not change a ticket's phase or edit the task the author closed.
```

Two shapes of task, both worth seeding: `dev` hands off one PR per run, and a
*sweep* task — "review the oldest un-reviewed PR, then re-task yourself for the
rest" — clears the pile whenever it grows. The loop is a gate, not a merge
button: an approve-shaped verdict still waits for the human. (One caveat if your
agents commit under your own account: GitHub refuses to let an account approve
its own PR, so put the verdict in the review's words, not its state.)

`workspace: root` here for the same reason as the PM's: the follow-up task it
files is a change to the queue, and the queue is the checkout.

## Composing them into one fleet

The four are one system, and the coupling between them is a single queue — no
agent knows another's name beyond the `assignee` it writes, and no agent holds
credentials for the backend:

1. **You, or your project's own board, puts one task on the PM** — or the
   queue's `default_agent` does it for you: unassigned work goes to the PM
   because unassigned means unspecced, and the PM is the one that specs it.
2. **The PM clears a column.** Of the three verdicts it can reach, exactly one —
   `ready for agent` — is a status the fleet is allowed to pick up.
3. **A dev run takes one of those tickets**, on its own branch, and opens a PR.
4. **A reviewer run reads the PR**, posts a verdict, and files every blocking
   finding as a follow-up task assigned to `dev`.
5. **The human merges**, or answers the one question the PM said was blocking.

Steps 3 and 4 are where work changes hands, and the command is the same one
both times:

```bash
herdr-docket task assign myapp/TASK-12 dev        # the PM cleared it: a dev's
herdr-docket task assign myapp/TASK-13 reviewer   # a PR is on the pile: review it
```

`assign` moves the task and keeps the id and the thread, which is why an agent
can hand on work it did not finish: the next run reads what the last one wrote
as notes. A reassigned open task is also the one run ending that is not a
verdict — the fleet reads it as handed on, not abandoned.

Nothing above needs a fleet-wide setting to be true, which is the point: the
fleet is four personas, one `sources:` block for the project's own board, and
the queue between them.
