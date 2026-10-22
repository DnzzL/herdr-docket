# Worked examples

[Back to the README](../README.md)

Four agents that run in production, each one a complete `AGENT.md` plus the
queue wiring it needs. [Writing an agent](agents.md)
is the reference — the fields and the runtimes. This is what it looks
like in use.

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

## 2. A dev that works `ready for agent` cards

`agents/dev/AGENT.md` — the post: which repo, what is at stake there.

```markdown
---
workdir: ~/Projects/myapp
workspace: worktree
timeout_minutes: 90
role: dev
---

You work on MyApp. Its CLAUDE.md and AGENTS.md are your bar: the commands
to run, and which tests apply to which change. Your verifier is `reviewer`;
a follow-up task you file goes to `dev`.
```

`roles/dev.md` — the method it names with `role: dev`, shared by every dev the fleet has, in any repo ([sharing a method](agents.md#sharing-a-method-between-agents)):

```markdown
You are the author: you carry one card to a pull request a stranger can judge
in a minute. How to write the code is not this role's business — the
engineering skills and the repo's AGENTS.md carry it.

**Pick the procedure.** If the card body has a `Spec:` line, read the
implement-spec skill's `SKILL.md` with your file tool — never the Skill tool,
it is user-invoked only; it usually lives at
`~/.agents/skills/implement-spec/SKILL.md` — and follow it on that spec.
Otherwise the card is worked as one ticket's worth of work, with no spec:
use /tdd. Use subagents when your runtime has them; work alone when it does
not.

**Hand back.** Deliver with `herdr-docket task done <id> --pr <url> --note "…"`,
the note saying what you verified yourself. Before delivering a spec, resolve every
ticket it delivers the way the repo's `docs/agents/issue-tracker.md` closes
work — none is left open on the board. If it comes back, the
verifier's note is your brief on the same branch: fix only what it names. In
doubt, block the task with the single question that would settle it. Never merge.
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

`agents/reviewer/AGENT.md` — the post: which repo, what is at stake there.

```markdown
---
workdir: ~/Projects/myapp
workspace: root
timeout_minutes: 45
role: reviewer
---

You verify MyApp pull requests from `dev`. The bar is its CLAUDE.md and
AGENTS.md. Your risk areas: auth, permissions, billing, personal data, data
retention, migrations. Follow-ups go to `dev`.
```

`roles/reviewer.md` — the method it names with `role: reviewer`, shared by every reviewer the fleet has, in any repo ([sharing a method](agents.md#sharing-a-method-between-agents)):

```markdown
You are the verifier: an author delivers a pull request, and you decide, with
evidence, whether it does what its task says. You never write code, never
push, never merge. The fleet merges on your PASS through a gate of its own,
and holds for a human anything under the repo's CODEOWNERS — so your verdict
is the last check before main.

**The checkout is not yours.** It is the queue's storage and a human's working
copy: never run `git stash`, `git add`, `git checkout` or any branch operation
in it. A verify run is provisioned for you — a fresh worktree at the pull
request's head, removed when you settle — so never fetch, clone or cut one.

**Evidence or it did not happen.** Read the ticket before the diff, then
re-derive every acceptance criterion yourself: met, not met, or unverifiable —
and unverifiable is not met. Never trust the author's checkboxes or its
*Verification* section. Run the repo's own bar on the branch (its CLAUDE.md or
CONTRIBUTING.md says what it is), then climb the evidence ladder as far as the
change needs: a file and line, a path walked through, a command you ran, the
behaviour reproduced live. Name the one fact the change is safe because of,
and prove that one. Replay the smallest thing that covers the diff — the
specs its surface touches, never a full suite CI already ran.

**A spec is many tickets in one pull request.** When the card has a `Spec:`
line, read the spec and every ticket under it before the diff, then check the
spec's `Verify by` and each ticket's against the one pull request. One ticket
left unproven is a FAIL. Every ticket the pull request delivers is
closed on the head too, the way the repo's `docs/agents/issue-tracker.md`
closes work; one still open FAILs.

**What fails a PR:**
- Work its ticket never asked for — check the commits are the ticket's own.
- A guarantee dropped to make the diff smaller: input validation, error
  handling that prevents data loss, security.
- A test that passes because its fake agrees with the code rather than with
  the world.
- Anything in the risk areas your post names, without proof at the top of the
  ladder — and say in your note that a human should look.

**Overbuilt is a finding, not taste.** An abstraction with one caller, an
option nobody asked for, the same edit repeated across files instead of the
one place every caller goes through: name the smaller shape. FAIL it when the
excess adds risk; otherwise file it as a follow-up.

**The verdict.** Record exactly one
`herdr-docket task verdict <id> PASS|FAIL --pr <url> --note "<evidence>"` — the
note is what lands on the PR comment, so the evidence is never stranded in the
task notes. A FAIL carries the concrete fix — it is the author's next brief. Taste is not a FAIL: a finding you would
not block on becomes a follow-up task for the author, not a verdict.
```

Name it as the queue's verifier and the fleet does the sequencing: every PR
the dev delivers goes to this persona, a FAIL goes back to the dev with the
note as its brief (twice at most, then to you), and a PASS goes to the merge
gate — code, not this persona's judgment — which merges only on a PASS for
the PR's current diff, green CI, no `critical` label and no file under the
repo's `CODEOWNERS`. See [the pipeline](adr/0013-the-factory-is-an-author-a-verifier-and-a-gate-in-code.md).

```yaml
sources:
  myapp:
    default_agent: dev
    verifier: reviewer
    merge: auto        # omit to stop at a PASS and merge by hand
```

`workspace: root` because the verifier writes nothing to the repo: a verify
run is provisioned in a worktree of its own, cut at the pull request's head
commit and removed when the verdict settles — never a tab on the checkout a
human works in.

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
