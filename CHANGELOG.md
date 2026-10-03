# Changelog

What changed for someone using the plugin. Dates are release dates.

## Unreleased

- **A queue can verify and merge on its own.** Name a `verifier:` on a queue
  and a PR delivered with `task done --pr` stays open, goes to that agent,
  comes back to its author on a FAIL (twice at most), and on a PASS reaches a
  merge gate in code: verdict on the PR's current diff, green CI, no
  `critical` label, no `CODEOWNERS` path, and `merge: auto`. Anything else is
  held for you with the reason, and notified. `task verdict ID PASS|FAIL --pr
  URL` is the verifier's one command. A queue without `verifier:` works as
  before.
- **The factory's reviewer verifies, it no longer merges**, and the
  `review-sweep` automation is gone: every delivered PR reaches the verifier.

- **The board pane is gone.** Herdr's sidebar and `herdr agent list` show what
  is running; `herdr-docket history` and `logs` show what ran.
  `herdr-docket pane` and the plugin's board overlay no longer exist.
- **Budgets are gone.** `runs_per_day` and `minutes_per_day` are ignored;
  `agent pause` stops an agent that runs away.
- **The factory has no pm.** Intake blocks what it cannot verify, with the one
  question for you; lookback files its structural fixes to `dev`.
  `init --factory` writes five personas, not six.

## v0.8.0 — 2026-10-02

- **The prompt names a CLI the agent can actually run.** The plugin is not on
  a pane's PATH, and every run was told to close itself with
  `herdr-docket task done`. The work got done; the closing command did not
  exist; the fleet recorded the silence that followed as the agent's failure.
  Seventeen runs in a fortnight, including one that had already pushed a green
  pull request. The binary writing the prompt knows where it lives, so the
  commands it prints are absolute when the bare name does not resolve — and
  the prose still reads `herdr-docket`, because an agent copies the indented
  lines and a sentence full of path is a sentence nobody gains from.

- **A task with a run in flight cannot be started twice.** The run lock was
  keyed per agent (or per shared checkout), so it kept two runs off one
  workspace — but a `fleet.yaml` edit re-routes the *next* pick from under a
  live run, and two different agents hold two different keys: an unassigned
  task mid-run under one default agent was picked up by another within one
  tick of the default changing. Every run now also holds a flock keyed by the
  task id (`run-taskid-<id>.lock` in the state dir), across processes — the
  daemon and a manual `herdr-docket run_TASK-x` cannot double a task while
  either one is live. A run whose process died releases the task as before:
  flock dies with the process, so the In-Progress self-heal still picks work
  up the next tick.

- **New work a task create files lands where the daemon can claim it.**
  `herdr-docket task create` never named a status, so the queue filed new
  work into whatever the project considered its default — and where that
  word was not the word this fleet maps to `To Do`, every task the fleet
  created (the intake meta-task, a reviewer's follow-up, a lookback fix)
  sat open in a column no run would ever read. A Backlog.md queue now files
  them into its own pickup status directly at creation, in the project's own
  vocabulary; a queue with no column to start a task in (Basecamp) is
  untouched and the CLI says where the task landed rather than implying the
  fleet chose. No new flag: the fleet knows which word it wants, and a human
  picking a specific column still does it where the queue lives.

- **A run branches from the queue's default branch, not your working copy.** A
  worktree run used to fork whatever the project's main checkout had checked
  out at the moment it started, so a human mid-branch silently became the
  base an agent built on — observed as a pull request carrying two commits by
  two authors. A run now cuts from the queue repo's own default branch
  (`origin/HEAD`); a source block can pin another ref with `worktree_base:`.
  The run record names the exact commit the branch is cut from (`cut <sha>`
  in `herdr-docket history`), so a PR carrying anything past that is visible
  from the history alone. Root-mode runs are untouched — borrowing the
  checkout they stand in is their design.

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

- **A root-mode run opens a tab in the project's workspace instead of a
  workspace beside it.** A fleet of three projects was putting one sidebar
  entry per run in front of you — 29 of them, of which three meant anything.
  Worktree runs were already grouped: herdr derives that from the repository,
  not from anything the fleet says. Root runs had no such tie and could not
  get one, so they borrow the workspace the project is already open in. The
  parent is found, never configured — a workspace id written into a config
  file is a reference that rots the first time somebody closes it. A project
  nobody has open still gets a workspace of its own: the fleet never creates
  the primary it would then be borrowing. Stopping or cleaning up a borrowed
  run closes its tab and leaves the workspace, the runs beside it, and your
  own work in it alone.

- **The review sweep stops re-reviewing a PR it has already judged.** Coverage
  was "no open task mentions this PR", but a review task always closes, and a
  PR the reviewer refuses to merge stays open by design — so every morning the
  same PR qualified again. Coverage is now a fact about the PR: a verdict at
  its current head commit, which a new commit invalidates and nothing else.

- **A task routed to an agent nobody answers to is told who does answer.** The
  daemon's refusal used to quote the ghost and offer `add agents/<name>/AGENT.md`
  — an instruction-shaped option one model took during the 2026-09-25 review
  leg, writing a retired name back onto a task mid-run. The comment now lists
  the fleet's agents and says reassign the task to one of them; creating an
  agent is a human's decision and stays out of the note. Refusal still written
  once per task and unknown name, and only for names no agent answers to.

- **An agent is not resumed onto a checkout that is not there.** `init
  --factory` ships three agents on a placeholder workdir and prints "point its
  workdir at your repo, then resume"; nothing enforced the *then*, so the
  agent went active, the daemon routed work to it, and the run died at
  provision with the board saying only that it failed. `agent resume` now
  refuses and names the path to edit. Pausing is untouched: stopping an agent
  is always safe.

- **A failed run shows a desktop notification.** Runs the daemon starts while
  you are elsewhere — the morning sweep working its way down the queue — used
  to fail silently: the task read failed on the board, and the record in
  `herdr-docket history` said so, but only if you came back and looked. The
  daemon now raises Herdr's own notification (`herdr notification show`) once
  per failed run, naming the task and the reason. Done and cancelled runs stay
  quiet — they are the fleet working and a human deciding respectively. The
  notification is asked for once, best-effort: a channel that cannot fire
  never changes a verdict's record.

- **`herdr-docket logs` reads the daemon log.** The daemon's stderr has always
  been redirected into the plugin's state dir, but the path was remembered by
  whoever set it up. The tail (`-n LINES`, 100 by default) is now one command
  away — a report of what the daemon has said so far, not a second log.

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
