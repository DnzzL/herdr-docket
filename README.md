# herdr-docket

**A shared task queue worked by your coding agents.** A [Backlog.md](https://backlog.md)
project as the queue — or Basecamp, or a GitHub Projects board, if that's where
your work already lives (see [Where the work lives](#where-the-work-lives)) —
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

## How much autonomy

Nothing here is all-or-nothing. Autonomy is a handful of levers, each one a
line of config, and you can move them one project at a time:

| Lever | Keep a hand on it | Let it run |
| --- | --- | --- |
| `statuses.todo` | a column you fill by hand — `ready-for-agent` | your project's default column: everything new is fair game |
| assignee / `default_agent` | name the agent on the task, one at a time | `default_agent` picks up everything unassigned — per project, so each one has its own intake |
| `statuses.failed` | points at a human column — `needs-info`, `ready-for-human` | points at a real `Failed`; nobody is paged |
| a reviewer agent | gates the dev's PRs ([Example 4](#example-4--a-reviewer-that-gates-the-devs-prs)) | no gate; the dev merges its own work |
| `runs_per_day`, `minutes_per_day` | a ceiling on the day ([Budgeting](#budgeting-an-agent)) | unset — unbounded |
| `disabled` / `agent pause` | park an agent while you look at something | never paused |

The one lever with teeth is the first: **the fleet only ever picks up the
status you name**, so handing it a project means handing it one column, not a
board. Your triage, your wontfix, your waiting-on-a-human columns stay yours —
see [Where the work lives](#where-the-work-lives).

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
  body for the persona every one of its runs opens with.
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

## Writing an agent

An agent is a folder in `~/fleet/agents/<name>/` with one `AGENT.md`. The name
is what tasks put in `assignee`.

```markdown
---
model: claude-sonnet-5        # optional — passed to the agent as --model
workdir: ~/Projects/myapp     # required — where the work happens
workspace: worktree           # worktree (default): fresh branch per run
                              # root: work directly on the checkout
timeout_minutes: 60           # optional — the run's time budget (default 60)
runs_per_day: 6               # optional — cap poll-triggered runs per rolling 24h
minutes_per_day: 180          # optional — cap their total minutes over that window
agent: claude                 # optional — any kind `herdr agent start` supports
disabled: false               # optional — true parks the agent: no new runs
---

You are the persona every run of this agent opens with. Say who the agent
is, what it owns, what its bias is, and how it should decide. This body is
the difference between a generic LLM and a colleague.
```

`workspace: worktree` gives every run a disposable branch (`fleet/task-12-…`) —
a run that went sideways is a diff you throw away. `root` is for agents whose
job *is* the working copy: backlog grooming, docs, anything that must see
uncommitted state.

### Running a post on another agent runtime

Herdr speaks more than one coding agent, and `agent:` picks which one a post
runs on — Claude Code by default. Your subscription is per post, not per fleet:

```yaml
---
role: dev
agent: opencode                     # herdr kinds: pi, claude, codex, gemini,
model: opencode-go/kimi-k2.7-code   # opencode, cursor, amp, grok, qwen, …
workdir: ~/Projects/myapp
---
```

```yaml
---
role: pm
agent: pi
model: anthropic/claude-sonnet-4-5
agent_args: ["--provider", "anthropic"]
workdir: ~/Projects/myapp
---
```

Two things to know before you switch a post over:

- **`model:` is written straight through as `--model`**, so it has to be the
  *runtime's* spelling. Claude Code takes the short alias (`sonnet`); opencode
  and pi want `provider/model` (`opencode models` and `pi update` list what you
  have). A short alias on opencode is not a fallback — it is a model id that
  does not exist.
- **`mcp_config:` emits `--mcp-config`, which is Claude Code's flag.** Neither
  pi nor opencode has it — they manage their own servers (`opencode mcp`,
  `pi install`). Leave `mcp_config` empty on a post that is not Claude Code, or
  the agent fails to start.

Anything else the runtime takes goes in `agent_args`, passed through untouched
and *after* the two above — so an explicit `agent_args` entry wins over them.

### Sharing a method between agents

An agent is one post: a role *in one repo*. Two posts doing the same job in two
projects — a dev on each — want the same method and a different context. `role:`
is how they share it:

```yaml
---
role: dev                             # → ~/fleet/roles/dev.md
model: opus
workdir: ~/Projects/myapp
---
```

Three layers reach every run, widest first: `FLEET.md` (the whole fleet), then
`roles/<role>.md` (this role), then the persona (this post). Any of them may be
absent. A role file is plain markdown — no frontmatter, no inheritance between
roles — and you write your own: `roles/marketer.md`, `roles/reviewer.md`.

A `role:` naming a file that does not exist **grounds that agent**, on purpose.
A brief nobody asked for may be missing; one an agent points at may not, or the
run is assembled with a third of its instructions gone and nothing says so.
`herdr-docket agent list` names what failed, and the rest of the fleet keeps
working.

One thing that does *not* belong in any of the three: your project's status
words. The fleet already knows them, and the prompt tells each run the words its
queue accepts — so a persona that says "move it to `ready-for-agent`" is a
rename away from being wrong, and does not need to exist.

### Budgeting an agent

An agent that keeps creating follow-up work for itself — the marketer that
decomposes its own plan, the reviewer's sweep — is the design working: a run
hands on what it found instead of expanding its own scope. It is also an
unbounded loop, and `timeout_minutes` caps one run, not the day. Two optional
fields cap the day:

- `runs_per_day` — how many poll-triggered runs the agent may complete in a
  rolling 24 hours.
- `minutes_per_day` — how many minutes of run time it may spend over the same
  rolling window.

Unset means unbounded, exactly as before. Reaching a limit spends it: the run
that would cross the line waits. A spent agent is like a busy one — its `To Do`
tasks stay open with nothing written on them, and the rest of the queue keeps
moving — and because the window rolls, the budget re-opens on its own as old
runs age out. `herdr-docket agent list` (and the board's `g` view) shows the
spend for any agent that has a budget:

```bash
herdr-docket agent list           # dev  active  ~/Projects/myapp  4/6 runs today, 130/180 min
```

Budgets are scheduling policy, not a lock: `herdr-docket run TASK-12` still
starts a run for an over-budget agent, because pressing the button is human
intent — the same rule that lets a manual run reach a paused agent.

### Parking an agent

`disabled: true` keeps the persona on disk but takes the agent out of
scheduling: the daemon starts nothing new for it, and its `To Do` tasks wait
in the queue without a word written on them. Pause from the CLI instead of
by hand:

```bash
herdr-docket agent pause dev      # resume with: agent resume dev
herdr-docket agent list           # dev  paused  ~/Projects/myapp
```

The daemon re-reads `agents/` on every tick, so a pause lands within ~15s and
needs no restart. Two things it deliberately does *not* do: it never kills a
run already in flight (that agent keeps its full timeout and still reports its
task), and it never overrides you — `herdr-docket run TASK-12` reaches a paused
agent, because pressing the button is human intent, not scheduling.

### Example 1 — a PM that triages your project's backlog

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

### Example 2 — a dev that works `ready for agent` tickets

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

### Example 3 — a marketer that owns a strategy

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

### Example 4 — a reviewer that gates the dev's PRs

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

Note the `workspace: root`. Personas that must *write* to a project's own
`backlog/` need the real checkout — a checkbox ticked inside a disposable
worktree is gone with the worktree. Do the reading in a scratch worktree, the
recording in the repo root, and say so in the persona.

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

## Quick start

```bash
herdr plugin install DnzzL/herdr-docket
herdr-docket init                     # backlog project + example agent in ~/fleet
$EDITOR ~/fleet/agents/example/AGENT.md
herdr-docket task create "First task" -a example
```

The board pane (overlay in Herdr, or `herdr-docket pane`) is the fleet's
triage surface: what runs next, what is running now, and the keys to act on
both. `r` runs a task now, `x` stops the run on the selected row, `enter` jumps
into the workspace a run opened, `v` reads the task, `s` re-routes it, `a` adds
one, `/` filters, and `g` switches to the agent roster where `p` pauses and
resumes an agent. Rows show the task's queue and the id its own board uses, the
Running group sits above the phases, and a run past its agent's timeout is
marked stale. It never closes a task with a verdict — see
[ADR 0005](docs/adr/0005-the-board-is-a-triage-surface.md) for what the pane
shows and what it deliberately refuses. Bind it to a chord in
`~/.config/herdr/config.toml`:

```toml
[[keys.command]]
key = "prefix+f"
type = "shell"
command = "herdr plugin pane open --plugin dnzzl.docket --entrypoint board --placement overlay"
```

Nothing happens when you press the chord? A linked plugin is registered but not
necessarily *built*, and it can be *disabled* — and because the chord is a plain
shell command it fails silently, with `herdr config check` reporting `ok`. Ask
for the pane directly, which prints the error the keybinding swallows:

```bash
herdr plugin list                      # enabled? and does bin/ actually exist?
herdr plugin pane open --plugin dnzzl.docket --entrypoint board --placement overlay
herdr plugin pane close PANE_ID        # this opens the board without any chord
```

`plugin_disabled` → `herdr plugin enable dnzzl.docket`. `... because it does not
exist` → run the manifest's build step (`sh scripts/install.sh`). Both take
effect without restarting the Herdr server, so no running agent loses its pane.

Config is optional — `fleet.yaml` in the plugin config dir:

```yaml
dir: ~/somewhere/else     # fleet dir (default ~/fleet)
default_agent: dev        # picks up unassigned tasks; unset = leave them alone
source:
  kind: backlogmd         # where the queue lives (default: the Backlog.md
                          # project in the fleet dir — see below)
```

## Where the work lives

By default the queue is the Backlog.md project in the fleet dir, and there is
nothing to configure. Three things change that, in rising order of how much
config they cost.

### Work a project you already have

Point a source at your own repo with `dir:` — absolute, `~`, or relative to the
fleet dir. Your project has its own status words, and there are two ways to
meet:

**Let the fleet own the vocabulary.** Add its five statuses to your
`backlog/config.yml` and there is nothing else to configure:

```yaml
statuses: ["To Do", "In Progress", "Blocked", "Failed", "Done"]
```

**Or keep your own words** — usually the right call on a board humans already
read — and name them once:

```yaml
sources:
  myapp:
    kind: backlogmd
    dir: ~/Projects/myapp
    statuses:
      todo: ready-for-agent       # the only status the fleet picks up
      done: done
      failed: ready-for-human     # your word for "a human now"
      blocked: needs-info
      # in_progress omitted: this project has no word for it
```

Read `statuses:` as a whitelist: **a status not named there is not the fleet's
business.** That is the whole point of it — a real board has a triage column, a
wontfix column, a waiting-on-a-human column, and a fleet that treated every
unrecognised status as open work would put an agent on them. It is also the
first lever in [How much autonomy](#how-much-autonomy): `todo` is exactly how
much of your board you are handing over.

`todo`, `done`, `failed` and `blocked` are required — a queue the fleet can
pick from but cannot close leaves every task open for the next tick to pick up
again. `in_progress` is optional: it is display only, so a project with no word
for it simply never shows one. A source with a `dir:` of its own is never
scaffolded or patched by `herdr-docket init`; its config stays yours.

### Several projects at once

One fleet can work several projects, each keeping the tool it already uses. Use
`sources:` — a map of name → the same block `source:` takes — instead of
`source:`:

```yaml
sources:
  myapp:                      # your repo, its own words
    kind: backlogmd
    dir: ~/Projects/myapp
    statuses: {todo: ready-for-agent, done: done, failed: ready-for-human, blocked: needs-info}
  agency:                     # a client's Basecamp, worked alongside it
    kind: basecamp
    basecamp:
      account_id: "9999999"
      lists:
        dev: "1111111"
```

Each source's tasks carry its name as an id prefix — `myapp/TASK-12`,
`agency/987654` — so every command that takes an id (`task view`, `note`,
`done|fail|block`) routes to the right project, and `herdr-docket list` and the
board show them all together (the board gives each queue a column of its own
and shows the id without the prefix, which is the part a person retypes).
Creating work then names the project, because a bare `task create` cannot
guess — and the pane's `a` asks for it:

```bash
herdr-docket task create "Triage the backlog" -a pm -s myapp
```

Each source names the agent its unassigned work falls to, because an agent
carries its own `workdir` — one global default would send one project's tasks
into another project's checkout:

```yaml
sources:
  myapp:
    dir: ~/Projects/myapp
    default_agent: myapp-pm     # this project's intake
  agency:
    kind: basecamp
    default_agent: agency-pm
```

A source that names none falls back to the fleet's `default_agent`, and no
default anywhere still means no default: unassigned work is left alone.
Pointing the default at a **PM rather than a dev** is the safe setting — an
unassigned task is by definition unspecced, so the agent that receives it
should be the one that specs it and then hands it on:

```bash
herdr-docket task assign myapp/TASK-12 dev
```

`assign` is how one agent passes work to another without closing it: same
task, same id, whole history in one place. It is also the one way a run may
end without a verdict — the fleet reads a reassigned open task as handed on
rather than abandoned, and routes it on the next tick.

The prefix *is* the source, so an agent's follow-up task inherits it without
the agent knowing a second queue exists. `source:` and `sources:` are mutually
exclusive; `source:` stays exactly what it was — one unnamed queue, no prefix
anywhere.

### A hosted queue: Basecamp

If the work already lives somewhere else, the queue does not have to be local:

```yaml
source:
  kind: basecamp
  basecamp:
    account_id: "9999999"     # the account the lists are in
    lists:                    # Basecamp has no labels, so the container is
      dev: "1111111"          # the routing key: one to-do list per agent
      pm: "2222222"
```

Basecamp wants an account, so sign in once. The fleet doesn't ship an
application of its own — register one at
[launchpad.37signals.com/integrations](https://launchpad.37signals.com/integrations)
with the redirect URI `http://localhost:8917/callback`, then:

```sh
export HERDR_DOCKET_BASECAMP_CLIENT_ID=...
export HERDR_DOCKET_BASECAMP_CLIENT_SECRET=...   # only if your app has one
herdr-docket auth basecamp
```

The tokens land in `credentials.yaml` (0600) beside `fleet.yaml` and never in
it — `fleet.yaml` is the file you paste into a bug report. A Basecamp access
token lives two weeks, so the fleet refreshes it on the way out: a machine left
alone for a month heals itself on the next poll instead of failing every one of
them.

Agents never see any of this. `task list`, `view`, `create`, `note` and
`done|fail|block` are the same commands, and the fleet CLI is the only thing
they talk to. What differs is what the backend can hold: Basecamp has no labels
(the list is the assignee), no priority, and one word for an ending — so a
`fail` or `block` completes the to-do and says which it was in a comment. A
to-do in a list the fleet doesn't know about is simply not its work.

### A hosted queue: GitHub Projects

A GitHub Projects v2 board is a queue too — useful when the work is already
issues in a repo, and the board is how you and a human colleague already look
at it:

```yaml
source:
  kind: github
  github:
    owner: your-org           # the board's owner: an org or a user
    project: 3                # the number in .../projects/3
    repo: your-org/your-repo  # where Create writes, and what List reads
    # agent_field: Agent      # the single-select field carrying the agent
    # status_field: Status    # the board's column field
    # in_progress: In Progress
```

**The board is the queue and the repo is the container.** A task is a real
issue that has been put on the board — not one of the board's own draft cards,
because a draft card cannot be commented on and work that leaves no trace is
not work a fleet can hand on. An issue in the repo that is not on the board is
somebody else's inbox: the fleet says so and never adds it for you.

Sign in once. The token needs the `project` scope (and `repo`, to write
issues):

```sh
gh auth login --scopes project                       # or export HERDR_DOCKET_GITHUB_TOKEN
herdr-docket auth github                             # checks the token, makes the Agent field
herdr-docket auth github --token ghp_...             # or hand it a PAT to store (0600)
```

The token is looked for in that order: `HERDR_DOCKET_GITHUB_TOKEN`, then
`gh auth token`, then the copy `auth github` stored in `credentials.yaml`.
Signing in also provisions the **`Agent`** single-select field with one option
per `agents/<name>/`, which is the routing key — an agent is an option on the
board, and `assign` moves a task between options. It never removes an option
(removing one clears every item's value that pointed at it) and it says so if
the name it needs is already a field of another type.

What the board can hold, and what that costs:

- A closed issue is closed work, whatever column it sits in: the issue decides,
  the column only describes. The **verdict** is a `Verdict: done` comment on the
  issue (with `closed as completed` / `not planned` alongside it), because
  GitHub has no word that tells failed from blocked.
- `List` reads **one page of 100 items** in board order and stops there. Past
  that, the tail is dropped — see
  [ADR 0002](docs/adr/0002-list-is-a-page-and-a-verdict-costs-a-request.md).
- A **pull request or a draft card** on the board is not the fleet's work, and
  neither is an issue in another repo, so the bot that opened a PR is not a
  task.
- The poll asks for titles and the two columns only. Bodies, checklists and
  threads are read by `task view`, which is why a read of the board costs about
  one of GitHub's 5,000 hourly points and not a hundred.

Why a board can be a queue at all, and what that costs: [ADR
0007](docs/adr/0007-a-board-is-a-queue.md).

### Writing an adapter — contributions welcome

Linear, Jira, Notion, a directory of text files: if it holds tasks, it can be a
queue. An adapter is one package under `internal/work/` implementing five
methods — `List`, `Get`, `Create`, `Comment`, `Close` — plus the optional
`Phaser` (a backend that can show work in hand) and `Assigner` (one that can
hand a task to another agent). Nothing above an adapter knows the backend's
name, its status words, or its id format: the core owns the vocabulary
([ADR 0001](docs/adr/0001-the-core-owns-the-vocabulary.md)) and the adapter
translates on the way in and out.

`internal/work/worktest` is a conformance suite any adapter can run against
itself, so "does this behave like a queue?" is a test rather than a review.
Register the new kind in `internal/fleet/source.go` — the one place an adapter
is constructed — and the daemon, the CLI and the board all pick it up at once.
PRs welcome.

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
