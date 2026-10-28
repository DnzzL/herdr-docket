# 05: A GitHub repo gets its own template

**What to build:** `herdr-docket init <repo> --github <owner>/<project-number>` writes a GitHub `docs/agents/issue-tracker.md` instead (Spec = issue put on the board and assigned to an agent; Ticket = sub-issue of it, blockers as a "Blocked by: #n" line; fetch = `gh issue view`; close = `gh issue close`) and a `kind: github` source. It skips `backlog init`.

**Blocked by:** 03

**Status:** resolved

- [x] `init <repo> --github org/3` writes the GitHub template and a github source with owner, project and repo (repo read from the repo's `origin`)
- [x] No `backlog/` is created
- [x] `docs/queues.md` GitHub section shows the command

**Done when:** a GitHub-tracked repo is prepared with one command.

**Verify by:** `go test ./...` green, with a test on a temp repo whose `origin` is `git@github.com:org/app.git`.
