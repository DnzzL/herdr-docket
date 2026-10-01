# Changelog

What changed for someone using the plugin. Dates are release dates.

## Unreleased

- **A task lands in the queue of the project you are standing in.** A fleet
  that grows a second queue used to break every prompt and persona written
  before it: they all call `task create` without naming one, and the CLI
  refused rather than guess. It no longer has to — an agent runs inside the
  checkout of the project it was given, or a worktree of it, so the working
  directory already names the queue. `-s` still wins, and `default_source` in
  `fleet.yaml` covers the one caller standing nowhere: a scheduled automation
  filing work for somebody else. A `default_source` naming no declared queue
  is refused when the config loads, not at 3am.
- **Work is never routed to an agent of another project.** An agent works one
  checkout; a task belongs to the queue it came from. When those disagreed the
  run went ahead and did the work in the wrong repository — succeeding, so
  nothing downstream could see it. The daemon now leaves that task alone and
  says once what disagrees with what, the way it already does for an assignee
  nobody answers to. The binding is derived from the agent's workdir, so it
  cannot drift from the truth the run would use.

## v0.7.0 — 2026-09-30

- **The factory loop, installed in one command.** `herdr-docket init --factory`
  writes all six personas into an existing fleet — never overwriting one you
  already have — and appends the four schedules that drive the loop to the
  automations plugin's config when it is present, pointing `repo:` at the
  fleet it was installed into. The three that read the queue (`intake`,
  `stall`, `lookback`) are pointed at the fleet and run; the three that work a
  repo (`pm`, `dev`, `reviewer`) land paused on a placeholder workdir, because
  every stage the loop names has to exist — intake and lookback file to `pm`,
  the reviewer takes the dev's handoff — and none of them may run against a
  repo you did not choose. Edit the workdir, `herdr-docket agent resume`, and
  drop the `disabled: true` the `review-sweep` entry ships with. Without the
  automations plugin the personas still land and the output says they await
  schedules. The fleet's own `fleet.yaml` is not touched.

