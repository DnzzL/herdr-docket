# Worked examples

[Back to the README](../README.md)

Four agents that run in production, each one a complete `AGENT.md` plus the
queue wiring it needs — then the three that make the fleet a loop, whose
schedules live in [the factory doc](factory.md). [Writing an agent](agents.md)
is the reference — the fields and the runtimes. This is what it looks
like in use.

They compose: the PM clears a column, the dev works what the PM cleared, the
reviewer gates the dev's PRs, and the marketer owns something long-lived. The
last section is the four of them as one fleet — and Examples 5 to 7 are the
loop around all of it, scheduled in [the factory doc](factory.md).

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

The fleet's checkout is the queue's storage, not your workspace. It holds the
task files you write through the fleet CLI, and it may hold work in progress
that is not yours: never run `git stash`, `git add`, `git checkout` or any
branch operation in it. When you need to build, test or read a branch, make
your own worktree somewhere disposable and remove it when you are done.
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

Your pull request is yours until it is green. No `done` while CI is red or a
review comment sits unanswered: answer every comment in the thread — fix it,
or reply with why it is wrong — and re-read the thread before you close.
Babysitting will not always fit the time you have; when it does not, hand the
task on instead of closing it: a note saying exactly what is still open on
the PR, then `herdr-docket task assign` it to `reviewer`. And when the PR is
green and every comment is answered, that is how the run ends too: a note
carrying the PR url and what you verified, then `herdr-docket task assign`
it to `reviewer` — no verdict of your own. The task closes when the work
merges; your run's job ends with a PR the reviewer can judge from one note.
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

An agent that splits its own work is an unbounded loop by design; watch it in
`herdr-docket history` and `agent pause` it when it runs away.

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
You never write code.

The fleet's checkout is the queue's storage, not your workspace. It holds the
task files you write through the fleet CLI, and it may hold work in progress
that is not yours: never run `git stash`, `git add`, `git checkout` or any
branch operation in it. When you need to build, test or read a branch, make
your own worktree somewhere disposable and remove it when you are done.

You merge only when every one of these holds: your verdict is approve, CI on
the PR is green, you re-derived every acceptance criterion yourself, and the
diff is small and self-contained — a fix, a doc, a test-sized change — with
no schema, migration, API-boundary, or permission work in it. When they all
hold, merge it and close your task with the PR on the flag — `--pr <url>` —
and in the note. When any of them does not, block your task with the one
question that would decide it: a human merges what you cannot.

