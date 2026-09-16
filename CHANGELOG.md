# Changelog

What changed for someone using the plugin. Dates are release dates.

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
