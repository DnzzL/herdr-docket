- **The repo is the queue: `herdr-docket init <repo>`.** One command makes a
  project repo ready for the factory: its Backlog.md project with the fleet's
  statuses, a `docs/agents/issue-tracker.md` the skills publish through, and
  the repo named as the source in `fleet.yaml`. Plain `init` no longer creates a
  queue in the fleet dir, and a `backlogmd` source with no `dir` now fails to
  load ("source.dir must name a project repo"). To migrate a fleet-dir queue,
  run `herdr-docket init <repo>` (or set `source.dir` to the repo yourself), and
  if you want its open tasks there, `mv ~/fleet/backlog/tasks/*` into the
  repo's `backlog/tasks/` and commit (two repos: `git mv` cannot cross them),
  checking first that no id collides with the repo's own tasks.
