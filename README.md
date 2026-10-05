# herdr-docket

**A shared task queue worked by your coding agents — and the software
factory that runs on it.** A [Backlog.md](https://backlog.md)
project as the queue — or Basecamp, or a GitHub Projects board, if that's where
your work already lives (see [Where the queue lives](docs/queues.md)) —
`AGENT.md` personas as the workers, and a daemon that routes every open task to
the agent it names — agents in parallel, one run each, in
[Herdr](https://herdr.dev) workspaces you can watch, join, or close.

*docket*: the list on the wall of the work a crew will get to — what the queue is, all of it.
Part of the [Herdr plugin family](https://herdr.dev/docs/plugins/).

Wired end to end it is a minimal software factory: intake turns feedback into
tasks that say how they will be verified, an author agent carries each one to a
green pull request, a verifier that did not write it re-derives every claim,
and a merge gate — code, not a prompt — merges what passed and holds the rest
for you. `herdr-docket init --factory` writes it in one command; the stages,
schedules and refusals live in [The factory](docs/factory.md).

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26-00ADD8.svg)](https://go.dev)

```text
 you / intake ──► task ──► author ──► PR ──► verifier ──► gate ──► merged
 (says how it's                ▲               │ FAIL       │
  verified)                    └───────────────┘            └──► held: your Blocked column
```

Write a task the way you would brief a senior engineer, and walk away:

```bash
herdr-docket task create "A plan respects a vegetarian preference written in French" -a dev \
  -d "Users who type « végétarien » still get meat in their week plan.
Done when: the planner treats the preference the same in any language.
Verify by: a planner test with « végétarien » that is red on main, green after."
```

Within 15 seconds the daemon opens a [Herdr](https://herdr.dev) workspace on the
agent's repo and starts a real coding agent with its persona plus the task.
The author delivers a PR; the verifier replays the task's own check and records
PASS or FAIL; a PASS on green CI merges on its own unless the task is
`critical` or the diff touches a path in your `CODEOWNERS`. You come back to
merged work, a notification per merge or hold, and a `Blocked` column holding
only the questions that are genuinely yours.

[paperclip.ing](https://paperclip.ing) runs a company of agents autonomously.
This is the smallest version of that idea that still works, built for Herdr:
named agents, a shared queue, no org chart, markdown all the way down — and
autonomy as a dial you set yourself, per project, rather than a mode you switch
on.

**Docs:** [working with the factory](#working-with-a-software-factory) ·
[writing an agent](docs/agents.md) · [worked examples](docs/examples.md) ·
[the factory](docs/factory.md) · [where the queue lives](docs/queues.md)

## Install and first run

```bash
herdr plugin install DnzzL/herdr-docket
herdr-docket init                     # backlog project + example agent in ~/fleet
$EDITOR ~/fleet/agents/example/AGENT.md
herdr-docket task create "First task" -a example
```

`init` writes the fleet dir — a Backlog.md project and one example agent — and
Herdr starts the daemon for you. Edit that persona, create a task, and the
daemon opens a workspace on it within ~15 seconds. `herdr-docket list` is the
queue as a list.

Where the queue already lives somewhere else — your own repo's backlog, a
Basecamp project, a GitHub Projects board — or the fleet works several projects
at once, that is a few lines of `fleet.yaml`:
[Where the queue lives](docs/queues.md).

## How much autonomy

Nothing here is all-or-nothing. Autonomy is a handful of levers, each one a
line of config, and you can move them one project at a time:

| Lever | Keep a hand on it | Let it run |
| --- | --- | --- |
| `statuses.todo` | a column you fill by hand — `ready-for-agent` | your project's default column: everything new is fair game |
| assignee / `default_agent` | name the agent on the task, one at a time | `default_agent` picks up everything unassigned — per project, so each one has its own intake |
| `statuses.failed` | points at a human column — `needs-info`, `ready-for-human` | points at a real `Failed`; nobody is paged |
| `verifier` | a reviewer verifies every delivered PR, you merge ([Example 4](docs/examples.md#4-a-reviewer-that-verifies-the-devs-prs)) | `merge: auto` — the gate merges a PASS on green CI, except `critical` tasks and `CODEOWNERS` paths ([the pipeline](docs/factory.md#the-pipeline)) |
| `disabled` / `agent pause` / `pause` | park an agent — or the whole fleet — while you look at something | never paused |

The one lever with teeth is the first: **the fleet only ever picks up the
status you name**, so handing it a project means handing it one column, not a
board. Your triage, your wontfix, your waiting-on-a-human columns stay yours —
see [Where the queue lives](docs/queues.md).

It never overrides you in the other direction either: `herdr-docket run TASK-12`
reaches a paused agent, because pressing the button is
human intent, not scheduling.

These levers are also the whole of a running *factory* — intake polling
sources into verified tasks, a verifier and a merge gate on every PR, a stall
sweep, a weekly lookback
over what keeps recurring. The loop is personas and cron, wired in
[docs/factory.md](docs/factory.md); `herdr-docket init --factory` writes it
into an existing fleet, the repo-side personas paused until you name the repo.

## Working with a software factory

A factory turns well-stated work into merged code while you sleep. It does not
decide what is worth building, and it is only as good as the tasks it is fed
and the corrections it keeps. The habits below are the difference.

**The task is the prompt.** Every task answers three things, and the third is
the one that matters:

- **Outcome** — the problem in a user's words, not the fix you imagine.
- **Done when** — the observable change, one sentence.
- **Verify by** — how a stranger proves it: the test that is red today, the
  request and its expected response, the screen and what it should show.

If you cannot write *Verify by*, the task is not ready for an agent — it is
still a decision (see below). One task, one pull request, a few hours of
work: a task you would not review in ten minutes is two tasks.

| Instead of | Write |
| --- | --- |
| "Improve onboarding" | "New users drop at step 2. Done when: step 2 needs one field, not four. Verify by: the signup e2e completes with only an email." |
| "Fix the flaky test" | "`week-plan.spec.ts` fails 1 run in 5 on CI. Done when: it waits on hydration, not a timeout. Verify by: 20 consecutive green runs, pasted." |
| "Refactor the planner" | Don't. Name the bug or the feature the refactor unblocks, and let the author choose the shape. |

**Delegate ideas, not solutions.** An idea goes in as a *spike*: "Investigate
X; deliverable: a note on the task with two or three options, their cost, and
a recommendation — no code." The author researches, you choose, and the
choice becomes the next task with its *Verify by*. Agents are good at options;
choosing between them is yours.

**What stays human** — and how the factory keeps it that way:

- **Product decisions**: what to build, for whom, what to drop. Intake files
  anything it cannot verify straight into your `Blocked` column with the one
  question that would unblock it. That column is your inbox: answer the
  question in a note, move the task back to the pickup column, done.
- **The irreversible**: schema and migrations, auth, payments, deploys,
  anything touching personal data. List those paths in the repo's
  `CODEOWNERS` and the gate holds every PR that touches them; label a single
  task `critical` for the same effect.
- **Taste and voice**: brand, copy, design direction. Give the factory a
  style guide in the repo and it will follow it; do not expect it to invent one.

**Correct the system, not the run.** When a merged PR is wrong, resist fixing
it by hand. File the fix as a task, and put the lesson where the next run
will read it: a line in the repo's `CLAUDE.md`, a lint rule, a test that
fails on the mistake. The second time you write the same review comment, it
should become a check. The weekly lookback files these patterns for you; the
repo is the factory's memory, and it compounds.

**Raise the dial slowly.** Start a queue with `verifier:` and no `merge:` —
read the verdicts, merge by hand. When a week of PASSes would have been your
merges too, add `merge: auto`, and add a `CODEOWNERS` line every time a merge
makes you nervous. `herdr-docket pause --now` stops everything at once;
`agent pause` parks one agent.

**A daily rhythm that works:**

- *Morning*: read the merge and hold notifications, answer the `Blocked`
  column, skim what merged (`herdr-docket history`).
- *During the day*: turn what you notice into tasks — two minutes each, with
  *Verify by*. Ideas become spikes.
- *Evening*: leave the queue with a night's worth of small, verifiable work.

**Acquisition and growth.** The factory ships; it does not find users. Split
the work the same way as product: positioning, channels, pricing and talking
to users are yours. What it does well is the measurable execution behind
them — landing-page variants, SEO pages from a keyword list, release notes
from merged PRs, analytics events, onboarding emails as code — each a task
whose *Verify by* is a number or an observable (the event fires, the page
scores 90 on Lighthouse, the email renders in the preview). Point intake at
where your users already talk — issues, support inbox, reviews — and their
complaints arrive as verifiable tasks: that is the loop from users to code.

**What breaks a factory:** vague tasks; big tasks; fixing agents' PRs by hand
so nothing is learned; letting `Failed` pile up unread; auto-merge on a repo
with no CI; one model judging its own work.

## The model

Everything lives in one **fleet dir** (default `~/fleet`) — a git repo you can
read, diff, and back up:

```
~/fleet/
├── backlog/           # the Backlog.md project: one markdown file per task
│                      # (the default queue — see "Where the queue lives")
└── agents/
    ├── dev/AGENT.md       # who the agents are: an author…
    └── reviewer/AGENT.md  # …and the verifier its queue names
```

- **A task** is one unit of work in the queue: goal, description, acceptance
  criteria, priority, and an `assignee` that names the agent. By default that
  queue is Backlog.md, and its phases are the lifecycle:
  `To Do → In Progress → Done | Failed | Blocked`.
- **An agent** is one markdown file: YAML frontmatter for the run parameters,
  body for the persona every one of its runs opens with. Every field, and the
  runtimes a post can run on: [Writing an agent](docs/agents.md).
- **A shared brief.** If `~/fleet/FLEET.md` exists, its body is prepended to
  every agent's persona — the one place for what is true of the whole company:
  what the product is, who the human is, what never to do. Absent, every prompt
  is exactly what it would have been without it. `herdr-docket init` writes a
  commented example; delete it or fill it in.
- **One at a time, per agent.** Agents work in parallel, but each agent runs
  a single task to completion — and root-mode agents sharing a checkout are
  serialized, because the thing to protect is the working copy, not a queue.
  A second `To Do` task for a busy agent simply waits its turn.
- **The agent closes its own task** through the fleet CLI — it reports where
  things stand with `herdr-docket task note`, then closes with exactly one of
  `herdr-docket task done | fail | block`. It never needs credentials for, or
  knowledge of, whatever backend is behind the queue. If it ends silent, the
  daemon closes the task for it: a run that ends with nothing to show for it
  goes `Failed`, a workspace you closed mid-run goes `Blocked` (you decided,
  and the queue says so). In a queue with a verifier, an author's PR is a
  delivery, not a close: `task done --pr` leaves the task open for the
  pipeline, and the fleet closes it — `Done` once merged, `Blocked` when held.
- **Approvals are the agent's own.** Claude Code asks in its pane like it always
  does; jump in from Herdr's sidebar, answer, leave. Configure permissiveness per repo
  the way you already do (`.claude/settings.json`).
- **Cleanup is automatic where it's safe.** `Done` → the workspace is torn down
  (the work is in the repo and the notes). `Failed`/`Blocked` → the workspace
  stays open as the place to resume, and the ticket gets a note naming it.

Personas built this way — a dev, a verifier, an intake, a lookback:
[Worked examples](docs/examples.md).

## Anatomy of a run

1. The daemon polls the queue (~15s) and, for every idle
   agent, picks its most urgent routed open task: priority, then ordinal,
   then age.
2. It claims the task (`In Progress`), provisions a Herdr workspace on the
   agent's `workdir`, and starts the agent with an assembled prompt: persona
   - full task (description, acceptance criteria, notes from previous runs)
   - the reporting protocol.
3. The agent works — you can watch it live, jump in, answer its permission
   prompts, or close its workspace to call the run off.
4. The agent reports back with `herdr-docket task note` as it goes, then closes
   with `done`, `fail`, or `block` and a note saying what happened and how it
   knows. The daemon reconciles anything left hanging and records the run in
   an append-only `history.jsonl`.
5. In a queue with a verifier, a delivered PR goes on: the verifier runs on
   the same task, a FAIL sends the author back with the note (twice at most),
   and a PASS reaches the merge gate — [the pipeline](docs/factory.md#the-pipeline).

Runs have a time budget. The prompt tells the agent the honest way out of a
task that won't fit: one coherent slice, a handoff note, a follow-up task —
the queue itself is the checkpoint mechanism.

## Watching the fleet

Herdr already shows what is live: each run is a workspace in the sidebar with
its agent's status, and `herdr agent list` names them all. The past is
`herdr-docket history` — how long each run took, a `timed-out` mark when it
ran past its deadline, its verdict, its branch,
commits and PR, and for a verifier run `verified PASS p=…` — and
`herdr-docket logs` is the daemon's own account. A run
that fails, blocks or is held back from merging raises a Herdr notification.

The fleet's own runs in flight are `herdr-docket runs`, or the plugin's
**Herdr-Docket: runs** overlay pane — `x` stops the selected run, `p` pauses
the fleet. A stopped run ends cancelled and leaves its task Blocked.

**Pausing the fleet** (`herdr-docket pause`) is the credit brake: the daemon
starts no run and a pipeline holds its next stage — verifier, rework — until
`herdr-docket resume`. Runs in flight finish; `pause --now` stops them too.
`herdr-docket run` still works, since that is you asking. Schedules in
herdr-automations are not paused: disable them there.

## Commands

| | |
| --- | --- |
| `herdr-docket daemon` | the worker (Herdr starts it for you) |
| `herdr-docket init` | bootstrap the fleet dir |
| `herdr-docket init --factory` | …and install the factory loop (see [docs/factory.md](docs/factory.md)) |
| `herdr-docket auth basecamp` | sign in to a hosted queue, once |
| `herdr-docket auth github` | same, for a GitHub Projects board (`--token <pat>` to store one) |
| `herdr-docket list` | the queue, grouped by phase, with the routed agent |
| `herdr-docket run TASK-12` | run one task now |
| `herdr-docket task list` | the queue as the agent sees it (`--all` includes closed work) |
| `herdr-docket task view ID` | one task: body, notes, criteria, and who it is routed to |
| `herdr-docket task create "…" -a AGENT` | add work to the queue (`-s SOURCE` when several, repeatable `--ac "…"` for acceptance criteria) |
| `herdr-docket task assign ID AGENT` | hand a task to another agent, same id, same thread |
| `herdr-docket task note ID "…"` | say where things stand without closing |
| `herdr-docket task done\|fail\|block ID` | close with a verdict (`--note "…"` for the evidence; `--pr URL` delivers a PR to the verifier) |
| `herdr-docket task verdict ID PASS\|FAIL --pr URL` | a verifier's verdict, pinned to the PR's diff |
| `herdr-docket agent list` | the agents, and which are parked |
| `herdr-docket agent pause\|resume NAME` | park an agent, or unschedule nothing more for it |
| `herdr-docket runs` | the runs in flight; a record past its timeout is marked `stale?` |
| `herdr-docket stop TASK-12` | stop a task's run in flight |
| `herdr-docket pause [--now]` / `resume` | pause the whole fleet — no new runs, no pipeline stages (`--now` stops runs in flight) |
| `herdr-docket pane` | the runs overlay (the plugin opens it for you) |
| `herdr-docket history [TASK-12]` | recent runs: how long, the verdict, what it produced — branch, commits, PR |
| `herdr-docket logs` | the daemon log's tail (`-n LINES`), without guessing where it lives |
| `herdr-docket install-skill` | teach your coding agent to write fleet tasks |

## The queue

By default it is the Backlog.md project in the fleet dir, and there is nothing
to configure. Three things change that, in rising order of how much config they
cost, and each one is a few lines of `fleet.yaml`:

1. **A project you already have.** Point a source at your repo with `dir:`, then
   either add the fleet's five statuses to your `backlog/config.yml`, or name
   your own in `statuses:` — a status you don't name is not the fleet's
   business, which is how your triage and wontfix columns stay yours.
2. **Several projects at once.** Use `sources:` — a map of name → the same
   block `source:` takes. Each source's tasks carry its name as an id prefix
   (`myapp/TASK-12`), and each source names the agent its unassigned work falls
   to, because an agent carries its `workdir`.
3. **A hosted queue.** Basecamp to-do lists or a GitHub Projects v2 board:
   sign in once with `herdr-docket auth`, and the commands do not change.

Linear, Jira, Notion, a directory of text files: an adapter is one package under
`internal/work/` and five methods, and
[contributions are welcome](docs/queues.md#writing-an-adapter--contributions-welcome).

```yaml
dir: ~/somewhere/else     # fleet dir (default ~/fleet)
default_agent: dev        # picks up unassigned tasks; unset = leave them alone
source:
  kind: backlogmd         # where the queue lives (default: the Backlog.md
                          # project in the fleet dir)
```

Every backend, every field, and what each one costs:
[Where the queue lives](docs/queues.md).

## What it isn't

- **Not a scheduler.** Recurring work belongs to
  [herdr-automations](https://github.com/DnzzL/herdr-automations) — point an
  automation's prompt at `herdr-docket task create` and the two plugins compose:
  automations decide *when*, fleet decides *what* and *who*.
- **Not a workflow engine.** A task is one goal; its only fixed sequence is
  author → verifier → gate. Fan-out happens the honest way: an agent creates
  follow-up tasks in the same queue.
- **Not a job scheduler with priorities and preemption.** One run per agent,
  serialized per shared checkout, nothing preempted: the concurrency model is
  what a git checkout can survive, with zero infrastructure.
- **No store.** By default the queue is markdown in a git repo, run history is
  one JSONL file, and uninstalling leaves both behind.

## Teaching your agents

`skills/fleet-tasks/SKILL.md` teaches a coding agent to create well-formed fleet
tasks — real descriptions, at least one acceptance criterion, and the rule that
keeps the fleet queue separate from a project's own. Agents only discover skills
under `~/.claude/skills`, so install it once:

```bash
herdr-docket install-skill     # symlinks into ~/.claude/skills
```

It points a symlink at the bundled skill, so plugin upgrades update the skill
too. Start a new agent session afterwards.

## License

MIT.
