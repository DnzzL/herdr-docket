# herdr-fleet

**A shared task backlog worked by your coding agents.** A [Backlog.md](https://backlog.md)
project as the queue, `AGENT.md` personas as the workers, and a daemon that routes
every `To Do` task to the agent it names — one run at a time, in a [Herdr](https://herdr.dev)
workspace you can watch, join, or close.

```
      you ─────────┐
 an automation ────┼──► fleet backlog (To Do) ──► daemon ──► herdr agent run ──► Done | Failed | Blocked
 another agent ────┘         markdown, git             one at a time         (the agent reports itself)
```

Write a task, assign it to an agent, walk away:

```bash
BACKLOG_CWD=~/fleet backlog task create "Triage the project backlog" \
  -d "Route every needs-triage task: out of scope, agent-ready, or needs a human." \
  --ac "no task left in needs-triage" -a pm
```

Within 15 seconds the daemon opens a workspace on that agent's repo, starts a real
coding agent (Claude Code by default) with the agent's persona plus the task, and
the agent reports back into the backlog itself: notes, checked criteria, final
status. You come back to a board that tells the truth — and to nothing else,
because a run that ends `Done` cleans its own workspace up.

If you've wanted a tiny [paperclip.ing](https://paperclip.ing)-style company of
agents without the org chart: this is the smallest version of that idea that
still works. Named agents, a shared backlog, a human gate. Markdown all the way
down.

## The model

Everything lives in one **fleet dir** (default `~/fleet`) — a git repo you can
read, diff, and back up:

```
~/fleet/
├── backlog/           # the Backlog.md project: one markdown file per task
└── agents/
    ├── pm/AGENT.md    # who the agents are
    └── dev/AGENT.md
```

- **A task** is a Backlog.md task: goal, description, acceptance criteria,
  priority, and an `assignee` that names the agent. Statuses are the lifecycle:
  `To Do → In Progress → Done | Failed | Blocked`.
- **An agent** is one markdown file: YAML frontmatter for the run parameters,
  body for the persona every one of its runs opens with.
- **One at a time.** No locking protocol, no checkout races, no pile-ups: the
  daemon runs a single task to completion, then polls again. A second `To Do`
  task simply waits its turn.
- **The agent closes its own task** through the `backlog` CLI — it checks off
  acceptance criteria, appends what it did to the notes, and sets the final
  status. If it doesn't, the daemon does: a run that ends silent goes `Failed`,
  a workspace you closed mid-run goes `Blocked` (you decided; the board says so).
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
agent: claude                 # optional — any kind `herdr agent start` supports
---

You are the persona every run of this agent opens with. Say who the agent
is, what it owns, what its bias is, and how it should decide. This body is
the difference between a generic LLM and a colleague.
```

`workspace: worktree` gives every run a disposable branch (`fleet/task-12-…`) —
a run that went sideways is a diff you throw away. `root` is for agents whose
job *is* the working copy: backlog grooming, docs, anything that must see
uncommitted state.

### Example 1 — a PM that triages your project's backlog

The fleet backlog routes agents; your project's own `backlog/` tracks its dev
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
BACKLOG_CWD=~/fleet backlog task create "Triage the backlog" -a pm \
  --ac "no task left in needs-triage" --ac "every verdict has a comment"
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

Every run lands on its own `fleet/…` branch: review the diff, merge or delete.

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
assigned to yourself — the backlog is your plan.
```

Seed it once — *"Write the publication strategy, then decompose it into
tasks"* — and it fans out: each follow-up run is one concrete step (draft the
Show HN post, prepare the launch thread…), created by the agent itself.

## Anatomy of a run

1. The daemon polls `backlog task list --json` (~15s) and picks the most
   urgent routed `To Do` task: priority, then ordinal, then age.
2. It claims the task (`In Progress`), provisions a Herdr workspace on the
   agent's `workdir`, and starts the agent with an assembled prompt: persona
   + full task (description, acceptance criteria, notes from previous runs)
   + the reporting protocol.
3. The agent works — you can watch it live, jump in, answer its permission
   prompts, or close its workspace to call the run off.
4. The agent reports: checks `--check-ac`, appends `--append-notes`, sets
   `Done`, `Failed`, or `Blocked`. The daemon reconciles anything left hanging
   and records the run in an append-only `history.jsonl`.

Runs have a time budget. The prompt tells the agent the honest way out of a
task that won't fit: one coherent slice, a handoff note, a follow-up task —
the backlog itself is the checkpoint mechanism.

## Quick start

```bash
herdr plugin install DnzzL/herdr-fleet
herdr-fleet init                     # backlog project + example agent in ~/fleet
$EDITOR ~/fleet/agents/example/AGENT.md
BACKLOG_CWD=~/fleet backlog task create "First task" -a example
```

The board pane (overlay in Herdr, or `herdr-fleet pane`) shows the queue grouped
by status: `r` runs a task now, `enter` jumps into the workspace a run opened,
`a` adds a task. Bind it to a chord in `~/.config/herdr/config.toml`:

```toml
[[keys.command]]
key = "prefix+f"
type = "shell"
command = "herdr plugin pane open --plugin dnzzl.fleet --entrypoint board --placement overlay"
```

Config is optional — `fleet.yaml` in the plugin config dir:

```yaml
dir: ~/somewhere/else     # fleet dir (default ~/fleet)
default_agent: dev        # picks up unassigned tasks; unset = leave them alone
```

## Commands

| | |
|---|---|
| `herdr-fleet daemon` | the worker (Herdr starts it for you) |
| `herdr-fleet init` | bootstrap the fleet dir |
| `herdr-fleet list` | tasks by status, with the routed agent |
| `herdr-fleet run TASK-12` | run one task now |
| `herdr-fleet history [TASK-12]` | recent runs |
| `herdr-fleet pane` | the interactive board |

## What it isn't

- **Not a scheduler.** Recurring work belongs to
  [herdr-automations](https://github.com/DnzzL/herdr-automations) — point an
  automation's prompt at `backlog task create` and the two plugins compose:
  automations decide *when*, fleet decides *what* and *who*.
- **Not a workflow engine.** A task is one goal for one agent. Fan-out happens
  the honest way: an agent creates follow-up tasks in the same backlog.
- **Not parallel.** One run at a time is the concurrency model, not a missing
  feature — it's what makes the queue race-free with zero infrastructure.
- **No store.** The backlog is markdown in a git repo, run history is one JSONL
  file, and uninstalling leaves both behind.

## Teaching your agents

`skills/fleet-tasks/SKILL.md` teaches a coding agent to create well-formed fleet
tasks — real descriptions, at least one acceptance criterion, and the rule that
keeps the fleet backlog separate from a project's own. Symlink or copy it into
`~/.claude/skills/`.

## License

MIT.