- **The loop is written down**: `docs/factory.md` is the wiring — the cron
  entries, the dials it leans on, and the refusals (the fleet never merges;
  agents do, under a policy paragraph in the reviewer's persona). The loop's
  three new personas join the worked examples, and the dev and reviewer
  personas gain what makes their stage real: the dev babysits its PR to
  green, the reviewer's merge policy decides what ships unattended.

- **A run's delivery is read, not believed.** A worktree-mode run's history
  records the branch it was provisioned on, how many commits that branch
  gained, and the pull request it shipped as — reported through the new
  `herdr-docket task done|fail|block --pr <url>` — so `herdr-docket history`
  answers *what did this run produce* without reading prose on a task. A
  root-mode run claims no branch, because it has none to vouch for.

- **A worktree that still holds uncommitted changes is not torn down.** When a
  disposable worktree ends `done` with changes nobody committed, the fleet
  keeps the workspace, says so on the task, records the fact on the run, and
  moves the task to the human-decides column — the agent's report and the
  worktree's state disagree, and the board now says which it found. A delivery
  that cannot be read is kept and called unverified; blindness is not evidence.

- **A reported verdict is no longer recorded as silence.** A run whose agent
  judged the ticket and closed it — a reviewer refusing to merge a migration,
  say — was filed with the same sentence as an agent that closed nothing:
  `the agent settled without reporting a verdict`. On a fleet whose config
  points `failed` and `blocked` at one column, that was every correct refusal.
  The run still ends failed, because the task did; the record now says whether
  anybody needs to go and look at the agent.

- **A budget counts runs, not log lines.** Attaching a pull request to a run
  that had already closed appended a second closing record for it, and the
  budget charged both — an agent that had run four times read as `5/4` and was
  parked with spend left. The window now collapses per run, the way the run
  list always has. The line also says the window it measures: `4/6 runs in 24h`,
  because the window is a rolling day and never reset at midnight.

- **A persona that does not load is named on the board.** `agent list` printed
  it and the daemon logged it; the pane dropped it, so the agent vanished from
  the roster and its work drew as `ghost?` — telling you to fix an assignee
  when the fault was a file you had just edited. The roster now names the file
  and the reason, and the agent column says `broken` rather than `ghost?`.

- **The board refreshes in one pass over the history.** It read a record per
  row, so a refresh walked the whole log once per task — 1.4s every five
  seconds on a fleet of 80 tasks, on a file the plugin never truncates. Now
  30ms, and it grows with the log rather than with the log times the board.

- **`g` reaches the roster from the task detail**, which is what the line under
  it always said; it went to the board instead.

- **An agent in root mode is told the checkout is not its workspace.** The
  `reviewer` and `pm` personas run in the project checkout because the tasks
  they file are files in it; nothing said the rest of that checkout belongs to
  a human. Found on a live fleet as a four-day-old `reviewer-stash` holding
  thirty task files — an agent had stashed a working tree to get a clean one
  for its own testing and never popped it. Both personas now refuse git in the
  root checkout and build their own throwaway worktree instead.

## v0.6.0 — 2026-09-21

- **The board pane is the fleet's triage surface: what runs next, what is
  running now, and the keys to act on both.** A task with a run in flight is in
  a `Running` group above the phases — shown once, not twice — with the time it
  has been going against the timeout it was started with, marked `stale` past
  it. Everything below is in the order the daemon works it, so the top of
  `To Do` is the task the next tick picks up, which it was not before.
- A row names the queue a task came from and the id that queue's own board
  uses — `TASK-12` beside `myapp`, not another truncation of `myapp/TASK-12` —
  in a colour that stays with the queue. A fleet with a single queue shows no
  such column. The agent column says the two ways routing can be wrong: yellow
  `paused` for an agent the scheduler is skipping, red `ghost?` for a name
  nobody answers to.
- Three new keys and one new view. `v` reads the selected task — body,
  criteria and notes, the same renderer as `herdr-docket task view` — and `s`
  re-routes it to another agent. `x` stops the run on the selected row, which
  closes its workspace and ends the task `Blocked` through the path a cancelled
  run already took — and then says which of the two it found: `closed
  TASK-12's workspace`, or `its workspace is already gone` when the record
  outlived the run. `g` now shows the roster with each agent busy (and on what)
  or idle, and `p` pauses or resumes the agent under the cursor: pausing writes
  `disabled:` to its `AGENT.md` and never touches a run already in flight — `x`
  is the key that stops one, and `r` still runs a paused agent's task by hand.
- **An assignee written `@name` routes to the agent `name`.** That is how
  Backlog.md prints an assignee and how its CLI accepts one, so the sigil
  reaches the task file; it used to carry on into the router, where the task
  went to nobody while its row looked routed. A name no agent answers to is
  still shown as unknown — the sigil comes off, the name does not.
- `a` asks which queue a task belongs in when the fleet has more than one, so
  creating work from the pane no longer fails on a bare `task create`. Nothing
  in the pane closes a task with a verdict: see
  [ADR 0008](docs/adr/0008-the-board-is-a-triage-surface.md) for what it shows
  and what it deliberately refuses.

## v0.5.0 — 2026-09-16

- **A GitHub Projects board can be the queue.** `kind: github` with the board's
  `owner`, its `project` number and the `repo` its issues live in: a task is a
  real issue that has been put on the board, `assign` writes the board's
  `Agent` field, and `note`, `done`, `fail` and `block` land on the issue's
  thread as they do anywhere else. The token wants the `project` scope;
  `herdr-docket auth github` checks it, stores it (`--token ghp_…` stores a PAT
  instead) and creates the `Agent` field with one option per agent. It is looked
  for in `HERDR_DOCKET_GITHUB_TOKEN`, then `gh auth token`, then the stored copy.
- A GitHub board is read **one page of 100 at a time**, in the board's own
  order: past that the tail waits for the next poll, and a pull request or a
  draft card on the board is not the fleet's work. The poll asks for titles and
  the two columns only — bodies, checklists and comments are read by
  `task view` — so a tick costs about one of GitHub's 5,000 hourly points
  rather than a hundred.
- The README named `HERDR_FLEET_BASECAMP_CLIENT_ID` and `_SECRET` for the
  Launchpad app. The names the code reads are `HERDR_DOCKET_BASECAMP_CLIENT_ID`
  and `HERDR_DOCKET_BASECAMP_CLIENT_SECRET`.

## v0.4.0 — 2026-09-14

- **The plugin is `herdr-docket`, where it was `herdr-fleet`.** The id moves
  with it — `dnzzl.herdr-docket` — so the config dir, the state dir, and every
  command (`herdr-docket ...`) follow. Re-link or reinstall, and point any
  existing fleet setup at the new id.
- `herdr-docket install-skill` writes the bundled agent skill into a runtime's
  own skill location, so an agent learns the queue verbs — `note`,
  `done`/`fail`/`block`, `assign`, `create` — instead of you copying the file by
  hand.
- Reading `herdr`'s error envelope from the stream it actually writes: a failing
  `herdr` call used to surface as a parse error instead of the real message.

## v0.3.0 — 2026-09-13

- **Roles: a method two agents share.** `role: dev` in an agent's frontmatter
  names `roles/dev.md`, and a run is assembled from three layers, widest first —
  the fleet brief, the role, the persona. Two devs on two projects share one
  method instead of drifting apart. A `role:` naming a file that does not exist
  is an error, not silence: the agent does not load, and `agent list` says why.
- The prompt now states the queue's own status words for the task's project, so
  an agent that writes a board status by hand uses a word its own CLI accepts.
- Running an agent on `pi`, `opencode`, or another runtime is documented, with
  the model-id spelling each runtime expects.

## v0.2.0 — 2026-09-13

- **Budgets.** `runs_per_day` and `minutes_per_day` cap an agent over a rolling
  24 hours. A spent agent is like a busy one — its tasks stay open, nothing is
  written on them, and the rest of the queue keeps moving. Unset means
  unbounded. `agent list` shows the spend for any agent that has a budget.
- **A shared brief.** `FLEET.md` in the fleet dir, when present, is prepended to
  every persona — what is true of the whole fleet lives in one file instead of
  being repeated in each `AGENT.md`.
- **Several queues behind one port.** A `sources:` block names one queue per
  project, addressed by a prefixed id (`myapp/TASK-12`). Each project's queue has
  its own intake and its own default agent, and a source can work a project's own
  backlog rather than a central one.
- **`task assign <agent>`.** A run can hand its task to another agent instead of
  closing it: the task keeps its whole history in one place, and the fleet routes
  it on the next tick. A task re-routed after a clean run is read as handed on,
  not abandoned.

## v0.1.0 — 2026-09-12

The first release.

- A shared task queue worked by Herdr agents: a Backlog.md project as the queue,
  `AGENT.md` personas as the workers, and a daemon that routes every open task to
  the agent it names — agents in parallel, one run per checkout, in workspaces
  you can watch, join, or close. A `Done` run cleans up its own workspace.
- **Backends behind one port.** Backlog.md and Basecamp both satisfy
  `work.Source`; a conformance suite judges any new backend by behaviour, and
  nothing above the port knows which backend answered.
- **The fleet's words are its own.** Phase and verdict are the fleet's
  vocabulary, translated by each adapter; a priority is the backend's rank.
- The board — live search, an agents view, and `r` to run a task now.
