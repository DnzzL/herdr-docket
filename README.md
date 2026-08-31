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
BACKLOG_CWD=~/fleet backlog task create "Draft the Show HN post" \
  -d "150-200 words, link the repo, no superlatives" \
  --ac "a draft lands in docs/launch/" -a dishnow-marketing
```

Within 15 seconds the daemon opens a workspace on that agent's repo, starts a real
coding agent (Claude Code by default) with the agent's persona plus the task, and
the agent reports back into the backlog itself: notes, checked criteria, final status.

## The model

- **The fleet dir** (default `~/fleet`) holds everything: a Backlog.md project
  (`backlog/`) and the agents (`agents/<name>/AGENT.md`). It's a git repo — your
  queue has a history when you decide to commit it.
- **An agent** is a markdown file: YAML frontmatter for the run parameters, body
  for the persona every run opens with.

  ```markdown
  ---
  model: claude-sonnet-5
  workdir: ~/Projects/dishnow-v2
  workspace: root          # or worktree: a fresh branch per run
  ---
  You own dishnow's publication strategy. Read docs/strategy.md first...
  ```

- **A task** is a Backlog.md task. `assignee` picks the agent; priority, ordinal
  and age decide what runs first. Statuses: `To Do → In Progress → Done | Failed | Blocked`.
- **One at a time.** No locking, no races, no pile-ups: the daemon runs a single
  task and polls again when it's done.
- **The agent closes its own task** through the `backlog` CLI. If it doesn't, the
  daemon does — a run that ends silent goes `Failed`, a workspace you closed goes
  `Blocked` (you decided; the board says so).
- **Approvals are the agent's own.** Claude Code asks in its pane like it always
  does; jump in from the board, answer, leave.

## What it isn't

- **Not a scheduler.** Recurring work belongs to
  [herdr-automations](https://github.com/DnzzL/herdr-automations) — point an
  automation's prompt at `backlog task create` and the two plugins compose.
- **Not a workflow engine.** A task is one goal for one agent. Fan-out happens
  the honest way: an agent creates follow-up tasks in the same backlog.
- **No store.** The backlog is markdown, run history is one JSONL file, and
  uninstalling leaves both behind.

## Quick start

```bash
herdr plugin install DnzzL/herdr-fleet
herdr-fleet init                     # backlog project + example agent in ~/fleet
$EDITOR ~/fleet/agents/example/AGENT.md
BACKLOG_CWD=~/fleet backlog task create "First task" -a example
```

The board pane (overlay in Herdr, or `herdr-fleet pane`) shows the queue grouped
by status: `r` runs a task now, `enter` jumps into the workspace a run opened,
`a` adds a task.

Config is optional — `fleet.yaml` in the plugin config dir:

```yaml
dir: ~/somewhere/else     # fleet dir (default ~/fleet)
default_agent: backlog    # picks up unassigned tasks; unset = leave them alone
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

## Teaching your agents

`skills/fleet-tasks/SKILL.md` teaches a coding agent to create well-formed fleet
tasks (and to keep the fleet backlog separate from a project's own). Symlink or
copy it into `~/.claude/skills/`.
