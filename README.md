# herdr-docket

**A shared task queue worked by your coding agents.** A [Backlog.md](https://backlog.md)
project as the queue — or Basecamp, or a GitHub Projects board, if that's where
your work already lives (see [Where the queue lives](docs/queues.md)) —
`AGENT.md` personas as the workers, and a daemon that routes every open task to
the agent it names — agents in parallel, one run each, in
[Herdr](https://herdr.dev) workspaces you can watch, join, or close.

*docket*: the list on the wall of the work a crew will get to — what the queue is, all of it.
Part of the [Herdr plugin family](https://herdr.dev/docs/plugins/).

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26-00ADD8.svg)](https://go.dev)

```text
      you ─────────┐
 an automation ────┼──► the queue (open) ──► daemon ──► herdr agent run ──► Done | Failed | Blocked
 another agent ────┘    markdown, Basecamp    one run per agent          (the agent reports itself)
                        or a GitHub board
```

Write a task, assign it to an agent, walk away:

```bash
herdr-docket task create "Triage the project backlog" \
  -d "Route every needs-triage task: out of scope, agent-ready, or needs a human." \
  -a pm
```

Within 15 seconds the daemon opens a workspace on that agent's repo, starts a real
coding agent (Claude Code by default) with the agent's persona plus the task, and
the agent reports back into the queue itself with `herdr-docket task done|fail|block`.
You come back to a board that tells the truth — and to nothing else,
because a run that ends `Done` cleans its own workspace up.

[paperclip.ing](https://paperclip.ing) runs a company of agents autonomously.
This is the smallest version of that idea that still works, built for Herdr:
named agents, a shared queue, no org chart, markdown all the way down — and
autonomy as a dial you set yourself, per project, rather than a mode you switch
on.

**Docs:** [writing an agent](docs/agents.md) · [worked examples](docs/examples.md) ·
[where the queue lives](docs/queues.md) · [the board pane](docs/pane.md)

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
queue as a list; `herdr-docket pane` is the board over it.

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
| a reviewer agent | gates the dev's PRs ([Example 4](docs/examples.md#4-a-reviewer-that-gates-the-devs-prs)) | no gate; the dev merges its own work |
| `runs_per_day`, `minutes_per_day` | a ceiling on the day ([Budgeting](docs/agents.md#budgeting-an-agent)) | unset — unbounded |
| `disabled` / `agent pause` | park an agent while you look at something | never paused |

The one lever with teeth is the first: **the fleet only ever picks up the
status you name**, so handing it a project means handing it one column, not a
board. Your triage, your wontfix, your waiting-on-a-human columns stay yours —
see [Where the queue lives](docs/queues.md).

It never overrides you in the other direction either: `herdr-docket run TASK-12`
reaches a paused agent and an over-budget one, because pressing the button is
human intent, not scheduling.

## The model

Everything lives in one **fleet dir** (default `~/fleet`) — a git repo you can
read, diff, and back up:

```
~/fleet/
├── backlog/           # the Backlog.md project: one markdown file per task
│                      # (the default queue — see "Where the queue lives")
└── agents/
    ├── pm/AGENT.md    # who the agents are
    └── dev/AGENT.md
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
  and the board says so).
- **Approvals are the agent's own.** Claude Code asks in its pane like it always
  does; jump in from the board, answer, leave. Configure permissiveness per repo
  the way you already do (`.claude/settings.json`).
- **Cleanup is automatic where it's safe.** `Done` → the workspace is torn down
  (the work is in the repo and the notes). `Failed`/`Blocked` → the workspace
  stays open as the place to resume, and the ticket gets a note naming it.

Four personas built this way, from a PM that triages to a reviewer that gates:
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

Runs have a time budget. The prompt tells the agent the honest way out of a
task that won't fit: one coherent slice, a handoff note, a follow-up task —
the queue itself is the checkpoint mechanism.

## The board

The board pane (overlay in Herdr, or `herdr-docket pane`) is the fleet's
triage surface: what runs next, what is running now, and the keys to act on
both. `r` runs a task now, `x` stops the run on the selected row, `enter` jumps
into the workspace a run opened, `v` reads the task, `s` re-routes it, `a` adds
one, `/` filters, and `g` switches to the agent roster where `p` pauses and
resumes an agent. Rows show the task's queue and the id its own board uses, the
Running group sits above the phases, and a run past its agent's timeout is
marked stale.

![The Docket board: the Running group, the queue each task came from, a run past its timeout, and the roster behind g](docs/board.gif)

```
  TASK-12   myapp  Fix the parser           dev paused      12m / 45m
  TASK-4    ops    Rotate the deploy keys   example         last run done Wed 14:28
```

The queue column appears only when the fleet has more than one, and the agent
column says the two ways routing can be wrong: yellow `paused` for an agent the
scheduler is skipping, red `ghost?` for a name nobody answers to. The last
column is the run — elapsed against the timeout it was started with, marked
stale past it — or how the last run ended. It never closes a task with a
verdict: see
[ADR 0008](docs/adr/0008-the-board-is-a-triage-surface.md) for what the pane
shows and what it deliberately refuses. Binding it to a chord, and what to do
when the key does nothing: [the board pane](docs/pane.md).

## Commands

| | |
| --- | --- |
| `herdr-docket daemon` | the worker (Herdr starts it for you) |
| `herdr-docket init` | bootstrap the fleet dir |
| `herdr-docket auth basecamp` | sign in to a hosted queue, once |
| `herdr-docket auth github` | same, for a GitHub Projects board (`--token <pat>` to store one) |
| `herdr-docket list` | the queue, grouped by phase, with the routed agent |
| `herdr-docket run TASK-12` | run one task now |
| `herdr-docket task list` | the queue as the agent sees it (`--all` includes closed work) |
| `herdr-docket task view ID` | one task: body, notes, criteria, and who it is routed to |
| `herdr-docket task create "…" -a AGENT` | add work to the queue (`-s SOURCE` when several) |
| `herdr-docket task assign ID AGENT` | hand a task to another agent, same id, same thread |
| `herdr-docket task note ID "…"` | say where things stand without closing |
| `herdr-docket task done\|fail\|block ID` | close with a verdict (`--note "…"` for the evidence) |
| `herdr-docket agent list` | the agents, and which are parked |
| `herdr-docket agent pause\|resume NAME` | park an agent, or unschedule nothing more for it |
| `herdr-docket history [TASK-12]` | recent runs |
| `herdr-docket pane` | the interactive board: what runs next, what is running now |
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
- **Not a workflow engine.** A task is one goal for one agent. Fan-out happens
  the honest way: an agent creates follow-up tasks in the same queue.
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
