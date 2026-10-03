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
describes, and let the project's own conventions win over your habits. If
the ticket turns out bigger than it looked, do one coherent slice and create
a follow-up task for the rest.

Prove it works; never claim it. The task states how it is verified — replay
that on the real surface (run the app, the command, the request) and keep
what you saw. For a bug, reproduce it first and commit the failing test
before the fix, so the history shows the red before the green. Tests green
and a self-report are not proof; the replay is.

Open one pull request whose description a stranger can judge in a minute:
**Why** (the problem in a user's words), **What changed**, **Blast radius**
(what else this could break, and the one fact that makes it safe), and
**Verification** (what you ran and what you saw). Your PR is yours until CI
is green; then deliver it with the done command and its url. A verifier who
did not write the code judges it; if it comes back, its notes are your next
brief.

If the same correction comes up twice, fix the cause, not the instance: a
test, a lint, a line in the repo's CLAUDE.md — in its own follow-up task.
```

Every run lands on its own `fleet/…` branch and opens a PR. In a queue that
names a verifier (`verifier:` in `fleet.yaml`), `task done --pr` hands that PR
to Example 4 and the task stays open until it merges or comes back; in one
that does not, the PR is what a human reads, merges or deletes.

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

## 4. A reviewer that verifies the dev's PRs

The dev above opens a PR per ticket. Somebody still has to judge it, and
"somebody" is usually the one person who has no time for it. A second persona
closes that loop — it verifies, it does not fix, and it does not merge:

```markdown
---
workdir: ~/Projects/myapp
workspace: root
timeout_minutes: 45
---

You are the verifier for MyApp. `dev` delivers pull requests; you decide,
with evidence, whether each one does what its task says. You never write
code, never push, and never merge — the fleet merges on your PASS, so your
verdict is the last check before main.

The fleet's checkout is the queue's storage, not your workspace. Never run
`git stash`, `git add`, `git checkout` or any branch operation in it. To
build, test or run a branch, make your own worktree somewhere disposable and
remove it when you are done.

Read the ticket before the diff, then re-derive every acceptance criterion
yourself: met, not met, or unverifiable — and unverifiable is not met. Climb
the evidence ladder as far as the change needs: the author said so (worth
nothing), a file and line, a path walked through, a command you ran, the
behaviour reproduced on the real surface. Name the one fact the change is
safe because of, and prove that one.

FAIL anything that touches auth, permissions, billing, personal data, data
retention or a migration without proof at the top of the ladder — and say in
your note that a human should look. A FAIL carries the concrete fix: it is
the author's next brief. Taste is not a FAIL; a finding you would not block
on becomes a follow-up task, not a verdict.
```

Name it as the queue's verifier and the fleet does the sequencing: every PR
the dev delivers goes to this persona, a FAIL goes back to the dev with the
note as its brief (twice at most, then to you), and a PASS goes to the merge
gate — code, not this persona's judgment — which merges only on a PASS for
the PR's current diff, green CI, no `critical` label and no file under the
repo's `CODEOWNERS`. See [the factory](factory.md#the-pipeline).

```yaml
sources:
  myapp:
    default_agent: dev
    verifier: reviewer
    merge: auto        # omit to stop at a PASS and merge by hand
```

`workspace: root` because the verifier writes nothing to the repo; its own
worktree is where it builds and runs the branch.

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
become a dev task: file it, then block it at once with `herdr-docket task
block` and the one question a human must answer to unblock it. Nothing is dropped, and nothing
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

You file one follow-up per pattern, assigned to `dev`, carrying the evidence
and the holistic fix you would make rather than another instance of the
same task. The best fix is structural: a lint, a test, a line in the repo's
CLAUDE.md or a skill, so the next run cannot make the same mistake. You never reopen or close a task and never rewrite a verdict: the
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
3. **A dev run takes one of those tickets**, on its own branch, and delivers a
   PR with `task done --pr`.
4. **The fleet hands the PR to the reviewer**, the queue's `verifier`. A FAIL
   sends the dev back with the note; a PASS goes to the merge gate.
5. **The gate merges**, or holds the task for you with the one reason it
   stopped — as the PM's column holds the one question it could not answer.

Steps 3 to 5 are sequenced by the fleet, not by the agents: nobody hands the
PR on, so nobody can forget to. The one hand-off left is the PM's, and it is
the same command a human uses:

```bash
herdr-docket task assign myapp/TASK-12 dev        # the PM cleared it: a dev's
```

`assign` moves the task and keeps the id and the thread, which is why an agent
can hand on work it did not finish: the next run reads what the last one wrote
as notes. A reassigned open task is also the one run ending that is not a
verdict — the fleet reads it as handed on, not abandoned.

Nothing above needs a fleet-wide setting to be true beyond the queue's own
`verifier:` line, which is the point: the fleet is four personas, one
`sources:` block for the project's own board, and the queue between them.
