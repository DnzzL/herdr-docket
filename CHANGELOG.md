# Changelog

What changed for someone using the plugin. Dates are release dates.

## v0.4.0 — unreleased

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
