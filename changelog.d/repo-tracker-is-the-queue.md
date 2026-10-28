- **The repo's own tracker is the queue: `herdr-docket init <repo>`.** Plain
  `init` no longer creates a queue in the fleet dir; `init <repo>` prepares a
  project repo instead — its Backlog.md project with the fleet's statuses, a
  `docs/agents/issue-tracker.md` the skills publish through, and the repo named
  as a source in `fleet.yaml`, beside any queue already there. A second repo turns `source:` into
  `sources:`, so task ids gain the queue's name as a prefix (`app/TASK-1`):
  finish or stop runs in flight first. To migrate, run
  `herdr-docket init <repo>` on your existing `fleet.yaml`: it sets the repo as
  the `dir` of a Backlog.md source that had none. To keep the old open tasks,
  move `~/fleet/backlog/tasks/*` into the repo's `backlog/tasks/` and commit,
  checking first that no id collides with the repo's own.
- **Basecamp and Notara are no longer queues.** A `fleet.yaml` with
  `kind: basecamp` or `kind: notara` fails to start and names the kinds left,
  `backlogmd` and `github`; `auth basecamp` and `auth notara` are gone. Move
  the work into the repo's Backlog.md or a GitHub Projects board and delete
  the old block and its section of `credentials.yaml`.
- **A spec's tickets are never handed to an agent.** A ticket (a Backlog.md
  `--parent` task or a GitHub sub-issue) is the implementer's plan: the fleet
  routes only its spec, and `herdr-docket list` shows it as a ticket of that spec.
- **The dev and reviewer roles read the card as the spec**, its tickets fetched
  the way the repo's `docs/agents/issue-tracker.md` says. A `roles/dev.md` or
  `roles/reviewer.md` from an earlier `init` keeps the old wording until you
  delete it and re-run `init`.
- **A GitHub-tracked repo is prepared in one command:**
  `herdr-docket init <repo> --github <owner>/<project-number>` writes the
  GitHub tracker doc and a `kind: github` source whose `repo` is read from
  `origin`, with no Backlog.md project.
- **Ideas are grilled on the card.** Two new statuses: a card in `Triage` goes
  to the agent a source names as `triager:` whatever its assignee (no triager,
  it waits), and a card in `Question` waits on your answer and is never picked.
  Re-run `herdr-docket init <repo>` to add both to `backlog/config.yml`; a
  project with its own `statuses:` names them as `triage:` and `question:`.
- **A card is grilled through its notes.** `herdr-docket task ask <id>` moves a
  card to `question`; `herdr-docket task answer <id> "1.A 2. free text"` posts
  `## Answer N` for the card's newest `## Round N` note and moves it back to
  `triage`, and fails naming the card when it has no round.
- **`scripts/box-setup` sets up an always-on box**: a re-runnable wizard for
  Tailscale, the Herdr server, `gh` and agent CLI logins, the plugin,
  `init <repo>` per repo, one `intake` workspace per repo, and a `max_runs`
  sized from RAM. `--dry-run` prints the commands without running them.
- **`herdr-docket status` says whether the daemon is alive:** `daemon live ·
  tick 3s ago`, `daemon down for 4 min` once no tick has run for three
  intervals, or `daemon never started`.
- **The triager grills a card one round per run.** A `Triage` card starts a
  run of the source's `triager` (`trigger: triage` in `history`): it posts a
  `## Round N` and moves the card to `Question`, or rewrites it as the spec,
  publishes its tickets and hands it to the author in `To Do` — no pull
  request, no verifier. A run that leaves the card in `Triage` fails. `init`
  now writes `roles/triager.md`; name it with `role: triager`.
