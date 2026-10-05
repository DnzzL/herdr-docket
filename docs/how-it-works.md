# How it works

[Back to the README](../README.md)

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
  runtimes a post can run on: [Writing an agent](agents.md).
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
[Worked examples](examples.md).

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
   and a PASS reaches the merge gate — [the pipeline](factory.md#the-pipeline).

Runs have a time budget. The prompt tells the agent the honest way out of a
task that won't fit: one coherent slice, a handoff note, a follow-up task —
the queue itself is the checkpoint mechanism.

## The levers

Nothing here is all-or-nothing. Autonomy is a handful of levers, each one a
line of config, and you can move them one project at a time:

| Lever | Keep a hand on it | Let it run |
| --- | --- | --- |
| `statuses.todo` | a column you fill by hand — `ready-for-agent` | your project's default column: everything new is fair game |
| assignee / `default_agent` | name the agent on the task, one at a time | `default_agent` picks up everything unassigned — per project, so each one has its own intake |
| `statuses.failed` | points at a human column — `needs-info`, `ready-for-human` | points at a real `Failed`; nobody is paged |
| `verifier` | a reviewer verifies every delivered PR, you merge ([Example 4](examples.md#4-a-reviewer-that-verifies-the-devs-prs)) | `merge: auto` — the gate merges a PASS on green CI, except `critical` tasks and `CODEOWNERS` paths ([the pipeline](factory.md#the-pipeline)) |
| `disabled` / `agent pause` / `pause` | park an agent — or the whole fleet — while you look at something | never paused |

The one lever with teeth is the first: **the fleet only ever picks up the
status you name**, so handing it a project means handing it one column, not a
board. Your triage, your wontfix, your waiting-on-a-human columns stay yours —
see [Where the queue lives](queues.md).

It never overrides you in the other direction either: `herdr-docket run TASK-12`
reaches a paused agent, because pressing the button is
human intent, not scheduling.

These levers are also the whole of a running *factory* — intake polling
sources into verified tasks, a verifier and a merge gate on every PR, a stall
sweep, a weekly lookback
over what keeps recurring. The loop is personas and cron, wired in
[docs/factory.md](factory.md); `herdr-docket init --factory` writes it
into an existing fleet, the repo-side personas paused until you name the repo.

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
