# Where the queue lives

[Back to the README](../README.md)

The queue is a project repo's own tracker: its Backlog.md project, or a GitHub
Projects board ([ADR 0023](adr/0023-the-repos-own-tracker-is-the-queue.md)).
The fleet dir holds agents and roles, never a queue. One command makes a repo
the queue:

```sh
herdr-docket init ~/Projects/myapp
```

It creates the fleet if it is missing, then in the repo: runs `backlog init`
if there is no `backlog/`, adds the fleet's statuses to `backlog/config.yml`
(the repo's own columns are kept), writes `docs/agents/issue-tracker.md` — how
the skills publish, fetch and resolve Specs and Tickets there — unless the repo
already has one, and names the repo as the source in `fleet.yaml`. Running it
again changes nothing. A `fleet.yaml` that already names another queue gets the
repo beside it: `source:` becomes `sources:`, each entry named after its repo
directory, and the old queue stays `default_source`. The file is edited in
place, comments and order kept. A `fleet.yaml` from before the repo was the
queue — a Backlog.md `source:` with no `dir` — gets the repo set as its `dir`.

## The file all of it goes in

Config is optional — `fleet.yaml` in the plugin config dir:

```yaml
dir: ~/somewhere/else     # fleet dir (default ~/fleet)
default_agent: dev        # picks up unassigned tasks; unset = leave them alone
source:
  kind: backlogmd         # where the queue lives (the default kind)
  dir: ~/Projects/myapp   # the repo whose Backlog.md is the queue (required)
```

| Key | What it does |
| --- | --- |
| `dir` | Where the fleet lives: `agents/`, `roles/`, `FLEET.md`. Never the queue. |
| `default_agent` | Who picks up a task with no assignee. Unset: nobody — unassigned work is left alone. |
| `source` | One queue. |
| `sources` | Several, as a map of name → the same block `source:` takes. Each name becomes the id prefix on that queue's tasks. |
| `max_runs` | How many runs the daemon keeps in flight at once, all queues together — every run is a coding agent on this machine. Unset or `0`: no cap. `herdr-docket run` is yours and does not count. |

`source:` and `sources:` are mutually exclusive: one fleet works one arrangement
of queues. Setting both is an error, and the fleet will not pick one for you.

And the block both of them take:

| Key | What it does |
| --- | --- |
| `kind` | `backlogmd` (the default) or `github`. |
| `dir` | The project repo whose Backlog.md is the queue: absolute, `~`, or relative to the fleet dir. Required for `backlogmd`: a block without it fails to load with "source.dir must name a project repo". |
| `default_agent` | Who picks up *this* queue's unassigned tasks, overriding the fleet's. |
| `verifier` | The agent that judges every PR an agent delivers before it can merge ([the pipeline](adr/0013-the-factory-is-an-author-a-verifier-and-a-gate-in-code.md)). Unset: no pipeline, a run ends on the agent's own verdict. |
| `merge` | `auto` lets the gate merge a verified PR on its own. Anything else stops at a PASS for a human. |
| `max_runs` | This queue's cap, under the fleet's: a repo whose test suite cannot run twice at once says `1`. Unset or `0`: no cap. A run's verifier and rework stages count as the one run. |
| `worktree_base` | The git ref a worktree run of this queue branches from. Default: the queue repo's own default branch (`origin/HEAD`) at claim time — never whatever its main checkout happens to have checked out. A fork worked alongside its upstream might want the fork's own `main`. |
| `statuses` | The project's own status words — [a project you already have](#work-a-project-you-already-have). |
| `github` | `owner`, `project`, `repo`, `agent_field`, `status_field`, `in_progress` — [a hosted board](#a-hosted-queue-github-projects). |

## Work a project you already have

`herdr-docket init <repo>` points a source at your repo, or write `dir:`
yourself — absolute, `~`, or relative to the fleet dir. Your project has its own status words, and there are two ways to
meet:

**Let the fleet own the vocabulary.** `init <repo>` adds its five statuses to
your `backlog/config.yml` and there is nothing else to configure:

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
first lever in [How much autonomy](how-it-works.md#the-levers): `todo` is
exactly how much of your board you are handing over.

`todo`, `done`, `failed` and `blocked` are required — a queue the fleet can
pick from but cannot close leaves every task open for the next tick to pick up
again. `in_progress` is optional: it is display only, so a project with no word
for it simply never shows one. Plain `herdr-docket init` never touches a
repo; `init <repo>` only adds missing statuses (the source's own words when it
names them) and never removes one.

Assignees are agent names. Backlog.md prints one as `@name` and accepts one
written that way, so a task filed from its own interface carries the sigil into
the file; the fleet takes it off when it reads the task, because an agent is a
folder in the fleet dir and its name has no `@` in it. A name no agent answers
to still shows as unknown — the sigil comes off, the name does not.

## Several projects at once

One fleet can work several projects, each keeping the tool it already uses. Use
`sources:` — a map of name → the same block `source:` takes — instead of
`source:`:

```yaml
sources:
  myapp:                      # your repo, its own words
    kind: backlogmd
    dir: ~/Projects/myapp
    statuses: {todo: ready-for-agent, done: done, failed: ready-for-human, blocked: needs-info}
  agency:                     # a client's GitHub board, worked alongside it
    kind: github
    github:
      owner: agency
      project: 3
      repo: agency/site
```

Each source's tasks carry its name as an id prefix — `myapp/TASK-12`,
`agency/987` — so every command that takes an id (`task view`, `note`,
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
    kind: github
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

## A hosted queue: GitHub Projects

A GitHub Projects v2 board is a queue too — useful when the work is already
issues in a repo, and the board is how you and a human colleague already look
at it. One command prepares the repo, reading `repo` from its `origin`:

```sh
herdr-docket init ~/Projects/app --github your-org/3   # owner/project-number
```

It writes a GitHub `docs/agents/issue-tracker.md` (a Spec is an issue on the
board with its Agent set, a Ticket a sub-issue of it) and adds the block
below to `fleet.yaml` — beside any queue it already names.
No `backlog/` is created. By hand, the block is:

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
  [ADR 0002](adr/0002-list-is-a-page-and-a-verdict-costs-a-request.md).
- A **pull request or a draft card** on the board is not the fleet's work, and
  neither is an issue in another repo, so the bot that opened a PR is not a
  task.
- The poll asks for titles and the two columns only. Bodies, checklists and
  threads are read by `task view`, which is why a read of the board costs about
  one of GitHub's 5,000 hourly points and not a hundred.

Why a board can be a queue at all, and what that costs:
[ADR 0007](adr/0007-a-board-is-a-queue.md).

## Writing an adapter — contributions welcome

Linear, Jira, Notion, a directory of text files: if it holds tasks, it can be a
queue. An adapter is one package under `internal/work/` implementing five
methods — `List`, `Get`, `Create`, `Comment`, `Close` — plus the optional
`Phaser` (a backend that can show work in hand) and `Assigner` (one that can
hand a task to another agent). Nothing above an adapter knows the backend's
name, its status words, or its id format: the core owns the vocabulary
([ADR 0001](adr/0001-the-core-owns-the-vocabulary.md)) and the adapter
translates on the way in and out.

`internal/work/worktest` is a conformance suite any adapter can run against
itself, so "does this behave like a queue?" is a test rather than a review.
Register the new kind in `internal/fleet/source.go` — the one place an adapter
is constructed — and the daemon, the CLI and the board all pick it up at once.
PRs welcome.