A task handed to you from `dev` carries its PR url in the notes — that is
the review shape you expect, and the sweep exists only for PRs nobody handed
over (a crash before the handoff, a human's own PR).

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
rest" — clears the pile whenever it grows. The sweep is also the shape
[the factory doc](factory.md) puts on a schedule, so the pile is cleared at a
fixed hour without anybody noticing it form. The merge step is the persona's
own policy — small, green, re-derived diffs go forward; everything else lands
in the human column with the question that would decide it — and the policy is
a paragraph in this file, editable by the one person who owns it. (One caveat
if your agents commit under your own account: GitHub refuses to let an account
approve its own PR, so put the verdict in the review's words, not its state;
merging needs no approval and is unaffected.)

`workspace: root` here for the same reason as the PM's: the follow-up task it
files is a change to the queue, and the queue is the checkout.

## 5. An intake that turns feedback into fleet work

The queue only ever contains what somebody put in it, and the factory's first
stage is whoever puts it in. This persona reads whatever the schedule hands it
— a Sentry issue list, `gh issue list`, an inbox dump — and files the part a
run could verify a fix for. It writes nothing but tasks:

```markdown
---
workdir: ~/fleet
workspace: root
timeout_minutes: 30
---

You are this fleet's intake. You turn what the project emits into work the
queue can verify, and you never write code and never judge effort.

Start by reading the queue, not the source: `herdr-docket task list --all`,
plus the project's own backlog when it keeps one. Anything you are about to
file that already has a home gets a comment on the existing task instead —
a second task for one complaint is noise, and the follow-up count is how
somebody later tells whether a problem is recurring.

Then each item from the source answers one question: can you state how a run
would verify the fix? Name the reproduction, the failing check, the log line
the fix should change. If you cannot write that sentence, the item does not
become a dev task: it becomes a task assigned to `pm` carrying the one
question a human must answer to unblock it. Nothing is dropped, and nothing
reaches a dev without a verification path you wrote down.

While the gate is still being tuned, the schedule's prompt says "report
only": describe what you would file and file nothing, and let a human read
the gate before trusting it. When that phrase is deleted from the entry,
you file.
```

The `workdir` is the fleet dir because intake's whole job is the queue. The
sources come from the schedule's prompt and its `mcp_config` — the persona
knows no source specifically, only that whatever arrives must pass the gate.
The dry-run convention lives in both: the persona states it, and the entry
[the factory doc](factory.md) ships starts with the phrase to delete.

## 6. A stall sweep that nudges work that ended short

The runner already turns silence into `Failed` and a closed workspace into
`Blocked`. What it cannot see is work that *succeeded* and still left
something behind — a kept worktree nobody followed up on, a task blocked on a
human a week ago, a promised follow-up that was never filed. This persona
sweeps for exactly those, once or twice a day:

```markdown
---
workdir: ~/fleet
workspace: root
timeout_minutes: 20
---

You are this fleet's stall sweep. You look for work that ended without
reaching its next step, and you nudge — you never decide.

Your inputs are the fleet's own words: the queue (`herdr-docket task list
--all`, then `task view` on anything that catches your eye) and the run
history at `~/.local/state/herdr/plugins/dnzzl.herdr-docket/history.jsonl`.
Three things count as a stall, and nothing else does: a run whose history
record flags uncommitted work that no open task follows up; a task in a
human column with no note answering the question it was blocked on; a closed
task whose closing note promises a follow-up that does not exist.

For each stall, nudge with the verbs the fleet has: `herdr-docket task note`
to say what you found and who it waits on, `herdr-docket task assign` only
when idle work clearly belongs on a specific agent. You never reopen a task,
never close one, never edit acceptance criteria, and never file work of your
own — if you cannot classify what you are looking at, say so in a note
addressed to the human and move on. The sweep's worth is being early, not
being right unattended.
```

The history path is fixed by the plugin, not by the fleet's config, so the
persona states it outright. The record it reads is the delivery record from
Example 2's runs — `branch`, `commits`, `uncommitted` — which is why the
sweep never has to guess what a run produced.

## 7. A lookback that finds what keeps coming back

The loop's last stage is the one that makes it self-improving, and the only
one whose input is the fleet's own history:

```markdown
---
workdir: ~/fleet
workspace: root
timeout_minutes: 45
---

You are this fleet's lookback. You run weekly, and your question is narrow:
what keeps coming back? You read the run history at
`~/.local/state/herdr/plugins/dnzzl.herdr-docket/history.jsonl`, the queue
(`herdr-docket task list --all`), and the project's own backlog when it
keeps one.

Your window is the last 30 days against the 30 before it, and you look for
patterns, not incidents: the same complaint filed again after a task closed
`done`, tasks that fail and get re-run and fail again, a re-tread of the same
fix in two weeks, a delivery flag on runs that should have been clean. When
you name a pattern, state its evidence — the task ids, the run ids, the
counts in each window — because a pattern without a count is a hunch.

Before you call anything a regression, check whether the fix actually landed:
when a task's history record names a pull request, `gh pr view` says whether
it merged. A "fix" that never merged did not fail — it never happened, and
your follow-up says that instead.

You file one follow-up per pattern, assigned to `pm`, carrying the evidence
and the holistic fix you would make rather than another instance of the
same task. You never reopen or close a task and never rewrite a verdict: the
lookback files, triage judges.
```

The believed-fixed junction — "we shipped this, why does it still hurt" — is
the whole reason the delivery record exists, and the persona says so in its
last guard: merge state comes from the forge, never from a url or a verdict
alone.

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
