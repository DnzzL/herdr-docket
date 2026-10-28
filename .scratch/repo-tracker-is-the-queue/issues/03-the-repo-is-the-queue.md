# 03: The repo is the queue

**What to build:** a backlogmd source with no `dir` fails to load: "source.dir must name a project repo". `init` no longer creates a Backlog.md project in the fleet dir. `herdr-docket init <repo>` creates the fleet if it is missing (roles, agents, `FLEET.md`, as today), then in `<repo>`: runs `backlog init` if `backlog/` is absent, sets the fleet's statuses in `backlog/config.yml`, writes `docs/agents/issue-tracker.md` from an embedded backlogmd template (unless the file exists: then it says so and leaves it), and adds the repo to `fleet.yaml` as a source with `dir:`. Running it twice changes nothing.

The template tells Pocock's skills: a Spec is `backlog task create "<title>" -d <spec body> -a <agent> -s "To Do"` (the card); each Ticket is `backlog task create "<title>" --parent <spec-id> --dep <ids> --ac <criterion>…`; fetch is `backlog task <id> --plain`; close is ticking ACs and setting Done.

**Blocked by:** 01

**Status:** ready-for-agent

- [ ] backlogmd with an empty `dir` fails `LoadSettings` with the message above
- [ ] plain `init` creates no `backlog/` in the fleet dir
- [ ] `init <repo>` on a fresh git repo writes `backlog/config.yml` (fleet statuses), `docs/agents/issue-tracker.md`, and a `source`/`sources` entry with `dir`
- [ ] `init <repo>` twice is a no-op the second time, and an existing `issue-tracker.md` is kept
- [ ] `docs/queues.md`, `docs/commands.md`, README quick start updated; changelog says how to migrate a fleet-dir queue

**Done when:** a repo is ready for the factory with one command, and no queue lives in the fleet dir.

**Verify by:** `go test ./...` green; manual: `herdr-docket init /tmp/r` on a fresh `git init` repo, then `cat /tmp/r/docs/agents/issue-tracker.md`.
