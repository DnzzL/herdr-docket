# Writing an agent

[Back to the README](../README.md)

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

Every field:

| Field | What it does |
| --- | --- |
| `workdir` | **Required.** The repo the run happens in. |
| `workspace` | `worktree` (default) — a fresh `fleet/task-12-…` branch per run; `root` — work directly on the checkout. |
| `model` | Passed to the runtime as `--model`; the *runtime's* spelling, not the fleet's. |
| `agent` | Which runtime starts the run: `pi`, `claude`, `codex`, `gemini`, `opencode`, `cursor`, … (default `claude`). |
| `agent_args` | Anything else the runtime takes, passed through untouched and *after* `--model` and `--mcp-config`. |
| `mcp_config` | Emits `--mcp-config`, which is Claude Code's flag — leave it empty on any other runtime. |
| `role` | Shared method: `roles/<role>.md`, prepended after `FLEET.md`. |
| `timeout_minutes` | One run's time budget (default 60). |
| `runs_per_day` | Cap on poll-triggered runs per rolling 24 hours. Unset — unbounded. |
| `minutes_per_day` | Cap on their total minutes over that same window. |
| `disabled` | `true` keeps the persona and takes the agent out of scheduling. |

`workspace: worktree` gives every run a disposable branch (`fleet/task-12-…`) —
a run that went sideways is a diff you throw away. `root` is for agents whose
job *is* the working copy: backlog grooming, docs, anything that must see
uncommitted state.

The choice matters most for personas that must *write* to a project's own
`backlog/`: a checkbox ticked inside a disposable worktree is gone with the
worktree. Do the reading in a scratch worktree, the recording in the repo root,
and say so in the persona.

## Running a post on another agent runtime

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

## Sharing a method between agents

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
rename away from being wrong, and does not need to exist. The words live in
`fleet.yaml`; see [Where the queue lives](queues.md).

## Budgeting an agent

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

## Parking an agent

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
agent, because pressing the button is human intent, not scheduling. The board
says the same thing in yellow on every row an agent is paused on.

## See also

- [Worked examples](examples.md) — four personas you can copy, and how they
  compose into one fleet.
- [Where the queue lives](queues.md) — the status words a persona must not name.
- [The board pane](pane.md) — watching a run, and jumping in.
